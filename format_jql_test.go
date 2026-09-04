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

// TestJQL_DegradedQueryIsFlatOnBothLegs: a ::jql label is a QUERY STRING,
// so every leg has to read it the same way — as its plain text. The
// complete card already does: the datasource puts ast.PlainText into
// parameters.jql, so "project = **INFRA**" reaches the remote as
// "project = INFRA" whatever the label's inline structure was. The
// degraded leg encoded through EncodeInlines instead, which kept the
// marks, so the same label meant two different things depending on
// whether the attributes happened to be present — and the formatter,
// which mirrors the encode, wrote the flat text and produced a document
// whose ADF no longer matched.
//
// The equality at the end is the one that matters: the formatted document
// is what a push sends, so encode(format(md)) must be encode(md).
//
// Measured with the degraded branch back on
// &adf.Paragraph{Content: ctx.EncodeInlines(n.Children)}:
//
//	--- FAIL: TestJQL_DegradedQueryIsFlatOnBothLegs (0.00s)
//	    --- FAIL: TestJQL_DegradedQueryIsFlatOnBothLegs/strong (0.00s)
//	        ADF diverged:
//	             got: {"type":"doc","content":[{"type":"blockCard","attrs":{"datasource":{"id":"d8b5","parameters":{"cloudId":"abc-123","jql":"project = APIARY"}}}},{"type":"paragraph","content":[{"type":"text","text":"project = "},{"type":"text","marks":[{"type":"strong"}],"text":"INFRA"}]}],"version":1}
//	            want: {"type":"doc","content":[{"type":"blockCard","attrs":{"datasource":{"id":"d8b5","parameters":{"cloudId":"abc-123","jql":"project = APIARY"}}}},{"type":"paragraph","content":[{"type":"text","text":"project = INFRA"}]}],"version":1}
//	    --- FAIL: TestJQL_DegradedQueryIsFlatOnBothLegs/code_span (0.00s)
//	        ADF diverged:
//	             got: {"type":"doc","content":[...,{"type":"paragraph","content":[{"type":"text","text":"project = "},{"type":"text","marks":[{"type":"code"}],"text":"INFRA"}]}],"version":1}
//	            want: {"type":"doc","content":[...,{"type":"paragraph","content":[{"type":"text","text":"project = INFRA"}]}],"version":1}
//	    --- FAIL: TestJQL_DegradedQueryIsFlatOnBothLegs/link (0.00s)
//	        ADF diverged:
//	             got: {"type":"doc","content":[...,{"type":"paragraph","content":[{"type":"text","text":"project = "},{"type":"text","marks":[{"attrs":{"href":"https://e.com/i"},"type":"link"}],"text":"INFRA"}]}],"version":1}
//	            want: {"type":"doc","content":[...,{"type":"paragraph","content":[{"type":"text","text":"project = INFRA"}]}],"version":1}
//	    --- FAIL: TestJQL_DegradedQueryIsFlatOnBothLegs/two_marks_and_a_gap (0.00s)
//	        ADF diverged:
//	             got: {"type":"doc","content":[...,{"type":"paragraph","content":[{"type":"text","text":"project = "},{"type":"text","marks":[{"type":"em"}],"text":"INFRA"},{"type":"text","text":" and x = "},{"type":"text","marks":[{"type":"strike"}],"text":"y"}]}],"version":1}
//	            want: {"type":"doc","content":[...,{"type":"paragraph","content":[{"type":"text","text":"project = INFRA and x = y"}]}],"version":1}
//
// The blockCard is elided as "..." in all but the first subtest above; it
// is the good card, identical in every one of them and in both columns.
//
// The "plain query" subtest passed on both versions and is the good case:
// a label with no inline structure must keep degrading to exactly the
// text it always did, so a fix that reached for the label's source bytes
// or dropped the paragraph fails there.
func TestJQL_DegradedQueryIsFlatOnBothLegs(t *testing.T) {
	for _, tc := range []struct {
		name, label, query string
	}{
		{"plain query", "project = INFRA", "project = INFRA"},
		{"strong", "project = **INFRA**", "project = INFRA"},
		{"code span", "project = `INFRA`", "project = INFRA"},
		{"link", "project = [INFRA](https://e.com/i)", "project = INFRA"},
		{"two marks and a gap", "project = *INFRA* and x = ~~y~~", "project = INFRA and x = y"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One document holding BOTH: the complete card, whose label
			// the datasource already flattens, and the incomplete one.
			md := jqlCard + "\n\n::jql[" + tc.label + "]\n"

			var diags []convert.Diagnostic
			got := mdToADF(md, WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

			assertSameADF(t, doc(
				&adf.BlockCard{Datasource: map[string]any{
					"id":         "d8b5",
					"parameters": map[string]any{"cloudId": "abc-123", "jql": "project = APIARY"},
				}},
				p(txt(tc.query)),
			), got)

			// The diagnostic names the query the same flat way, so a
			// reader is told what actually traveled.
			if len(diags) != 1 || diags[0].Code != convert.CodeJQLDegraded {
				t.Fatalf("want one %s diagnostic, got %+v", convert.CodeJQLDegraded, diags)
			}
			if !strings.Contains(diags[0].Message, tc.query) {
				t.Errorf("diagnostic does not name the query %q: %q", tc.query, diags[0].Message)
			}

			// The formatter writes the flat query, and its output must
			// encode to the ADF the unformatted document encoded to.
			formatted := fmtMD(md)
			if want := jqlCard + "\n\n" + tc.query + "\n"; formatted != want {
				t.Errorf("format:\n got %q\nwant %q", formatted, want)
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
