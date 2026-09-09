package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// A GFM table cell is ONE LINE whose pipes are escaped. That is the whole
// invariant, and it was broken by sixteen inline shapes, all of them
// reachable from a hand-built tree or an ADF conversion but from no
// Markdown parse (a cell is one line, so a parse cannot put an end of line
// in one, and it escapes a pipe on the way in).
//
// The shapes had no writer in common. Twelve wrote a raw end of line —
// through a text value, a code span's value, the hard-break writer in both
// its spellings, a link or image URL, a link title, and the same text
// value seen through an emphasis, a strikethrough, a directive label, a
// blockquote and a list item. Four wrote a bare pipe — through a link
// title, an image URL, an image title and a directive attribute value,
// the three fields the per-writer escaping never consulted while it
// already covered text and link URLs.
//
// So the rule is asserted here the way a reader can check it: the table
// still has three rows, and its body row has exactly three UNESCAPED
// pipes. Both halves matter and they fail differently — an end of line
// makes the row count four and the table stops being a table, while a bare
// pipe keeps three rows and splits the cell into two columns.

// unescapedPipes counts the pipes in s that a GFM parse would read as cell
// delimiters: the ones NOT preceded by an odd run of backslashes. A plain
// count over the string reports an escaped "\|" as a delimiter and would
// call a correct row broken.
func unescapedPipes(s string) int {
	n, backslashes := 0, 0
	for i := range len(s) {
		switch s[i] {
		case '|':
			if backslashes%2 == 0 {
				n++
			}
			backslashes = 0
		case '\\':
			backslashes++
		default:
			backslashes = 0
		}
	}
	return n
}

func TestUnescapedPipes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{in: "", want: 0},
		{in: "| a | b |", want: 3},
		{in: `| a\|b |`, want: 2},
		{in: `| a\\|b |`, want: 3},   // the backslash is escaped, the pipe is not
		{in: `| a\\\|b |`, want: 2},  // three backslashes: the pipe is escaped
		{in: `| a\\\\|b |`, want: 3}, // four: the pipe is bare again
	}
	for _, tc := range cases {
		if got := unescapedPipes(tc.in); got != tc.want {
			t.Errorf("unescapedPipes(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestTableCell_StaysOneRowWithEscapedPipes is the mechanism test: every
// shape that broke the invariant, and beside them the shapes that already
// held it. The already-correct rows are the good cases in the same test —
// a fix that reached the invariant by mangling the cell wholesale (say by
// escaping every pipe twice, or by dropping content that holds a newline)
// turns those red while the broken rows go green.
func TestTableCell_StaysOneRowWithEscapedPipes(t *testing.T) {
	txt := func(s string) ast.Node { return &ast.Text{Value: s} }
	nl := func() ast.Node { return txt("x\ny") }
	cases := []struct {
		kid  ast.Node
		name string
		body string
	}{
		// --- was a raw end of line: four rows, no table left ---
		{name: "text value", kid: nl(), body: "| A    | x y     |"},
		{name: "code span value", kid: &ast.InlineCode{Value: "x\ny"}, body: "| A    | `x y`   |"},
		{name: "break, backslash form", kid: &ast.Break{}, body: "| A    |         |"},
		{name: "break, trailing-space form", kid: &ast.Break{Value: "  "}, body: "| A    |         |"},
		{name: "under emphasis", kid: &ast.Emphasis{Children: []ast.Node{nl()}}, body: "| A    | _x y_   |"},
		{name: "under strikethrough", kid: &ast.Delete{Children: []ast.Node{nl()}}, body: "| A    | ~~x y~~ |"},
		{
			name: "in a directive label",
			kid:  &ast.TextDirective{Name: "d", Children: []ast.Node{nl()}},
			body: "| A    | :d[x y] |",
		},
		{
			name: "link url",
			kid:  &ast.Link{URL: "x\ny", Children: []ast.Node{txt("t")}},
			body: "| A    | [t](<x y>) |",
		},
		{
			name: "link title",
			kid:  &ast.Link{URL: "u", Title: "x\ny", Children: []ast.Node{txt("t")}},
			body: `| A    | [t](u "x y") |`,
		},
		{name: "image url", kid: &ast.Image{URL: "x\ny"}, body: "| A    | ![](<x y>) |"},
		{
			name: "break under a blockquote",
			kid: &ast.Blockquote{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				txt("p"), &ast.Break{}, txt("q"),
			}}}},
			body: "| A    | p q     |",
		},
		{
			name: "text value under a list item",
			kid: &ast.List{Children: []ast.Node{&ast.ListItem{Children: []ast.Node{
				&ast.Paragraph{Children: []ast.Node{nl()}},
			}}}},
			body: "| A    | x y     |",
		},

		// --- was a bare pipe: three rows, but the cell split in two ---
		{
			name: "pipe in a link title",
			kid:  &ast.Link{URL: "u", Title: "a|b", Children: []ast.Node{txt("t")}},
			body: `| A    | [t](u "a\|b") |`,
		},
		{name: "pipe in an image url", kid: &ast.Image{URL: "a|b"}, body: `| A    | ![](a\|b) |`},
		{name: "pipe in an image title", kid: &ast.Image{URL: "u", Title: "a|b"}, body: `| A    | ![](u "a\|b") |`},
		{
			name: "pipe in a directive attribute",
			kid:  &ast.TextDirective{Name: "d", Attrs: map[string]string{"k": "a|b"}},
			body: `| A    | :d{k="a\|b"} |`,
		},

		// --- the good cases: already correct, and must stay that way ---
		{name: "good: pipe in a text value", kid: txt("a|b"), body: `| A    | a\|b    |`},
		{
			name: "good: pipe in a link url",
			kid:  &ast.Link{URL: "a|b", Children: []ast.Node{txt("t")}},
			body: `| A    | [t](a\|b) |`,
		},
		{name: "good: a code block", kid: &ast.Code{Lang: "go", Value: "A"}, body: "| A    | `A`     |"},
		{name: "good: plain text", kid: txt("x y"), body: "| A    | x y     |"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(cellTable(tc.kid))
			lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
			if len(lines) != 3 {
				t.Fatalf("cell holding a %s is not one row: %d lines, want 3\ngot %q",
					tc.name, len(lines), got)
			}
			if n := unescapedPipes(lines[2]); n != 3 {
				t.Errorf("cell holding a %s splits the row: %d unescaped pipes, want 3\ngot %q",
					tc.name, n, lines[2])
			}
			if lines[2] != tc.body {
				t.Errorf("cell holding a %s:\ngot  %q\nwant %q", tc.name, lines[2], tc.body)
			}
		})
	}
}

// TestParagraphKeepsWhatACellFolds is the counterweight PIN. The fold and
// the pipe escape are the CELL's rule; a paragraph is not one line and its
// pipes carry no meaning, so the same values there must come out untouched.
// Without this the invariant could be reached by folding ends of line
// everywhere, which would silently join every wrapped value in the
// document.
func TestParagraphKeepsWhatACellFolds(t *testing.T) {
	cases := []struct {
		node ast.Node
		name string
		want string
	}{
		{
			name: "end of line in a text value",
			node: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "x\ny"},
			}}}},
			want: "x\ny\n",
		},
		{
			name: "pipe in a text value",
			node: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "a|b"},
			}}}},
			want: "a|b\n",
		},
		{
			name: "pipe in an image title",
			node: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Image{URL: "u", Title: "a|b"},
			}}}},
			want: "![](u \"a|b\")\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(tc.node); got != tc.want {
				t.Errorf("%s in a paragraph:\ngot  %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}
