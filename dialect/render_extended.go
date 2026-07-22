package dialect

import (
	"github.com/pmarschik/adfast/extension"
)

// The ast→md path of the extended dialect kinds (see render.go).

// RenderMarkdown implements extension.Node.
func (n *Date) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("date", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Placeholder) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("placeholder", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Emoji) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("emoji", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Annotation) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("annotation", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *FontSize) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("fontSize", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *InlineExtension) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective("extension", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Extension) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("extension", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *SyncBlock) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("syncBlock", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *MediaCaption) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("media", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *BodiedExtension) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("extension", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Frame) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("frame", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *BodiedSyncBlock) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("syncBlock", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Section) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("section", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Column) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("column", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node: the alignment value is the
// directive name (:::center / :::end).
func (n *Align) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective(n.Align, n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Indent) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("indent", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Breakout) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("breakout", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *DataConsumer) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("dataConsumer", n.Attrs, n.Children)
}

// RenderMarkdown implements extension.Node.
func (n *Fragment) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective("fragment", n.Attrs, n.Children)
}
