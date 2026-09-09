package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The backtick's spelling is RE-DERIVED, not preserved, and that is why '`'
// is not in PreservedEscapes.
//
// This file is the measurement behind that sentence in PreservedEscapes' doc
// comment. The natural reading of the divergence below is that the format leg
// forgets the author's spelling and that adding '`' to the preserved set would
// give it back. It would not, and the table separates the two halves that get
// conflated:
//
//   - AN AUTHORED ESCAPE ALREADY SURVIVES BYTE FOR BYTE, with no provenance.
//     escapesInlineMarker escapes a backtick unconditionally, so the parse can
//     decode "a\`b" to the value "a`b" and the render writes the same
//     backslash back.
//   - THE DIVERGENT ROW IS THE BARE SPELLING, which has no backslash for
//     provenance to record. "a`b" cannot be carried through as bare by any
//     widening of PreservedEscapes; only escapesInlineMarker decides it.
//
// Measured 2026-09-09 against the authoritative prettier 3.8.1 (the copy the
// frozen TS reference pins) with the parity flags --no-config --parser
// markdown --prose-wrap always --print-width 80
// --embedded-language-formatting off, and against mdast-util-to-markdown over
// mdast-util-from-markdown + micromark-extension-gfm for the remark leg:
//
//	input          prettier 3.8.1   mdast          this file's legs
//	a`b            a`b              a\`b           a\`b   (fmt DIVERGES)
//	a\`b           a\`b             a\`b           a\`b   (both agree)
//	a\\`b          a\\`b            a\\\`b         a\\\`b (fmt DIVERGES)
//	a`b`c          a`b`c            a`b`c          a`b`c  (both agree)
//	a\`b\`c        a\`b\`c          a\`b\`c        a\`b\`c
//	<b>a`b</b>     <b>a`b</b>       <b>a\`b</b>    <b>a\`b</b>
//
// prettier is a fixpoint on every spelling because it echoes source bytes it
// never has to re-derive; adfast renders from ADF too, where a text value
// carrying a backtick has no source to echo and MUST be escaped or the next
// parse turns it into a code span. On every row the remark reference and this
// leg agree, so the divergence is prettier's echo, not a rule adfast is
// missing.
//
// A browser sees no difference either: micromark renders "<p><b>a`b</b></p>"
// and "<p><b>a\`b</b></p>" to the same HTML, so the backslash never reaches a
// reader.
var backtickSpellingCases = []struct {
	name string
	src  string
	// want is what BOTH legs write; the backtick rule does not vary by mode.
	want string
}{
	{name: "a bare backtick gains the defensive escape", src: "a`b\n", want: "a\\`b\n"},
	{name: "an authored escape comes back unchanged", src: "a\\`b\n", want: "a\\`b\n"},
	{
		// The parity row: two backslashes are an authored LITERAL backslash
		// before a bare backtick, so the output needs three — one to keep
		// the backslash, one to escape the backtick.
		name: "a literal backslash before a bare backtick keeps both",
		src:  "a\\\\`b\n",
		want: "a\\\\\\`b\n",
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
		// rule applies there like anywhere else.
		name: "inside a raw HTML span",
		src:  "<b>a`b</b>\n",
		want: "<b>a\\`b</b>\n",
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
// and the remark leg at once: the backtick rule is one of the few that does
// not vary by mode, which is exactly why it needs no provenance.
func TestBacktickSpellingIsReDerivedOnBothLegs(t *testing.T) {
	t.Parallel()
	for _, c := range backtickSpellingCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gotFmt := Render(convert.NormalizeFormat(Parse([]byte(c.src))), WithPrettierText())
			if gotFmt != c.want {
				t.Errorf("format %q = %q, want %q", c.src, gotFmt, c.want)
			}
			if gotRem := Render(Parse([]byte(c.src))); gotRem != c.want {
				t.Errorf("remark %q = %q, want %q", c.src, gotRem, c.want)
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
// read the reason above before it lands. '_', '*' and '`' are all re-derived
// by escapesInlineMarker; '#' is re-derived by renderHeading (see the doc
// comment for why that one is under protest).
func TestPreservedEscapesOmitsTheReDerivedMarkers(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"_", "*", "`", "#"} {
		if strings.Contains(PreservedEscapes, c) {
			t.Errorf("PreservedEscapes contains %q, whose escape the render re-derives", c)
		}
	}
	// And the set is otherwise the whole escapable class, so the omission
	// above is the only thing this test is asserting.
	for i := range len(escapableASCIIPunctBytes) {
		c := escapableASCIIPunctBytes[i : i+1]
		if strings.ContainsAny(c, "_*`#") {
			continue
		}
		if !strings.Contains(PreservedEscapes, c) {
			t.Errorf("PreservedEscapes is missing %q", c)
		}
	}
}

// escapableASCIIPunctBytes is CommonMark's escapable set: the 32 ASCII
// punctuation bytes a backslash can escape.
const escapableASCIIPunctBytes = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
