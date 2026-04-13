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
func (b *picoclawBackend) GatewayBinary() string { return "picoclaw-launcher" }

func (b *picoclawBackend) launcherArgs(inst InstanceInfo) []string {
	args := []string{
		"-console",
		"-no-browser",
	}
	if inst.GetPort() > 0 {
		args = append(args, "-port", fmt.Sprintf("%d", inst.GetPort()))
	}
	args = append(args, filepath.Join(inst.GetWorkDir(), "config.json"))
	return args
}

func (b *picoclawBackend) pidFilePath(workDir string) string {
	return filepath.Join(workDir, ".picoclaw.pid")
}

func (b *picoclawBackend) IsRunning(workDir string) (int, bool, error) {
	pid := b.findLauncherPid(workDir)
	if pid == 0 {
		return 0, false, nil
	}
	return pid, true, nil
}

func (b *picoclawBackend) StatusDetail(workDir string) (*StatusDetail, error) {
	pid := b.findLauncherPid(workDir)
	if pid == 0 {
		return nil, fmt.Errorf("launcher not running")
	}
	return &StatusDetail{}, nil
}

func (b *picoclawBackend) findLauncherPid(workDir string) int {
	out, err := exec.Command("ps", "-axo", "pid,args").Output()
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "picoclaw-launcher") &&
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
	// Create default config.json if it doesn't exist.
	configPath := filepath.Join(inst.GetWorkDir(), "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := fmt.Sprintf(`{
  "version": 2,
  "agents": {
    "defaults": {
      "workspace": "%s/workspace",
      "restrict_to_workspace": true,
      "allow_read_outside_workspace": false
    }
  }
}
`, inst.GetWorkDir())
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
			return fmt.Errorf("write config.json: %w", err)
		}
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
	if pid := b.findLauncherPid(inst.GetWorkDir()); pid > 0 {
		return fmt.Errorf("already running (PID %d)", pid)
	}

	args := b.launcherArgs(inst)

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = inst.GetWorkDir()

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
		return fmt.Errorf("start launcher: %w", err)
	}
	logFile.Close()

	// Wait briefly for the launcher to start.
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		if pid := b.findLauncherPid(inst.GetWorkDir()); pid > 0 {
			return nil
		}
		proc, _ := os.FindProcess(cmd.Process.Pid)
		if proc == nil || !isProcessRunning(proc.Pid) {
			return fmt.Errorf("launcher process exited unexpectedly")
		}
	}

	// Timeout waiting, but process might still be initializing.
	proc, _ := os.FindProcess(cmd.Process.Pid)
	if proc != nil && isProcessRunning(proc.Pid) {
		return nil
	}
	return fmt.Errorf("launcher did not start properly")
}

func (b *picoclawBackend) Stop(inst InstanceInfo) error {
	pid := b.findLauncherPid(inst.GetWorkDir())
	if pid == 0 {
		return fmt.Errorf("not running (launcher not found)")
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
	return nil
}

func (b *picoclawBackend) ResetWorkspace(inst InstanceInfo) error {
	if err := onboard.CopyWorkspaceTemplates(inst.GetWorkDir()); err != nil {
		return fmt.Errorf("copy workspace templates: %w", err)
	}
	return nil
}

// GatherInfo parses the gateway log and returns instance info (e.g., dashboard token).
func (b *picoclawBackend) GatherInfo(workDir string) map[string]any {
	info := make(map[string]any)

	logPath := filepath.Join(workDir, ".gateway.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return info
	}

	// Parse dashboard token from log
	// Format: "Dashboard token (this run): <token>"
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Dashboard token") {
			parts := strings.Split(line, "Dashboard token")
			if len(parts) < 2 {
				continue
			}
			tokenPart := strings.Split(parts[1], ":")
			if len(tokenPart) >= 2 {
				token := strings.TrimSpace(tokenPart[len(tokenPart)-1])
				info["dashboard_token"] = token
			}
			break
		}
	}

	return info
}
