package markdown

import "testing"

// TestMailtoPrefixRescuesTheEmailLiteral covers the one shape
// foldDirectivesIntoURLs rescues for the email family: the directive parser
// (priority 800) claims the ':' trigger byte ahead of colonURLParser (999)
// whenever a legal directive name follows, so "mailto:x@y.z" parses as text,
// ':', directive "x", text "@y.z" — a split goldmark's own linkify (which
// never triggers on ':') cannot see across. The "mailto:" word is not a
// recognized URL scheme (see linkifyscheme_test.go's TestBareSchemeSetIsGFM);
// it is incidental prose, and the address right after it links through GFM's
// email-autolink-literal grammar alone.
//
// Measured against the reference (remark-parse 11.0.0 + remark-gfm 4.0.1):
//
//	"see mailto:x@y.z here"     ref  mailto:x@y.z
//	"see mailto:x@y.z. here"    ref  mailto:x@y.z   (trailing "." stays prose)
func TestMailtoPrefixRescuesTheEmailLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, want string }{
		{"see mailto:x@y.z here\n", "mailto:x@y.z"},
		{"see mailto:x@y.z. here\n", "mailto:x@y.z"},
	} {
		t.Run(c.src, func(t *testing.T) {
			t.Parallel()
			if got := linkURLsOf(Parse([]byte(c.src))); !equalStrings(got, []string{c.want}) {
				t.Errorf("link URLs of %q = %v, want %v", c.src, got, []string{c.want})
			}
		})
	}
}

// TestMailtoPrefixDoesNotOverTrigger pins the negative shapes beside the good
// one above: a directive dissolve must not invent a link the reference would
// not form either.
//
//   - A code span's content is verbatim, never prose, so the directive-steal
//     and the email rescue both leave it alone — the address stays inside the
//     span, unlinked.
//   - "x@y" and "x@y.9" are not valid GFM email-literal domains (no dot; a
//     digit-ending final label), the same gate gfmEmailValid already applies
//     to every other bare-email recognizer in this package, so the rescued
//     text does not gain a link either.
// TestEscapedEmailStaysUnlinkedBesideARescuedMailto pins the scoping fix in
// foldDirectivesIntoURLs/dissolveSpannedDirectives: a directive-steal rescue
// elsewhere in the same paragraph must not blanket-rescan the whole sibling
// run for email literals. An escaped address a few words away from a
// genuinely rescued "mailto:" literal must stay exactly what the author
// wrote it as — plain, unlinked text with the escape intact — while the
// rescued literal still links.
//
// Before the fix, relinkifyTexts threaded a single list-wide "emailRescued"
// bool out of foldDirectivesIntoURLs: any rescue anywhere in the paragraph
// gated a blanket splitEmailLiterals pass over every Text node in it,
// including one holding "pat\@example.org" — not involved in the dissolve at
// all — which both stripped the escape and emitted a link the reference
// would never form for an escaped address.
func TestEscapedEmailStaysUnlinkedBesideARescuedMailto(t *testing.T) {
	t.Parallel()
	src := "Contact pat\\@example.org for help. Also see mailto:p@q.rs here.\n"
	got := linkURLsOf(Parse([]byte(src)))
	want := []string{"mailto:p@q.rs"}
	if !equalStrings(got, want) {
		t.Errorf("link URLs of %q = %v, want %v (escaped address must stay unlinked)", src, got, want)
	}
}

func TestMailtoPrefixDoesNotOverTrigger(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"see mailto:`x@y.z` here\n",
		"see mailto:x@y here\n",
		"see mailto:x@y.9 here\n",
	} {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			if got := linkURLsOf(Parse([]byte(src))); len(got) != 0 {
				t.Errorf("link URLs of %q = %v, want none", src, got)
			}
		})
	}
}
