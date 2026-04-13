package onboard

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:generate cp -r ../../../../workspace .
//go:embed workspace
var embeddedFiles embed.FS

// CopyWorkspaceTemplates copies the embedded workspace templates to targetDir.
// It creates targetDir/workspace/ and targetDir/skills/ and populates them
// with all template files from the embedded workspace.
func CopyWorkspaceTemplates(targetDir string) error {
	workspaceDir := filepath.Join(targetDir, "workspace")
	skillsDir := filepath.Join(targetDir, "skills")

	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}

	return fs.WalkDir(embeddedFiles, "workspace", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Strip the "workspace/" prefix to get the relative path inside the workspace.
		rel := strings.TrimPrefix(path, "workspace/")
		if rel == path {
			return nil // shouldn't happen
		}

		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return err
		}

		var targetPath string
		if strings.HasPrefix(rel, "skills/") {
			// Strip "skills/" prefix → skills/agent-browser/SKILL.md → agent-browser/SKILL.md
			inner := strings.TrimPrefix(rel, "skills/")
			targetPath = filepath.Join(skillsDir, inner)
		} else {
			targetPath = filepath.Join(workspaceDir, rel)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	})
}
