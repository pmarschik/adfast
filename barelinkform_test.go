package adfast

import "testing"

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
