package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallHermesVersion installs a specific version of hermes-agent from GitHub.
// It clones the repository, checks out the tag, creates a venv, and installs the package.
func InstallHermesVersion(version string) error {
	// Resolve version (latest -> actual tag)
	tag := version
	if version == "latest" {
		var err error
		tag, err = FetchLatestTag("NousResearch/hermes-agent")
		if err != nil {
			return fmt.Errorf("resolve latest version: %w", err)
		}
	}

	installDir, err := getHermesInstallDir()
	if err != nil {
		return err
	}

	versionPath := filepath.Join(installDir, tag)
	if _, err := os.Stat(versionPath); err == nil {
		fmt.Printf("Version %s is already installed at ~/.clawctl/claw_release/hermes/%s/\n", tag, tag)
		return nil
	}

	fmt.Printf("Installing hermes-agent %s...\n", tag)

	// Create temp directory for clone
	tmpDir, err := os.MkdirTemp("", "hermes-clone-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up proxy environment for git commands
	proxyEnv := os.Environ()
	// Check for common proxy env vars and apply them
	if httpProxy := os.Getenv("https_proxy"); httpProxy != "" {
		proxyEnv = append(proxyEnv, "https_proxy="+httpProxy)
	} else if httpProxy := os.Getenv("HTTPS_PROXY"); httpProxy != "" {
		proxyEnv = append(proxyEnv, "HTTPS_PROXY="+httpProxy)
	}
	if httpProxy := os.Getenv("http_proxy"); httpProxy != "" {
		proxyEnv = append(proxyEnv, "http_proxy="+httpProxy)
	} else if httpProxy := os.Getenv("HTTP_PROXY"); httpProxy != "" {
		proxyEnv = append(proxyEnv, "HTTP_PROXY="+httpProxy)
	}
	if allProxy := os.Getenv("all_proxy"); allProxy != "" {
		proxyEnv = append(proxyEnv, "all_proxy="+allProxy)
	} else if allProxy := os.Getenv("ALL_PROXY"); allProxy != "" {
		proxyEnv = append(proxyEnv, "ALL_PROXY="+allProxy)
	}

	// Clone the repository
	fmt.Printf("Cloning hermes-agent repository...\n")
	cloneCmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/NousResearch/hermes-agent.git", ".")
	cloneCmd.Dir = tmpDir
	cloneCmd.Env = proxyEnv
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Fetch all tags to get the version tag
	fetchCmd := exec.Command("git", "fetch", "--tags", "origin", "tag", tag)
	fetchCmd.Dir = tmpDir
	fetchCmd.Env = proxyEnv
	if err := fetchCmd.Run(); err != nil {
		// If fetch tags fails, try to get the tag differently
		fmt.Printf("Warning: failed to fetch tag %s: %v\n", tag, err)
	}

	// Checkout the specific tag
	checkoutCmd := exec.Command("git", "checkout", tag)
	checkoutCmd.Dir = tmpDir
	checkoutCmd.Env = proxyEnv
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("git checkout %s: %w", tag, err)
	}

	// Create version directory
	if err := os.MkdirAll(versionPath, 0o755); err != nil {
		return fmt.Errorf("create version dir: %w", err)
	}

	// Create venv in the version directory
	venvPath := filepath.Join(versionPath, "venv")
	fmt.Printf("Creating virtual environment...\n")
	venvCmd := exec.Command("python3", "-m", "venv", venvPath)
	if err := venvCmd.Run(); err != nil {
		return fmt.Errorf("create venv: %w", err)
	}

	// Install the package using uv or pip
	// NOTE: we copy source to versionPath first because editable installs (-e)
	// reference the original directory which gets deleted.
	fmt.Printf("Installing hermes-agent...\n")

	// Copy source to versionPath for installation
	srcPath := filepath.Join(versionPath, "src")
	if err := copyDir(tmpDir, srcPath); err != nil {
		return fmt.Errorf("copy source: %w", err)
	}

	// Try uv first
	uvCmd := exec.Command(filepath.Join(venvPath, "bin", "uv"), "pip", "install", ".")
	uvCmd.Dir = srcPath
	uvCmd.Env = proxyEnv
	err = uvCmd.Run()
	if err != nil {
		// Fallback to pip
		pipCmd := exec.Command(filepath.Join(venvPath, "bin", "pip"), "install", ".")
		pipCmd.Dir = srcPath
		pipCmd.Env = proxyEnv
		err = pipCmd.Run()
		if err != nil {
			return fmt.Errorf("pip install: %w", err)
		}
	}

	// Create wrapper script
	wrapperPath := filepath.Join(versionPath, "hermes")
	srcPath = filepath.Join(versionPath, "src")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
# Hermes Agent Wrapper Script
# Version: %s
export HERMES_HOME="${HERMES_HOME:-%s}"
export PYTHONPATH="%s:${PYTHONPATH:-}"
exec "%s" "$@"
`, tag, versionPath, srcPath, filepath.Join(venvPath, "bin", "hermes"))
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0o755); err != nil {
		return fmt.Errorf("write wrapper: %w", err)
	}

	fmt.Printf("Installed hermes-agent %s to ~/.clawctl/claw_release/hermes/%s/\n", tag, tag)
	return nil
}

// copyDir copies a directory from src to dest recursively.
func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, info.Mode())
	})
}

// getHermesInstallDir returns the directory where hermes versions are installed.
func getHermesInstallDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".clawctl", "claw_release", "hermes"), nil
}

// FindHermesBinary finds the hermes binary path for a given version.
func FindHermesBinary(version string) (string, error) {
	installDir, err := getHermesInstallDir()
	if err != nil {
		return "", err
	}
	versionPath := filepath.Join(installDir, version)
	hermesPath := filepath.Join(versionPath, "hermes")
	if _, err := os.Stat(hermesPath); os.IsNotExist(err) {
		return "", fmt.Errorf("hermes not found for version %s", version)
	}
	return hermesPath, nil
}

// ListHermesVersions returns all installed hermes versions.
func ListHermesVersions() ([]string, error) {
	installDir, err := getHermesInstallDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	versions := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			// Check if it's a valid hermes installation
			hermesPath := filepath.Join(installDir, e.Name(), "hermes")
			if _, err := os.Stat(hermesPath); err == nil {
				versions = append(versions, e.Name())
			}
		}
	}
	return versions, nil
}

// UninstallHermesVersion removes a hermes version.
func UninstallHermesVersion(version string) error {
	installDir, err := getHermesInstallDir()
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
	fmt.Printf("Uninstalled hermes-agent %s\n", version)
	return nil
}

// HasHermesBinary checks if a hermes binary exists for the given version.
func HasHermesBinary(version string) bool {
	path, err := FindHermesBinary(version)
	return err == nil && path != ""
}
