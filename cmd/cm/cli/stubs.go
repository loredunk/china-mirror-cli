package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These commands are placeholders. Each is replaced by a real implementation
// in its own package as the rollout proceeds (config, plugin).

func newBackupCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "backup [tool]",
		Short: "Back up configuration files for one or all tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("backup: not implemented yet — see internal/config")
		},
	}
}

func newRestoreCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore [tool]",
		Short: "Restore configuration from the most recent (or specified) backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("restore: not implemented yet — see internal/config")
		},
	}
	cmd.Flags().Bool("latest", false, "restore the latest backup")
	cmd.Flags().Bool("list", false, "list available backups")
	return cmd
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
