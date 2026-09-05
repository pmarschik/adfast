package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The format leg keeps a bare "www." literal in the spelling the author wrote.
//
// A "www." literal is the one bare autolink whose URL is not its text — the
// parser completes it with the scheme goldmark's own www branch prepends — so
// it missed the bare-autolink path and went out as "[www.x](http://www.x)".
// That is link syntax the author never typed, in prose as ordinary as
// "see www.example.com for details".
//
// Measured 2026-09-05 on the frozen prettier 3.8.1 install with the parity
// flags (--no-config --parser markdown --prose-wrap always --print-width 80
// --embedded-language-formatting off). Every bare row below is a fixpoint
// there, and an explicit link is left alone:
//
//	"see www.x b"     -> "see www.x b"
//	"z-www.a.b c"     -> "z-www.a.b c"
//	"a www.x.y, b"    -> "a www.x.y, b"
//	"see WWW.X b"     -> "see WWW.X b"
//	"[www.x](http://www.x)" -> unchanged
//
// The remark leg is deliberately NOT changed: mdast-util-to-markdown, the
// engine remark-stringify is, really does bracket a www literal — measured the
// same day, "see www.x b" comes back as "see [www.x](http://www.x) b" — and
// each leg is measured against the tool it is a port of. The remark rows below
// hold that line, so a fix that simply stopped bracketing everywhere fails
// them.
var wwwLiteralSpellingCases = []struct {
	name string
	in   string
	// fmtWant is the format-leg output, which must also be a fixpoint.
	fmtWant string
	// remWant is the remark-leg output, unchanged by this fix.
	remWant string
}{{
	name:    "a bare literal in prose keeps its spelling",
	in:      "see www.x b",
	fmtWant: "see www.x b\n",
	remWant: "see [www.x](http://www.x) b\n",
}, {
	name:    "so does one opened by the widened punctuation boundary",
	in:      "z-www.a.b c",
	fmtWant: "z-www.a.b c\n",
	remWant: "z-[www.a.b](http://www.a.b) c\n",
}, {
	name:    "trailing punctuation the host gives back stays in the prose",
	in:      "a www.x.y, b",
	fmtWant: "a www.x.y, b\n",
	remWant: "a [www.x.y](http://www.x.y), b\n",
}, {
	name:    "the prefix is case folded, and so is the spelling kept",
	in:      "see WWW.X b",
	fmtWant: "see WWW.X b\n",
	remWant: "see [WWW.X](http://WWW.X) b\n",
}, {
	// GOOD CASE. An EXPLICIT link is what the author wrote, brackets and
	// all, and it keeps them on both legs — the label happening to equal
	// the host is not license to unwrap it.
	name:    "an explicit link to the same host keeps its brackets",
	in:      "[www.x](http://www.x)",
	fmtWant: "[www.x](http://www.x)\n",
	remWant: "[www.x](http://www.x)\n",
}, {
	// GOOD CASE. The destination differs from the label, so no spelling
	// could reproduce it and the brackets are the only correct output.
	name:    "an explicit link whose destination differs keeps its brackets",
	in:      "[www.x](http://www.x/other)",
	fmtWant: "[www.x](http://www.x/other)\n",
	remWant: "[www.x](http://www.x/other)\n",
}, {
	// GOOD CASE. A schemed literal already took the bare path on both
	// legs, so this row is unmoved by the fix and pins that it stays so.
	name:    "a schemed literal is unaffected",
	in:      "z.http://a.b c",
	fmtWant: "z.http://a.b c\n",
	remWant: "z.http://a.b c\n",
}}

func TestWWWLiteralKeepsItsSourceSpellingWhenFormatting(t *testing.T) {
	t.Parallel()
	for _, c := range wwwLiteralSpellingCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := formatMD(t, c.in)
			if got != c.fmtWant {
				t.Errorf("format(%q) = %q, want %q", c.in, got, c.fmtWant)
			}
			if again := formatMD(t, got); again != got {
				t.Errorf("format is not a fixpoint on %q: %q then %q", c.in, got, again)
			}
			if rem := Render(Parse([]byte(c.in))); rem != c.remWant {
				t.Errorf("Render(Parse(%q)) = %q, want %q", c.in, rem, c.remWant)
			}
		})
	}
}

// The bare spelling has to parse back to the same link, or the formatter would
// be deleting one. The tree is the assertion rather than the bytes: a literal
// whose extent depends on what follows it ("www.0.a0" links only "www.0.a")
// would be the way to get this wrong.
func TestWWWLiteralBareSpellingReparsesToTheSameLinks(t *testing.T) {
	t.Parallel()
	for _, c := range wwwLiteralSpellingCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := linkURLsOf(Parse([]byte(c.in)))
			got := linkURLsOf(convert.NormalizeFormat(Parse([]byte(formatMD(t, c.in)))))
			if !equalStrings(got, want) {
				t.Errorf("link URLs after format(%q) = %v, want %v", c.in, got, want)
			}
		})
	}
}
