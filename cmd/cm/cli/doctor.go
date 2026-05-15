package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/loredunk/china-mirror/internal/doctor"
	"github.com/loredunk/china-mirror/internal/output"
)

func newDoctorCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the local network and tool environment",
		Long: `Inspect the local machine for the things that affect mirror configuration:

  - OS / distro / shell
  - HTTP(S) proxy environment variables and git http.proxy
  - Installed developer tools (pip, npm, docker, ...)
  - Whether each installed tool already points at a China mirror
  - Connectivity to a small set of well-known endpoints (official vs. China mirror)

This command is read-only — it never writes to your config files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := doctor.Run(cmd.Context())
			switch output.Parse(g.Format) {
			case output.JSON:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			case output.YAML:
				enc := yaml.NewEncoder(os.Stdout)
				enc.SetIndent(2)
				defer enc.Close()
				return enc.Encode(r)
			default:
				if err := r.RenderText(os.Stdout); err != nil {
					return fmt.Errorf("render: %w", err)
				}
				return nil
			}
		},
	}
}
