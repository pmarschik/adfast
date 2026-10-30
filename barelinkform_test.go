package adfast

import (
	"strings"
	"testing"
	"time"
)

// A BARE LINK HAS TWO SPELLINGS, ONE PER RENDER LEG, AND THE TWO LEGS NOW
// DELIBERATELY AGREE. That is the whole content of this file.
//
//   - THE ADF LEG (adfToMD, the pull path: ADF -> markdown) carries no "was it
//     written in angle brackets" bit — a link mark is a URL and some text —
//     so a link whose text equals its URL has exactly one spelling available,
//     chosen by convert's ast.Link construction (Bare: true), not read off
//     the source.
//   - THE FORMAT LEG (fmtMD, md -> md with WithPrettierFormat) never touches
//     ADF, so it still has the source form on ast.Link.Bare, and preserves
//     whatever the author wrote: an angle source stays angle, a bare literal
//     stays bare.
//
// DECIDED 2026-10-22: the ADF leg used to answer to
// mdast-util-to-markdown and write the angle form here, which put it at odds
// with the format leg (which writes bare for an unauthored bare URL) on the
// same input — a user who formats then pushes got different bytes than one
// who pushed directly. With no frozen reference left to defer to, the two
// legs were made to agree instead: the ADF leg now writes bare too, for
// INTERNAL CONSISTENCY. The cost, accepted deliberately rather than
// overlooked: an angle autolink the author actually wrote, taken through
// ADF and back, now comes out bare as well — ADF has nowhere to keep the
// brackets, so round-tripping through it was always going to pick one
// spelling for both; this file records that the pick is now bare, not
// angle.
//
// The bare preference has one limit: bare is written only where it re-parses
// as the same link between the bytes it actually touches ("x<http://a.b>"
// cannot go bare as "xhttp://a.b"). Where it would not, the ADF leg keeps the
// angle form — see TestBareLinkFromADFKeepsAngleWhenBareWouldNotRoundTrip.
//
// The "www." rows are untouched by any of this: there the URL and the text
// DIFFER (href "http://www.ex.com", text "www.ex.com"), so neither the bare
// nor the angle autolink form can express the node at all, and the ADF leg
// still writes the full [label](url) regardless of Bare.
func TestBareLinkFromADFWritesBareForInternalConsistency(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		{"bare literal in the source", "http://ex.com", "http://ex.com\n"},
		// ADF cannot keep the angle spelling across the hop — see the
		// package comment above. This is the accepted cost, not a miss.
		{"angle form in the source, lost across the ADF hop", "<http://ex.com>", "http://ex.com\n"},
		{"a path keeps the same (now bare) form", "https://ex.com/x", "https://ex.com/x\n"},
		{"an uppercase scheme keeps the same (now bare) form", "HTTP://ex.com", "HTTP://ex.com\n"},
		{"a dotless host", "http://0", "http://0\n"},
		{"www literal: href and text differ", "www.ex.com", "[www.ex.com](http://www.ex.com)\n"},
		{"www literal with a numeric label", "www.0.a", "[www.0.a](http://www.0.a)\n"},
		// A custom scheme is angle-autolink valid but not GFM
		// bare-literal valid (isBareAutolinkScheme in convert/
		// adf_to_ast.go restricts Bare to http(s)/mailto for exactly
		// this reason): writing it bare would hand back plain text no
		// reparse turns back into a link, which is what the fuzz
		// corpus "!<Ad:><Ad:>0" caught before this restriction
		// existed. It must keep the angle form here.
		{"a custom scheme keeps the angle form, not bare", "<tel:123>", "<tel:123>\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := adfToMD(mdToADF(c.md)); got != c.want {
				t.Errorf("adfToMD(mdToADF(%q)) = %q, want %q (the ADF leg now "+
					"writes bare, matching the format leg, for a link whose "+
					"text equals its URL)",
					c.md, got, c.want)
			}
		})
	}
}

// TestMailtoLinkMarkTextDecidesTheBareForm is the end-to-end proof behind
// isBareAutolinkScheme's split handling of mailto: unlike http(s), where
// text-equals-href is the only question that matters, a mailto link mark's
// safe-to-render-bare answer also depends on WHICH text the mark carries,
// so it needs ADF built by hand rather than mdToADF — mdToADF can never
// produce the "text is the full mailto: URL" shape below, because goldmark
// itself does not parse that literal as a link to begin with (this is the
// same fact isBareAutolinkScheme's doc comment relies on).
//
// Both rows arrive as ADF a Jira/Confluence editor could plausibly have
// produced directly (a link mark pasted or typed in rich text, not through
// our own markdown parser), which is exactly the shape mdToADF cannot
// reach and this test exists to cover instead.
func TestMailtoLinkMarkTextDecidesTheBareForm(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, text, want string
	}{
		// The text is the bare address only: convertGoldmarkAutoLink's own
		// parse auto-prepends "mailto:" to a bare email, so writing just the
		// address back out round-trips to the same href.
		{"text is the bare address", "a@ex.com", "a@ex.com\n"},
		// The text is the FULL "mailto:" URL: goldmark's linkify never
		// matches this as any kind of autolink on reparse (its email scan
		// breaks on the ':' before ever reaching '@'), so the bare form
		// would silently drop the link. It must keep the angle form.
		{"text is the full mailto: URL", "mailto:a@ex.com", "<mailto:a@ex.com>\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(`{"version":1,"type":"doc","content":[{"type":"paragraph","content":[` +
				`{"type":"text","text":"` + c.text + `","marks":[{"type":"link","attrs":{"href":"mailto:a@ex.com"}}]}` +
				`]}]}`)
			if got := adfToMD(doc); got != c.want {
				t.Errorf("adfToMD(link text %q, href \"mailto:a@ex.com\") = %q, want %q",
					c.text, got, c.want)
			}
		})
	}
}

// TestPrettierFormatLegKeepsTheSourceAutolinkForm is the other leg of the pin
// above. Nothing here goes through ADF, so ast.Link.Bare still holds the form
// the parse saw and prettier's rule — print what the author wrote — applies.
func TestPrettierFormatLegKeepsTheSourceAutolinkForm(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		{"bare literal stays bare", "http://ex.com", "http://ex.com\n"},
		{"angle form stays angle", "<http://ex.com>", "<http://ex.com>\n"},
		{"bare literal with a path stays bare", "https://ex.com/x", "https://ex.com/x\n"},
		{"uppercase scheme stays bare", "HTTP://ex.com", "HTTP://ex.com\n"},
		{"dotless host stays bare", "http://0", "http://0\n"},
		{"dotless host in angles stays angled", "<http://0>", "<http://0>\n"},
		{"www literal stays bare", "www.ex.com", "www.ex.com\n"},
		{"an explicit label is untouched", "[www.ex.com](http://www.ex.com)", "[www.ex.com](http://www.ex.com)\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fmtMD(c.md); got != c.want {
				t.Errorf("fmtMD(%q) = %q, want %q (prettier preserves the source "+
					"autolink form)", c.md, got, c.want)
			}
		})
	}
}

// TestBareLinkFromADFKeepsAngleWhenBareWouldNotRoundTrip pins the limit of
// the bare preference above: the ADF leg writes bare only where the bare
// bytes, between the neighbors they actually sit next to, re-parse as the
// same link. ADF says nothing about where the link's source boundaries
// were, so a bare spelling merges with whatever text touches it and the
// reparse picks its own boundaries. Before the round-trip check
// (markdown/render_bare_autolink.go) each row below rendered bare and then
// did not come back:
//
//	"<http://>"      -> "http://"      second render "http\://" (link lost)
//	"<http://0*>"    -> "http://0*"    link "http://0" plus text "*"
//	"<http://0`>"    -> "http://0`"    link "http://0" plus a stray backtick
//	"x<http://a.b>"  -> "xhttp://a.b"  link lost
//	"<http://a.b>x"  -> "http://a.bx"  link TARGET silently "http://a.bx"
//	"a<b@c.d>"       -> "ab@c.d"       link TARGET silently "mailto:ab@c.d"
//	"<b@c.d>-x"      -> "b@c.d-x"      link TARGET silently "mailto:b@c.d-x"
//
// The angle form is the fallback because nothing can merge into it: '<' and
// '>' are ordinary punctuation. Each row checks the first render, that a
// second pass reproduces it byte for byte, and that the ADF the first render
// parses back to is the ADF the source produced — the last is the check the
// retargeting rows failed while still looking idempotent.
func TestBareLinkFromADFKeepsAngleWhenBareWouldNotRoundTrip(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		{"an empty host does not re-link bare", "<http://>", "<http://>\n"},
		{"a trailing asterisk is cut off a bare literal", "<http://0*>", "<http://0*>\n"},
		{"a trailing backtick is cut off a bare literal", "<http://0`>", "<http://0`>\n"},
		{"a letter before the scheme stops the literal linking", "x<http://a.b>", "x<http://a.b>\n"},
		{"a letter after the host would extend the target", "<http://a.b>x", "<http://a.b>x\n"},
		{"a letter before an address would extend the mailbox", "a<b@c.d>", "a<b@c.d>\n"},
		{"a hyphen run after an address would extend the domain", "<b@c.d>-x", "<b@c.d>-x\n"},
		// The word runs on past an emphasis marker: bare, the closing '_'
		// joins the address ("mailto:a@b.co_-x") or the opening one joins
		// the text before it, and the emphasis is lost with it.
		{"a hyphen run after the closing marker would extend the domain", "*<a@b.co>*-x", "_<a@b.co>_-x\n"},
		{"a dot run after the closing marker would extend the domain", "*<a@b.co>*.x", "_<a@b.co>_.x\n"},
		{"a hyphen run before the opening marker would join the mailbox", "x-*<a@b.co>*", "x-_<a@b.co>_\n"},
		// A preceding link's own '<' must stay IN the window, not be cut
		// past: dropping it left only "http://x.y>x-" behind the next link,
		// which reads as a bare URL rather than the true "<http://x.y>" and
		// wrongly let the mailbox go bare too.
		{"a preceding link's own angle bracket is kept, not cut past", "<http://x.y>x-*<a@b.co>*", "<http://x.y>x-_<a@b.co>_\n"},
		// A backslash-escaped '<' is not a wall: the renderer's own escape
		// (escapeAngle) writes a literal '<' as "\<", so the window must
		// drop the backslash along with the '<' here too, exactly as it
		// does for whitespace. Keeping it unconditionally (as the row
		// above's fix first did) planted an unescaped '<' in the window
		// that the real output never had, letting "<span><a@b.co>" misread
		// differently from "\<span><a@b.co>" and wrongly flip this link's
		// own safety verdict.
		{"a backslash-escaped angle bracket is not a wall", "\\<span><a@b.co>", "\\<span><a@b.co>\n"},
		{"an empty host inside emphasis does not re-link bare", "*<http://>*", "_<http://>_\n"},
		// Controls: where no neighbor can merge, the bare preference holds.
		{"a lone URL stays bare", "http://ex.com", "http://ex.com\n"},
		{"spaces either side keep it bare", "a <http://ex.com> b", "a http://ex.com b\n"},
		{"trailing sentence punctuation keeps it bare", "see <http://ex.com>.", "see http://ex.com.\n"},
		{"parentheses keep it bare", "(<http://ex.com>)", "(http://ex.com)\n"},
		{"a lone address stays bare", "<a@ex.com>", "a@ex.com\n"},
		{"an address alone in emphasis stays bare", "*<a@b.co>*", "_a@b.co_\n"},
		// '*' is not an address byte, so the strong marker stops the word
		// from reaching the "x-" two constructs out.
		{"an address two markers in from a hyphen run stays bare", "x-**_<a@b.co>_**", "x-**_a@b.co_**\n"},
		// Overly careful, by design: the first link's look-ahead assumes
		// the second keeps its brackets, so the first keeps its own, though
		// the all-bare "http://a.b*http://c.d*" would also have come back.
		{"a link followed in its word by another keeps its brackets", "<http://a.b>*<http://c.d>*", "<http://a.b>_http://c.d_\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			doc := mdToADF(c.md)
			first := adfToMD(doc)
			if first != c.want {
				t.Fatalf("adfToMD(mdToADF(%q)) = %q, want %q", c.md, first, c.want)
			}
			if second := adfToMD(mdToADF(first)); second != first {
				t.Errorf("render of %q is not a fixpoint: %q then %q", c.md, first, second)
			}
			if got, want := string(mustJSON(mdToADF(first))), string(mustJSON(doc)); got != want {
				t.Errorf("re-parsing %q changed the document:\n got  %s\n want %s", first, got, want)
			}
		})
	}
}

// TestBareLinkFromADFKeepsAngleWhenEscapeRunCrossesTheWindowCap is a
// regression test for a second escaped-'<' bug the row above's fix didn't
// close: escapedAt only counts the backslashes still inside the capped
// 256-byte look-behind window (bareAutolinkWindowCap, render_bare_autolink.go),
// so a run long enough to cross the cap can read with the wrong parity. At
// n=249 the run is 250 backslashes before the '<' (even, NOT escaped — the
// '<' is a real wall), but it fills the whole window up to the cap, so
// escapedAt could never rule out more backslashes beyond it: keeping the '<'
// on that unproven parity used to lose the mailbox link's own brackets on
// round-trip ("a@b.co" bare, able to swallow whatever precedes it) — a
// non-fixpoint, ADF-changing render confirmed by probing with the guard
// removed. bareAutolinkPrefix now gives up (treats the window as unproven)
// whenever the backslash run it can see reaches all the way back to the
// window's own start, since a capped window can never rule out more
// backslashes beyond it — see runReachesWindowStart.
func TestBareLinkFromADFKeepsAngleWhenEscapeRunCrossesTheWindowCap(t *testing.T) {
	t.Parallel()
	md := strings.Repeat(`\`, 249) + `\<span><a@b.co>`
	doc := mdToADF(md)
	first := adfToMD(doc)
	if !strings.Contains(first, "<a@b.co>") {
		t.Fatalf("mailbox link lost its brackets: adfToMD(mdToADF(...)) = %q", first)
	}
	if second := adfToMD(mdToADF(first)); second != first {
		t.Errorf("render is not a fixpoint: %q then %q", first, second)
	}
	if got, want := string(mustJSON(mdToADF(first))), string(mustJSON(doc)); got != want {
		t.Errorf("re-parsing changed the document:\n got  %s\n want %s", got, want)
	}
}

// TestBareAutolinkWindowStaysBoundedInAWhitespaceFreeWord is a regression
// test for a quadratic-time blowup in the window
// bareAutolinkRoundTrips/wordAhead build to answer the bare-vs-angle
// question (markdown/render_bare_autolink.go): the window used to extend to
// the nearest whitespace with no other bound, so many autolinks sitting in
// one whitespace-free "word" — links separated only by punctuation, a common
// Jira/Confluence shape ("bold link-dash-bold link-dash...") — made every
// link in the run re-render and re-parse the whole rest of the word, which is
// quadratic in the link count.
//
// Measured before the fix: 4000 repeats of "*<http://a.b>*-" took minutes on
// adfToMD alone; this test's 4000 repeats finishes in well under a second
// once the window is bounded by the next autolink's own '<' rather than by
// whitespace alone. It is a real test, not a benchmark, so a reintroduced
// O(n^2) scan fails it on an ordinary `go test` run rather than needing
// `-bench`.
func TestBareAutolinkWindowStaysBoundedInAWhitespaceFreeWord(t *testing.T) {
	t.Parallel()
	const n = 4000
	md := strings.Repeat("*<http://a.b>*-", n)

	start := time.Now()
	got := adfToMD(mdToADF(md))
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("adfToMD(mdToADF(%d-link word)) took %s, want well under 1s; "+
			"the bare-autolink window may have regressed to scanning the "+
			"whole word instead of stopping at the next autolink", n, elapsed)
	}
	if !strings.Contains(got, "http://a.b") {
		t.Fatalf("output lost the links entirely: first 80 bytes: %q", got[:min(80, len(got))])
	}
}

// TestPrettierFormatLegIgnoresTheBareRoundTripCheck is the control for the
// test above: the format leg keeps the form the author wrote, which the
// source already proved parses, and never brackets a bare literal on its
// own. Only the remark leg runs the round-trip check.
func TestPrettierFormatLegIgnoresTheBareRoundTripCheck(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		// The remark leg brackets this one (see
		// TestDotlessHostFusedUnderscoreRoundTrips); the format leg keeps
		// the bare spelling the author wrote, byte for byte as before the
		// check existed.
		// A known, accepted non-idempotent case of the format leg, not a
		// regression: formatting this output again escapes the closing
		// underscore, http://0#_0\_ — main does the same — and
		// FuzzFormatSemanticsPreserved skips the shape (urlFusedWithUnderscore
		// in skipRenderedURLClasses).
		{"bare literal before emphasis stays bare", "http://0#*0*", "http://0#_0_\n"},
		{"angle form before a letter stays angle", "<http://a.b>x", "<http://a.b>x\n"},
		{"angle address after a letter stays angle", "a<b@c.d>", "a<b@c.d>\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fmtMD(c.md); got != c.want {
				t.Errorf("fmtMD(%q) = %q, want %q", c.md, got, c.want)
			}
		})
	}
}
