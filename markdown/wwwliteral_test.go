package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The scheme-less "www." autolink literal, and the one thing about it that
// was wrong: the host after the prefix had to hold a dot of its own, so
// "www.x" was prose here and a link in the reference implementation adfast
// round-trips against. That is a LINK-VERSUS-TEXT divergence and not a
// shifted extent — a canonical diff over a document holding a bare "www."
// host disagreed about the payload, and a push sent prose where the
// reference sends a link.
//
// Both of the reference's recognizers accept it, for two different reasons,
// and the second is the one that makes the prefix load-bearing: micromark's
// tokenizer needs no dot in a domain at all, while
// mdast-util-gfm-autolink-literal's transform splits the host on '.' and
// demands two segments — which "www" and "x" are.

// wwwLiteralCases pins what urlLiteralRe reads as a scheme-less literal,
// candidate by candidate, so the widening's edges are written down rather
// than implied by the end-to-end test below.
var wwwLiteralCases = []struct {
	name string
	src  string
	// want is what urlLiteralRe matches, "" for no match.
	want string
}{{
	// THE WIDENING. One host character after the prefix is a literal.
	name: "a dotless host after the prefix",
	src:  "www.x",
	want: "www.x",
}, {
	name: "a dotless host with a path",
	src:  "www.x/y",
	want: "www.x/y",
}, {
	// PRESERVED BEHAVIOR. The shape that always linked, at the extent it
	// always had.
	name: "a dotted host",
	src:  "www.ex.com",
	want: "www.ex.com",
}, {
	name: "a dotted host with a path",
	src:  "www.ex.com/a",
	want: "www.ex.com/a",
}, {
	// THE EXTENT GUARD, and the reason urlLiteralHostDotted comes FIRST in
	// the alternation. '~' is a host byte, so a lone dotless alternative
	// would read this whole string as the address; the dotted one stops at
	// the TLD and leaves "~foo" to the path gap urlliteral.go's header
	// describes. A change that reorders the alternatives passes every other
	// row here.
	name: "a tilde tail still stops the literal",
	src:  "www.ex.com~foo",
	want: "www.ex.com",
}, {
	// THE NEGATIVES. The prefix alone is not a host, and the dotless
	// alternative opens on an alphanumeric exactly as the schemed one does
	// ("https://-x" is prose too).
	name: "the prefix alone is not a host",
	src:  "www.",
	want: "",
}, {
	name: "a hyphen does not open the host",
	src:  "www.-x",
	want: "",
}, {
	name: "an underscore does not open the host",
	src:  "www._x",
	want: "",
}, {
	// The prefix is the prefix: two w's do not make one.
	name: "a two-letter prefix is not the literal",
	src:  "ww.x",
	want: "",
}}

func TestWWWLiteralPattern(t *testing.T) {
	t.Parallel()
	for _, c := range wwwLiteralCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := urlLiteralRe.FindString(c.src); got != c.want {
				t.Errorf("urlLiteralRe.FindString(%q) = %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// TestWWWLiteralLinksASchemeLessHost is the end-to-end half: one document
// holding the shape that CHANGED next to the shapes that must NOT, so a
// widening that also swallowed its neighbors would fail here rather than
// pass quietly.
//
// The three neighbors are chosen for what each would catch:
//
//   - "ww.x" is the escaper's fixture (see render_escape_dot_test.go). If
//     the prefix ever stopped being literal, that whole test would start
//     measuring a link instead of an escape.
//   - "www. x" is the prefix with nothing after it, which must not reach
//     across the space for a host.
//   - "www.ex.com~foo" is the extent guard, end to end: the href stops at
//     the TLD and the tilde stays in the prose.
func TestWWWLiteralLinksASchemeLessHost(t *testing.T) {
	t.Parallel()
	const src = "www.x ww.x www. x www.ex.com~foo\n"
	want := []string{"http://www.x", "http://www.ex.com"}
	if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
		t.Errorf("link URLs of %q = %v, want %v", src, got, want)
	}
	// The prose around the two links is what the neighbors assert: nothing
	// but the two addresses became a link.
	const wantRender = "[www.x](http://www.x) ww\\.x www. x " +
		"[www.ex.com](http://www.ex.com)\\~foo\n"
	got := Render(Parse([]byte(src)))
	if got != wantRender {
		t.Errorf("Render(Parse(%q)) = %q, want %q", src, got, wantRender)
	}
	if again := Render(Parse([]byte(got))); again != got {
		t.Errorf("not idempotent: %q -> %q", got, again)
	}
}

// TestWWWLiteralLinksThroughAnAuthoredDotEscape records the half of the
// reference's www handling that only makes sense next to the other half.
// Its escaper writes "www\." into text that would otherwise linkify (the
// dotAfterWwwEscapes rule), and its recognizer then links THROUGH that
// escape on the way back in, because the recognizer reads the decoded text.
// Both halves are the reference's, so a document that has been through it
// once comes back as a link, not as escaped prose.
func TestWWWLiteralLinksThroughAnAuthoredDotEscape(t *testing.T) {
	t.Parallel()
	const src = "see www\\.x b\n"
	want := []string{"http://www.x"}
	if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
		t.Errorf("link URLs of %q = %v, want %v", src, got, want)
	}
}

// linkURLsOf collects every link destination in the tree, in document order.
func linkURLsOf(root ast.Node) []string {
	var out []string
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if l, ok := n.(*ast.Link); ok {
			out = append(out, l.URL)
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	walk(root)
	return out
}

func equalStrings(a, b []string) bool {
	return len(a) == len(b) && strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
