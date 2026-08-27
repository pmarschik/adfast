package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// codeBlockLanguages walks a document and collects the language of every
// codeBlock, in document order.
func codeBlockLanguages(t *testing.T, doc adf.Doc) []string {
	t.Helper()
	var out []string
	for _, top := range doc.Content {
		for n := range adf.Walk(top) {
			if cb, ok := n.(*adf.CodeBlock); ok {
				out = append(out, cb.Language)
			}
		}
	}
	return out
}

// TestCanonicalCodeLanguages_Encode is the acceptance case: a ```bash
// fence encodes as ADF language "shell" with the option, and verbatim
// without it.
func TestCanonicalCodeLanguages_Encode(t *testing.T) {
	const md = "```bash\necho hi\n```\n"

	got := codeBlockLanguages(t, mdToADF(md, WithCanonicalCodeLanguages(AtlaskitCodeLanguageAliases)))
	if len(got) != 1 || got[0] != "shell" {
		t.Fatalf("with the option: languages = %v, want [shell]", got)
	}

	got = codeBlockLanguages(t, mdToADF(md))
	if len(got) != 1 || got[0] != "bash" {
		t.Fatalf("without the option: languages = %v, want [bash]", got)
	}
}

// TestCanonicalCodeLanguages_Cases covers what the map does and does not
// touch: aliases fold to the canonical spelling, case folds with them, a
// canonical tag is left as it is, an unknown tag keeps the author's exact
// text (case included), and a fence with no language stays bare.
func TestCanonicalCodeLanguages_Cases(t *testing.T) {
	for _, tc := range []struct{ fence, want string }{
		{"bash", "shell"},
		{"sh", "shell"},
		{"zsh", "shell"},
		{"BASH", "shell"},
		{"Bash", "shell"},
		{"shell", "shell"},
		{"js", "javascript"},
		{"yml", "yaml"},
		{"cpp", "c++"},
		{"go", "go"},
		{"none", "none"},
		{"mermaid", "mermaid"},
		{"Mermaid", "Mermaid"},
		{"", ""},
	} {
		t.Run(tc.fence, func(t *testing.T) {
			doc := mdToADF("```"+tc.fence+"\nx\n```\n", WithCanonicalCodeLanguages(AtlaskitCodeLanguageAliases))
			got := codeBlockLanguages(t, doc)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("fence %q: languages = %v, want [%s]", tc.fence, got, tc.want)
			}
		})
	}
}

// TestCanonicalCodeLanguages_Idempotent: encoding the canonical output
// again changes nothing, which is what lets a diff canonicalize both
// sides and compare.
func TestCanonicalCodeLanguages_Idempotent(t *testing.T) {
	opt := WithCanonicalCodeLanguages(AtlaskitCodeLanguageAliases)
	for _, fence := range []string{"bash", "js", "cpp", "postgres", "mermaid"} {
		first := adfToMD(mdToADF("```"+fence+"\nx\n```\n", opt))
		second := adfToMD(mdToADF(first, opt))
		if first != second {
			t.Errorf("fence %q is not idempotent:\nfirst:  %q\nsecond: %q", fence, first, second)
		}
	}
}

// TestCanonicalCodeLanguages_RunsBeforeTheCheck pins the ordering the
// option's contract promises: a normalized alias satisfies
// WithCodeLanguages even when only the canonical spelling is in the set,
// and an unknown tag still reports.
func TestCanonicalCodeLanguages_RunsBeforeTheCheck(t *testing.T) {
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	opts := []Option{
		WithDiagnostics(sink),
		WithCodeLanguages([]string{"shell"}),
		WithCanonicalCodeLanguages(AtlaskitCodeLanguageAliases),
	}

	mdToADF("```bash\nx\n```\n", opts...)
	if len(diags) != 0 {
		t.Fatalf("a canonicalized alias must not report: %+v", diags)
	}

	diags = nil
	mdToADF("```brainfuck\nx\n```\n", opts...)
	if len(diags) != 1 || diags[0].Code != convert.CodeUnsupportedCodeLanguage ||
		!strings.Contains(diags[0].Message, `"brainfuck"`) {
		t.Fatalf("an unknown tag must still report: %+v", diags)
	}
}

// TestCanonicalCodeLanguages_RenderIsUnchanged is the round-trip cost,
// stated as a test rather than left to a comment.
//
// The option is markdown→ADF only, so the render direction never
// rewrites a fence: formatting the file leaves ```bash alone, which is
// what keeps a working copy in the author's spelling. What is NOT
// recoverable is the alias once it has been through ADF — the document
// holds "shell" and nothing else, so rendering it back yields ```shell.
// A diff that canonicalizes its local side the same way (md→ADF→md) sees
// "shell" on both sides and reports nothing, which is the point; a raw
// pull that overwrites the file does change the spelling.
func TestCanonicalCodeLanguages_RenderIsUnchanged(t *testing.T) {
	const md = "```bash\necho hi\n```\n"
	opt := WithCanonicalCodeLanguages(AtlaskitCodeLanguageAliases)

	// Render side: untouched, with the option present and absent alike.
	if got := fmtMD(md, opt); got != md {
		t.Errorf("formatting rewrote the fence: got %q, want %q", got, md)
	}
	if got, want := fmtMD(md, opt), fmtMD(md); got != want {
		t.Errorf("the option changed the render direction: got %q, want %q", got, want)
	}

	// The cost: what comes back out of ADF is the canonical spelling.
	roundTripped := adfToMD(mdToADF(md, opt))
	if !strings.Contains(roundTripped, "```shell") {
		t.Errorf("round trip = %q, want a ```shell fence", roundTripped)
	}
	// And the two sides of such a diff agree, which is what the option buys.
	if local, remote := adfToMD(mdToADF(md, opt)), adfToMD(mdToADF(roundTripped, opt)); local != remote {
		t.Errorf("canonicalized sides disagree:\nlocal:  %q\nremote: %q", local, remote)
	}
}

// TestCanonicalCodeLanguages_EmptyMapIsInert: an empty or nil map must
// leave conversion exactly as it was.
func TestCanonicalCodeLanguages_EmptyMapIsInert(t *testing.T) {
	const md = "```bash\nx\n```\n"
	want := codeBlockLanguages(t, mdToADF(md))
	for name, aliases := range map[string]map[string]string{
		"nil":   nil,
		"empty": {},
	} {
		got := codeBlockLanguages(t, mdToADF(md, WithCanonicalCodeLanguages(aliases)))
		if len(got) != len(want) || got[0] != want[0] {
			t.Errorf("%s map: languages = %v, want %v", name, got, want)
		}
	}
}

// TestCanonicalCodeLanguages_CustomMap proves the option is data-driven,
// not wired to the atlaskit list: a caller's own map is what applies.
func TestCanonicalCodeLanguages_CustomMap(t *testing.T) {
	doc := mdToADF("```bash\nx\n```\n", WithCanonicalCodeLanguages(map[string]string{"bash": "sh"}))
	if got := codeBlockLanguages(t, doc); len(got) != 1 || got[0] != "sh" {
		t.Fatalf("languages = %v, want [sh]", got)
	}
}
