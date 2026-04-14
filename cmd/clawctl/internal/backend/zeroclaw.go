package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func init() {
	b := &zeroclawBackend{}
	Register("zero", BackendSpec{Backend: b, Configurator: b})
}

type zeroclawBackend struct{}

func (b *zeroclawBackend) AllocateInstance(_ context.Context, cfg *config.Config, name string, explicitPort int, version, workDir string) (config.Instance, error) {
	port := explicitPort
	if port == 0 {
		var err error
		port, err = allocatePort(18792, collectReservedPorts(cfg))
		if err != nil {
			return nil, err
		}
	} else if !isPortAvailable(port) {
		return nil, fmt.Errorf("port %d is already in use", port)
	}
	return config.NewInstance("zero", name, port, version, workDir), nil
}

func (b *zeroclawBackend) ReconcileInstance(_ context.Context, _ *config.Config, inst config.Instance) (config.Instance, bool, error) {
	return inst, false, nil
}

// PIDFilePath is kept for interface compliance but not used internally.
func (b *zeroclawBackend) PIDFilePath(workDir string) string {
	return filepath.Join(workDir, ".zeroclaw.pid")
}

func (b *zeroclawBackend) Type() string     { return "zero" }
func (b *zeroclawBackend) Repo() string     { return "zeroclaw-labs/zeroclaw" }
func (b *zeroclawBackend) DefaultPort() int { return 18792 }
func (b *zeroclawBackend) BinaryNames() []string {
	return []string{"zeroclaw"}
}
func (b *zeroclawBackend) GatewayBinary() string { return "zeroclaw" }
func (b *zeroclawBackend) GatewayArgs() []string { return []string{"daemon"} }

// findDaemonPid finds the zeroclaw daemon process PID by matching workDir in process arguments.
func (b *zeroclawBackend) findDaemonPid(workDir string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-axo", "pid,args")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "zeroclaw") &&
			strings.Contains(line, "--config-dir") &&
			strings.Contains(line, workDir) {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				pid, _ := strconv.Atoi(fields[0])
				return pid
			}
		}
	}
	return 0
}

func (b *zeroclawBackend) IsRunning(workDir string) (int, bool, error) {
	pid := b.findDaemonPid(workDir)
	if pid == 0 {
		return 0, false, nil
	}
	return pid, true, nil
}

func (b *zeroclawBackend) StatusDetail(workDir string) (*StatusDetail, error) {
	pid := b.findDaemonPid(workDir)
	if pid == 0 {
		return nil, fmt.Errorf("daemon not running")
	}
	return &StatusDetail{}, nil
}

func (b *zeroclawBackend) InitWorkDir(inst InstanceInfo) error {
	if err := os.MkdirAll(inst.GetWorkDir(), 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(inst.GetWorkDir(), "agent_type"), []byte(b.Type()+"\n"), 0o644); err != nil {
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

func (b *zeroclawBackend) Start(inst InstanceInfo, binaryPath string) error {
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}

	// Check if already running.
	if pid := b.findDaemonPid(inst.GetWorkDir()); pid > 0 {
		return fmt.Errorf("already running (PID %d)", pid)
	}

	if err := os.MkdirAll(inst.GetWorkDir(), 0o755); err != nil {
		return fmt.Errorf("create work_dir: %w", err)
	}

	// Build args: --config-dir WORKDIR daemon [-p PORT]
	args := []string{"--config-dir", inst.GetWorkDir()}
	args = append(args, b.GatewayArgs()...)
	if inst.GetPort() > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", inst.GetPort()))
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = inst.GetWorkDir()

	env := os.Environ()
	env = append(env, "ZEROCLAW_WORKSPACE="+inst.GetWorkDir())
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
		return fmt.Errorf("start gateway: %w", err)
	}
	logFile.Close()

	parentPid := cmd.Process.Pid

	// Wait for parent process to exit (zeroclaw forks, then parent exits).
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isProcessRunning(parentPid) {
			break
		}
	}

	// Verify the daemon is running by finding it via workDir.
	if pid := b.findDaemonPid(inst.GetWorkDir()); pid == 0 {
		return fmt.Errorf("daemon process not found after start")
	}

	return nil
}

func (b *zeroclawBackend) Stop(inst InstanceInfo) error {
	pid := b.findDaemonPid(inst.GetWorkDir())
	if pid == 0 {
		return fmt.Errorf("not running (daemon not found)")
	}

	proc, _ := os.FindProcess(pid)
	if proc != nil {
		_ = proc.Signal(syscall.SIGTERM)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !isProcessRunning(pid) {
			break
		}
	}

	if isProcessRunning(pid) {
		proc, _ := os.FindProcess(pid)
		if proc != nil {
			_ = proc.Kill()
		}
		time.Sleep(500 * time.Millisecond)
	}

	if isProcessRunning(pid) {
		return fmt.Errorf("failed to stop process (PID %d)", pid)
	}
	return nil
}

func (b *zeroclawBackend) ResetWorkspace(_ InstanceInfo) error {
	return ErrNotSupported
}

func (b *zeroclawBackend) GatherInfo(workDir string) map[string]any {
	return nil
}
