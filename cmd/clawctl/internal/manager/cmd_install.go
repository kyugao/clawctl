package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/agent"
)

func NewInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <claw_type> <version>",
		Short: "Install a Claw version (e.g. picoclaw v0.2.6, nightly, latest)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clawType := args[0]
			version := args[1]

			spec, err := agent.Get(clawType)
			if err != nil {
				return err
			}

			fmt.Printf("Installing %s %s from %s...\n", clawType, version, spec.Repo)

			if err := InstallVersion(clawType, version); err != nil {
				return fmt.Errorf("install failed: %w", err)
			}
			return nil
		},
	}
	return cmd
}
