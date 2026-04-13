package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("hermes", &hermesBackend{})
}

type hermesBackend struct{}

// DefaultPort returns the default gateway port for hermes.
func (b *hermesBackend) DefaultPort() int { return 8642 }

func (b *hermesBackend) Repo() string { return "NousResearch/hermes-agent" }

func (b *hermesBackend) BinaryNames() []string {
	return []string{"hermes"}
}

func (b *hermesBackend) GatewayBinary() string { return "hermes" }

func (b *hermesBackend) GatewayArgs() []string { return []string{"gateway", "run"} }

func (b *hermesBackend) IsRunning(workDir string) (int, bool, error) {
	pid := readHermesPid(workDir)
	if pid == 0 {
		return 0, false, nil
	}
	// Verify process is actually running
	if !isProcessRunning(pid) {
		return 0, false, nil
	}
	return pid, true, nil
}

func (b *hermesBackend) StatusDetail(workDir string) (*StatusDetail, error) {
	pid := readHermesPid(workDir)
	if pid == 0 {
		return nil, fmt.Errorf("hermes not running")
	}
	return &StatusDetail{Port: 8642, Host: "127.0.0.1"}, nil
}

// hermesPidFilePath returns the path to the PID file for hermes.
func hermesPidFilePath(workDir string) string {
	return filepath.Join(workDir, ".hermes.pid")
}

// writeHermesPid writes the PID to a file for later detection.
func writeHermesPid(workDir string, pid int) error {
	return os.WriteFile(hermesPidFilePath(workDir), []byte(strconv.Itoa(pid)), 0o644)
}

// readHermesPid reads the PID from the PID file.
func readHermesPid(workDir string) int {
	data, err := os.ReadFile(hermesPidFilePath(workDir))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

// findHermesPid finds hermes PID using ps matching (fallback method).
func (b *hermesBackend) findHermesPid(workDir string) int {
	out, err := exec.Command("ps", "-axo", "pid,args").Output()
	if err != nil {
		return 0
	}

	workDirResolved, _ := filepath.EvalSymlinks(workDir)

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "hermes") {
			continue
		}
		if !strings.Contains(line, "gateway") {
			continue
		}
		// Check if HERMES_HOME or workDir is in the command
		if strings.Contains(line, "HERMES_HOME="+workDir) ||
			strings.Contains(line, "HERMES_HOME="+workDirResolved) ||
			strings.Contains(line, workDir) ||
			strings.Contains(line, workDirResolved) {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				pid, _ := strconv.Atoi(fields[0])
				if pid > 0 {
					return pid
				}
			}
		}
	}
	return 0
}

func (b *hermesBackend) InitWorkDir(inst InstanceInfo) error {
	if err := os.MkdirAll(inst.GetWorkDir(), 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(inst.GetWorkDir(), "agent_type"), []byte("hermes\n"), 0o644); err != nil {
		return fmt.Errorf("write agent_type: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(inst.GetWorkDir(), "workspace"), 0o755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(inst.GetWorkDir(), "skills"), 0o755); err != nil {
		return fmt.Errorf("create skills: %w", err)
	}
	return nil
}

func (b *hermesBackend) Start(inst InstanceInfo, binaryPath string) error {
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}

	if err := os.MkdirAll(inst.GetWorkDir(), 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}

	// Check if already running.
	if pid := readHermesPid(inst.GetWorkDir()); pid > 0 {
		if isProcessRunning(pid) {
			return fmt.Errorf("already running (PID %d)", pid)
		}
		// Stale PID file - clean it up
		os.Remove(hermesPidFilePath(inst.GetWorkDir()))
	}

	args := b.GatewayArgs()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = inst.GetWorkDir()

	env := os.Environ()
	env = append(env, "HERMES_HOME="+inst.GetWorkDir())
	cmd.Env = env

	logFile, err := os.OpenFile(
		filepath.Join(inst.GetWorkDir(), ".gateway.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644,
	)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start hermes: %w", err)
	}
	logFile.Close()

	// Write PID file for tracking
	if err := writeHermesPid(inst.GetWorkDir(), cmd.Process.Pid); err != nil {
		// Non-fatal: just log warning
		fmt.Printf("warning: failed to write PID file: %v\n", err)
	}

	// Wait briefly for the process to start.
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		if pid := readHermesPid(inst.GetWorkDir()); pid > 0 {
			return nil
		}
		proc, _ := os.FindProcess(cmd.Process.Pid)
		if proc == nil || !isProcessRunning(proc.Pid) {
			return fmt.Errorf("hermes process exited unexpectedly")
		}
	}

	// Timeout waiting, but process might still be initializing.
	proc, _ := os.FindProcess(cmd.Process.Pid)
	if proc != nil && isProcessRunning(proc.Pid) {
		return nil
	}
	return fmt.Errorf("hermes did not start properly")
}

func (b *hermesBackend) Stop(inst InstanceInfo) error {
	pid := readHermesPid(inst.GetWorkDir())
	if pid == 0 {
		return fmt.Errorf("not running (hermes not found)")
	}

	p, err := os.FindProcess(pid)
	if err == nil && p != nil {
		_ = p.Signal(syscall.SIGTERM)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !isProcessRunning(pid) {
			break
		}
	}

	if isProcessRunning(pid) {
		p, _ := os.FindProcess(pid)
		if p != nil {
			_ = p.Kill()
		}
		time.Sleep(500 * time.Millisecond)
	}

	if isProcessRunning(pid) {
		return fmt.Errorf("failed to stop process (PID %d)", pid)
	}

	// Clean up PID file
	os.Remove(hermesPidFilePath(inst.GetWorkDir()))
	return nil
}

func (b *hermesBackend) ResetWorkspace(inst InstanceInfo) error {
	return ErrNotSupported
}
