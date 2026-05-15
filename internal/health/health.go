// Package health performs HTTP reachability checks against every
// mirror defined in mirrors.yml. Ported from scripts/check_mirrors.py
// so that the resulting JSON report stays drop-in compatible with
// reports/report.json consumers (CI, README renderer, etc.).
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/loredunk/china-mirror/internal/mirrors"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxWorkers = 10
	defaultUserAgent  = "china-mirror-skills/0.1.0 (Health Check)"
)

// Result mirrors MirrorCheckResult in check_mirrors.py field-for-field.
type Result struct {
	ID             string  `json:"id"               yaml:"id"`
	Name           string  `json:"name"             yaml:"name"`
	Category       string  `json:"category"         yaml:"category"`
	URL            string  `json:"url"              yaml:"url"`
	Status         string  `json:"status"           yaml:"status"`
	HTTPStatus     *int    `json:"http_status"      yaml:"http_status"`
	ResponseTimeMS *float64 `json:"response_time_ms" yaml:"response_time_ms"`
	ErrorMessage   string  `json:"error_message,omitempty"   yaml:"error_message,omitempty"`
	CheckedURL     string  `json:"checked_url,omitempty"     yaml:"checked_url,omitempty"`
	VerifyType     string  `json:"verify_type,omitempty"     yaml:"verify_type,omitempty"`
	IsInconclusive bool    `json:"is_inconclusive"           yaml:"is_inconclusive"`
	Timestamp      string  `json:"timestamp"                 yaml:"timestamp"`
}

// Report has the same top-level shape as the Python script's output.
type Report struct {
	Metadata          Metadata                    `json:"metadata"`
	Summary           Summary                     `json:"summary"`
	Results           []Result                    `json:"results"`
	CategoryRankings  map[string]CategoryRanking  `json:"category_rankings"`
	Recommendations   []Recommendation            `json:"recommendations"`
}

type Metadata struct {
	GeneratedAt        string `json:"generated_at"`
	Version            string `json:"version"`
	MirrorDataUpdated  string `json:"mirror_data_updated"`
}

type Summary struct {
	TotalMirrors        int                          `json:"total_mirrors"`
	Healthy             int                          `json:"healthy"`
	Unhealthy           int                          `json:"unhealthy"`
	HealthRate          float64                      `json:"health_rate"`
	CategoryStats       map[string]CategoryStat      `json:"category_stats"`
	StatusBreakdown     map[string]int               `json:"status_breakdown"`
	CriticalCategories  []string                     `json:"critical_categories"`
}

type CategoryStat struct {
	Total  int `json:"total"`
	OK     int `json:"ok"`
	NonOK  int `json:"non_ok"`
	Failed int `json:"failed"`
}

type CategoryRanking struct {
	RecommendedMirror   *RankedResult  `json:"recommended_mirror"`
	RecommendedMirrorID string         `json:"recommended_mirror_id,omitempty"`
	RankedResults       []RankedResult `json:"ranked_results"`
	AllFailed           bool           `json:"all_failed"`
	Critical            bool           `json:"critical"`
}

// RankedResult embeds Result + the per-category rank and the mirror's
// static priority, matching the Python `asdict + rank/priority` shape.
type RankedResult struct {
	Result
	Rank     int  `json:"rank"`
	Priority *int `json:"priority"`
}

type Recommendation struct {
	Category string `json:"category"`
	Severity string `json:"severity"` // critical | warning
	Message  string `json:"message"`
	Action   string `json:"action"`
}

// Options bundles per-run knobs that override the defaults in mirrors.yml.
type Options struct {
	Timeout    time.Duration
	Retries    int
	UserAgent  string
	MaxWorkers int
	Parallel   bool
	Category   string // optional filter; empty = all
	Progress   func(Result)
}

func optsFromStore(s *mirrors.Store, override Options) Options {
	cfg := s.File.HealthCheck
	out := Options{
		Timeout:    defaultTimeout,
		Retries:    0,
		UserAgent:  defaultUserAgent,
		MaxWorkers: defaultMaxWorkers,
		Parallel:   true,
		Category:   override.Category,
		Progress:   override.Progress,
	}
	if cfg.Timeout > 0 {
		out.Timeout = time.Duration(cfg.Timeout) * time.Second
	}
	if cfg.Retries > 0 {
		out.Retries = cfg.Retries
	}
	if cfg.UserAgent != "" {
		out.UserAgent = cfg.UserAgent
	}
	if cfg.MaxWorkers > 0 {
		out.MaxWorkers = cfg.MaxWorkers
	}
	out.Parallel = cfg.Parallel
	if override.Timeout > 0 {
		out.Timeout = override.Timeout
	}
	if override.Retries > 0 {
		out.Retries = override.Retries
	}
	if override.UserAgent != "" {
		out.UserAgent = override.UserAgent
	}
	if override.MaxWorkers > 0 {
		out.MaxWorkers = override.MaxWorkers
	}
	return out
}

// Run probes every mirror in store and returns the full report. It does
// not write anything to disk; callers decide where the JSON goes.
func Run(ctx context.Context, store *mirrors.Store, opts Options) *Report {
	o := optsFromStore(store, opts)

	pool := store.All()
	if o.Category != "" {
		pool = store.ByCategory(o.Category)
	}

	results := checkAll(ctx, pool, o)
	return buildReport(results, store)
}

func checkAll(ctx context.Context, pool []mirrors.Mirror, o Options) []Result {
	results := make([]Result, len(pool))

	work := func(i int) {
		results[i] = checkOne(ctx, pool[i], o)
		if o.Progress != nil {
			o.Progress(results[i])
		}
	}

	if !o.Parallel || o.MaxWorkers <= 1 {
		for i := range pool {
			work(i)
		}
		return results
	}

	sem := make(chan struct{}, o.MaxWorkers)
	var wg sync.WaitGroup
	for i := range pool {
		i := i
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			work(i)
		}()
	}
	wg.Wait()
	return results
}

// httpClient returns a Client tuned for health probes — short
// connection setup, no keep-alives that would cross goroutines.
func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		// Allow redirects; check_mirrors.py uses allow_redirects=True.
	}
}

func checkOne(ctx context.Context, m mirrors.Mirror, o Options) Result {
	acceptable := m.Verify.AcceptableStatuses()
	checkURL := m.Verify.ProbeURL(m.URL)
	method := strings.ToLower(m.Verify.Method)
	if method == "" {
		method = "get"
	}
	inconclusive := map[string]struct{}{}
	for _, code := range m.Verify.InconclusiveStatuses {
		inconclusive[fmt.Sprintf("http_%d", code)] = struct{}{}
	}

	res := Result{
		ID:         m.ID,
		Name:       m.Name,
		Category:   m.Category,
		URL:        m.URL,
		Status:     "unknown",
		CheckedURL: checkURL,
		VerifyType: m.Verify.Type,
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
	}

	client := httpClient(o.Timeout)

	for attempt := 0; attempt <= o.Retries; attempt++ {
		// Reset attempt-scoped fields so stale data from a failed earlier
		// attempt doesn't leak into a later success.
		res.ErrorMessage = ""
		res.HTTPStatus = nil
		res.ResponseTimeMS = nil
		res.IsInconclusive = false
		shouldRetry := false

		req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), checkURL, nil)
		if err != nil {
			res.Status = "error"
			res.ErrorMessage = err.Error()
			break
		}
		req.Header.Set("User-Agent", o.UserAgent)

		start := time.Now()
		resp, err := client.Do(req)
		elapsedMS := float64(time.Since(start).Microseconds()) / 1000.0

		if err != nil {
			res.Status, res.ErrorMessage = classifyTransportError(err, o.Timeout)
			shouldRetry = res.Status == "timeout" || strings.HasSuffix(res.Status, "_error")
		} else {
			rt := roundTo(elapsedMS, 2)
			res.ResponseTimeMS = &rt
			code := resp.StatusCode
			res.HTTPStatus = &code
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			resp.Body.Close()

			switch {
			case containsInt(acceptable, code):
				if m.Verify.Type == "go_proxy" && code == 200 {
					if validGoProxyJSON(body) {
						res.Status = "ok"
					} else {
						res.Status = "unexpected_response"
						res.ErrorMessage = "go_proxy @v/*.info JSON is missing Version/Time fields"
					}
				} else {
					res.Status = "ok"
				}
			case code >= 500 && code < 600:
				res.Status = "http_5xx"
				res.ErrorMessage = fmt.Sprintf("Server error: %d", code)
				shouldRetry = true
			default:
				res.Status = fmt.Sprintf("http_%d", code)
				_, res.IsInconclusive = inconclusive[res.Status]
				if len(acceptable) == 1 {
					res.ErrorMessage = fmt.Sprintf("Expected %d, got %d", acceptable[0], code)
				} else {
					res.ErrorMessage = fmt.Sprintf("Expected one of %v, got %d", acceptable, code)
				}
			}
		}

		if !shouldRetry || attempt >= o.Retries {
			break
		}
		time.Sleep(time.Duration(800*(attempt+1)) * time.Millisecond)
	}

	return res
}

func classifyTransportError(err error, timeout time.Duration) (status, msg string) {
	low := strings.ToLower(err.Error())
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", fmt.Sprintf("Request timed out after %s", timeout)
	}
	switch {
	case strings.Contains(low, "no such host"),
		strings.Contains(low, "name or service not known"),
		strings.Contains(low, "getaddrinfo"),
		strings.Contains(low, "failed to resolve"):
		return "dns_error", "DNS resolution failed: " + err.Error()
	case strings.Contains(low, "tls"),
		strings.Contains(low, "x509"),
		strings.Contains(low, "certificate"),
		strings.Contains(low, "handshake"):
		return "tls_error", "TLS/SSL error: " + err.Error()
	case strings.Contains(low, "connection refused"),
		strings.Contains(low, "connection reset"),
		strings.Contains(low, "network is unreachable"),
		strings.Contains(low, "no route to host"),
		strings.Contains(low, "broken pipe"),
		strings.Contains(low, "i/o timeout"):
		return "connection_error", err.Error()
	}
	return "connection_error", err.Error()
}

func validGoProxyJSON(body []byte) bool {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}
	_, hasVersion := data["Version"]
	_, hasTime := data["Time"]
	return hasVersion && hasTime
}

// --- report assembly (mirrors generate_report in check_mirrors.py) ---

func buildReport(results []Result, store *mirrors.Store) *Report {
	rankings := buildCategoryRankings(results, store)
	criticalCategories := []string{}
	for cat, r := range rankings {
		if r.Critical {
			criticalCategories = append(criticalCategories, cat)
		}
	}
	sort.Strings(criticalCategories)

	stats := map[string]CategoryStat{}
	breakdown := map[string]int{}
	okCount := 0
	for _, r := range results {
		s := stats[r.Category]
		s.Total++
		if r.Status == "ok" {
			s.OK++
			okCount++
		} else {
			s.NonOK++
			s.Failed++
		}
		stats[r.Category] = s
		breakdown[r.Status]++
	}

	total := len(results)
	healthRate := 0.0
	if total > 0 {
		healthRate = roundTo(float64(okCount)/float64(total)*100, 2)
	}

	return &Report{
		Metadata: Metadata{
			GeneratedAt:       time.Now().UTC().Format(time.RFC3339Nano),
			Version:           store.File.Version,
			MirrorDataUpdated: store.File.UpdatedAt,
		},
		Summary: Summary{
			TotalMirrors:       total,
			Healthy:            okCount,
			Unhealthy:          total - okCount,
			HealthRate:         healthRate,
			CategoryStats:      stats,
			StatusBreakdown:    breakdown,
			CriticalCategories: criticalCategories,
		},
		Results:          results,
		CategoryRankings: rankings,
		Recommendations:  recommend(rankings),
	}
}

func buildCategoryRankings(results []Result, store *mirrors.Store) map[string]CategoryRanking {
	byCat := map[string][]Result{}
	for _, r := range results {
		byCat[r.Category] = append(byCat[r.Category], r)
	}

	out := map[string]CategoryRanking{}
	for cat, list := range byCat {
		sort.SliceStable(list, func(i, j int) bool {
			return sortKey(list[i], store) < sortKey(list[j], store)
		})
		ranked := make([]RankedResult, 0, len(list))
		for i, r := range list {
			meta, _ := store.Get(r.ID)
			prio := meta.Priority
			ranked = append(ranked, RankedResult{
				Result:   r,
				Rank:     i + 1,
				Priority: &prio,
			})
		}
		var rec *RankedResult
		for i := range ranked {
			if ranked[i].Status == "ok" {
				rec = &ranked[i]
				break
			}
		}
		critical := rec == nil && containsDefinitiveFailure(ranked)
		recID := ""
		if rec != nil {
			recID = rec.ID
		}
		out[cat] = CategoryRanking{
			RecommendedMirror:   rec,
			RecommendedMirrorID: recID,
			RankedResults:       ranked,
			AllFailed:           rec == nil,
			Critical:            critical,
		}
	}
	return out
}

func containsDefinitiveFailure(rs []RankedResult) bool {
	for _, r := range rs {
		if !r.IsInconclusive {
			return true
		}
	}
	return false
}

// sortKey is a string-comparable form of the (status_rank, latency,
// priority, name) tuple used by check_mirrors.py.
func sortKey(r Result, store *mirrors.Store) string {
	statusRank := 2
	if r.Status == "ok" {
		statusRank = 0
	} else if r.IsInconclusive {
		statusRank = 1
	}
	lat := 9_999_999.0
	if r.ResponseTimeMS != nil {
		lat = *r.ResponseTimeMS
	}
	prio := 99
	if m, ok := store.Get(r.ID); ok {
		prio = m.Priority
	}
	return fmt.Sprintf("%d|%012.2f|%03d|%s", statusRank, lat, prio, strings.ToLower(r.Name))
}

func recommend(rankings map[string]CategoryRanking) []Recommendation {
	out := []Recommendation{}
	keys := make([]string, 0, len(rankings))
	for k := range rankings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, cat := range keys {
		r := rankings[cat]
		working := []RankedResult{}
		failed := []RankedResult{}
		for _, item := range r.RankedResults {
			if item.Status == "ok" {
				working = append(working, item)
			} else {
				failed = append(failed, item)
			}
		}
		definitive := []RankedResult{}
		inconclusive := []RankedResult{}
		for _, item := range failed {
			if item.IsInconclusive {
				inconclusive = append(inconclusive, item)
			} else {
				definitive = append(definitive, item)
			}
		}

		switch {
		case len(working) == 0 && len(definitive) > 0:
			out = append(out, Recommendation{
				Category: cat,
				Severity: "critical",
				Message:  fmt.Sprintf("No working mirrors found for %s. All mirrors failed.", cat),
				Action:   "Please check your network connection or report this issue.",
			})
		case len(working) == 0 && len(inconclusive) > 0:
			out = append(out, Recommendation{
				Category: cat,
				Severity: "warning",
				Message:  fmt.Sprintf("No mirror probe returned a definitive success for %s; current failures are inconclusive.", cat),
				Action:   "Check the official help page or validate manually from a real client.",
			})
		case len(failed) > 0:
			best := working[0]
			out = append(out, Recommendation{
				Category: cat,
				Severity: "warning",
				Message:  fmt.Sprintf("%d mirror(s) failed for %s.", len(failed), cat),
				Action:   fmt.Sprintf("Recommended mirror: %s (%s)", best.Name, best.URL),
			})
		}
	}
	return out
}

// --- helpers ---

func containsInt(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func roundTo(v float64, places int) float64 {
	pow := 1.0
	for i := 0; i < places; i++ {
		pow *= 10
	}
	return float64(int64(v*pow+0.5)) / pow
}

// CountCriticalFailures matches check_mirrors.py:count_critical_category_failures,
// used by `cmc health` to decide its exit code in CI.
func CountCriticalFailures(rep *Report) int {
	return len(rep.Summary.CriticalCategories)
}
