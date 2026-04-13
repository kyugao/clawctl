package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
	"github.com/sipeed/clawctl/cmd/clawctl/internal/onboard"
)

func NewResetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset <instance>",
		Short: "Reset instance workspace from templates (keeps config)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			inst, ok := cfg.Instances[name]
			if !ok {
				return fmt.Errorf("instance %q not found", name)
			}

			// Only picoclaw types support template reset for now.
			if inst.ClawType != "picoclaw" {
				return fmt.Errorf("reset is only supported for picoclaw type (got %q)", inst.ClawType)
			}

			if err := onboard.CopyWorkspaceTemplates(inst.WorkDir); err != nil {
				return fmt.Errorf("copy workspace templates: %w", err)
			}
			fmt.Printf("Workspace templates reset for instance %q\n", name)
			return nil
		},
	}
	return cmd
}
