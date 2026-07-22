package extension_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// goodNode satisfies every structural requirement: ast.Parent,
// ContainerForm, and InlineLead.
type goodNode struct{ children []ast.Node }

func (*goodNode) Kind() string                                 { return "good" }
func (n *goodNode) ChildNodes() []ast.Node                     { return n.children }
func (n *goodNode) SetChildNodes(kids []ast.Node)              { n.children = kids }
func (*goodNode) ContainerDirectiveForm()                      {}
func (*goodNode) MarkdownLead() byte                           { return ':' }
func (*goodNode) RenderMarkdown(extension.RenderContext)       {}
func (*goodNode) EncodeADF(extension.EncodeContext) []adf.Node { return nil }

// bareNode implements extension.Node but none of the structural
// interfaces (no ast.Parent, no ContainerForm, no InlineLead).
type bareNode struct{}

func (*bareNode) Kind() string                                 { return "bare" }
func (*bareNode) RenderMarkdown(extension.RenderContext)       {}
func (*bareNode) EncodeADF(extension.EncodeContext) []adf.Node { return nil }

func decodeNever(adf.Node, extension.DecodeContext) (ast.Node, bool) { return nil, false }

func TestValidate_PrototypeChecks(t *testing.T) {
	good := extension.Registration{
		Kind:        "good",
		Containers:  map[string]func(*ast.ContainerDirective) extension.Node{"good": func(*ast.ContainerDirective) extension.Node { return &goodNode{} }},
		Texts:       map[string]func(*ast.TextDirective) extension.Node{"good": func(*ast.TextDirective) extension.Node { return &goodNode{} }},
		DecodeBlock: decodeNever,
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid registration rejected: %v", err)
	}

	tests := []struct {
		name string
		want string
		reg  extension.Registration
	}{
		{
			name: "nil prototype",
			reg: extension.Registration{
				Kind:        "niler",
				Leaves:      map[string]func(*ast.LeafDirective) extension.Node{"niler": func(*ast.LeafDirective) extension.Node { return nil }},
				DecodeBlock: decodeNever,
			},
			want: "returned nil",
		},
		{
			name: "missing ast.Parent",
			reg: extension.Registration{
				Kind:        "bare",
				Leaves:      map[string]func(*ast.LeafDirective) extension.Node{"bare": func(*ast.LeafDirective) extension.Node { return &bareNode{} }},
				DecodeBlock: decodeNever,
			},
			want: "ast.Parent",
		},
		{
			name: "container missing ContainerForm",
			reg: extension.Registration{
				Kind:        "noform",
				Containers:  map[string]func(*ast.ContainerDirective) extension.Node{"noform": func(*ast.ContainerDirective) extension.Node { return &inlineOnlyNode{} }},
				DecodeBlock: decodeNever,
			},
			want: "ContainerForm",
		},
		{
			name: "text missing InlineLead",
			reg: extension.Registration{
				Kind:        "nolead",
				Texts:       map[string]func(*ast.TextDirective) extension.Node{"nolead": func(*ast.TextDirective) extension.Node { return &blockOnlyNode{} }},
				DecodeBlock: decodeNever,
			},
			want: "InlineLead",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.reg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

// inlineOnlyNode has Parent + InlineLead but no ContainerForm.
type inlineOnlyNode struct{ children []ast.Node }

func (*inlineOnlyNode) Kind() string                                 { return "inlineOnly" }
func (n *inlineOnlyNode) ChildNodes() []ast.Node                     { return n.children }
func (n *inlineOnlyNode) SetChildNodes(kids []ast.Node)              { n.children = kids }
func (*inlineOnlyNode) MarkdownLead() byte                           { return ':' }
func (*inlineOnlyNode) RenderMarkdown(extension.RenderContext)       {}
func (*inlineOnlyNode) EncodeADF(extension.EncodeContext) []adf.Node { return nil }

// blockOnlyNode has Parent + ContainerForm but no InlineLead.
type blockOnlyNode struct{ children []ast.Node }

func (*blockOnlyNode) Kind() string                                 { return "blockOnly" }
func (n *blockOnlyNode) ChildNodes() []ast.Node                     { return n.children }
func (n *blockOnlyNode) SetChildNodes(kids []ast.Node)              { n.children = kids }
func (*blockOnlyNode) ContainerDirectiveForm()                      {}
func (*blockOnlyNode) RenderMarkdown(extension.RenderContext)       {}
func (*blockOnlyNode) EncodeADF(extension.EncodeContext) []adf.Node { return nil }

func TestValidate_DecodeTextMarkCountsAsDecodeHook(t *testing.T) {
	reg := extension.Registration{
		Kind:  "markonly",
		Texts: map[string]func(*ast.TextDirective) extension.Node{"markonly": func(*ast.TextDirective) extension.Node { return &goodNode{} }},
		DecodeTextMark: func(adf.Mark, []ast.Node) (ast.Node, bool) {
			return nil, false
		},
	}
	if err := reg.Validate(); err != nil {
		t.Errorf("DecodeTextMark-only registration rejected: %v", err)
	}
}

func TestValidateSet_DuplicateNames(t *testing.T) {
	mk := func(kind, name string) extension.Registration {
		return extension.Registration{
			Kind:        kind,
			Texts:       map[string]func(*ast.TextDirective) extension.Node{name: func(*ast.TextDirective) extension.Node { return &goodNode{} }},
			DecodeBlock: decodeNever,
		}
	}
	if err := extension.ValidateSet([]extension.Registration{mk("a", "x"), mk("b", "y")}); err != nil {
		t.Fatalf("distinct names rejected: %v", err)
	}
	err := extension.ValidateSet([]extension.Registration{mk("a", "x"), mk("b", "x")})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate names accepted: %v", err)
	}
}
