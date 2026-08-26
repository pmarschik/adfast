// Package ast defines the pivot Markdown AST both conversion directions
// share: a minimal tree mirroring the node shapes of remark's mdast. It
// is the intermediate representation between ADF documents and Markdown
// text: ToMarkdown = ADF → AST → render, FromMarkdown = parse → goldmark
// AST → AST → ADF. All Markdown string quirks (escaping, wrapping, bullet
// choice, list tightness) live in the renderer (markdown); all ADF quirks
// (flat marks, inline cards, task lists) live in the two ADF-side
// transforms (convert).
//
// The tree is typed: one concrete Go type per node kind, each implementing
// the Node interface. Nodes are always constructed and passed as pointers
// (&Paragraph{…}); Kind returns the stable kind name matching remark's AST
// vocabulary ("paragraph", "text", …). The interface is open — consumers
// define their own node kinds through the extension package's contract
// (see extension.Node), and the dialect package implements the known
// dialect on it.
//
// Container kinds carry their child list in a Children field; the package
// helpers Children and SetChildren provide the generic access used by
// kind-agnostic walks (foreign container kinds hook in via the Parent
// interface). Block kinds that record source blank-line structure
// embed BlockSpacing; GapBefore and SetGapBefore are the generic
// accessors.
//
// The audience is the markdown and convert subpackages plus advanced
// consumers assembling custom pipelines; most users should stay on the
// root adfast facade. The surface tracks those packages and may grow.
package ast

import "strings"

// Node is a single node in the Markdown AST: an open interface over the
// concrete node kinds in this package (consumers may add their own).
type Node interface {
	// Kind returns the stable kind name, matching remark's AST vocabulary
	// ("paragraph", "text", …); it doubles as the debug label.
	Kind() string
}

// Parent is the child-access contract of every container kind: the
// kind-agnostic Children and SetChildren helpers — and every walk built
// on them — reach a subtree through it and nothing else. The container
// kinds in this package implement it in parent.go; a foreign kind (see
// the extension package) joins the walks by implementing it too.
type Parent interface {
	Node
	// ChildNodes returns the node's child list.
	ChildNodes() []Node
	// SetChildNodes replaces the node's child list.
	SetChildNodes([]Node)
}

// BlockSpacing records the source blank-line structure around a block. It
// is embedded in the block kinds that can appear as siblings in a block
// sequence; use GapBefore/SetGapBefore for kind-agnostic access.
type BlockSpacing struct {
	// GapBefore marks a block that was separated from its previous sibling
	// by a blank line in the source (drives within-item joins).
	GapBefore bool
}

// Spacing returns the embedded spacing; every block kind embedding
// BlockSpacing satisfies the Spaced interface through it.
func (s *BlockSpacing) Spacing() *BlockSpacing { return s }

// Spaced is implemented (via the BlockSpacing embed) by the block kinds
// that record source blank-line structure.
type Spaced interface {
	Spacing() *BlockSpacing
}

// GapBefore reports whether the block was separated from its previous
// sibling by a blank line in the source; false for kinds without spacing.
func GapBefore(n Node) bool {
	if s, ok := n.(Spaced); ok {
		return s.Spacing().GapBefore
	}
	return false
}

// SetGapBefore sets the blank-line-before flag; a no-op for kinds without
// spacing.
func SetGapBefore(n Node, v bool) {
	if s, ok := n.(Spaced); ok {
		s.Spacing().GapBefore = v
	}
}

// ---------------------------------------------------------------------------
// Block kinds
// ---------------------------------------------------------------------------

// Root is the document root.
type Root struct {
	Children []Node
}

// Kind implements Node.
func (*Root) Kind() string { return "root" }

// Paragraph is a paragraph of phrasing content.
type Paragraph struct {
	Children []Node
	BlockSpacing
	// DirectiveLabel marks a paragraph that holds a container directive's
	// [label] (remark's data.directiveLabel). For :::expand it carries the
	// ADF title; panel labels stay plain paragraphs.
	DirectiveLabel bool
}

// Kind implements Node.
func (*Paragraph) Kind() string { return "paragraph" }

// Heading is an ATX/setext heading.
type Heading struct {
	ID       string
	Children []Node
	Depth    int
	BlockSpacing
}

// Kind implements Node.
func (*Heading) Kind() string { return "heading" }

// ThematicBreak is a horizontal rule.
type ThematicBreak struct {
	BlockSpacing
}

// Kind implements Node.
func (*ThematicBreak) Kind() string { return "thematicBreak" }

// Blockquote is a block quote.
type Blockquote struct {
	Children []Node
	BlockSpacing
}

// Kind implements Node.
func (*Blockquote) Kind() string { return "blockquote" }

// List is a bullet, ordered, or task list.
type List struct {
	Children []Node
	// Start is the ordered list start (attrs.order in ADF).
	Start int
	// OrderedGap is the source gap (spaces) after the first item's ordered
	// marker, clamped to 1..2; prettier aligns all items' content to
	// firstMarkerWidth+gap. Zero means default (1).
	OrderedGap int
	BlockSpacing
	Ordered bool
	// PerItemSpread marks a goldmark-sourced list whose item Spread/GapAfter
	// flags carry the source blank-line structure; the renderer then ignores
	// the list-level Spread (which holds CommonMark whole-list looseness for
	// the ADF tight-attribute flow).
	PerItemSpread bool
	// Increment renumbers ordered items start+i instead of repeating start.
	// Set only by the goldmark path when the source list increments
	// (prettier's git-diff-friendly rule); ADF-derived lists repeat start.
	Increment bool
	Spread    bool // loose list: blank lines between items
}

// Kind implements Node.
func (*List) Kind() string { return "list" }

// ListItem is a single list item.
type ListItem struct {
	// Checked distinguishes task list items: nil = plain list item,
	// otherwise the ADF taskItem state (true = DONE).
	Checked  *bool
	Children []Node
	Spread   bool // blank lines between the item's own blocks
	// GapAfter marks a list item that was separated from its successor by a
	// blank line in the source (per-item looseness, like AST spread).
	GapAfter bool
}

// Kind implements Node.
func (*ListItem) Kind() string { return "listItem" }

// Code is a fenced or indented code block.
type Code struct {
	// Lang is the info string.
	Lang string
	// Value holds the code text.
	Value string
	BlockSpacing
}

// Kind implements Node.
func (*Code) Kind() string { return "code" }

// HTML is raw HTML — a block in flow position, an inline span in phrasing
// position.
type HTML struct {
	// Value holds the raw HTML text.
	Value string
	BlockSpacing
}

// Kind implements Node.
func (*HTML) Kind() string { return "html" }

// Frontmatter is a leading metadata block (YAML frontmatter or a custom
// FrontmatterProvider's raw header), kept verbatim.
type Frontmatter struct {
	// Value holds the raw metadata block, delimiters included.
	Value string
	BlockSpacing
}

// Kind implements Node.
func (*Frontmatter) Kind() string { return "yaml" }

// Table is a GFM table.
type Table struct {
	Children []Node
	// Align is the per-visual-column alignment the delimiter row spells,
	// leftmost column first (mdast's table `align`). It is nil when no
	// column asks for one, and may be shorter than the table is wide —
	// read it with ColumnAlign. ADF tables have no alignment attribute,
	// so this rides the ADF leg as a synthetic never-wire carrier
	// (adf.Table.Align; see adf.IsWireSafe), which the product bundles
	// lower onto the alignment block mark of each column's blocks before
	// submission (adf.LowerTableAlign).
	Align []Alignment
	BlockSpacing
}

// Kind implements Node.
func (*Table) Kind() string { return "table" }

// TableRow is a table row.
type TableRow struct {
	Children []Node
}

// Kind implements Node.
func (*TableRow) Kind() string { return "tableRow" }

// TableCell is a table cell.
type TableCell struct {
	Children []Node
	// ColSpan/RowSpan are table-cell merge spans (>1 when the cell covers
	// multiple visual columns/rows; remark-extended-table syntax).
	ColSpan int
	RowSpan int
}

// Kind implements Node.
func (*TableCell) Kind() string { return "tableCell" }

// ContainerDirective is a :::name fenced container directive.
// The directive label is not a field: remark represents container labels
// as the first child paragraph (see Paragraph.DirectiveLabel).
type ContainerDirective struct {
	Name     string
	Attrs    map[string]string
	Children []Node
	BlockSpacing
}

// Kind implements Node.
func (*ContainerDirective) Kind() string { return "containerDirective" }

// LeafDirective is a ::name[label]{attrs} leaf directive; the label is the
// inline children.
type LeafDirective struct {
	Name     string
	Attrs    map[string]string
	Children []Node
	BlockSpacing
}

// Kind implements Node.
func (*LeafDirective) Kind() string { return "leafDirective" }

// ---------------------------------------------------------------------------
// Inline kinds
// ---------------------------------------------------------------------------

// Text is a plain text span.
type Text struct {
	// Value holds the decoded (semantic) text content — the currency ADF
	// consumes. CommonMark backslash escapes and character references are
	// resolved (`\+` → "+", `&#x61;` → "a").
	Value string
	// Raw holds the escape-preserving source form used only by the prettier
	// md→md formatter: it keeps prettier's literal escapes (see the markdown
	// package's PreservedEscapes) undecoded so formatting re-emits them
	// byte-for-byte, while Value stays fully decoded. A Markdown parse sets
	// Raw only when it differs from Value; nodes built from ADF or by hand
	// leave it empty, and Rendered then falls back to Value.
	Raw string
}

// Kind implements Node.
func (*Text) Kind() string { return "text" }

// Rendered returns the text the formatter serializes: the escape-preserving
// source form (Raw) when a Markdown parse captured it, else the decoded
// Value. The plain (non-prettier) render and the ADF encode read Value
// directly; only the prettier formatter's canonicalization consults this.
func (t *Text) Rendered() string {
	if t.Raw != "" {
		return t.Raw
	}
	return t.Value
}

// Emphasis is emphasized (italic) phrasing content.
type Emphasis struct {
	Children []Node
}

// Kind implements Node.
func (*Emphasis) Kind() string { return "emphasis" }

// Strong is strongly emphasized (bold) phrasing content.
type Strong struct {
	Children []Node
}

// Kind implements Node.
func (*Strong) Kind() string { return "strong" }

// Delete is struck-through (GFM) phrasing content.
type Delete struct {
	Children []Node
}

// Kind implements Node.
func (*Delete) Kind() string { return "delete" }

// InlineCode is an inline code span.
type InlineCode struct {
	// Value holds the code text.
	Value string
}

// Kind implements Node.
func (*InlineCode) Kind() string { return "inlineCode" }

// Break is a hard line break.
type Break struct {
	// Value preserves the source break style: "  " for the trailing-space
	// form, empty for the backslash form.
	Value string
}

// Kind implements Node.
func (*Break) Kind() string { return "break" }

// Link is a hyperlink; the label is the inline children.
type Link struct {
	// URL is the destination.
	URL string
	// Title is the optional link title ([x](url "title")).
	Title    string
	Children []Node
	// Bare marks an autolink written without angle brackets in the
	// source (linkified). The renderer preserves that form like prettier.
	Bare bool
	// Explicit marks a source [label](url) resource link, which never
	// collapses to the <url> shortcut even when label == url.
	Explicit bool
	// InlineCard marks a link node produced from an ADF inlineCard, so
	// that the AST→ADF conversion can restore the inlineCard without
	// re-deriving it.
	InlineCard bool
}

// Kind implements Node.
func (*Link) Kind() string { return "link" }

// Image is an image; the alt text is the inline children.
type Image struct {
	// URL is the destination.
	URL string
	// Title is the optional image title (![x](url "title")).
	Title    string
	Children []Node
}

// Kind implements Node.
func (*Image) Kind() string { return "image" }

// TextDirective is a :name[label]{attrs} inline directive; the label is
// the inline children.
type TextDirective struct {
	Name     string
	Attrs    map[string]string
	Children []Node
}

// Kind implements Node.
func (*TextDirective) Kind() string { return "textDirective" }

// ---------------------------------------------------------------------------
// Generic access
// ---------------------------------------------------------------------------

// Children returns n's child list; nil for leaf kinds and for foreign
// nodes that do not implement Parent.
func Children(n Node) []Node {
	if p, ok := n.(Parent); ok {
		return p.ChildNodes()
	}
	return nil
}

// SetChildren replaces n's child list; a no-op for leaf kinds and for
// foreign nodes that do not implement Parent.
func SetChildren(n Node, kids []Node) {
	if p, ok := n.(Parent); ok {
		p.SetChildNodes(kids)
	}
}

// PlainText concatenates the text content of inline AST nodes.
//
// An unregistered TextDirective (one with no typed dialect node, e.g. a
// stray ":name" or an intraword "word:name") gets the same colon-prefixed
// literal reconstruction the two other renderers already give it —
// flattenTextDirective on the ADF leg, normalize.go's md formatter on the
// markdown leg. Without this case PlainText fell into the default branch
// below and silently dropped the ":name" text, which truncated an image
// alt ("![Over:view](x.png)" read back as "Over") and made the colon
// invisible to any lint that reads PlainText as "what the reader sees".
// LeafDirective and ContainerDirective need no case here: both are
// block-level constructs the parser only ever builds while walking block
// children (markdown/goldmark_to_ast.go's convertStructuredBlock); neither
// is reachable from convertGoldmarkInline, so neither can occupy a slot in
// an inline Children slice that PlainText walks.
func PlainText(nodes []Node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch v := n.(type) {
		case *Text:
			b.WriteString(v.Value)
		case *InlineCode:
			b.WriteString(v.Value)
		case *TextDirective:
			b.WriteString(":" + v.Name)
			b.WriteString(PlainText(v.Children))
		default:
			b.WriteString(PlainText(Children(n)))
		}
	}
	return b.String()
}
