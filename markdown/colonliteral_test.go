package markdown

import "testing"

// TestColonBoundaryTrimsTheLiteralEnd covers the byte colonURLParser owns.
// Its acceptance is the package's two patterns, but the pattern's extent is
// not the parser's: goldmark trims a trailing "?!.,:*_~" run off a literal,
// and reading the raw match length skipped that trim on this one path.
// Measured against the frozen reference, whole bodies:
//
//	"z:http://a.b?! c"  ->  "z:<http://a.b>?! c"   (here: href "http://a.b?!")
//
// The dot and paren rows are the good cases: they trimmed correctly before,
// because the pattern itself stops there.
func TestColonBoundaryTrimsTheLiteralEnd(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, want string }{
		{"z:http://a.b?! c\n", "http://a.b"},
		{"z:http://a.b. c\n", "http://a.b"},
		{"z:http://a.b) c\n", "http://a.b"},
	} {
		t.Run(c.src, func(t *testing.T) {
			t.Parallel()
			if got := linkURLsOf(Parse([]byte(c.src))); !equalStrings(got, []string{c.want}) {
				t.Errorf("link URLs of %q = %v, want %v", c.src, got, []string{c.want})
			}
		})
	}
}
