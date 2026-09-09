package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// A hard break and a link reference definition are the two inline kinds
// that used to destroy the cell they sat in rather than merely render
// oddly: the break wrote its end of line into a row that is one line, and
// the definition wrote nothing at all because everything it carries is a
// field and the inline fallback only writes children.
//
// Every case renders through cellTable, whose FIRST body cell is an
// ordinary paragraph that always worked. That cell is the good case in
// the same document: a break that writes an end of line cuts the table in
// half, so the plain cell's own row stops rendering as a row too and the
// want string below fails on its account as much as on the subject's.

func TestTableCell_HardBreakFoldsToASpace(t *testing.T) {
	cases := []struct {
		kid  ast.Node
		name string
		want string
	}{
		{
			// The break's own end of line is what breaks the row, so a
			// break with nothing around it is the smallest case.
			name: "bare",
			kid:  &ast.Break{},
			want: "| good | subject |\n| ---- | ------- |\n| A    |         |\n",
		},
		{
			// The shape a conversion produces: text, break, text. The
			// space has to land BETWEEN them, which is what tells a
			// fold apart from a drop.
			name: "between two texts",
			kid: &ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "a"}, &ast.Break{}, &ast.Text{Value: "b"},
			}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | a b     |\n",
		},
		{
			// The trailing-space spelling is the same hazard: two
			// spaces plus an end of line still ends the row.
			name: "trailing-space form",
			kid: &ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "a"}, &ast.Break{Value: "  "}, &ast.Text{Value: "b"},
			}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | a b     |\n",
		},
		{
			// Nested below a block wrapper the cell projection
			// flattens, so the fold has to hold for a break the
			// projection re-parents rather than only for a direct child.
			name: "under a blockquote",
			kid: &ast.Blockquote{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "p"}, &ast.Break{}, &ast.Text{Value: "q"},
			}}}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | p q     |\n",
		},
		{
			// Nested below an INLINE wrapper, which the projection does
			// not walk into: the fold has to travel with the inline
			// state, not with the projection.
			name: "under emphasis",
			kid: &ast.Emphasis{Children: []ast.Node{
				&ast.Text{Value: "a"}, &ast.Break{}, &ast.Text{Value: "b"},
			}},
			want: "| good | subject |\n| ---- | ------- |\n| A    | _a b_   |\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(cellTable(tc.kid)); got != tc.want {
				t.Errorf("hard break in a cell (%s):\ngot  %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestHardBreakOutsideACellKeepsItsEndOfLine is the counterweight PIN: the
// fold is the table cell's rule and only the table cell's, so a break
// anywhere a line may end still writes one. Without this the cell fix
// could be spelled as "never write an end of line for a break", which
// would silently flatten every paragraph in the document.
func TestHardBreakOutsideACellKeepsItsEndOfLine(t *testing.T) {
	cases := []struct {
		node ast.Node
		name string
		want string
	}{
		{
			name: "paragraph",
			node: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "a"}, &ast.Break{}, &ast.Text{Value: "b"},
			}}}},
			want: "a\\\nb\n",
		},
		{
			name: "blockquote",
			node: &ast.Root{Children: []ast.Node{&ast.Blockquote{Children: []ast.Node{
				&ast.Paragraph{Children: []ast.Node{
					&ast.Text{Value: "a"}, &ast.Break{}, &ast.Text{Value: "b"},
				}},
			}}}},
			want: "> a\\\n> b\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(tc.node); got != tc.want {
				t.Errorf("hard break in a %s:\ngot  %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestTableCell_DefinitionKeepsItsText covers the other half. A definition
// keeps its label, destination and title in FIELDS and has no children, so
// degrading it to its children wrote nothing and the cell came out empty —
// the same "falls through to nothing" loss the projection already fixed
// for a code block and for frontmatter. Its block spelling is one line
// anyway, so writing it inline gives up nothing to fit.
func TestTableCell_DefinitionKeepsItsText(t *testing.T) {
	cases := []struct {
		kid  ast.Node
		name string
		want string
	}{
		{
			name: "label and url",
			kid:  &ast.Definition{Label: "x", URL: "u"},
			want: "| good | subject |\n| ---- | ------- |\n| A    | [x]: u  |\n",
		},
		{
			name: "with a title",
			kid:  &ast.Definition{Label: "x", URL: "u", Title: "t"},
			want: "| good | subject    |\n| ---- | ---------- |\n| A    | [x]: u \"t\" |\n",
		},
		{
			// The empty destination still has to be written as the
			// angle pair, or the line is not a definition at all —
			// renderDefinition's rule, inherited by writing the same
			// spelling rather than a second one.
			name: "empty url",
			kid:  &ast.Definition{Label: "x"},
			want: "| good | subject |\n| ---- | ------- |\n| A    | [x]: <> |\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(cellTable(tc.kid)); got != tc.want {
				t.Errorf("definition in a cell (%s):\ngot  %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestDefinitionAtBlockLevelIsUnchanged is the PIN for the other side of
// the definition change: writing the block spelling from inline position
// must not have altered what block position writes, including the blank
// line a definition run takes between its entries.
func TestDefinitionAtBlockLevelIsUnchanged(t *testing.T) {
	got := Render(&ast.Root{Children: []ast.Node{
		&ast.Definition{Label: "a", URL: "./a.md"},
		&ast.Definition{Label: "b", URL: "./b.md", Title: "B"},
	}})
	want := "[a]: ./a.md\n\n[b]: ./b.md \"B\"\n"
	if got != want {
		t.Errorf("definitions at block level:\ngot  %q\nwant %q", got, want)
	}
}
