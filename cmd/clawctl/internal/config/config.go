package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// InstanceRecord is the JSON-serializable form persisted in config files.
type InstanceRecord struct {
	Name      string         `json:"name,omitempty"`
	ClawType  string         `json:"claw_type"`
	WorkDir   string         `json:"work_dir"`
	Port      int            `json:"port"`
	Version   string         `json:"version"`
	CreatedAt string         `json:"created_at"`
	Info      map[string]any `json:"info,omitempty"`
}

// Instance is the runtime representation used by manager and backends.
type Instance interface {
	GetName() string
	GetClawType() string
	GetWorkDir() string
	GetPort() int
	GetVersion() string
	GetCreatedAt() string
	GetInfo() map[string]any
	AsRecord() InstanceRecord
}

type baseInstance struct {
	record InstanceRecord
}

func (i *baseInstance) GetName() string      { return i.record.Name }
func (i *baseInstance) GetClawType() string  { return i.record.ClawType }
func (i *baseInstance) GetWorkDir() string   { return i.record.WorkDir }
func (i *baseInstance) GetPort() int         { return i.record.Port }
func (i *baseInstance) GetVersion() string   { return i.record.Version }
func (i *baseInstance) GetCreatedAt() string { return i.record.CreatedAt }
func (i *baseInstance) GetInfo() map[string]any {
	return cloneInfoMap(i.record.Info)
}
func (i *baseInstance) AsRecord() InstanceRecord {
	record := i.record
	record.Info = cloneInfoMap(record.Info)
	return record
}

func cloneInfoValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneInfoMap(typed)
	case []any:
		clone := make([]any, len(typed))
		for i, item := range typed {
			clone[i] = cloneInfoValue(item)
		}
		return clone
	default:
		return typed
	}
}

func cloneInfoMap(info map[string]any) map[string]any {
	if len(info) == 0 {
		return nil
	}
	clone := make(map[string]any, len(info))
	for k, v := range info {
		clone[k] = cloneInfoValue(v)
	}
	return clone
}

func newInstanceFromRecord(record InstanceRecord) Instance {
	record.Info = cloneInfoMap(record.Info)
	return &baseInstance{record: record}
}

// NewInstanceFromRecord creates a runtime instance from a persisted record.
func NewInstanceFromRecord(record InstanceRecord) Instance {
	return newInstanceFromRecord(record)
}

// Config is the top-level clawctl configuration file.
type Config struct {
	Instances map[string]Instance `json:"-"`
	Default   string              `json:"default"`
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
		var record InstanceRecord
		if err := json.Unmarshal(val, &record); err != nil {
			return nil, fmt.Errorf("parse instance %q: %w", key, err)
		}
		if record.Name == "" {
			record.Name = key
		}
		cfg.Instances[key] = newInstanceFromRecord(record)
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
		record := inst.AsRecord()
		if record.Name == "" {
			record.Name = name
		}
		raw[name] = record
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
	if version == "" {
		version = "latest"
	}
	return newInstanceFromRecord(InstanceRecord{
		Name:      name,
		ClawType:  clawType,
		WorkDir:   workDir,
		Port:      port,
		Version:   version,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// UpdateInstance updates an existing instance atomically.
func UpdateInstance(name string, mutate func(record InstanceRecord) (InstanceRecord, error)) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	inst, ok := cfg.Instances[name]
	if !ok {
		return fmt.Errorf("instance %q not found", name)
	}
	record, err := mutate(inst.AsRecord())
	if err != nil {
		return err
	}
	if record.Name == "" {
		record.Name = name
	}
	cfg.Instances[name] = newInstanceFromRecord(record)
	return Save(cfg)
}

// SetInfoPath sets a nested info value using the provided path.
func SetInfoPath(info map[string]any, value any, path ...string) map[string]any {
	if len(path) == 0 {
		return cloneInfoMap(info)
	}
	root := cloneInfoMap(info)
	if root == nil {
		root = make(map[string]any)
	}
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok || next == nil {
			next = make(map[string]any)
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = cloneInfoValue(value)
	return root
}

// GetInfoPath returns a nested info value using the provided path.
func GetInfoPath(info map[string]any, path ...string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current := info
	for idx, key := range path {
		value, ok := current[key]
		if !ok {
			return nil, false
		}
		if idx == len(path)-1 {
			return value, true
		}
		next, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

// GetInstanceInfoInt returns a nested integer value from instance info.
func GetInstanceInfoInt(inst Instance, path ...string) (int, bool) {
	value, ok := GetInfoPath(inst.GetInfo(), path...)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err == nil {
			return int(n), true
		}
	case string:
		n, err := strconv.Atoi(typed)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

// GetInstanceInfoString returns a nested string value from instance info.
func GetInstanceInfoString(inst Instance, path ...string) (string, bool) {
	value, ok := GetInfoPath(inst.GetInfo(), path...)
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	case json.Number:
		return typed.String(), true
	default:
		return fmt.Sprintf("%v", typed), true
	}
}

func mergeInfoMaps(dst, src map[string]any) map[string]any {
	if len(dst) == 0 && len(src) == 0 {
		return nil
	}
	merged := cloneInfoMap(dst)
	if merged == nil {
		merged = make(map[string]any)
	}
	for key, value := range src {
		if srcMap, ok := value.(map[string]any); ok {
			existing, _ := merged[key].(map[string]any)
			merged[key] = mergeInfoMaps(existing, srcMap)
			continue
		}
		merged[key] = cloneInfoValue(value)
	}
	return merged
}

// UpdateInstanceInfo updates the info field for an existing instance.
func UpdateInstanceInfo(name string, info map[string]any) error {
	return UpdateInstance(name, func(record InstanceRecord) (InstanceRecord, error) {
		record.Info = mergeInfoMaps(record.Info, info)
		return record, nil
	})
}
