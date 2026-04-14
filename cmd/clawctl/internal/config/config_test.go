package config

import (
	"testing"
)

func TestUpdateInstanceInfoMergesNestedMaps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{
		Instances: map[string]Instance{
			"pico1": NewInstanceFromRecord(InstanceRecord{
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
			}),
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	err := UpdateInstanceInfo("pico1", map[string]any{
		"runtime": map[string]any{
			"dashboard_token": "secret-token",
		},
	})
	if err != nil {
		t.Fatalf("update instance info: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	inst := loaded.Instances["pico1"]

	if launcher, ok := GetInstanceInfoInt(inst, "ports", "launcher"); !ok || launcher != 18800 {
		t.Fatalf("expected launcher port 18800, got %d, ok=%v", launcher, ok)
	}
	if gateway, ok := GetInstanceInfoInt(inst, "ports", "gateway"); !ok || gateway != 18790 {
		t.Fatalf("expected gateway port 18790, got %d, ok=%v", gateway, ok)
	}
	if token, ok := GetInstanceInfoString(inst, "runtime", "dashboard_token"); !ok || token != "secret-token" {
		t.Fatalf("expected merged dashboard token, got %q, ok=%v", token, ok)
	}
}

func TestSetInfoPathCreatesNestedMaps(t *testing.T) {
	info := SetInfoPath(nil, 18791, "ports", "gateway")
	info = SetInfoPath(info, "token", "runtime", "dashboard_token")

	inst := NewInstanceFromRecord(InstanceRecord{
		Name:      "pico1",
		ClawType:  "picoclaw",
		WorkDir:   "/tmp/pico1",
		Port:      18800,
		Version:   "latest",
		CreatedAt: "2026-04-14T00:00:00Z",
		Info:      info,
	})

	if gateway, ok := GetInstanceInfoInt(inst, "ports", "gateway"); !ok || gateway != 18791 {
		t.Fatalf("expected gateway port 18791, got %d, ok=%v", gateway, ok)
	}
	if token, ok := GetInstanceInfoString(inst, "runtime", "dashboard_token"); !ok || token != "token" {
		t.Fatalf("expected dashboard token, got %q, ok=%v", token, ok)
	}
}
