package manager

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sipeed/clawctl/cmd/clawctl/internal/agent"
	"github.com/sipeed/clawctl/cmd/clawctl/internal/config"
	"github.com/sipeed/picoclaw/pkg/pid"
)

// GatewayRunner starts and manages a gateway process for a Claw instance.
type GatewayRunner struct {
	Instance   config.Instance
	Spec       agent.ClawSpec
	BinaryPath string
}

// NewGatewayRunner creates a runner for the given instance.
func NewGatewayRunner(inst config.Instance) (*GatewayRunner, error) {
	spec, err := agent.Get(inst.ClawType)
	if err != nil {
		return nil, err
	}

	// Resolve version tag (latest → actual, nightly → actual).
	version := inst.Version
	if version == "latest" || version == "nightly" {
		tag, err := FetchLatestTag(spec.Repo)
		if err != nil {
			return nil, fmt.Errorf("resolve %s version: %w", version, err)
		}
		version = tag
	}

	binPath, err := VersionBinaryPath(inst.ClawType, version, spec.GatewayBinary)
	if err != nil {
		return nil, fmt.Errorf("find binary: %w", err)
	}

	return &GatewayRunner{
		Instance:   inst,
		Spec:       spec,
		BinaryPath: binPath,
	}, nil
}

// PIDFilePath returns the path to the PID file for the instance.
func PIDFilePath(workDir string) string {
	return filepath.Join(workDir, ".picoclaw.pid")
}

// ZeroclawPIDFilePath returns the path to the PID file for a zeroclaw instance.
func ZeroclawPIDFilePath(workDir string) string {
	return filepath.Join(workDir, ".zeroclaw.pid")
}

// writeZeroclawPID writes a simple PID file for zeroclaw processes.
func writeZeroclawPID(workDir string, pid int) error {
	return os.WriteFile(ZeroclawPIDFilePath(workDir), []byte(fmt.Sprintf("%d\n", pid)), 0o644)
}

// readZeroclawPID reads the PID from zeroclaw's PID file and checks if process is alive.
func readZeroclawPID(workDir string) (int, bool) {
	data, err := os.ReadFile(ZeroclawPIDFilePath(workDir))
	if err != nil {
		return 0, false
	}
	var p int
	if _, err := fmt.Sscanf(string(data), "%d", &p); err != nil {
		return 0, false
	}
	if !isProcessRunning(p) {
		_ = os.Remove(ZeroclawPIDFilePath(workDir))
		return 0, false
	}
	return p, true
}

// Status returns the running status and PID data for the instance.
// For zero type, it reads the zeroclaw-specific PID file.
func Status(inst config.Instance) (running bool, pidData *pid.PidFileData, err error) {
	if inst.ClawType == "zero" {
		p, ok := readZeroclawPID(inst.WorkDir)
		if !ok {
			return false, nil, nil
		}
		// Return a minimal PidFileData for zero; zeroclaw doesn't have the full struct.
		return true, &pid.PidFileData{PID: p}, nil
	}
	pidData = pid.ReadPidFileWithCheck(inst.WorkDir)
	running = pidData != nil
	return running, pidData, nil
}

// Start launches the gateway process for the instance.
// It sets appropriate env vars and runs the binary as a detached child process.
func (r *GatewayRunner) Start() error {
	// Check if already running.
	if r.Instance.ClawType == "zero" {
		if p, ok := readZeroclawPID(r.Instance.WorkDir); ok {
			return fmt.Errorf("already running (PID %d)", p)
		}
	} else {
		if pidData := pid.ReadPidFileWithCheck(r.Instance.WorkDir); pidData != nil {
			return fmt.Errorf("already running (PID %d)", pidData.PID)
		}
	}

	// Verify binary exists.
	if _, err := os.Stat(r.BinaryPath); err != nil {
		return fmt.Errorf("binary not found: %s", r.BinaryPath)
	}

	// Ensure work_dir has basic structure.
	if err := os.MkdirAll(r.Instance.WorkDir, 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}

	// Build the command using the claw-type-specific gateway args.
	// For zeroclaw, also pass --config-dir and -p/--port.
	// args order: global flags (--config-dir) come before the subcommand.
	args := make([]string, 0, len(r.Spec.GatewayArgs)+6)
	if r.Instance.ClawType == "zero" {
		// zeroclaw: --config-dir goes before the subcommand.
		args = append(args, "--config-dir", r.Instance.WorkDir)
		args = append(args, r.Spec.GatewayArgs...)
		// -p flag goes after the subcommand (daemon).
		if r.Instance.Port > 0 {
			args = append(args, "-p", fmt.Sprintf("%d", r.Instance.Port))
		}
	} else {
		args = append(args, r.Spec.GatewayArgs...)
	}

	cmd := exec.Command(r.BinaryPath, args...)
	cmd.Dir = r.Instance.WorkDir

	// Set environment.
	env := os.Environ()
	env = append(env,
		"PICOCLAW_HOME="+r.Instance.WorkDir,
		"PICOCLAW_CONFIG="+filepath.Join(r.Instance.WorkDir, "config.json"),
		"PICOCLAW_GATEWAY_HOST=127.0.0.1",
	)
	if r.Instance.Port > 0 {
		env = append(env, fmt.Sprintf("PICOCLAW_GATEWAY_PORT=%d", r.Instance.Port))
	}
	if r.Instance.ClawType == "zero" {
		// zeroclaw uses ZEROCLAW_WORKSPACE env var to locate workspace dir.
		env = append(env, "ZEROCLAW_WORKSPACE="+r.Instance.WorkDir)
	}
	cmd.Env = env

	// Redirect output to a log file in work_dir.
	logFile, err := os.OpenFile(
		filepath.Join(r.Instance.WorkDir, ".gateway.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644,
	)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach: start in a new session and process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// Don't inherit fd 0,1,2 — we've redirected them.
	cmd.ExtraFiles = []*os.File{logFile}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start gateway: %w", err)
	}
	logFile.Close()

	// For zeroclaw, we write the PID file ourselves since zeroclaw doesn't create one.
	// For picoclaw, the process writes its own .picoclaw.pid.
	if r.Instance.ClawType == "zero" {
		if err := writeZeroclawPID(r.Instance.WorkDir, cmd.Process.Pid); err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("write zeroclaw pid file: %w", err)
		}
	}

	// The gateway process (cmd.Process) is now running.
	// It will write its own PID to the PID file.
	// Wait briefly for the PID file to appear.
	pidPath := PIDFilePath(r.Instance.WorkDir)
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		if _, err := os.Stat(pidPath); err == nil {
			// PID file created.
			pidData := pid.ReadPidFileWithCheck(r.Instance.WorkDir)
			if pidData != nil {
				return nil // started successfully
			}
		}
		// Check if the process is still alive.
		proc, _ := os.FindProcess(cmd.Process.Pid)
		if proc == nil || !isProcessRunning(proc.Pid) {
			return fmt.Errorf("gateway process exited unexpectedly")
		}
	}

	// Timeout waiting for PID file, but process might still be initializing.
	// Check if the process is alive.
	proc, _ := os.FindProcess(cmd.Process.Pid)
	if proc != nil && isProcessRunning(proc.Pid) {
		// Process is running but PID file not yet written.
		// This can happen with certain configurations. Still considered a success.
		return nil
	}
	return fmt.Errorf("gateway process did not start properly")
}

// Stop terminates the gateway process for the instance.
func Stop(inst config.Instance) error {
	var pidPath string
	var pidData *pid.PidFileData

	if inst.ClawType == "zero" {
		pidPath = ZeroclawPIDFilePath(inst.WorkDir)
		p, ok := readZeroclawPID(inst.WorkDir)
		if !ok {
			return fmt.Errorf("not running (no PID file found)")
		}
		pidData = &pid.PidFileData{PID: p}
	} else {
		pidPath = PIDFilePath(inst.WorkDir)
		pidData = pid.ReadPidFileWithCheck(inst.WorkDir)
		if pidData == nil {
			return fmt.Errorf("not running (no PID file found)")
		}
	}

	// Try SIGTERM first for graceful shutdown.
	p, err := os.FindProcess(pidData.PID)
	if err == nil && p != nil {
		_ = p.Signal(syscall.SIGTERM)
	}

	// Wait up to 5 seconds for graceful shutdown.
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !isProcessRunning(pidData.PID) {
			break
		}
	}

	// If still running, SIGKILL.
	if isProcessRunning(pidData.PID) {
		p, _ := os.FindProcess(pidData.PID)
		if p != nil {
			_ = p.Kill()
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Remove PID file.
	_ = os.Remove(pidPath)

	if isProcessRunning(pidData.PID) {
		return fmt.Errorf("failed to stop process (PID %d)", pidData.PID)
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is alive.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal(0) checks existence on Unix without sending a signal.
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.EPERM
}
