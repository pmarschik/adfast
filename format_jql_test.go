package adfast

import "testing"

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
