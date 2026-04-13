package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

// ListLocalVersions scans ~/.clawctl/claw_release/<type>/ and returns installed version names.
func ListLocalVersions(clawType string) ([]string, error) {
	dir, err := config.ReleasesDir(clawType)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read release dir: %w", err)
	}
	versions := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	return versions, nil
}

// ResolveVersion resolves a version alias (latest, nightly) to an actual tag name.
func ResolveVersion(repo, version string) (string, error) {
	switch version {
	case "latest":
		return FetchLatestTag(repo)
	case "nightly":
		// Nightly tag is just "nightly" in the releases API
		return "nightly", nil
	default:
		return version, nil
	}
}

// InstallVersion downloads and installs a specific version to ~/.clawctl/claw_release/<type>/<version>/.
func InstallVersion(clawType, version string) error {
	be, err := backend.Get(clawType)
	if err != nil {
		return err
	}

	// Resolve version alias.
	tag, err := ResolveVersion(be.Repo(), version)
	if err != nil {
		return fmt.Errorf("resolve version %q: %w", version, err)
	}

	// Check if already installed.
	localVersions, _ := ListLocalVersions(clawType)
	for _, v := range localVersions {
		if v == tag {
			fmt.Printf("Version %s is already installed at ~/.clawctl/claw_release/%s/%s/\n", tag, clawType, tag)
			return nil
		}
	}

	// Build release API URL for this specific tag.
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", be.Repo(), tag)

	// Download and extract.
	destDir, err := DownloadAndExtractRelease(releaseURL, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.RemoveAll(destDir)

	// Move to final location.
	installDir, err := config.ReleasesDir(clawType)
	if err != nil {
		return err
	}
	finalPath := filepath.Join(installDir, tag)
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	// Move files from extracted dir to finalPath, filtering by BinaryNames.
	if err := moveAndFilterBinaries(destDir, finalPath, be.BinaryNames()); err != nil {
		return fmt.Errorf("install binaries: %w", err)
	}

	fmt.Printf("Installed %s %s to ~/.clawctl/claw_release/%s/%s/\n", clawType, tag, clawType, tag)
	return nil
}

// moveAndFilterBinaries moves files from srcDir to destDir, keeping only binaries in keepNames.
func moveAndFilterBinaries(srcDir, destDir string, keepNames []string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Check if this binary should be kept.
		keep := false
		for _, kn := range keepNames {
			if name == kn {
				keep = true
				break
			}
		}
		if !keep {
			// Skip this file (it's an extra binary not needed).
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destDir, name), data, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// UninstallVersion removes a version directory.
func UninstallVersion(clawType, version string) error {
	installDir, err := config.ReleasesDir(clawType)
	if err != nil {
		return err
	}
	versionPath := filepath.Join(installDir, version)
	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		return fmt.Errorf("version %q is not installed", version)
	}
	if err := os.RemoveAll(versionPath); err != nil {
		return fmt.Errorf("remove version dir: %w", err)
	}
	fmt.Printf("Uninstalled %s %s\n", clawType, version)
	return nil
}

// VersionBinaryPath returns the full path to a specific binary for a version.
// Returns "" if the binary is not found.
func VersionBinaryPath(clawType, version, binaryName string) (string, error) {
	installDir, err := config.ReleasesDir(clawType)
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(installDir, version, binaryName)
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return "", fmt.Errorf("binary %s not found for version %s", binaryName, version)
	}
	return binPath, nil
}
