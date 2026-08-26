package adfast

import (
	"strings"
	"testing"
)

// ast.PlainText used to have no case for a *ast.TextDirective node, so an
// unregistered inline directive (a bare ":name", or a colon landing
// mid-word like "Over:view") contributed the empty string instead of its
// literal form.
//
// This test pins the leg the bug did NOT reach: FormatMarkdown
// (ToMarkdown(FromMarkdown(…, WithPrettierFormat()))) is a pure
// md→ast→md pass, and by the time it reaches render_inline.go's
// writeImage the "prettier" pipeline has already run the document through
// convert/normalize.go's flattener, whose own TextDirective case (line
// ~275) rebuilds ":view" as literal text nodes before writeImage's
// ast.PlainText(node.Children) ever sees a TextDirective node. Confirmed
// by mutation: reverting the ast.PlainText fix left this test passing.
// Kept anyway as a straightforward coherence pin — it is the md→ADF→md
// leg below that the fix actually changes.
func TestImageAltWithDirectiveColonSurvivesFormat(t *testing.T) {
	md := "![Over:view](x.png)\n"
	once := fmtMD(md)
	if once != md {
		t.Fatalf("format truncated the alt text:\n in:  %q\n out: %q", md, once)
	}
	if twice := fmtMD(once); twice != once {
		t.Fatalf("not idempotent:\n once:  %q\n twice: %q", once, twice)
	}
}

// This pins the md→ADF leg directly: an inline image with no asset store
// degrades to a link (see TestExternalInlineImageDegradesToALink), and the
// link text is the alt, built by convert/ast_to_adf.go through
// ast.PlainText(img.Children). Before the fix the link text was "Over",
// dropping everything from the colon on. Confirmed by mutation: reverting
// the ast.PlainText fix turns this red.
func TestImageAltWithDirectiveColonSurvivesADF(t *testing.T) {
	got := adfJSON(t, mdToADF("see ![Over:view](https://x/y.png) here\n"))

	want := `{"type":"text","marks":[{"attrs":{"href":"https://x/y.png"},"type":"link"}],"text":"Over:view"}`
	if !strings.Contains(got, want) {
		t.Errorf("want the alt text preserved whole in the degraded link\n  %s\ngot\n  %s", want, got)
	}
}

// This pins the actual bug the bead measured: a full md→ADF→md round trip
// (not the pure formatter above) truncates the alt at the colon, because
// building the ADF loses the ":view" half and the markdown that comes back
// out of that ADF only ever had "Over" to work with. A block-position image
// (its own paragraph, not surrounded by text) takes the mediaSingle/media
// path rather than the inline degraded-link path exercised above, so this
// also pins that second ADF shape. Confirmed by mutation: reverting the
// ast.PlainText fix turns this red ("Over:view" -> "Over").
func TestImageAltWithDirectiveColonSurvivesADFRoundTrip(t *testing.T) {
	md := "![Over:view](https://x/y.png)\n"
	doc := mdToADF(md)
	if got := adfToMD(doc); got != md {
		t.Errorf("round trip truncated the alt text:\n in:  %q\n out: %q", md, got)
	}
}
