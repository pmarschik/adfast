// Package skill ships the adfast markdown dialect as an installable
// agent skill: a SKILL.md entry point plus reference documents that
// teach AI coding agents to read and write markdown destined for
// Jira/Confluence via adfast.
//
// Hosts embed the content directly (Files) or materialize it into
// their agent-skills directory (Install), e.g.:
//
//	skill.Install(filepath.Join(projectDir, ".claude", "skills"))
//
// which creates .claude/skills/adfast-markdown/SKILL.md and its
// references/ tree.
package skill

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed assets
var assets embed.FS

// Name is the skill's canonical directory name; Install materializes
// the content under it.
const Name = "adfast-markdown"

// Files returns the skill content rooted at the skill directory:
// SKILL.md at the top, the full material under references/.
func Files() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		// The embedded tree always contains "assets"; failing to root
		// there is a build defect, not a runtime condition.
		panic(fmt.Sprintf("skill: embedded assets missing: %v", err))
	}
	return sub
}

// Install materializes the skill into dir/adfast-markdown, creating
// directories as needed and overwriting existing files (so hosts can
// refresh an installed skill in place). dir is the host's agent-skills
// directory (e.g. .claude/skills) and must be non-empty. Files are
// written owner-only (0600/0750, before umask): agent skills are read
// by the same user the agent runs as.
func Install(dir string) error {
	if dir == "" {
		return errors.New("skill: install dir must not be empty")
	}
	root := filepath.Join(dir, Name)
	content := Files()
	return fs.WalkDir(content, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := fs.ReadFile(content, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}
