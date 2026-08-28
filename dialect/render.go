package dialect

import (
	"github.com/pmarschik/adfast/extension"
)

// This file implements the ast→md path of the dialect kinds: every node
// renders its directive form through the renderer-controlled primitives
// (extension.RenderContext), byte-identical to the generic directive
// rendering it replaced.

// RenderMarkdown implements extension.Node. Panel attrs have no ADF
// equivalent, but they are written back out: they survive the markdown →
// AST → markdown round trip like the generic container form's, so a
// re-render does not delete what the author wrote.
func (n *Panel) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective(n.PanelType, n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node. Expand attrs have no ADF
// equivalent, and render back out like Panel's.
func (n *Expand) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("expand", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Media) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("media", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *JQL) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("jql", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *LinkCard) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("linkCard", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *LinkEmbed) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("linkEmbed", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Colwidths) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("colwidths", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Decisions) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("decisions", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Mention) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("mention", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Status) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("status", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *MediaInline) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("media", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Color) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("color", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Bg) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("bg", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Underline) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("u", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Sub) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("sub", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Sup) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("sup", n.Attrs, n.Children)
}
