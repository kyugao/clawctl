package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/agent"
)

func NewUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <claw_type> <version>",
		Short: "Uninstall a Claw version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clawType := args[0]
			version := args[1]

			if _, err := agent.Get(clawType); err != nil {
				return err
			}
			if err := UninstallVersion(clawType, version); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}
			return nil
		},
	}
	return cmd
}
