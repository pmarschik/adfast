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

// TestTableCell_EveryBlockWrapperIsReachedThrough pins the whole list of
// kinds the cell projection recurses into, one case per kind. Without the
// pins only the blockquote entry was load-bearing: dropping any other kind
// left the suite green, because the inline writer's own fallback degrades
// that kind to its children and produces the same bytes. The list earns
// its keep only where a rewritten leaf sits below it, so every entry gets
// a leaf below it here — an ast.Code, whose text the fallback cannot
// reach on its own.
func TestTableCell_EveryBlockWrapperIsReachedThrough(t *testing.T) {
	code := func() ast.Node { return &ast.Code{Lang: "go", Value: "A"} }
	cases := []struct {
		kid  ast.Node
		name string
	}{
		{name: "root", kid: &ast.Root{Children: []ast.Node{code()}}},
		{name: "paragraph", kid: &ast.Paragraph{Children: []ast.Node{code()}}},
		{name: "heading", kid: &ast.Heading{Depth: 2, Children: []ast.Node{code()}}},
		{name: "blockquote", kid: &ast.Blockquote{Children: []ast.Node{code()}}},
		{name: "list", kid: &ast.List{Children: []ast.Node{
			&ast.ListItem{Children: []ast.Node{code()}},
		}}},
		{name: "list item", kid: &ast.ListItem{Children: []ast.Node{code()}}},
		{name: "table", kid: &ast.Table{Children: []ast.Node{
			&ast.TableRow{Children: []ast.Node{&ast.TableCell{Children: []ast.Node{code()}}}},
		}}},
		{name: "table row", kid: &ast.TableRow{Children: []ast.Node{
			&ast.TableCell{Children: []ast.Node{code()}},
		}}},
		{name: "table cell", kid: &ast.TableCell{Children: []ast.Node{code()}}},
		{name: "container directive", kid: &ast.ContainerDirective{Name: "note", Children: []ast.Node{code()}}},
		{name: "leaf directive", kid: &ast.LeafDirective{Name: "note", Children: []ast.Node{code()}}},
	}
	want := "| good | subject |\n| ---- | ------- |\n| A    | `A`     |\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(cellTable(tc.kid)); got != want {
				t.Errorf("code block under a %s in a cell:\ngot  %q\nwant %q", tc.name, got, want)
			}
		})
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

// cellTableTwoRows builds the cell table with a second body row, so the
// one-line value and the multi-line one are measured in one document: a
// fold that went too far would show up on the good row.
func cellTableTwoRows(good, multi ast.Node) ast.Node {
	return &ast.Root{Children: []ast.Node{&ast.Table{Children: []ast.Node{
		&ast.TableRow{Children: []ast.Node{textCell("good"), textCell("subject")}},
		&ast.TableRow{Children: []ast.Node{
			&ast.TableCell{Children: []ast.Node{cellPara("A")}},
			&ast.TableCell{Children: []ast.Node{good}},
		}},
		&ast.TableRow{Children: []ast.Node{
			&ast.TableCell{Children: []ast.Node{cellPara("B")}},
			&ast.TableCell{Children: []ast.Node{multi}},
		}},
	}}}}
}

// TestTableCell_MultiLineCodeBlockStaysOnItsRow measures the ordinary
// code block: a fence holds several lines, so recovering the text without
// folding the ends of line put those lines straight into the row. The
// one-line code block above is the unusual shape.
func TestTableCell_MultiLineCodeBlockStaysOnItsRow(t *testing.T) {
	got := Render(cellTableTwoRows(&ast.Code{Value: "one"}, &ast.Code{Value: "x\ny"}))
	want := "| good | subject |\n| ---- | ------- |\n| A    | `one`   |\n| B    | `x y`   |\n"
	if got != want {
		t.Errorf("multiline code block in a cell:\ngot  %q\nwant %q", got, want)
	}
	// A CRLF is one end of line and folds to one space, not to two.
	got = Render(cellTableTwoRows(&ast.Code{Value: "one"}, &ast.Code{Value: "x\r\ny"}))
	if got != want {
		t.Errorf("CRLF code block in a cell:\ngot  %q\nwant %q", got, want)
	}
	// The value that is nothing but an end of line still has to leave a
	// span the cell can hold, and that span has to survive a re-parse.
	got = Render(cellTable(&ast.Code{Value: "\n"}))
	want = "| good | subject |\n| ---- | ------- |\n| A    | ` `     |\n"
	if got != want {
		t.Errorf("code block of one newline in a cell:\ngot  %q\nwant %q", got, want)
	}
	if round := Render(Parse([]byte(got))); round != got {
		t.Errorf("re-render of the folded cell:\ngot  %q\nwant %q", round, got)
	}
}

// TestTableCell_MultiLineFrontmatterStaysOnItsRow measures the same fold
// for the other value the projection recovers. Frontmatter is multi-line
// by nature, so the folded shape is the normal one here as well.
func TestTableCell_MultiLineFrontmatterStaysOnItsRow(t *testing.T) {
	got := Render(cellTableTwoRows(
		&ast.Frontmatter{Value: "x: 1"},
		&ast.Frontmatter{Value: "x: 1\ny: 2"},
	))
	want := "| good | subject     |\n| ---- | ----------- |\n| A    | `x: 1`      |\n" +
		"| B    | `x: 1 y: 2` |\n"
	if got != want {
		t.Errorf("multiline frontmatter in a cell:\ngot  %q\nwant %q", got, want)
	}
}

func TestTableCell_RawHTMLNewlineCannotBreakTheRow(t *testing.T) {
	// Raw HTML is written verbatim, so a value spanning lines used to put
	// those lines straight into the row and the table stopped being a
	// table. A cell is one line, so no parse can produce this value —
	// only a hand-built tree or an ADF conversion.
	got := Render(cellTable(&ast.HTML{Value: "<div>\nA\n</div>"}))
	want := "| good | subject        |\n| ---- | -------------- |\n| A    | <div> A </div> |\n"
	if got != want {
		t.Errorf("multiline raw HTML in a cell:\ngot  %q\nwant %q", got, want)
	}
}

func TestTableCell_RawHTMLPipeStaysEscaped(t *testing.T) {
	// A bare pipe in raw HTML split the row into two columns. It reaches
	// the cell only from a hand-built tree: a parsed cell keeps the
	// backslash inside the html value itself (see the pin below).
	got := Render(cellTable(&ast.HTML{Value: "<b>a|b</b>"}))
	want := "| good | subject     |\n| ---- | ----------- |\n| A    | <b>a\\|b</b> |\n"
	if got != want {
		t.Errorf("piped raw HTML in a cell:\ngot  %q\nwant %q", got, want)
	}
}

func TestTableCell_RawHTMLAlreadyEscapedPipeIsLeftAlone(t *testing.T) {
	// The escape must not stack, or every render of a parsed table would
	// add a backslash. Both the value a parse produces and a value whose
	// backslash is itself escaped are measured here.
	got := Render(cellTable(&ast.HTML{Value: `<a title="a\|b">`}))
	want := "| good | subject          |\n| ---- | ---------------- |\n| A    | <a title=\"a\\|b\"> |\n"
	if got != want {
		t.Errorf("pre-escaped raw HTML in a cell:\ngot  %q\nwant %q", got, want)
	}
	got = Render(cellTable(&ast.HTML{Value: `<a t="a\\|b">`}))
	want = "| good | subject        |\n| ---- | -------------- |\n| A    | <a t=\"a\\\\\\|b\"> |\n"
	if got != want {
		t.Errorf("escaped backslash before a pipe:\ngot  %q\nwant %q", got, want)
	}
}

// TestTableCell_ParsedHTMLPipeSurvivesReRender pins why the escape above
// cannot cost a round trip: a pipe written inside a cell's HTML keeps its
// backslash in the html value, so the render leaves it exactly as parsed.
func TestTableCell_ParsedHTMLPipeSurvivesReRender(t *testing.T) {
	src := "| h                     |\n| --------------------- |\n| <a title=\"a\\|b\">x</a> |\n"
	if got := Render(Parse([]byte(src))); got != src {
		t.Errorf("parsed HTML pipe re-render:\ngot  %q\nwant %q", got, src)
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
			// Raw HTML that already fits a row is written verbatim, like
			// the reference writes it.
			name: "inline raw HTML",
			kids: []ast.Node{&ast.HTML{Value: "<b>B</b>"}},
			want: "| good | subject  |\n| ---- | -------- |\n| A    | <b>B</b> |\n",
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
