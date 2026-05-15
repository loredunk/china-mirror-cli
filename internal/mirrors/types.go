package mirrors

type Status string

const (
	StatusActive     Status = "active"
	StatusTesting    Status = "testing"
	StatusDeprecated Status = "deprecated"
	StatusCommunity  Status = "community"
)

type Verify struct {
	Type         string `yaml:"type"           json:"type"`
	URL          string `yaml:"url"            json:"url"`
	ExpectStatus int    `yaml:"expect_status"  json:"expect_status"`
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
