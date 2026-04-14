package manager

import (
	"fmt"
	"strings"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func instancePortSummary(inst config.Instance) string {
	parts := []string{fmt.Sprintf("%d", inst.GetPort())}
	if gatewayPort, ok := config.GetInstanceInfoInt(inst, "ports", "gateway"); ok && gatewayPort > 0 && gatewayPort != inst.GetPort() {
		parts = append(parts, fmt.Sprintf("gw:%d", gatewayPort))
	}
	return strings.Join(parts, " ")
}

func instanceDetailLines(inst config.Instance) []string {
	lines := make([]string, 0, 3)
	if launcherPort, ok := config.GetInstanceInfoInt(inst, "ports", "launcher"); ok {
		lines = append(lines, fmt.Sprintf("  Launcher Port: %d", launcherPort))
	}
	if gatewayPort, ok := config.GetInstanceInfoInt(inst, "ports", "gateway"); ok {
		lines = append(lines, fmt.Sprintf("  Gateway Port:  %d", gatewayPort))
	}
	if token, ok := config.GetInstanceInfoString(inst, "runtime", "dashboard_token"); ok && token != "" {
		lines = append(lines, fmt.Sprintf("  Dashboard Token: %s", token))
	}
	return lines
}
