package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

// GatewayRunner starts and manages a gateway process for a Claw instance.
type GatewayRunner struct {
	Instance   config.Instance
	Backend    backend.Backend
	BinaryPath string
}

// NewGatewayRunner creates a runner for the given instance.
func NewGatewayRunner(inst config.Instance) (*GatewayRunner, error) {
	be := backend.MustGet(inst.GetClawType())

	// Resolve version tag (latest → actual, nightly → actual).
	version := inst.GetVersion()
	if version == "latest" || version == "nightly" {
		tag, err := FetchLatestTag(be.Repo())
		if err != nil {
			return nil, fmt.Errorf("resolve %s version: %w", version, err)
		}
		version = tag
	}

	binPath, err := VersionBinaryPath(inst.GetClawType(), version, be.GatewayBinary())
	if err != nil {
		return nil, fmt.Errorf("find binary: %w", err)
	}

	return &GatewayRunner{
		Instance:   inst,
		Backend:    be,
		BinaryPath: binPath,
	}, nil
}

// Status returns the running status and PID for the instance.
func Status(inst config.Instance) (running bool, pidData *backend.PidData, err error) {
	be := backend.MustGet(inst.GetClawType())
	p, running, err := be.IsRunning(inst.GetWorkDir())
	if err != nil || !running {
		return running, nil, err
	}
	// Try to get extended status detail (port, host, version).
	pidData = &backend.PidData{PID: p}
	if detail, err := be.StatusDetail(inst.GetWorkDir()); err == nil {
		pidData.Port = detail.Port
		pidData.Host = detail.Host
		pidData.Version = detail.Version
	}
	return running, pidData, nil
}

// Start launches the gateway process for the instance.
func (r *GatewayRunner) Start() error {
	if err := r.Backend.Start(r.Instance, r.BinaryPath); err != nil {
		return err
	}
	if info := r.Backend.GatherInfo(r.Instance.GetWorkDir()); len(info) > 0 {
		if err := config.UpdateInstanceInfo(r.Instance.GetName(), info); err != nil {
			return fmt.Errorf("update instance info: %w", err)
		}
	}
	return nil
}

// Stop terminates the gateway process for the instance.
func Stop(inst config.Instance) error {
	be := backend.MustGet(inst.GetClawType())
	return be.Stop(inst)
}

// ReconcileInstanceForStart prepares an instance before start/restart.
func ReconcileInstanceForStart(ctx context.Context, cfg *config.Config, inst config.Instance) (config.Instance, error) {
	spec := backend.MustGetSpec(inst.GetClawType())
	updated, changed, err := spec.Configurator.ReconcileInstance(ctx, cfg, inst)
	if err != nil {
		return nil, err
	}
	if !changed {
		return updated, nil
	}
	cfg.Instances[inst.GetName()] = updated
	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("save reconciled config: %w", err)
	}
	return updated, nil
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
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.EPERM
}

// PIDFilePath returns the path to the PID file for picoclaw (for backward compat).
func PIDFilePath(workDir string) string {
	return filepath.Join(workDir, ".picoclaw.pid")
}
