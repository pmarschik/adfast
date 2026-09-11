package markdown

import (
	"fmt"
	"slices"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// An emphasis beside a BARE link literal must be written with a delimiter
// that survives touching the address.
//
// A bare autolink — the linkified form, "www.example.com" or "http://ex.com"
// or "x@ex.com", as opposed to an explicit "[x](y)" or an angle "<http://x>"
// — writes its ADDRESS, so the byte at each of its ends is a LETTER or a
// digit. Two peeks used to answer for it with the punctuation of a syntax the
// renderer never writes: linkLeadRune reported the '<' or '[' of the form it
// did not take, and VisitLink reported the ')' of a "](url)" tail it did not
// write. '_' may flank against punctuation and cannot flank intraword, so
// both lies took '_' for an emphasis that then landed INSIDE a word — and a
// '_' that is not a delimiter is not emphasis at all.
//
// THE MARK IS LOST, WHICH IS WHY THIS IS NOT A SPELLING NIT. Measured on the
// format leg at the previous commit, ToADF(FromMarkdown(...)) on both sides:
//
//	"*a*www.example.com" -> "_a_www.example.com"   em gone
//	"www.example.com*a*" -> "www.example.com_a_"   em AND link gone
//	"*0*www.0"           -> "_0_www.0"             em gone
//	"www.0*00A00*"       -> "www.0_00A00_"         em AND link gone
//	"*a*http://ex.com"   -> "_a_http://ex.com"     em gone, BOTH legs
//	"x@ex.com*a*"        -> "x@ex.com_a_"          em AND link gone, BOTH legs
//
// The left-hand rows are the worse half: two differently-marked nodes collapse
// into ONE plain text node, so the link goes with the emphasis.
//
// EVERY ROW ASSERTS THE MEANING, not only the bytes. kindsOf compares the
// inline node kinds of the re-parse against those of the source parse, which
// is a claim no delimiter spelling can satisfy by accident — and it is the
// claim that holds on BOTH legs even where the legs write different bytes,
// because the remark leg brackets a "www." literal and a peeked '[' is the
// truth there.
var bareLinkFlankingCases = []struct {
	name string
	src  string
	// fmtWant is the format leg (prettier's md -> md mode).
	fmtWant string
	// remWant is the remark leg (mdast-util-to-markdown), which writes a
	// "www." literal as an explicit link and so keeps '_' on those rows.
	remWant string
}{{
	// FIX. The peeked '[' of the link form the format leg does not take.
	name:    "emphasis before a www literal",
	src:     "*a*www.example.com",
	fmtWant: "*a*www.example.com\n",
	remWant: "_a_[www.example.com](http://www.example.com)\n",
}, {
	// FIX, the worse half: the assumed ')' of a tail the bare form has not got.
	name:    "emphasis after a www literal",
	src:     "www.example.com*a*",
	fmtWant: "www.example.com*a*\n",
	remWant: "[www.example.com](http://www.example.com)_a_\n",
}, {
	name:    "digits are word bytes too",
	src:     "*0*www.0",
	fmtWant: "*0*www.0\n",
	remWant: "_0_[www.0](http://www.0)\n",
}, {
	name:    "and on the left of a digit host",
	src:     "www.0*00A00*",
	fmtWant: "www.0*00A00*\n",
	remWant: "[www.0](http://www.0)_00A00_\n",
}, {
	// FIX on BOTH legs: a schemed literal is written bare by each of them,
	// so neither ever had a '<' or a ')' to report.
	name:    "emphasis before a bare schemed literal",
	src:     "*a*http://ex.com",
	fmtWant: "*a*http://ex.com\n",
	remWant: "*a*http://ex.com\n",
}, {
	name:    "emphasis after a bare schemed literal",
	src:     "http://ex.com*a*",
	fmtWant: "http://ex.com*a*\n",
	remWant: "http://ex.com*a*\n",
}, {
	// FIX on both legs: an email literal is the third bare form, and its
	// mailto: URL reaches the same branch of writeLink.
	name:    "emphasis before a bare email literal",
	src:     "*a*x@ex.com",
	fmtWant: "*a*x@ex.com\n",
	remWant: "*a*x@ex.com\n",
}, {
	name:    "emphasis after a bare email literal",
	src:     "x@ex.com*a*",
	fmtWant: "x@ex.com*a*\n",
	remWant: "x@ex.com*a*\n",
}, {
	// FIX. One literal with an emphasis on each side needs both peeks right.
	name:    "a literal between two emphases",
	src:     "*a*www.x.y*b*",
	fmtWant: "*a*www.x.y*b*\n",
	remWant: "_a_[www.x.y](http://www.x.y)_b_\n",
}, {
	// THE GOOD CASES. '_' is CORRECT next to an explicit link: the peeked
	// '[' really is the byte written, and a fix that stopped trusting the
	// peek in general would move every row below.
	name:    "an explicit link keeps the preferred underscore",
	src:     "*a*[x](y)",
	fmtWant: "_a_[x](y)\n",
	remWant: "_a_[x](y)\n",
}, {
	name:    "and on the left of the emphasis",
	src:     "[x](y)*a*",
	fmtWant: "[x](y)_a_\n",
	remWant: "[x](y)_a_\n",
}, {
	// GOOD: the ANGLE form really does write '<' and really does end in '>'.
	name:    "an angle autolink keeps the preferred underscore",
	src:     "*a*<http://ex.com>",
	fmtWant: "_a_<http://ex.com>\n",
	remWant: "_a_<http://ex.com>\n",
}, {
	name:    "and on the left of the emphasis",
	src:     "<http://ex.com>*a*",
	fmtWant: "<http://ex.com>_a_\n",
	remWant: "<http://ex.com>_a_\n",
}, {
	// GOOD: an image writes its '!' and its ')' whatever its URL is.
	name:    "an image keeps the preferred underscore",
	src:     "![i](p.png)*a*",
	fmtWant: "![i](p.png)_a_\n",
	remWant: "![i](p.png)_a_\n",
}, {
	// GOOD: a space between them defeats nothing, and '_' must stay.
	name:    "a space beside the literal keeps the preferred underscore",
	src:     "www.x.y *a* b",
	fmtWant: "www.x.y _a_ b\n",
	remWant: "[www.x.y](http://www.x.y) _a_ b\n",
}, {
	// PRESERVED-BEHAVIOR PIN, not a fix row: strong is written with '*',
	// which has an intraword form, so it never depended on either peek.
	// It is here so a later change to the peeks cannot move it unnoticed.
	name:    "strong beside a literal was never affected",
	src:     "www.x.y**a**",
	fmtWant: "www.x.y**a**\n",
	remWant: "[www.x.y](http://www.x.y)**a**\n",
}}

func TestBareLinkLeadIsTheAddressNotBracket(t *testing.T) {
	t.Parallel()
	for _, c := range bareLinkFlankingCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := kindsOf(Parse([]byte(c.src)))

			got := formatMD(t, c.src)
			if got != c.fmtWant {
				t.Errorf("format(%q) = %q, want %q", c.src, got, c.fmtWant)
			}
			if again := formatMD(t, got); again != got {
				t.Errorf("format is not a fixpoint on %q: %q then %q", c.src, got, again)
			}
			if kinds := kindsOf(Parse([]byte(got))); !slices.Equal(kinds, want) {
				t.Errorf("format(%q) = %q re-parses as %v, want %v", c.src, got, kinds, want)
			}

			rem := Render(Parse([]byte(c.src)))
			if rem != c.remWant {
				t.Errorf("remark(%q) = %q, want %q", c.src, rem, c.remWant)
			}
			if kinds := kindsOf(Parse([]byte(rem))); !slices.Equal(kinds, want) {
				t.Errorf("remark(%q) = %q re-parses as %v, want %v", c.src, rem, kinds, want)
			}
		})
	}
}

// kindsOf is the sorted list of inline node kinds a tree holds. It is the
// meaning assertion the byte tables above rest on: a lost emphasis loses its
// *ast.Emphasis, and a mark that merges into its neighbor loses that
// neighbor's node with it, whatever delimiter the render happened to spell.
//
// Kinds and not values, so a leg that writes the same document with different
// bytes — "www.x" bare on the format leg and "[www.x](http://www.x)" on the
// remark leg — still answers the same list.
func kindsOf(root ast.Node) []string {
	var out []string
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for _, c := range ast.Children(n) {
			out = append(out, fmt.Sprintf("%T", c))
			walk(c)
		}
	}
	walk(root)
	slices.Sort(out)
	return out
}
