package backend

import (
	"context"
	"fmt"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func init() {
	b := &openclawBackend{}
	Register("openclaw", BackendSpec{Backend: b, Configurator: b})
}

type openclawBackend struct{}

func (b *openclawBackend) AllocateInstance(_ context.Context, cfg *config.Config, name string, explicitPort int, version, workDir string) (config.Instance, error) {
	port := explicitPort
	if port == 0 {
		var err error
		port, err = allocatePort(18791, collectReservedPorts(cfg))
		if err != nil {
			return nil, err
		}
	} else if !isPortAvailable(port) {
		return nil, fmt.Errorf("port %d is already in use", port)
	}
	return config.NewInstance("openclaw", name, port, version, workDir), nil
}

func (b *openclawBackend) ReconcileInstance(_ context.Context, _ *config.Config, inst config.Instance) (config.Instance, bool, error) {
	return inst, false, nil
}

func (b *openclawBackend) Repo() string { return "sipeed/openclaw" }
func (b *openclawBackend) BinaryNames() []string {
	return []string{"openclaw", "openclaw-launcher"}
}
func (b *openclawBackend) GatewayBinary() string { return "openclaw-launcher" }

func (b *openclawBackend) IsRunning(workDir string) (int, bool, error) {
	return 0, false, nil
}

func (b *openclawBackend) StatusDetail(workDir string) (*StatusDetail, error) {
	return nil, fmt.Errorf("openclaw not implemented")
}

func (b *openclawBackend) InitWorkDir(inst InstanceInfo) error {
	return fmt.Errorf("openclaw support not yet implemented")
}

func (b *openclawBackend) Start(_ InstanceInfo, _ string) error {
	return fmt.Errorf("openclaw support not yet implemented")
}

func (b *openclawBackend) Stop(_ InstanceInfo) error {
	return fmt.Errorf("openclaw support not yet implemented")
}

func (b *openclawBackend) ResetWorkspace(_ InstanceInfo) error {
	return ErrNotSupported
}

func (b *openclawBackend) GatherInfo(workDir string) map[string]any {
	return nil
}
