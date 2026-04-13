package manager

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
	"github.com/sipeed/picoclaw/pkg/pid"
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
				status := "stopped"
				if pidData := pid.ReadPidFileWithCheck(inst.WorkDir); pidData != nil {
					status = fmt.Sprintf("running (PID %d)", pidData.PID)
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
