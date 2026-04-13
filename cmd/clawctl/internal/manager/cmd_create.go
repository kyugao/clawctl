package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func NewCreateCommand() *cobra.Command {
	var clawType string
	var port int
	var version string
	var workDir string

	cmd := &cobra.Command{
		Use:   "create <instance>",
		Short: "Create a new instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if clawType == "" {
				return fmt.Errorf("--type is required")
			}
			be, err := backend.Get(clawType)
			if err != nil {
				return err
			}
			inst := config.NewInstance(clawType, name, port, version, workDir)
			if inst.Version == "" {
				inst.Version = "latest"
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if _, ok := cfg.Instances[name]; ok {
				return fmt.Errorf("instance %q already exists", name)
			}
			cfg.Instances[name] = inst
			if cfg.Default == "" {
				cfg.Default = name
			}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			// Init work_dir via backend.
			if err := be.InitWorkDir(inst); err != nil {
				return fmt.Errorf("init work_dir: %w", err)
			}

			fmt.Printf("Instance %q created (type=%s, version=%s, port=%d, work_dir=%s)\n",
				name, clawType, inst.Version, inst.Port, inst.WorkDir)
			if be.GatewayBinary() != "" {
				fmt.Printf("  gateway binary: %s\n", be.GatewayBinary())
			}
			fmt.Printf("  Run 'clawctl reset %s' to initialize workspace templates.\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&clawType, "type", "", "Claw type (required)")
	cmd.Flags().IntVar(&port, "port", 0, "Gateway port (default varies by type)")
	cmd.Flags().StringVar(&version, "version", "latest", "Claw version")
	cmd.Flags().StringVar(&workDir, "dir", "", "Work directory")
	return cmd
}
