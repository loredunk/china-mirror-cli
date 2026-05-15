package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/health"
	"github.com/loredunk/china-mirror/internal/mirrors"
	"github.com/loredunk/china-mirror/internal/output"
	"github.com/loredunk/china-mirror/internal/sysexits"
)

func newHealthCmd(g *GlobalFlags) *cobra.Command {
	var (
		category    string
		outputPath  string
		quiet       bool
		timeoutSec  int
		retries     int
		maxWorkers  int
	)
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Run HTTP health checks against every configured mirror",
		Long: `Probe every mirror defined in data/mirrors.yml (plus any plugin-provided
ones) and report which are reachable. The JSON output is drop-in
compatible with reports/report.json from the legacy check_mirrors.py,
so the existing CI workflow can swap directly to:

  cm health --format json --output reports/report.json

When --category is given, only mirrors in that category are probed.

Exit code is 1 when at least one category has zero working mirrors (the
"critical category" condition); 0 otherwise.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := mirrors.Load()
			if err != nil {
				return fmt.Errorf("load mirrors: %w", err)
			}

			opts := health.Options{
				Category:   category,
				Retries:    retries,
				MaxWorkers: maxWorkers,
			}
			if timeoutSec > 0 {
				opts.Timeout = time.Duration(timeoutSec) * time.Second
			}
			if !quiet {
				opts.Progress = func(r health.Result) {
					icon := "✗"
					if r.Status == "ok" {
						icon = "✓"
					}
					line := fmt.Sprintf("%s %s (%s): %s", icon, r.Name, r.Category, r.Status)
					if r.Status != "ok" && r.ErrorMessage != "" {
						line += " — " + truncateErr(r.ErrorMessage)
					}
					fmt.Fprintln(os.Stderr, line)
				}
			}

			rep := health.Run(cmd.Context(), store, opts)

			format := output.Parse(g.Format)
			out := os.Stdout
			if outputPath != "" {
				f, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("create %s: %w", outputPath, err)
				}
				defer f.Close()
				out = f
				// When writing to disk, default to JSON unless the user picked another format.
				if g.Format == "table" {
					format = output.JSON
				}
			}

			switch format {
			case output.JSON, output.YAML:
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
			default:
				renderHealthSummary(rep)
			}

			if health.CountCriticalFailures(rep) > 0 {
				os.Exit(sysexits.Unavailable)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "only probe mirrors in this category")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "write JSON report to this path instead of stdout")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress per-mirror progress lines on stderr")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 0, "per-request timeout in seconds (overrides mirrors.yml)")
	cmd.Flags().IntVar(&retries, "retries", 0, "retries on transient failure (overrides mirrors.yml)")
	cmd.Flags().IntVar(&maxWorkers, "workers", 0, "max concurrent workers (overrides mirrors.yml)")
	return cmd
}

func renderHealthSummary(r *health.Report) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("MIRROR HEALTH CHECK SUMMARY")
	fmt.Println("============================================================")
	fmt.Println("Generated:           ", r.Metadata.GeneratedAt)
	fmt.Println("Total mirrors:       ", r.Summary.TotalMirrors)
	fmt.Printf("Healthy:              %d (%.2f%%)\n", r.Summary.Healthy, r.Summary.HealthRate)
	fmt.Println("Unhealthy:           ", r.Summary.Unhealthy)

	fmt.Println("\nCategory statistics:")
	for cat, s := range r.Summary.CategoryStats {
		icon := "✓"
		if s.NonOK > 0 {
			icon = "⚠"
		}
		fmt.Printf("  %s %-18s %d/%d working\n", icon, cat, s.OK, s.Total)
	}
	if len(r.Recommendations) > 0 {
		fmt.Println("\nRecommendations:")
		for _, rec := range r.Recommendations {
			icon := "🟡"
			if rec.Severity == "critical" {
				icon = "🔴"
			}
			fmt.Printf("  %s [%s] %s\n      Action: %s\n", icon, rec.Category, rec.Message, rec.Action)
		}
	}
	fmt.Println("============================================================")
}

func truncateErr(s string) string {
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}
