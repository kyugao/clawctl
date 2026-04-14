package manager

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func NewRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart <instance>",
		Short: "Restart an instance",
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
			if _, err := backend.Get(inst.GetClawType()); err != nil {
				return err
			}

			// Stop first (ignore error if not running).
			_ = Stop(inst)

			inst, err = ReconcileInstanceForStart(context.Background(), cfg, inst)
			if err != nil {
				return fmt.Errorf("reconcile instance: %w", err)
			}

			runner, err := NewGatewayRunner(inst)
			if err != nil {
				return fmt.Errorf("prepare runner: %w", err)
			}

			fmt.Printf("Restarting %s...\n", name)
			if err := runner.Start(); err != nil {
				return fmt.Errorf("start failed: %w", err)
			}
			fmt.Printf("Restarted %s\n", name)
			return nil
		},
	}
	return cmd
}
