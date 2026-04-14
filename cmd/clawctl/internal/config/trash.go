package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TrashItem represents a soft-deleted instance.
type TrashItem struct {
	ID           string         `json:"id"`            // name-timestamp
	InstanceName string         `json:"instance_name"` // original instance name
	Instance     InstanceRecord `json:"instance"`      // full instance config for restore
	TrashPath    string         `json:"trash_path"`    // absolute path in trash
	DeletedAt    string         `json:"deleted_at"`    // RFC3339 timestamp
}

// TrashMeta holds all trash items.
type TrashMeta struct {
	Items []TrashItem `json:"items"`
}

// trashMetaPath returns ~/.clawctl/trash-meta.json.
func trashMetaPath() (string, error) {
	dir, err := ClawctlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "trash-meta.json"), nil
}

// LoadTrashMeta reads the trash metadata file.
func LoadTrashMeta() (*TrashMeta, error) {
	path, err := trashMetaPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrashMeta{Items: []TrashItem{}}, nil
		}
		return nil, fmt.Errorf("read trash meta: %w", err)
	}
	var meta TrashMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse trash meta: %w", err)
	}
	return &meta, nil
}

// SaveTrashMeta writes the trash metadata file atomically.
func SaveTrashMeta(meta *TrashMeta) error {
	path, err := trashMetaPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trash meta: %w", err)
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

// TrashDir returns ~/.clawctl/trash.
func TrashDir() (string, error) {
	dir, err := ClawctlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "trash"), nil
}

// MoveToTrash moves an instance's work_dir to the trash directory.
// Returns the TrashItem describing where it was moved to.
func MoveToTrash(name string, inst Instance) (*TrashItem, error) {
	trashDir, err := TrashDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return nil, fmt.Errorf("create trash dir: %w", err)
	}

	timestamp := time.Now().Unix()
	id := fmt.Sprintf("%s-%d", name, timestamp)
	trashPath := filepath.Join(trashDir, id)

	// Move the directory.
	if err := os.Rename(inst.GetWorkDir(), trashPath); err != nil {
		return nil, fmt.Errorf("move to trash: %w", err)
	}

	absTrashPath, err := filepath.Abs(trashPath)
	if err != nil {
		absTrashPath = trashPath
	}

	item := &TrashItem{
		ID:           id,
		InstanceName: name,
		Instance:     inst.AsRecord(),
		TrashPath:    absTrashPath,
		DeletedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	meta, err := LoadTrashMeta()
	if err != nil {
		return item, nil // non-fatal: meta file might not exist yet
	}
	meta.Items = append(meta.Items, *item)
	if err := SaveTrashMeta(meta); err != nil {
		return item, nil // non-fatal: instance is already moved
	}
	return item, nil
}

// RemoveFromTrash permanently deletes a trash item from the trash directory and meta.
func RemoveFromTrash(id string) error {
	meta, err := LoadTrashMeta()
	if err != nil {
		return err
	}
	var item *TrashItem
	idx := -1
	for i := range meta.Items {
		if meta.Items[i].ID == id {
			item = &meta.Items[i]
			idx = i
			break
		}
	}
	if item == nil {
		return fmt.Errorf("trash item %q not found", id)
	}

	// Remove from filesystem.
	if err := os.RemoveAll(item.TrashPath); err != nil {
		return fmt.Errorf("remove trash dir: %w", err)
	}

	// Update meta.
	meta.Items = append(meta.Items[:idx], meta.Items[idx+1:]...)
	return SaveTrashMeta(meta)
}

// RestoreFromTrash moves a trash item back to instances/ and restores its config entry.
func RestoreFromTrash(id string) (string, error) {
	meta, err := LoadTrashMeta()
	if err != nil {
		return "", err
	}
	var item *TrashItem
	idx := -1
	for i := range meta.Items {
		if meta.Items[i].ID == id {
			item = &meta.Items[i]
			idx = i
			break
		}
	}
	if item == nil {
		return "", fmt.Errorf("trash item %q not found", id)
	}

	// Check if destination already exists.
	instancesDir, err := InstancesDir()
	if err != nil {
		return "", err
	}
	restoredPath := filepath.Join(instancesDir, item.InstanceName)
	if _, err := os.Stat(restoredPath); err == nil {
		return "", fmt.Errorf("restored path %q already exists", restoredPath)
	}

	// Move back.
	if err := os.Rename(item.TrashPath, restoredPath); err != nil {
		return "", fmt.Errorf("restore from trash: %w", err)
	}

	// Restore config entry.
	cfg, err := Load()
	if err == nil {
		cfg.Instances[item.InstanceName] = newInstanceFromRecord(item.Instance)
		Save(cfg) // best effort
	}

	// Remove from meta.
	meta.Items = append(meta.Items[:idx], meta.Items[idx+1:]...)
	SaveTrashMeta(meta) // best effort

	return restoredPath, nil
}
