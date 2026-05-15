package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/adapter"
)

// newAdapterCmd builds a cobra sub-command tree for one adapter, e.g.
//
//	cmc python setup [flags]
//	cmc python install [flags]
//
// A per-adapter --tool flag is exposed so users can scope multi-tool
// adapters: `cmc python setup --tool pip` only touches pip.
func newAdapterCmd(a adapter.Adapter, g *GlobalFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:   a.Name(),
		Short: a.Description(),
	}
	for _, c := range a.Commands() {
		c := c
		var toolFlag string
		sub := &cobra.Command{
			Use:   c.Name,
			Short: c.Description,
			RunE: func(cmd *cobra.Command, args []string) error {
				opts := g.toOptions()
				if toolFlag != "" {
					opts.Extra["tool"] = toolFlag
				}
				if err := a.Run(c.Name, opts); err != nil {
					return fmt.Errorf("%s %s: %w", a.Name(), c.Name, err)
				}
				return nil
			},
		}
		sub.Flags().StringVarP(&toolFlag, "tool", "t", "", "scope to a single tool (e.g. pip, npm) — default: every installed tool in this adapter")
		parent.AddCommand(sub)
	}
	return parent
}
