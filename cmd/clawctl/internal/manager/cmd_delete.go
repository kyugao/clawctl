package manager

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
)

func NewDeleteCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <instance>",
		Short: "Move an instance to trash (soft delete)",
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

			if !force {
				fmt.Printf("Move instance %q to trash? (work_dir=%s)\n", name, inst.WorkDir)
				fmt.Print("Type 'yes' to confirm: ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			// Move to trash.
			item, err := config.MoveToTrash(name, inst)
			if err != nil {
				return fmt.Errorf("move to trash: %w", err)
			}
			fmt.Printf("Moved to trash: %s\n", item.TrashPath)
			fmt.Printf("Run 'clawctl trash' to see all trashed items.\n")

			// Remove from config.
			delete(cfg.Instances, name)
			if cfg.Default == name {
				cfg.Default = ""
			}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	return cmd
}
