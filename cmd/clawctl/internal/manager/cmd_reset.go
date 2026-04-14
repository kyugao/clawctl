package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
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

			be := backend.MustGet(inst.GetClawType())
			if err := be.ResetWorkspace(inst); err != nil {
				if err == backend.ErrNotSupported {
					return fmt.Errorf("reset is not supported for %s type", inst.GetClawType())
				}
				return fmt.Errorf("reset failed: %w", err)
			}
			fmt.Printf("Workspace templates reset for instance %q\n", name)
			return nil
		},
	}
	return cmd
}
