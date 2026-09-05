package dialect

import (
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
	"github.com/pmarschik/adfast/internal/mediaurl"
)

// This file implements the adf→ast path of the dialect kinds (the decode
// hooks recognizing the ADF shapes each kind owns) and the md→ast parse
// promotions, bundled per kind into Registrations.

// Registrations returns the default dialect registration set. The
// markdown parser and the convert decoder wire it automatically; the
// slice ORDER is the decode dispatch order and is significant — the JQL
// datasource hook must probe blockCards before the LinkCard fallback.
//
// The set is assembled from one group per node shape. The groups are
// concatenated in the order below and each group keeps its own internal
// order, so the dispatch order is the reading order of the four
// functions taken together.
//
// Every text-directive constructor in the assembled set is wrapped by
// guardBareText, so a ":name" carrying neither label nor attributes
// stays the prose it is indistinguishable from (see bare.go).
func Registrations() []extension.Registration {
	regs := blockRegistrations()
	regs = append(regs, inlineRegistrations()...)
	regs = append(regs, markRegistrations()...)
	regs = append(regs, extendedRegistrations()...)
	return guardBareText(regs)
}

// blockRegistrations returns the kinds that decode from an ADF BLOCK
// node. The JQL datasource hook probes blockCards, so "jql" must precede
// the "linkCard" fallback.
func blockRegistrations() []extension.Registration {
	return []extension.Registration{
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
	}
}

// inlineRegistrations returns the kinds that decode from an ADF INLINE
// node.
func inlineRegistrations() []extension.Registration {
	return []extension.Registration{
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
	}
}

// markRegistrations returns the kinds that decode from ADF text MARKS,
// not nodes: convert's mark machinery owns which marks project and their
// canonical nesting order, and dispatches each mark to the
// DecodeTextMark hooks below for node construction (see the package
// comment).
func markRegistrations() []extension.Registration {
	return []extension.Registration{
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
	m := NewMedia(d.Attrs, d.Children)
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
		if img := mediaAsImage(media, single, ctx.PreserveLocalImages()); img != nil {
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
// paragraph decodeMediaNode's image forms produce. A linked image keeps
// the picture one level down, inside the link wrapper, so the walk
// descends into it: the title belongs to the IMAGE
// ([![alt](url "caption")](href)), never to the link.
func setImageTitle(n ast.Node, title string) {
	p, ok := n.(*ast.Paragraph)
	if !ok {
		return
	}
	for _, child := range p.Children {
		if link, ok := child.(*ast.Link); ok && len(link.Children) == 1 {
			child = link.Children[0]
		}
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
	if img := mediaAsImage(media, single, ctx.PreserveLocalImages()); img != nil {
		return img
	}
	if img := fileMediaAsImage(media, single, ctx); img != nil {
		return img
	}
	return mediaLeafNode(media, single, false, ctx)
}

// singleBlocksImage reports whether a mediaSingle wrapper carries
// properties the plain-image markdown form cannot hold: a display width
// or widthType, or a layout other than the given default.
//
// naturalWidth is naturalDisplayWidth's answer for the wrapper's own
// media, and it excuses the display width: a no-op resize is ALREADY
// omitted from the canonical ::media (see mediaOmissionsOf), so letting
// it block the image form here would mean the projection both declares
// it redundant and treats it as content. The directive it produced would
// then project to an image on the NEXT pass — the formatter changing its
// own output, which is the one thing an idempotent formatter may not do.
func singleBlocksImage(single *adf.MediaSingle, defaultLayout string, naturalWidth bool) bool {
	if single == nil {
		return false
	}
	if !naturalWidth {
		if single.Width != nil || single.WidthType != nil {
			return true
		}
	}
	// A non-numeric width or non-string widthType (kept in Extra) is not a
	// value naturalDisplayWidth could have compared, so it still blocks.
	if adf.HasExtra(single, "width") || adf.HasExtra(single, "widthType") {
		return true
	}
	if single.Layout != nil && *single.Layout != defaultLayout {
		return true
	}
	// A non-string layout value (kept in Extra) never equals the default.
	return adf.HasExtra(single, "layout")
}

// naturalDisplayWidth reports whether the wrapper's display width is a
// no-op resize: a pixel width equal to the picture's own intrinsic
// width, so the image renders at its natural size (the ~68%
// Jira-default case).
func naturalDisplayWidth(media *adf.Media, single *adf.MediaSingle) bool {
	return single != nil && single.Width != nil && media.Width != nil &&
		*single.Width == *media.Width && strDeref(single.WidthType) == "pixel"
}

// mediaBlocksImage reports whether the media node (or its mediaSingle
// wrapper) carries a property the plain-image markdown form cannot hold:
// an occurrenceKey, a non-empty collection, a border or annotation mark,
// or a wrapper richer than the given default layout. It is the half the
// file and the external image paths share; each adds its own
// type-specific checks.
//
// The link mark is the one member of the media mark union that does NOT
// block: markdown has a form for it, the [![alt](url)](href) wrapper
// imageParagraph writes.
func mediaBlocksImage(media *adf.Media, single *adf.MediaSingle, defaultLayout string) bool {
	if media.OccurrenceKey != nil || adf.HasExtra(media, "occurrenceKey") {
		return true
	}
	if media.Collection != nil && *media.Collection != "" {
		return true
	}
	if adf.HasMark(media.Marks, "border") {
		return true
	}
	if _, ok := mediaAnnotationMark(media.Marks); ok {
		return true
	}
	return singleBlocksImage(single, defaultLayout, naturalDisplayWidth(media, single))
}

// imageParagraph wraps a plain ![alt](url) image in its own paragraph,
// the block form both image paths return. A non-empty href makes it the
// LINKED image form, [![alt](url)](href "hrefTitle") — the exact markdown
// the encode leg turns back into media plus a link mark, so the two
// cannot drift.
//
// hrefTitle is the link's own title and goes on the LINK. The image's
// title is a different fact — the caption, written by setImageTitle onto
// the picture one level down — and the two can ride together:
// [![alt](url "caption")](href "hrefTitle").
func imageParagraph(url, alt, href, hrefTitle string) ast.Node {
	img := &ast.Image{URL: url}
	if alt != "" {
		img.Children = []ast.Node{&ast.Text{Value: alt}}
	}
	var child ast.Node = img
	if href != "" {
		// Explicit: the resource form is what a markdown parse of this
		// output produces, so both legs describe one node identically.
		child = &ast.Link{URL: href, Title: hrefTitle, Explicit: true, Children: []ast.Node{img}}
	}
	return &ast.Paragraph{Children: []ast.Node{child}}
}

// mediaLink answers what a link mark puts on a media node: the
// destination the picture links to, and the advisory title spelled after
// it.
//
// Two placements are schema-legal (see docs/adf-coverage.md). adfast
// WRITES the mark on the media node — that is the placement the Jira ADF
// reference documents for media in so many words — so the media node is
// read first. The mediaSingle wrapper is read as a fallback, because a
// remote written by Atlassian's own editor may spell it there and
// dropping it would lose the same href this fix exists to keep. When
// BOTH carry one and they disagree, the media node wins: a document with
// two different destinations on one picture has no markdown form (a link
// cannot nest), and the media node's is the one adfast itself would have
// written.
//
// The title comes from the mark that won, never from the other one: the
// two are one markdown link, and pairing a wrapper's title with the
// media node's href would invent a link no document contains.
//
// The mark's remaining attributes still drop — the id, collection and
// occurrenceKey a link mark may carry have no place in a markdown link,
// and the text-link projection drops them the same way (see convert's
// flatInline).
func mediaLink(media *adf.Media, single *adf.MediaSingle) (href, title string) {
	if link, ok := mediaLinkMark(media.Marks); ok {
		return *link.Href, strDeref(link.Title)
	}
	if single != nil {
		if link, ok := mediaLinkMark(single.Marks); ok {
			return *link.Href, strDeref(link.Title)
		}
	}
	return "", ""
}

// mediaLinkMark answers the first link mark that names a destination. An
// absent href (or an empty one) is no destination: there is nothing to
// click and nothing to keep, matching the encode side's refusal to write
// a link mark without one.
func mediaLinkMark(marks []adf.Mark) (*adf.Link, bool) {
	link, ok := adf.FindMark[*adf.Link](marks)
	if !ok || link.Href == nil || *link.Href == "" {
		return nil, false
	}
	return link, true
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
	if !dimsMatchAsset(media, asset) {
		return nil
	}
	if mediaBlocksImage(media, single, "align-start") {
		return nil
	}
	href, hrefTitle := mediaLink(media, single)
	return imageParagraph(asset.Path, media.Alt, href, hrefTitle)
}

// dimsMatchAsset reports whether the media's recorded intrinsic
// dimensions are exactly the local file's own, the case the encode side
// re-derives rather than reads back from the markdown.
func dimsMatchAsset(media *adf.Media, asset extension.MediaAsset) bool {
	if media.Width == nil || media.Height == nil {
		return false
	}
	return float64(asset.Width) == *media.Width && float64(asset.Height) == *media.Height
}

// mediaAsImage renders an external media node as a plain markdown image
// when every ADF property is expressible by ![alt](url): external type
// with a non-empty URL (the shape the encode side maps back to external
// media — anything else would drop on re-encode), no
// dimensions/occurrenceKey/collection, and at most the default
// layout="center" wrapper. Anything richer falls back to ::media. The URL
// may be absolute (http(s), the usual embedded-image case) or a
// document-relative path (a local image reference not yet uploaded to
// Jira — preserved so the round-trip and the push upload can still see it).
func mediaAsImage(media *adf.Media, single *adf.MediaSingle, preserveLocal bool) ast.Node {
	if media.Type != "external" {
		return nil
	}
	// mediaurl.ProjectsToImage owns the URL decision for every leg of the
	// projection — an empty url, and a document-relative one outside
	// WithPreserveLocalImages, stay a ::media directive here.
	if !mediaurl.ProjectsToImage(media.URL, preserveLocal) {
		return nil
	}
	if hasDimension(media, "width", media.Width) || hasDimension(media, "height", media.Height) {
		return nil
	}
	if mediaBlocksImage(media, single, "center") {
		return nil
	}
	href, hrefTitle := mediaLink(media, single)
	return imageParagraph(media.URL, media.Alt, href, hrefTitle)
}

// hasDimension reports whether the media carries the named intrinsic
// dimension, either as the typed field or as an unmodeled Extra entry
// (a non-numeric value the typed field cannot hold).
func hasDimension(media *adf.Media, name string, field *float64) bool {
	return field != nil || adf.HasExtra(media, name)
}

// mediaOmissions are the facts a canonical ::media leaves attributes out on:
// what the asset store has for this media, whether its recorded dimensions are
// just the local file's own, and whether its display width is a no-op resize.
type mediaOmissions struct {
	asset        extension.MediaAsset
	isLocal      bool
	dimsMatch    bool
	naturalWidth bool
}

// mediaOmissionsOf derives them for one media node.
func mediaOmissionsOf(media *adf.Media, single *adf.MediaSingle, ctx extension.DecodeContext) mediaOmissions {
	var om mediaOmissions
	om.asset, om.isLocal = ctx.Asset(media.ID)
	// A downloaded asset whose intrinsic dimensions match the file lets us omit
	// width/height — encode re-derives them from the local file (AssetDims).
	om.dimsMatch = om.isLocal && om.asset.HasDim &&
		media.Width != nil && media.Height != nil &&
		float64(om.asset.Width) == *media.Width && float64(om.asset.Height) == *media.Height
	// A no-op resize (see naturalDisplayWidth) lets us drop the redundant
	// layoutWidth/widthType; the plain-size media is reconstructed on encode
	// without them. This normalizes an explicit natural width to the
	// no-width form (a one-time, visually-identical change on the next
	// push), matching the many media that carry no display width at all.
	// The same answer excuses it in singleBlocksImage, so the projection
	// cannot call it redundant here and content there.
	om.naturalWidth = naturalDisplayWidth(media, single)
	return om
}

// mediaAnnotationAttrs writes the annotation mark's attributes: the
// third member of the media mark union, a Confluence inline-comment
// anchor sitting on the picture. It has no markdown form of its own —
// :annotation[…] wraps inline CONTENT, and a block media leaf is not
// content it can wrap — so it rides as attributes the way the border
// mark does, under a compound name because a bare id on a ::media node
// is already the media's own.
//
// Pushing a body without the anchor orphans the comment thread on the
// remote, which is why this is carried rather than reported: a
// diagnostic would name the loss without preventing it.
//
// An annotation with no id anchors nothing and is not written back. That
// is the same answer the inline projection gives (an :annotation with no
// id dissolves to its children), so a picture is not degraded out of its
// image form for a mark that could never find its thread.
func mediaAnnotationAttrs(marks []adf.Mark, attrs map[string]string) {
	ann, ok := mediaAnnotationMark(marks)
	if !ok {
		return
	}
	attrs["annotationId"] = ann.ID
	attrs["annotationType"] = annotationTypeOr(ann.AnnotationType)
}

// mediaAnnotationMark answers a media node's usable annotation mark: one
// that carries an id, the only kind that anchors a comment thread and
// therefore the only kind worth blocking the image form for. It reads
// the mark slice rather than the node so the block leaf and the inline
// chip share it, as they share the link mark's reader.
func mediaAnnotationMark(marks []adf.Mark) (*adf.Annotation, bool) {
	ann, ok := adf.FindMark[*adf.Annotation](marks)
	if !ok || ann.ID == "" {
		return nil, false
	}
	return ann, true
}

// annotationTypeOr defaults an empty annotation type to the one value
// the schema defines, matching decodeAnnotationMark on the inline side.
func annotationTypeOr(annotationType string) string {
	if annotationType == "" {
		return "inlineComment"
	}
	return annotationType
}

// mediaBorderAttrs writes the border mark's attributes.
func mediaBorderAttrs(media *adf.Media, attrs map[string]string) {
	border, ok := adf.FindMark[*adf.Border](media.Marks)
	if !ok {
		return
	}
	if border.Color != "" {
		attrs["borderColor"] = border.Color
	}
	if border.Size != 0 {
		attrs["borderSize"] = strconv.Itoa(border.Size)
	}
}

// mediaLinkAttrs writes the link mark's destination and its advisory
// title. The directive form carries them as attributes for the same
// reason it carries the border mark as two: a ::media node is a block
// leaf with no room for a markdown link around it, and the alternative
// is losing the href. The image-expressible forms use the
// [![alt](url)](href "hrefTitle") wrapper instead (see imageParagraph).
//
// The title is written only beside an href, because that is the only
// shape the encode can rebuild the mark from (see linkMarkFromAttrs).
func mediaLinkAttrs(media *adf.Media, single *adf.MediaSingle, attrs map[string]string) {
	href, hrefTitle := mediaLink(media, single)
	if href == "" {
		return
	}
	attrs["href"] = href
	if hrefTitle != "" {
		attrs["hrefTitle"] = hrefTitle
	}
}

// mediaShapeAttrs writes what the media leaf says about itself: its container,
// its intrinsic size, its occurrence key.
func mediaShapeAttrs(media *adf.Media, om mediaOmissions, group bool, attrs map[string]string) {
	// Omit an empty collection on file media (the attachment default);
	// mediaFromAttrs re-adds it.
	if media.Collection != nil && (media.Type != "file" || *media.Collection != "") {
		attrs["collection"] = *media.Collection
	}
	if group {
		attrs["group"] = "true"
	}
	if media.Height != nil && !om.dimsMatch {
		attrs["height"] = formatJSNumber(*media.Height)
	}
	if media.Width != nil && !om.dimsMatch {
		attrs["width"] = formatJSNumber(*media.Width)
	}
	if v := strDeref(media.OccurrenceKey); v != "" {
		attrs["occurrenceKey"] = v
	}
}

// mediaSingleAttrs writes what the mediaSingle wrapper says: how the image is
// aligned, and how large it is displayed.
func mediaSingleAttrs(media *adf.Media, single *adf.MediaSingle, om mediaOmissions, attrs map[string]string) {
	if single == nil {
		return
	}
	// Omit the file-media default layout ("align-start"); mediaSingleFromAttrs
	// re-infers it when a file-type directive carries no layout, so the
	// round-trip stays lossless while the directive stays terse.
	//
	// The elision stops there, and deliberately: "center", which
	// mediaSingleFromAttrs stands in for every OTHER media type, is written
	// out rather than omitted. The reference corpus pins the contrast in two
	// adjacent rows of testdata/directive_fixtures.json — a file media
	// carrying align-start renders without the attribute, while an external
	// media carrying center renders as `::media[shot]{height="50"
	// layout="center" type="external" …}`. Offered the same chance to elide,
	// the reference declines for external media, so the canonical directive
	// for a non-file media spells its layout. Eliding it here instead would
	// make the round trip an identity at the cost of that corpus row, which
	// is the trade this asymmetry deliberately refuses.
	if layout := strDeref(single.Layout); layout != "" && (media.Type != "file" || layout != "align-start") {
		attrs["layout"] = layout
	}
	if single.Width != nil && !om.naturalWidth {
		attrs["layoutWidth"] = formatJSNumber(*single.Width)
	}
	if v := strDeref(single.WidthType); v != "" && !om.naturalWidth {
		attrs["widthType"] = v
	}
}

// mediaSourceAttrs writes where the media comes from.
func mediaSourceAttrs(media *adf.Media, om mediaOmissions, attrs map[string]string) {
	// When the media is a locally downloaded asset, emit its markdown-relative
	// path and OMIT the explicit id — encode resolves the id back from the path
	// via the (issue-scoped) asset store. When it is not local, keep the
	// explicit id (nothing can resolve it).
	if om.isLocal {
		attrs["path"] = om.asset.Path
	} else if media.ID != "" {
		attrs["id"] = media.ID
	}
	// Omit the default media type ("file"); mediaFromAttrs re-infers it when a
	// directive carries no type.
	if media.Type != "" && media.Type != "file" {
		attrs["type"] = media.Type
	}
	if media.URL != "" {
		attrs["url"] = media.URL
	}
}

// mediaLeafNode serializes a media node (with its optional mediaSingle
// wrapper or group membership) as a ::media node: the alt text is the
// label, every other ADF attribute rides as a directive attribute (the
// inverse of Media.EncodeADF).
func mediaLeafNode(media *adf.Media, single *adf.MediaSingle, group bool, ctx extension.DecodeContext) *Media {
	om := mediaOmissionsOf(media, single, ctx)
	attrs := map[string]string{}
	mediaAnnotationAttrs(media.Marks, attrs)
	mediaBorderAttrs(media, attrs)
	mediaLinkAttrs(media, single, attrs)
	mediaShapeAttrs(media, om, group, attrs)
	mediaSingleAttrs(media, single, om, attrs)
	mediaSourceAttrs(media, om, attrs)
	var children []ast.Node
	if media.Alt != "" {
		children = []ast.Node{&ast.Text{Value: media.Alt}}
	}
	return NewMedia(attrs, children)
}

// decodeDatasource converts a JQL-datasource blockCard to a
// ::jql[<query>] node when its shape is fully expressible: a jira/jql
// datasource with cloudId+jql parameters and at most one table view with
// plain column keys (the documented ADF shape — implemented from the
// Atlassian schema, and not yet verified against a datasource card
// captured from a live instance). Richer shapes fall back to ::linkCard
// (decodeBlockCard runs after this hook).
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
	// Omit the default media type ("file"); mediaInline encode re-infers it.
	if mi.Type != "" && mi.Type != "file" {
		attrs["type"] = mi.Type
	}
	// A linked inline attachment keeps its destination, and the title
	// spelled after it, as attributes: this directive form is what a
	// mediaInline the asset store cannot place falls back to, and a
	// :media[…] label is not a link. Same attribute names as the block
	// form, because the two are one projection (see mediaLinkAttrs).
	if link, ok := mediaLinkMark(mi.Marks); ok {
		attrs["href"] = *link.Href
		if title := strDeref(link.Title); title != "" {
			attrs["hrefTitle"] = title
		}
	}
	// An inline-comment anchor rides along too, under the same names the
	// block form uses (see mediaAnnotationAttrs). The inline chip has no
	// image form to be degraded out of, so nothing else changes here.
	mediaAnnotationAttrs(mi.Marks, attrs)
	var children []ast.Node
	if mi.Alt != "" {
		children = []ast.Node{&ast.Text{Value: mi.Alt}}
	}
	return []ast.Node{&MediaInline{
		MediaType:  mi.Type,
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
