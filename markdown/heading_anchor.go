package markdown

import (
	"regexp"

	gast "github.com/yuin/goldmark/ast"

	"github.com/pmarschik/adfast/ast"
)

// Heading anchors: the trailing {#id} on a heading line (the pandoc /
// remark-heading-id spelling) parses into ast.Heading.ID instead of
// staying literal text.
//
// goldmark ships parser.WithHeadingAttribute(), which strips the same
// suffix at block-parse time, but it is measured too broad for this
// dialect: it also consumes {.class}, {key=value} and the empty {#},
// none of which the dialect models, so enabling it would silently drop
// text that remark keeps. The strip below runs on the parsed heading
// instead and accepts only the one shape the dialect can render back.

// headingAnchorRe matches the trailing {#id} of a heading, with the id
// charset ast.HeadingIDPattern defines (that doc comment carries the why).
// Anything else ("{#}", "{#a b}", "{.class}", "{#a:b}") stays literal.
// Sharing the one pattern keeps the strip, the render escape and the
// addon-side lift check exact mirrors of each other, and keeps the id free
// of markdown escapes so it needs no decode here and no re-escape on the
// way out.
var headingAnchorRe = regexp.MustCompile(`\{#(` + ast.HeadingIDPattern + `)\}$`)

// splitHeadingAnchor detaches a trailing {#id} from n's inline content and
// returns the id ("" when the heading has none). The anchor must sit at
// the very end of the heading, in plain text (not inside a code span or
// emphasis), and be separated from the heading text by whitespace — or be
// the heading's entire content, as in "## {#id}". A backslash-escaped
// brace ("## Title \{#id}") is left alone, which is how a literal {#…}
// survives the round trip.
//
// It mutates the goldmark tree (shrinking or dropping the last text node)
// and so must run before the inline conversion reads it.
func splitHeadingAnchor(n *gast.Heading, src []byte) string {
	run := trailingTextRun(n)
	if len(run) == 0 {
		return ""
	}
	start, stop := run[0].Segment.Start, run[len(run)-1].Segment.Stop
	val := src[start:stop]
	m := headingAnchorRe.FindSubmatchIndex(val)
	if len(m) == 0 || escapedAt(src, start+m[0], start) {
		return ""
	}
	cut := m[0]
	switch {
	case cut > 0 && isSpaceOrTab(val[cut-1]):
		for cut > 0 && isSpaceOrTab(val[cut-1]) {
			cut--
		}
	case cut == 0 && n.FirstChild() == run[0]:
		// "## {#id}": the anchor is the whole heading, which leaves an
		// empty heading behind (pandoc does the same).
	default:
		// "## Title{#id}" — no separating whitespace, so not an anchor.
		return ""
	}
	id := string(val[m[2]:m[3]])
	cutAt := start + cut
	for _, tn := range run {
		switch {
		case tn.Segment.Start >= cutAt:
			n.RemoveChild(n, tn)
		case tn.Segment.Stop > cutAt:
			tn.Segment = tn.Segment.WithStop(cutAt)
		}
	}
	return id
}

// trailingTextRun returns n's maximal run of trailing plain-text children,
// in source order, or nil when the heading does not end in plain text.
//
// The run matters because goldmark splits a text node at every inline
// delimiter candidate whether or not it forms a span: "T {#a_b}" arrives
// as three text nodes ("T {#a", "_", "b}"), so the anchor shape spans
// several of them. Only source-contiguous nodes join the run, which also
// stops it at a soft break — so a setext heading's anchor is matched
// against its last line alone.
func trailingTextRun(n *gast.Heading) []*gast.Text {
	var run []*gast.Text
	for c := n.LastChild(); c != nil; c = c.PreviousSibling() {
		tn, ok := c.(*gast.Text)
		if !ok || tn.IsRaw() {
			break
		}
		if len(run) > 0 && tn.Segment.Stop != run[0].Segment.Start {
			break
		}
		run = append([]*gast.Text{tn}, run...)
	}
	return run
}

// escapeHeadingAnchorTail escapes the opening brace of a trailing {#id}
// that a heading's own text ends with, so it renders as literal text
// instead of re-parsing as an anchor. It is the exact inverse of
// splitHeadingAnchor's acceptance rule — including the requirement that
// the brace follow whitespace or start the heading — so shapes the parser
// leaves alone (a :directive{#id}, "{#a b}", an already-escaped brace)
// are passed through untouched.
func escapeHeadingAnchorTail(inner string) string {
	m := headingAnchorRe.FindStringIndex(inner)
	if m == nil {
		return inner
	}
	at := m[0]
	if at > 0 && !isSpaceOrTab(inner[at-1]) {
		return inner
	}
	if escapedAt([]byte(inner), at, 0) {
		return inner
	}
	return inner[:at] + `\` + inner[at:]
}

// isSpaceOrTab reports whether c is one of the two inline whitespace
// bytes a heading line can hold (a heading is always a single line).
func isSpaceOrTab(c byte) bool { return c == ' ' || c == '\t' }

// escapedAt reports whether the byte at pos is preceded by an odd number
// of backslashes — i.e. is markdown-escaped — scanning no further back
// than floor.
func escapedAt(src []byte, pos, floor int) bool {
	n := 0
	for i := pos - 1; i >= floor && src[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}
