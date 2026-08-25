package confluence

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// Table column alignment through the Confluence bundles. ADF tables have
// no alignment attribute, so MarkdownOptions lowers the delimiter row
// onto the alignment block mark of the blocks in each column (see
// adf.LowerTableAlign) and RenderOptions lifts it back.

// The encode side: a centered and a right-aligned column reach the wire
// as marks, and the synthetic carrier does not.
func TestTableAlignEncodeShape(t *testing.T) {
	doc := mdToADF(t, "| a | b |\n|:-:|--:|\n| 1 | 2 |\n")
	js := docJSON(t, doc)

	if got := strings.Count(js, `{"attrs":{"align":"center"},"type":"alignment"}`); got != 2 {
		t.Errorf("%d center marks, want 2 (header and body):\n%s", got, js)
	}
	if got := strings.Count(js, `{"attrs":{"align":"end"},"type":"alignment"}`); got != 2 {
		t.Errorf("%d end marks, want 2 (header and body):\n%s", got, js)
	}
	if strings.Contains(js, `"align":["center","right"]`) {
		t.Errorf("synthetic table attribute survived encoding:\n%s", js)
	}
	if !adf.IsWireSafe(doc) {
		t.Error("encoded document is not wire-safe")
	}
}

// A table nobody aligned encodes exactly as it did before alignment
// existed, so no existing consumer's payload changes.
func TestTableAlignLeavesAPlainTableAlone(t *testing.T) {
	js := docJSON(t, mdToADF(t, "| a | b |\n| - | - |\n| 1 | 2 |\n"))

	if strings.Contains(js, `"alignment"`) {
		t.Errorf("an unaligned table must carry no mark:\n%s", js)
	}
}

// The acceptance criterion: the two alignments the mark can spell
// survive md → ADF → md through the Confluence bundles.
func TestTableAlignRoundTripsThroughConfluence(t *testing.T) {
	md := "|  a  |  b |\n| :-: | -: |\n|  1  |  2 |\n"

	if got := adfToMD(t, mdToADF(t, md)); got != md {
		t.Errorf("md → adf → md:\n want %q\n  got %q", md, got)
	}
}

// Left is the one alignment the ADF mark cannot spell, so it comes back
// as no alignment at all. The rendered table looks the same; the
// delimiter row does not.
func TestTableAlignLosesLeftThroughConfluence(t *testing.T) {
	got := adfToMD(t, mdToADF(t, "| a  | b |\n| :- | - |\n| 1  | 2 |\n"))

	if strings.Contains(got, ":-") {
		t.Errorf("left alignment cannot survive the ADF mark, got %q", got)
	}
}

// A column the page disagrees about takes the alignment most of its
// cells carry, and the cell out of line keeps its own mark. A delimiter
// row has one alignment per column and no way to say "except this one".
func TestTableAlignTakesTheMajorityOfAConfluenceColumn(t *testing.T) {
	center := func(text string) adf.Node {
		return &adf.TableCell{Content: []adf.Node{&adf.Paragraph{
			Marks:   []adf.Mark{&adf.Alignment{Align: "center"}},
			Content: []adf.Node{&adf.Text{Text: text}},
		}}}
	}
	end := &adf.TableCell{Content: []adf.Node{&adf.Paragraph{
		Marks:   []adf.Mark{&adf.Alignment{Align: "end"}},
		Content: []adf.Node{&adf.Text{Text: "3"}},
	}}}
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Table{Content: []adf.Node{
		&adf.TableRow{Content: []adf.Node{center("a")}},
		&adf.TableRow{Content: []adf.Node{center("1")}},
		&adf.TableRow{Content: []adf.Node{end}},
	}}}}

	if got := adfToMD(t, doc); !strings.Contains(got, ":-:") {
		t.Errorf("want a centered column, got %q", got)
	}
}
