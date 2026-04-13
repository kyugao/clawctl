package manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/agent"
	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
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
			// Validate claw_type.
			spec, err := agent.Get(clawType)
			if err != nil {
				return err
			}
			inst := config.NewInstance(clawType, name, port, version, workDir)
			inst.Version = version
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

			// Create work_dir structure.
			if err := os.MkdirAll(inst.WorkDir, 0o755); err != nil {
				return fmt.Errorf("create work_dir: %w", err)
			}
			// Write agent_type file (plain text, one line).
			if err := os.WriteFile(filepath.Join(inst.WorkDir, "agent_type"), []byte(clawType+"\n"), 0o644); err != nil {
				return fmt.Errorf("write agent_type: %w", err)
			}
			// Create workspace/ directory.
			if err := os.MkdirAll(filepath.Join(inst.WorkDir, "workspace"), 0o755); err != nil {
				return fmt.Errorf("create workspace: %w", err)
			}
			// Create skills/ directory.
			if err := os.MkdirAll(filepath.Join(inst.WorkDir, "skills"), 0o755); err != nil {
				return fmt.Errorf("create skills: %w", err)
			}

			fmt.Printf("Instance %q created (type=%s, version=%s, port=%d, work_dir=%s)\n",
				name, clawType, inst.Version, inst.Port, inst.WorkDir)
			if spec.GatewayBinary != "" {
				fmt.Printf("  gateway binary: %s\n", spec.GatewayBinary)
			}
			fmt.Println("  Run 'clawctl reset %s' to initialize workspace templates.", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&clawType, "type", "", "Claw type (required)")
	cmd.Flags().IntVar(&port, "port", 0, "Gateway port (default varies by type)")
	cmd.Flags().StringVar(&version, "version", "latest", "Claw version")
	cmd.Flags().StringVar(&workDir, "dir", "", "Work directory")
	return cmd
}
