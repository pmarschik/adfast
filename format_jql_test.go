package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// TestFormatMarkdown_JQLPreserved: the style-preserving formatter must
// not drop a ::jql block — the synthetic mdGapBefore marker the
// formatter stores on the blockCard must not make the datasource shape
// inexpressible (regression: the strict Extra check refused promotion
// and the URL-less raw blockCard was dropped).
func TestFormatMarkdown_JQLPreserved(t *testing.T) {
	md := "before\n\n::jql[project = APIARY]{cloudId=\"abc-123\" datasource=\"d8b5\"}\n\nafter\n"
	if got := fmtMD(md); got != md {
		t.Errorf("format dropped or altered the jql block:\n got %q\nwant %q", got, md)
	}
	// Leading position (no gap marker) keeps working too.
	solo := "::jql[project = APIARY]{cloudId=\"abc-123\" datasource=\"d8b5\"}\n"
	if got := fmtMD(solo); got != solo {
		t.Errorf("solo jql: got %q", got)
	}
}

// jqlCard is the complete directive that DOES encode — the good case
// every subtest below carries alongside the incomplete one, so a fix for
// the degradation cannot quietly break the shape that already worked.
const jqlCard = "::jql[project = APIARY]{cloudId=\"abc-123\" datasource=\"d8b5\"}"

// TestJQL_IncompleteDegradesToQueryText: an ADF JQL-datasource blockCard
// addresses its datasource by cloudId + datasource id, so a ::jql
// directive missing either has no card to become. It must not vanish:
// the QUERY degrades to a paragraph and a jql-degraded diagnostic
// reports the lost live-table intent (regression: EncodeADF returned nil
// for the incomplete shape, so a ::jql lost its query on a round trip —
// silently, with the document left holding an empty paragraph).
func TestJQL_IncompleteDegradesToQueryText(t *testing.T) {
	for _, tc := range []struct {
		name string
		bad  string
	}{
		{"no attrs at all", "::jql[project = INFRA]"},
		{"cloudId without datasource", "::jql[project = INFRA]{cloudId=\"abc-123\"}"},
		{"datasource without cloudId", "::jql[project = INFRA]{datasource=\"d8b5\"}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One document holding BOTH: the complete card must survive
			// untouched while the incomplete one degrades beside it.
			md := jqlCard + "\n\n" + tc.bad + "\n"

			var diags []convert.Diagnostic
			got := mdToADF(md, WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

			// Differential against the AST: the good directive still
			// encodes to its datasource card, the bad one to a paragraph
			// carrying the query verbatim.
			assertSameADF(t, doc(
				&adf.BlockCard{Datasource: map[string]any{
					"id":         "d8b5",
					"parameters": map[string]any{"cloudId": "abc-123", "jql": "project = APIARY"},
				}},
				p(txt("project = INFRA")),
			), got)

			// Exactly one report, naming the query so a reader can find
			// the directive it is about.
			if len(diags) != 1 || diags[0].Code != convert.CodeJQLDegraded {
				t.Fatalf("want one %s diagnostic, got %+v", convert.CodeJQLDegraded, diags)
			}
			if want := "project = INFRA"; !strings.Contains(diags[0].Message, want) {
				t.Errorf("diagnostic does not name the query %q: %q", want, diags[0].Message)
			}

			// The round trip keeps the query as prose rather than
			// emitting a bare newline, and the good card comes back whole.
			back := adfToMD(got)
			if want := jqlCard + "\n\nproject = INFRA\n"; back != want {
				t.Errorf("round trip lost the query:\n got %q\nwant %q", back, want)
			}
			assertMdStable(t, md)

			// The prettier formatter mirrors the encode: it rewrites the
			// incomplete directive to the same prose instead of erasing
			// the line, and its output encodes to the same ADF.
			formatted := fmtMD(md)
			if want := jqlCard + "\n\nproject = INFRA\n"; formatted != want {
				t.Errorf("format lost the query:\n got %q\nwant %q", formatted, want)
			}
			assertSameADF(t, got, mdToADF(formatted))
		})
	}
}

// TestJQL_EmptyQueryDropsLoudly: a complete-looking directive with an
// EMPTY label has no query text to keep, so it is the one ::jql that
// still drops. It must not drop in silence — the diagnostic is the only
// trace left of it.
func TestJQL_EmptyQueryDropsLoudly(t *testing.T) {
	md := jqlCard + "\n\n::jql[]{cloudId=\"abc-123\" datasource=\"d8b5\"}\n"

	var diags []convert.Diagnostic
	got := mdToADF(md, WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

	// The good card survives; the empty one contributes no node at all.
	assertSameADF(t, doc(
		&adf.BlockCard{Datasource: map[string]any{
			"id":         "d8b5",
			"parameters": map[string]any{"cloudId": "abc-123", "jql": "project = APIARY"},
		}},
	), got)

	if len(diags) != 1 || diags[0].Code != convert.CodeJQLDegraded {
		t.Fatalf("want one %s diagnostic, got %+v", convert.CodeJQLDegraded, diags)
	}
	if !strings.Contains(diags[0].Message, "no query") {
		t.Errorf("diagnostic should say the query is missing too: %q", diags[0].Message)
	}
}
