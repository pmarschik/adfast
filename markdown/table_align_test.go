package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// tableOf parses src and returns its single table.
func tableOf(t *testing.T, src string) *ast.Table {
	t.Helper()
	kids := ast.Children(Parse([]byte(src)))
	if len(kids) != 1 {
		t.Fatalf("parse %q: want 1 block, got %d", src, len(kids))
	}
	tbl, ok := kids[0].(*ast.Table)
	if !ok {
		t.Fatalf("parse %q: want *ast.Table, got %T", src, kids[0])
	}
	return tbl
}

func TestTableAlign_Parse(t *testing.T) {
	tests := []struct {
		name, src string
		want      []ast.Alignment
	}{
		{"none", "| a | b |\n| - | - |\n", nil},
		{
			"left and right", "| a | b |\n|:--|--:|\n",
			[]ast.Alignment{ast.AlignLeft, ast.AlignRight},
		},
		{"center", "| a |\n|:-:|\n", []ast.Alignment{ast.AlignCenter}},
		{
			"mixed with a bare column", "| a | b | c |\n|:-|-|-:|\n",
			[]ast.Alignment{ast.AlignLeft, ast.AlignNone, ast.AlignRight},
		},
		// A single colon-less column leaves nothing to carry, so a table
		// without alignment keeps a nil list (see ast.AnyAligned).
		{"all bare", "| a | b | c |\n|---|---|---|\n", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tableOf(t, tc.src).Align
			if len(got) != len(tc.want) {
				t.Fatalf("align = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("align = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestTableAlign_RoundTrip pins md → ast → md byte-for-byte against
// remark-stringify with remark-gfm (measured with remark 15 /
// mdast-util-gfm-table, which serializes through markdown-table). The
// delimiter row carries the colons, AND the cell padding follows the
// alignment: right-aligned content is pushed to the right of its column,
// centered content splits the padding with the odd space in front.
func TestTableAlign_RoundTrip(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{
			"left and right widen their columns",
			"| a | b |\n|:--|--:|\n| 1 | 2 |\n",
			"| a  |  b |\n| :- | -: |\n| 1  |  2 |\n",
		},
		{
			"every alignment at once",
			"| a | b | c | d |\n|:-|-:|:-:|-|\n| 1 | 2 | 3 | 4 |\n",
			"| a  |  b |  c  | d |\n| :- | -: | :-: | - |\n| 1  |  2 |  3  | 4 |\n",
		},
		{
			"a wide column fills the delimiter with dashes",
			"| aaaa | bbbb | cccc | dddd |\n|:-|-:|:-:|-|\n| 1 | 2 | 3 | 4 |\n",
			"| aaaa | bbbb | cccc | dddd |\n| :--- | ---: | :--: | ---- |\n| 1    |    2 |   3  | 4    |\n",
		},
		{
			"the colons widen a column narrower than they are",
			"| a |\n|:-:|\n",
			"|  a  |\n| :-: |\n",
		},
		{
			"an odd center pad goes in front",
			"| ab |\n|:-:|\n",
			"|  ab |\n| :-: |\n",
		},
		{
			"a column as wide as its colons needs no padding",
			"| abc |\n|:-:|\n",
			"| abc |\n| :-: |\n",
		},
		{
			"empty cells still get a delimiter dash",
			"|  |\n|:-|\n",
			"|    |\n| :- |\n",
		},
		{
			"a right-aligned body cell trails its column",
			"| ab |\n|-:|\n| c |\n",
			"| ab |\n| -: |\n|  c |\n",
		},
		{
			"astral-free CJK counts one unit per rune",
			"| 日本 | b |\n|:-:|-:|\n| x | yy |\n",
			"|  日本 |  b |\n| :-: | -: |\n|  x  | yy |\n",
		},
		{
			"centring both columns",
			"| aa | b |\n|:--:|:--:|\n| c | dddd |\n",
			"|  aa |   b  |\n| :-: | :--: |\n|  c  | dddd |\n",
		},
		// The unaligned form must not move: this is the byte pin every
		// existing table fixture depends on.
		{
			"no alignment renders exactly as before",
			"| a |\n| - |\n| b |\n",
			"| a |\n| - |\n| b |\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(Parse([]byte(tc.src)))
			if got != tc.want {
				t.Fatalf("render:\n want %q\n  got %q", tc.want, got)
			}
			if again := Render(Parse([]byte(got))); again != got {
				t.Fatalf("render is not idempotent:\n first %q\nsecond %q", got, again)
			}
		})
	}
}
