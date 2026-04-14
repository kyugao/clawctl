package backend

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func TestPicoclawAllocateInstanceAssignsLauncherAndGatewayPorts(t *testing.T) {
	b := &picoclawBackend{}
	cfg := &config.Config{Instances: map[string]config.Instance{}}

	inst, err := b.AllocateInstance(context.Background(), cfg, "pico1", 0, "latest", t.TempDir())
	if err != nil {
		t.Fatalf("allocate instance: %v", err)
	}

	if inst.GetPort() < 18800 {
		t.Fatalf("expected launcher port >= 18800, got %d", inst.GetPort())
	}
	if launcherPort, ok := config.GetInstanceInfoInt(inst, "ports", "launcher"); !ok || launcherPort != inst.GetPort() {
		t.Fatalf("expected launcher port in info to match instance port, got %d, ok=%v", launcherPort, ok)
	}
	gatewayPort, ok := config.GetInstanceInfoInt(inst, "ports", "gateway")
	if !ok {
		t.Fatalf("expected gateway port in instance info")
	}
	if gatewayPort == inst.GetPort() {
		t.Fatalf("expected gateway port to differ from launcher port, both were %d", gatewayPort)
	}
}

func TestPicoclawReconcileInstanceAddsGatewayPortAndWritesConfig(t *testing.T) {
	workDir := t.TempDir()
	inst := config.NewInstance("picoclaw", "pico1", 18800, "latest", workDir)

	b := &picoclawBackend{}
	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"pico1": inst,
		},
	}

	if err := os.MkdirAll(filepath.Join(workDir, "workspace"), 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "skills"), 0o755); err != nil {
		t.Fatalf("create skills: %v", err)
	}

	reconciled, changed, err := b.ReconcileInstance(context.Background(), cfg, inst)
	if err != nil {
		t.Fatalf("reconcile instance: %v", err)
	}
	if !changed {
		t.Fatalf("expected reconcile to report changes for legacy instance")
	}

	gatewayPort, ok := config.GetInstanceInfoInt(reconciled, "ports", "gateway")
	if !ok || gatewayPort <= 0 {
		t.Fatalf("expected reconciled gateway port, got %d, ok=%v", gatewayPort, ok)
	}

	raw, err := os.ReadFile(filepath.Join(workDir, "config.json"))
	if err != nil {
		t.Fatalf("read generated config.json: %v", err)
	}

	var cfgFile map[string]any
	if err := json.Unmarshal(raw, &cfgFile); err != nil {
		t.Fatalf("parse generated config.json: %v", err)
	}

	gateway, ok := cfgFile["gateway"].(map[string]any)
	if !ok {
		t.Fatalf("expected gateway section in config.json")
	}
	if port, ok := gateway["port"].(float64); !ok || int(port) != gatewayPort {
		t.Fatalf("expected config.json gateway.port=%d, got %v", gatewayPort, gateway["port"])
	}
}

func TestPicoclawGatherInfoParsesRuntimeToken(t *testing.T) {
	workDir := t.TempDir()
	logPath := filepath.Join(workDir, ".gateway.log")
	content := "something\nDashboard token (this run): abc123\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	info := (&picoclawBackend{}).GatherInfo(workDir)
	inst := config.NewInstanceFromRecord(config.InstanceRecord{
		Name:      "pico1",
		ClawType:  "picoclaw",
		WorkDir:   workDir,
		Port:      18800,
		Version:   "latest",
		CreatedAt: "2026-04-14T00:00:00Z",
		Info:      info,
	})

	if token, ok := config.GetInstanceInfoString(inst, "runtime", "dashboard_token"); !ok || token != "abc123" {
		t.Fatalf("expected parsed dashboard token abc123, got %q, ok=%v", token, ok)
	}
}
