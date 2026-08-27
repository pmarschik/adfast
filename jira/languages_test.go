package jira

import (
	"maps"
	"slices"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// TestCodeLanguages_MatchesAtlaskitList proves the move of the language
// list into the root module did not alter Jira's set: same length, same
// order, same values.
func TestCodeLanguages_MatchesAtlaskitList(t *testing.T) {
	if !slices.Equal(CodeLanguages, adfast.AtlaskitCodeLanguages) {
		t.Fatalf("jira.CodeLanguages diverges from adfast.AtlaskitCodeLanguages:\ngot:  %v\nwant: %v",
			CodeLanguages, adfast.AtlaskitCodeLanguages)
	}
}

// TestCodeLanguageAliases_MatchesAtlaskitMap pins the same derivation for
// the canonicalization map: Jira's picker IS the atlaskit one, so the
// clone must agree entry for entry.
func TestCodeLanguageAliases_MatchesAtlaskitMap(t *testing.T) {
	if !maps.Equal(CodeLanguageAliases, adfast.AtlaskitCodeLanguageAliases) {
		t.Fatalf("jira.CodeLanguageAliases diverges from adfast.AtlaskitCodeLanguageAliases")
	}
	if got := CodeLanguageAliases["bash"]; got != "shell" {
		t.Errorf(`CodeLanguageAliases["bash"] = %q, want "shell"`, got)
	}
}

// TestCodeLanguageAliases_PushesCanonicalLanguage is the acceptance case
// through the Jira bundle: a ```bash fence pushes ADF language "shell"
// once the caller opts in, and stays "bash" under the bare bundle.
func TestCodeLanguageAliases_PushesCanonicalLanguage(t *testing.T) {
	const md = "```bash\necho hi\n```\n"

	base := MarkdownOptions("https://example.atlassian.net", ExpandExplicit)
	if got := codeBlockLanguage(t, md, base); got != "bash" {
		t.Errorf("MarkdownOptions alone: language = %q, want %q", got, "bash")
	}

	opts := append(slices.Clone(base), adfast.WithCanonicalCodeLanguages(CodeLanguageAliases))
	if got := codeBlockLanguage(t, md, opts); got != "shell" {
		t.Errorf("with WithCanonicalCodeLanguages: language = %q, want %q", got, "shell")
	}
}

// codeBlockLanguage encodes md and returns the language of its single
// code block.
func codeBlockLanguage(t *testing.T, md string, opts []adfast.Option) string {
	t.Helper()
	doc := adfast.ToADF(adfast.FromMarkdown(md, opts...), opts...)
	for _, top := range doc.Content {
		for n := range adf.Walk(top) {
			if cb, ok := n.(*adf.CodeBlock); ok {
				return cb.Language
			}
		}
	}
	t.Fatalf("no code block in the encoded document")
	return ""
}
