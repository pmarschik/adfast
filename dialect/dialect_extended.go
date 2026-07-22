package dialect

import (
	"encoding/json"
	"strings"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// The extended dialect kinds: dates, placeholders, emoji fallbacks,
// captioned media, the extension family, synced blocks, page layouts,
// and the mark wrappers (inline annotation/fontSize, block
// alignment/indentation/breakout/dataConsumer/fragment). Every kind
// follows the established pattern: raw directive payload (Attrs,
// Children) as the source of truth, typed convenience fields for the
// documented attributes.

// ---------------------------------------------------------------------------
// Inline kinds
// ---------------------------------------------------------------------------

// Date is :date[2026-07-15]{timestamp="…"} ⇄ ADF date. The timestamp
// attribute (milliseconds since epoch) is authoritative; the label is
// the human-readable UTC day derived from it.
type Date struct {
	// Attrs is the raw directive attribute payload (timestamp, localId).
	Attrs map[string]string
	// Children hold the YYYY-MM-DD label.
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Date) Kind() string { return "date" }

// ChildNodes implements ast.Parent.
func (n *Date) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Date) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Date) MarkdownLead() byte { return ':' }

// Placeholder is :placeholder[Type something…] ⇄ ADF placeholder; the
// label is the placeholder text.
type Placeholder struct {
	// Attrs is the raw directive attribute payload (localId).
	Attrs map[string]string
	// Children hold the placeholder text label.
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Placeholder) Kind() string { return "placeholder" }

// ChildNodes implements ast.Parent.
func (n *Placeholder) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Placeholder) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Placeholder) MarkdownLead() byte { return ':' }

// Emoji is :emoji{shortName=":name:" id?} ⇄ an ADF emoji whose rendered
// text is unknown. Emojis WITH a text attribute keep rendering as that
// text (deliberately lossy: shortName/id degrade to plain text across
// markdown persistence); this directive is the fallback for the rest,
// after the shortname → unicode table (EmojiUnicode) also missed.
type Emoji struct {
	// Attrs is the raw directive attribute payload (shortName, id, text).
	Attrs map[string]string
	// Children are unused (the directive is attribute-only).
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Emoji) Kind() string { return "emoji" }

// ChildNodes implements ast.Parent.
func (n *Emoji) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Emoji) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Emoji) MarkdownLead() byte { return ':' }

// Annotation is :annotation[text]{#id annotationType="inlineComment"} ⇄
// the ADF annotation mark on the content's text nodes (decoded by
// convert's mark machinery). Confluence inline-comment threads are
// anchored by these marks — submitting a body without them orphans the
// threads (they are NOT re-inserted server-side); the mapping exists so
// annotated text can be edited through markdown without severing them.
type Annotation struct {
	// ID is the annotation id (the {#id} shortcut).
	ID string
	// Attrs is the raw directive attribute payload (id, annotationType).
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*Annotation) Kind() string { return "annotation" }

// ChildNodes implements ast.Parent.
func (n *Annotation) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Annotation) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*Annotation) MarkdownLead() byte { return ':' }

// FontSize is :fontSize[text]{small}, a RETIRED inline mark directive.
// It still parses (the single bare attribute is the size; a named
// size="…" attribute wins when both are present) so existing documents
// read cleanly, but no Atlassian product supports the ADF fontSize mark:
// its EncodeADF unwraps to the inline text and the core ADF decode
// dissolves any legacy fontSize mark to bare text, both with a
// convert.CodeFontSizeDropped diagnostic. The size is not preserved.
type FontSize struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*FontSize) Kind() string { return "fontSize" }

// ChildNodes implements ast.Parent.
func (n *FontSize) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *FontSize) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*FontSize) MarkdownLead() byte { return ':' }

// Size returns the fontSize value: the named size attribute, or the
// single bare attribute ({small}).
func (n *FontSize) Size() string { return namedOrBareValue(n.Attrs, "size") }

// InlineExtension is :extension{type key parameters? text? localId?} ⇄
// ADF inlineExtension. The directive attributes type/key are the short
// surface for the ADF extensionType/extensionKey fields.
type InlineExtension struct {
	// Attrs is the raw directive attribute payload; parameters carries
	// the canonical-JSON encoding (see EncodeJSONAttr).
	Attrs map[string]string
	// Children are unused (the directive is attribute-only).
	Children []ast.Node
}

// Kind implements ast.Node.
func (*InlineExtension) Kind() string { return "inlineExtension" }

// ChildNodes implements ast.Parent.
func (n *InlineExtension) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *InlineExtension) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*InlineExtension) MarkdownLead() byte { return ':' }

// ---------------------------------------------------------------------------
// Leaf kinds
// ---------------------------------------------------------------------------

// Extension is ::extension{type key …} ⇄ ADF extension (a bodiless
// macro). The directive attributes type/key are the short surface for
// the ADF extensionType/extensionKey fields.
type Extension struct {
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children are unused (the directive is attribute-only).
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Extension) Kind() string { return "extension" }

// ChildNodes implements ast.Parent.
func (n *Extension) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Extension) SetChildNodes(kids []ast.Node) { n.Children = kids }

// SyncBlock is ::syncBlock{resourceId="…" localId="…"} ⇄ ADF syncBlock
// (a reference to a synced block).
type SyncBlock struct {
	// Attrs is the raw directive attribute payload.
	Attrs map[string]string
	// Children are unused (the directive is attribute-only).
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*SyncBlock) Kind() string { return "syncBlock" }

// ChildNodes implements ast.Parent.
func (n *SyncBlock) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *SyncBlock) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ---------------------------------------------------------------------------
// Container kinds
// ---------------------------------------------------------------------------

// MediaCaption is the container form of ::media — :::media[alt]{…} with
// the caption's inline content as the body — ⇄ ADF mediaSingle with a
// caption child. Media without a caption keeps the ::media leaf (or
// plain-image) form byte-identically.
type MediaCaption struct {
	// Attrs is the raw directive attribute payload (same set as ::media).
	Attrs map[string]string
	// Children: an optional leading directive-label paragraph (the alt
	// text) followed by the caption body blocks.
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*MediaCaption) Kind() string { return "mediaCaption" }

// ChildNodes implements ast.Parent.
func (n *MediaCaption) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *MediaCaption) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*MediaCaption) ContainerDirectiveForm() {}

// BodiedExtension is :::extension{…} ⇄ ADF bodiedExtension (body
// blocks) or multiBodiedExtension (when every child is a :::frame
// container, or the multi attribute marks a frameless one).
type BodiedExtension struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*BodiedExtension) Kind() string { return "bodiedExtension" }

// ChildNodes implements ast.Parent.
func (n *BodiedExtension) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *BodiedExtension) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*BodiedExtension) ContainerDirectiveForm() {}

// Frame is :::frame ⇄ ADF extensionFrame: one body of a
// multiBodiedExtension.
type Frame struct {
	// Attrs is the raw directive attribute payload (none documented).
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Frame) Kind() string { return "extensionFrame" }

// ChildNodes implements ast.Parent.
func (n *Frame) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Frame) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Frame) ContainerDirectiveForm() {}

// BodiedSyncBlock is :::syncBlock{resourceId localId} ⇄ ADF
// bodiedSyncBlock (the source body of a synced block).
type BodiedSyncBlock struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*BodiedSyncBlock) Kind() string { return "bodiedSyncBlock" }

// ChildNodes implements ast.Parent.
func (n *BodiedSyncBlock) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *BodiedSyncBlock) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*BodiedSyncBlock) ContainerDirectiveForm() {}

// Section is :::section{columnRuleStyle? localId?} ⇄ ADF layoutSection;
// its children are :::column containers.
type Section struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Section) Kind() string { return "layoutSection" }

// ChildNodes implements ast.Parent.
func (n *Section) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Section) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Section) ContainerDirectiveForm() {}

// Column is :::column{width="33.33" valign? localId?} ⇄ ADF
// layoutColumn.
type Column struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Column) Kind() string { return "layoutColumn" }

// ChildNodes implements ast.Parent.
func (n *Column) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Column) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Column) ContainerDirectiveForm() {}

// ---------------------------------------------------------------------------
// Block-mark wrapper kinds
// ---------------------------------------------------------------------------

// Align is :::center / :::end ⇄ the ADF alignment block mark on each
// wrapped block (decoded by convert's block-mark wrapping).
type Align struct {
	// Align is "center" or "end" (the directive name).
	Align string
	// Attrs is the raw directive attribute payload (none documented).
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Align) Kind() string { return "align" }

// ChildNodes implements ast.Parent.
func (n *Align) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Align) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Align) ContainerDirectiveForm() {}

// Indent is :::indent{2} ⇄ the ADF indentation block mark; the single
// bare attribute is the level (a named level="…" attribute wins).
type Indent struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Indent) Kind() string { return "indent" }

// ChildNodes implements ast.Parent.
func (n *Indent) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Indent) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Indent) ContainerDirectiveForm() {}

// Level returns the indentation level string: the named level
// attribute, or the single bare attribute ({2}).
func (n *Indent) Level() string { return namedOrBareValue(n.Attrs, "level") }

// Breakout is :::breakout{wide} ⇄ the ADF breakout block mark; the
// single bare attribute is the mode (a named mode="…" attribute wins),
// width rides as a named attribute.
type Breakout struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Breakout) Kind() string { return "breakout" }

// ChildNodes implements ast.Parent.
func (n *Breakout) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Breakout) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Breakout) ContainerDirectiveForm() {}

// Mode returns the breakout mode: the named mode attribute, or the
// single bare attribute ({wide}).
func (n *Breakout) Mode() string { return namedOrBareValue(n.Attrs, "mode") }

// DataConsumer is :::dataConsumer{sources="id1,id2"} ⇄ the ADF
// dataConsumer block mark; sources is a comma-separated list of opaque
// source ids (see ParseSources). The ADF wire model keeps sources as a
// JSON string array — only the directive surface is the comma list.
type DataConsumer struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*DataConsumer) Kind() string { return "dataConsumer" }

// ChildNodes implements ast.Parent.
func (n *DataConsumer) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *DataConsumer) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*DataConsumer) ContainerDirectiveForm() {}

// Fragment is :::fragment{localId="…" name?} ⇄ the ADF fragment block
// mark (a stable reference to a table or extension).
type Fragment struct {
	// Attrs is the raw directive attribute payload.
	Attrs    map[string]string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Fragment) Kind() string { return "fragment" }

// ChildNodes implements ast.Parent.
func (n *Fragment) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Fragment) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*Fragment) ContainerDirectiveForm() {}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// namedOrBareValue resolves a single-valued directive's value: the
// named attribute when non-empty, otherwise the single bare attribute
// (exactly one key with an empty value — the {value} form). Ambiguous
// payloads (several bare attributes) resolve to "".
func namedOrBareValue(attrs map[string]string, named string) string {
	if v := attrs[named]; v != "" {
		return v
	}
	bare, count := "", 0
	for k, v := range attrs {
		if v == "" {
			bare = k
			count++
		}
	}
	if count == 1 {
		return bare
	}
	return ""
}

// EncodeJSONAttr serializes an arbitrary JSON value into a directive
// attribute value: canonical JSON (json.Marshal — sorted object keys, no
// insignificant whitespace, & < > escaped as \uXXXX inside strings). The
// result is the plain JSON string; the markdown renderer owns the
// quote-safe serialization (single-quoting a value that carries a double
// quote but no single quote, or the &quot; character-reference fallback
// otherwise — see writeDirectiveAttrs). DecodeJSONAttr is the inverse.
func EncodeJSONAttr(v any) (string, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// DecodeJSONAttr reverses EncodeJSONAttr. Any &quot; character references
// are decoded first (they only appear as the renderer's fallback
// encoding for payloads carrying both a double and a single quote;
// json.Marshal escapes & as &, so raw JSON never carries a literal
// &quot;). A value that does not parse as JSON is returned verbatim as a
// JSON string value, so hand-written payloads degrade instead of dropping.
func DecodeJSONAttr(s string) any {
	raw := strings.ReplaceAll(s, "&quot;", `"`)
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}

// ParseSources splits a dataConsumer sources attribute — a
// comma-separated list of opaque source ids — into its ids, trimming
// surrounding spaces and dropping empty entries. Source ids are opaque
// strings (UUID-like) that do not contain commas, so a comma list is a
// lossless surface for them; an id with a literal comma is unsupported.
func ParseSources(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if id := strings.TrimSpace(p); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// EncodeSources joins source ids into the comma-separated dataConsumer
// sources attribute surface (see ParseSources).
func EncodeSources(sources []string) string {
	return strings.Join(sources, ",")
}

// Compile-time contract checks for the extended kinds.
var (
	_ extension.Node          = (*Date)(nil)
	_ extension.Node          = (*Placeholder)(nil)
	_ extension.Node          = (*Emoji)(nil)
	_ extension.Node          = (*Annotation)(nil)
	_ extension.Node          = (*FontSize)(nil)
	_ extension.Node          = (*InlineExtension)(nil)
	_ extension.Node          = (*Extension)(nil)
	_ extension.Node          = (*SyncBlock)(nil)
	_ extension.ContainerForm = (*MediaCaption)(nil)
	_ extension.ContainerForm = (*BodiedExtension)(nil)
	_ extension.ContainerForm = (*Frame)(nil)
	_ extension.ContainerForm = (*BodiedSyncBlock)(nil)
	_ extension.ContainerForm = (*Section)(nil)
	_ extension.ContainerForm = (*Column)(nil)
	_ extension.ContainerForm = (*Align)(nil)
	_ extension.ContainerForm = (*Indent)(nil)
	_ extension.ContainerForm = (*Breakout)(nil)
	_ extension.ContainerForm = (*DataConsumer)(nil)
	_ extension.ContainerForm = (*Fragment)(nil)
)
