package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

func spanTable(t *testing.T) adf.Doc {
	t.Helper()
	header := func(text string) adf.Node {
		return &adf.TableHeader{Content: []adf.Node{
			&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: text}}},
		}}
	}
	cell := func(text string, colspan, rowspan int) adf.Node {
		return &adf.TableCell{Colspan: colspan, Rowspan: rowspan, Content: []adf.Node{
			&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: text}}},
		}}
	}
	return adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Table{
		Content: []adf.Node{
			&adf.TableRow{Content: []adf.Node{
				header("A"),
				header("B"),
				header("C"),
			}},
			&adf.TableRow{Content: []adf.Node{
				cell("wide", 2, 0),
				cell("tall", 0, 2),
			}},
			&adf.TableRow{Content: []adf.Node{
				cell("x", 0, 0),
				cell("y", 0, 0),
			}},
		},
	}}}
}

// spanTableCell returns the typed table cell at the given row/column.
func spanTableCell(t *testing.T, table adf.Node, row, col int) *adf.TableCell {
	t.Helper()
	rows := adf.NodeContent(table)
	cells := adf.NodeContent(rows[row])
	cell, ok := cells[col].(*adf.TableCell)
	if !ok {
		t.Fatalf("cell %d/%d is %T", row, col, cells[col])
	}
	return cell
}

func TestTableSpans_RenderAndRoundTrip(t *testing.T) {
	doc := spanTable(t)
	md := adfToMD(doc)
	for _, want := range []string{"| > | wide | tall |", "| x | y    | ^    |"} {
		if !strings.Contains(md, want) {
			t.Fatalf("render missing %q:\n%s", want, md)
		}
	}

	back := mdToADF(md)
	again := adfToMD(back)
	if again != md {
		t.Errorf("round trip unstable:\nfirst:  %q\nsecond: %q", md, again)
	}

	// The spans survive into ADF.
	table := back.Content[0]
	if got := spanTableCell(t, table, 1, 0).Colspan; got != 2 {
		t.Errorf("colspan: %d", got)
	}
	if got := spanTableCell(t, table, 1, 1).Rowspan; got != 2 {
		t.Errorf("rowspan: %d", got)
	}
	if cells := adf.NodeContent(adf.NodeContent(table)[2]); len(cells) != 2 {
		t.Errorf("continuation row cells: %d", len(cells))
	}
}

func TestTableSpans_LiteralMarkersEscape(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Table{
		Content: []adf.Node{
			&adf.TableRow{Content: []adf.Node{
				&adf.TableHeader{Content: []adf.Node{&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: ">"}}}}},
				&adf.TableHeader{Content: []adf.Node{&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "^"}}}}},
			}},
		},
	}}}
	md := adfToMD(doc)
	if !strings.Contains(md, `\>`) || !strings.Contains(md, `\^`) {
		t.Fatalf("literal markers must escape: %q", md)
	}
	back := mdToADF(md)
	row := adf.NodeContent(back.Content[0])[0]
	cells := adf.NodeContent(row)
	if len(cells) != 2 {
		t.Fatalf("cells: %d", len(cells))
	}
	if txt := adf.NodeText(adf.NodeContent(adf.NodeContent(cells[0])[0])[0]); txt != ">" {
		t.Errorf("literal > lost: %q", txt)
	}
}

// The md→adf direction of the literal-marker escape (the inverse of
// TestTableSpans_LiteralMarkersEscape): "\>" / "\^" cells parse as
// literal text — no merge, matching remark-extended-table — and
// round-trip stably through ADF.
func TestTableSpans_EscapedMarkersParseLiteral(t *testing.T) {
	md := "| a | b |\n| - | - |\n| \\> | \\^ |\n"
	doc := mdToADF(md)
	rows := adf.NodeContent(doc.Content[0])
	body := adf.NodeContent(rows[1])
	if len(body) != 2 {
		t.Fatalf("escaped markers must stay cells, got %d", len(body))
	}
	for i, want := range []string{">", "^"} {
		cell := adf.NodeContent(body[i])
		if txt := adf.NodeText(adf.NodeContent(cell[0])[0]); txt != want {
			t.Errorf("cell %d text = %q, want %q", i, txt, want)
		}
	}
	if cell, ok := body[0].(*adf.TableCell); !ok || cell.Colspan > 1 || cell.Rowspan > 1 {
		t.Errorf("escaped marker must not merge: %+v", body[0])
	}
	first := adfToMD(doc)
	if want := "| a  | b  |\n| -- | -- |\n| \\> | \\^ |\n"; first != want {
		t.Errorf("render = %q, want %q", first, want)
	}
	if second := adfToMD(mdToADF(first)); second != first {
		t.Errorf("round-trip not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestTableSpans_ColwidthsInterplay(t *testing.T) {
	// ::colwidths assigns one width per VISUAL column: the colspan cell
	// carries two widths; the row after a rowspan skips the covered column.
	md := "::colwidths[100,200,300]\n\n" +
		"| A | B | C |\n| - | - | - |\n| > | wide | tall |\n| x | y | ^ |\n"
	doc := mdToADF(md)
	table := doc.Content[0]
	if got := spanTableCell(t, table, 1, 0).Colwidth; !equalWidths(got, 100, 200) {
		t.Errorf("colspan widths: %v", got)
	}
	if got := spanTableCell(t, table, 1, 1).Colwidth; !equalWidths(got, 300) {
		t.Errorf("rowspan cell widths: %v", got)
	}
	if got := spanTableCell(t, table, 2, 1).Colwidth; !equalWidths(got, 200) {
		t.Errorf("continuation second cell widths: %v", got)
	}

	// Round trip keeps directive and spans stable.
	out := adfToMD(doc)
	if again := adfToMD(mdToADF(out)); again != out {
		t.Errorf("unstable:\nfirst:  %q\nsecond: %q", out, again)
	}
}

func equalWidths(arr []float64, want ...float64) bool {
	if len(arr) != len(want) {
		return false
	}
	for i, w := range want {
		if arr[i] != w {
			return false
		}
	}
	return true
}

func TestTableSpans_UnresolvableMarkersStayLiteral(t *testing.T) {
	// "^" in the first row has nothing above it; trailing ">" has no
	// following cell.
	md := "| ^ | b |\n| - | - |\n| c | > |\n"
	doc := mdToADF(md)
	rows := adf.NodeContent(doc.Content[0])
	if len(adf.NodeContent(rows[0])) != 2 || len(adf.NodeContent(rows[1])) != 2 {
		t.Fatalf("unexpected structure: %+v", rows)
	}
}

// spanDiagnostics converts md and returns the span-marker-invalid
// diagnostic messages.
func spanDiagnostics(md string) []string {
	var msgs []string
	mdToADF(md, WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeSpanMarkerInvalid {
			msgs = append(msgs, d.Message)
		}
	}))
	return msgs
}

// Every position where a span marker's merge cannot apply reports a
// span-marker-invalid diagnostic naming the marker's row and column;
// the marker keeps its literal-text fallback.
func TestTableSpans_InvalidPositionDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want []string
	}{
		{
			// A ">" run at the end of a row has no content cell to absorb it.
			name: "colspan marker in the last column",
			md:   "| a | b |\n| - | - |\n| c | > |\n",
			want: []string{`">" at row 2, column 2`},
		},
		{
			// A "^" in the header row has no row above it.
			name: "rowspan marker in the header row",
			md:   "| ^ | b |\n| - | - |\n| c | d |\n",
			want: []string{`"^" at row 1, column 1`},
		},
		{
			// A ">" run interrupted by "^" cannot attach to a content cell;
			// the whole pending run reverts. (The "^" itself still merges —
			// the header cell above spans its column.)
			name: "colspan run interrupted by rowspan marker",
			md:   "| a | b |\n| - | - |\n| > | ^ |\n",
			want: []string{`">" at row 2, column 1`},
		},
		{
			name: "valid spans report nothing",
			md:   "| a | b |\n| - | - |\n| > | wide |\n| ^ | x |\n",
			want: nil,
		},
		{
			// Unlike remark-extended-table, a "^" in the first body row
			// extends the HEADER cell above (ADF headers carry rowspan and
			// must round-trip) — the merge applies, so no diagnostic.
			name: "rowspan into the header row is a valid merge",
			md:   "| a | b |\n| - | - |\n| ^ | x |\n",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := spanDiagnostics(tt.md)
			if len(msgs) != len(tt.want) {
				t.Fatalf("diagnostics = %q, want %d entries %q", msgs, len(tt.want), tt.want)
			}
			for i, frag := range tt.want {
				if !strings.Contains(msgs[i], frag) {
					t.Errorf("diagnostic %d = %q, want fragment %q", i, msgs[i], frag)
				}
			}
		})
	}
}

// TestTableDelimiterAmbiguousCellIdempotent guards the round-trip fixpoint
// for a table cell whose content is only dashes/colons. A header-only table
// emits its delimiter row bare ("| - |"); when two such tables render
// adjacently they merge on re-parse (GFM concatenates the blocks) so the
// second delimiter row becomes a body cell. That cell must render bare too —
// escaping it to "| \- |" (and widening the column) makes adfToMD∘mdToADF
// non-idempotent. See bareDelimiterAmbiguousCell.
func TestTableDelimiterAmbiguousCellIdempotent(t *testing.T) {
	// The reported repro: adjacent header-only tables inside a list item.
	md := "*\n      0\n  0\n--\n\n  0\n-- "
	first := adfToMD(mdToADF(md))
	second := adfToMD(mdToADF(first))
	if first != second {
		t.Errorf("round-trip not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
	want := "- ```\n  0\n  ```\n  | 0 |\n  | - |\n  | 0 |\n  | - |\n"
	if first != want {
		t.Errorf("first render = %q, want %q", first, want)
	}

	// A genuine table with a lone-dash body cell renders bare and is stable.
	for _, md := range []string{
		"| h |\n| - |\n| - |\n",
		"| h |\n| - |\n| -- |\n",
		"| h |\n| - |\n| -: |\n",
	} {
		r1 := adfToMD(mdToADF(md))
		r2 := adfToMD(mdToADF(r1))
		if r1 != r2 {
			t.Errorf("dash-body table unstable for %q:\nr1: %q\nr2: %q", md, r1, r2)
		}
		if strings.Contains(r1, `\-`) {
			t.Errorf("dash-only cell should render bare, got %q", r1)
		}
	}
}
