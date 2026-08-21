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
	type carry struct {
		rowsLeft int
		width    int
	}
	pending := map[int]carry{}
	visual = make([][]string, len(rows))
	for ri := range rows {
		var cells []string
		col := 0
		drain := func() {
			for {
				c, ok := pending[col]
				if !ok {
					break
				}
				start := col
				for range c.width {
					cells = append(cells, "^")
					col++
				}
				if c.rowsLeft > 1 {
					pending[start] = carry{rowsLeft: c.rowsLeft - 1, width: c.width}
				} else {
					delete(pending, start)
				}
			}
		}
		for _, rowChild := range ast.Children(rows[ri]) {
			cell, ok := rowChild.(*ast.TableCell)
			if !ok {
				continue
			}
			drain()
			content := r.renderCellString(cell.Children)
			if content == ">" || content == "^" {
				content = `\` + content
			}
			content = bareDelimiterAmbiguousCell(content)
			span := max(cell.ColSpan, 1)
			start := col
			for range span - 1 {
				cells = append(cells, ">")
				col++
			}
			cells = append(cells, content)
			col++
			if cell.RowSpan > 1 {
				pending[start] = carry{rowsLeft: cell.RowSpan - 1, width: span}
			}
		}
		drain()
		visual[ri] = cells
		colCount = max(colCount, col)
	}
	return visual, colCount
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
