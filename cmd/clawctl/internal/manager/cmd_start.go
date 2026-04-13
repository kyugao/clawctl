package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <instance>",
		Short: "Start an instance",
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
			if _, err := backend.Get(inst.ClawType); err != nil {
				return err
			}

			runner, err := NewGatewayRunner(inst)
			if err != nil {
				return fmt.Errorf("prepare runner: %w", err)
			}

			fmt.Printf("Starting %s (type=%s, version=%s)...\n", name, inst.ClawType, inst.Version)
			if err := runner.Start(); err != nil {
				return fmt.Errorf("start failed: %w", err)
			}

			// Gather backend-specific info after successful start
			if info := runner.Backend.GatherInfo(inst.WorkDir); len(info) > 0 {
				if err := config.UpdateInstanceInfo(name, info); err != nil {
					fmt.Printf("warning: failed to update instance info: %v\n", err)
				}
			}

			fmt.Printf("Started %s\n", name)
			return nil
		},
	}
	return cmd
}
