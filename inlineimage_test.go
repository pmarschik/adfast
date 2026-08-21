package adfast

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// adfJSON renders a converted document as compact JSON, so a test can pin the
// exact inline shape an image became.
func adfJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

// The bug this file exists for: an inline image used to vanish from the ADF
// payload with nothing said about it. Whatever an inline image becomes now, the
// text around it must never lose the image itself in silence.
func TestInlineImageIsNotSilentlyDropped(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF("before ![alt](https://x/y.png) after\n",
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	if !strings.Contains(got, "https://x/y.png") {
		t.Errorf("inline image URL lost from the payload:\n%s", got)
	}
	for _, want := range []string{`"before "`, `" after"`} {
		if !strings.Contains(got, want) {
			t.Errorf("surrounding text %s lost:\n%s", want, got)
		}
	}
	if !contains(codes, convert.CodeInlineImageDegraded) {
		t.Errorf("want a %s diagnostic, got %v", convert.CodeInlineImageDegraded, codes)
	}
}

// An external inline image has no faithful ADF form, so it degrades to the link
// it can still be: the alt text labels it and the image URL is the href.
func TestExternalInlineImageDegradesToALink(t *testing.T) {
	got := adfJSON(t, mdToADF("see ![the shot](https://x/y.png) here\n"))

	want := `{"type":"text","marks":[{"attrs":{"href":"https://x/y.png"},"type":"link"}],"text":"the shot"}`
	if !strings.Contains(got, want) {
		t.Errorf("want the degraded link\n  %s\ngot\n  %s", want, got)
	}
}

// With no alt text there is still a label to show: the filename the URL ends
// in, the same fallback a file card's label uses.
func TestExternalInlineImageWithoutAltLabelsWithTheFilename(t *testing.T) {
	got := adfJSON(t, mdToADF("see ![](https://x/y/shot.png) here\n"))

	if !strings.Contains(got, `"text":"shot.png"`) {
		t.Errorf("want the filename as the degraded label, got:\n%s", got)
	}
}

// The degradation has to be stable, or the round-trip idempotence invariant
// breaks: converting the degraded markdown again must not change it further.
func TestExternalInlineImageDegradationIsStable(t *testing.T) {
	once := roundTrip(t, "before ![alt](https://x/y.png) after\n")
	twice := roundTrip(t, once)

	if once != twice {
		t.Errorf("degradation is not idempotent:\n first: %q\nsecond: %q", once, twice)
	}
	if !strings.Contains(once, "[alt](https://x/y.png)") {
		t.Errorf("want a link in the degraded markdown, got %q", once)
	}
}

// roundTrip runs the full md → adf → md route the fuzzer pins.
func roundTrip(t *testing.T, md string) string {
	t.Helper()
	return ToMarkdown(FromADF(ToADF(FromMarkdown(md))))
}

// The case the upstream issue actually asked for: a path the asset store knows
// has a media id, so it becomes a real mediaInline rather than being dropped.
func TestStoreResolvableInlineImageBecomesMediaInline(t *testing.T) {
	got := adfJSON(t, mdToADF("see ![the shot](assets/shot.png) here\n",
		WithAssetIDResolver(func(ref string) (string, bool) {
			if ref == "assets/shot.png" {
				return "abc-123", true
			}
			return "", false
		})))

	want := `{"type":"mediaInline","attrs":{"alt":"the shot","collection":"","id":"abc-123","type":"file"}}`
	if !strings.Contains(got, want) {
		t.Errorf("want\n  %s\ngot\n  %s", want, got)
	}
}

// An inline image inside a table cell used to leave the cell empty. It is the
// same conversion, so the cell must keep its content too.
func TestInlineImageInATableCellKeepsTheCellContent(t *testing.T) {
	got := adfJSON(t, mdToADF("| a |\n| - |\n| ![alt](https://x/y.png) |\n"))

	if strings.Contains(got, `{"type":"tableCell","content":[{"type":"paragraph"}]}`) {
		t.Errorf("table cell collapsed to an empty paragraph:\n%s", got)
	}
	if !strings.Contains(got, "https://x/y.png") {
		t.Errorf("cell lost the image URL:\n%s", got)
	}
}

// A local path the store cannot map keeps the documented behavior: dropped,
// but reported, so an upload flow can pick the asset up.
func TestUnresolvableLocalInlineImageStillReportsUnresolvedAsset(t *testing.T) {
	var codes []string
	mdToADF("see ![alt](assets/missing.png) here\n",
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) }))

	if !contains(codes, convert.CodeUnresolvedAsset) {
		t.Errorf("want a %s diagnostic, got %v", convert.CodeUnresolvedAsset, codes)
	}
}

// The decode side mirrors the block mediaAsImage: an attachment the store has
// the file for reads back as the inline image it came from, so a store-aware
// round trip is symmetric.
func TestMediaInlineReadsBackAsAnInlineImageWhenStoreAware(t *testing.T) {
	md := "see ![the shot](assets/shot.png) here\n"
	adfDoc := ToADF(FromMarkdown(md), WithAssetIDResolver(
		func(ref string) (string, bool) { return "abc-123", ref == "assets/shot.png" }))

	got := adfToMD(adfDoc, WithMediaAssets(map[string]convert.MediaAsset{
		"abc-123": {Path: "assets/shot.png", Width: 8, Height: 4, HasDim: true},
	}))

	if got != md {
		t.Errorf("store-aware round trip is not symmetric:\n want %q\n  got %q", md, got)
	}
}

// Without the store there is no path to render, so the card keeps the directive
// surface rather than inventing one.
func TestMediaInlineKeepsTheDirectiveWithoutTheStore(t *testing.T) {
	adfDoc := ToADF(FromMarkdown("see ![the shot](assets/shot.png) here\n"),
		WithAssetIDResolver(func(string) (string, bool) { return "abc-123", true }))

	got := adfToMD(adfDoc)

	if strings.Contains(got, "![") {
		t.Errorf("unresolved mediaInline must not render as an image, got %q", got)
	}
	if !strings.Contains(got, ":media") {
		t.Errorf("unresolved mediaInline must stay a directive, got %q", got)
	}
}

func contains(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}
