// Package dialect implements the known directive dialect as first-class
// typed AST nodes on the public extension contract (see the extension
// package): each kind implements extension.Node (RenderMarkdown +
// EncodeADF) and ships its Registration (parse promotion + ADF decode),
// so the dialect exercises exactly the same four-path contract a library
// consumer's custom kinds would.
//
// Every node keeps the raw directive payload (Attrs map, label Children)
// as the source of truth for rendering and encoding — attribute presence
// semantics (an empty-but-present "collection" differs from an absent
// one) and label escaping stay byte-identical to the generic directive
// forms — while the documented attributes additionally bind into typed
// convenience fields.
//
// Registrations returns the default set; the markdown parser and the
// convert decoder wire it automatically, so the root facade behaves as
// if the dialect were built in. Unknown directive names keep the generic
// ast.ContainerDirective/LeafDirective/TextDirective kinds and degrade
// exactly like remark.
//
// A few dialect behaviors intentionally stay structural in convert
// rather than inside the node implementations, because the extension
// contract is per-node and these cross node boundaries:
//
//   - ::colwidths attaches widths to the FOLLOWING sibling table on
//     encode (and is emitted from a table's cell attrs on decode); the
//     Colwidths node owns its parse/render/encode payload, convert owns
//     the cross-sibling application (Registration.DecodedByCore).
//   - ::decisions marks the FOLLOWING sibling plain bullet list as an
//     ADF decisionList on encode (and is emitted before the decoded
//     list on decode); like colwidths, convert owns the cross-sibling
//     application (Registration.DecodedByCore).
//   - the inline mark directives (:color/:bg/:u/:sub/:sup, :annotation)
//     decode from ADF text MARKS, not nodes: convert's mark machinery
//     owns which marks project and their canonical nesting order, each
//     kind's Registration.DecodeTextMark hook constructs the typed node,
//     and their EncodeADF layers the marks back via
//     EncodeContext.EncodeInlinesStyled.
//   - :fontSize is a RETIRED mark directive: it still parses (surviving
//     round trips through markdown) but no Atlassian product supports the
//     ADF mark, so its EncodeADF unwraps to the inline text and the core
//     decode dissolves any legacy fontSize mark to bare text — both with
//     a convert.CodeFontSizeDropped diagnostic.
//   - the block-mark wrappers (:::center/:::end, :::indent, :::breakout,
//     :::dataConsumer, :::fragment) decode from ADF block MARKS:
//     convert's block-mark wrapping constructs them around the marked
//     block's decoded form (Registration.DecodedByCore), and their
//     EncodeADF re-appends the mark to every encoded child block.
//   - :emoji decodes in convert's inline visitor (Emoji is
//     DecodedByCore): text-present emojis project as their text, the
//     EmojiUnicode shortname table restores unicode renderings, and only
//     custom/site emojis take the directive form.
package dialect

import (
	"strconv"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// ---------------------------------------------------------------------------
// Container kinds
// ---------------------------------------------------------------------------

// Panel is :::info/:::note/:::warning/:::success/:::error ⇄ ADF panel;
// the panel type is the directive name. A directive label, when present,
// is simply the first child paragraph (remark's representation) and
// converts as regular panel content.
type Panel struct {
	// PanelType is info|note|warning|success|error.
	PanelType string
	// Attrs is the raw directive attribute payload (no ADF equivalent;
	// kept for payload fidelity, dropped by render and encode like the
	// generic container form).
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Panel) Kind() string { return "panel" }

// ChildNodes implements ast.Parent.
func (n *Panel) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Panel) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Panel) ContainerDirectiveForm() {}

// Expand is :::expand[Title] ⇄ ADF expand/nestedExpand. The title is not
// a field: like remark's container labels, it is a leading child
// paragraph flagged DirectiveLabel (see Title), so the render path is
// identical to the generic container form — including an empty [] label.
type Expand struct {
	// Attrs is the raw directive attribute payload (no ADF equivalent).
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Expand) Kind() string { return "expand" }

// ChildNodes implements ast.Parent.
func (n *Expand) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Expand) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Expand) ContainerDirectiveForm() {}

// Title returns the expand title: the plain text of the leading
// directive-label paragraph, "" without one.
func (n *Expand) Title() string {
	if p, ok := labelParagraph(n.Children); ok {
		return ast.PlainText(p.Children)
	}
	return ""
}

// labelParagraph returns children[0] when it is a directive-label
// paragraph.
func labelParagraph(children []ast.Node) (*ast.Paragraph, bool) {
	if len(children) == 0 {
		return nil, false
	}
	p, ok := children[0].(*ast.Paragraph)
	if !ok || !p.DirectiveLabel {
		return nil, false
	}
	return p, true
}

// ---------------------------------------------------------------------------
// Leaf kinds
// ---------------------------------------------------------------------------

// Media is ::media[alt]{…} ⇄ ADF mediaSingle/mediaGroup/media: the alt
// text is the label, every other ADF attribute rides as a directive
// attribute. Attrs is the source of truth (presence matters: an empty
// "collection" still encodes); the typed fields bind the documented
// attributes for consumers.
type Media struct {
	MediaType     string
	URL           string
	Collection    string
	Path          string
	Layout        string
	ID            string
	OccurrenceKey string
	WidthType     string
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children hold the alt-text label.
	Children    []ast.Node
	Width       float64
	Height      float64
	LayoutWidth float64
	ast.BlockSpacing
	// Group marks a mediaGroup member (adjacent group items reassemble
	// one mediaGroup on encode).
	Group bool
}

// Kind implements ast.Node.
func (*Media) Kind() string { return "media" }

// ChildNodes implements ast.Parent.
func (n *Media) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Media) SetChildNodes(kids []ast.Node) { n.Children = kids }

// newMedia builds a Media from its raw directive payload, binding the
// documented attributes into the typed fields.
func newMedia(attrs map[string]string, children []ast.Node) *Media {
	return &Media{
		MediaType:     attrs["type"],
		URL:           attrs["url"],
		Collection:    attrs["collection"],
		Path:          attrs["path"],
		Layout:        attrs["layout"],
		ID:            attrs["id"],
		OccurrenceKey: attrs["occurrenceKey"],
		WidthType:     attrs["widthType"],
		Width:         floatAttr(attrs, "width"),
		Height:        floatAttr(attrs, "height"),
		LayoutWidth:   floatAttr(attrs, "layoutWidth"),
		Group:         attrs["group"] == "true",
		Attrs:         attrs,
		Children:      children,
	}
}

// floatAttr parses a numeric directive attribute (0 when absent/invalid).
func floatAttr(attrs map[string]string, key string) float64 {
	f, err := strconv.ParseFloat(attrs[key], 64)
	if err != nil {
		return 0
	}
	return f
}

// JQL is ::jql[query]{cloudId datasource columns url} ⇄ an ADF
// JQL-datasource blockCard; the query is the label.
type JQL struct {
	CloudID    string
	Datasource string
	Columns    string
	URL        string
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children hold the JQL query label.
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*JQL) Kind() string { return "jql" }

// ChildNodes implements ast.Parent.
func (n *JQL) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *JQL) SetChildNodes(kids []ast.Node) { n.Children = kids }

// Query returns the JQL query (the label's plain text).
func (n *JQL) Query() string { return ast.PlainText(n.Children) }

// newJQL builds a JQL from its raw directive payload.
func newJQL(attrs map[string]string, children []ast.Node) *JQL {
	return &JQL{
		CloudID:    attrs["cloudId"],
		Datasource: attrs["datasource"],
		Columns:    attrs["columns"],
		URL:        attrs["url"],
		Attrs:      attrs,
		Children:   children,
	}
}

// LinkCard is ::linkCard[url-or-key] ⇄ ADF blockCard: the label is the
// URL, or the short key when a SmartLinks resolver knows it.
type LinkCard struct {
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children hold the url-or-key label.
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*LinkCard) Kind() string { return "linkCard" }

// ChildNodes implements ast.Parent.
func (n *LinkCard) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *LinkCard) SetChildNodes(kids []ast.Node) { n.Children = kids }

// Target returns the card target (the label's plain text): a full URL or
// a bare smart-link key.
func (n *LinkCard) Target() string { return ast.PlainText(n.Children) }

// LinkEmbed is ::linkEmbed[url]{layout width} ⇄ ADF embedCard.
type LinkEmbed struct {
	Layout string
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children hold the url-or-key label.
	Children []ast.Node
	Width    float64
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*LinkEmbed) Kind() string { return "linkEmbed" }

// ChildNodes implements ast.Parent.
func (n *LinkEmbed) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *LinkEmbed) SetChildNodes(kids []ast.Node) { n.Children = kids }

// Target returns the embed target (the label's plain text).
func (n *LinkEmbed) Target() string { return ast.PlainText(n.Children) }

// newLinkEmbed builds a LinkEmbed from its raw directive payload.
func newLinkEmbed(attrs map[string]string, children []ast.Node) *LinkEmbed {
	return &LinkEmbed{
		Layout:   attrs["layout"],
		Width:    floatAttr(attrs, "width"),
		Attrs:    attrs,
		Children: children,
	}
}

// Colwidths is ::colwidths[79,320] ⇄ colwidth attrs on the FOLLOWING
// table's cells. The node owns its payload and all four paths; the
// cross-sibling application (attach-to-next-table on encode, emission
// from a table's cell attrs on decode) is structural and lives in the
// convert package (see the package comment and ColwidthsPlaceholder).
type Colwidths struct {
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children hold the comma-separated width list label.
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Colwidths) Kind() string { return "colwidths" }

// ChildNodes implements ast.Parent.
func (n *Colwidths) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Colwidths) SetChildNodes(kids []ast.Node) { n.Children = kids }

// Decisions is ::decisions ⇄ the ADF decisionList of the FOLLOWING plain
// bullet list: the directive marks that list, each list item becomes a
// decisionItem (state DECIDED). Like ::colwidths, the node owns its
// parse/render payload while the cross-sibling application (mark the
// next list on encode, emission before the decoded list on decode) is
// structural and lives in the convert package; a ::decisions with no
// bullet list on the following line drops with a "decisions-orphan"
// diagnostic.
type Decisions struct {
	// Attrs is the raw directive attribute payload (no ADF equivalent).
	Attrs map[string]string
	// Children hold the (normally empty) directive label.
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Decisions) Kind() string { return "decisions" }

// ChildNodes implements ast.Parent.
func (n *Decisions) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Decisions) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ---------------------------------------------------------------------------
// Inline kinds
// ---------------------------------------------------------------------------

// Mention is :mention[Name]{#account-id} ⇄ ADF mention. The directive
// itself IS the @: the label carries the bare display name (the render
// emits no leading @, and the ADF mention text gains its "@" prefix on
// encode). Parsing still accepts the legacy :mention[@Name] form — a
// leading "@" in the label is stripped.
type Mention struct {
	AccountID string
	// Attrs is the raw directive attribute payload (id, accessLevel).
	Attrs map[string]string
	// Children hold the display-name label.
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Mention) Kind() string { return "mention" }

// ChildNodes implements ast.Parent.
func (n *Mention) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Mention) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Mention) MarkdownLead() byte { return ':' }

// Status is :status[Label]{color="green"} ⇄ ADF status.
type Status struct {
	Color string
	// Attrs is the raw directive attribute payload (color, style).
	Attrs map[string]string
	// Children hold the status label.
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Status) Kind() string { return "status" }

// ChildNodes implements ast.Parent.
func (n *Status) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Status) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Status) MarkdownLead() byte { return ':' }

// MediaInline is inline :media[alt]{…} ⇄ ADF mediaInline.
type MediaInline struct {
	MediaType  string
	ID         string
	Collection string
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children hold the alt-text label.
	Children []ast.Node
}

// Kind implements ast.Node.
func (*MediaInline) Kind() string { return "mediaInline" }

// ChildNodes implements ast.Parent.
func (n *MediaInline) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *MediaInline) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*MediaInline) MarkdownLead() byte { return ':' }

// Color is :color[content]{color="#hex"} ⇄ the ADF textColor mark on the
// content's text nodes (decoded by convert's mark machinery).
type Color struct {
	Color string
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Color) Kind() string { return "color" }

// ChildNodes implements ast.Parent.
func (n *Color) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Color) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Color) MarkdownLead() byte { return ':' }

// Bg is :bg[content]{color="#hex"} ⇄ the ADF backgroundColor mark.
type Bg struct {
	Color string
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Bg) Kind() string { return "bg" }

// ChildNodes implements ast.Parent.
func (n *Bg) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Bg) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Bg) MarkdownLead() byte { return ':' }

// Underline is :u[content] ⇄ the ADF underline mark.
type Underline struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Underline) Kind() string { return "underline" }

// ChildNodes implements ast.Parent.
func (n *Underline) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Underline) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Underline) MarkdownLead() byte { return ':' }

// Sub is :sub[content] ⇄ the ADF subsup(sub) mark.
type Sub struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Sub) Kind() string { return "sub" }

// ChildNodes implements ast.Parent.
func (n *Sub) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Sub) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Sub) MarkdownLead() byte { return ':' }

// Sup is :sup[content] ⇄ the ADF subsup(sup) mark.
type Sup struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Sup) Kind() string { return "sup" }

// ChildNodes implements ast.Parent.
func (n *Sup) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Sup) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Sup) MarkdownLead() byte { return ':' }

// Compile-time contract checks.
var (
	_ extension.ContainerForm = (*Panel)(nil)
	_ extension.ContainerForm = (*Expand)(nil)
	_ extension.Node          = (*Media)(nil)
	_ extension.Node          = (*JQL)(nil)
	_ extension.Node          = (*LinkCard)(nil)
	_ extension.Node          = (*LinkEmbed)(nil)
	_ extension.Node          = (*Colwidths)(nil)
	_ extension.Node          = (*Decisions)(nil)
	_ extension.Node          = (*Mention)(nil)
	_ extension.Node          = (*Status)(nil)
	_ extension.Node          = (*MediaInline)(nil)
	_ extension.Node          = (*Color)(nil)
	_ extension.Node          = (*Bg)(nil)
	_ extension.Node          = (*Underline)(nil)
	_ extension.Node          = (*Sub)(nil)
	_ extension.Node          = (*Sup)(nil)
)
