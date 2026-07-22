package frontmatter

import (
	"strings"
	"testing"
)

func TestPatchPreservingKeepsOrderCommentsAndQuoting(t *testing.T) {
	block := strings.Join([]string{
		"---",
		"# top comment",
		"title: 'My Story'   # inline note",
		"zeta: last",
		"status: Open",
		"---",
	}, "\n")

	got, err := PatchPreserving(block, map[string]any{"status": "Done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Head comment preserved.
	if !strings.Contains(got, "# top comment") {
		t.Errorf("head comment lost:\n%s", got)
	}
	// Inline comment on an untouched key preserved.
	if !strings.Contains(got, "# inline note") {
		t.Errorf("inline comment lost:\n%s", got)
	}
	// Single-quote style on an untouched key preserved.
	if !strings.Contains(got, "title: 'My Story'") {
		t.Errorf("quoting style lost:\n%s", got)
	}
	// Author order (title, zeta, status) preserved — status stays last,
	// unlike alphabetical Render.
	iTitle := strings.Index(got, "title:")
	iZeta := strings.Index(got, "zeta:")
	iStatus := strings.Index(got, "status:")
	if iTitle >= iZeta || iZeta >= iStatus {
		t.Errorf("author order not preserved:\n%s", got)
	}
	// The changed value is updated.
	if !strings.Contains(got, "status: Done") {
		t.Errorf("status not updated:\n%s", got)
	}
}

func TestPatchPreservingAddDelete(t *testing.T) {
	block := "---\na: 1\nb: 2\nc: 3\n---"

	got, err := PatchPreserving(block, map[string]any{
		"b":     nil,   // delete
		"added": "new", // append
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "b: 2") {
		t.Errorf("key b not deleted:\n%s", got)
	}
	if !strings.Contains(got, "a: 1") || !strings.Contains(got, "c: 3") {
		t.Errorf("untouched keys altered:\n%s", got)
	}
	if !strings.Contains(got, "added: new") {
		t.Errorf("new key not appended:\n%s", got)
	}
	// Appended after existing keys.
	if strings.Index(got, "added:") < strings.Index(got, "c: 3") {
		t.Errorf("new key not appended after existing:\n%s", got)
	}
}

func TestPatchPreservingFromEmpty(t *testing.T) {
	got, err := PatchPreserving("", map[string]any{"status": "Open"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "---\nstatus: Open\n---" {
		t.Errorf("unexpected block from empty input:\n%s", got)
	}
}

func TestPatchPreservingDeleteAllYieldsEmpty(t *testing.T) {
	got, err := PatchPreserving("---\nonly: 1\n---", map[string]any{"only": nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty block, got:\n%q", got)
	}
}
