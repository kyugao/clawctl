package backend

import (
	"fmt"
)

func init() {
	Register("openclaw", &openclawBackend{})
}

type openclawBackend struct{}

func (b *openclawBackend) Repo() string    { return "sipeed/openclaw" }
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
