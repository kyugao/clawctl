package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// defaultPorts maps claw types to their default ports.
var defaultPorts = map[string]int{
	"picoclaw": 18790,
	"openclaw": 18791,
	"zero":     18792,
}

// Instance represents a single Claw instance.
type Instance struct {
	ClawType  string         `json:"claw_type"`
	WorkDir   string         `json:"work_dir"`
	Port      int            `json:"port"`
	Version   string         `json:"version"`
	CreatedAt string         `json:"created_at"`
	Info      map[string]any `json:"info,omitempty"`
}

// GetClawType returns the claw type.
func (i Instance) GetClawType() string { return i.ClawType }

// GetWorkDir returns the work directory.
func (i Instance) GetWorkDir() string { return i.WorkDir }

// GetPort returns the gateway port.
func (i Instance) GetPort() int { return i.Port }

// GetVersion returns the version string.
func (i Instance) GetVersion() string { return i.Version }

// Config is the top-level clawctl configuration file.
type Config struct {
	Instances map[string]Instance `json:"-"`
	Default   string               `json:"default"`
}

// clawctlConfigPath returns the path to ~/.clawctl/config.json.
func clawctlConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".clawctl", "config.json"), nil
}

// Load reads and parses the clawctl config file.
func Load() (*Config, error) {
	path, err := clawctlConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Instances: map[string]Instance{}}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Decode into a raw map first so we can extract known keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &Config{Instances: map[string]Instance{}}
	for key, val := range raw {
		if key == "default" {
			if err := json.Unmarshal(val, &cfg.Default); err != nil {
				return nil, fmt.Errorf("parse default: %w", err)
			}
			continue
		}
		var inst Instance
		if err := json.Unmarshal(val, &inst); err != nil {
			return nil, fmt.Errorf("parse instance %q: %w", key, err)
		}
		cfg.Instances[key] = inst
	}

	return cfg, nil
}

// Save writes the clawctl config file atomically.
func Save(cfg *Config) error {
	path, err := clawctlConfigPath()
	if err != nil {
		return err
	}

	// Build a flat map with "default" key.
	raw := make(map[string]any)
	raw["default"] = cfg.Default
	for name, inst := range cfg.Instances {
		raw[name] = inst
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// EnsureClawctlHome creates ~/.clawctl if it doesn't exist.
func EnsureClawctlHome() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(home, ".clawctl"), 0o755)
}

// ClawctlDir returns ~/.clawctl.
func ClawctlDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".clawctl"), nil
}

// InstancesDir returns ~/.clawctl/instances.
func InstancesDir() (string, error) {
	dir, err := ClawctlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "instances"), nil
}

// ReleasesDir returns ~/.clawctl/claw_release/<type>.
func ReleasesDir(clawType string) (string, error) {
	dir, err := ClawctlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "claw_release", clawType), nil
}

// InstanceWorkDir returns the default work directory for a new instance.
func InstanceWorkDir(name string) (string, error) {
	dir, err := InstancesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// NewInstance creates a new Instance with defaults filled in.
func NewInstance(clawType string, name string, port int, version string, workDir string) Instance {
	if workDir == "" {
		workDir, _ = InstanceWorkDir(name)
	}
	if port == 0 {
		port = defaultPorts[clawType]
	}
	return Instance{
		ClawType:  clawType,
		WorkDir:   workDir,
		Port:      port,
		Version:   version,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// UpdateInstanceInfo updates the info field for an existing instance.
func UpdateInstanceInfo(name string, info map[string]any) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	inst, ok := cfg.Instances[name]
	if !ok {
		return fmt.Errorf("instance %q not found", name)
	}
	inst.Info = info
	cfg.Instances[name] = inst
	return Save(cfg)
}
