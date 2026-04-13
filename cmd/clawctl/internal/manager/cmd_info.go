package manager

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
)

func NewInfoCommand() *cobra.Command {
	var showAll bool
	cmd := &cobra.Command{
		Use:   "info [instance]",
		Short: "Show instance details",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(args) == 0 || showAll {
				if len(cfg.Instances) == 0 {
					fmt.Println("No instances found.")
					return nil
				}
				for name, inst := range cfg.Instances {
					printInstance(name, inst, name == cfg.Default)
				}
				return nil
			}

			name := args[0]
			inst, ok := cfg.Instances[name]
			if !ok {
				return fmt.Errorf("instance %q not found", name)
			}
			printInstance(name, inst, name == cfg.Default)
			return nil
		},
	}
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all instances (same as no argument)")
	return cmd
}

func printInstance(name string, inst config.Instance, isDefault bool) {
	fmt.Printf("Instance: %s%s\n", name, map[bool]string{true: " (default)"}[isDefault])
	fmt.Printf("  Type:    %s\n", inst.ClawType)
	fmt.Printf("  Version: %s\n", inst.Version)
	fmt.Printf("  Port:    %d\n", inst.Port)
	fmt.Printf("  WorkDir: %s\n", inst.WorkDir)
	fmt.Printf("  Created: %s\n", inst.CreatedAt)
	if _, err := os.Stat(inst.WorkDir); os.IsNotExist(err) {
		fmt.Printf("  Status:  work_dir missing\n")
	}
	fmt.Println()
}
