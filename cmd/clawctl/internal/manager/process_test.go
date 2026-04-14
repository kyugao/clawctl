package manager

import (
	"context"
	"testing"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func TestReconcileInstanceForStartPersistsPicoclawGatewayPort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := t.TempDir()
	inst := config.NewInstance("picoclaw", "pico1", 18800, "latest", workDir)
	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"pico1": inst,
		},
		Default: "pico1",
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	reconciled, err := ReconcileInstanceForStart(context.Background(), cfg, inst)
	if err != nil {
		t.Fatalf("reconcile instance for start: %v", err)
	}

	gatewayPort, ok := config.GetInstanceInfoInt(reconciled, "ports", "gateway")
	if !ok || gatewayPort <= 0 {
		t.Fatalf("expected reconciled gateway port, got %d, ok=%v", gatewayPort, ok)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	persisted := loaded.Instances["pico1"]
	persistedGatewayPort, ok := config.GetInstanceInfoInt(persisted, "ports", "gateway")
	if !ok || persistedGatewayPort != gatewayPort {
		t.Fatalf("expected persisted gateway port %d, got %d, ok=%v", gatewayPort, persistedGatewayPort, ok)
	}
}
