package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/adapter"
)

// newAdapterCmd builds a cobra sub-command tree for one adapter, e.g.
//
//	cmc python setup [flags]
//	cmc python install [flags]
//
// Two universal flags are exposed at the sub-command level:
//
//	--tool/-t   scope multi-tool adapters (python: pip|uv|poetry, ...)
//	--extra/-x  pass adapter-specific options as key=value pairs (repeatable),
//	            e.g. `cmc github setup -x release-url=https://github.com/...`
func newAdapterCmd(a adapter.Adapter, g *GlobalFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:   a.Name(),
		Short: a.Description(),
	}
	for _, c := range a.Commands() {
		c := c
		var (
			toolFlag string
			extras   []string
		)
		sub := &cobra.Command{
			Use:   c.Name,
			Short: c.Description,
			RunE: func(cmd *cobra.Command, args []string) error {
				opts := g.toOptions()
				if toolFlag != "" {
					opts.Extra["tool"] = toolFlag
				}
				for _, kv := range extras {
					k, v, ok := strings.Cut(kv, "=")
					if !ok {
						return fmt.Errorf("--extra expects key=value, got %q", kv)
					}
					opts.Extra[k] = v
				}
				if err := a.Run(c.Name, opts); err != nil {
					return fmt.Errorf("%s %s: %w", a.Name(), c.Name, err)
				}
				return nil
			},
		}
		sub.Flags().StringVarP(&toolFlag, "tool", "t", "", "scope to a single tool (e.g. pip, npm) — default: every installed tool in this adapter")
		sub.Flags().StringArrayVarP(&extras, "extra", "x", nil, "adapter-specific option as key=value (repeatable)")
		parent.AddCommand(sub)
	}
	return parent
}
