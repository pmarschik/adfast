package confluence

import (
	"maps"
	"slices"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// encodeDiagnostics encodes md under MarkdownOptions("") and returns the
// diagnostics reported.
func encodeDiagnostics(t *testing.T, md string) []convert.Diagnostic {
	t.Helper()
	var diags []convert.Diagnostic
	opts := append(MarkdownOptions(""),
		adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))
	adfast.ToADF(adfast.FromMarkdown(md, opts...), opts...)
	return diags
}

// TestCodeLanguages_ADFPathLanguages pins the 2026-08-25 measurement
// against ixolit.atlassian.net page 1190100993: the Confluence ADF path
// accepts the atlaskit editor's language set, not just the legacy code
// block macro's. "go" and "json" were observed directly on that page;
// the rest ride the same picker.
func TestCodeLanguages_ADFPathLanguages(t *testing.T) {
	for _, lang := range []string{"go", "json", "kotlin", "rust", "typescript", "yaml", "shell"} {
		t.Run(lang, func(t *testing.T) {
			diags := encodeDiagnostics(t, "```"+lang+"\nx\n```\n")
			if len(diags) != 0 {
				t.Errorf("language %q must not report on the Confluence ADF path: %+v", lang, diags)
			}
		})
	}
}

// TestCodeLanguages_DerivesFromAtlaskitList locks the derivation in both
// directions: every atlaskit entry must be present (superset), and the
// only extra entries must be exactly the two legacy-macro-only
// spellings — so a future edit to either side is caught.
func TestCodeLanguages_DerivesFromAtlaskitList(t *testing.T) {
	for _, lang := range adfast.AtlaskitCodeLanguages {
		if !slices.Contains(CodeLanguages, lang) {
			t.Errorf("confluence.CodeLanguages is missing atlaskit entry %q", lang)
		}
	}

	atlaskit := make(map[string]bool, len(adfast.AtlaskitCodeLanguages))
	for _, lang := range adfast.AtlaskitCodeLanguages {
		atlaskit[lang] = true
	}
	var extra []string
	for _, lang := range CodeLanguages {
		if !atlaskit[lang] {
			extra = append(extra, lang)
		}
	}
	slices.Sort(extra)
	want := []string{"html/xml", "vb"}
	if !slices.Equal(extra, want) {
		t.Errorf("confluence.CodeLanguages \\ adfast.AtlaskitCodeLanguages = %v, want %v", extra, want)
	}
}

// TestCodeLanguages_KeepsLegacyMacroSpellings is the "macro set
// survives" pin: the two legacy code block macro spellings the atlaskit
// list does not carry must still pass without a diagnostic.
func TestCodeLanguages_KeepsLegacyMacroSpellings(t *testing.T) {
	if !slices.Contains(CodeLanguages, "html/xml") {
		t.Error(`confluence.CodeLanguages must contain "html/xml"`)
	}
	if !slices.Contains(CodeLanguages, "vb") {
		t.Error(`confluence.CodeLanguages must contain "vb"`)
	}

	diags := encodeDiagnostics(t, "```vb\nDim x\n```\n")
	if len(diags) != 0 {
		t.Errorf("legacy macro spelling %q must not report: %+v", "vb", diags)
	}
}

// TestCodeLanguageAliases_MatchesAtlaskitMap locks the canonicalization
// map to the shared atlaskit one — the code snippet element uses that
// picker — and pins the deliberate exclusion of the two legacy macro
// spellings, which have no picker entry to canonicalize to.
func TestCodeLanguageAliases_MatchesAtlaskitMap(t *testing.T) {
	if !maps.Equal(CodeLanguageAliases, adfast.AtlaskitCodeLanguageAliases) {
		t.Fatal("confluence.CodeLanguageAliases diverges from adfast.AtlaskitCodeLanguageAliases")
	}
	for _, lang := range legacyMacroOnlyLanguages {
		if got, ok := CodeLanguageAliases[lang]; ok {
			t.Errorf("legacy macro spelling %q must have no canonical entry, got %q", lang, got)
		}
	}
}

// TestCodeLanguageAliases_EncodesCanonicalLanguage is the acceptance case
// through the Confluence bundle, plus the pass-through it must not
// break: a ```bash fence encodes as "shell" once the caller opts in,
// while a legacy macro spelling keeps its own text.
func TestCodeLanguageAliases_EncodesCanonicalLanguage(t *testing.T) {
	opts := append(MarkdownOptions(""), adfast.WithCanonicalCodeLanguages(CodeLanguageAliases))
	for _, tc := range []struct{ fence, want string }{
		{"bash", "shell"},
		{"vb", "vb"},
		{"html/xml", "html/xml"},
	} {
		doc := adfast.ToADF(adfast.FromMarkdown("```"+tc.fence+"\nx\n```\n", opts...), opts...)
		got := ""
		for _, top := range doc.Content {
			for n := range adf.Walk(top) {
				if cb, ok := n.(*adf.CodeBlock); ok {
					got = cb.Language
				}
			}
		}
		if got != tc.want {
			t.Errorf("fence %q encoded language %q, want %q", tc.fence, got, tc.want)
		}
	}
}
