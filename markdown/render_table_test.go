package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// A table cell in the pivot AST may hold block nodes — the ADF model puts
// blocks in a tableCell — and a table row is one line, so the cell writes
// their content without their markers. These tests measure that the
// content survives, and every one of them carries the single-paragraph
// cell that already rendered correctly in the same table, so the good case
// is measured on the same render as the subject.

// cellPara is a paragraph holding one text node.
func cellPara(s string) ast.Node {
	return &ast.Paragraph{Children: []ast.Node{&ast.Text{Value: s}}}
}

// textCell is a cell holding one bare text node.
func textCell(s string) ast.Node {
	return &ast.TableCell{Children: []ast.Node{&ast.Text{Value: s}}}
}

// cellTable builds a two-column table whose second body cell holds kids
// and whose first body cell is the single-paragraph cell that already
// worked, so one render measures both.
func cellTable(kids ...ast.Node) ast.Node {
	return &ast.Root{Children: []ast.Node{&ast.Table{Children: []ast.Node{
		&ast.TableRow{Children: []ast.Node{textCell("good"), textCell("subject")}},
		&ast.TableRow{Children: []ast.Node{
			&ast.TableCell{Children: []ast.Node{cellPara("A")}},
			&ast.TableCell{Children: kids},
		}},
	}}}}
}

func TestTableCell_CodeBlockKeepsItsText(t *testing.T) {
	// ast.Code carries its text in Value, not in Children, so the inline
	// fallback that degrades a block in inline position never saw it and
	// the cell came out empty.
	got := Render(cellTable(&ast.Code{Lang: "go", Value: "A"}))
	want := "| good | subject |\n| ---- | ------- |\n| A    | `A`     |\n"
	if got != want {
		t.Errorf("code block in a cell:\ngot  %q\nwant %q", got, want)
	}
}

func TestTableCell_CodeBlockInsideABlockquoteKeepsItsText(t *testing.T) {
	// The recovery has to reach through the block wrappers too, not just
	// the cell's own children.
	got := Render(cellTable(&ast.Blockquote{Children: []ast.Node{
		cellPara("B"),
		&ast.Code{Lang: "go", Value: "A"},
	}}))
	want := "| good | subject |\n| ---- | ------- |\n| A    | B`A`    |\n"
	if got != want {
		t.Errorf("code block nested in a cell:\ngot  %q\nwant %q", got, want)
	}
}

func TestTableCell_CodeBlockPipeStaysEscaped(t *testing.T) {
	// The recovered text goes through the cell's own inline writer, so
	// the pipe that would split the row is escaped like any other.
	got := Render(cellTable(&ast.Code{Value: "a|b"}))
	want := "| good | subject |\n| ---- | ------- |\n| A    | `a\\|b`  |\n"
	if got != want {
		t.Errorf("piped code block in a cell:\ngot  %q\nwant %q", got, want)
	}
}

func TestTableCell_FrontmatterKeepsItsText(t *testing.T) {
	// Metadata in a cell is not something a parse produces, but a
	// hand-built tree can hold it and it kept its text in Value too.
	got := Render(cellTable(&ast.Frontmatter{Value: "x: 1"}))
	want := "| good | subject |\n| ---- | ------- |\n| A    | `x: 1`  |\n"
	if got != want {
		t.Errorf("frontmatter in a cell:\ngot  %q\nwant %q", got, want)
	}
}

// TestTableCell_BlockWrappersDegradeToTheirContent pins what the cell
// already did with the block kinds whose content is their children: the
// content is written and the marker is not. Preserved behavior, not a
// change — the reference serializer writes the markers instead, and for
// any block that spans lines that stops being a table.
func TestTableCell_BlockWrappersDegradeToTheirContent(t *testing.T) {
	cases := []struct {
		name string
		want string
		kids []ast.Node
	}{
		{
			name: "paragraph",
			kids: []ast.Node{cellPara("B")},
			want: "| good | subject |\n| ---- | ------- |\n| A    | B       |\n",
		},
		{
			name: "blockquote",
			kids: []ast.Node{&ast.Blockquote{Children: []ast.Node{cellPara("B")}}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | B       |\n",
		},
		{
			name: "bullet list",
			kids: []ast.Node{&ast.List{Children: []ast.Node{
				&ast.ListItem{Children: []ast.Node{cellPara("B")}},
			}}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | B       |\n",
		},
		{
			name: "heading",
			kids: []ast.Node{&ast.Heading{Depth: 3, Children: []ast.Node{&ast.Text{Value: "B"}}}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | B       |\n",
		},
		{
			name: "two paragraphs",
			kids: []ast.Node{cellPara("B"), cellPara("C")},
			want: "| good | subject |\n| ---- | ------- |\n| A    | BC      |\n",
		},
		{
			name: "nested table",
			kids: []ast.Node{&ast.Table{Children: []ast.Node{
				&ast.TableRow{Children: []ast.Node{textCell("B")}},
			}}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | B       |\n",
		},
		{
			// The one block with no content at all, so the cell has
			// nothing to lose by writing nothing.
			name: "thematic break",
			kids: []ast.Node{&ast.ThematicBreak{}},
			want: "| good | subject |\n| ---- | ------- |\n| A    |         |\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(cellTable(tc.kids...)); got != tc.want {
				t.Errorf("%s in a cell:\ngot  %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}
