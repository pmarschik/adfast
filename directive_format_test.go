package adfast

import (
	"strings"
	"testing"
)

// The prettier md→md formatter is total: every node that goes in comes
// back out. A Markdown formatter may reshape an author's syntax, but it
// may never delete it, and prettier itself (which has no directive
// grammar at all) preserves an unknown directive verbatim.
//
// Before the format-leg canonicalizer existed, ToMarkdown's prettier
// branch ran the encode-leg convert.Normalize, which drops what ADF has
// no node for:
//
//	"::include{path=\"section.md\"}\n"      → "\n"
//	":::sidebar\none\n\ntwo\n:::\n"         → "\n"
//	"text :textdir[label] more\n"           → "text \\:textdirlabel more\n"
//
// storysmith-md hit this through `storymd page pull`, which formats every
// Markdown file it writes: an author's ::include vanished from the page
// document the pull wrote back.
func TestPrettierFormatKeepsGenericDirectives(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"leaf directive", "::include{path=\"section.md\"}\n", "::include{path=\"section.md\"}\n"},
		{"leaf directive with a label", "::include[Label]{path=\"section.md\"}\n", "::include[Label]{path=\"section.md\"}\n"},
		{"leaf directive with no attributes", "::include\n", "::include\n"},
		{"leaf directive between blocks", "before\n\n::include{path=\"a.md\"}\n\nafter\n", "before\n\n::include{path=\"a.md\"}\n\nafter\n"},
		{"container with no children", ":::sidebar\n:::\n", ":::sidebar\n:::\n"},
		{"container with one child", ":::sidebar\nonly\n:::\n", ":::sidebar\nonly\n:::\n"},
		{"container with two children", ":::sidebar\none\n\ntwo\n:::\n", ":::sidebar\none\n\ntwo\n:::\n"},
		{"container with a label", ":::sidebar[Title]\none\n\ntwo\n:::\n", ":::sidebar[Title]\none\n\ntwo\n:::\n"},
		{"container with attributes", ":::sidebar{k=\"v\"}\none\n\ntwo\n:::\n", ":::sidebar{k=\"v\"}\none\n\ntwo\n:::\n"},
		{"text directive", "text :textdir[label] more\n", "text :textdir[label] more\n"},
		{"text directive with attributes", "text :textdir[label]{k=v} more\n", "text :textdir[label]{k=\"v\"} more\n"},
		{"text directive under a mark", "**bold :textdir[label] end**\n", "**bold :textdir[label] end**\n"},
		// A link is not a nesting span the regrouper rebuilds; it is a
		// mark carried on the atom, so a kept directive has to be
		// re-wrapped in it by hand (see atomLeaf). Without that the link
		// itself was the thing deleted.
		{"text directive inside a link", "[:name](https://e.com)\n", "[:name](https://e.com)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tc.in)
			if got != tc.want {
				t.Errorf("format dropped the directive\n in:   %q\n got:  %q\n want: %q", tc.in, got, tc.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Errorf("format is not idempotent\n once:  %q\n twice: %q", got, twice)
			}
		})
	}
}

// The nesting the storysmith page document actually uses: the include
// sits inside other block structure, where the container walk (rather
// than the document walk) reaches it.
func TestPrettierFormatKeepsGenericDirectivesWhenNested(t *testing.T) {
	t.Parallel()
	cases := []string{
		"> ::include{path=\"a.md\"}\n",
		"- ::include{path=\"a.md\"}\n",
		":::outer\n::include{path=\"a.md\"}\n\ntail\n:::\n",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := fmtMD(in); !strings.Contains(got, "::include{path=\"a.md\"}") {
				t.Errorf("format dropped the nested directive\n in:  %q\n got: %q", in, got)
			}
		})
	}
}

// PIN (preserved behavior, red on neither side of the fix). Only the
// format leg is total. The ADF encode still drops what ADF cannot
// represent: ADF has no node for an unknown directive, so a generic leaf
// and a multi-child generic container encode to nothing, and a generic
// text directive flattens to its literal ":name" + label. Keeping the
// nodes through the format canonicalizer must not leak into this leg.
func TestADFEncodeStillDropsGenericDirectives(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"leaf directive drops", "::include{path=\"section.md\"}\n", `"content":[{"type":"paragraph"}]`},
		{"container with two children drops", ":::sidebar\none\n\ntwo\n:::\n", `"content":[{"type":"paragraph"}]`},
		{"container with one child dissolves", ":::sidebar\nonly\n:::\n", `"text":"only"`},
		{"text directive flattens to literal", "text :textdir[label] more\n", `{"type":"text","text":":textdir"},{"type":"text","text":"label"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := adfJSON(t, mdToADF(tc.in))
			if !strings.Contains(got, tc.want) {
				t.Errorf("ADF encode changed\n in:   %q\n got:  %s\n want to contain: %s", tc.in, got, tc.want)
			}
		})
	}
}

// Keeping a generic text directive alive widens what reaches the
// renderer's directive machinery: a ":name" the encode leg used to
// dissolve into text now arrives as a node, next to neighbors that were
// never asked about one. Each case below is a mechanism that ALREADY
// separated the directive from what follows it, on top of which the
// empty attribute block went out anyway — redundant, and unstable,
// because the block appears on the second format and not the first (the
// first parse has no directive in that position at all).
//
// All four were found by FuzzFormatSemanticsPreserved; their inputs are
// seeds under testdata/fuzz. The assertion that matters is idempotence —
// the outputs are pinned to say which spelling won.
func TestPrettierFormatIsStableAroundAKeptTextDirective(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		// The renderer's own "\_" escape ends the directive name.
		// See escapesLeadUnderscore.
		{"underscore escape separates", "0:0_", "0:0\\_\n"},
		// The emphasis repair writes the following "0" as "&#x30;",
		// and a '&' cannot fuse onto a name. See hexEncodesLead.
		{"hex-encoded lead separates", "*:*0*0*0", "_:0&#x30;_&#x30;\n"},
		// Not a hazard case: the '@' escape is decided per text node,
		// and splitting ":0_@" into a directive plus "_@" moved a
		// word byte into the node's predecessor, which read as an
		// email local part running out of the node. No domain follows,
		// so no autolink is possible either way. See linkifiesAsEmail.
		{"email escape does not turn on", ":0_@", ":0\\_@\n"},
		// Not a hazard case either: goldmark-directive reads ":30" as
		// a directive inside a link LABEL and hands the parse back as
		// three links. See joinMarkWrappers.
		{"a directive in a link label keeps one link", "[Call at 5:30](https://e.com)\n", "[Call at 5:30](https://e.com)\n"},
		{"a directive in a link label under a mark", "[**0*:*00**]()", "_[0:00]()_\n"},
		// The '!' of an image is what follows the name, not the first
		// rune of its alt text, and '!' cannot fuse onto a name.
		// See nodeLeadRune.
		{"an image leads with its own marker", ":dir![i](p.png)\n", ":dir![i](p.png)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tc.in)
			if got != tc.want {
				t.Errorf("format changed\n in:   %q\n got:  %q\n want: %q", tc.in, got, tc.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Errorf("format is not idempotent\n once:  %q\n twice: %q", got, twice)
			}
		})
	}
}
