package dialect

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// rawNodeRegistrations returns the rawNode directive's registration (see
// rawnode.go). Appended in Registrations (decode.go) after the historical
// and extended sets — dispatch order is not significant here: RawNode is
// the one ADF kind no other hook's type assertion matches.
func rawNodeRegistrations() []extension.Registration {
	return []extension.Registration{
		{
			Kind: "rawNode",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"rawNode": func(d *ast.ContainerDirective) extension.Node {
					return &RawBlock{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
				},
			},
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"rawNode": func(d *ast.LeafDirective) extension.Node {
					return &RawLeaf{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
				},
			},
			Texts: map[string]func(*ast.TextDirective) extension.Node{
				"rawNode": func(d *ast.TextDirective) extension.Node {
					return &RawText{Attrs: d.Attrs, Children: d.Children}
				},
			},
			DecodeBlock:  decodeRawNodeBlock,
			DecodeInline: decodeRawNodeInline,
		},
	}
}

// decodeRawNodeBlock converts an adf.RawNode reached at block position: a
// container directive when it carried content (decoded as block
// children), a leaf directive when it did not. The shape decision reads
// the ORIGINAL RawNode.Content, not however many AST nodes it decodes to,
// so a content list that happens to decode away to nothing still renders
// as the container form it came from.
func decodeRawNodeBlock(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	raw, ok := n.(*adf.RawNode)
	if !ok {
		return nil, false
	}
	attrs := rawNodeDirectiveAttrs(raw)
	if len(raw.Content) == 0 {
		return &RawLeaf{Attrs: attrs}, true
	}
	return &RawBlock{Attrs: attrs, Children: ctx.DecodeBlocks(raw.Content)}, true
}

// decodeRawNodeInline converts an adf.RawNode reached at inline position:
// a text directive, with inline children decoded from its content when it
// carried any (see decodeRawNodeBlock on the shape decision).
func decodeRawNodeInline(n adf.Node, ctx extension.DecodeContext) ([]ast.Node, bool) {
	raw, ok := n.(*adf.RawNode)
	if !ok {
		return nil, false
	}
	attrs := rawNodeDirectiveAttrs(raw)
	if len(raw.Content) == 0 {
		return []ast.Node{&RawText{Attrs: attrs}}, true
	}
	return []ast.Node{&RawText{Attrs: attrs, Children: ctx.DecodeInlines(raw.Content)}}, true
}

// rawNodeDirectiveAttrs builds the rawNode directive's attribute payload:
// "type" spelled directly, "data" the JSON encoding of Attrs/Text (omitted
// when both are empty — see encodeRawNodeData).
func rawNodeDirectiveAttrs(raw *adf.RawNode) map[string]string {
	attrs := map[string]string{"type": raw.Type}
	if data, ok := encodeRawNodeData(raw.Attrs, raw.Text); ok {
		attrs["data"] = data
	}
	return attrs
}
