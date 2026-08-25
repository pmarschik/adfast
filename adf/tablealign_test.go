package adf

import "testing"

// cellOf builds a body cell holding one paragraph of text.
func cellOf(text string) *TableCell {
	return &TableCell{Content: []Node{&Paragraph{Content: []Node{&Text{Text: text}}}}}
}

// markedCell builds a body cell whose one paragraph already carries the
// alignment mark.
func markedCell(text, align string) *TableCell {
	return &TableCell{Content: []Node{&Paragraph{
		Marks:   []Mark{&Alignment{Align: align}},
		Content: []Node{&Text{Text: text}},
	}}}
}

// rowOf wraps cells in a table row.
func rowOf(cells ...Node) *TableRow { return &TableRow{Content: cells} }

// alignTable builds a table with the given column alignment.
func alignTable(align []string, rows ...Node) *Table {
	return &Table{Align: align, Content: rows}
}

// cellMark reads the alignment mark off the first block of the cell at
// row/column, failing the test when the shape is not the expected one.
func cellMark(t *testing.T, tbl *Table, row, cell int) string {
	t.Helper()
	r, ok := tbl.Content[row].(*TableRow)
	if !ok {
		t.Fatalf("row %d is %T, want a table row", row, tbl.Content[row])
	}
	blocks := NodeContent(r.Content[cell])
	if len(blocks) == 0 {
		t.Fatalf("cell %d,%d is empty", row, cell)
	}
	return blockAlignMark(blocks[0])
}

// onlyTable pulls the single table out of a transformed document.
func onlyTable(t *testing.T, doc Doc) *Table {
	t.Helper()
	if len(doc.Content) != 1 {
		t.Fatalf("document holds %d roots, want 1", len(doc.Content))
	}
	tbl, ok := doc.Content[0].(*Table)
	if !ok {
		t.Fatalf("root is %T, want a table", doc.Content[0])
	}
	return tbl
}

// The two spellings the mark has, and the one it does not: a centered
// column marks "center", a right-aligned one "end", and a left-aligned
// one nothing at all.
func TestLowerTableAlignMarksTheColumns(t *testing.T) {
	doc := wireDoc(alignTable(
		[]string{"center", "right", "left"},
		rowOf(cellOf("a"), cellOf("b"), cellOf("c")),
		rowOf(cellOf("d"), cellOf("e"), cellOf("f")),
	))

	tbl := onlyTable(t, LowerTableAlign(doc))
	for row := range 2 {
		if got := cellMark(t, tbl, row, 0); got != "center" {
			t.Errorf("row %d column 0 mark = %q, want center", row, got)
		}
		if got := cellMark(t, tbl, row, 1); got != "end" {
			t.Errorf("row %d column 1 mark = %q, want end", row, got)
		}
		if got := cellMark(t, tbl, row, 2); got != "" {
			t.Errorf("row %d column 2 mark = %q, want none: left is the absence of a mark", row, got)
		}
	}
}

// The synthetic attribute is what made the document unsubmittable, so
// lowering has to clear it.
func TestLowerTableAlignClearsTheAttribute(t *testing.T) {
	doc := wireDoc(alignTable([]string{"center"}, rowOf(cellOf("a"))))
	if IsWireSafe(doc) {
		t.Fatal("the fixture is already wire-safe, so it tests nothing")
	}

	out := LowerTableAlign(doc)
	if tbl := onlyTable(t, out); tbl.Align != nil {
		t.Errorf("align attribute = %v, want nil", tbl.Align)
	}
	if !IsWireSafe(out) {
		t.Error("lowered document is not wire-safe")
	}
}

// A table without the attribute is not a table to align.
func TestLowerTableAlignLeavesAnUnalignedTable(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{rowOf(cellOf("a"))}})

	if got := cellMark(t, onlyTable(t, LowerTableAlign(doc)), 0, 0); got != "" {
		t.Errorf("mark = %q, want none", got)
	}
}

// Headers are cells too: a delimiter row aligns the whole column, and
// the header is the part of it a reader looks at first.
func TestLowerTableAlignMarksHeaders(t *testing.T) {
	doc := wireDoc(alignTable(
		[]string{"center"},
		rowOf(&TableHeader{Content: []Node{&Heading{Level: 3, Content: []Node{&Text{Text: "h"}}}}}),
	))

	if got := cellMark(t, onlyTable(t, LowerTableAlign(doc)), 0, 0); got != "center" {
		t.Errorf("header mark = %q, want center", got)
	}
}

// An explicit ":::center" in the cell says more about that block than
// the delimiter row says about the column, so it stays as written.
func TestLowerTableAlignKeepsAnExplicitMark(t *testing.T) {
	doc := wireDoc(alignTable([]string{"right"}, rowOf(markedCell("a", "center"))))

	if got := cellMark(t, onlyTable(t, LowerTableAlign(doc)), 0, 0); got != "center" {
		t.Errorf("mark = %q, want the explicit center to survive", got)
	}
}

// Only the direct children of a cell are marked. A paragraph inside a
// list keeps the alignment of its list, the way the editor treats it.
func TestLowerTableAlignSkipsNestedBlocks(t *testing.T) {
	nested := &Paragraph{Content: []Node{&Text{Text: "n"}}}
	doc := wireDoc(alignTable([]string{"center"}, rowOf(&TableCell{Content: []Node{
		&BulletList{Content: []Node{&ListItem{Content: []Node{nested}}}},
	}})))

	LowerTableAlign(doc)
	if got := blockAlignMark(nested); got != "" {
		t.Errorf("nested paragraph mark = %q, want none", got)
	}
}

// A merged cell has one alignment. When the columns it covers ask for
// the same one it gets it, and when they disagree it gets none.
func TestLowerTableAlignHandlesAMergedCell(t *testing.T) {
	agreeing := wireDoc(alignTable(
		[]string{"center", "center"},
		rowOf(&TableCell{Colspan: 2, Content: []Node{&Paragraph{Content: []Node{&Text{Text: "wide"}}}}}),
	))
	if got := cellMark(t, onlyTable(t, LowerTableAlign(agreeing)), 0, 0); got != "center" {
		t.Errorf("merged cell mark = %q, want center", got)
	}

	disagreeing := wireDoc(alignTable(
		[]string{"center", "right"},
		rowOf(&TableCell{Colspan: 2, Content: []Node{&Paragraph{Content: []Node{&Text{Text: "wide"}}}}}),
	))
	if got := cellMark(t, onlyTable(t, LowerTableAlign(disagreeing)), 0, 0); got != "" {
		t.Errorf("merged cell mark = %q, want none: the columns it covers disagree", got)
	}
}

// A cell carried over by a rowspan holds its column in the rows below,
// so the cell after it starts one column further right.
func TestLowerTableAlignCountsRowspanColumns(t *testing.T) {
	second := cellOf("b2")
	doc := wireDoc(alignTable(
		[]string{"center", "right"},
		rowOf(&TableCell{Rowspan: 2, Content: []Node{&Paragraph{Content: []Node{&Text{Text: "tall"}}}}}, cellOf("b1")),
		rowOf(second),
	))

	tbl := onlyTable(t, LowerTableAlign(doc))
	if got := cellMark(t, tbl, 1, 0); got != "end" {
		t.Errorf("second-row cell mark = %q, want end: it sits in column 1", got)
	}
}

// Copy-on-write: the caller's document is never rewritten under it.
func TestLowerTableAlignLeavesTheInputAlone(t *testing.T) {
	original := alignTable([]string{"center"}, rowOf(cellOf("a")))
	doc := wireDoc(original)

	LowerTableAlign(doc)
	if original.Align == nil {
		t.Error("LowerTableAlign cleared the input's attribute")
	}
	if got := cellMark(t, original, 0, 0); got != "" {
		t.Errorf("input mark = %q, want none: LowerTableAlign marked the input", got)
	}
}

// The lift: a column every cell of which says the same thing becomes a
// delimiter row, and the marks it stood for are gone.
func TestLiftTableAlignReadsAUniformColumn(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{
		rowOf(markedCell("a", "center"), markedCell("b", "end")),
		rowOf(markedCell("c", "center"), markedCell("d", "end")),
	}})

	tbl := onlyTable(t, LiftTableAlign(doc))
	want := []string{"center", "right"}
	if len(tbl.Align) != 2 || tbl.Align[0] != want[0] || tbl.Align[1] != want[1] {
		t.Fatalf("align = %v, want %v", tbl.Align, want)
	}
	for row := range 2 {
		for cell := range 2 {
			if got := cellMark(t, tbl, row, cell); got != "" {
				t.Errorf("cell %d,%d still marked %q", row, cell, got)
			}
		}
	}
}

// A column that disagrees with itself takes what most of it says, and
// the cells out of line keep their own mark.
func TestLiftTableAlignTakesTheMajorityOfAMixedColumn(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{
		rowOf(markedCell("a", "center")),
		rowOf(markedCell("b", "center")),
		rowOf(markedCell("c", "end")),
	}})

	tbl := onlyTable(t, LiftTableAlign(doc))
	if len(tbl.Align) != 1 || tbl.Align[0] != "center" {
		t.Fatalf("align = %v, want [center]: two cells of three say so", tbl.Align)
	}
	for row := range 2 {
		if got := cellMark(t, tbl, row, 0); got != "" {
			t.Errorf("row %d mark = %q, want none: the column says it now", row, got)
		}
	}
	if got := cellMark(t, tbl, 2, 0); got != "end" {
		t.Errorf("minority mark = %q, want end: the column does not speak for it", got)
	}
}

// An unmarked block votes too, so one centered cell in a column of plain
// text does not center the column.
func TestLiftTableAlignCountsUnmarkedBlocks(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{
		rowOf(cellOf("a")),
		rowOf(cellOf("b")),
		rowOf(markedCell("c", "center")),
	}})

	tbl := onlyTable(t, LiftTableAlign(doc))
	if tbl.Align != nil {
		t.Fatalf("align = %v, want nil: most of the column carries no mark", tbl.Align)
	}
	if got := cellMark(t, tbl, 2, 0); got != "center" {
		t.Errorf("mark = %q, want the center to stay put", got)
	}
}

// A tie goes to the alignment the reader meets first, so the answer does
// not depend on map order.
func TestLiftTableAlignBreaksATieByDocumentOrder(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{
		rowOf(markedCell("a", "end")),
		rowOf(markedCell("b", "center")),
	}})

	if tbl := onlyTable(t, LiftTableAlign(doc)); len(tbl.Align) != 1 || tbl.Align[0] != "right" {
		t.Errorf("align = %v, want [right]: the end mark came first", tbl.Align)
	}
}

// Two alignments inside one cell are two votes, not one.
func TestLiftTableAlignCountsEachBlockOfACell(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{rowOf(&TableCell{Content: []Node{
		&Paragraph{Marks: []Mark{&Alignment{Align: "center"}}, Content: []Node{&Text{Text: "a"}}},
		&Paragraph{Marks: []Mark{&Alignment{Align: "center"}}, Content: []Node{&Text{Text: "b"}}},
		&Paragraph{Marks: []Mark{&Alignment{Align: "end"}}, Content: []Node{&Text{Text: "c"}}},
	}})}})

	tbl := onlyTable(t, LiftTableAlign(doc))
	if len(tbl.Align) != 1 || tbl.Align[0] != "center" {
		t.Fatalf("align = %v, want [center]", tbl.Align)
	}
	row, ok := tbl.Content[0].(*TableRow)
	if !ok {
		t.Fatalf("row is %T, want a table row", tbl.Content[0])
	}
	blocks := NodeContent(row.Content[0])
	if got := blockAlignMark(blocks[2]); got != "end" {
		t.Errorf("third block mark = %q, want end", got)
	}
}

// A blank cell is not evidence that the column is unaligned. The editor
// puts no mark on an empty paragraph either.
func TestLiftTableAlignPassesOverAnEmptyCell(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{
		rowOf(markedCell("a", "center")),
		rowOf(&TableCell{Content: []Node{&Paragraph{}}}),
	}})

	tbl := onlyTable(t, LiftTableAlign(doc))
	if len(tbl.Align) != 1 || tbl.Align[0] != "center" {
		t.Errorf("align = %v, want [center]", tbl.Align)
	}
}

// A table nobody aligned keeps its nil attribute, so an unaligned table
// converts to exactly what it did before alignment existed.
func TestLiftTableAlignLeavesAnUnmarkedTable(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{rowOf(cellOf("a"))}})

	if tbl := onlyTable(t, LiftTableAlign(doc)); tbl.Align != nil {
		t.Errorf("align = %v, want nil", tbl.Align)
	}
}

// A table that already carries the attribute is the caller's, not ours.
func TestLiftTableAlignLeavesAnAlignedTable(t *testing.T) {
	doc := wireDoc(alignTable([]string{"left"}, rowOf(markedCell("a", "center"))))

	tbl := onlyTable(t, LiftTableAlign(doc))
	if len(tbl.Align) != 1 || tbl.Align[0] != "left" {
		t.Errorf("align = %v, want [left] unchanged", tbl.Align)
	}
	if got := cellMark(t, tbl, 0, 0); got != "center" {
		t.Errorf("mark = %q, want it left in place", got)
	}
}

// A merged cell sits in no single column: it does not vote, and it keeps
// its mark so that re-lowering puts the document back as it was.
func TestLiftTableAlignPassesOverAMergedCell(t *testing.T) {
	doc := wireDoc(&Table{Content: []Node{
		rowOf(&TableCell{Colspan: 2, Content: []Node{
			&Paragraph{Marks: []Mark{&Alignment{Align: "end"}}, Content: []Node{&Text{Text: "wide"}}},
		}}),
		rowOf(markedCell("a", "center"), markedCell("b", "center")),
	}})

	tbl := onlyTable(t, LiftTableAlign(doc))
	want := []string{"center", "center"}
	if len(tbl.Align) != 2 || tbl.Align[0] != want[0] || tbl.Align[1] != want[1] {
		t.Fatalf("align = %v, want %v: the merged cell must not vote", tbl.Align, want)
	}
	if got := cellMark(t, tbl, 0, 0); got != "end" {
		t.Errorf("merged cell mark = %q, want it left in place", got)
	}
}

// Copy-on-write on the lift side too.
func TestLiftTableAlignLeavesTheInputAlone(t *testing.T) {
	original := &Table{Content: []Node{rowOf(markedCell("a", "center"))}}

	LiftTableAlign(wireDoc(original))
	if original.Align != nil {
		t.Errorf("align = %v, want nil: LiftTableAlign wrote to the input", original.Align)
	}
	if got := cellMark(t, original, 0, 0); got != "center" {
		t.Errorf("input mark = %q, want center: LiftTableAlign unmarked the input", got)
	}
}

// The pair is a round trip for the alignments the mark can spell.
func TestTableAlignRoundTrips(t *testing.T) {
	doc := wireDoc(alignTable(
		[]string{"center", "right"},
		rowOf(cellOf("a"), cellOf("b")),
		rowOf(cellOf("c"), cellOf("d")),
	))

	back := onlyTable(t, LiftTableAlign(LowerTableAlign(doc)))
	want := []string{"center", "right"}
	if len(back.Align) != 2 || back.Align[0] != want[0] || back.Align[1] != want[1] {
		t.Fatalf("align = %v, want %v", back.Align, want)
	}
	for row := range 2 {
		for cell := range 2 {
			if got := cellMark(t, back, row, cell); got != "" {
				t.Errorf("cell %d,%d still marked %q", row, cell, got)
			}
		}
	}
}

// Left is the one alignment the mark cannot spell, so it comes back as
// no alignment. The rendered table looks the same; the delimiter row
// does not.
func TestTableAlignLosesLeftOnTheWayBack(t *testing.T) {
	doc := wireDoc(alignTable([]string{"left"}, rowOf(cellOf("a"))))

	if tbl := onlyTable(t, LiftTableAlign(LowerTableAlign(doc))); tbl.Align != nil {
		t.Errorf("align = %v, want nil: the mark has no left", tbl.Align)
	}
}
