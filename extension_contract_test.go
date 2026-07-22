package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// testInline is a minimal inline extension kind used by the contract
// tests: it renders as :NAME[children] and encodes by degrading to its
// children.
type testInline struct {
	name     string
	children []ast.Node
}

func (n *testInline) Kind() string                  { return n.name }
func (n *testInline) ChildNodes() []ast.Node        { return n.children }
func (n *testInline) SetChildNodes(kids []ast.Node) { n.children = kids }
func (*testInline) MarkdownLead() byte              { return ':' }
func (n *testInline) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective(n.name, nil, n.children)
}

func (n *testInline) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return ctx.EncodeInlines(n.children)
}

// testInlineReg builds a text-directive registration for testInline.
func testInlineReg(kind string, mark func(adf.Mark, []ast.Node) (ast.Node, bool), inline func(adf.Node, extension.DecodeContext) ([]ast.Node, bool)) extension.Registration {
	return extension.Registration{
		Kind: kind,
		Texts: map[string]func(*ast.TextDirective) extension.Node{
			kind: func(d *ast.TextDirective) extension.Node {
				return &testInline{name: kind, children: d.Children}
			},
		},
		DecodeTextMark: mark,
		DecodeInline:   inline,
	}
}

func TestDecodeTextMark_CustomMarkKind(t *testing.T) {
	// A custom ADF mark kind ("highlight", unknown to the model → RawMark)
	// decodes through a user DecodeTextMark hook into a typed node.
	reg := testInlineReg("hl", func(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
		raw, ok := mark.(*adf.RawMark)
		if !ok || raw.Type != "highlight" {
			return nil, false
		}
		return &testInline{name: "hl", children: inner}, true
	}, nil)

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{
			&adf.Text{Text: "lit", Marks: []adf.Mark{&adf.RawMark{Type: "highlight"}}},
		}},
	}}
	out := adfToMD(doc, WithExtensions(reg))
	if !strings.Contains(out, ":hl[lit]") {
		t.Errorf("custom mark did not decode through DecodeTextMark: %q", out)
	}
	// Without the registration the mark drops (historical behavior).
	if out := adfToMD(doc); !strings.Contains(out, "lit") || strings.Contains(out, ":hl") {
		t.Errorf("unregistered mark should drop, got %q", out)
	}
}

func TestDecodeTextMark_UserOverridesDialect(t *testing.T) {
	// A user hook claiming the underline mark wins over the dialect's :u.
	reg := testInlineReg("uline", func(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
		if _, ok := mark.(*adf.Underline); !ok {
			return nil, false
		}
		return &testInline{name: "uline", children: inner}, true
	}, nil)

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{
			&adf.Text{Text: "under", Marks: []adf.Mark{&adf.Underline{}}},
		}},
	}}
	out := adfToMD(doc, WithExtensions(reg))
	if !strings.Contains(out, ":uline[under]") {
		t.Errorf("user DecodeTextMark did not override dialect: %q", out)
	}
	if out := adfToMD(doc); !strings.Contains(out, ":u[under]") {
		t.Errorf("dialect underline decode broken: %q", out)
	}
}

func TestDecodeInline_UserOverridesDialect(t *testing.T) {
	// A user DecodeInline hook claiming ADF status nodes wins over the
	// dialect's :status decode.
	reg := testInlineReg("st", nil, func(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
		status, ok := n.(*adf.Status)
		if !ok || status.Text == nil {
			return nil, false
		}
		return []ast.Node{&testInline{name: "st", children: []ast.Node{&ast.Text{Value: *status.Text}}}}, true
	})

	text := "Ready"
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{&adf.Status{Text: &text}}},
	}}
	out := adfToMD(doc, WithExtensions(reg))
	if !strings.Contains(out, ":st[Ready]") {
		t.Errorf("user DecodeInline did not override dialect: %q", out)
	}
	if out := adfToMD(doc); !strings.Contains(out, ":status[Ready]") {
		t.Errorf("dialect status decode broken: %q", out)
	}
}

func TestExtensions_DuplicateUserNamesPanic(t *testing.T) {
	a := testInlineReg("a", nil, func(adf.Node, extension.DecodeContext) ([]ast.Node, bool) { return nil, false })
	b := testInlineReg("b", nil, func(adf.Node, extension.DecodeContext) ([]ast.Node, bool) { return nil, false })
	// Both register the text directive name "a".
	b.Texts = map[string]func(*ast.TextDirective) extension.Node{
		"a": func(d *ast.TextDirective) extension.Node { return &testInline{name: "b", children: d.Children} },
	}

	assertPanics := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("duplicate user registration names did not panic")
				}
			}()
			fn()
		})
	}
	assertPanics("parse side", func() { mdToADF("x\n", WithExtensions(a, b)) })
	assertPanics("decode side", func() {
		adfToMD(adf.Doc{Type: "doc", Version: 1}, WithExtensions(a, b))
	})
}

func TestExtensions_UserParsePromotionOverridesDialect(t *testing.T) {
	// A user registration for the dialect's :status name wins promotion.
	reg := testInlineReg("status", nil, func(adf.Node, extension.DecodeContext) ([]ast.Node, bool) { return nil, false })
	doc := mdToADF(":status[Ready]{color=\"green\"}\n", WithExtensions(reg))
	// testInline encodes by degrading to children, so the ADF must hold
	// plain text instead of the dialect's status node.
	found := false
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			switch tn := n.(type) {
			case *adf.Status:
				t.Fatalf("dialect promotion ran despite user override")
			case *adf.Text:
				if tn.Text == "Ready" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("user-promoted node did not encode its children")
	}
}
