package manager

import (
	"testing"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func TestInstancePortSummaryIncludesGatewayPortForPicoclaw(t *testing.T) {
	inst := config.NewInstanceFromRecord(config.InstanceRecord{
		Name:      "pico1",
		ClawType:  "picoclaw",
		WorkDir:   "/tmp/pico1",
		Port:      18800,
		Version:   "latest",
		CreatedAt: "2026-04-14T00:00:00Z",
		Info: map[string]any{
			"ports": map[string]any{
				"launcher": 18800,
				"gateway":  18790,
			},
		},
	})

	got := instancePortSummary(inst)
	want := "18800 gw:18790"
	if got != want {
		t.Fatalf("expected port summary %q, got %q", want, got)
	}
}

func TestInstanceDetailLinesIncludeRuntimeInfo(t *testing.T) {
	inst := config.NewInstanceFromRecord(config.InstanceRecord{
		Name:      "pico1",
		ClawType:  "picoclaw",
		WorkDir:   "/tmp/pico1",
		Port:      18800,
		Version:   "latest",
		CreatedAt: "2026-04-14T00:00:00Z",
		Info: map[string]any{
			"ports": map[string]any{
				"launcher": 18800,
				"gateway":  18790,
			},
			"runtime": map[string]any{
				"dashboard_token": "token-123",
			},
		},
	})

	lines := instanceDetailLines(inst)
	if len(lines) != 3 {
		t.Fatalf("expected 3 detail lines, got %d: %#v", len(lines), lines)
	}
}
