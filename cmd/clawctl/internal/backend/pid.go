package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const pidFileName = ".picoclaw.pid"

// PidData holds the contents of the picoclaw PID file.
type PidData struct {
	PID     int    `json:"pid"`
	Token   string `json:"token"`
	Version string `json:"version"`
	Port    int    `json:"port"`
	Host    string `json:"host"`
}

// ReadPidFileWithCheck reads the picoclaw PID file and checks if the process is still alive.
// Returns nil if the file is missing, unreadable, or the process has exited.
func ReadPidFileWithCheck(workDir string) *PidData {
	pidPath := filepath.Join(workDir, pidFileName)

	raw, err := os.ReadFile(pidPath)
	if err != nil {
		return nil
	}

	var data PidData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}

	if data.PID <= 0 {
		return nil
	}

	if !isProcessRunning(data.PID) {
		return nil
	}

	return &data
}

func pidFilePath(workDir string) string {
	return filepath.Join(workDir, pidFileName)
}

// RemovePidFile deletes the PID file if it exists.
func RemovePidFile(workDir string) error {
	pidPath := pidFilePath(workDir)
	_, err := os.Stat(pidPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat pid file: %w", err)
	}
	return os.Remove(pidPath)
}
