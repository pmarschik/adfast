package adf

// GFM table column alignment ⇄ the ADF alignment block mark.
//
// An ADF table has no column-alignment attribute, so a GFM delimiter row
// (":--", "--:", ":-:") rides as the synthetic never-wire attribute
// Table.Align (see IsWireSafe). ADF does have an alignment BLOCK mark
// though, the one the editor's align buttons write, and a table cell is
// one of the places it is valid: @atlaskit/adf-schema 57.1.3 spells
// table_cell_content with paragraph_with_alignment_node and
// heading_with_alignment_node among its members, and table_header_node
// takes the same content. Lowering the column attribute onto the blocks
// inside that column's cells is therefore the closest wire form the
// format offers, and it is what a reader sees in Confluence: the column
// looks aligned.
//
// The mark spells only two positions, "center" and "end". Left alignment
// is the absence of a mark, so ":--" and "--" lower to the same document
// and a left-aligned column never comes back. That loss is accepted:
// left is what an unaligned column already renders as.
//
// Both passes work on whole documents rather than one node at a time,
// because the alignment of a cell is a property of the column it sits in
// — which the cell itself does not know. Column positions come from the
// same colspan and rowspan arithmetic the ADF→AST conversion does, so a
// merged cell lands in the column a reader sees it in.
//
// LowerTableAlign runs before submission (it clears the synthetic
// attribute, so its result is wire-safe) and LiftTableAlign before
// decode, the same pair as confluence.LowerAnchors / LiftAnchors.

// The two ADF alignment mark values, and the GFM alignments they stand
// for. The GFM spellings are ast.Alignment's, quoted rather than
// imported: adf sits below ast and carries the attribute as strings.
const (
	alignMarkCenter = "center"
	alignMarkEnd    = "end"
	gfmAlignCenter  = "center"
	gfmAlignRight   = "right"
)

// LowerTableAlign gives every alignable block inside an aligned column's
// cells that column's alignment mark, and clears the table's synthetic
// Align attribute. Tables without the attribute are untouched, and the
// result is wire-safe.
//
// Only the direct block children of a cell are marked. A paragraph
// nested deeper — inside a list, say — keeps its own alignment, the way
// the editor treats it.
//
// A block that already carries an alignment mark keeps it. That mark
// came from an explicit ":::center" in the cell, which says more about
// that one block than the delimiter row says about the whole column.
func LowerTableAlign(doc Doc) Doc {
	return Transform(doc, func(n Node) ([]Node, bool) {
		t, ok := n.(*Table)
		if !ok || t.Align == nil {
			return nil, false
		}
		lowered := *t
		lowered.Align = nil
		lowered.Content = lowerAlignedRows(t.Content, t.Align)
		return []Node{&lowered}, true
	})
}

// lowerAlignedRows rewrites the rows copy-on-write, marking each cell
// with the alignment of the column it occupies.
func lowerAlignedRows(rows []Node, align []string) []Node {
	positions := tableCellColumns(rows)
	out := make([]Node, len(rows))
	for i, rowNode := range rows {
		row, isRow := rowNode.(*TableRow)
		if !isRow {
			out[i] = rowNode
			continue
		}
		cells := make([]Node, len(row.Content))
		changed := false
		for j, cell := range row.Content {
			cells[j] = cell
			mark := spannedAlignMark(align, positions[i][j])
			if mark == "" {
				continue
			}
			if marked, cellChanged := markCellBlocks(cell, mark); cellChanged {
				cells[j] = marked
				changed = true
			}
		}
		if !changed {
			out[i] = rowNode
			continue
		}
		out[i] = WithContent(row, cells)
	}
	return out
}

// spannedAlignMark answers the mark a cell at pos gets: the alignment of
// its column, or — for a cell that spans several columns — the one
// alignment all of them ask for. Columns that disagree leave the cell
// unmarked, because a merged cell has one alignment and they name two.
func spannedAlignMark(align []string, pos cellPos) string {
	mark := alignMark(columnAlign(align, pos.col))
	for c := pos.col + 1; c < pos.col+pos.span; c++ {
		if alignMark(columnAlign(align, c)) != mark {
			return ""
		}
	}
	return mark
}

// columnAlign reads column ci out of the attribute, which is allowed to
// be shorter than the table is wide.
func columnAlign(align []string, ci int) string {
	if ci < 0 || ci >= len(align) {
		return ""
	}
	return align[ci]
}

// alignMark maps a GFM column alignment to the ADF mark value, empty for
// the two the mark cannot spell (left, and no alignment at all).
func alignMark(gfm string) string {
	switch gfm {
	case gfmAlignCenter:
		return alignMarkCenter
	case gfmAlignRight:
		return alignMarkEnd
	}
	return ""
}

// gfmAlign is the inverse of alignMark, empty for an unknown value.
func gfmAlign(mark string) string {
	switch mark {
	case alignMarkCenter:
		return gfmAlignCenter
	case alignMarkEnd:
		return gfmAlignRight
	}
	return ""
}

// markCellBlocks adds the alignment mark to the cell's direct alignable
// blocks, reporting whether anything changed.
func markCellBlocks(cell Node, mark string) (Node, bool) {
	blocks := NodeContent(cell)
	if len(blocks) == 0 {
		return cell, false
	}
	out := make([]Node, len(blocks))
	changed := false
	for i, block := range blocks {
		out[i] = block
		if !alignable(block) || blockAlignMark(block) != "" {
			continue
		}
		out[i] = withMarks(block, append(append([]Mark{}, NodeMarks(block)...), &Alignment{Align: mark}))
		changed = true
	}
	if !changed {
		return cell, false
	}
	return WithContent(cell, out), true
}

// LiftTableAlign is the inverse: a column whose blocks carry an
// alignment mark gives that alignment to the table's synthetic Align
// attribute, so the decode renders a GFM delimiter row, and the blocks
// that said it lose the mark. Tables that already carry the attribute
// are untouched.
//
// A column whose blocks disagree takes the alignment most of them carry,
// and the blocks out of line keep their own mark. A delimiter row has
// one alignment per column and no way to say "except this cell", so the
// majority is the reading that matches the most cells; leaving the rest
// marked keeps them aligned in the product. Blocks carrying no mark
// count as a vote too, which is why a column of plain text with one
// centered cell stays unaligned.
//
// A cell spanning several columns is neither evidence nor an obstacle.
// It sits in no single column, so it cannot vote, and its mark stays on
// it. Re-lowering the lifted table puts that mark back where it was.
func LiftTableAlign(doc Doc) Doc {
	return Transform(doc, func(n Node) ([]Node, bool) {
		t, ok := n.(*Table)
		if !ok || t.Align != nil {
			return nil, false
		}
		align, ok := liftedColumns(t.Content)
		if !ok {
			return nil, false
		}
		lifted := *t
		lifted.Align = align
		lifted.Content = unmarkAlignedRows(t.Content, align)
		return []Node{&lifted}, true
	})
}

// columnTally counts what the blocks of one column say. The answers are
// kept in the order they first appear, so a tie breaks the same way on
// every run: the alignment the reader met first wins.
type columnTally struct {
	marks  []string
	counts []int
}

// vote records one block's alignment, the empty string included.
func (t *columnTally) vote(mark string) {
	for i, m := range t.marks {
		if m == mark {
			t.counts[i]++
			return
		}
	}
	t.marks = append(t.marks, mark)
	t.counts = append(t.counts, 1)
}

// winner answers the alignment most of the column's blocks carry.
func (t *columnTally) winner() string {
	best, bestCount := "", 0
	for i, m := range t.marks {
		if t.counts[i] > bestCount {
			best, bestCount = m, t.counts[i]
		}
	}
	return best
}

// liftedColumns reads a per-column alignment out of the marks, answering
// false when no column earns one.
//
// Empty blocks abstain: the editor puts no mark on an empty paragraph
// either, so a column of aligned text with one blank gap in it must not
// read as a column that disagrees with itself.
func liftedColumns(rows []Node) ([]string, bool) {
	positions := tableCellColumns(rows)
	var tallies []columnTally
	for i, rowNode := range rows {
		row, isRow := rowNode.(*TableRow)
		if !isRow {
			continue
		}
		for j, cell := range row.Content {
			pos := positions[i][j]
			if pos.span != 1 {
				continue
			}
			for pos.col >= len(tallies) {
				tallies = append(tallies, columnTally{})
			}
			for _, block := range NodeContent(cell) {
				if !alignable(block) || len(NodeContent(block)) == 0 {
					continue
				}
				tallies[pos.col].vote(blockAlignMark(block))
			}
		}
	}
	align := make([]string, len(tallies))
	aligned := false
	for i := range tallies {
		if gfm := gfmAlign(tallies[i].winner()); gfm != "" {
			align[i] = gfm
			aligned = true
		}
	}
	if !aligned {
		return nil, false
	}
	return align, true
}

// unmarkAlignedRows removes the alignment mark from the cells of every
// lifted column, leaving the columns that declined untouched.
func unmarkAlignedRows(rows []Node, align []string) []Node {
	positions := tableCellColumns(rows)
	out := make([]Node, len(rows))
	for i, rowNode := range rows {
		row, isRow := rowNode.(*TableRow)
		if !isRow {
			out[i] = rowNode
			continue
		}
		cells := make([]Node, len(row.Content))
		changed := false
		for j, cell := range row.Content {
			cells[j] = cell
			pos := positions[i][j]
			won := alignMark(columnAlign(align, pos.col))
			if pos.span != 1 || won == "" {
				continue
			}
			if bare, cellChanged := unmarkCellBlocks(cell, won); cellChanged {
				cells[j] = bare
				changed = true
			}
		}
		if !changed {
			out[i] = rowNode
			continue
		}
		out[i] = WithContent(row, cells)
	}
	return out
}

// unmarkCellBlocks drops the mark from the cell's direct alignable
// blocks that carry exactly the alignment won, reporting whether
// anything changed. A block carrying a different alignment keeps it: the
// delimiter row does not speak for that block, so nothing about it may
// be thrown away.
func unmarkCellBlocks(cell Node, won string) (Node, bool) {
	blocks := NodeContent(cell)
	out := make([]Node, len(blocks))
	changed := false
	for i, block := range blocks {
		out[i] = block
		if !alignable(block) || blockAlignMark(block) != won {
			continue
		}
		marks := NodeMarks(block)
		kept := make([]Mark, 0, len(marks))
		for _, m := range marks {
			if _, isAlign := m.(*Alignment); !isAlign {
				kept = append(kept, m)
			}
		}
		if len(kept) == 0 {
			kept = nil
		}
		out[i] = withMarks(block, kept)
		changed = true
	}
	if !changed {
		return cell, false
	}
	return WithContent(cell, out), true
}

// alignable reports whether the alignment mark is valid on the node. The
// ADF schema puts it on paragraphs and headings only.
func alignable(n Node) bool {
	switch n.(type) {
	case *Paragraph, *Heading:
		return true
	}
	return false
}

// blockAlignMark answers the node's alignment mark value, empty when it
// carries none.
func blockAlignMark(n Node) string {
	for _, m := range NodeMarks(n) {
		if a, ok := m.(*Alignment); ok {
			return a.Align
		}
	}
	return ""
}

// withMarks returns a shallow copy of n with its marks replaced; kinds
// without a marks slot return n unchanged.
func withMarks(n Node, marks []Mark) Node {
	if n.slots().marks == nil {
		return n
	}
	c := n.shallowCopy()
	*c.slots().marks = marks
	return c
}

// cellPos is where a table cell sits: the visual column it starts at,
// and how many columns it covers.
type cellPos struct {
	col  int
	span int
}

// tableCellColumns maps every cell of every row to its visual position,
// one entry per row in row order. Cells carried over by a rowspan from
// an earlier row occupy their columns without appearing again, so a cell
// after them starts past the columns they hold — the arithmetic a reader
// does by eye, and the one the ADF→AST conversion does.
func tableCellColumns(rows []Node) [][]cellPos {
	// covered[c] counts the rows column c is still held for, this row
	// included; it is aged by one at the end of each row.
	var covered []int
	out := make([][]cellPos, len(rows))
	for i, rowNode := range rows {
		row, isRow := rowNode.(*TableRow)
		if !isRow {
			continue
		}
		positions := make([]cellPos, len(row.Content))
		col := 0
		for j, cell := range row.Content {
			for col < len(covered) && covered[col] > 0 {
				col++
			}
			colspan, rowspan := cellSpans(cell)
			positions[j] = cellPos{col: col, span: colspan}
			for c := col; c < col+colspan; c++ {
				for c >= len(covered) {
					covered = append(covered, 0)
				}
				covered[c] = rowspan
			}
			col += colspan
		}
		out[i] = positions
		for c := range covered {
			if covered[c] > 0 {
				covered[c]--
			}
		}
	}
	return out
}

// cellSpans answers a cell's colspan and rowspan, both at least 1.
func cellSpans(cell Node) (colspan, rowspan int) {
	switch c := cell.(type) {
	case *TableCell:
		return max(c.Colspan, 1), max(c.Rowspan, 1)
	case *TableHeader:
		return max(c.Colspan, 1), max(c.Rowspan, 1)
	case *RawNode:
		return max(IntAttr(c.Attrs, "colspan", 1), 1), max(IntAttr(c.Attrs, "rowspan", 1), 1)
	}
	return 1, 1
}
