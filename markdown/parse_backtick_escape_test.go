package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

// The backtick's spelling is RE-DERIVED on the remark leg and PRESERVED on
// the prettier leg, which is why '`' is in PreservedEscapes but still goes
// through escapesInlineMarker rather than being carried through untouched
// like the rest of that set (see both doc comments).
//
// This file is the measurement behind PreservedEscapes' backtick paragraph.
// The natural reading of the old divergence below was that the format leg
// forgot the author's spelling and that adding '`' to the preserved set
// would give it back. A bare backtick decodes to the same bytes it started
// as, so the usual "Raw only when it differs" gate never fires for it —
// widening PreservedEscapes alone really was a no-op, which is what made the
// table's first row diverge before convertGoldmarkText and textSlice were
// taught to force Raw for a backtick regardless of equality. The table
// splits the two halves that used to get conflated:
//
//   - AN AUTHORED ESCAPE ALREADY SURVIVED BYTE FOR BYTE, with no special
//     casing needed. escapesInlineMarker escaped a backtick unconditionally
//     before this fix, so the parse could decode "a\`b" to the value "a`b"
//     and the render wrote the same backslash back; today sourceEscaped
//     recognizes the authored escape directly and the result is unchanged.
//   - THE ONCE-DIVERGENT ROWS ARE EVERY BARE SPELLING, which had no backslash
//     for provenance to record under the old rule — not only the plain "a`b"
//     case, but also a bare backtick after a literal (even-parity) backslash
//     run and a bare backtick inside raw HTML, which carry no escape for a
//     different-looking but identical reason. All three are exactly the rows
//     this fix targets: the prettier leg now reproduces each bare, matching
//     prettier 3.8.1, while the remark leg still escapes all three, matching
//     mdast, which has no provenance to read at all.
//
// Measured 2026-09-09 against the authoritative prettier 3.8.1 (the copy the
// frozen TS reference pins) with the parity flags --no-config --parser
// markdown --prose-wrap always --print-width 80
// --embedded-language-formatting off, and against mdast-util-to-markdown over
// mdast-util-from-markdown + micromark-extension-gfm for the remark leg:
//
//	input          prettier 3.8.1   mdast          this leg's fmt   this leg's remark
//	a`b            a`b              a\`b           a`b   (now matches)   a\`b
//	a\`b           a\`b             a\`b           a\`b                  a\`b
//	a\\`b          a\\`b            a\\\`b         a\\`b (now matches)   a\\\`b
//	a`b`c          a`b`c            a`b`c          a`b`c                 a`b`c
//	a\`b\`c        a\`b\`c          a\`b\`c        a\`b\`c               a\`b\`c
//	<b>a`b</b>     <b>a`b</b>       <b>a\`b</b>    <b>a`b</b> (now matches) <b>a\`b</b>
//
// prettier is a fixpoint on every spelling because it echoes source bytes it
// never has to re-derive; the remark leg still escapes a bare backtick on
// every row, because remark's render reads the fully decoded Value with no
// provenance at all — matching mdast, which does the same. adfast also
// renders from ADF, where a text value carrying a backtick has no source to
// echo and MUST be escaped or the next parse turns it into a code span (see
// ast.Text.Rendered); that defensive escape is unaffected by this fix and is
// covered separately below.
//
// A browser sees no difference either: micromark renders "<p><b>a`b</b></p>"
// and "<p><b>a\`b</b></p>" to the same HTML, so the backslash never reaches a
// reader.
var backtickSpellingCases = []struct {
	name string
	src  string
	// want is what both legs write, EXCEPT where wantFmt/wantRem override it
	// for the one row where the fixed prettier leg now diverges from remark.
	want    string
	wantFmt string // overrides want for the prettier leg when non-empty
	wantRem string // overrides want for the remark leg when non-empty
}{
	{
		name:    "a bare backtick keeps the source spelling on format, still gets the defensive escape on remark",
		src:     "a`b\n",
		wantFmt: "a`b\n",
		wantRem: "a\\`b\n",
	},
	{name: "an authored escape comes back unchanged", src: "a\\`b\n", want: "a\\`b\n"},
	{
		// The parity row: two backslashes are an authored LITERAL backslash,
		// which leaves the backtick after them just as bare as if nothing
		// preceded it (an even run never escapes). Measured: prettier 3.8.1
		// also leaves this one bare ("a\\`b"); mdast still escapes it, since
		// remark has no provenance to read the parity from at all.
		name:    "a literal backslash before a bare backtick keeps the backslash and leaves the backtick alone",
		src:     "a\\\\`b\n",
		wantFmt: "a\\\\`b\n",
		wantRem: "a\\\\\\`b\n",
	},
	{
		// The GOOD rows: a matched backtick pair is a code span, and a code
		// span's own delimiters are never escaped. A "fix" that stopped
		// escaping backticks passes the first row and leaves these
		// untouched; a fix that escaped them everywhere breaks these.
		name: "a matched pair stays a code span",
		src:  "a`b`c\n",
		want: "a`b`c\n",
	},
	{
		name: "two authored escapes stay text, not a code span",
		src:  "a\\`b\\`c\n",
		want: "a\\`b\\`c\n",
	},
	{
		// Raw HTML is the shape this was first reported as. The '<b>' and
		// '</b>' stay HTML and the 'a`b' between them is text, so the text
		// rule applies there like anywhere else — bare on format, matching
		// prettier 3.8.1's "<b>a`b</b>"; still escaped on remark.
		name:    "inside a raw HTML span",
		src:     "<b>a`b</b>\n",
		wantFmt: "<b>a`b</b>\n",
		wantRem: "<b>a\\`b</b>\n",
	},
	{
		// The row that makes the escape load-bearing rather than cosmetic,
		// and the one prettier survives only by echoing: the value here is
		// "a`b " + emphasis + " c`d", two unpaired backticks in one
		// paragraph. Written bare, the next parse pairs them into a code
		// span and swallows the emphasis.
		name: "two unpaired backticks around an emphasis",
		src:  "a\\`b *x* c\\`d\n",
		want: "a\\`b _x_ c\\`d\n",
	},
}

// TestBacktickSpellingIsReDerivedOnBothLegs pins the table on the format leg
// and the remark leg at once. The two legs now agree on every row except the
// bare-spelling one, where wantFmt/wantRem record the deliberate divergence.
func TestBacktickSpellingIsReDerivedOnBothLegs(t *testing.T) {
	t.Parallel()
	for _, c := range backtickSpellingCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			wantFmt, wantRem := c.want, c.want
			if c.wantFmt != "" {
				wantFmt = c.wantFmt
			}
			if c.wantRem != "" {
				wantRem = c.wantRem
			}
			gotFmt := Render(convert.NormalizeFormat(Parse([]byte(c.src))), WithPrettierText())
			if gotFmt != wantFmt {
				t.Errorf("format %q = %q, want %q", c.src, gotFmt, wantFmt)
			}
			if gotRem := Render(Parse([]byte(c.src))); gotRem != wantRem {
				t.Errorf("remark %q = %q, want %q", c.src, gotRem, wantRem)
			}
			// Idempotence, because a re-derived spelling that is not a
			// fixpoint would add a backslash on every pass.
			twice := Render(convert.NormalizeFormat(Parse([]byte(gotFmt))), WithPrettierText())
			if twice != gotFmt {
				t.Errorf("not idempotent:\n once:  %q\n twice: %q", gotFmt, twice)
			}
		})
	}
}

// TestPreservedEscapesOmitsTheReDerivedMarkers states the omission as a
// property of the set rather than as a literal, so a future widening has to
// read the reason above before it lands. '_' and '*' are re-derived by
// escapesInlineMarker unconditionally; '#' is re-derived by renderHeading
// (see the doc comment for why that one is under protest). '`' is IN the set
// — it is re-derived too, but through the Raw/Rendered provenance mechanism
// rather than by decoding it away, so it does not belong in this exclusion
// list (see PreservedEscapes' doc comment).
func TestPreservedEscapesOmitsTheReDerivedMarkers(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"_", "*", "#"} {
		if strings.Contains(PreservedEscapes, c) {
			t.Errorf("PreservedEscapes contains %q, whose escape the render re-derives", c)
		}
	}
	if !strings.Contains(PreservedEscapes, "`") {
		t.Error("PreservedEscapes is missing \"`\": it now carries the backtick's provenance")
	}
	// And the set is otherwise the whole escapable class, so the omissions
	// above are the only thing this test is asserting.
	for i := range len(escapableASCIIPunctBytes) {
		c := escapableASCIIPunctBytes[i : i+1]
		if strings.ContainsAny(c, "_*#") {
			continue
		}
		if !strings.Contains(PreservedEscapes, c) {
			t.Errorf("PreservedEscapes is missing %q", c)
		}
	}
}

// TestHandBuiltBacktickPairDoesNotSwallowEmphasis is the swallow-risk case
// the unconditional-escape removal broke, reconstructed the only way it can
// actually arise. A real single Markdown parse can never hand convertGoldmarkText
// two unpaired bare backticks in the same paragraph with an emphasis between
// them: CommonMark's code-span scan pairs any two unescaped backticks of
// matching run length in one paragraph before emphasis is resolved at all, so
// that source parses as one code span straight through the asterisks, not as
// three siblings. ADF has no code-span concept at the text level, though, so
// an editor that is not this parser — Jira's own, or a hand-built tree — can
// produce exactly that shape: two Text nodes with Raw empty (ADF-origin, no
// "I saw this bare" signal) flanking an Emphasis. This builds that tree
// directly, without going through Parse, and checks both that the escape
// still appears and that the escape is load-bearing: a re-parse of the
// escaped output keeps the emphasis rather than swallowing it into a code
// span spanning both backticks.
func TestHandBuiltBacktickPairDoesNotSwallowEmphasis(t *testing.T) {
	t.Parallel()
	tree := &ast.Root{Children: []ast.Node{
		&ast.Paragraph{Children: []ast.Node{
			&ast.Text{Value: "a`b "},
			&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "x"}}},
			&ast.Text{Value: " c`d"},
		}},
	}}

	got := Render(convert.NormalizeFormat(tree), WithPrettierText())
	want := "a\\`b _x_ c\\`d\n"
	if got != want {
		t.Fatalf("format = %q, want %q", got, want)
	}

	reparsed := Parse([]byte(got))
	root, ok := reparsed.(*ast.Root)
	if !ok || len(root.Children) != 1 {
		t.Fatalf("reparse: want one block, got %#v", reparsed)
	}
	para, ok := root.Children[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("reparse: want a paragraph, got %#v", root.Children[0])
	}
	var sawEmphasis, sawCodeSpan bool
	for _, c := range para.Children {
		switch c.(type) {
		case *ast.Emphasis:
			sawEmphasis = true
		case *ast.InlineCode:
			sawCodeSpan = true
		}
	}
	if !sawEmphasis {
		t.Errorf("reparse swallowed the emphasis: %#v", para.Children)
	}
	if sawCodeSpan {
		t.Errorf("reparse paired the two backticks into a code span: %#v", para.Children)
	}
}

// TestRenderEscapesHandBuiltBacktickWithoutNormalize is
// TestHandBuiltBacktickPairDoesNotSwallowEmphasis's tree rendered the OTHER
// way: Render(tree, WithPrettierText()) directly, with no
// convert.NormalizeFormat pass in between. markdown.Render and
// markdown.WithPrettierText are exported on their own — the package doc
// frames the markdown package as stable for "advanced consumers that need
// direct parser access" — and a caller that reaches them that way never
// touches NormalizeFormat at all.
//
// Before writeTextInline read node.Rendered() instead of node.Value, this
// path depended on NormalizeFormat having pre-baked the defensive escape
// into Value; a caller that skipped it got the escape dropped silently,
// because escapesInlineMarker's backtick case stands down in prettier mode
// (see its doc comment) and nothing upstream had re-derived it. On a
// Raw-empty node that is the swallow shape itself: "a`" and "`b" rendered
// bare around the Emphasis reparse as one code span, same as the measured
// pre-fix output "a`_x_`b\n".
func TestRenderEscapesHandBuiltBacktickWithoutNormalize(t *testing.T) {
	t.Parallel()
	tree := &ast.Root{Children: []ast.Node{
		&ast.Paragraph{Children: []ast.Node{
			&ast.Text{Value: "a`"},
			&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "x"}}},
			&ast.Text{Value: "`b"},
		}},
	}}

	got := Render(tree, WithPrettierText())
	want := "a\\`_x_\\`b\n"
	if got != want {
		t.Fatalf("Render(tree, WithPrettierText()) = %q, want %q", got, want)
	}

	reparsed := Parse([]byte(got))
	root, ok := reparsed.(*ast.Root)
	if !ok || len(root.Children) != 1 {
		t.Fatalf("reparse: want one block, got %#v", reparsed)
	}
	para, ok := root.Children[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("reparse: want a paragraph, got %#v", root.Children[0])
	}
	if len(para.Children) != 3 {
		t.Fatalf("reparse: want three children (text, emphasis, text), got %d: %#v", len(para.Children), para.Children)
	}
	var sawEmphasis, sawCodeSpan bool
	for _, c := range para.Children {
		switch c.(type) {
		case *ast.Emphasis:
			sawEmphasis = true
		case *ast.InlineCode:
			sawCodeSpan = true
		}
	}
	if !sawEmphasis {
		t.Errorf("reparse swallowed the emphasis: %#v", para.Children)
	}
	if sawCodeSpan {
		t.Errorf("reparse paired the two backticks into a code span: %#v", para.Children)
	}
}

// escapableASCIIPunctBytes is CommonMark's escapable set: the 32 ASCII
// punctuation bytes a backslash can escape.
const escapableASCIIPunctBytes = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
