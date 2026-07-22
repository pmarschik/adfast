package skill_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/skill"
)

// referenceFiles is the full expected skill tree relative to its root.
var referenceFiles = []string{
	"SKILL.md",
	"references/syntax.md",
	"references/adf-coverage.md",
	"references/example.md",
	"references/pitfalls.md",
}

func TestFiles_ExposesSkillTree(t *testing.T) {
	files := skill.Files()
	for _, name := range referenceFiles {
		if _, err := fs.ReadFile(files, name); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	top, err := fs.ReadFile(files, "SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(top), "---\nname: "+skill.Name+"\n") {
		t.Errorf("SKILL.md must open with its name frontmatter, got %q", firstLines(string(top), 2))
	}
	if !strings.Contains(string(top), "description: ") {
		t.Error("SKILL.md frontmatter must carry a description")
	}
}

func TestInstall_WritesAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := skill.Install(dir); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, skill.Name)
	for _, name := range referenceFiles {
		path := filepath.Join(root, filepath.FromSlash(name))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("not installed: %v", err)
		}
	}

	// A stale file is overwritten in place on re-install.
	stale := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := skill.Install(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "stale" {
		t.Error("Install must overwrite existing files")
	}
}

func TestInstall_RejectsEmptyDir(t *testing.T) {
	if err := skill.Install(""); err == nil {
		t.Error("Install(\"\") must fail")
	}
}

// TestExampleReferenceIsFormatStable runs the embedded worked example
// through the adfast formatter and requires it to be a fixed point, so
// the reference document cannot drift from what adfast actually accepts
// and produces.
func TestExampleReferenceIsFormatStable(t *testing.T) {
	raw, err := fs.ReadFile(skill.Files(), "references/example.md")
	if err != nil {
		t.Fatal(err)
	}
	example := string(raw)
	formatted := adfast.ToMarkdown(adfast.FromMarkdown(example, adfast.WithPrettierFormat()), adfast.WithPrettierFormat())
	if formatted != example {
		t.Errorf("example.md must be format-stable:\ngot:  %q\nwant: %q", formatted, example)
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
