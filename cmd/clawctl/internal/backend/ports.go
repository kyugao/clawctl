package backend

import (
	"fmt"
	"net"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func isPortAvailable(port int) bool {
	if port <= 0 {
		return false
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func collectReservedPorts(cfg *config.Config) map[int]struct{} {
	reserved := make(map[int]struct{})
	if cfg == nil {
		return reserved
	}
	for _, inst := range cfg.Instances {
		if port := inst.GetPort(); port > 0 {
			reserved[port] = struct{}{}
		}
		if gatewayPort, ok := config.GetInstanceInfoInt(inst, "ports", "gateway"); ok && gatewayPort > 0 {
			reserved[gatewayPort] = struct{}{}
		}
	}
	return reserved
}

func collectReservedPortsExcept(cfg *config.Config, instanceName string) map[int]struct{} {
	reserved := make(map[int]struct{})
	if cfg == nil {
		return reserved
	}
	for name, inst := range cfg.Instances {
		if name == instanceName {
			continue
		}
		if port := inst.GetPort(); port > 0 {
			reserved[port] = struct{}{}
		}
		if gatewayPort, ok := config.GetInstanceInfoInt(inst, "ports", "gateway"); ok && gatewayPort > 0 {
			reserved[gatewayPort] = struct{}{}
		}
	}
	return reserved
}

func allocatePort(start int, reserved map[int]struct{}) (int, error) {
	if start <= 0 {
		start = 1024
	}
	for port := start; port < 65535; port++ {
		if _, exists := reserved[port]; exists {
			continue
		}
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found starting from %d", start)
}
