package dialect

import (
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
	"github.com/pmarschik/adfast/internal/mediasrc"
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
		Title:   new(title),
		Content: ctx.EncodeBlocks(children),
	}}
}

// EncodeADF implements extension.Node: the inverse of decodeMediaNode —
// a media node wrapped in mediaSingle, or a single-item mediaGroup that
// the converter merges with adjacent group items.
func (n *Media) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	media := mediaFromAttrs(ctx, n.Attrs, ast.PlainText(n.Children))
	if n.Attrs["group"] == "true" {
		return []adf.Node{&adf.MediaGroup{Content: []adf.Node{media}}}
	}
	return []adf.Node{mediaSingleFromAttrs(n.Attrs, media)}
}

// mediaFromAttrs builds the ADF media leaf from a media directive's
// attribute payload and alt label (shared by ::media and :::media).
//
// The store resolution happens here, not at the call sites, because a
// media leaf that carries only a path is not addressable in ADF at all:
// ADF names an attachment by media id, so an encode site that forgot to
// recover it would emit a media node with an empty id. It was spelled
// out at the ::media site only, and :::media did exactly that.
func mediaFromAttrs(ctx extension.EncodeContext, attrs map[string]string, alt string) *adf.Media {
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
		media.Collection = new(v)
	} else if media.Type == "file" {
		// Re-add the empty collection omitted on decode for file media.
		media.Collection = new("")
	}
	if v, ok := attrs["height"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			media.Height = &f
		}
	}
	if v := attrs["occurrenceKey"]; v != "" {
		media.OccurrenceKey = new(v)
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
	if mark := linkMarkFromAttrs(attrs); mark != nil {
		media.Marks = append(media.Marks, mark)
	}
	// A local asset omits its id + intrinsic dimensions (decode drops them);
	// resolve them back from the markdown-relative path via the asset store.
	// mediasrc owns that recovery for every leg of the projection — the
	// md→md formatter performs the same one over its own media shape — so
	// a path-addressed directive cannot be a picture on one leg and opaque
	// on the other.
	path := attrs["path"]
	media.ID = mediasrc.ID(media.ID, path, ctx.AssetID)
	media.Width, media.Height = mediasrc.Dims(media.Width, media.Height, path, ctx.AssetDims)
	return media
}

// linkMarkFromAttrs builds the ADF link mark carried as the href
// attribute on the media directive forms — what the picture links to.
// The mark goes on the MEDIA node, the placement the Jira ADF reference
// documents and the one the markdown [![alt](img)](href) form encodes to,
// so a linked image reaches the same ADF whichever markdown spelling it
// arrived in.
//
// hrefTitle is the link's advisory title, the "Home" of
// [![alt](img)](href "Home"). It is a compound attribute name for the
// same reason borderColor and borderSize are: it belongs to the href, and
// a bare `title` beside `url` would read as the picture's own caption,
// which the block form spells as a mediaSingle caption instead.
//
// No href, no mark, so no title either: a title for a destination that is
// not there has nothing to be advisory about, and the md→md leg drops it
// in the same place so both legs write one document.
func linkMarkFromAttrs(attrs map[string]string) adf.Mark {
	href := attrs["href"]
	if href == "" {
		return nil
	}
	mark := &adf.Link{Href: &href}
	if title := attrs["hrefTitle"]; title != "" {
		mark.Title = &title
	}
	return mark
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
		single.Layout = new(v)
	} else if media.Type == "file" {
		// Re-infer the file-media default layout omitted on decode.
		single.Layout = new("align-start")
	}
	if v, ok := attrs["layoutWidth"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			single.Width = &f
		}
	}
	if v := attrs["widthType"]; v != "" {
		single.WidthType = new(v)
	}
	return single
}

// EncodeADF implements extension.Node, converting the directive back to
// its JQL-datasource blockCard (see decodeDatasource for the shape).
// A directive missing cloudId or datasource has no card shape to become
// (see EncodesAsDatasource), so the QUERY DEGRADES to a paragraph rather
// than vanishing with the card; convert reports it as
// CodeJQLDegraded. An empty query leaves nothing to keep and drops.
func (n *JQL) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	jql := ast.PlainText(n.Children)
	if !n.EncodesAsDatasource() {
		if jql == "" {
			return nil
		}
		return []adf.Node{&adf.Paragraph{Content: ctx.EncodeInlines(n.Children)}}
	}
	ds := map[string]any{
		"id": n.Attrs["datasource"],
		"parameters": map[string]any{
			"cloudId": n.Attrs["cloudId"],
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

// EncodeADF implements extension.Node, converting the directive back to
// its ADF blockCard. A label that resolves to no URL has no card shape
// to become (see ResolveCardURL), so the LABEL DEGRADES to a paragraph
// rather than vanishing with the card; convert reports it as
// CodeSmartLinkDegraded. A label with no text leaves nothing to keep and
// drops.
func (n *LinkCard) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	url, label, ok := ResolveCardURL(n, ctx.SmartLinkURL)
	if !ok {
		return degradedCardLabel(label)
	}
	return []adf.Node{&adf.BlockCard{URL: url}}
}

// EncodeADF implements extension.Node, converting the directive back to
// its ADF embedCard. It degrades exactly like LinkCard.EncodeADF above
// when the label resolves to no URL.
func (n *LinkEmbed) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	url, label, ok := ResolveCardURL(n, ctx.SmartLinkURL)
	if !ok {
		return degradedCardLabel(label)
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

// degradedCardLabel is the shared fallback of the two smart-link
// encoders: the label the author wrote survives as a paragraph, and only
// a label with no text at all drops. label is ResolveCardURL's trimmed
// text, so the paragraph is plain text — a card label is a URL or a
// smart-link key, and spelling it identically here and in the prettier
// formatter's mirror is what keeps the two paths from diverging.
func degradedCardLabel(label string) []adf.Node {
	if label == "" {
		return nil
	}
	return []adf.Node{&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: label}}}}
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
		Text:        new("@" + label),
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
		Text:  new(label),
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
		mi.Collection = new(v)
	}
	if label := strings.TrimSpace(ast.PlainText(n.Children)); label != "" {
		mi.Alt = label
	}
	if mark := linkMarkFromAttrs(n.Attrs); mark != nil {
		mi.Marks = append(mi.Marks, mark)
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
