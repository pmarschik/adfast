package adfast_test

import (
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/ast"
)

// directiveLabelProbes are the label texts the round trip has to carry
// through a [label]: the bracket forms the renderer escapes, the
// backslash that could eat the escape, and the constructs the label's
// own inline parse could swallow.
var directiveLabelProbes = []string{
	"Bracket [x]",
	"a]b",
	"a[b",
	"]",
	"[",
	"a]b[c",
	"[nested [pair]]",
	`a\]b`,
	`trailing\`,
	"[x](https://example.com)",
	"chars [x] back\\slash",
	"plain",
}

// labelForm is one of the three directive shapes, each building a
// document whose only content is a directive carrying the label, and
// reading the label back out of a parsed document.
type labelForm struct {
	build func(label string) ast.Node
	read  func(ast.Node) (string, bool)
	name  string
}

func directiveLabelForms() []labelForm {
	return []labelForm{
		{
			name: "leaf",
			build: func(label string) ast.Node {
				return &ast.Root{Children: []ast.Node{&ast.LeafDirective{
					Name:     "includePage",
					Children: []ast.Node{&ast.Text{Value: label}},
				}}}
			},
			read: func(n ast.Node) (string, bool) {
				kids := ast.Children(n)
				if len(kids) != 1 {
					return "", false
				}
				d, ok := kids[0].(*ast.LeafDirective)
				if !ok {
					return "", false
				}
				return ast.PlainText(d.Children), true
			},
		},
		{
			name: "text",
			build: func(label string) ast.Node {
				return &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
					&ast.TextDirective{Name: "inc", Children: []ast.Node{&ast.Text{Value: label}}},
				}}}}
			},
			read: func(n ast.Node) (string, bool) {
				kids := ast.Children(n)
				if len(kids) != 1 {
					return "", false
				}
				p, ok := kids[0].(*ast.Paragraph)
				if !ok || len(p.Children) != 1 {
					return "", false
				}
				d, ok := p.Children[0].(*ast.TextDirective)
				if !ok {
					return "", false
				}
				return ast.PlainText(d.Children), true
			},
		},
		{
			name: "container",
			build: func(label string) ast.Node {
				return &ast.Root{Children: []ast.Node{&ast.ContainerDirective{
					Name: "sidebar",
					Children: []ast.Node{
						&ast.Paragraph{DirectiveLabel: true, Children: []ast.Node{&ast.Text{Value: label}}},
						&ast.Paragraph{Children: []ast.Node{&ast.Text{Value: "body"}}},
					},
				}}}
			},
			read: func(n ast.Node) (string, bool) {
				kids := ast.Children(n)
				if len(kids) != 1 {
					return "", false
				}
				d, ok := kids[0].(*ast.ContainerDirective)
				if !ok || len(d.Children) == 0 {
					return "", false
				}
				p, ok := d.Children[0].(*ast.Paragraph)
				if !ok || !p.DirectiveLabel {
					return "", false
				}
				return ast.PlainText(p.Children), true
			},
		},
	}
}

// TestDirectiveLabelRoundTrips is the general invariant: whatever the
// renderer writes for a directive [label], the parser reads back as the
// same directive with the same label, and rendering that again produces
// the same bytes.
//
// The minimal repro is the leaf form with the label "Bracket [x]": the
// renderer wrote "::includePage[Bracket \[x\]]" (remark-stringify's own
// escaping) and the label scan ended at the first ']' whatever preceded
// it, so those bytes came back as a paragraph of literal text and a page
// title with a square bracket published as prose instead of as a macro.
func TestDirectiveLabelRoundTrips(t *testing.T) {
	t.Parallel()
	for _, form := range directiveLabelForms() {
		t.Run(form.name, func(t *testing.T) {
			t.Parallel()
			for _, label := range directiveLabelProbes {
				md := adfast.ToMarkdown(form.build(label))
				parsed := adfast.FromMarkdown(md)
				got, ok := form.read(parsed)
				if !ok {
					t.Errorf("label %q rendered %q, which does not re-parse as a %s directive", label, md, form.name)
					continue
				}
				if got != label {
					t.Errorf("label %q rendered %q, which re-parses with the label %q", label, md, got)
				}
				if second := adfast.ToMarkdown(parsed); second != md {
					t.Errorf("label %q is not a render fixpoint:\n first:  %q\n second: %q", label, md, second)
				}
			}
		})
	}
}
