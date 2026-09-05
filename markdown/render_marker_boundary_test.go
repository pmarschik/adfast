package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The "after" character a mark's LAST CHILD is asked about is the mark's own
// CLOSING MARKER, not the byte standing behind the whole mark.
//
// Measured 2026-09-05 against the frozen reference — mdast-util-to-markdown,
// the engine remark-stringify is, driven on hand-built trees so no parse step
// could reinterpret a string first, with emphasis:'_' so its preferred
// delimiter lines up with this renderer's. Four unsafe rows of the reference's
// table read the same "after" and all four answer it with the marker:
//
//	emphasis["see https:"] + text "//x b"  ->  _see https:_//x b
//	emphasis["www."]       + text "..A"    ->  _www\._..A
//	emphasis["x@"]         + text "y.com"  ->  _x\@_&#x79;.com
//	emphasis["a&"]         + text "amp;"   ->  _a&_&#x61;mp;
//
// Threading the byte behind the mark instead got the ':' and '&' rows wrong
// and agreed on the '.' and '@' rows by accident, because '_' and '.' fall in
// the same character class for those two. So a table that pinned only the dot
// would not have told the two threadings apart; the four together do.
var markerAfterCases = []struct {
	name string
	// nodes is the paragraph's inline run, built rather than parsed.
	nodes []ast.Node
	// want is the reference's whole body for that tree.
	want string
}{{
	// THE ':' ROW. Its unsafe entry escapes only before '/', and the closing
	// marker is not one, so the colon stays bare — where the byte behind the
	// emphasis IS a '/' and would have forced the backslash.
	name:  "a colon at an emphasis end sees the marker, not the slash behind it",
	nodes: []ast.Node{&ast.Emphasis{Children: textRun("see https:")}, &ast.Text{Value: "//x b"}},
	want:  "_see https:_//x b\n",
}, {
	name:  "the same colon inside a strong sees its own closer",
	nodes: []ast.Node{&ast.Strong{Children: textRun("https:")}, &ast.Text{Value: "//x"}},
	want:  "**https:**//x\n",
}, {
	name:  "and inside a strikethrough",
	nodes: []ast.Node{&ast.Delete{Children: textRun("https:")}, &ast.Text{Value: "//x"}},
	want:  "~~https:~~//x\n",
}, {
	// THE '&' ROW. Its unsafe entry wants '[#A-Za-z]' next; '_' is neither,
	// so the ampersand stays bare where the 'a' behind the emphasis would
	// have escaped it. The trailing character reference in the want is the
	// separate flanking repair, unchanged by the threading.
	name:  "an ampersand at an emphasis end is not an entity opener",
	nodes: []ast.Node{&ast.Emphasis{Children: textRun("a&")}, &ast.Text{Value: "amp;"}},
	want:  "_a&_&#x61;mp;\n",
}, {
	// THE '.' ROW — a GOOD CASE that passed before and still passes. The
	// reference's dot rule takes '[-.\w]' after, and '_' is a word character,
	// so the escape stays. A fix that simply stopped threading anything would
	// drop this backslash.
	name:  "a www dot at an emphasis end keeps its escape: the marker is a word byte",
	nodes: []ast.Node{&ast.Emphasis{Children: textRun("www.")}, &ast.Text{Value: "..A"}},
	want:  "_www\\._..A\n",
}, {
	// THE '@' ROW — the other good case, and the one the afterLead field was
	// introduced for. '_' is in the email rule's after class, so the escape
	// stays here too.
	name:  "an at sign at an emphasis end keeps its escape for the same reason",
	nodes: []ast.Node{&ast.Emphasis{Children: textRun("x@")}, &ast.Text{Value: "y.com"}},
	want:  "_x\\@_&#x79;.com\n",
}, {
	// A colon whose slash is INSIDE the node is untouched by any threading:
	// the rule reads the next byte of its own text and finds the '/'.
	name:  "a colon followed by its slash inside the node still escapes",
	nodes: []ast.Node{&ast.Emphasis{Children: textRun("see https:// bare")}},
	want:  "_see https\\:// bare_\n",
}}

func textRun(v string) []ast.Node { return []ast.Node{&ast.Text{Value: v}} }

func TestRender_MarkLastChildSeesTheClosingMarker(t *testing.T) {
	t.Parallel()
	for _, c := range markerAfterCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: c.nodes}}}
			if got := Render(root); got != c.want {
				t.Errorf("Render() = %q, want %q", got, c.want)
			}
		})
	}
}
