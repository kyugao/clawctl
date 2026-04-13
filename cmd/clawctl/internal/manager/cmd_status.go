package manager

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/agent"
	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
)

func NewStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <instance>",
		Short: "Show instance status",
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
			spec, err := agent.Get(inst.ClawType)
			if err != nil {
				return err
			}

			running, pidData, err := Status(inst)
			if err != nil {
				return fmt.Errorf("status check failed: %w", err)
			}

			fmt.Printf("Instance: %s\n", name)
			fmt.Printf("  Type:    %s\n", inst.ClawType)
			fmt.Printf("  Version: %s\n", inst.Version)
			fmt.Printf("  Port:    %d\n", inst.Port)
			fmt.Printf("  WorkDir: %s\n", inst.WorkDir)
			fmt.Printf("  Binary:  %s\n", spec.GatewayBinary)

			if !running {
				fmt.Printf("  Status:  stopped\n")
			} else {
				fmt.Printf("  Status:  running (PID %d)\n", pidData.PID)
				if pidData.Port > 0 {
					fmt.Printf("  Gateway: %s:%d\n", pidData.Host, pidData.Port)
				}
				fmt.Printf("  Version: %s\n", pidData.Version)
			}

			// Show log file location if exists.
			logPath := fmt.Sprintf("%s/.gateway.log", inst.WorkDir)
			if _, err := os.Stat(logPath); err == nil {
				fmt.Printf("  Log:     %s\n", logPath)
			}

			return nil
		},
	}
	return cmd
}
