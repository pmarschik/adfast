package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// Table column alignment through the Jira bundles. ADF tables have no
// alignment attribute, so MarkdownOptions lowers the delimiter row onto
// the alignment block mark of the blocks in each column (see
// adf.LowerTableAlign) and RenderOptions lifts it back.

// jiraMdToADF converts markdown with the Jira encode bundle.
func jiraMdToADF(t *testing.T, md string) adf.Doc {
	t.Helper()
	opts := MarkdownOptions("https://x.atlassian.net", ExpandExplicit)
	return adfast.ToADF(adfast.FromMarkdown(md, opts...), opts...)
}

// jiraADFToMD renders a document with the Jira decode bundle.
func jiraADFToMD(t *testing.T, doc adf.Doc) string {
	t.Helper()
	opts := RenderOptions()
	return adfast.ToMarkdown(adfast.FromADF(doc, opts...), opts...)
}

// jiraDocJSON is the document's ADF JSON, for shape assertions.
func jiraDocJSON(t *testing.T, doc adf.Doc) string {
	t.Helper()
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(js)
}

// The encode side: the delimiter row reaches the wire as marks, and the
// synthetic carrier does not.
func TestTableAlignEncodeShape(t *testing.T) {
	doc := jiraMdToADF(t, "| a | b |\n|:-:|--:|\n| 1 | 2 |\n")
	js := jiraDocJSON(t, doc)

	if got := strings.Count(js, `{"attrs":{"align":"center"},"type":"alignment"}`); got != 2 {
		t.Errorf("%d center marks, want 2 (header and body):\n%s", got, js)
	}
	if got := strings.Count(js, `{"attrs":{"align":"end"},"type":"alignment"}`); got != 2 {
		t.Errorf("%d end marks, want 2 (header and body):\n%s", got, js)
	}
	if !adf.IsWireSafe(doc) {
		t.Error("encoded document is not wire-safe")
	}
}

// A table nobody aligned encodes exactly as it did before alignment
// existed, so no existing consumer's payload changes.
func TestTableAlignLeavesAPlainTableAlone(t *testing.T) {
	js := jiraDocJSON(t, jiraMdToADF(t, "| a | b |\n| - | - |\n| 1 | 2 |\n"))

	if strings.Contains(js, `"alignment"`) {
		t.Errorf("an unaligned table must carry no mark:\n%s", js)
	}
}

// The acceptance criterion: the two alignments the mark can spell
// survive md → ADF → md through the Jira bundles.
func TestTableAlignRoundTripsThroughJira(t *testing.T) {
	md := "|  a  |  b |\n| :-: | -: |\n|  1  |  2 |\n"

	if got := jiraADFToMD(t, jiraMdToADF(t, md)); got != md {
		t.Errorf("md → adf → md:\n want %q\n  got %q", md, got)
	}
}
