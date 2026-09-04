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
	// THE OTHER SIDE OF THE SAME COIN, and the one shape in this file where
	// the WRONG href went out rather than none: goldmark's ASCII TLD class
	// ended an ACCEPTED literal early, so "https://ex.coſ/x" linked as
	// "https://ex.co". Both halves of the reference run it to the end of the
	// word — the tokenizer with a Unicode domain, the transform with an ASCII
	// host and a permissive path — so urlLiteralHostRune widens the shared
	// TLD class and both columns now hold the whole address. Measured:
	//
	//	"See https://ex.coſ/x end"  ->  "See <https://ex.coſ/x> end"
	//	"a [ https://ex.coſ/x b"    ->  "a \\[ <https://ex.coſ/x> b"
	name: "a non-ASCII TLD no longer truncates the literal",
	src:  "https://ex.coſ/x",
	raw:  "https://ex.coſ/x",
	text: "https://ex.coſ/x",
}, {
	// The same widening, with the non-ASCII letter LAST and no path, and
	// with a Latin-1 letter rather than a long s — the class is a script
	// negation, not a spot fix for U+017F.
	name: "a non-ASCII TLD with no path",
	src:  "https://ex.coſ",
	raw:  "https://ex.coſ",
	text: "https://ex.coſ",
}, {
	name: "an accented TLD letter",
	src:  "https://ex.comé/x",
	raw:  "https://ex.comé/x",
	text: "https://ex.comé/x",
}, {
	// NON-ASCII PUNCTUATION IS STILL NOT A HOST CHARACTER, and the class
	// must not swallow it: micromark's domain stops at `\p{P}`/`\p{S}`, so
	// the host is "ex.com" on both sides. That the literal ENDS here is the
	// path gap, not the host rule — the reference then hands "—x" to its
	// path, which runs to whitespace, and links the whole thing (measured:
	// "See https://ex.com—x end" comes back "See <https://ex.com—x> end").
	// Exactly the "https://ex.com~foo" row below, in the non-ASCII half.
	name: "an em dash ends the host, and the path gap ends the literal",
	src:  "https://ex.com—x",
	raw:  "https://ex.com",
	text: "https://ex.com",
}, {
	// The same for a currency sign, which is `\p{S}` rather than `\p{P}`:
	// both branches of micromark's unicodePunctuation `/\p{P}|\p{S}/u` are
	// out of the host class, so neither is a spot exclusion.
	name: "a currency sign ends the host",
	src:  "https://ex.com€x",
	raw:  "https://ex.com",
	text: "https://ex.com",
}, {
	// PRESERVED BEHAVIOR PIN, the remaining gap: a host whose FIRST segment
	// is non-ASCII. The reference's raw-source tokenizer links this whole
	// and its decoded-text transform rejects it outright (measured:
	// "a [ https://例.com/x b" comes back unlinked), so closing it means
	// widening the RAW pattern alone — a change to a rejected literal's
	// verdict, not to this one's extent.
	name: "a non-ASCII first host segment is not a host",
	src:  "https://例.com/x",
	raw:  "",
	text: "",
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
	// AN EMPTY LEADING SEGMENT. The reference's decoded-text host is
	// `[-.\w]+`, so it may open on a dot, and its isCorrectDomain skips an
	// empty segment instead of rejecting it. Measured, whole bodies:
	//
	//	"See https://.x end"          ->  "See <https://.x> end"
	//	"See https://.internal/x end" ->  "See <https://.internal/x> end"
	//
	// goldmark's `{1,256}` needed one character before the dot, so both
	// came out as prose. Both patterns take it: the reference's raw-source
	// tokenizer rejects a punctuation-led host, but its decoded-text
	// transform then links the leftover text anyway, so the observable
	// answer is a link either way.
	name: "an empty leading host segment",
	src:  "https://.x",
	raw:  "https://.x",
	text: "https://.x",
}, {
	name: "an empty leading segment with a path",
	src:  "https://.internal/x",
	raw:  "https://.internal/x",
	text: "https://.internal/x",
}, {
	// The empty segment is not a license for an EMPTY HOST: the TLD class
	// still needs one character. The reference rejects both of these too —
	// its splitUrl strips the trailing dot and refuses the empty remainder.
	name: "a lone dot is not a host",
	src:  "https://.",
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
