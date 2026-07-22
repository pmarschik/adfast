package dialect

import (
	"strconv"
	"time"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// The ast→adf path of the extended dialect kinds (see encode.go). An
// empty result drops the node; wrapper kinds dissolve into their
// content when their payload is unusable, matching remark degradation.

// EncodeADF implements extension.Node: the timestamp attribute is
// authoritative; without one, the YYYY-MM-DD label is parsed as a UTC
// day. Neither usable drops the node.
func (n *Date) EncodeADF(_ extension.EncodeContext) []adf.Node {
	ts := n.Attrs["timestamp"]
	if ts == "" {
		t, err := time.Parse(time.DateOnly, ast.PlainText(n.Children))
		if err != nil {
			return nil
		}
		ts = strconv.FormatInt(t.UnixMilli(), 10)
	}
	return []adf.Node{&adf.Date{Timestamp: ts, LocalID: n.Attrs["localId"]}}
}

// EncodeADF implements extension.Node: the label is the placeholder
// text; an empty label drops the node.
func (n *Placeholder) EncodeADF(_ extension.EncodeContext) []adf.Node {
	text := ast.PlainText(n.Children)
	if text == "" {
		return nil
	}
	return []adf.Node{&adf.Placeholder{Text: text, LocalID: n.Attrs["localId"]}}
}

// EncodeADF implements extension.Node: an emoji without a shortName is
// dropped. The text attribute, when present, restores the rendered
// fallback text.
func (n *Emoji) EncodeADF(_ extension.EncodeContext) []adf.Node {
	shortName := n.Attrs["shortName"]
	if shortName == "" {
		return nil
	}
	e := &adf.Emoji{ShortName: shortName, ID: n.Attrs["id"]}
	if text, ok := n.Attrs["text"]; ok {
		e.Text = strPtr(text)
	}
	return []adf.Node{e}
}

// EncodeADF implements extension.Node: the annotation mark layers onto
// the inherited mark context; without an id the content is encoded
// unannotated (the threads it would anchor cannot be addressed).
func (n *Annotation) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	id := n.Attrs["id"]
	if id == "" {
		return ctx.EncodeInlines(n.Children)
	}
	annotationType := n.Attrs["annotationType"]
	if annotationType == "" {
		annotationType = "inlineComment"
	}
	return ctx.EncodeInlinesStyled(extension.InlineStyle{
		Annotation: &extension.Annotation{ID: id, AnnotationType: annotationType},
	}, n.Children)
}

// EncodeADF implements extension.Node: fontSize is a RETIRED mark — no
// Atlassian product supports it (Jira REST rejects it, Confluence strips
// it on save), so adfast never produces one. The directive unwraps to
// its inline content; the size is dropped. The convert layer emits the
// fontsize-dropped diagnostic when it dissolves the node (EncodeADF has
// no diagnostics sink of its own).
func (n *FontSize) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return ctx.EncodeInlines(n.Children)
}

// EncodeADF implements extension.Node: extensionType and extensionKey
// are required by the schema; missing either drops the node.
func (n *InlineExtension) EncodeADF(_ extension.EncodeContext) []adf.Node {
	if n.Attrs["type"] == "" || n.Attrs["key"] == "" {
		return nil
	}
	return []adf.Node{&adf.InlineExtension{
		ExtensionType: n.Attrs["type"],
		ExtensionKey:  n.Attrs["key"],
		Parameters:    parametersFromAttrs(n.Attrs),
		Text:          n.Attrs["text"],
		LocalID:       n.Attrs["localId"],
	}}
}

// EncodeADF implements extension.Node (see InlineExtension).
func (n *Extension) EncodeADF(_ extension.EncodeContext) []adf.Node {
	if n.Attrs["type"] == "" || n.Attrs["key"] == "" {
		return nil
	}
	return []adf.Node{&adf.Extension{
		ExtensionType: n.Attrs["type"],
		ExtensionKey:  n.Attrs["key"],
		Parameters:    parametersFromAttrs(n.Attrs),
		Text:          n.Attrs["text"],
		Layout:        n.Attrs["layout"],
		LocalID:       n.Attrs["localId"],
	}}
}

// parametersFromAttrs decodes the canonical-JSON parameters attribute
// (nil when absent or empty).
func parametersFromAttrs(attrs map[string]string) any {
	v, ok := attrs["parameters"]
	if !ok || v == "" {
		return nil
	}
	return DecodeJSONAttr(v)
}

// EncodeADF implements extension.Node: bodiedExtension for body blocks,
// multiBodiedExtension when every child is a :::frame container (or the
// multi attribute marks a frameless one). Without the required
// extensionType/extensionKey the container dissolves into its content.
func (n *BodiedExtension) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	if n.Attrs["type"] == "" || n.Attrs["key"] == "" {
		return ctx.EncodeBlocks(n.Children)
	}
	_, multi := n.Attrs["multi"]
	if !multi && len(n.Children) > 0 {
		multi = true
		for _, child := range n.Children {
			if _, isFrame := child.(*Frame); !isFrame {
				multi = false
				break
			}
		}
	}
	content := ctx.EncodeBlocks(n.Children)
	if multi {
		return []adf.Node{&adf.MultiBodiedExtension{
			ExtensionType: n.Attrs["type"],
			ExtensionKey:  n.Attrs["key"],
			Parameters:    parametersFromAttrs(n.Attrs),
			Text:          n.Attrs["text"],
			Layout:        n.Attrs["layout"],
			LocalID:       n.Attrs["localId"],
			Content:       content,
		}}
	}
	return []adf.Node{&adf.BodiedExtension{
		ExtensionType: n.Attrs["type"],
		ExtensionKey:  n.Attrs["key"],
		Parameters:    parametersFromAttrs(n.Attrs),
		Text:          n.Attrs["text"],
		Layout:        n.Attrs["layout"],
		LocalID:       n.Attrs["localId"],
		Content:       content,
	}}
}

// EncodeADF implements extension.Node.
func (n *Frame) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return []adf.Node{&adf.ExtensionFrame{Content: ctx.EncodeBlocks(n.Children)}}
}

// EncodeADF implements extension.Node: resourceId is required; without
// one the reference is meaningless and drops.
func (n *SyncBlock) EncodeADF(_ extension.EncodeContext) []adf.Node {
	if n.Attrs["resourceId"] == "" {
		return nil
	}
	return []adf.Node{&adf.SyncBlock{ResourceID: n.Attrs["resourceId"], LocalID: n.Attrs["localId"]}}
}

// EncodeADF implements extension.Node: without a resourceId the
// container dissolves into its content.
func (n *BodiedSyncBlock) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	if n.Attrs["resourceId"] == "" {
		return ctx.EncodeBlocks(n.Children)
	}
	return []adf.Node{&adf.BodiedSyncBlock{
		ResourceID: n.Attrs["resourceId"],
		LocalID:    n.Attrs["localId"],
		Content:    ctx.EncodeBlocks(n.Children),
	}}
}

// EncodeADF implements extension.Node.
func (n *Section) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return []adf.Node{&adf.LayoutSection{
		LocalID:         n.Attrs["localId"],
		ColumnRuleStyle: n.Attrs["columnRuleStyle"],
		Content:         ctx.EncodeBlocks(n.Children),
	}}
}

// EncodeADF implements extension.Node.
func (n *Column) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	col := &adf.LayoutColumn{
		LocalID: n.Attrs["localId"],
		VAlign:  n.Attrs["valign"],
		Content: ctx.EncodeBlocks(n.Children),
	}
	if f, err := strconv.ParseFloat(n.Attrs["width"], 64); err == nil {
		col.Width = &f
	}
	return []adf.Node{col}
}

// EncodeADF implements extension.Node: the media form of the caption
// carrier — a mediaSingle whose second child is the caption built from
// the body blocks (paragraphs joined by hardBreaks).
func (n *MediaCaption) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	children := n.Children
	alt := ""
	if p, ok := labelParagraph(children); ok {
		alt = ast.PlainText(p.Children)
		children = children[1:]
	}
	media := mediaFromAttrs(n.Attrs, alt)
	single := mediaSingleFromAttrs(n.Attrs, media)
	if inlines := captionInlines(ctx, children); len(inlines) > 0 {
		single.Content = append(single.Content, &adf.Caption{Content: inlines})
	}
	return []adf.Node{single}
}

// captionInlines flattens the caption body blocks to the caption's
// inline content, joining blocks with hardBreaks.
func captionInlines(ctx extension.EncodeContext, blocks []ast.Node) []adf.Node {
	var out []adf.Node
	for _, block := range blocks {
		inlines := ctx.EncodeInlines(ast.Children(block))
		if len(inlines) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, &adf.HardBreak{})
		}
		out = append(out, inlines...)
	}
	return out
}

// ---------------------------------------------------------------------------
// Block-mark wrappers
// ---------------------------------------------------------------------------

// encodeMarkedBlocks encodes the wrapper's children and appends the
// block mark to each encoded block (one mark instance per block); a nil
// mark dissolves the wrapper into its content.
func encodeMarkedBlocks(ctx extension.EncodeContext, children []ast.Node, mark func() adf.Mark) []adf.Node {
	blocks := ctx.EncodeBlocks(children)
	if mark == nil {
		return blocks
	}
	for _, b := range blocks {
		adf.AddMarks(b, mark())
	}
	return blocks
}

// EncodeADF implements extension.Node.
func (n *Align) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	if n.Align != "center" && n.Align != "end" {
		return encodeMarkedBlocks(ctx, n.Children, nil)
	}
	return encodeMarkedBlocks(ctx, n.Children, func() adf.Mark {
		return &adf.Alignment{Align: n.Align}
	})
}

// EncodeADF implements extension.Node: an unparsable level dissolves
// the wrapper.
func (n *Indent) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	level, err := strconv.Atoi(n.Level())
	if err != nil || level < 1 {
		return encodeMarkedBlocks(ctx, n.Children, nil)
	}
	return encodeMarkedBlocks(ctx, n.Children, func() adf.Mark {
		return &adf.Indentation{Level: level}
	})
}

// EncodeADF implements extension.Node: a missing mode dissolves the
// wrapper; width rides along when parseable.
func (n *Breakout) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	mode := n.Mode()
	if mode == "" {
		return encodeMarkedBlocks(ctx, n.Children, nil)
	}
	return encodeMarkedBlocks(ctx, n.Children, func() adf.Mark {
		mark := &adf.Breakout{Mode: mode}
		if f, err := strconv.ParseFloat(n.Attrs["width"], 64); err == nil {
			mark.Width = &f
		}
		return mark
	})
}

// EncodeADF implements extension.Node: sources is a comma-separated list
// of source ids; an empty list dissolves the wrapper.
func (n *DataConsumer) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	sources := ParseSources(n.Attrs["sources"])
	if len(sources) == 0 {
		return encodeMarkedBlocks(ctx, n.Children, nil)
	}
	return encodeMarkedBlocks(ctx, n.Children, func() adf.Mark {
		return &adf.DataConsumer{Sources: sources}
	})
}

// EncodeADF implements extension.Node: localId is required; without one
// the wrapper dissolves.
func (n *Fragment) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	if n.Attrs["localId"] == "" {
		return encodeMarkedBlocks(ctx, n.Children, nil)
	}
	return encodeMarkedBlocks(ctx, n.Children, func() adf.Mark {
		return &adf.Fragment{LocalID: n.Attrs["localId"], Name: n.Attrs["name"]}
	})
}
