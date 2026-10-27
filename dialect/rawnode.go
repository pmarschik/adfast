package dialect

import (
	"strconv"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// RawBlock, RawLeaf, and RawText are the rawNode directive: the lossless
// markdown projection of an adf.RawNode (an ADF node kind adfast's typed
// model does not know). Before this, the kind was the one lossy hop in an
// otherwise lossless ADF⇄ADF pipeline — the markdown projection dropped it
// or recursed into its first child, discarding the wrapper's own Type,
// Attrs, and Marks (adf/lossless_test.go already proves the ADF⇄ADF leg
// round-trips RawNode perfectly; this closes the ADF→markdown→ADF leg to
// match).
//
// Which Go type a RawNode decodes to is fixed by WHERE it sat and WHETHER
// it carried content, not by anything about the unknown kind itself:
//
//   - block position, Content non-empty — RawBlock (:::rawNode, a fenced
//     container; Content decodes as block children)
//   - block position, Content empty — RawLeaf (::rawNode, a leaf
//     directive — there is nothing to hold as a body)
//   - inline position, Content empty or not — RawText (:rawNode, a text
//     directive). Content decodes as inline children when present;
//     ast.LeafDirective is structurally block-only (inlineRenderContext's
//     WriteLeafDirective is a documented no-op in that position), so the
//     content-less inline case takes the text-directive form too, rather
//     than mirroring RawLeaf.
//
// All three carry the same two directive attributes: "type" is the unknown
// node's own ADF type, spelled directly; "data" is the JSON encoding of its
// Attrs and Text (see encodeRawNodeData), through the shared
// EncodeJSONAttr/DecodeJSONAttr helper rather than a parallel encoder.
//
// RawNode.Marks needs no handling here: at block position they already
// wrap through the generic block-mark path (wrapBlockMarks operates on the
// original adf.Node, independent of what decoded it); at inline position a
// directive-producing decode hook's marks are dropped today, uniformly
// across every inline extension kind (mention, status, mediaInline) — a
// pre-existing limitation this directive does not narrow or widen.
type RawBlock struct {
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*RawBlock) Kind() string { return "rawNode" }

// ChildNodes implements ast.Parent.
func (n *RawBlock) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *RawBlock) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*RawBlock) ContainerDirectiveForm() {}

// DirectiveAttrs implements RawNodeValidator.
func (n *RawBlock) DirectiveAttrs() map[string]string { return n.Attrs }

// RenderMarkdown implements extension.Node.
func (n *RawBlock) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("rawNode", n.Attrs, n.Children)
}

// RawLeaf is ::rawNode{type="…" data=…} ⇄ an unknown ADF node decoded at
// block position with no content (see RawBlock).
type RawLeaf struct {
	Attrs map[string]string
	// Children is unused: the directive is attribute-only.
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*RawLeaf) Kind() string { return "rawNode" }

// ChildNodes implements ast.Parent.
func (n *RawLeaf) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *RawLeaf) SetChildNodes(kids []ast.Node) { n.Children = kids }

// DirectiveAttrs implements RawNodeValidator.
func (n *RawLeaf) DirectiveAttrs() map[string]string { return n.Attrs }

// RenderMarkdown implements extension.Node.
func (n *RawLeaf) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("rawNode", n.Attrs, n.Children)
}

// RawText is :rawNode{type="…" data=…} ⇄ an unknown ADF node decoded at
// inline position, with or without content (see RawBlock).
type RawText struct {
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*RawText) Kind() string { return "rawNode" }

// ChildNodes implements ast.Parent.
func (n *RawText) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *RawText) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*RawText) MarkdownLead() byte { return ':' }

// DirectiveAttrs implements RawNodeValidator.
func (n *RawText) DirectiveAttrs() map[string]string { return n.Attrs }

// RenderMarkdown implements extension.Node.
func (n *RawText) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("rawNode", n.Attrs, n.Children)
}

// RawNodeValidator is implemented by RawBlock, RawLeaf, and RawText: the
// directive attribute payload a hand-edit of the markdown can corrupt (see
// RawNodeCorrupted).
type RawNodeValidator interface {
	ast.Node
	DirectiveAttrs() map[string]string
}

// RawNodeCorrupted reports whether a rawNode directive's attributes cannot
// reconstruct the adf.RawNode it decoded from: a missing "type" attribute,
// or a "data" attribute that does not parse into the shape
// encodeRawNodeData produces. Either is something only a hand-edit of the
// markdown produces — FromMarkdown/ToMarkdown never write one — and the
// design this directive implements calls for failing loudly on it rather
// than guessing: EncodeADF degrades a corrupted rawNode like any other
// malformed directive (drops it, or for RawBlock dissolves into its
// decoded children) because EncodeADF has no diagnostics sink of its own;
// convert's encode dispatch calls RawNodeCorrupted first, alongside
// EncodeADF, so it can still report the corruption instead of letting it
// degrade silently.
func RawNodeCorrupted(n RawNodeValidator) (string, bool) {
	attrs := n.DirectiveAttrs()
	if attrs["type"] == "" {
		return "rawNode directive is missing its required type attribute", true
	}
	if _, _, ok := decodeRawNodeData(attrs["data"]); !ok {
		return "rawNode directive " + strconv.Quote(attrs["type"]) + " has an unparsable data attribute", true
	}
	return "", false
}

// Compile-time contract checks.
var (
	_ extension.Node          = (*RawBlock)(nil)
	_ extension.ContainerForm = (*RawBlock)(nil)
	_ RawNodeValidator        = (*RawBlock)(nil)
	_ extension.Node          = (*RawLeaf)(nil)
	_ RawNodeValidator        = (*RawLeaf)(nil)
	_ extension.Node          = (*RawText)(nil)
	_ extension.InlineLead    = (*RawText)(nil)
	_ RawNodeValidator        = (*RawText)(nil)
)
