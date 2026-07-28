package dialect

import (
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// This file implements the ast→adf path of the dialect kinds: the
// name-based directive interpretation that used to live inside the
// convert package, moved onto each node's EncodeADF. An empty result
// drops the node, matching the remark reference pipeline.

// ColwidthsPlaceholder is the synthetic ADF node type Colwidths.EncodeADF
// emits (adf.ColwidthsHint); the convert package resolves it structurally
// onto the following table's cells (colwidth attrs) and drops orphans
// with a "colwidths-orphan" diagnostic. It never appears in final
// documents.
const ColwidthsPlaceholder = adf.ColwidthsHintType

// strPtr returns a pointer to v (presence-sensitive ADF attributes are
// pointer fields).
func strPtr(v string) *string { return &v }

// EncodeADF implements extension.Node. Directive attributes have no ADF
// equivalent on panels.
func (n *Panel) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return []adf.Node{&adf.Panel{
		PanelType: n.PanelType,
		Content:   ctx.EncodeBlocks(n.Children),
	}}
}

// EncodeADF implements extension.Node. The leading directive-label
// paragraph, when present, becomes the ADF title.
func (n *Expand) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	children := n.Children
	title := ""
	if p, ok := labelParagraph(children); ok {
		title = ast.PlainText(p.Children)
		children = children[1:]
	}
	return []adf.Node{&adf.Expand{
		Title:   strPtr(title),
		Content: ctx.EncodeBlocks(children),
	}}
}

// EncodeADF implements extension.Node: the inverse of decodeMediaNode —
// a media node wrapped in mediaSingle, or a single-item mediaGroup that
// the converter merges with adjacent group items.
func (n *Media) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	media := mediaFromAttrs(n.Attrs, ast.PlainText(n.Children))
	// A local asset omits its id + intrinsic dimensions (decode drops them);
	// resolve them back from the markdown-relative path via the asset store.
	if media.ID == "" {
		if id, ok := ctx.AssetID(n.Attrs["path"]); ok {
			media.ID = id
		}
	}
	if media.Width == nil || media.Height == nil {
		if w, h, ok := ctx.AssetDims(n.Attrs["path"]); ok {
			wf, hf := float64(w), float64(h)
			media.Width, media.Height = &wf, &hf
		}
	}
	if n.Attrs["group"] == "true" {
		return []adf.Node{&adf.MediaGroup{Content: []adf.Node{media}}}
	}
	return []adf.Node{mediaSingleFromAttrs(n.Attrs, media)}
}

// mediaFromAttrs builds the ADF media leaf from a media directive's
// attribute payload and alt label (shared by ::media and :::media).
func mediaFromAttrs(attrs map[string]string, alt string) *adf.Media {
	media := &adf.Media{Type: "file"}
	if t := attrs["type"]; t != "" {
		media.Type = t
	}
	if id := attrs["id"]; id != "" {
		media.ID = id
	}
	if alt != "" {
		media.Alt = alt
	}
	if v, ok := attrs["collection"]; ok {
		media.Collection = strPtr(v)
	} else if media.Type == "file" {
		// Re-add the empty collection omitted on decode for file media.
		media.Collection = strPtr("")
	}
	if v, ok := attrs["height"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			media.Height = &f
		}
	}
	if v := attrs["occurrenceKey"]; v != "" {
		media.OccurrenceKey = strPtr(v)
	}
	if v := attrs["url"]; v != "" {
		media.URL = v
	}
	if v, ok := attrs["width"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			media.Width = &f
		}
	}
	if mark := borderMarkFromAttrs(attrs); mark != nil {
		media.Marks = append(media.Marks, mark)
	}
	return media
}

// borderMarkFromAttrs builds the ADF border mark carried as
// borderColor/borderSize attributes on the media directive forms.
func borderMarkFromAttrs(attrs map[string]string) adf.Mark {
	color, hasColor := attrs["borderColor"]
	sizeStr, hasSize := attrs["borderSize"]
	if !hasColor && !hasSize {
		return nil
	}
	border := &adf.Border{Color: color}
	if size, err := strconv.Atoi(sizeStr); err == nil {
		border.Size = size
	}
	return border
}

// mediaSingleFromAttrs wraps a media leaf in its mediaSingle per the
// directive's layout attributes.
func mediaSingleFromAttrs(attrs map[string]string, media *adf.Media) *adf.MediaSingle {
	single := &adf.MediaSingle{Content: []adf.Node{media}}
	if v := attrs["layout"]; v != "" {
		single.Layout = strPtr(v)
	} else if media.Type == "file" {
		// Re-infer the file-media default layout omitted on decode.
		single.Layout = strPtr("align-start")
	}
	if v, ok := attrs["layoutWidth"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			single.Width = &f
		}
	}
	if v := attrs["widthType"]; v != "" {
		single.WidthType = strPtr(v)
	}
	return single
}

// EncodeADF implements extension.Node, converting the directive back to
// its JQL-datasource blockCard (see decodeDatasource for the shape).
func (n *JQL) EncodeADF(_ extension.EncodeContext) []adf.Node {
	jql := ast.PlainText(n.Children)
	id := n.Attrs["datasource"]
	cloudID := n.Attrs["cloudId"]
	if jql == "" || id == "" || cloudID == "" {
		return nil
	}
	ds := map[string]any{
		"id": id,
		"parameters": map[string]any{
			"cloudId": cloudID,
			"jql":     jql,
		},
	}
	if columns, ok := n.Attrs["columns"]; ok && columns != "" {
		cols := []any{}
		for key := range strings.SplitSeq(columns, ",") {
			cols = append(cols, map[string]any{"key": key})
		}
		ds["views"] = []any{map[string]any{
			"type":       "table",
			"properties": map[string]any{"columns": cols},
		}}
	}
	return []adf.Node{&adf.BlockCard{URL: n.Attrs["url"], Datasource: ds}}
}

// EncodeADF implements extension.Node.
func (n *LinkCard) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	url := ctx.SmartLinkURL(ast.PlainText(n.Children))
	if url == "" {
		return nil
	}
	return []adf.Node{&adf.BlockCard{URL: url}}
}

// EncodeADF implements extension.Node.
func (n *LinkEmbed) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	url := ctx.SmartLinkURL(ast.PlainText(n.Children))
	if url == "" {
		return nil
	}
	layout := n.Attrs["layout"]
	if layout == "" {
		layout = "center"
	}
	card := &adf.EmbedCard{URL: url, Layout: layout}
	if widthStr, ok := n.Attrs["width"]; ok {
		if width, err := strconv.ParseFloat(widthStr, 64); err == nil {
			card.Width = &width
		}
	}
	return []adf.Node{card}
}

// EncodeADF implements extension.Node, emitting the ColwidthsHint
// placeholder the convert package attaches to the following table; a
// label without positive widths drops the node.
func (n *Colwidths) EncodeADF(_ extension.EncodeContext) []adf.Node {
	var widths []float64
	for part := range strings.SplitSeq(ast.PlainText(n.Children), ",") {
		if f, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil && f > 0 {
			widths = append(widths, f)
		}
	}
	if len(widths) == 0 {
		return nil
	}
	return []adf.Node{&adf.ColwidthsHint{Widths: widths}}
}

// EncodeADF implements extension.Node. A ::decisions directive has no
// standalone ADF form: the convert package consumes it structurally
// (turning the FOLLOWING plain bullet list into a decisionList) before
// encoding reaches the node, and drops orphans with a "decisions-orphan"
// diagnostic. A node that still reaches encoding (a non-sibling
// position) drops like an orphan.
func (*Decisions) EncodeADF(_ extension.EncodeContext) []adf.Node {
	return nil
}

// EncodeADF implements extension.Node. The label is the bare display
// name; the ADF mention text carries the conventional "@" prefix (the
// directive itself is the @ in the markdown form).
func (n *Mention) EncodeADF(_ extension.EncodeContext) []adf.Node {
	label := strings.TrimSpace(ast.PlainText(n.Children))
	if label == "" {
		return nil
	}
	return []adf.Node{&adf.Mention{
		ID:          n.Attrs["id"],
		Text:        strPtr("@" + label),
		AccessLevel: n.Attrs["accessLevel"],
	}}
}

// EncodeADF implements extension.Node.
func (n *Status) EncodeADF(_ extension.EncodeContext) []adf.Node {
	label := strings.TrimSpace(ast.PlainText(n.Children))
	if label == "" {
		return nil
	}
	color := n.Attrs["color"]
	if color == "" {
		color = "neutral"
	}
	return []adf.Node{&adf.Status{
		Text:  strPtr(label),
		Color: color,
		Style: n.Attrs["style"],
	}}
}

// EncodeADF implements extension.Node.
func (n *MediaInline) EncodeADF(_ extension.EncodeContext) []adf.Node {
	mi := &adf.MediaInline{Type: "file"}
	if t := n.Attrs["type"]; t != "" {
		mi.Type = t
	}
	if id := n.Attrs["id"]; id != "" {
		mi.ID = id
	}
	if v, ok := n.Attrs["collection"]; ok {
		mi.Collection = strPtr(v)
	}
	if label := strings.TrimSpace(ast.PlainText(n.Children)); label != "" {
		mi.Alt = label
	}
	return []adf.Node{mi}
}

// EncodeADF implements extension.Node: the textColor mark overwrites any
// inherited one (even with an empty value, which clears it).
func (n *Color) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	color := n.Attrs["color"]
	return ctx.EncodeInlinesStyled(extension.InlineStyle{TextColor: &color}, n.Children)
}

// EncodeADF implements extension.Node: the backgroundColor mark
// overwrites any inherited one.
func (n *Bg) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	color := n.Attrs["color"]
	return ctx.EncodeInlinesStyled(extension.InlineStyle{BackgroundColor: &color}, n.Children)
}

// EncodeADF implements extension.Node.
func (n *Underline) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return ctx.EncodeInlinesStyled(extension.InlineStyle{Underline: true}, n.Children)
}

// EncodeADF implements extension.Node.
func (n *Sub) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	sub := "sub"
	return ctx.EncodeInlinesStyled(extension.InlineStyle{SubSup: &sub}, n.Children)
}

// EncodeADF implements extension.Node.
func (n *Sup) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	sup := "sup"
	return ctx.EncodeInlinesStyled(extension.InlineStyle{SubSup: &sup}, n.Children)
}
