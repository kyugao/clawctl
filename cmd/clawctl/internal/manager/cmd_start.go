package manager

import (
	"context"
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
			if _, err := backend.Get(inst.GetClawType()); err != nil {
				return err
			}
			inst, err = ReconcileInstanceForStart(context.Background(), cfg, inst)
			if err != nil {
				return fmt.Errorf("reconcile instance: %w", err)
			}

			runner, err := NewGatewayRunner(inst)
			if err != nil {
				return fmt.Errorf("prepare runner: %w", err)
			}

			fmt.Printf("Starting %s (type=%s, version=%s)...\n", name, inst.GetClawType(), inst.GetVersion())
			if err := runner.Start(); err != nil {
				return fmt.Errorf("start failed: %w", err)
			}

			fmt.Printf("Started %s\n", name)
			return nil
		},
	}
	return cmd
}
