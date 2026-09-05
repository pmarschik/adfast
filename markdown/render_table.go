package markdown

import (
	"strings"

	"github.com/pmarschik/adfast/ast"
)

// Table rendering: GFM table serialization with remark-extended-table
// span markers and mdast-util-gfm-table column padding. Split from
// render.go.

func (r *mdRenderer) renderTable(b *strings.Builder, node *ast.Table) {
	rows := node.Children
	if len(rows) == 0 {
		return
	}
	// Expand merged cells to visual columns (remark-extended-table: ">"
	// markers precede a colspan cell, "^" continues a rowspan), then pad
	// to per-column width like mdast-util-gfm-table (alignDelimiters).
	rendered, colCount := r.expandTableSpans(rows)
	widths := make([]int, colCount)
	for ri := range rendered {
		for len(rendered[ri]) < colCount {
			rendered[ri] = append(rendered[ri], "")
		}
		for ci := range colCount {
			widths[ci] = max(widths[ci], utf16Length(rendered[ri][ci]))
		}
	}
	delimiters := tableDelimiterRow(node.Align, widths)

	writeRow := func(cells []string) {
		b.WriteString("|")
		for ci, cell := range cells {
			lead, trail := tableCellPad(ast.ColumnAlign(node.Align, ci), widths[ci]-utf16Length(cell))
			b.WriteString(" ")
			b.WriteString(strings.Repeat(" ", lead))
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", trail))
			b.WriteString(" |")
		}
		b.WriteString("\n")
	}

	writeRow(rendered[0])
	writeRow(delimiters)
	for _, cells := range rendered[1:] {
		writeRow(cells)
	}
}

// tableDelimiterRow writes the delimiter cells and widens the columns the
// way markdown-table does (the table serializer behind
// mdast-util-gfm-table, measured): a cell is the column's colons around at
// least one '-', filling the column width when it can. A column narrower
// than its own colons therefore grows to fit them — ":-:" widens a
// one-character centered column to three — which is why widths is updated
// in place here, before any row is padded.
func tableDelimiterRow(align []ast.Alignment, widths []int) []string {
	cells := make([]string, len(widths))
	for ci := range widths {
		var before, after string
		switch ast.ColumnAlign(align, ci) {
		case ast.AlignLeft:
			before = ":"
		case ast.AlignRight:
			after = ":"
		case ast.AlignCenter:
			before, after = ":", ":"
		case ast.AlignNone:
		}
		dashes := max(1, widths[ci]-len(before)-len(after))
		cells[ci] = before + strings.Repeat("-", dashes) + after
		widths[ci] = max(widths[ci], len(cells[ci]))
	}
	return cells
}

// tableCellPad splits a cell's padding into leading and trailing spaces
// for its column's alignment (markdown-table, measured): a right-aligned
// cell takes all of it in front, a centered one splits it with the odd
// space in front, and every other one trails it.
func tableCellPad(align ast.Alignment, pad int) (lead, trail int) {
	switch align {
	case ast.AlignRight:
		return pad, 0
	case ast.AlignCenter:
		lead = (pad + 1) / 2
		return lead, pad - lead
	case ast.AlignLeft, ast.AlignNone:
	}
	return 0, pad
}

// expandTableSpans renders every row's cells into visual columns:
// a colspan-N cell becomes N-1 ">" markers followed by its content, a
// rowspan continues as "^" markers in the following rows, and literal
// ">"/"^" cell texts are escaped so they don't read as markers.
func (r *mdRenderer) expandTableSpans(rows []ast.Node) (visual [][]string, colCount int) {
	ex := &spanExpander{pending: map[int]rowspanRun{}}
	visual = make([][]string, len(rows))
	for ri := range rows {
		ex.startRow()
		for _, rowChild := range ast.Children(rows[ri]) {
			cell, ok := rowChild.(*ast.TableCell)
			if !ok {
				continue
			}
			ex.drain()
			ex.place(r.renderCellString(cellContent(cell.Children)), cell)
		}
		ex.drain()
		visual[ri] = ex.cells
		colCount = max(colCount, ex.col)
	}
	return visual, colCount
}

// cellContent projects a table cell's children onto the nodes that can be
// written on one table line, flattening every block wrapper it meets.
//
// A cell in the pivot AST may hold blocks: the ADF model puts blocks in a
// tableCell, and a caller may build one by hand. A table row is one line,
// so the block's own markers cannot be written — but its CONTENT must
// survive, and before this projection some of it did not. Measured on the
// bare AST (Render of a two-row table whose body cell holds the node):
// an ast.Code cell rendered "|   |" and an ast.Frontmatter cell rendered
// "|   |" — the text was gone, because both keep their text in a Value
// field rather than in Children, and the inline fallback that degrades a
// block in inline position writes only children.
//
// The markers are dropped deliberately, not by omission. The reference
// serializer (mdast-util-to-markdown with mdast-util-gfm-table, the
// frozen dependency tree) hands each block child to its own block
// handler and concatenates the results, so it does write them — and for
// any block that spans lines the result is no longer a table. Measured
// on the same two-row shape: an ast.Code cell serializes to
// "| H           |\n| ----------- |\n| ```go\nA\n``` |\n", whose third
// row is three lines and re-parses as a broken table followed by a
// paragraph. Keeping the table intact is the one invariant this layer
// can hold, so the cell keeps the content and gives up the markers.
//
// The kinds this recurses into are exactly the ones the inline write
// visitor degrades by writing their children, so flattening them here
// reproduces what the cell already rendered; the switch adds only the
// kinds that carried their text somewhere the fallback never looked.
//
// That equivalence was measured, one kind at a time: with every kind but
// blockquote removed from the recursion list the whole suite stayed
// green, because the inline writer's own fallback reaches the same
// children and emits the same bytes. The list therefore earns its keep
// in one situation only — a rewritten leaf nested below one of these
// wrappers, which the fallback would reach but the rewrite would not.
// Each entry is pinned on exactly that shape by
// TestTableCell_EveryBlockWrapperIsReachedThrough, so dropping any one
// of them now turns its case red; do not trim the list on the strength
// of a green run without checking that test.
func cellContent(children []ast.Node) []ast.Node {
	var out []ast.Node
	for _, child := range children {
		out = appendCellContent(out, child)
	}
	return out
}

// appendCellContent appends node's cell projection to out.
func appendCellContent(out []ast.Node, node ast.Node) []ast.Node {
	switch n := node.(type) {
	case *ast.Code:
		// The fence and the language need their own lines, so neither
		// fits a table row; an inline code span is the nearest form that
		// does, and it keeps both the text and the code-ness. Re-parsing
		// it yields the same span, so the cell is a render fixpoint.
		if n.Value == "" {
			return out
		}
		return append(out, &ast.InlineCode{Value: n.Value})
	case *ast.Frontmatter:
		// Metadata inside a cell is not something a parse produces, but
		// a hand-built tree can hold it and its text must not vanish.
		if n.Value == "" {
			return out
		}
		return append(out, &ast.InlineCode{Value: n.Value})
	case *ast.HTML:
		// Raw HTML stays raw — the reference writes it verbatim and so
		// does the cell, and an inline span is the common case here
		// because a parse splits inline HTML into tag-only nodes. But
		// verbatim is not automatically safe: a value carrying a newline
		// or a bare pipe breaks the row it sits in. See cellSafeHTML.
		safe := cellSafeHTML(n.Value)
		if safe == n.Value {
			return append(out, node)
		}
		return append(out, &ast.HTML{Value: safe})
	case *ast.ThematicBreak:
		// The one block with no content at all: there is nothing for the
		// cell to lose. The reference writes "***", which inside a cell
		// re-parses as literal text rather than as a rule, so it carries
		// no information either.
		return out
	case *ast.Root, *ast.Paragraph, *ast.Heading, *ast.Blockquote, *ast.List,
		*ast.ListItem, *ast.Table, *ast.TableRow, *ast.TableCell,
		*ast.ContainerDirective, *ast.LeafDirective:
		for _, kid := range ast.Children(node) {
			out = appendCellContent(out, kid)
		}
		return out
	}
	return append(out, node)
}

// cellSafeHTML makes a raw HTML value writable on one table line: every
// newline becomes a space, and a bare pipe is escaped.
//
// Both rewrites are unreachable from a parse, which is why they cannot
// cost a round trip. A cell is one line, so a parsed HTML value never
// holds a newline; and a pipe written inside a cell's HTML keeps its
// backslash in the value itself — parsing "| <a title=\"a\\|b\">x</a> |"
// yields the html node `<a title="a\|b">`, already escaped, which this
// leaves alone. A bare pipe therefore only reaches here from a
// hand-built tree or an ADF conversion, where before this it split the
// row into two columns: "<b>a|b</b>" rendered "| <b>a|b</b> |".
//
// The reference does neither. Measured on a two-row table whose body
// cell holds the node (mdast-util-to-markdown with mdast-util-gfm-table),
// an html value of "<div>\nA\n</div>" serializes to a third row of three
// lines and "<b>a|b</b>" to a row of three columns — its unsafe patterns
// guard text serialization only, and raw HTML skips them. That is the
// same table-destroying output this layer already refuses to copy for a
// code block. The space is the reference's own answer to an end of line
// that the enclosing construct cannot hold: a break inside a table cell
// serializes to " " rather than to "\\\n".
func cellSafeHTML(s string) string {
	if !strings.ContainsAny(s, "|\n\r") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	backslashes := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\r':
			// A CRLF is one end of line, so it collapses to one space.
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			b.WriteByte(' ')
			backslashes = 0
		case '\n':
			b.WriteByte(' ')
			backslashes = 0
		case '|':
			// An odd run of backslashes already escapes this pipe.
			if backslashes%2 == 0 {
				b.WriteByte('\\')
			}
			b.WriteByte(c)
			backslashes = 0
		case '\\':
			backslashes++
			b.WriteByte(c)
		default:
			backslashes = 0
			b.WriteByte(c)
		}
	}
	return b.String()
}

// spanExpander is expandTableSpans' running state: the rowspan
// continuations owed to the rows below (keyed by the visual column each
// starts at) plus the cursor into the row being built.
type spanExpander struct {
	pending map[int]rowspanRun
	cells   []string
	col     int
}

// rowspanRun is one rowspan still owed to the rows below: how many more rows
// it covers, and how many visual columns wide it is.
type rowspanRun struct {
	rowsLeft int
	width    int
}

// startRow resets the per-row cursor.
func (ex *spanExpander) startRow() {
	ex.cells, ex.col = nil, 0
}

// drain emits the "^" continuation markers owed at the current column,
// advancing past every rowspan that reaches it.
func (ex *spanExpander) drain() {
	for {
		run, ok := ex.pending[ex.col]
		if !ok {
			return
		}
		start := ex.col
		for range run.width {
			ex.cells = append(ex.cells, "^")
			ex.col++
		}
		if run.rowsLeft > 1 {
			ex.pending[start] = rowspanRun{rowsLeft: run.rowsLeft - 1, width: run.width}
		} else {
			delete(ex.pending, start)
		}
	}
}

// place emits one cell: the ">" markers its colspan needs, the content
// itself, and the rowspan continuation it owes the rows below.
func (ex *spanExpander) place(content string, cell *ast.TableCell) {
	if content == ">" || content == "^" {
		content = `\` + content
	}
	content = bareDelimiterAmbiguousCell(content)
	span := max(cell.ColSpan, 1)
	start := ex.col
	for range span - 1 {
		ex.cells = append(ex.cells, ">")
		ex.col++
	}
	ex.cells = append(ex.cells, content)
	ex.col++
	if cell.RowSpan > 1 {
		ex.pending[start] = rowspanRun{rowsLeft: cell.RowSpan - 1, width: span}
	}
}

// bareDelimiterAmbiguousCell strips the line-start backslash that
// escapeText adds to a cell whose content is only dashes and colons
// (e.g. "-" → "\-", "--" → "\--"). A delimiter row is always emitted
// bare ("| - |"), so a body/header cell holding the same dash/colon run
// must render bare too: otherwise the two disagree, and a header-only
// table's delimiter row that re-parses as a body cell after two adjacent
// tables merge (GFM concatenates the blocks with no blank line between
// them) would re-render escaped and widened — a round-trip that never
// reaches a fixpoint. Rendered bare, the cell matches the delimiter form
// and re-parses to the same literal text, so the render is idempotent on
// the first pass. Only a leading "\-" run is affected; colon-led runs
// (":-", ":-:") already render bare, and "-x"/"\- " keep their escape.
func bareDelimiterAmbiguousCell(content string) string {
	if !strings.HasPrefix(content, `\-`) {
		return content
	}
	rest := content[1:]
	for i := range len(rest) {
		if rest[i] != '-' && rest[i] != ':' {
			return content
		}
	}
	return rest
}

// utf16Length measures a string in UTF-16 code units — the unit
// mdast-util-gfm-table pads table columns with (JS string .length).
func utf16Length(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xFFFF {
			n++
		}
	}
	return n
}
