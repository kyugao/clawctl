package manager

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "NAME\tTYPE\tPORT\tVERSION\tSTATUS\tWORK_DIR\n")

			for name, inst := range cfg.Instances {
				be := backend.MustGet(inst.ClawType)
				_, running, _ := be.IsRunning(inst.WorkDir)
				status := "stopped"
				if running {
					pid, _, _ := be.IsRunning(inst.WorkDir)
					status = fmt.Sprintf("running (PID %d)", pid)
				}
				marker := "  "
				if name == cfg.Default {
					marker = "* "
				}
				fmt.Fprintf(w, "%s%s\t%s\t%d\t%s\t%s\t%s\n",
					marker, name, inst.ClawType, inst.Port, inst.Version, status, inst.WorkDir)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}
