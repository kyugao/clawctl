package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/manager"
)

func main() {
	if err := config.EnsureClawctlHome(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ~/.clawctl: %v\n", err)
		os.Exit(1)
	}

	root := &cobra.Command{Use: "clawctl"}

	root.AddCommand(
		manager.NewListCommand(),
		manager.NewInfoCommand(),
		manager.NewCreateCommand(),
		manager.NewDeleteCommand(),
		manager.NewResetCommand(),
		manager.NewUseCommand(),
		manager.NewStartCommand(),
		manager.NewStopCommand(),
		manager.NewRestartCommand(),
		manager.NewStatusCommand(),
		manager.NewVersionsCommand(),
		manager.NewInstallCommand(),
		manager.NewUninstallCommand(),
		manager.NewTrashCommand(),
		manager.NewLogsCommand(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func printErr(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
}

func getInstanceOrFail(name string) (string, config.Instance) {
	cfg, err := config.Load()
	if err != nil {
		printErr("load config: %v", err)
		os.Exit(1)
	}
	inst, ok := cfg.Instances[name]
	if !ok {
		printErr("instance %q not found (use 'clawctl list' to see all instances)", name)
		os.Exit(1)
	}
	// Validate claw_type is known.
	if _, err := backend.Get(inst.ClawType); err != nil {
		printErr("instance %q has unknown claw_type %q: %v", name, inst.ClawType, err)
		os.Exit(1)
	}
	return name, inst
}
