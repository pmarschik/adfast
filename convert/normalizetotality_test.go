// This file is an external test package on purpose: the two rules below
// separate Normalize from NormalizeFormat, and the facade exposes only
// the format leg, so a test driven through it could not tell them apart.
package convert_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/dialect"
)

// inlineKinds lists the kinds left in the first paragraph of a
// normalized tree, with a text leaf spelled out so a merge is visible.
func inlineKinds(t *testing.T, n ast.Node) string {
	t.Helper()
	root, ok := n.(*ast.Root)
	if !ok {
		t.Fatalf("normalize returned %T, want *ast.Root", n)
	}
	if len(root.Children) != 1 {
		t.Fatalf("normalize returned %d blocks, want 1", len(root.Children))
	}
	para, ok := root.Children[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("normalize returned a %T block, want *ast.Paragraph", root.Children[0])
	}
	var parts []string
	for _, kid := range para.Children {
		if txt, isText := kid.(*ast.Text); isText {
			parts = append(parts, "text("+txt.Value+")")
			continue
		}
		parts = append(parts, kid.Kind())
	}
	return strings.Join(parts, " ")
}

// leg names one of the two normalization legs so a table can run over
// both. The field order is the one govet's fieldalignment asks for.
type leg struct {
	norm func(ast.Node, ...convert.Option) ast.Node
	name string
}

func bothLegs() []leg {
	return []leg{{convert.NormalizeFormat, "NormalizeFormat"}, {convert.Normalize, "Normalize"}}
}

// inlineTree wraps one inline node in the paragraph the two legs read.
func inlineTree(inline ast.Node) *ast.Root {
	return &ast.Root{Children: []ast.Node{
		&ast.Paragraph{Children: []ast.Node{
			&ast.Text{Value: "a "},
			inline,
			&ast.Text{Value: " b"},
		}},
	}}
}

// TestOnlyTheFormatLegKeepsADirectiveTheKindCannotRead: the two legs
// answer differently on purpose. A typed inline kind whose canonical
// re-derivation produces nothing has no ADF node to become, so the
// encode leg drops it — that is what keeps ToADF(NormalizeFormat(n))
// equal to ToADF(n) even though the format leg keeps the node. Only the
// format leg is total; Normalize is not, and must not become so here.
func TestOnlyTheFormatLegKeepsADirectiveTheKindCannotRead(t *testing.T) {
	t.Parallel()
	// A status whose text is spelled nowhere the kind reads it: the
	// grammar puts the text in the LABEL, and there is no label.
	unreadable := func() ast.Node {
		return &dialect.Status{Attrs: map[string]string{"color": "red", "text": "Done"}}
	}
	if got, want := inlineKinds(t, convert.NormalizeFormat(inlineTree(unreadable()))), "text(a ) status text( b)"; got != want {
		t.Errorf("NormalizeFormat\n got  %q\n want %q", got, want)
	}
	if got, want := inlineKinds(t, convert.Normalize(inlineTree(unreadable()))), "text(a  b)"; got != want {
		t.Errorf("Normalize\n got  %q\n want %q", got, want)
	}

	// GOOD CASE — a payload the kind DOES read re-derives to a node on
	// both legs, so the keep is not what puts it there.
	readable := func() ast.Node {
		return &dialect.Status{Attrs: map[string]string{"color": "red"}, Children: []ast.Node{&ast.Text{Value: "Done"}}}
	}
	for _, tc := range bothLegs() {
		if got, want := inlineKinds(t, tc.norm(inlineTree(readable()))), "text(a ) status text( b)"; got != want {
			t.Errorf("%s of a readable status\n got  %q\n want %q", tc.name, got, want)
		}
	}
}

// TestNeitherLegKeepsABlockDialectKindInInlinePosition: the keep goes
// only to a kind that declares an inline form. A block dialect kind has
// no inline ADF form and no inline markdown spelling either — its
// markdown is a ":::name" fence — so keeping it would write a fence
// inside a paragraph's inline run. Only a hand-built tree can put one
// here; no parse does.
func TestNeitherLegKeepsABlockDialectKindInInlinePosition(t *testing.T) {
	t.Parallel()
	block := func() ast.Node { return &dialect.Panel{PanelType: "info", Attrs: map[string]string{"a": "b"}} }
	for _, tc := range bothLegs() {
		if got, want := inlineKinds(t, tc.norm(inlineTree(block()))), "text(a  b)"; got != want {
			t.Errorf("%s of an inline panel\n got  %q\n want %q", tc.name, got, want)
		}
	}

	// GOOD CASE — the same panel in the block position it belongs in is
	// untouched by either leg, so the drop above is about the position,
	// not about the kind being unwelcome.
	for _, tc := range bothLegs() {
		root, ok := tc.norm(&ast.Root{Children: []ast.Node{block()}}).(*ast.Root)
		if !ok {
			t.Fatalf("%s returned a non-root", tc.name)
		}
		if len(root.Children) != 1 || root.Children[0].Kind() != "panel" {
			t.Errorf("%s of a block-position panel dropped it: %v", tc.name, root.Children)
		}
	}
}
