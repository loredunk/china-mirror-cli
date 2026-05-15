package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/output"
)

func newBackupCmd(g *GlobalFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "backup [tool]",
		Short: "Back up configuration files for one or all tools",
		Long: `Back up the on-disk config files known to a given tool to
~/.china-mirror-backup/<tool>/<timestamp>/. Compatible with the bash
backup_config.sh format so legacy backups can be restored by cm too.

Examples:
  cmc backup            # list known tools (use --all to back up all)
  cmc backup pip        # back up pip configs only
  cmc backup --all      # back up every known tool's configs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tools := []string{}
			switch {
			case all:
				tools = config.KnownTools()
				sort.Strings(tools)
			case len(args) > 0:
				tools = args
			default:
				fmt.Println("Known tools:")
				known := config.KnownTools()
				sort.Strings(known)
				for _, t := range known {
					fmt.Println("  - " + t)
				}
				fmt.Println("\nRun `cmc backup <tool>` or `cmc backup --all`.")
				return nil
			}

			for _, t := range tools {
				existing := config.ExistingConfigs(t)
				if len(existing) == 0 {
					fmt.Printf("  %s: no config files found, skipping\n", t)
					continue
				}
				for _, p := range existing {
					dir, err := config.BackupFile(p, t)
					if err != nil {
						return fmt.Errorf("backup %s/%s: %w", t, p, err)
					}
					fmt.Printf("  ✓ %s → %s\n", p, dir)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "back up configs for every known tool")
	return cmd
}

func newRestoreCmd(g *GlobalFlags) *cobra.Command {
	var (
		list      bool
		latest    bool
		backupID  string
	)
	cmd := &cobra.Command{
		Use:   "restore [tool]",
		Short: "Restore configuration from a previous backup",
		Long: `Restore tool configs that were backed up by cmc (or by the legacy
bash backup_config.sh — the on-disk formats are identical).

Examples:
  cmc restore --list          # list every backup, newest first
  cmc restore pip --latest    # restore the newest pip backup
  cmc restore pip --id 20260515_071245
                              # restore a specific snapshot`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return runRestoreList(g, args)
			}
			if len(args) < 1 {
				return fmt.Errorf("restore requires a <tool> argument (or use --list)")
			}
			tool := args[0]

			if backupID != "" {
				p, err := config.RestoreByID(tool, backupID)
				if err != nil {
					return err
				}
				fmt.Printf("✓ restored %s\n", p)
				return nil
			}
			if !latest {
				return fmt.Errorf("specify --latest or --id <backup_id>")
			}
			p, err := config.RestoreLatest(tool)
			if err != nil {
				return err
			}
			fmt.Printf("✓ restored %s\n", p)
			return nil
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "list available backups")
	cmd.Flags().BoolVar(&latest, "latest", false, "restore the latest backup for the given tool")
	cmd.Flags().StringVar(&backupID, "id", "", "restore a specific backup by id (e.g. 20260515_071245)")
	return cmd
}

func runRestoreList(g *GlobalFlags, args []string) error {
	scope := ""
	if len(args) > 0 {
		scope = args[0]
	}
	entries, err := config.List(scope)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No backups found in", config.BackupDir())
		return nil
	}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{e.Tool, e.BackupID, e.Time, e.OriginalPath})
	}
	t := output.Tabular{
		Headers: []string{"TOOL", "BACKUP_ID", "TIME", "ORIGINAL_PATH"},
		Rows:    rows,
		Value:   entries,
	}
	return output.Render(os.Stdout, output.Parse(g.Format), t)
}

func newPluginCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "plugin",
		Short: "Manage external adapter plugins (OpenCLI-style)",
	}
	root.AddCommand(&cobra.Command{
		Use:   "install <github:user/repo>",
		Short: "Install a plugin from a GitHub repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("plugin install: not implemented yet — see internal/plugin")
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("plugin list: not implemented yet")
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("plugin remove: not implemented yet")
		},
	})
	return root
}
