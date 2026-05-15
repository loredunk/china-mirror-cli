package adapter

// Options carries the global CLI flags down to each adapter command.
type Options struct {
	Mirror  string
	DryRun  bool
	Yes     bool
	Force   bool
	Verbose bool
	Format  string
	Extra   map[string]string
}

// Command describes a sub-action of an adapter (e.g. "setup", "install").
type Command struct {
	Name        string
	Description string
}

// Adapter is implemented by every tool integration (built-in or plugin).
type Adapter interface {
	Name() string
	Categories() []string
	Description() string
	Commands() []Command
	Run(cmd string, opts Options) error
	BackupTargets() []string
}
