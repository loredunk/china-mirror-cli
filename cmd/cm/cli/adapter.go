package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/loredunk/china-mirror/internal/adapter"
)

// newAdapterCmd builds a cobra sub-command tree for one adapter, e.g.
//
//	cm python setup [flags]
//	cm python install [flags]
func newAdapterCmd(a adapter.Adapter, g *GlobalFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:   a.Name(),
		Short: a.Description(),
	}
	for _, c := range a.Commands() {
		c := c
		parent.AddCommand(&cobra.Command{
			Use:   c.Name,
			Short: c.Description,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := a.Run(c.Name, g.toOptions()); err != nil {
					return fmt.Errorf("%s %s: %w", a.Name(), c.Name, err)
				}
				return nil
			},
		})
	}
	return parent
}
