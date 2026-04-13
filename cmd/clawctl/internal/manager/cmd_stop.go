package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/agent"
	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
)

func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <instance>",
		Short: "Stop an instance",
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
			if _, err := agent.Get(inst.ClawType); err != nil {
				return err
			}

			fmt.Printf("Stopping %s...\n", name)
			if err := Stop(inst); err != nil {
				return fmt.Errorf("stop failed: %w", err)
			}
			fmt.Printf("Stopped %s\n", name)
			return nil
		},
	}
	return cmd
}
