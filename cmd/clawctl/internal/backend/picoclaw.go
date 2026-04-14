package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/onboard"
)

func init() {
	b := &picoclawBackend{}
	Register("picoclaw", BackendSpec{Backend: b, Configurator: b})
}

type picoclawBackend struct{}

func (b *picoclawBackend) AllocateInstance(_ context.Context, cfg *config.Config, name string, explicitPort int, version, workDir string) (config.Instance, error) {
	reserved := collectReservedPorts(cfg)

	launcherPort := explicitPort
	if launcherPort == 0 {
		var err error
		launcherPort, err = allocatePort(18800, reserved)
		if err != nil {
			return nil, err
		}
	} else {
		if _, exists := reserved[launcherPort]; exists {
			return nil, fmt.Errorf("port %d is already reserved by another instance", launcherPort)
		}
		if !isPortAvailable(launcherPort) {
			return nil, fmt.Errorf("port %d is already in use", launcherPort)
		}
	}
	reserved[launcherPort] = struct{}{}

	gatewayPort, err := allocatePort(18790, reserved)
	if err != nil {
		return nil, err
	}

	inst := config.NewInstance("picoclaw", name, launcherPort, version, workDir)
	record := inst.AsRecord()
	record.Info = config.SetInfoPath(record.Info, launcherPort, "ports", "launcher")
	record.Info = config.SetInfoPath(record.Info, gatewayPort, "ports", "gateway")
	return config.NewInstanceFromRecord(record), nil
}

func (b *picoclawBackend) ReconcileInstance(_ context.Context, cfg *config.Config, inst config.Instance) (config.Instance, bool, error) {
	record := inst.AsRecord()
	changed := false

	if launcherPort, ok := config.GetInstanceInfoInt(inst, "ports", "launcher"); !ok || launcherPort != inst.GetPort() {
		record.Info = config.SetInfoPath(record.Info, inst.GetPort(), "ports", "launcher")
		changed = true
	}

	gatewayPort, ok := config.GetInstanceInfoInt(inst, "ports", "gateway")
	if !ok || gatewayPort <= 0 {
		reserved := collectReservedPortsExcept(cfg, inst.GetName())
		reserved[inst.GetPort()] = struct{}{}
		var err error
		gatewayPort, err = allocatePort(18790, reserved)
		if err != nil {
			return nil, false, err
		}
		record.Info = config.SetInfoPath(record.Info, gatewayPort, "ports", "gateway")
		changed = true
	}

	updatedInst := config.NewInstanceFromRecord(record)
	configChanged, err := b.ensureConfigFile(updatedInst)
	if err != nil {
		return nil, false, err
	}
	if configChanged {
		changed = true
	}
	return updatedInst, changed, nil
}

func (b *picoclawBackend) Repo() string { return "sipeed/picoclaw" }
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-axo", "pid,args")
	out, err := cmd.Output()
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
	_, err := b.ensureConfigFile(inst)
	return err
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
	if token, ok := b.readManualLauncherToken(workDir); ok {
		info = config.SetInfoPath(info, token, "runtime", "dashboard_token")
		return info
	}

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
				info = config.SetInfoPath(info, token, "runtime", "dashboard_token")
			}
			break
		}
	}

	return info
}

func (b *picoclawBackend) readManualLauncherToken(workDir string) (string, bool) {
	configPath := filepath.Join(workDir, "launcher-config.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", false
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", false
	}
	token, ok := cfg["launcher_token"].(string)
	token = strings.TrimSpace(token)
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

func (b *picoclawBackend) ensureConfigFile(inst InstanceInfo) (bool, error) {
	configPath := filepath.Join(inst.GetWorkDir(), "config.json")

	var data map[string]any
	if raw, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			return false, fmt.Errorf("parse config.json: %w", err)
		}
	} else if os.IsNotExist(err) {
		data = map[string]any{
			"version": 2,
			"agents": map[string]any{
				"defaults": map[string]any{
					"workspace":                    filepath.Join(inst.GetWorkDir(), "workspace"),
					"restrict_to_workspace":        true,
					"allow_read_outside_workspace": false,
				},
			},
		}
	} else {
		return false, fmt.Errorf("read config.json: %w", err)
	}

	changed := false

	if version, ok := data["version"].(float64); !ok || int(version) != 2 {
		data["version"] = 2
		changed = true
	}

	agents, ok := data["agents"].(map[string]any)
	if !ok || agents == nil {
		agents = make(map[string]any)
		data["agents"] = agents
		changed = true
	}
	defaults, ok := agents["defaults"].(map[string]any)
	if !ok || defaults == nil {
		defaults = make(map[string]any)
		agents["defaults"] = defaults
		changed = true
	}
	workspacePath := filepath.Join(inst.GetWorkDir(), "workspace")
	if defaults["workspace"] != workspacePath {
		defaults["workspace"] = workspacePath
		changed = true
	}
	if defaults["restrict_to_workspace"] != true {
		defaults["restrict_to_workspace"] = true
		changed = true
	}
	if defaults["allow_read_outside_workspace"] != false {
		defaults["allow_read_outside_workspace"] = false
		changed = true
	}

	gateway, ok := data["gateway"].(map[string]any)
	if !ok || gateway == nil {
		gateway = make(map[string]any)
		data["gateway"] = gateway
		changed = true
	}

	instance, ok := inst.(config.Instance)
	if !ok {
		return false, fmt.Errorf("picoclaw config requires config.Instance")
	}
	gatewayPort, ok := config.GetInstanceInfoInt(instance, "ports", "gateway")
	if !ok || gatewayPort <= 0 {
		return false, fmt.Errorf("picoclaw instance missing gateway port")
	}
	currentGatewayPort, hasGatewayPort := configNumberToInt(gateway["port"])
	if !hasGatewayPort || currentGatewayPort != gatewayPort {
		gateway["port"] = gatewayPort
		changed = true
	}

	if !changed {
		return false, nil
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal config.json: %w", err)
	}
	if err := os.WriteFile(configPath, encoded, 0o644); err != nil {
		return false, fmt.Errorf("write config.json: %w", err)
	}
	return true, nil
}

func configNumberToInt(v any) (int, bool) {
	switch typed := v.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err == nil {
			return int(n), true
		}
	}
	return 0, false
}
