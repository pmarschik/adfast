package adfast

import (
	"strings"
	"testing"
)

// TestAtlaskitCodeLanguages_Shape guards the transcription of the 169
// literals moved out of jira/languages.go: the one step where a
// copy/paste can quietly lose a line.
func TestAtlaskitCodeLanguages_Shape(t *testing.T) {
	const want = 169
	if len(AtlaskitCodeLanguages) != want {
		t.Fatalf("len(AtlaskitCodeLanguages) = %d, want %d", len(AtlaskitCodeLanguages), want)
	}

	seen := make(map[string]bool, len(AtlaskitCodeLanguages))
	for _, lang := range AtlaskitCodeLanguages {
		if seen[lang] {
			t.Errorf("duplicate entry %q", lang)
		}
		seen[lang] = true
		if lang != strings.ToLower(lang) {
			t.Errorf("entry %q is not lowercase", lang)
		}
	}

	for _, want := range []string{"go", "json", "kotlin", "rust", "typescript", "yaml", "shell", "none"} {
		if !seen[want] {
			t.Errorf("expected %q in AtlaskitCodeLanguages", want)
		}
	}
}
