package cli

import (
	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/adapter"
)

// Version is overridden at build time via -ldflags.
var Version = "0.1.0-dev"

// GlobalFlags holds flags shared by every adapter sub-command.
type GlobalFlags struct {
	Mirror  string
	DryRun  bool
	Yes     bool
	Force   bool
	Verbose bool
	Format  string
}

func (g *GlobalFlags) toOptions() adapter.Options {
	return adapter.Options{
		Mirror:  g.Mirror,
		DryRun:  g.DryRun,
		Yes:     g.Yes,
		Force:   g.Force,
		Verbose: g.Verbose,
		Format:  g.Format,
		Extra:   map[string]string{},
	}
}

func NewRoot() *cobra.Command {
	g := &GlobalFlags{}

	root := &cobra.Command{
		Use:   "cmc",
		Short: "China Mirror CLI — configure dev tools to use China mirrors",
		Long: `cmc is a single-binary CLI that points pip, npm, docker, apt, conda,
rust, go, flutter, github (and more) at mirrors hosted in China.

It is designed to be driven equally well by humans (typed in a terminal,
OpenCLI-style) and by AI agents (Claude / Cursor / Codex calling it
through their shell). Mirror metadata lives in mirrors.yml and is read
at runtime — adding a tool is one adapter, not a new bash script.

Outputs are table/json/yaml/md/csv so the same command works in pipes,
in dashboards, and in chat replies.`,
		SilenceUsage: true,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&g.Mirror, "mirror", "m", "", "mirror id (default: highest-priority active for the category)")
	pf.BoolVarP(&g.DryRun, "dry-run", "d", false, "preview changes without writing")
	pf.BoolVarP(&g.Yes, "yes", "y", false, "skip confirmation prompts")
	pf.BoolVarP(&g.Force, "force", "f", false, "overwrite existing configuration")
	pf.BoolVarP(&g.Verbose, "verbose", "v", false, "verbose logging")
	pf.StringVar(&g.Format, "format", "table", "output format: table|json|yaml|md|csv")

	root.AddCommand(
		newVersionCmd(),
		newListCmd(g),
		newDoctorCmd(g),
		newHealthCmd(g),
		newBackupCmd(g),
		newRestoreCmd(g),
		newPluginCmd(),
	)

	// Register every built-in adapter as a child command, e.g. `cmc python setup`.
	for _, a := range adapter.All() {
		root.AddCommand(newAdapterCmd(a, g))
	}

	return root
}
