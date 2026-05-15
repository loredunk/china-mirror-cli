package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/mirrors"
	"github.com/loredunk/china-mirror/internal/output"
)

func newListCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List adapters or mirrors",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListTools(g)
		},
	}
	cmd.AddCommand(newListMirrorsCmd(g))
	cmd.AddCommand(&cobra.Command{
		Use:   "tools",
		Short: "List built-in and plugin adapters",
		RunE:  func(cmd *cobra.Command, args []string) error { return runListTools(g) },
	})
	return cmd
}

func runListTools(g *GlobalFlags) error {
	all := adapter.All()
	t := output.Tabular{
		Headers: []string{"NAME", "CATEGORIES", "COMMANDS", "DESCRIPTION"},
		Value:   adaptersForOutput(all),
	}
	for _, a := range all {
		t.Rows = append(t.Rows, []string{
			a.Name(),
			join(a.Categories()),
			joinCommands(a.Commands()),
			a.Description(),
		})
	}
	return output.Render(os.Stdout, output.Parse(g.Format), t)
}

func newListMirrorsCmd(g *GlobalFlags) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "mirrors",
		Short: "List configured mirror sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := mirrors.Load()
			if err != nil {
				return fmt.Errorf("load mirrors: %w", err)
			}
			var ms []mirrors.Mirror
			if category != "" {
				ms = store.ByCategory(category)
			} else {
				ms = store.All()
			}
			t := output.Tabular{
				Headers: []string{"ID", "CATEGORY", "STATUS", "PRIORITY", "URL", "NAME"},
				Value:   ms,
			}
			for _, m := range ms {
				t.Rows = append(t.Rows, []string{
					m.ID,
					m.Category,
					string(m.Status),
					fmt.Sprintf("%d", m.Priority),
					m.URL,
					m.Name,
				})
			}
			return output.Render(os.Stdout, output.Parse(g.Format), t)
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "filter by category (pip, npm, docker-hub, ...)")
	return cmd
}

// --- helpers ---

type adapterSummary struct {
	Name        string   `json:"name" yaml:"name"`
	Categories  []string `json:"categories" yaml:"categories"`
	Commands    []string `json:"commands" yaml:"commands"`
	Description string   `json:"description" yaml:"description"`
}

func adaptersForOutput(as []adapter.Adapter) []adapterSummary {
	out := make([]adapterSummary, 0, len(as))
	for _, a := range as {
		cmds := make([]string, 0, len(a.Commands()))
		for _, c := range a.Commands() {
			cmds = append(cmds, c.Name)
		}
		out = append(out, adapterSummary{
			Name:        a.Name(),
			Categories:  a.Categories(),
			Commands:    cmds,
			Description: a.Description(),
		})
	}
	return out
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func joinCommands(cs []adapter.Command) string {
	out := ""
	for i, c := range cs {
		if i > 0 {
			out += ","
		}
		out += c.Name
	}
	return out
}
