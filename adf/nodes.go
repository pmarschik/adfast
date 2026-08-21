package adf

// The known node kinds. Field ↔ attribute mapping notes:
//
//   - Plain string/int/bool/slice fields encode iff non-zero; a decoded
//     attribute whose value is the zero value (or does not match the
//     field's type exactly) stays in Extra so re-encoding is lossless.
//   - Pointer fields model attributes whose presence matters even at the
//     zero value (an empty-but-present "collection" differs from an
//     absent one, taskItem always carries localId "").
//   - The Tight list field maps to the synthetic "tight" attribute
//     WithPreserveListTightness stores, the Heading Anchor field to the
//     synthetic "anchor" attribute, and the Table Align field to the
//     synthetic "align" attribute; all three are internal-only and never
//     appear in documents sent to Jira (see IsWireSafe).

// Paragraph is an ADF paragraph block.
type Paragraph struct {
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// Heading is an ADF heading block; Level is the "level" attribute (1–6).
//
// Anchor is the synthetic "anchor" attribute carrying a markdown heading's
// explicit {#id} (ast.Heading.ID). ADF has no platform-neutral anchor
// construct — Confluence spells one as an anchor-macro inlineExtension
// inside the heading content, and Jira has none at all — so this is a
// never-wire carrier (see IsWireSafe) that a product addon must lower or
// drop before submission. It is NOT heading.attrs.localId: that attribute
// is a node-identity UUID and creates no link target.
type Heading struct {
	Extra   map[string]any
	Anchor  string
	Content []Node
	Marks   []Mark
	Level   int
}

// Blockquote is an ADF blockquote block.
type Blockquote struct {
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// Rule is an ADF rule (thematic break).
type Rule struct {
	Extra map[string]any
	Marks []Mark
}

// CodeBlock is an ADF codeBlock; Language is the "language" attribute.
// Its content is text nodes (which may carry syntax-highlight marks).
type CodeBlock struct {
	Extra    map[string]any
	Language string
	Content  []Node
	Marks    []Mark
}

// BulletList is an ADF bulletList. Tight is the "tight"
// list-tightness attribute written by WithPreserveListTightness (never
// wire; see IsWireSafe).
type BulletList struct {
	Tight   *bool
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// OrderedList is an ADF orderedList; Order is the "order" attribute
// (start number, kept as a pointer because 0 is a genuine "0)" list).
// Tight is the tightness attribute like BulletList's.
type OrderedList struct {
	Order   *int
	Tight   *bool
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// ListItem is an ADF listItem.
type ListItem struct {
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// TaskList is an ADF taskList; LocalID is the "localId" attribute
// (always present on Jira documents, "" when locally built).
type TaskList struct {
	LocalID *string
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// TaskItem is an ADF taskItem; State is "TODO" or "DONE".
type TaskItem struct {
	LocalID *string
	Extra   map[string]any
	State   string
	Content []Node
	Marks   []Mark
}

// DecisionList is an ADF decisionList.
type DecisionList struct {
	LocalID *string
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// DecisionItem is an ADF decisionItem; State is "DECIDED".
type DecisionItem struct {
	LocalID *string
	Extra   map[string]any
	State   string
	Content []Node
	Marks   []Mark
}

// Table is an ADF table; the layout attributes are carried for
// losslessness (the markdown projection does not read them).
//
// Align is the synthetic "align" attribute carrying a GFM table's column
// alignment (ast.Table.Align), one entry per visual column from the left:
// "left", "right", "center", or "" for a column the delimiter row leaves
// bare. ADF tables have no alignment attribute of any kind — the only
// alignment in the schema is the paragraph/heading Alignment mark, which
// is not a table column property — so this is a never-wire carrier (see
// IsWireSafe) that keeps md → adf → md faithful and that a consumer must
// strip before submission. It is nil for a table without alignment, which
// is why an unaligned table's wire payload is unchanged.
type Table struct {
	Width                 *float64
	IsNumberColumnEnabled *bool
	Extra                 map[string]any
	Layout                string
	LocalID               string
	DisplayMode           string
	Align                 []string
	Content               []Node
	Marks                 []Mark
}

// TableRow is an ADF tableRow.
type TableRow struct {
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// TableCell is an ADF tableCell; Colspan/Rowspan cover merged cells and
// Colwidth is the per-cell column-width array Jira repeats on all rows.
type TableCell struct {
	Extra    map[string]any
	Colwidth []float64
	Content  []Node
	Marks    []Mark
	Colspan  int
	Rowspan  int
}

// TableHeader is an ADF tableHeader (same attributes as TableCell).
type TableHeader struct {
	Extra    map[string]any
	Colwidth []float64
	Content  []Node
	Marks    []Mark
	Colspan  int
	Rowspan  int
}

// Panel is an ADF panel; PanelType is info|note|warning|success|error.
type Panel struct {
	Extra     map[string]any
	PanelType string
	Content   []Node
	Marks     []Mark
}

// Expand is an ADF expand; Title is the "title" attribute (a pointer:
// the markdown conversion always emits it, even empty).
type Expand struct {
	Title   *string
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// NestedExpand is an ADF nestedExpand (an expand inside another block).
type NestedExpand struct {
	Title   *string
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// MediaSingle is an ADF mediaSingle wrapper around one media node. The
// attribute fields are pointers because their presence (even zero)
// makes the media too rich for the plain-image markdown form.
type MediaSingle struct {
	Layout    *string
	Width     *float64
	WidthType *string
	Extra     map[string]any
	Content   []Node
	Marks     []Mark
}

// MediaGroup is an ADF mediaGroup (an attachment strip).
type MediaGroup struct {
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// Media is an ADF media leaf: a file attachment (Type "file", ID) or an
// external image (Type "external", URL). Collection and OccurrenceKey
// are pointers because an empty-but-present value is meaningful.
type Media struct {
	Collection    *string
	Width         *float64
	Height        *float64
	OccurrenceKey *string
	Extra         map[string]any
	Type          string
	ID            string
	URL           string
	Alt           string
	Marks         []Mark
}

// BlockCard is an ADF blockCard smart link; Datasource carries the raw
// JQL-datasource payload verbatim when present.
type BlockCard struct {
	Datasource map[string]any
	Extra      map[string]any
	URL        string
	Marks      []Mark
}

// EmbedCard is an ADF embedCard smart link.
type EmbedCard struct {
	Width  *float64
	Extra  map[string]any
	URL    string
	Layout string
	Marks  []Mark
}

// InlineCard is an ADF inlineCard smart link; URL is a pointer because
// a card without a url attribute is dropped by the markdown projection
// (its "data" payload form rides in Extra).
type InlineCard struct {
	URL   *string
	Extra map[string]any
	Marks []Mark
}

// Text is an ADF text leaf with its flat mark array.
type Text struct {
	Extra map[string]any
	Text  string
	Marks []Mark
}

// HardBreak is an ADF hardBreak.
type HardBreak struct {
	Extra map[string]any
	Marks []Mark
}

// Emoji is an ADF emoji leaf; Text is the rendered fallback (a pointer:
// an emoji without text renders as nothing).
type Emoji struct {
	Text      *string
	Extra     map[string]any
	ShortName string
	ID        string
	Marks     []Mark
}

// Mention is an ADF mention leaf; Text is the display name (a pointer:
// a mention without text is dropped by the markdown projection).
type Mention struct {
	Text        *string
	Extra       map[string]any
	ID          string
	AccessLevel string
	Marks       []Mark
}

// Status is an ADF status lozenge.
type Status struct {
	Text    *string
	Extra   map[string]any
	Color   string
	Style   string
	LocalID string
	Marks   []Mark
}

// MediaInline is an ADF mediaInline leaf (an inline attachment chip).
type MediaInline struct {
	Collection *string
	Extra      map[string]any
	Type       string
	ID         string
	Alt        string
	Marks      []Mark
}

// ColwidthsHint is a synthetic kind (never wire): the placeholder the
// ::colwidths directive encodes to; the convert package resolves it
// onto the following table's cells and drops orphans.
type ColwidthsHint struct {
	Extra  map[string]any
	Widths []float64
	Marks  []Mark
}

// ColwidthsHintType is ColwidthsHint's Kind string (the dialect package
// re-exports it as ColwidthsPlaceholder).
const ColwidthsHintType = "__colwidths"

// RawNode is the lossless escape hatch for unknown node kinds: the
// generic type/attrs/marks/text/content shape verbatim. It is also the
// decode fallback for a known kind whose shape does not fit its typed
// struct (text on a container, content on a leaf).
type RawNode struct {
	Type    string
	Attrs   map[string]any
	Marks   []Mark
	Text    string
	Content []Node
}

// Kind implements Node.
func (*Paragraph) Kind() string { return "paragraph" }

// Kind implements Node.
func (*Heading) Kind() string { return "heading" }

// Kind implements Node.
func (*Blockquote) Kind() string { return "blockquote" }

// Kind implements Node.
func (*Rule) Kind() string { return "rule" }

// Kind implements Node.
func (*CodeBlock) Kind() string { return "codeBlock" }

// Kind implements Node.
func (*BulletList) Kind() string { return "bulletList" }

// Kind implements Node.
func (*OrderedList) Kind() string { return "orderedList" }

// Kind implements Node.
func (*ListItem) Kind() string { return "listItem" }

// Kind implements Node.
func (*TaskList) Kind() string { return "taskList" }

// Kind implements Node.
func (*TaskItem) Kind() string { return "taskItem" }

// Kind implements Node.
func (*DecisionList) Kind() string { return "decisionList" }

// Kind implements Node.
func (*DecisionItem) Kind() string { return "decisionItem" }

// Kind implements Node.
func (*Table) Kind() string { return "table" }

// Kind implements Node.
func (*TableRow) Kind() string { return "tableRow" }

// Kind implements Node.
func (*TableCell) Kind() string { return "tableCell" }

// Kind implements Node.
func (*TableHeader) Kind() string { return "tableHeader" }

// Kind implements Node.
func (*Panel) Kind() string { return "panel" }

// Kind implements Node.
func (*Expand) Kind() string { return "expand" }

// Kind implements Node.
func (*NestedExpand) Kind() string { return "nestedExpand" }

// Kind implements Node.
func (*MediaSingle) Kind() string { return "mediaSingle" }

// Kind implements Node.
func (*MediaGroup) Kind() string { return "mediaGroup" }

// Kind implements Node.
func (*Media) Kind() string { return "media" }

// Kind implements Node.
func (*BlockCard) Kind() string { return "blockCard" }

// Kind implements Node.
func (*EmbedCard) Kind() string { return "embedCard" }

// Kind implements Node.
func (*InlineCard) Kind() string { return "inlineCard" }

// Kind implements Node.
func (*Text) Kind() string { return "text" }

// Kind implements Node.
func (*HardBreak) Kind() string { return "hardBreak" }

// Kind implements Node.
func (*Emoji) Kind() string { return "emoji" }

// Kind implements Node.
func (*Mention) Kind() string { return "mention" }

// Kind implements Node.
func (*Status) Kind() string { return "status" }

// Kind implements Node.
func (*MediaInline) Kind() string { return "mediaInline" }

// Kind implements Node.
func (*ColwidthsHint) Kind() string { return ColwidthsHintType }

// Kind implements Node.
func (n *RawNode) Kind() string { return n.Type }

func (*Paragraph) adfNode()     {}
func (*Heading) adfNode()       {}
func (*Blockquote) adfNode()    {}
func (*Rule) adfNode()          {}
func (*CodeBlock) adfNode()     {}
func (*BulletList) adfNode()    {}
func (*OrderedList) adfNode()   {}
func (*ListItem) adfNode()      {}
func (*TaskList) adfNode()      {}
func (*TaskItem) adfNode()      {}
func (*DecisionList) adfNode()  {}
func (*DecisionItem) adfNode()  {}
func (*Table) adfNode()         {}
func (*TableRow) adfNode()      {}
func (*TableCell) adfNode()     {}
func (*TableHeader) adfNode()   {}
func (*Panel) adfNode()         {}
func (*Expand) adfNode()        {}
func (*NestedExpand) adfNode()  {}
func (*MediaSingle) adfNode()   {}
func (*MediaGroup) adfNode()    {}
func (*Media) adfNode()         {}
func (*BlockCard) adfNode()     {}
func (*EmbedCard) adfNode()     {}
func (*InlineCard) adfNode()    {}
func (*Text) adfNode()          {}
func (*HardBreak) adfNode()     {}
func (*Emoji) adfNode()         {}
func (*Mention) adfNode()       {}
func (*Status) adfNode()        {}
func (*MediaInline) adfNode()   {}
func (*ColwidthsHint) adfNode() {}
func (*RawNode) adfNode()       {}
