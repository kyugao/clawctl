package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/onboard"
)

func init() {
	Register("picoclaw", &picoclawBackend{})
}

type picoclawBackend struct{}

func (b *picoclawBackend) Repo() string    { return "sipeed/picoclaw" }
func (b *picoclawBackend) BinaryNames() []string {
	return []string{"picoclaw", "picoclaw-launcher", "picoclaw-launcher-tui"}
}
func (b *picoclawBackend) GatewayBinary() string { return "picoclaw" }

func (b *picoclawBackend) gatewayArgs() []string { return []string{"gateway", "-E"} }

func (b *picoclawBackend) pidFilePath(workDir string) string {
	return filepath.Join(workDir, ".picoclaw.pid")
}

func (b *picoclawBackend) IsRunning(workDir string) (int, bool, error) {
	pidData := ReadPidFileWithCheck(workDir)
	if pidData == nil {
		return 0, false, nil
	}
	return pidData.PID, true, nil
}

func (b *picoclawBackend) StatusDetail(workDir string) (*StatusDetail, error) {
	pidData := ReadPidFileWithCheck(workDir)
	if pidData == nil {
		return nil, fmt.Errorf("no pid file")
	}
	return &StatusDetail{Port: pidData.Port, Host: pidData.Host, Version: pidData.Version}, nil
}

func (b *picoclawBackend) InitWorkDir(inst InstanceInfo) error {
	if err := os.MkdirAll(inst.GetWorkDir(), 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(inst.GetWorkDir(), "agent_type"), []byte("picoclaw\n"), 0o644); err != nil {
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

func (b *picoclawBackend) Start(inst InstanceInfo, binaryPath string) error {
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}

	if err := os.MkdirAll(inst.GetWorkDir(), 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}

	// Check if already running.
	if pidData := ReadPidFileWithCheck(inst.GetWorkDir()); pidData != nil {
		return fmt.Errorf("already running (PID %d)", pidData.PID)
	}

	args := b.gatewayArgs()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = inst.GetWorkDir()

	env := os.Environ()
	env = append(env,
		"PICOCLAW_HOME="+inst.GetWorkDir(),
		"PICOCLAW_CONFIG="+filepath.Join(inst.GetWorkDir(), "config.json"),
		"PICOCLAW_GATEWAY_HOST=127.0.0.1",
	)
	if inst.GetPort() > 0 {
		env = append(env, fmt.Sprintf("PICOCLAW_GATEWAY_PORT=%d", inst.GetPort()))
	}
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
	cmd.ExtraFiles = []*os.File{logFile}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start gateway: %w", err)
	}
	logFile.Close()

	// Wait briefly for the PID file to appear.
	pidPath := b.pidFilePath(inst.GetWorkDir())
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		if _, err := os.Stat(pidPath); err == nil {
			pidData := ReadPidFileWithCheck(inst.GetWorkDir())
			if pidData != nil {
				return nil
			}
		}
		proc, _ := os.FindProcess(cmd.Process.Pid)
		if proc == nil || !isProcessRunning(proc.Pid) {
			return fmt.Errorf("gateway process exited unexpectedly")
		}
	}

	// Timeout waiting for PID file, but process might still be initializing.
	proc, _ := os.FindProcess(cmd.Process.Pid)
	if proc != nil && isProcessRunning(proc.Pid) {
		return nil
	}
	return fmt.Errorf("gateway process did not start properly")
}

func (b *picoclawBackend) Stop(inst InstanceInfo) error {
	pidPath := b.pidFilePath(inst.GetWorkDir())
	pidData := ReadPidFileWithCheck(inst.GetWorkDir())
	if pidData == nil {
		return fmt.Errorf("not running (no PID file found)")
	}

	p, err := os.FindProcess(pidData.PID)
	if err == nil && p != nil {
		_ = p.Signal(syscall.SIGTERM)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !isProcessRunning(pidData.PID) {
			break
		}
	}

	if isProcessRunning(pidData.PID) {
		p, _ := os.FindProcess(pidData.PID)
		if p != nil {
			_ = p.Kill()
		}
		time.Sleep(500 * time.Millisecond)
	}

	_ = os.Remove(pidPath)

	if isProcessRunning(pidData.PID) {
		return fmt.Errorf("failed to stop process (PID %d)", pidData.PID)
	}
	return nil
}

func (b *picoclawBackend) ResetWorkspace(inst InstanceInfo) error {
	if err := onboard.CopyWorkspaceTemplates(inst.GetWorkDir()); err != nil {
		return fmt.Errorf("copy workspace templates: %w", err)
	}
	return nil
}
