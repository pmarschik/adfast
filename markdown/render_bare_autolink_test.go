package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The window a link inside emphasis, strong or strikethrough is judged in
// holds the construct's own markers on both sides, then the parent run's word.
//
// Pinned on the window bytes rather than on a rendered document: no closing
// marker the renderer writes ('_', '*', "~~") is a byte a URL or address
// literal ends on, so dropping the closer from the right edge changes no
// render measured (a sweep of three wrappers, five links and thirty-odd
// tails). The window is still the bytes the reparse sees, and the left edge,
// where the opener does matter, is built the same way.
func TestWrappedWordHoldsTheMarkers(t *testing.T) {
	t.Parallel()
	link := &ast.Link{URL: "mailto:a@b.co", Bare: true, Children: []ast.Node{&ast.Text{Value: "a@b.co"}}}
	nodes := []ast.Node{&ast.Emphasis{Children: []ast.Node{link}}, &ast.Text{Value: "-x"}}
	var b strings.Builder
	b.WriteString("y x-")
	r := &mdRenderer{}
	closeProblem := false
	word := r.wrappedWord(&b, nodes, 0, 0, "_", &closeProblem, &inlineContext{})
	if word.before != "x-_" {
		t.Errorf("before = %q, want %q", word.before, "x-_")
	}
	if got, ok := word.after(); got != "_-x" || !ok {
		t.Errorf("after() = (%q, %v), want (%q, true)", got, ok, "_-x")
	}
}
