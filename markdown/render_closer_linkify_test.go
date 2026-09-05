package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// An emphasis delimiter must not become part of a bare-URL literal.
//
// '_' is both a byte goldmark's linkify extension triggers on and a byte its
// host class accepts, so an emphasis whose content ends inside a half-finished
// address can have its CLOSING marker annexed by the host. The format leg then
// deletes the author's emphasis and puts a link in its place, silently.
//
// Measured 2026-09-05 on the frozen prettier 3.8.1 install with the parity
// flags. prettier writes the same bytes as this renderer used to and is a
// fixpoint on them, because its pre-CommonMark parser has no autolink literals
// at all; the parser adfast round-trips against does linkify, so those bytes
// are not a fixpoint here:
//
//	"*www.*..A"  formatted to  "_www._..A"
//	             and again to  "\_[www.\_..A](http://www._..A)"
//
// Choosing '*' is the repair, and it is available precisely because '*' is a
// trigger byte that is NOT in the host class, so a literal always stops in
// front of it. This is a deliberate divergence from prettier's bytes of the
// kind escapeAt already makes for '@' (see linkifiesAsEmail).
var closerLinkifyCases = []struct {
	name string
	// in is the markdown source; the row asserts a Format fixpoint on it.
	in string
	// want is the first format pass, which must also be the second.
	want string
}{{
	name: "an empty host segment lets the closer into the address",
	in:   "*www.*..A",
	want: "*www.*..A\n",
}, {
	name: "one empty segment is enough",
	in:   "*www.*.a.A",
	want: "*www.*.a.A\n",
}, {
	name: "and the host may be longer",
	in:   "*www.*.a.b.A",
	want: "*www.*.a.b.A\n",
}, {
	name: "a schemed literal reaches the closer the same way",
	in:   "*https://ex.*.a.A",
	want: "*https://ex.*.a.A\n",
}, {
	// THE GOOD CASES. Plain emphasis keeps the preferred '_' — a fix that
	// simply stopped choosing '_' would pass every row above and break these.
	name: "ordinary emphasis keeps the preferred underscore",
	in:   "a *b* c",
	want: "a _b_ c\n",
}, {
	name: "so does emphasis at the start of a paragraph",
	in:   "*x* b",
	want: "_x_ b\n",
}, {
	// The parser's own trailing trim gives the closer back here: "www.x"
	// plus '_' matches "www.x_", which trims to "www.x" and leaves the
	// underscore outside the address. So this is not a hazard and the
	// preferred delimiter stands.
	name: "a closer the address trims away is not a hazard",
	in:   "a *www.x* b",
	want: "a _www.x_ b\n",
}}

func TestFormat_EmphasisDelimiterIsNotEatenByAURLLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range closerLinkifyCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := formatMD(t, c.in)
			if got != c.want {
				t.Errorf("format(%q) = %q, want %q", c.in, got, c.want)
			}
			if again := formatMD(t, got); again != got {
				t.Errorf("format is not a fixpoint on %q: %q then %q", c.in, got, again)
			}
		})
	}
}

// formatMD runs the md -> md format leg the way the facade composes it, which
// is what a user's "format this file" reaches.
func formatMD(t *testing.T, in string) string {
	t.Helper()
	return Render(convert.NormalizeFormat(Parse([]byte(in))), WithPrettierText())
}
