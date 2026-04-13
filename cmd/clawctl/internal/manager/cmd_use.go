package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func NewUseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <instance>",
		Short: "Set the default instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if _, ok := cfg.Instances[name]; !ok {
				return fmt.Errorf("instance %q not found", name)
			}
			cfg.Default = name
			return config.Save(cfg)
		},
	}
	return cmd
}
