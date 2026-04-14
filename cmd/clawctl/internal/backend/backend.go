package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

// ErrNotSupported is returned when a backend doesn't support an operation.
var ErrNotSupported = errors.New("operation not supported for this backend")

// StatusDetail holds extended status info (port, host, version).
type StatusDetail struct {
	Port    int
	Host    string
	Version string
}

// InstanceInfo describes the properties of an instance that backends operate on.
// Implemented by config.Instance.
type InstanceInfo interface {
	GetClawType() string
	GetWorkDir() string
	GetPort() int
	GetVersion() string
}

// Backend is implemented by each claw-type backend (picoclaw, zeroclaw, openclaw).
type Backend interface {
	// Repo returns the GitHub repository for release downloads (e.g. "sipeed/picoclaw").
	Repo() string

	// BinaryNames returns all binaries shipped in a release for this type.
	BinaryNames() []string

	// GatewayBinary returns the binary used to start the gateway process.
	GatewayBinary() string

	// IsRunning checks if a process is running by matching workDir in process command.
	// Returns (pid, running, error).
	IsRunning(workDir string) (int, bool, error)

	// StatusDetail returns extended status info.
	// Returns nil if not available.
	StatusDetail(workDir string) (*StatusDetail, error)

	// Start launches the gateway process.
	// binaryPath is the resolved path to the gateway binary.
	Start(inst InstanceInfo, binaryPath string) error

	// Stop terminates the gateway process.
	Stop(inst InstanceInfo) error

	// InitWorkDir creates the work_dir basic structure (mkdirs, type file, etc.).
	InitWorkDir(inst InstanceInfo) error

	// ResetWorkspace resets the workspace from templates (if supported).
	// Returns ErrNotSupported if the backend doesn't support template reset.
	ResetWorkspace(inst InstanceInfo) error

	// GatherInfo returns backend-specific instance information (e.g., dashboard token).
	// The returned map can contain arbitrary key-value pairs.
	GatherInfo(workDir string) map[string]any
}

// InstanceConfigurator creates backend-specific instances.
type InstanceConfigurator interface {
	AllocateInstance(ctx context.Context, cfg *config.Config, name string, explicitPort int, version, workDir string) (config.Instance, error)
	ReconcileInstance(ctx context.Context, cfg *config.Config, inst config.Instance) (config.Instance, bool, error)
}

// BackendSpec bundles process management and instance configuration responsibilities.
type BackendSpec struct {
	Backend      Backend
	Configurator InstanceConfigurator
}

// registry holds all registered backend specs by name.
var registry = make(map[string]BackendSpec)

// Register registers a backend with its name. Called by each backend's init().
func Register(name string, b BackendSpec) {
	registry[name] = b
}

// Get returns the Backend for a given type name.
func Get(t string) (Backend, error) {
	spec, ok := registry[t]
	if !ok {
		return nil, fmt.Errorf("unknown backend: %q", t)
	}
	return spec.Backend, nil
}

// GetSpec returns the full BackendSpec for a given type name.
func GetSpec(t string) (BackendSpec, error) {
	spec, ok := registry[t]
	if !ok {
		return BackendSpec{}, fmt.Errorf("unknown backend: %q", t)
	}
	return spec, nil
}

// MustGet returns the Backend for a given type name, panics if not found.
func MustGet(t string) Backend {
	b, err := Get(t)
	if err != nil {
		panic(err)
	}
	return b
}

// MustGetSpec returns the BackendSpec for a given type name, panics if not found.
func MustGetSpec(t string) BackendSpec {
	spec, err := GetSpec(t)
	if err != nil {
		panic(err)
	}
	return spec
}

// KnownTypes returns the list of all registered backend type names.
func KnownTypes() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
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
