// Package doctor diagnoses the local environment: OS, proxy, installed
// developer tools, and current mirror configuration for each one. It is
// read-only — no file in the user's home is modified.
//
// Ported from china-mirror/skills/china-mirror/scripts/diagnose.sh.
package doctor

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Report is the structured output of a diagnostic run.
type Report struct {
	Generated       time.Time         `json:"generated_at"  yaml:"generated_at"`
	OS              OSInfo            `json:"os"            yaml:"os"`
	Proxy           ProxyInfo         `json:"proxy"         yaml:"proxy"`
	Tools           []ToolInfo        `json:"tools"         yaml:"tools"`
	Connectivity    []ProbeResult     `json:"connectivity"  yaml:"connectivity"`
	Recommendations []Recommendation  `json:"recommendations" yaml:"recommendations"`
}

type OSInfo struct {
	GOOS     string `json:"goos"      yaml:"goos"`
	GOARCH   string `json:"goarch"    yaml:"goarch"`
	Platform string `json:"platform"  yaml:"platform"` // "macos" | "linux" | "windows"
	Distro   string `json:"distro,omitempty" yaml:"distro,omitempty"`
	Shell    string `json:"shell"     yaml:"shell"`
}

type ProxyInfo struct {
	EnvVars   map[string]string `json:"env_vars"  yaml:"env_vars"`
	GitProxy  string            `json:"git_proxy,omitempty" yaml:"git_proxy,omitempty"`
}

type ToolInfo struct {
	Name              string `json:"name"        yaml:"name"`
	Installed         bool   `json:"installed"   yaml:"installed"`
	Version           string `json:"version,omitempty"      yaml:"version,omitempty"`
	MirrorConfig      string `json:"mirror_config,omitempty" yaml:"mirror_config,omitempty"`
	UsingChinaMirror  bool   `json:"using_china_mirror"    yaml:"using_china_mirror"`
}

type ProbeResult struct {
	Label    string  `json:"label"    yaml:"label"`
	URL      string  `json:"url"      yaml:"url"`
	OK       bool    `json:"ok"       yaml:"ok"`
	HTTP     int     `json:"http,omitempty"       yaml:"http,omitempty"`
	LatencyS float64 `json:"latency_seconds,omitempty"  yaml:"latency_seconds,omitempty"`
	Error    string  `json:"error,omitempty"      yaml:"error,omitempty"`
}

type Severity string

const (
	SevOK       Severity = "ok"
	SevInfo     Severity = "info"
	SevMedium   Severity = "medium"
	SevHigh     Severity = "high"
)

type Recommendation struct {
	Severity Severity `json:"severity" yaml:"severity"`
	Tool     string   `json:"tool,omitempty" yaml:"tool,omitempty"`
	Message  string   `json:"message"  yaml:"message"`
}

// knownTools is the same list as diagnose.sh, kept in the same order so
// `cmc doctor` output remains visually consistent with the bash version
// during the migration window.
var knownTools = []string{
	"pip", "uv", "poetry",
	"npm", "yarn", "pnpm",
	"docker",
	"cargo",
	"go",
	"conda",
	"flutter",
	"brew",
}

// chinaMirrorMarkers — substrings whose presence in a tool's resolved
// registry/index URL implies a China mirror is already configured.
var chinaMirrorMarkers = []string{
	"tuna", "ustc", "aliyun", "npmmirror", "goproxy.cn",
	"huaweicloud", "tencent", "flutter-io.cn", "mirror.sjtu.edu.cn",
	"npmmirror.com", "rsproxy.cn",
}

// Run executes every diagnostic step and returns a populated Report.
func Run(ctx context.Context) *Report {
	r := &Report{
		Generated: time.Now().UTC(),
		OS:        detectOS(),
		Proxy:     detectProxy(),
	}
	r.Tools = detectTools()
	r.Connectivity = probeConnectivity(ctx)
	r.Recommendations = recommend(r)
	return r
}

func detectOS() OSInfo {
	info := OSInfo{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
		Shell:  os.Getenv("SHELL"),
	}
	switch runtime.GOOS {
	case "darwin":
		info.Platform = "macos"
	case "linux":
		info.Platform = "linux"
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "ID=") {
					info.Distro = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
					break
				}
			}
		}
	case "windows":
		info.Platform = "windows"
	default:
		info.Platform = runtime.GOOS
	}
	return info
}

func detectProxy() ProxyInfo {
	p := ProxyInfo{EnvVars: map[string]string{}}
	for _, v := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "all_proxy"} {
		if val := os.Getenv(v); val != "" {
			p.EnvVars[v] = val
		}
	}
	if out, err := runCommand("git", "config", "--global", "http.proxy"); err == nil && out != "" {
		p.GitProxy = out
	}
	return p
}

func detectTools() []ToolInfo {
	out := make([]ToolInfo, 0, len(knownTools))
	for _, name := range knownTools {
		t := ToolInfo{Name: name}
		path, err := exec.LookPath(name)
		if err != nil || path == "" {
			out = append(out, t)
			continue
		}
		t.Installed = true
		t.Version = firstLine(safeRun(name, "--version"))
		t.MirrorConfig = readMirrorConfig(name)
		t.UsingChinaMirror = looksLikeChinaMirror(t.MirrorConfig)
		out = append(out, t)
	}
	return out
}

// readMirrorConfig replicates the per-tool inspection from diagnose.sh.
// It returns a short string suitable for display; absence is "".
func readMirrorConfig(tool string) string {
	switch tool {
	case "pip":
		return firstLine(safeRun("pip", "config", "get", "global.index-url"))
	case "uv":
		if home, err := os.UserHomeDir(); err == nil {
			if data, err := os.ReadFile(home + "/.config/uv/uv.toml"); err == nil {
				return grep(string(data), "index-url")
			}
		}
		return ""
	case "npm":
		return firstLine(safeRun("npm", "config", "get", "registry"))
	case "yarn":
		return firstLine(safeRun("yarn", "config", "get", "registry"))
	case "pnpm":
		return firstLine(safeRun("pnpm", "config", "get", "registry"))
	case "docker":
		if data, err := os.ReadFile("/etc/docker/daemon.json"); err == nil {
			return grep(string(data), "registry-mirrors")
		}
		return ""
	case "cargo":
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		for _, p := range []string{home + "/.cargo/config.toml", home + "/.cargo/config"} {
			if data, err := os.ReadFile(p); err == nil {
				return grep(string(data), "[source")
			}
		}
		return ""
	case "go":
		return firstLine(safeRun("go", "env", "GOPROXY"))
	case "conda":
		return firstLine(safeRun("conda", "config", "--show", "default_channels"))
	case "flutter":
		return "FLUTTER_STORAGE_BASE_URL=" + os.Getenv("FLUTTER_STORAGE_BASE_URL") +
			"; PUB_HOSTED_URL=" + os.Getenv("PUB_HOSTED_URL")
	case "brew":
		return "HOMEBREW_API_DOMAIN=" + os.Getenv("HOMEBREW_API_DOMAIN") +
			"; HOMEBREW_BREW_GIT_REMOTE=" + os.Getenv("HOMEBREW_BREW_GIT_REMOTE")
	}
	return ""
}

func looksLikeChinaMirror(cfg string) bool {
	if cfg == "" {
		return false
	}
	low := strings.ToLower(cfg)
	for _, m := range chinaMirrorMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// probeConnectivity contacts a small fixed set of well-known endpoints
// (same as diagnose.sh) to give the user a feel for whether they need
// mirrors at all. Concurrent for speed.
func probeConnectivity(ctx context.Context) []ProbeResult {
	probes := []struct{ label, url string }{
		{"pypi.org", "https://pypi.org/simple/pip/"},
		{"registry.npmjs.org", "https://registry.npmjs.org/npm"},
		{"TUNA PyPI", "https://pypi.tuna.tsinghua.edu.cn/simple/pip/"},
		{"npmmirror", "https://registry.npmmirror.com/npm"},
		{"goproxy.cn", "https://goproxy.cn/"},
		{"USTC", "https://mirrors.ustc.edu.cn/"},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	results := make([]ProbeResult, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		i, p := i, p
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := ProbeResult{Label: p.label, URL: p.url}
			start := time.Now()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
			req.Header.Set("User-Agent", "china-mirror/cm doctor")
			resp, err := client.Do(req)
			r.LatencyS = time.Since(start).Seconds()
			if err != nil {
				r.Error = err.Error()
				results[i] = r
				return
			}
			defer resp.Body.Close()
			r.HTTP = resp.StatusCode
			r.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
			results[i] = r
		}()
	}
	wg.Wait()
	return results
}

func recommend(r *Report) []Recommendation {
	var recs []Recommendation
	if len(r.Proxy.EnvVars) > 0 {
		recs = append(recs, Recommendation{
			Severity: SevMedium,
			Message:  "Proxy environment variables detected — using a mirror together with a proxy often causes conflicts. Consider `unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY` before configuring mirrors.",
		})
	}
	for _, t := range r.Tools {
		if !t.Installed {
			continue
		}
		if t.UsingChinaMirror {
			recs = append(recs, Recommendation{Severity: SevOK, Tool: t.Name, Message: t.Name + " is already using a China mirror."})
		} else {
			recs = append(recs, Recommendation{
				Severity: SevHigh,
				Tool:     t.Name,
				Message:  t.Name + " is NOT using a China mirror — run `cmc " + recommendCommand(t.Name) + "` to configure it.",
			})
		}
	}
	sort.SliceStable(recs, func(i, j int) bool {
		// high > medium > info > ok, then tool name
		sevRank := map[Severity]int{SevHigh: 0, SevMedium: 1, SevInfo: 2, SevOK: 3}
		if sevRank[recs[i].Severity] != sevRank[recs[j].Severity] {
			return sevRank[recs[i].Severity] < sevRank[recs[j].Severity]
		}
		return recs[i].Tool < recs[j].Tool
	})
	return recs
}

// --- small helpers ---

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func safeRun(name string, args ...string) string {
	out, err := runCommand(name, args...)
	if err != nil {
		return ""
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// recommendCommand maps a detected tool name to the cmc sub-command
// that configures it. Adapters group multiple tools (pip/uv/poetry →
// `cmc python setup`), so the mapping is explicit.
func recommendCommand(tool string) string {
	switch tool {
	case "pip", "uv", "poetry":
		return "python setup --tool " + tool
	case "npm", "yarn", "pnpm":
		return "node setup --tool " + tool
	case "brew":
		return "homebrew setup"
	default:
		return tool + " setup"
	}
}

func grep(content, needle string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
