package dialect

import (
	"strconv"
	"time"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// The adf→ast decode hooks and md→ast parse promotions of the extended
// dialect kinds (see decode.go). The inline annotation mark decodes
// through a DecodeTextMark hook; fontSize is retired (DecodedByCore —
// the core dissolves the legacy mark to bare text); the block-mark
// wrappers stay DecodedByCore: convert's block-mark wrapping constructs
// them, their EncodeADF layers the marks back.

// extendedRegistrations returns the registrations Registrations appends
// after the historical set (dispatch order within this list is not
// significant — no two hooks probe the same ADF type), grouped the same
// way Registrations groups the historical set.
func extendedRegistrations() []extension.Registration {
	regs := extendedInlineRegistrations()
	regs = append(regs, extendedBlockRegistrations()...)
	return append(regs, blockMarkRegistrations()...)
}

// extendedInlineRegistrations returns the extended kinds that live inside
// a paragraph: three inline nodes, the annotation mark, and retired
// fontSize.
func extendedInlineRegistrations() []extension.Registration {
	return []extension.Registration{
		{
			Kind:         "date",
			Texts:        markConstructors("date", func(d *ast.TextDirective) extension.Node { return &Date{Attrs: d.Attrs, Children: d.Children} }),
			DecodeInline: decodeDate,
		},
		{
			Kind:         "placeholder",
			Texts:        markConstructors("placeholder", func(d *ast.TextDirective) extension.Node { return &Placeholder{Attrs: d.Attrs, Children: d.Children} }),
			DecodeInline: decodePlaceholder,
		},
		{
			// Emoji decode stays in convert's inline visitor (text-present
			// emojis render their text, the shortname table restores
			// unicode); the directive is the fallback form.
			Kind:          "emoji",
			Texts:         markConstructors("emoji", func(d *ast.TextDirective) extension.Node { return &Emoji{Attrs: d.Attrs, Children: d.Children} }),
			DecodedByCore: true,
		},
		{
			Kind: "annotation",
			Texts: markConstructors("annotation", func(d *ast.TextDirective) extension.Node {
				return &Annotation{ID: d.Attrs["id"], Attrs: d.Attrs, Children: d.Children}
			}),
			DecodeTextMark: decodeAnnotationMark,
		},
		{
			// fontSize is a RETIRED mark: it still PARSES (the directive
			// survives round trips through markdown) but never round-trips
			// through ADF — the encode drops the mark, and the core decode
			// dissolves a legacy fontSize ADF mark to bare text with a
			// fontsize-dropped diagnostic (no DecodeTextMark hook, hence
			// DecodedByCore). No Atlassian product supports the mark.
			Kind:          "fontSize",
			Texts:         markConstructors("fontSize", func(d *ast.TextDirective) extension.Node { return &FontSize{Attrs: d.Attrs, Children: d.Children} }),
			DecodedByCore: true,
		},
	}
}

// extendedBlockRegistrations returns the extended kinds that decode from
// an ADF BLOCK node. The "extension" kind spans all three directive
// surfaces because ADF spells it inlineExtension, extension and
// bodiedExtension.
func extendedBlockRegistrations() []extension.Registration {
	return []extension.Registration{
		{
			Kind: "extension",
			Texts: map[string]func(*ast.TextDirective) extension.Node{
				"extension": func(d *ast.TextDirective) extension.Node {
					return &InlineExtension{Attrs: d.Attrs, Children: d.Children}
				},
			},
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"extension": promoteExtension,
			},
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"extension": promoteBodiedExtension,
			},
			DecodeBlock:  decodeExtensionBlock,
			DecodeInline: decodeInlineExtension,
		},
		{
			Kind: "extensionFrame",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"frame": promoteFrame,
			},
			DecodeBlock: decodeExtensionFrame,
		},
		{
			Kind: "syncBlock",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"syncBlock": promoteSyncBlock,
			},
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"syncBlock": promoteBodiedSyncBlock,
			},
			DecodeBlock: decodeSyncBlocks,
		},
		{
			Kind: "layoutSection",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"section": promoteSection,
			},
			DecodeBlock: decodeLayoutSection,
		},
		{
			Kind: "layoutColumn",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"column": promoteColumn,
			},
			DecodeBlock: decodeLayoutColumn,
		},
	}
}

// blockMarkRegistrations returns the wrapper kinds that decode from ADF
// block MARKS, not nodes; convert's block-mark wrapping constructs them.
func blockMarkRegistrations() []extension.Registration {
	return []extension.Registration{
		{
			Kind: "align",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"center": promoteAlign,
				"end":    promoteAlign,
			},
			DecodedByCore: true,
		},
		{
			Kind: "indent",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"indent": func(d *ast.ContainerDirective) extension.Node {
					return &Indent{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
				},
			},
			DecodedByCore: true,
		},
		{
			Kind: "breakout",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"breakout": func(d *ast.ContainerDirective) extension.Node {
					return &Breakout{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
				},
			},
			DecodedByCore: true,
		},
		{
			Kind: "dataConsumer",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"dataConsumer": func(d *ast.ContainerDirective) extension.Node {
					return &DataConsumer{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
				},
			},
			DecodedByCore: true,
		},
		{
			Kind: "fragment",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"fragment": func(d *ast.ContainerDirective) extension.Node {
					return &Fragment{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
				},
			},
			DecodedByCore: true,
		},
	}
}

// ---------------------------------------------------------------------------
// Parse promotions (md→ast)
// ---------------------------------------------------------------------------

func promoteMediaCaption(d *ast.ContainerDirective) extension.Node {
	return &MediaCaption{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteExtension(d *ast.LeafDirective) extension.Node {
	return &Extension{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteBodiedExtension(d *ast.ContainerDirective) extension.Node {
	return &BodiedExtension{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteFrame(d *ast.ContainerDirective) extension.Node {
	return &Frame{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteSyncBlock(d *ast.LeafDirective) extension.Node {
	return &SyncBlock{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteBodiedSyncBlock(d *ast.ContainerDirective) extension.Node {
	return &BodiedSyncBlock{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteSection(d *ast.ContainerDirective) extension.Node {
	return &Section{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteColumn(d *ast.ContainerDirective) extension.Node {
	return &Column{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteAlign(d *ast.ContainerDirective) extension.Node {
	return &Align{Align: d.Name, Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

// ---------------------------------------------------------------------------
// Decode hooks (adf→ast)
// ---------------------------------------------------------------------------

// decodeAnnotationMark wraps content in :annotation for an annotation
// mark, defaulting the type to "inlineComment" like the encode side.
func decodeAnnotationMark(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
	m, ok := mark.(*adf.Annotation)
	if !ok {
		return nil, false
	}
	annotationType := m.AnnotationType
	if annotationType == "" {
		annotationType = "inlineComment"
	}
	return &Annotation{
		ID:       m.ID,
		Attrs:    map[string]string{"id": m.ID, "annotationType": annotationType},
		Children: inner,
	}, true
}

// decodeDate converts an ADF date node: the label is the UTC day
// derived from the timestamp (no label when unparsable — the
// authoritative timestamp attribute still rides along).
func decodeDate(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
	date, ok := n.(*adf.Date)
	if !ok {
		return nil, false
	}
	if date.Timestamp == "" {
		return nil, true
	}
	attrs := map[string]string{"timestamp": date.Timestamp}
	if date.LocalID != "" {
		attrs["localId"] = date.LocalID
	}
	var children []ast.Node
	if ms, err := strconv.ParseInt(date.Timestamp, 10, 64); err == nil {
		label := time.UnixMilli(ms).UTC().Format(time.DateOnly)
		children = []ast.Node{&ast.Text{Value: label}}
	}
	return []ast.Node{&Date{Attrs: attrs, Children: children}}, true
}

// decodePlaceholder converts an ADF placeholder node (empty text is
// consumed without output).
func decodePlaceholder(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
	ph, ok := n.(*adf.Placeholder)
	if !ok {
		return nil, false
	}
	if ph.Text == "" {
		return nil, true
	}
	attrs := map[string]string{}
	if ph.LocalID != "" {
		attrs["localId"] = ph.LocalID
	}
	return []ast.Node{&Placeholder{
		Attrs:    attrs,
		Children: []ast.Node{&ast.Text{Value: ph.Text}},
	}}, true
}

// extensionDirectiveAttrs builds the shared extension-family attribute
// payload. The ADF extensionType/extensionKey fields map to the short
// directive attributes type/key; parameters uses the canonical JSON attr
// encoding.
func extensionDirectiveAttrs(extensionType, extensionKey string, parameters any, text, layout, localID string) map[string]string {
	attrs := map[string]string{}
	if extensionType != "" {
		attrs["type"] = extensionType
	}
	if extensionKey != "" {
		attrs["key"] = extensionKey
	}
	if parameters != nil {
		if encoded, ok := EncodeJSONAttr(parameters); ok {
			attrs["parameters"] = encoded
		}
	}
	if text != "" {
		attrs["text"] = text
	}
	if layout != "" {
		attrs["layout"] = layout
	}
	if localID != "" {
		attrs["localId"] = localID
	}
	return attrs
}

// decodeInlineExtension converts an ADF inlineExtension node.
func decodeInlineExtension(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
	ext, ok := n.(*adf.InlineExtension)
	if !ok {
		return nil, false
	}
	attrs := extensionDirectiveAttrs(ext.ExtensionType, ext.ExtensionKey, ext.Parameters, ext.Text, "", ext.LocalID)
	return []ast.Node{&InlineExtension{Attrs: attrs}}, true
}

// decodeExtensionBlock converts the block extension family: bodiless
// extension → ::extension, bodiedExtension → :::extension with body,
// multiBodiedExtension → :::extension with :::frame children (a
// frameless one carries multi="true").
func decodeExtensionBlock(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	switch ext := n.(type) {
	case *adf.Extension:
		attrs := extensionDirectiveAttrs(ext.ExtensionType, ext.ExtensionKey, ext.Parameters, ext.Text, ext.Layout, ext.LocalID)
		return &Extension{Attrs: attrs}, true
	case *adf.BodiedExtension:
		attrs := extensionDirectiveAttrs(ext.ExtensionType, ext.ExtensionKey, ext.Parameters, ext.Text, ext.Layout, ext.LocalID)
		return &BodiedExtension{Attrs: attrs, Children: ctx.DecodeBlocks(ext.Content)}, true
	case *adf.MultiBodiedExtension:
		attrs := extensionDirectiveAttrs(ext.ExtensionType, ext.ExtensionKey, ext.Parameters, ext.Text, ext.Layout, ext.LocalID)
		children := ctx.DecodeBlocks(ext.Content)
		if len(children) == 0 {
			// The bare multi attribute keeps a frameless
			// multiBodiedExtension distinguishable from a bodied one.
			attrs["multi"] = ""
		}
		return &BodiedExtension{Attrs: attrs, Children: children}, true
	}
	return nil, false
}

// decodeExtensionFrame converts an ADF extensionFrame node (reached
// when DecodeBlocks recurses into a multiBodiedExtension's content).
func decodeExtensionFrame(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	frame, ok := n.(*adf.ExtensionFrame)
	if !ok {
		return nil, false
	}
	return &Frame{Children: ctx.DecodeBlocks(frame.Content)}, true
}

// syncBlockAttrs builds the syncBlock attribute payload.
func syncBlockAttrs(resourceID, localID string) map[string]string {
	attrs := map[string]string{}
	if resourceID != "" {
		attrs["resourceId"] = resourceID
	}
	if localID != "" {
		attrs["localId"] = localID
	}
	return attrs
}

// decodeSyncBlocks converts ADF syncBlock (reference leaf) and
// bodiedSyncBlock (source body container) nodes.
func decodeSyncBlocks(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	switch sb := n.(type) {
	case *adf.SyncBlock:
		return &SyncBlock{Attrs: syncBlockAttrs(sb.ResourceID, sb.LocalID)}, true
	case *adf.BodiedSyncBlock:
		return &BodiedSyncBlock{
			Attrs:    syncBlockAttrs(sb.ResourceID, sb.LocalID),
			Children: ctx.DecodeBlocks(sb.Content),
		}, true
	}
	return nil, false
}

// decodeLayoutSection converts an ADF layoutSection node.
func decodeLayoutSection(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	section, ok := n.(*adf.LayoutSection)
	if !ok {
		return nil, false
	}
	attrs := map[string]string{}
	if section.ColumnRuleStyle != "" {
		attrs["columnRuleStyle"] = section.ColumnRuleStyle
	}
	if section.LocalID != "" {
		attrs["localId"] = section.LocalID
	}
	return &Section{Attrs: attrs, Children: ctx.DecodeBlocks(section.Content)}, true
}

// decodeLayoutColumn converts an ADF layoutColumn node (reached when
// DecodeBlocks recurses into a layoutSection's content).
func decodeLayoutColumn(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	column, ok := n.(*adf.LayoutColumn)
	if !ok {
		return nil, false
	}
	attrs := map[string]string{}
	if column.LocalID != "" {
		attrs["localId"] = column.LocalID
	}
	if column.VAlign != "" {
		attrs["valign"] = column.VAlign
	}
	if column.Width != nil {
		attrs["width"] = formatJSNumber(*column.Width)
	}
	return &Column{Attrs: attrs, Children: ctx.DecodeBlocks(column.Content)}, true
}
