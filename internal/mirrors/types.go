package mirrors

type Status string

const (
	StatusActive     Status = "active"
	StatusTesting    Status = "testing"
	StatusDeprecated Status = "deprecated"
	StatusCommunity  Status = "community"
)

// Verify describes how a mirror's reachability is probed. The schema
// matches scripts/check_mirrors.py so the resulting JSON report stays
// drop-in compatible with reports/report.json consumers.
type Verify struct {
	Type                 string `yaml:"type"                            json:"type"`
	URL                  string `yaml:"url,omitempty"                   json:"url,omitempty"`
	Method               string `yaml:"method,omitempty"                json:"method,omitempty"`
	ExpectStatus         int    `yaml:"expect_status,omitempty"         json:"expect_status,omitempty"`
	ExpectStatusIn       []int  `yaml:"expect_status_in,omitempty"      json:"expect_status_in,omitempty"`
	InconclusiveStatuses []int  `yaml:"inconclusive_statuses,omitempty" json:"inconclusive_statuses,omitempty"`
	Prefix               string `yaml:"prefix,omitempty"                json:"prefix,omitempty"`
	TestURL              string `yaml:"test_url,omitempty"              json:"test_url,omitempty"`
}

// AcceptableStatuses returns the HTTP codes that count as "ok".
func (v Verify) AcceptableStatuses() []int {
	if len(v.ExpectStatusIn) > 0 {
		return v.ExpectStatusIn
	}
	if v.ExpectStatus != 0 {
		return []int{v.ExpectStatus}
	}
	return []int{200}
}

// ProbeURL returns the URL that should actually be fetched, falling back
// to the mirror's main URL when no explicit verify URL is configured.
func (v Verify) ProbeURL(fallback string) string {
	if v.Type == "github_proxy" {
		prefix := v.Prefix
		if prefix == "" {
			prefix = fallback
		}
		return prefix + v.TestURL
	}
	if v.URL != "" {
		return v.URL
	}
	return fallback
}

type Mirror struct {
	ID           string `yaml:"id"                       json:"id"`
	Name         string `yaml:"name"                     json:"name"`
	Category     string `yaml:"category"                 json:"category"`
	URL          string `yaml:"url"                      json:"url"`
	Homepage     string `yaml:"homepage,omitempty"       json:"homepage,omitempty"`
	Status       Status `yaml:"status"                   json:"status"`
	Priority     int    `yaml:"priority"                 json:"priority"`
	OfficialHelp string `yaml:"official_help,omitempty"  json:"official_help,omitempty"`
	Verify       Verify `yaml:"verify"                   json:"verify"`
	Notes        string `yaml:"notes,omitempty"          json:"notes,omitempty"`
	Source       string `yaml:"-"                        json:"source,omitempty"`
}

type Category struct {
	Name             string `yaml:"name"`
	Description      string `yaml:"description"`
	Icon             string `yaml:"icon,omitempty"`
	MirrorType       string `yaml:"mirror_type,omitempty"`
	QuickUseTemplate string `yaml:"quick_use_template,omitempty"`
}

type HealthCheck struct {
	Timeout    int    `yaml:"timeout"`
	Retries    int    `yaml:"retries"`
	Parallel   bool   `yaml:"parallel"`
	MaxWorkers int    `yaml:"max_workers"`
	UserAgent  string `yaml:"user_agent"`
}

type File struct {
	Version     string              `yaml:"version"`
	UpdatedAt   string              `yaml:"updated_at"`
	Mirrors     []Mirror            `yaml:"mirrors"`
	Categories  map[string]Category `yaml:"categories"`
	HealthCheck HealthCheck         `yaml:"health_check"`
}
