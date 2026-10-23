package adfast

import "testing"

// TestADFOriginBacktickStillGetsTheDefensiveEscape is the other half of the
// bare-backtick fix (see markdown.PreservedEscapes and
// markdown/parse_backtick_escape_test.go): a bare backtick keeps the source
// spelling on format ONLY when the parse itself saw it bare. A value that
// arrives by any other route — ADF, here — carries no such signal and must
// still be escaped defensively, or the next parse reads an unpaired backtick
// as the start of a code span.
//
// mdToADF followed by FromADF is the route that actually produces that
// origin: ToADF's VisitText (convert/ast_to_adf.go) reads only n.Value, never
// Raw, so a markdown parse's "I saw this bare" signal never reaches the ADF
// node, and FromADF (convert/adf_to_ast.go) builds a fresh *ast.Text from
// that decoded value alone — Raw empty, exactly like a hand-built node or one
// Jira's own editor produced directly.
func TestADFOriginBacktickStillGetsTheDefensiveEscape(t *testing.T) {
	doc := mdToADF("a`b\n")

	got := ToMarkdown(FromADF(doc), WithPrettierFormat())
	want := "a\\`b\n"
	if got != want {
		t.Fatalf("format(FromADF(doc)) = %q, want %q", got, want)
	}

	// The escape is load-bearing, not cosmetic: a re-parse of the bare form
	// would read "a`b" as an unterminated code-span opener and the ADF value
	// would not survive even one more round trip without it.
	again := adfToMD(mdToADF(got), WithPrettierFormat())
	if again != want {
		t.Errorf("escape did not round-trip: adfToMD(mdToADF(%q)) = %q, want %q", got, again, want)
	}
}
