package adfast

import "testing"

// A BARE LINK HAS TWO CORRECT SPELLINGS, ONE PER RENDER LEG, AND THE TWO LEGS
// ANSWER TO DIFFERENT REFERENCES. That is the whole content of this file, and
// it exists because the two are easy to compare crosswise and read as a
// divergence that neither leg has.
//
//   - THE ADF LEG (adfToMD, the pull path: ADF -> markdown) answers to
//     mdast-util-to-markdown. ADF carries no "was it written in angle
//     brackets" bit — a link mark is a URL and some text — so the leg has one
//     spelling available for a link whose text equals its URL, and remark's
//     is the angle form.
//   - THE FORMAT LEG (fmtMD, md -> md with WithPrettierFormat) answers to
//     prettier, never touches ADF, and therefore still has the source form on
//     ast.Link.Bare. Prettier preserves what the author wrote, so this leg
//     does too: an angle source stays angle, a bare literal stays bare.
//
// Read the bare output of the format leg as the ADF leg dropping its brackets
// and you get a divergence that is not there. Both references were measured
// directly, at unified 11.0.3 / remark-parse 11.0.0 / remark-gfm 4.0.0 /
// remark-directive 3.0.0 / remark-stringify 11.0.0, and prettier 3.8.1:
//
//	// mdast: paragraph > link{url, children: [text]}, via proc.stringify —
//	// the post-ADF tree built by hand, because the reference has no ADF hop
//	// to lose the form in.
//	{"link":"http://ex.com","text":"http://ex.com","stringify":"<http://ex.com>\n"}
//	{"link":"HTTP://ex.com","text":"HTTP://ex.com","stringify":"<HTTP://ex.com>\n"}
//	{"link":"http://0","text":"http://0","stringify":"<http://0>\n"}
//	{"link":"http://www.ex.com","text":"www.ex.com","stringify":"[www.ex.com](http://www.ex.com)\n"}
//
//	// prettier 3.8.1, parser "markdown"
//	{"in":"<http://ex.com>","prettier":"<http://ex.com>\n"}
//	{"in":"http://ex.com","prettier":"http://ex.com\n"}
//	{"in":"www.ex.com","prettier":"www.ex.com\n"}
//	{"in":"<http://0>","prettier":"<http://0>\n"}
//	{"in":"http://0","prettier":"http://0\n"}
//
// Both tests are preserved-behavior PINS, not fix tests: the behavior they
// record is already correct in both legs. What they buy is that the next
// reader of a crosswise comparison has to break a test to act on it.
//
// The "www." rows are the contrast that makes the pins say something. There
// the URL and the text DIFFER (href "http://www.ex.com", text "www.ex.com"),
// so the angle form cannot express the node at all and the ADF leg must write
// the full [label](url) — which is also what the reference writes. A blanket
// "always write the address bare" or "always wrap in angles" passes neither
// half.
func TestBareLinkFromADFWritesTheReferenceAngleForm(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		{"bare literal in the source", "http://ex.com", "<http://ex.com>\n"},
		{"angle form in the source", "<http://ex.com>", "<http://ex.com>\n"},
		{"a path keeps the same form", "https://ex.com/x", "<https://ex.com/x>\n"},
		{"an uppercase scheme keeps the same form", "HTTP://ex.com", "<HTTP://ex.com>\n"},
		// A dotless host: the decoded-text gate would refuse to re-linkify
		// this one bare (isCorrectDomain wants two dot-separated segments),
		// which is exactly why the angle form is the only lossless spelling.
		{"a dotless host", "http://0", "<http://0>\n"},
		{"www literal: href and text differ", "www.ex.com", "[www.ex.com](http://www.ex.com)\n"},
		{"www literal with a numeric label", "www.0.a", "[www.0.a](http://www.0.a)\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := adfToMD(mdToADF(c.md)); got != c.want {
				t.Errorf("adfToMD(mdToADF(%q)) = %q, want %q (mdast-util-to-markdown "+
					"writes this form for a link whose text equals its URL)",
					c.md, got, c.want)
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
