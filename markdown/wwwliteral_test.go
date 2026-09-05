package markdown

import (
	"slices"
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
	// hostRejected is urlLiteralHostAccepted's verdict on src, inverted so
	// the common answer is the zero value. A row with a nonempty want and
	// hostRejected set matches the pattern and is still prose in the
	// document: the "www." prefix supplies the dot the host rule counts
	// from, but it does not exempt the segments after it.
	hostRejected bool
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
	// The prefix alone leaves an EMPTY DOMAIN once it is stripped, and the
	// reference's domain production must consume at least one character, so
	// the gate refuses it as the pattern does.
	name:         "the prefix alone is not a host",
	src:          "www.",
	want:         "",
	hostRejected: true,
}, {
	name: "a hyphen does not open the host",
	src:  "www.-x",
	want: "",
}, {
	// Refused twice over, and the columns say which came first: the dotless
	// alternative does not open a host on an underscore, and the gate would
	// refuse "www._x" anyway because the last segment holds one.
	name:         "an underscore does not open the host",
	src:          "www._x",
	want:         "",
	hostRejected: true,
}, {
	// THE HOST GATE ON THE SCHEME-LESS SIDE. The dotted alternative reaches
	// past an underscore that the dotless one may not open on, so the pattern
	// takes both of these whole and urlLiteralHostAccepted is what refuses
	// them — an underscore in either of the last two segments, which is
	// micromark's domain rule and mdast-util's isCorrectDomain alike.
	name:         "an underscore in the second-to-last segment",
	src:          "www.a_b.com",
	want:         "www.a_b.com",
	hostRejected: true,
}, {
	// THE FORMAT-LEG HOST, and the reason this rule is not a parity nit. The
	// formatter may pick '_' as the emphasis delimiter, so "*www.*.A" is
	// rendered "_www._.A", whose bytes hold this literal. Linking it drops
	// the emphasis and invents a link the author never wrote; the last two
	// segments are "_" and "A", so the rule refuses it.
	name:         "the delimiter host the formatter can produce",
	src:          "www._.A",
	want:         "www._.A",
	hostRejected: true,
}, {
	// THE FUZZ REPRO for the '_' in urlLiteralHostDotted's TLD class, which
	// is the byte that lets this candidate match at all: the domain after the
	// prefix is ".._", nothing but punctuation. Trimming the trailing run off
	// the WHOLE literal left "www", a clean host, and "*www..*" formatted to
	// "_www.._" and linked "http://www" — an invented link, a shortened
	// address and lost emphasis in one input. The gate strips the prefix
	// first, so it sees the empty domain the reference sees.
	name:         "a domain of nothing but punctuation",
	src:          "www.._",
	want:         "www.._",
	hostRejected: true,
}, {
	// LAST TWO, NOT ANY: the same underscore one segment further left is
	// accepted, so the rule is a segment count and not a ban on the byte.
	name: "an underscore in the third-to-last segment is accepted",
	src:  "www.x_y.z.com",
	want: "www.x_y.z.com",
}, {
	// The one-letter TLD next to the row above, which is the accepted half of
	// the "www._.A" shape: same length, same segments, no underscore.
	name: "a single-letter TLD is accepted",
	src:  "www.a.A",
	want: "www.a.A",
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
			if got := !urlLiteralHostAccepted(c.src); got != c.hostRejected {
				t.Errorf("urlLiteralHostAccepted(%q) = %v, want %v",
					c.src, !got, !c.hostRejected)
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

// TestWWWLiteralIsReportedAsARawAutolinkSpan is the RAW-SPAN half, and it
// is a different question from the two tests above.
//
// Those read the TREE, and the tree is not goldmark's answer alone:
// relinkifyTexts rescans the decoded text with urlLiteralRe afterwards and
// puts back whatever goldmark declined. So a widening that only ever
// reached urlLiteralRe still produced the link — and Source.Autolinks,
// which reports goldmark's verdict and NOTHING else, still reported no
// span. Measured before this test existed, with the tree agreeing in both
// rows and the spans disagreeing:
//
//	"see www.x b\n"       autolinks []                     links [http://www.x]
//	"see www.ex.com b\n"  autolinks [{… Span:{4 14} …}]    links [http://www.ex.com]
//
// A caller driving off spans — a linter naming a location, a rewriter
// splicing the source — therefore lost the link for exactly the hosts the
// widening was meant to add, while the payload said it was there.
//
// ONE DOCUMENT HOLDS BOTH, so the row that changed is measured next to the
// row that must not: "www.x" is the dotless host the widening adds, and
// "www.ex.com" is the dotted host that always reported a span, at the
// offset it always had.
func TestWWWLiteralIsReportedAsARawAutolinkSpan(t *testing.T) {
	t.Parallel()
	const src = "see www.x and www.ex.com b\n"
	want := []Autolink{
		{Target: "http://www.x", Span: Span{4, 9}, Text: Span{4, 9}, Bare: true},
		{Target: "http://www.ex.com", Span: Span{14, 24}, Text: Span{14, 24}, Bare: true},
	}
	got := NewSource([]byte(src)).Autolinks()
	if !slices.Equal(got, want) {
		t.Errorf("Autolinks(%q) = %+v, want %+v", src, got, want)
	}
}

// TestWWWLiteralRawRecognizerMatchesThePattern closes the loop the test
// above opens: rather than pinning two hand-picked rows, it drives EVERY
// row of wwwLiteralCases through the parser and asserts the raw span is
// exactly what urlLiteralRe reads at that offset.
//
// ONE PATTERN AND ONE GATE, which is why the rejected rows expect no span at
// all rather than a shorter one: urlLiteralHostAccepted refuses a candidate
// whole, and the raw recognizer has to refuse it the same way the decoded-text
// scan does. A gate wired into only one of the two would show up here as a
// span for a host the tree has no link for.
//
// This is the property that makes one pattern one verdict. goldmark ships
// its own scheme-less pattern (`www\.…{1,256}\.[a-z]+`), and while the
// parser was left on it the raw recognizer and the decoded-text scan could
// disagree about any row here without a single existing test moving. The
// rows with a nonempty want that goldmark's pattern also accepted are the
// PRESERVED-BEHAVIOR half; the dotless ones are the half that changed.
func TestWWWLiteralRawRecognizerMatchesThePattern(t *testing.T) {
	t.Parallel()
	for _, c := range wwwLiteralCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			// The candidate sits mid-line so the reader reaches it through
			// the linkify parser's ' ' trigger, which is the path prose
			// takes; "see " is four bytes, so a match starts at offset 4.
			src := "see " + c.src + " b\n"
			var want []Autolink
			if c.want != "" && !c.hostRejected {
				stop := 4 + len(c.want)
				want = []Autolink{{
					Target: "http://" + c.want,
					Span:   Span{4, stop},
					Text:   Span{4, stop},
					Bare:   true,
				}}
			}
			if got := NewSource([]byte(src)).Autolinks(); !slices.Equal(got, want) {
				t.Errorf("Autolinks(%q) = %+v, want %+v", src, got, want)
			}
		})
	}
}

// TestWWWLiteralUppercasePrefixStaysProse is a PRESERVED-BEHAVIOR PIN on a
// DIVERGENCE, not on a fix: it records what this package does with an
// uppercase "WWW." prefix, which is not what the reference does.
//
// Measured against the frozen reference, whole bodies:
//
//	"see WWW.ex.com b"   ref links "http://WWW.ex.com"   here prose
//	"see WWW.x b"        ref links "http://WWW.x"        here prose
//	"see Www.Ex.Com b"   ref links "http://Www.Ex.Com"   here prose
//
// BOTH of the reference's recognizers are case-insensitive here, so this is
// a link-versus-text divergence with a payload difference: a push sends
// prose where the reference sends a link.
//
// IT IS NOT A PATTERN PROBLEM, which is why the pin sits here rather than a
// widened urlLiteralWWW. goldmark's linkify parser reaches its WWW pattern
// only after a hard-coded, case-SENSITIVE `bytes.HasPrefix(line, "www.")`
// that no option can replace — the schemed branch's equivalent pre-gate IS
// replaceable, and NewParser already opens it for both cases. Spelling the
// prefix `[wW][wW][wW]\.` would therefore change nothing about the raw
// verdict while making the decoded-text scan accept what the raw one still
// refuses, which is the inversion TestURLLiteralRawPatternIsTheWiderOne
// forbids for the schemed literal. Closing it needs an inline parser of
// this package's own.
//
// The lowercase row is in the same document deliberately: it is what makes
// the uppercase rows read as a case rule rather than as a broken prefix.
func TestWWWLiteralUppercasePrefixStaysProse(t *testing.T) {
	t.Parallel()
	const src = "see WWW.ex.com and WWW.x and Www.Ex.Com and www.ex.com b\n"
	want := []string{"http://www.ex.com"}
	if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
		t.Errorf("link URLs of %q = %v, want %v", src, got, want)
	}
	autolinks := NewSource([]byte(src)).Autolinks()
	if len(autolinks) != 1 || autolinks[0].Target != "http://www.ex.com" {
		t.Errorf("Autolinks(%q) = %+v, want the lowercase host alone", src, autolinks)
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
