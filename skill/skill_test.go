package skill_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/convert"
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

// TestPitfallsReferenceListsEveryDiagnosticCode keys the reference's
// diagnostic section on convert.Codes() rather than on a second
// hand-kept list. The section presents itself as "the convert.Code*
// vocabulary", so a code missing from it is a false claim of
// completeness — and that is exactly how it drifted before: it had
// fallen two codes behind (link-destination-dropped,
// unused-definition-dropped) while reading as exhaustive. Codes() is
// build-enforced against the constant block, so this test inherits that
// guarantee and a new code now fails here until the prose explains it.
func TestPitfallsReferenceListsEveryDiagnosticCode(t *testing.T) {
	raw, err := fs.ReadFile(skill.Files(), "references/pitfalls.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)

	all := convert.Codes()
	// The inventory has to be non-empty, or "every code is documented"
	// would be vacuously true.
	if len(all) == 0 {
		t.Fatal("convert.Codes() is empty; the check below would prove nothing")
	}

	// A documented code is one that heads a list item: "- `code` — …",
	// possibly sharing the item with its siblings ("- `a` / `b` — …").
	// Matching the HEAD of the item rather than the whole document is
	// what keeps a passing mention in unrelated prose from counting.
	heads := map[string]bool{}
	for line := range strings.SplitSeq(doc, "\n") {
		item, ok := strings.CutPrefix(line, "- ")
		if !ok {
			continue
		}
		head, _, found := strings.Cut(item, " — ")
		if !found {
			continue
		}
		for part := range strings.SplitSeq(head, "/") {
			if code := strings.Trim(strings.TrimSpace(part), "`"); code != "" {
				heads[code] = true
			}
		}
	}
	// The parse has to have found the section at all.
	if len(heads) == 0 {
		t.Fatal("no \"- `code` — …\" list items found in references/pitfalls.md; the check below would prove nothing")
	}

	var missing []string
	for _, code := range all {
		if !heads[code] {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		t.Errorf("references/pitfalls.md documents %d of %d diagnostic codes; missing %v",
			len(all)-len(missing), len(all), missing)
	}
}
