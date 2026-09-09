package markdown

import (
	"strings"
	"testing"
)

// THE LEFT BOUNDARY OF A BARE URL LITERAL, which this package read as
// goldmark's five-byte trigger set (' ', '*', '_', '~', '(') and the
// reference reads three different ways at once. Read out of the frozen
// install rather than inferred:
//
//   - micromark-extension-gfm-autolink-literal, SCHEMED literal:
//     `previousProtocol = !asciiAlpha(code)`. Any byte that is not an ASCII
//     letter opens one.
//   - the same tokenizer, "www." literal: `previousWww` is the closed set
//     `null | '(' | '*' | '_' | '[' | ']' | '~' | line ending | space`.
//   - mdast-util-gfm-autolink-literal's transform, both shapes: start of
//     input, Unicode whitespace, or Unicode punctuation.
//
// A literal links when ANY of the three accepts it, so the union is the
// behavior a round trip has to reproduce. That union is NOT "any Unicode
// punctuation": it also takes a digit and a non-ASCII letter in front of a
// schemed literal, which the table below records as still-divergent.

// punctBoundaryCases is one boundary byte per row, written into a body that
// is otherwise identical, so the only thing a row can measure is whether the
// byte in front of the address opens a literal.
//
// The rows are grouped by what they prove, and the linking and non-linking
// rows are deliberately in ONE table: a widening that swallowed the owned or
// held-back bytes would pass every positive row and fail here.
var punctBoundaryCases = []struct {
	name string
	// boundary is the single byte written between "z" and the address.
	boundary string
	// why records, for a non-linking row, the reason the byte is out. Empty
	// for a linking row.
	why string
	// wwwWant overrides the expected target of the scheme-less body for the
	// one boundary that changes what the address IS rather than whether it
	// links. Empty means "http://" + the written address.
	wwwWant string
	// links is what the parse must produce for the schemed body and for the
	// scheme-less one alike.
	links bool
}{
	// THE WIDENING. Every one of these was prose before punctLinkifyParser
	// and links in the reference.
	{name: "bang", boundary: "!", links: true},
	{name: "quote", boundary: "\"", links: true},
	{name: "dollar", boundary: "$", links: true},
	{name: "percent", boundary: "%", links: true},
	{name: "apostrophe", boundary: "'", links: true},
	{name: "close paren", boundary: ")", links: true},
	{name: "plus", boundary: "+", links: true},
	{name: "comma", boundary: ",", links: true},
	{name: "hyphen", boundary: "-", links: true},
	{name: "dot", boundary: ".", links: true},
	{name: "slash", boundary: "/", links: true},
	{name: "equals", boundary: "=", links: true},
	{name: "greater than", boundary: ">", links: true},
	{name: "question mark", boundary: "?", links: true},
	// '@' links the schemed body at the address alone, and the scheme-less
	// body as an EMAIL that starts one byte earlier — "z@www.a.b" is a
	// well-formed address, so the email literal wins over the www one. Both
	// halves measured against the frozen reference:
	//
	//	"z@http://a.b c"  ->  "http://a.b"        at 2-12
	//	"z@www.a.b c"     ->  "mailto:z@www.a.b"  at 0-9
	{name: "at sign", boundary: "@", links: true, wwwWant: "mailto:z@www.a.b"},
	{name: "close bracket", boundary: "]", links: true},
	// '<' was the last byte held back, and for a reason outside the parser:
	// Source.Autolinks used to read a '<' at Node.Pos as the ANGLE form and so
	// DROPPED the literal instead of placing it (measured with the byte in the
	// set and the sniff still there: tree link present, Autolinks() empty,
	// UnlocatedAutolinks() 1). The form is now carried on the node — see
	// autolinkIsAngle — so the byte is a plain boundary here and the guard
	// below places it at 2-12 like every other row.
	{name: "less than", boundary: "<", links: true},
	{name: "caret", boundary: "^", links: true},
	{name: "backtick", boundary: "`", links: true},
	{name: "open brace", boundary: "{", links: true},
	{name: "pipe", boundary: "|", links: true},
	{name: "close brace", boundary: "}", links: true},

	// PRESERVED BEHAVIOR. goldmark's own trigger set, which the wrapped
	// parser still answers at its own extent. A change that replaced the
	// stock recognizer rather than extending past it fails here.
	{name: "open paren", boundary: "(", links: true},
	{name: "asterisk", boundary: "*", links: true},
	{name: "underscore", boundary: "_", links: true},
	{name: "tilde", boundary: "~", links: true},
	{name: "space", boundary: " ", links: true},

	// HELD BACK: the three bytes of a character reference. A trigger at any
	// of them splits the text node there (goldmark ends a line with
	// parent.AppendChild, not MergeOrAppendTextSegment), and the renderer's
	// character-reference rule reads the bytes around a '&' inside ONE text
	// node — so the split makes it defuse a reference that needs none.
	// Measured with '&' in the set, "x&#x20;" formatted to "x\&#x20;" where
	// the base is a fixpoint.
	{name: "ampersand", boundary: "&", links: false, why: "character-reference byte"},
	{name: "hash", boundary: "#", links: false, why: "character-reference byte"},
	{name: "semicolon", boundary: ";", links: false, why: "character-reference byte"},

	// UNREACHABLE: another parser answers first and does not decline. The
	// link parser returns the bracket as text, so nothing behind it is asked.
	{name: "open bracket", boundary: "[", links: false, why: "owned by the link parser"},

	// OWNED: escapedLinkifyParser triggers on the backslash and reads the
	// byte after it as the boundary, so a literal backslash in front of an
	// address is not a boundary here.
	{name: "backslash", boundary: "\\", links: false, why: "owned by escapedLinkifyParser"},
}

// TestPunctBoundaryOpensASchemedLiteral is the schemed half. Every row is
// asserted on the tree AND on the raw spans, because the two views are what
// a boundary fix can most easily split: a decoded-text scan would satisfy
// the first and leave Source.Autolinks silent.
func TestPunctBoundaryOpensASchemedLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range punctBoundaryCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := "z" + c.boundary + "http://a.b c\n"
			var want []string
			if c.links {
				want = []string{"http://a.b"}
			}
			if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
				t.Errorf("link URLs of %q = %v, want %v (%s)", src, got, want, c.why)
			}
			spans := NewSource([]byte(src)).Autolinks()
			if got := len(spans) == 1; got != c.links {
				t.Errorf("Autolinks(%q) = %+v, want linked=%v", src, spans, c.links)
			}
			if c.links && len(spans) == 1 {
				// The boundary byte is not part of the address.
				if spans[0].Span != (Span{2, 12}) || spans[0].Target != "http://a.b" {
					t.Errorf("Autolinks(%q)[0] = %+v, want the address alone at 2-12", src, spans[0])
				}
			}
		})
	}
}

// TestPunctBoundaryOpensAWWWLiteral is the scheme-less half. It is a separate
// test rather than a column because the address has to come out COMPLETED —
// goldmark prepends the scheme on its own www branch, so a new parser that
// forgot to would put a relative href in the tree and pass every check that
// only asks whether a link exists.
func TestPunctBoundaryOpensAWWWLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range punctBoundaryCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := "z" + c.boundary + "www.a.b c\n"
			var want []string
			switch {
			case c.wwwWant != "":
				want = []string{c.wwwWant}
			case c.links:
				want = []string{"http://www.a.b"}
			}
			if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
				t.Errorf("link URLs of %q = %v, want %v (%s)", src, got, want, c.why)
			}
		})
	}
}

// TestPunctBoundaryLiteralIsARenderFixpoint is the round-trip half, and it is
// the one that makes the widening safe rather than merely closer. A boundary
// byte that starts linking changes what the renderer must write on BOTH legs:
// the address goes out as a link, and the byte in front of it goes out as
// prose that must not re-open a different literal on the re-parse.
func TestPunctBoundaryLiteralIsARenderFixpoint(t *testing.T) {
	t.Parallel()
	for _, c := range punctBoundaryCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, addr := range []string{"http://a.b", "www.a.b"} {
				src := "z" + c.boundary + addr + " c\n"
				first := Render(Parse([]byte(src)))
				second := Render(Parse([]byte(first)))
				if first != second {
					t.Errorf("render of %q is not a fixpoint:\n first:  %q\n second: %q",
						src, first, second)
				}
				if got := linkURLsOf(Parse([]byte(first))); len(got) != len(linkURLsOf(Parse([]byte(src)))) {
					t.Errorf("re-parse of %q changed the link count: %q -> %v", src, first, got)
				}
			}
		})
	}
}

// TestPunctBoundarySetsAgree pins the two boundary constants against each
// other. escapedLinkifyBoundaries has to be the unescaped set minus the space
// — only ASCII punctuation is escapable — or an escape starts deciding
// whether a URL is a link, which is the defect escapedLinkifyParser exists to
// prevent. A widening of one constant alone fails here.
func TestPunctBoundarySetsAgree(t *testing.T) {
	t.Parallel()
	for _, b := range linkifyBoundaryBytes + punctLinkifyBoundaries {
		if b == ' ' {
			if strings.ContainsRune(escapedLinkifyBoundaries, b) {
				t.Errorf("escapedLinkifyBoundaries holds a space, which no backslash can escape")
			}
			continue
		}
		if !strings.ContainsRune(escapedLinkifyBoundaries, b) {
			t.Errorf("%q is an unescaped boundary but not an escaped one", b)
		}
	}
	for _, b := range escapedLinkifyBoundaries {
		if !strings.ContainsRune(linkifyBoundaryBytes+punctLinkifyBoundaries, b) {
			t.Errorf("%q is an escaped boundary but not an unescaped one", b)
		}
	}
}

// TestBoundaryLiteralsAllResolveToASpan is the guard that pairs the two
// boundary constants with the SPAN VIEW, which is the one place a widening can
// do damage that no linking assertion notices.
//
// Source.Autolinks does not read the parser's sets. It resolves an autolink's
// written extent from Node.Pos — the byte BEFORE the address — so a boundary
// byte the parser starts claiming arrives at that view as a byte it has to
// place, and a byte it cannot place is DROPPED: counted in UnlocatedAutolinks
// and reported nowhere. A link assertion still passes, because the tree link
// is fine.
//
// THAT IS NOT HYPOTHETICAL, and it is why this test exists rather than a
// linking table alone. The view used to decide an autolink's FORM by sniffing
// Node.Pos for a '<', justified by no linkify parser triggering on that byte —
// so claiming '<' here made "z<http://a.b c" build a correct AutoLink, read it
// as the angle form, find no '>' behind the address, and drop it. Measured with
// '<' in punctLinkifyBoundaries and the sniff still in place, this test failed
// on both bodies with UnlocatedAutolinks() == 1 and an empty Autolinks(). The
// form is now carried on the node instead (autolinkIsAngle), the byte is in the
// set, and both bodies place at 2-12 and 2-9.
//
// So the property is asserted the honest way: every boundary byte this package
// claims must leave a bare literal that resolves to a TIGHT span AND leave the
// unlocated count at zero.
func TestBoundaryLiteralsAllResolveToASpan(t *testing.T) {
	t.Parallel()
	for _, b := range linkifyBoundaryBytes + punctLinkifyBoundaries {
		t.Run(string(b), func(t *testing.T) {
			t.Parallel()
			for _, addr := range []string{"http://a.b", "www.a.b"} {
				src := "z" + string(b) + addr + " c\n"
				s := NewSource([]byte(src))
				got := s.Autolinks()
				if n := s.UnlocatedAutolinks(); n != 0 {
					t.Errorf("UnlocatedAutolinks(%q) = %d, want 0: the span view dropped an autolink it could not place", src, n)
				}
				if len(linkURLsOf(Parse([]byte(src)))) == 0 {
					continue // '[' and '@'+www are covered by the tables above.
				}
				if len(got) != 1 {
					t.Errorf("Autolinks(%q) = %+v, want exactly one", src, got)
					continue
				}
				if got[0].Email {
					continue // "z@www.a.b" is one email address; it starts at 0.
				}
				if want := (Span{2, 2 + len(addr)}); got[0].Span != want {
					t.Errorf("Autolinks(%q)[0].Span = %+v, want %+v", src, got[0].Span, want)
				}
			}
		})
	}
}

// entityOpeningBytes are the three bytes a character reference is spelled
// with: the '&' that opens one, the '#' that makes it numeric, and the ';'
// that closes it. Named here because the property below is about the SET they
// are absent from, not about any one of them.
const entityOpeningBytes = "&#;"

// TestNoEntityByteIsALinkifyBoundary IS A PIN, NOT A REGRESSION TEST. It
// passes at the commit that adds it and it fails nothing today; its whole job
// is to fail on a FUTURE widening, with the reason attached, so that widening
// is a decision rather than an accident.
//
// THE DEFECT IT GUARDS IS CURRENTLY UNREACHABLE, and the pin exists because
// the unreachability is a property of two constants and nothing stronger.
// goldmark ends a line by appending its trailing run with parent.AppendChild
// rather than MergeOrAppendTextSegment (parser.parseBlock), so every inline
// TRIGGER byte splits the trailing text node at that byte. render_escape.go's
// character-reference rule reads the bytes around a '&' INSIDE ONE TEXT NODE,
// so a split it did not expect makes it defuse a reference that needs no
// defusing — "x&#x20;" going out as "x\&#x20;" where the base is a fixpoint,
// which changes what the document MEANS and not merely how it is spelled.
//
// What makes it unreachable is that neither trigger set holds a byte a
// character reference can be opened at: an entity is "&" then '#' or a letter,
// so a trigger at any of the three bytes above is the only way to split one,
// and neither linkifyBoundaryBytes nor punctLinkifyBoundaries holds any of
// them. The second half below asserts the damage directly, because the
// assertion above is a statement about two constants and a widening could
// arrive by a different route. Measured with '&', '#' and ';' added to
// punctLinkifyBoundaries, against the base in the left column:
//
//	                base                          with the three bytes in
//	"x&#x20;"       "x&#x20;"                     "x\&#x20;"          CHANGED
//	"x&plus;y"      "x+y"                         "x\&plus;y"         CHANGED
//	"x&#x20;*a*"    "x _a_"                       "x _a_"
//	"x&amp;*a*"     "x&_a_"                       "x&_a_"
//	"x&*a*"         "x&_a_"                       "x&_a_"
//	"***0*0**0"     "**_&#x30;_&#x30;**&#x30;"    same, but re-rendering as
//	                a fixpoint                    "…**\&#x30;" — NOT a fixpoint
//
// The first two rows are the meaning change: a character reference the author
// wrote is escaped into the literal text of itself. The last row is the same
// damage arriving as a lost round trip rather than a changed render, which is
// why both shapes of assertion are here — a fixpoint check alone passes the
// first two rows, and an output check alone passes the last.
//
// THE FIX IS NOT HERE if this ever fails. Widening a set to one of these bytes
// closes a real divergence — the reference opens a bare URL literal after all
// three — and the way to close it is to make the escape rule read the rendered
// output rather than the text node it is inside. Deleting this pin to get a
// green run reintroduces a silent meaning change; see punctLinkifyBoundaries
// for the measurement that took the bytes out in the first place.
func TestNoEntityByteIsALinkifyBoundary(t *testing.T) {
	t.Parallel()
	for _, set := range []struct{ name, bytes string }{
		{"linkifyBoundaryBytes", linkifyBoundaryBytes},
		{"punctLinkifyBoundaries", punctLinkifyBoundaries},
		// Derived from punctLinkifyBoundaries today, asserted anyway: an
		// escape must not decide whether a URL is a link, so if the two ever
		// stop tracking each other this byte class has to stay out of both.
		{"escapedLinkifyBoundaries", escapedLinkifyBoundaries},
	} {
		for _, b := range entityOpeningBytes {
			if strings.ContainsRune(set.bytes, b) {
				t.Errorf("%s holds %q, a character-reference byte: a trigger there splits the text node "+
					"a character reference lives in, and render_escape.go then defuses a reference that needs none",
					set.name, b)
			}
		}
	}
	// The shapes the absence protects, pinned as exact output: a widening that
	// slipped past the assertion above shows up here as a character reference
	// escaped into the literal text of itself.
	for _, c := range []struct{ src, want string }{
		{"x&#x20;\n", "x&#x20;\n"},
		{"x&plus;y\n", "x+y\n"},
		{"x&#x20;*a*\n", "x _a_\n"},
		{"x&amp;*a*\n", "x&_a_\n"},
		{"x&*a*\n", "x&_a_\n"},
	} {
		if got := Render(Parse([]byte(c.src))); got != c.want {
			t.Errorf("render of %q = %q, want %q: a character reference was defused", c.src, got, c.want)
		}
	}
	// The same damage in the shape where it costs a ROUND TRIP rather than the
	// first render — the emphasis nest renders identically either way, and
	// only its re-render tells the two apart.
	const nested = "***0*0**0\n"
	first := Render(Parse([]byte(nested)))
	if second := Render(Parse([]byte(first))); first != second {
		t.Errorf("render of %q is not a fixpoint:\n first:  %q\n second: %q", nested, first, second)
	}
}
