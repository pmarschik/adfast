package adfast

import (
	"slices"
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

// TestAtlaskitCodeLanguageGroups_Shape guards the grouped literal both
// derived values are built from: 86 groups (upstream's 85 plus the
// picker-only "none"), each non-empty, each alias lowercase and
// mentioned exactly once across the whole list.
func TestAtlaskitCodeLanguageGroups_Shape(t *testing.T) {
	const wantGroups = 86
	if len(atlaskitCodeLanguageGroups) != wantGroups {
		t.Fatalf("len(atlaskitCodeLanguageGroups) = %d, want %d", len(atlaskitCodeLanguageGroups), wantGroups)
	}

	seen := make(map[string]bool)
	for i, group := range atlaskitCodeLanguageGroups {
		if len(group) == 0 {
			t.Fatalf("group %d is empty", i)
		}
		for _, alias := range group {
			if alias != strings.ToLower(alias) {
				t.Errorf("alias %q in group %q is not lowercase", alias, group[0])
			}
			if seen[alias] {
				t.Errorf("alias %q appears in more than one group", alias)
			}
			seen[alias] = true
		}
	}
}

// TestAtlaskitCodeLanguageAliases_CoversTheList is the mutual-coverage
// pin between the two derived values: the alias map's keys and the flat
// accept set must name exactly the same identifiers, and every canonical
// value must itself be one of them. Without this, a group edit could
// grow one and not the other.
func TestAtlaskitCodeLanguageAliases_CoversTheList(t *testing.T) {
	if len(AtlaskitCodeLanguageAliases) != len(AtlaskitCodeLanguages) {
		t.Errorf("len(AtlaskitCodeLanguageAliases) = %d, len(AtlaskitCodeLanguages) = %d; want equal",
			len(AtlaskitCodeLanguageAliases), len(AtlaskitCodeLanguages))
	}
	for _, lang := range AtlaskitCodeLanguages {
		canonical, ok := AtlaskitCodeLanguageAliases[lang]
		if !ok {
			t.Errorf("AtlaskitCodeLanguageAliases has no entry for %q", lang)
			continue
		}
		if _, ok := AtlaskitCodeLanguageAliases[canonical]; !ok {
			t.Errorf("canonical %q (for %q) is not itself an accepted identifier", canonical, lang)
		}
	}
	for alias := range AtlaskitCodeLanguageAliases {
		if !slices.Contains(AtlaskitCodeLanguages, alias) {
			t.Errorf("alias %q is not in AtlaskitCodeLanguages", alias)
		}
	}
}

// TestAtlaskitCodeLanguageAliases_Idempotent pins the property the
// option leans on: a canonical identifier maps to itself, so applying
// the map twice is applying it once.
func TestAtlaskitCodeLanguageAliases_Idempotent(t *testing.T) {
	for alias, canonical := range AtlaskitCodeLanguageAliases {
		if again := AtlaskitCodeLanguageAliases[canonical]; again != canonical {
			t.Errorf("%q → %q → %q: canonicalization is not idempotent", alias, canonical, again)
		}
	}
}

// TestAtlaskitCodeLanguageAliases_UpstreamSpellings pins the specific
// answers transcribed from the pinned upstream file — the first alias of
// each group is what getLanguageIdentifier returns, so these are the
// spellings the editor stores.
func TestAtlaskitCodeLanguageAliases_UpstreamSpellings(t *testing.T) {
	for alias, want := range map[string]string{
		"bash":       "shell",
		"sh":         "shell",
		"ksh":        "shell",
		"zsh":        "shell",
		"shell":      "shell",
		"js":         "javascript",
		"ts":         "typescript",
		"py":         "python",
		"yml":        "yaml",
		"rb":         "ruby",
		"cpp":        "c++",
		"c#":         "csharp",
		"dockerfile": "docker",
		"terraform":  "hcl",
		"mysql":      "sql",
		"delphi":     "pas",
		"plaintext":  "text",
		"sml":        "standardml",
		"none":       "none",
		"go":         "go",
	} {
		if got := AtlaskitCodeLanguageAliases[alias]; got != want {
			t.Errorf("AtlaskitCodeLanguageAliases[%q] = %q, want %q", alias, got, want)
		}
	}

	// Not an atlaskit identifier at all: absent, so a caller leaves it alone.
	if got, ok := AtlaskitCodeLanguageAliases["mermaid"]; ok {
		t.Errorf("AtlaskitCodeLanguageAliases[%q] = %q, want absent", "mermaid", got)
	}
}
