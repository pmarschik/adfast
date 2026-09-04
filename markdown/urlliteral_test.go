package markdown

import "testing"

// urlLiteralHostCase is one host shape, with the verdict of each of the two
// patterns urlliteral.go defines: the RAW-SOURCE one an inline parser reads
// and the DECODED-TEXT one relinkifyTexts reads.
type urlLiteralHostCase struct {
	name string
	// src is the whole candidate, scheme included, and nothing else — both
	// patterns are applied to it alone so a match means "the pattern
	// accepts this address" and a full-length match means "all of it".
	src string
	// raw is what urlLiteralAnchoredRe matches, "" for no match.
	raw string
	// text is what urlLiteralRe matches, "" for no match.
	text string
}

// THE SPLIT IS THE POINT. The reference's two recognizers disagree about the
// host, and this table is where that disagreement is written down: its
// tokenizer reads the raw source and wants no dot, while
// mdast-util-gfm-autolink-literal's post-parse transform reads decoded text
// and its isCorrectDomain wants two dot-separated segments. Measured, whole
// bodies, against the frozen reference:
//
//	"a https\\://ex.com b\n"  ->  "a <https://ex.com> b\n"   escape undone
//	"1\\:yes and https\\://x\n" -> "1\\:yes and https\\://x\n"  escape kept
//
// A later change that gives both patterns one host rule passes every
// positive case in this package and silently breaks that second line, which
// is pinned in format_test.go as "colon escape preserved". Hence a table
// that asserts BOTH columns for every row.
var urlLiteralHostCases = []urlLiteralHostCase{{
	// The shape both recognizers have always taken.
	name: "a dotted host",
	src:  "https://ex.com/h",
	raw:  "https://ex.com/h",
	text: "https://ex.com/h",
}, {
	// THE WIDENING. Raw source only: an intranet name has no dot, and this
	// is the address the reference's tokenizer links and its transform does
	// not.
	name: "a dotless host",
	src:  "https://localhost:8080/x",
	raw:  "https://localhost:8080/x",
	text: "",
}, {
	name: "a dotless intranet host",
	src:  "https://jira/browse/X",
	raw:  "https://jira/browse/X",
	text: "",
}, {
	// Case. Both recognizers of the reference ignore it, so both patterns
	// here do.
	name: "an uppercase scheme and host",
	src:  "HTTPS://EX.COM/x",
	raw:  "HTTPS://EX.COM/x",
	text: "HTTPS://EX.COM/x",
}, {
	name: "a mixed-case scheme with a dotted host",
	src:  "Https://ex.com/y",
	raw:  "Https://ex.com/y",
	text: "Https://ex.com/y",
}, {
	name: "an uppercase scheme with a dotless host",
	src:  "HTTPS://LOCALHOST:8080/x",
	raw:  "HTTPS://LOCALHOST:8080/x",
	text: "",
}, {
	// CASE-INSENSITIVE, NOT CASE-FOLDED. Go's `(?i)` folds U+017F LATIN SMALL
	// LETTER LONG S onto "s", so `(?i:https?)` accepts this scheme and the
	// reference does not — measured, "See httpſ://ex.com/x end" comes back
	// unlinked. The pattern therefore spells the two ASCII cases out. A
	// change that shortens it back to `(?i:...)` passes every other row here.
	name: "a long-s scheme is not the https scheme",
	src:  "httpſ://ex.com/x",
	raw:  "",
	text: "",
}, {
	// PRESERVED BEHAVIOR PIN, and the other side of the same coin. The TLD
	// class is ASCII too, so a non-ASCII letter inside a host ends the
	// literal early: the reference links "https://ex.coſ/x" WHOLE (measured),
	// this stops at "ex.co", exactly where goldmark's `[a-z]+` stopped. That
	// is the host/path character-class divergence — an ACCEPTED literal's
	// extent — and widening the class here would change extents across the
	// corpus rather than change a rejected literal's verdict. Written down so
	// the truncation reads as known rather than as the fix missing a case.
	name: "a long-s TLD keeps goldmark's extent",
	src:  "https://ex.coſ/x",
	raw:  "https://ex.co",
	text: "https://ex.co",
}, {
	// THE NEGATIVES, which is what keeps the widening from being "anything
	// after ://". The reference rejects each of these, measured.
	name: "a host starting on an underscore",
	src:  "https://_x",
	raw:  "",
	text: "",
}, {
	name: "a host starting on a hyphen",
	src:  "https://-x",
	raw:  "",
	text: "",
}, {
	name: "a host starting on a slash",
	src:  "https:///x",
	raw:  "",
	text: "",
}, {
	name: "no host at all",
	src:  "https://",
	raw:  "",
	text: "",
}, {
	// A punctuation-led host is still accepted when it is DOTTED, because
	// goldmark accepted it and so does the reference's transform. Dropping
	// it would be a narrowing dressed up as a widening.
	name: "a hyphen-led dotted host",
	src:  "https://-a.com",
	raw:  "https://-a.com",
	text: "https://-a.com",
}, {
	// The path class is goldmark's and is NOT widened, so a literal still
	// stops where it always did. This row is the guard on that: it is the
	// third divergence, not this one.
	name: "a tilde tail stops the literal",
	src:  "https://ex.com~foo",
	raw:  "https://ex.com",
	text: "https://ex.com",
}}

func TestURLLiteralPatternsSplitOnTheHost(t *testing.T) {
	t.Parallel()
	for _, c := range urlLiteralHostCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			raw := urlLiteralAnchoredRe.FindString(c.src)
			if raw != c.raw {
				t.Errorf("urlLiteralAnchoredRe.FindString(%q) = %q, want %q", c.src, raw, c.raw)
			}
			text := urlLiteralRe.FindString(c.src)
			if text != c.text {
				t.Errorf("urlLiteralRe.FindString(%q) = %q, want %q", c.src, text, c.text)
			}
		})
	}
}

// TestURLLiteralRawPatternIsTheWiderOne states the relationship the two
// patterns must keep, rather than leaving it implied by the table: whatever
// the decoded-text pattern accepts at the head of a candidate, the
// raw-source pattern accepts too, and at the same extent. A change that
// widened only urlLiteralRe would invert the pair and pass every case in
// the table above that has a nonempty `raw`.
func TestURLLiteralRawPatternIsTheWiderOne(t *testing.T) {
	t.Parallel()
	for _, c := range urlLiteralHostCases {
		if c.text == "" {
			continue
		}
		if got := urlLiteralAnchoredRe.FindString(c.src); got != c.text {
			t.Errorf("%s: urlLiteralRe accepts %q but urlLiteralAnchoredRe gives %q",
				c.name, c.text, got)
		}
	}
}
