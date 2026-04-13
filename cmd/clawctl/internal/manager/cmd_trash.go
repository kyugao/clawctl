package manager

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
)

func NewTrashCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash",
		Short: "Manage trashed instances",
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(
		NewTrashListCommand(),
		NewTrashRestoreCommand(),
		NewTrashCleanCommand(),
		NewTrashPurgeCommand(),
	)
	return cmd
}

func NewTrashListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List trashed instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := config.LoadTrashMeta()
			if err != nil {
				return fmt.Errorf("load trash meta: %w", err)
			}
			if len(meta.Items) == 0 {
				fmt.Println("Trash is empty.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tINSTANCE\tDELETED\tORIGINAL_PATH\n")
			for _, item := range meta.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					item.ID, item.InstanceName, item.DeletedAt, item.Instance.WorkDir)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}

func NewTrashRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <trash-id>",
		Short: "Restore a trashed instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			path, err := config.RestoreFromTrash(id)
			if err != nil {
				return fmt.Errorf("restore: %w", err)
			}
			fmt.Printf("Restored to: %s\n", path)
			return nil
		},
	}
	return cmd
}

func NewTrashCleanCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "clean <trash-id>",
		Short: "Permanently delete a trashed instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !force {
				fmt.Printf("Permanently delete trash item %q? This cannot be undone.\n", id)
				fmt.Print("Type 'yes' to confirm: ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			if err := config.RemoveFromTrash(id); err != nil {
				return fmt.Errorf("clean: %w", err)
			}
			fmt.Printf("Permanently deleted: %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	return cmd
}

func NewTrashPurgeCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Empty the trash",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := config.LoadTrashMeta()
			if err != nil {
				return fmt.Errorf("load trash meta: %w", err)
			}
			if len(meta.Items) == 0 {
				fmt.Println("Trash is already empty.")
				return nil
			}
			if !force {
				fmt.Printf("Permanently delete %d trashed instance(s)? This cannot be undone.\n", len(meta.Items))
				fmt.Print("Type 'yes' to confirm: ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			for _, item := range meta.Items {
				if err := config.RemoveFromTrash(item.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", item.ID, err)
				}
			}
			fmt.Println("Trash emptied.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	return cmd
}
