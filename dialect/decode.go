package dialect

import (
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// This file implements the adf→ast path of the dialect kinds (the decode
// hooks recognizing the ADF shapes each kind owns) and the md→ast parse
// promotions, bundled per kind into Registrations.

// Registrations returns the default dialect registration set. The
// markdown parser and the convert decoder wire it automatically; the
// slice ORDER is the decode dispatch order and is significant — the JQL
// datasource hook must probe blockCards before the LinkCard fallback.
func Registrations() []extension.Registration {
	regs := []extension.Registration{
		{
			Kind:        "panel",
			Containers:  panelConstructors(),
			DecodeBlock: decodePanel,
		},
		{
			Kind: "expand",
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"expand": promoteExpand,
			},
			DecodeBlock: decodeExpand,
		},
		{
			Kind: "media",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"media": promoteMedia,
			},
			Containers: map[string]func(*ast.ContainerDirective) extension.Node{
				"media": promoteMediaCaption,
			},
			DecodeBlock:     decodeMediaBlock,
			DecodeBlockList: decodeMediaGroup,
		},
		{
			Kind: "jql",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"jql": promoteJQL,
			},
			DecodeBlock: decodeDatasource,
		},
		{
			Kind: "linkCard",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"linkCard": promoteLinkCard,
			},
			DecodeBlock: decodeBlockCard,
		},
		{
			Kind: "linkEmbed",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"linkEmbed": promoteLinkEmbed,
			},
			DecodeBlock: decodeEmbedCard,
		},
		{
			// The cross-sibling application is structural (see the
			// package comment): convert emits Colwidths from a table's
			// cell attrs on decode and resolves the ColwidthsHint
			// placeholder on encode.
			Kind: "colwidths",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"colwidths": promoteColwidths,
			},
			DecodedByCore: true,
		},
		{
			// Like colwidths, the cross-sibling application is structural
			// (see the package comment): convert turns the FOLLOWING plain
			// bullet list into an ADF decisionList on encode and emits
			// ::decisions before the decoded list on decode.
			Kind: "decisions",
			Leaves: map[string]func(*ast.LeafDirective) extension.Node{
				"decisions": promoteDecisions,
			},
			DecodedByCore: true,
		},
		{
			Kind: "mention",
			Texts: map[string]func(*ast.TextDirective) extension.Node{
				"mention": promoteMention,
			},
			DecodeInline: decodeMention,
		},
		{
			Kind: "status",
			Texts: map[string]func(*ast.TextDirective) extension.Node{
				"status": promoteStatus,
			},
			DecodeInline: decodeStatus,
		},
		{
			Kind: "mediaInline",
			Texts: map[string]func(*ast.TextDirective) extension.Node{
				"media": promoteMediaInline,
			},
			DecodeInline: decodeMediaInline,
		},
		// The mark kinds decode from ADF text MARKS, not nodes: convert's
		// mark machinery owns which marks project and their canonical
		// nesting order, and dispatches each mark to the DecodeTextMark
		// hooks below for node construction (see the package comment).
		{
			Kind: "color",
			Texts: markConstructors("color", func(d *ast.TextDirective) extension.Node {
				return &Color{Color: d.Attrs["color"], Attrs: d.Attrs, Children: d.Children}
			}),
			DecodeTextMark: decodeTextColorMark,
		},
		{
			Kind: "bg",
			Texts: markConstructors("bg", func(d *ast.TextDirective) extension.Node {
				return &Bg{Color: d.Attrs["color"], Attrs: d.Attrs, Children: d.Children}
			}),
			DecodeTextMark: decodeBgMark,
		},
		{
			Kind:           "underline",
			Texts:          markConstructors("u", func(d *ast.TextDirective) extension.Node { return &Underline{Attrs: d.Attrs, Children: d.Children} }),
			DecodeTextMark: decodeUnderlineMark,
		},
		{
			Kind:           "sub",
			Texts:          markConstructors("sub", func(d *ast.TextDirective) extension.Node { return &Sub{Attrs: d.Attrs, Children: d.Children} }),
			DecodeTextMark: decodeSubMark,
		},
		{
			Kind:           "sup",
			Texts:          markConstructors("sup", func(d *ast.TextDirective) extension.Node { return &Sup{Attrs: d.Attrs, Children: d.Children} }),
			DecodeTextMark: decodeSupMark,
		},
	}
	return append(regs, extendedRegistrations()...)
}

// ---------------------------------------------------------------------------
// Text-mark decode hooks (adf mark → ast wrapper node)
// ---------------------------------------------------------------------------

// decodeTextColorMark wraps content in :color for a textColor mark.
func decodeTextColorMark(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
	m, ok := mark.(*adf.TextColor)
	if !ok {
		return nil, false
	}
	return &Color{Color: m.Color, Attrs: map[string]string{"color": m.Color}, Children: inner}, true
}

// decodeBgMark wraps content in :bg for a backgroundColor mark.
func decodeBgMark(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
	m, ok := mark.(*adf.BackgroundColor)
	if !ok {
		return nil, false
	}
	return &Bg{Color: m.Color, Attrs: map[string]string{"color": m.Color}, Children: inner}, true
}

// decodeUnderlineMark wraps content in :u for an underline mark.
func decodeUnderlineMark(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
	if _, ok := mark.(*adf.Underline); !ok {
		return nil, false
	}
	return &Underline{Children: inner}, true
}

// decodeSubMark wraps content in :sub for a subsup mark whose type is
// not "sup" (unknown types normalize to sub, the historical default).
// The sub registration precedes sup, so this hook must decline "sup".
func decodeSubMark(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
	m, ok := mark.(*adf.SubSup)
	if !ok || m.Type == "sup" {
		return nil, false
	}
	return &Sub{Children: inner}, true
}

// decodeSupMark wraps content in :sup for a subsup(sup) mark.
func decodeSupMark(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
	m, ok := mark.(*adf.SubSup)
	if !ok || m.Type != "sup" {
		return nil, false
	}
	return &Sup{Children: inner}, true
}

// markConstructors builds the one-name text-constructor map of a mark
// kind.
func markConstructors(name string, ctor func(*ast.TextDirective) extension.Node) map[string]func(*ast.TextDirective) extension.Node {
	return map[string]func(*ast.TextDirective) extension.Node{name: ctor}
}

// ---------------------------------------------------------------------------
// Parse promotions (md→ast)
// ---------------------------------------------------------------------------

// panelConstructors maps the five panel names to the Panel promotion.
func panelConstructors() map[string]func(*ast.ContainerDirective) extension.Node {
	ctors := map[string]func(*ast.ContainerDirective) extension.Node{}
	for _, panel := range []string{"info", "note", "warning", "success", "error"} {
		ctors[panel] = func(d *ast.ContainerDirective) extension.Node {
			return &Panel{PanelType: d.Name, Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
		}
	}
	return ctors
}

func promoteExpand(d *ast.ContainerDirective) extension.Node {
	return &Expand{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteMedia(d *ast.LeafDirective) extension.Node {
	m := newMedia(d.Attrs, d.Children)
	m.BlockSpacing = d.BlockSpacing
	return m
}

func promoteJQL(d *ast.LeafDirective) extension.Node {
	j := newJQL(d.Attrs, d.Children)
	j.BlockSpacing = d.BlockSpacing
	return j
}

func promoteLinkCard(d *ast.LeafDirective) extension.Node {
	return &LinkCard{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteLinkEmbed(d *ast.LeafDirective) extension.Node {
	e := newLinkEmbed(d.Attrs, d.Children)
	e.BlockSpacing = d.BlockSpacing
	return e
}

func promoteColwidths(d *ast.LeafDirective) extension.Node {
	return &Colwidths{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteDecisions(d *ast.LeafDirective) extension.Node {
	return &Decisions{Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
}

func promoteMention(d *ast.TextDirective) extension.Node {
	return &Mention{AccountID: d.Attrs["id"], Attrs: d.Attrs, Children: stripMentionAt(d.Children)}
}

// stripMentionAt removes the leading "@" of a mention label (the legacy
// :mention[@Name] form): the directive itself is the @, so the label
// carries the bare display name.
func stripMentionAt(children []ast.Node) []ast.Node {
	if len(children) == 0 {
		return children
	}
	first, ok := children[0].(*ast.Text)
	if !ok || !strings.HasPrefix(first.Value, "@") {
		return children
	}
	rest := strings.TrimPrefix(first.Value, "@")
	if rest == "" {
		return children[1:]
	}
	out := append([]ast.Node{&ast.Text{Value: rest}}, children[1:]...)
	return out
}

func promoteStatus(d *ast.TextDirective) extension.Node {
	return &Status{Color: d.Attrs["color"], Attrs: d.Attrs, Children: d.Children}
}

func promoteMediaInline(d *ast.TextDirective) extension.Node {
	return &MediaInline{MediaType: d.Attrs["type"], ID: d.Attrs["id"], Collection: d.Attrs["collection"], Attrs: d.Attrs, Children: d.Children}
}

// ---------------------------------------------------------------------------
// Decode hooks (adf→ast)
// ---------------------------------------------------------------------------

func decodePanel(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	panel, ok := n.(*adf.Panel)
	if !ok {
		return nil, false
	}
	return &Panel{
		PanelType: panelTypeToDirective(panel.PanelType),
		Children:  ctx.DecodeBlocks(panel.Content),
	}, true
}

// panelTypeToDirective maps an ADF panelType to its container-directive
// name (unknown types degrade to info).
func panelTypeToDirective(panelType string) string {
	switch panelType {
	case "info", "note", "warning", "success", "error":
		return panelType
	default:
		return "info"
	}
}

// decodeExpand converts an ADF expand/nestedExpand to :::expand, with the
// title as a directive label paragraph.
func decodeExpand(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	var title string
	var content []adf.Node
	switch e := n.(type) {
	case *adf.Expand:
		title = strDeref(e.Title)
		content = e.Content
	case *adf.NestedExpand:
		title = strDeref(e.Title)
		content = e.Content
	default:
		return nil, false
	}
	children := ctx.DecodeBlocks(content)
	if strings.TrimSpace(title) != "" {
		label := &ast.Paragraph{
			DirectiveLabel: true,
			Children:       []ast.Node{&ast.Text{Value: title}},
		}
		children = append([]ast.Node{label}, children...)
	}
	return &Expand{Children: children}, true
}

// strDeref is the "" default over presence-sensitive string attributes.
func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// decodeMediaBlock converts a mediaSingle wrapper's media child (or a
// bare media node) to a plain image when expressible, otherwise a
// ::media node; a mediaSingle carrying a caption child takes the
// caption-aware forms (image title or the :::media container).
func decodeMediaBlock(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	switch w := n.(type) {
	case *adf.MediaSingle:
		var media *adf.Media
		var caption *adf.Caption
		for _, child := range w.Content {
			switch c := child.(type) {
			case *adf.Media:
				if media == nil {
					media = c
				}
			case *adf.Caption:
				if caption == nil {
					caption = c
				}
			}
		}
		if media == nil {
			return nil, true
		}
		if caption != nil && len(caption.Content) > 0 {
			return decodeCaptionedMedia(media, w, caption, ctx), true
		}
		return decodeMediaNode(media, w, ctx), true
	case *adf.Media:
		return decodeMediaNode(w, nil, ctx), true
	}
	return nil, false
}

// decodeCaptionedMedia converts a mediaSingle with a caption child: a
// plain-text caption on image-expressible media becomes the image
// title (![alt](path "caption")); anything richer becomes the
// :::media container with the caption inlines as its body paragraph.
func decodeCaptionedMedia(media *adf.Media, single *adf.MediaSingle, caption *adf.Caption, ctx extension.DecodeContext) ast.Node {
	inlines := ctx.DecodeInlines(caption.Content)
	if title, ok := plainCaptionText(inlines); ok {
		if img := mediaAsImage(media, single); img != nil {
			setImageTitle(img, title)
			return img
		}
		if img := fileMediaAsImage(media, single, ctx); img != nil {
			setImageTitle(img, title)
			return img
		}
	}
	leaf := mediaLeafNode(media, single, false, ctx)
	var children []ast.Node
	if len(leaf.Children) > 0 {
		children = append(children, &ast.Paragraph{
			DirectiveLabel: true,
			Children:       leaf.Children,
		})
	}
	if len(inlines) > 0 {
		children = append(children, &ast.Paragraph{Children: inlines})
	}
	return &MediaCaption{Attrs: leaf.Attrs, Children: children}
}

// plainCaptionText reports the caption's text when it is a single
// unformatted text run the quoted image-title form can carry verbatim:
// no newlines, and no '"' or '\' (the title renderer does not escape
// them). Anything else keeps the :::media container form.
func plainCaptionText(inlines []ast.Node) (string, bool) {
	if len(inlines) != 1 {
		return "", false
	}
	text, ok := inlines[0].(*ast.Text)
	if !ok || text.Value == "" || strings.ContainsAny(text.Value, "\n\r\"\\") {
		return "", false
	}
	return text.Value, true
}

// setImageTitle stores the caption text as the title of the image
// paragraph decodeMediaNode's image forms produce.
func setImageTitle(n ast.Node, title string) {
	p, ok := n.(*ast.Paragraph)
	if !ok {
		return
	}
	for _, child := range p.Children {
		if img, ok := child.(*ast.Image); ok {
			img.Title = title
			return
		}
	}
}

// decodeMediaGroup flattens a media group to one ::media node per
// attachment, tagged group so the ADF conversion can reassemble the
// group.
func decodeMediaGroup(n adf.Node, ctx extension.DecodeContext) ([]ast.Node, bool) {
	group, ok := n.(*adf.MediaGroup)
	if !ok {
		return nil, false
	}
	var out []ast.Node
	for _, child := range group.Content {
		if media, ok := child.(*adf.Media); ok {
			if m := mediaLeafNode(media, nil, true, ctx); m != nil {
				out = append(out, m)
			}
		}
	}
	return out, true
}

// decodeMediaNode converts one media node (with its optional mediaSingle
// wrapper) to a plain image when expressible, otherwise a ::media node.
func decodeMediaNode(media *adf.Media, single *adf.MediaSingle, ctx extension.DecodeContext) ast.Node {
	if img := mediaAsImage(media, single); img != nil {
		return img
	}
	if img := fileMediaAsImage(media, single, ctx); img != nil {
		return img
	}
	return mediaLeafNode(media, single, false, ctx)
}

// singleBlocksImage reports whether a mediaSingle wrapper carries
// properties the plain-image markdown form cannot hold: any display
// width or widthType, or a layout other than the given default.
func singleBlocksImage(single *adf.MediaSingle, defaultLayout string) bool {
	if single == nil {
		return false
	}
	if single.Width != nil || adf.HasExtra(single, "width") {
		return true
	}
	if single.WidthType != nil || adf.HasExtra(single, "widthType") {
		return true
	}
	if single.Layout != nil && *single.Layout != defaultLayout {
		return true
	}
	// A non-string layout value (kept in Extra) never equals the default.
	return adf.HasExtra(single, "layout")
}

// fileMediaAsImage renders a file-type media node as a plain
// ![alt](assets/name) image when the local asset can carry every ADF
// property: the asset store maps the path back to the media id on
// encode, the intrinsic width/height exactly match the local file (the
// encode re-derives them), the collection is empty, the layout is the
// align-start attachment default, and there is no display width or
// occurrenceKey. Anything richer keeps ::media.
func fileMediaAsImage(media *adf.Media, single *adf.MediaSingle, ctx extension.DecodeContext) ast.Node {
	if media.Type != "file" {
		return nil
	}
	asset, ok := ctx.Asset(media.ID)
	if media.ID == "" || !ok || !asset.HasDim {
		return nil
	}
	if media.Width == nil || media.Height == nil ||
		float64(asset.Width) != *media.Width || float64(asset.Height) != *media.Height {
		return nil
	}
	if media.Collection != nil && *media.Collection != "" {
		return nil
	}
	if media.OccurrenceKey != nil || adf.HasExtra(media, "occurrenceKey") {
		return nil
	}
	if adf.HasMark(media.Marks, "border") {
		return nil
	}
	if singleBlocksImage(single, "align-start") {
		return nil
	}
	img := &ast.Image{URL: asset.Path}
	if media.Alt != "" {
		img.Children = []ast.Node{&ast.Text{Value: media.Alt}}
	}
	return &ast.Paragraph{Children: []ast.Node{img}}
}

// mediaAsImage renders an external media node as a plain markdown image
// when every ADF property is expressible by ![alt](url): external type
// with an absolute http(s) URL (the shape the encode side maps back to
// external media — anything else would drop on re-encode), no
// dimensions/occurrenceKey/collection, and at most the default
// layout="center" wrapper. Anything richer falls back to ::media.
func mediaAsImage(media *adf.Media, single *adf.MediaSingle) ast.Node {
	if media.Type != "external" {
		return nil
	}
	if !strings.HasPrefix(media.URL, "http://") && !strings.HasPrefix(media.URL, "https://") {
		return nil
	}
	if media.Width != nil || adf.HasExtra(media, "width") {
		return nil
	}
	if media.Height != nil || adf.HasExtra(media, "height") {
		return nil
	}
	if media.OccurrenceKey != nil || adf.HasExtra(media, "occurrenceKey") {
		return nil
	}
	if media.Collection != nil && *media.Collection != "" {
		return nil
	}
	if adf.HasMark(media.Marks, "border") {
		return nil
	}
	if singleBlocksImage(single, "center") {
		return nil
	}
	img := &ast.Image{URL: media.URL}
	if media.Alt != "" {
		img.Children = []ast.Node{&ast.Text{Value: media.Alt}}
	}
	return &ast.Paragraph{Children: []ast.Node{img}}
}

// mediaLeafNode serializes a media node (with its optional mediaSingle
// wrapper or group membership) as a ::media node: the alt text is the
// label, every other ADF attribute rides as a directive attribute (the
// inverse of Media.EncodeADF).
func mediaLeafNode(media *adf.Media, single *adf.MediaSingle, group bool, ctx extension.DecodeContext) *Media {
	attrs := map[string]string{}
	if border, ok := adf.FindMark[*adf.Border](media.Marks); ok {
		if border.Color != "" {
			attrs["borderColor"] = border.Color
		}
		if border.Size != 0 {
			attrs["borderSize"] = strconv.Itoa(border.Size)
		}
	}
	if media.Collection != nil {
		attrs["collection"] = *media.Collection
	}
	if group {
		attrs["group"] = "true"
	}
	if media.Height != nil {
		attrs["height"] = formatJSNumber(*media.Height)
	}
	if media.ID != "" {
		attrs["id"] = media.ID
	}
	if single != nil {
		if layout := strDeref(single.Layout); layout != "" {
			attrs["layout"] = layout
		}
		if single.Width != nil {
			attrs["layoutWidth"] = formatJSNumber(*single.Width)
		}
	}
	if v := strDeref(media.OccurrenceKey); v != "" {
		attrs["occurrenceKey"] = v
	}
	if a, ok := ctx.Asset(media.ID); ok {
		attrs["path"] = a.Path
	}
	if media.Type != "" {
		attrs["type"] = media.Type
	}
	if media.URL != "" {
		attrs["url"] = media.URL
	}
	if media.Width != nil {
		attrs["width"] = formatJSNumber(*media.Width)
	}
	if single != nil {
		if v := strDeref(single.WidthType); v != "" {
			attrs["widthType"] = v
		}
	}
	var children []ast.Node
	if media.Alt != "" {
		children = []ast.Node{&ast.Text{Value: media.Alt}}
	}
	return newMedia(attrs, children)
}

// decodeDatasource converts a JQL-datasource blockCard to a
// ::jql[<query>] node when its shape is fully expressible: a jira/jql
// datasource with cloudId+jql parameters and at most one table view with
// plain column keys (the documented ADF shape — implemented from the
// Atlassian schema; verify against a real datasource card, see the azek
// bead). Richer shapes fall back to ::linkCard (decodeBlockCard runs
// after this hook).
func decodeDatasource(n adf.Node, _ extension.DecodeContext) (ast.Node, bool) {
	card, ok := n.(*adf.BlockCard)
	if !ok {
		return nil, false
	}
	ds := card.Datasource
	if ds == nil {
		return nil, false
	}
	id := adf.StrAttr(ds, "id")
	params, ok := ds["parameters"].(map[string]any)
	if !ok || id == "" {
		return nil, false
	}
	jql := adf.StrAttr(params, "jql")
	cloudID := adf.StrAttr(params, "cloudId")
	if jql == "" || cloudID == "" || len(params) != 2 {
		return nil, false
	}
	attrs := map[string]string{"cloudId": cloudID, "datasource": id}
	if card.URL != "" {
		attrs["url"] = card.URL
	}
	if views, ok := ds["views"].([]any); ok {
		columns, viewOK := datasourceTableColumns(views)
		if !viewOK {
			return nil, false
		}
		if columns != "" {
			attrs["columns"] = columns
		}
	}
	// The historical attribute-count guards, over the typed shape: the
	// card may carry only the datasource plus an optional non-empty url
	// (anything else lands in Extra), and the datasource only
	// id/parameters plus optional views.
	if len(card.Extra) > 0 || len(ds) > 2+boolToInt(ds["views"] != nil) {
		return nil, false
	}
	return newJQL(attrs, []ast.Node{&ast.Text{Value: jql}}), true
}

// datasourceTableColumns extracts the comma-joined column keys of a single
// table view; ok is false for any richer view configuration.
func datasourceTableColumns(views []any) (string, bool) {
	if len(views) == 0 {
		return "", true
	}
	if len(views) != 1 {
		return "", false
	}
	view, ok := views[0].(map[string]any)
	if !ok || adf.StrAttr(view, "type") != "table" || len(view) > 2 {
		return "", false
	}
	props, hasProps := view["properties"].(map[string]any)
	if !hasProps {
		return "", len(view) == 1
	}
	cols, ok := props["columns"].([]any)
	if !ok || len(props) != 1 {
		return "", false
	}
	var keys []string
	for _, c := range cols {
		col, ok := c.(map[string]any)
		if !ok || len(col) != 1 {
			return "", false
		}
		key := adf.StrAttr(col, "key")
		if key == "" || strings.ContainsAny(key, ",\"\n") {
			return "", false
		}
		keys = append(keys, key)
	}
	return strings.Join(keys, ","), true
}

// boolToInt is 1 for true, 0 for false.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// decodeBlockCard converts an ADF blockCard to a ::linkCard node (a
// URL-less card is consumed without output).
func decodeBlockCard(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	card, ok := n.(*adf.BlockCard)
	if !ok {
		return nil, false
	}
	if card.URL == "" {
		return nil, true
	}
	return &LinkCard{
		Children: []ast.Node{&ast.Text{Value: ctx.SmartLinkLabel(card.URL)}},
	}, true
}

// decodeEmbedCard converts an ADF embedCard to a ::linkEmbed node
// carrying layout/width attributes.
func decodeEmbedCard(n adf.Node, ctx extension.DecodeContext) (ast.Node, bool) {
	card, ok := n.(*adf.EmbedCard)
	if !ok {
		return nil, false
	}
	if card.URL == "" {
		return nil, true
	}
	attrs := map[string]string{}
	if card.Layout != "" {
		attrs["layout"] = card.Layout
	}
	if card.Width != nil {
		attrs["width"] = formatJSNumber(*card.Width)
	}
	return newLinkEmbed(attrs, []ast.Node{&ast.Text{Value: ctx.SmartLinkLabel(card.URL)}}), true
}

// decodeMention converts an ADF mention node.
func decodeMention(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
	mention, ok := n.(*adf.Mention)
	if !ok {
		return nil, false
	}
	if mention.Text != nil {
		attrs := map[string]string{}
		if mention.ID != "" {
			attrs["id"] = mention.ID
		}
		if mention.AccessLevel != "" {
			attrs["accessLevel"] = mention.AccessLevel
		}
		// The directive is the @: the label carries the bare display name
		// (ADF mention text conventionally leads with "@"; encode restores
		// it).
		label := strings.TrimPrefix(*mention.Text, "@")
		var children []ast.Node
		if label != "" {
			children = []ast.Node{&ast.Text{Value: label}}
		}
		return []ast.Node{&Mention{
			AccountID: attrs["id"],
			Attrs:     attrs,
			Children:  children,
		}}, true
	}
	return nil, true
}

// decodeStatus converts an ADF status node.
func decodeStatus(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
	status, ok := n.(*adf.Status)
	if !ok {
		return nil, false
	}
	if status.Text != nil {
		attrs := map[string]string{}
		if status.Color != "" {
			attrs["color"] = status.Color
		}
		if status.Style != "" {
			attrs["style"] = status.Style
		}
		return []ast.Node{&Status{
			Color:    attrs["color"],
			Attrs:    attrs,
			Children: []ast.Node{&ast.Text{Value: *status.Text}},
		}}, true
	}
	return nil, true
}

// decodeMediaInline converts an ADF mediaInline node.
func decodeMediaInline(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
	mi, ok := n.(*adf.MediaInline)
	if !ok {
		return nil, false
	}
	attrs := map[string]string{}
	if mi.Collection != nil {
		attrs["collection"] = *mi.Collection
	}
	if mi.ID != "" {
		attrs["id"] = mi.ID
	}
	if mi.Type != "" {
		attrs["type"] = mi.Type
	}
	var children []ast.Node
	if mi.Alt != "" {
		children = []ast.Node{&ast.Text{Value: mi.Alt}}
	}
	return []ast.Node{&MediaInline{
		MediaType:  attrs["type"],
		ID:         attrs["id"],
		Collection: attrs["collection"],
		Attrs:      attrs,
		Children:   children,
	}}, true
}

// formatJSNumber renders a float the way JavaScript String(n) does for the
// JSON numbers ADF carries ("686", "20.5").
func formatJSNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
