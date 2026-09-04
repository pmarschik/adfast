package markdown

import (
	"regexp"
	"testing"
)

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
	// THE SPLIT AT ITS SHARPEST: a host whose FIRST character is non-ASCII.
	// urlLiteralHostUnicodeLed accepts it in the RAW pattern only, and that
	// asymmetry is the reference's, measured both ways — its raw-source
	// tokenizer links the address whole, its decoded-text transform refuses
	// it because that recognizer's host is the ASCII `[-.\w]+` and wants one
	// such character right after the "://":
	//
	//	"See https://例.com/x end"  ->  "See <https://例.com/x> end"
	//	"a [ https://例.com/x b"    ->  "a \\[ https\\://例.com/x b"
	//
	// Before the widening goldmark read the whole address as prose, so an
	// author with an IDN host got no link at all.
	name: "a non-ASCII first host segment, raw only",
	src:  "https://例.com/x",
	raw:  "https://例.com/x",
	text: "",
}, {
	name: "a non-ASCII first host segment with no path",
	src:  "https://例.com",
	raw:  "https://例.com",
	text: "",
}, {
	// DOTLESS AND NON-ASCII AT ONCE. The tokenizer needs no dot, so this is
	// one address and not two rules meeting; measured, "See https://例/x end"
	// comes back "See <https://例/x> end".
	name: "a dotless non-ASCII host",
	src:  "https://例/x",
	raw:  "https://例/x",
	text: "",
}, {
	// A WHOLLY NON-ASCII HOST, both segments, in a script that is not CJK —
	// the class is a negation and not a spot fix for one alphabet.
	name: "a Cyrillic host and TLD",
	src:  "https://пример.рф/x",
	raw:  "https://пример.рф/x",
	text: "",
}, {
	// The ASCII host bytes still join a Unicode-led host: goldmark's own
	// class continues it, so a hyphen and a port behave as they always did.
	name: "a non-ASCII host with an ASCII tail",
	src:  "https://例-x",
	raw:  "https://例-x",
	text: "",
}, {
	name: "a non-ASCII host with a port",
	src:  "https://例.com:8080/x",
	raw:  "https://例.com:8080/x",
	text: "",
}, {
	// THE NEGATIVES FOR THAT WIDENING, and the reason it is keyed on
	// urlLiteralHostRune rather than "any non-ASCII byte": micromark's
	// afterProtocol refuses `\p{P}`/`\p{S}` right after the "://", so
	// non-ASCII PUNCTUATION opens no host. Measured, both come back as prose
	// from the reference ("See https\\://—.com end").
	name: "an em dash does not open a host",
	src:  "https://—.com",
	raw:  "",
	text: "",
}, {
	name: "a currency sign does not open a host",
	src:  "https://€.com",
	raw:  "",
	text: "",
}, {
	// THE SAME NEGATION ON THE TAIL, which the two rows above do not reach:
	// they only prove the FIRST character of a Unicode-led host is fenced.
	// The continuation is `urlLiteralHostByte | urlLiteralHostRune`, so
	// `\p{P}`/`\p{S}` ends the host wherever it appears, which is micromark's
	// domain rule and not just its afterProtocol gate. Widening the
	// continuation instead to "any non-ASCII byte" — the tail counterpart of
	// the lead over-widening the rows above catch — passed every test in the
	// module before these rows existed, and it is the more damaging half of
	// the two: a lead widening turns prose into a link, while a tail widening
	// takes an address that already links and lengthens its href, so
	// "https://例—x" would go out as an href through the em dash.
	name: "an em dash ends a Unicode-led host",
	src:  "https://例—x",
	raw:  "https://例",
	text: "",
}, {
	name: "a currency sign ends a Unicode-led host",
	src:  "https://例€x",
	raw:  "https://例",
	text: "",
}, {
	name: "a right double quote ends a Unicode-led host",
	src:  "https://例”x",
	raw:  "https://例",
	text: "",
}, {
	// PRESERVED BEHAVIOR PIN, and the one shape the Unicode-led host is
	// deliberately NOT extended to reach: a non-ASCII segment that is not the
	// first. This is micromark's own example, and BOTH halves of the
	// reference link it whole — the tokenizer through its Unicode domain, the
	// transform through an ASCII host of "www." plus its
	// whitespace-terminated path — while here the TLD class takes "點看" and
	// the path cannot open on the following '.', so the href is truncated.
	// That last step is the PATH gap, so this row moves when the path does.
	// Widening the raw host to reach it alone would break the same-extent
	// invariant TestURLLiteralRawPatternIsTheWiderOne states, because the
	// decoded-text pattern cannot follow without an ASCII path widening.
	name: "a non-ASCII middle segment still truncates",
	src:  "https://www.點看.com",
	raw:  "https://www.點看",
	text: "https://www.點看",
}, {
	name: "a non-ASCII TLD before another segment still truncates",
	src:  "https://a.coſ.com/x",
	raw:  "https://a.coſ",
	text: "https://a.coſ",
}, {
	// PRESERVED BEHAVIOR PIN for a divergence OLDER than any widening here,
	// recorded because the Unicode-led host reaches it too: an UNDERSCORE in
	// the host. micromark's domainAfter refuses an underscore in either of
	// the last two segments and the transform's isCorrectDomain refuses it as
	// well, so the reference reads all three of "https://ex_x",
	// "https://a_b.com/x" and "https://例_x" as prose, while goldmark's class
	// has always taken '_' as a host byte. Modeling the rule belongs to that
	// divergence, not to the host widenings.
	name: "an underscore host is accepted here and not by the reference",
	src:  "https://ex_x",
	raw:  "https://ex_x",
	text: "",
}, {
	name: "an underscore in a dotted host, same divergence",
	src:  "https://a_b.com/x",
	raw:  "https://a_b.com/x",
	text: "https://a_b.com/x",
}, {
	// The same divergence reached through the Unicode-led host, which is the
	// only shape the widening adds that the reference does not accept. The
	// row is here so the count of new divergences is one and written down,
	// not inferred.
	name: "an underscore after a non-ASCII host, same divergence",
	src:  "https://例_x",
	raw:  "https://例_x",
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

// TestURLLiteralASCIIExtentsAreGoldmarksExactly is the safety argument for
// every host widening in this file, stated as a property rather than as a
// promise in a comment: on an ALL-ASCII candidate the raw pattern must answer
// exactly what goldmark's own two host alternatives answer, so no widening
// here can move a literal in an ASCII document.
//
// It holds by construction, and the construction is what the test pins: each
// widening opens on urlLiteralHostRune, whose class excludes `\x00-\x7F`, so
// on ASCII input the alternative fails at its first character and Go's
// leftmost-FIRST alternation falls through to goldmark's. Spell the leading
// rune as a plain `[^\x00-\x7F]`, or reach for the host bytes before the
// widened alternative, and this test reports the ASCII candidate that moved.
//
// The sweep is every printable ASCII byte in the three positions a host rule
// can turn on — right after the "://", inside a segment, and after the TLD —
// because the interesting failures are the bytes no hand-written row thinks
// to try.
func TestURLLiteralASCIIExtentsAreGoldmarksExactly(t *testing.T) {
	t.Parallel()
	goldmarkOnly := regexp.MustCompile(
		`^(?:` + urlLiteralScheme + `(?:` + urlLiteralHostDotted +
			`|[a-zA-Z0-9]` + urlLiteralHostByte + `{0,255})` + urlLiteralPath + `)`)
	unicodeLedOnly := regexp.MustCompile(
		`^(?:` + urlLiteralScheme + urlLiteralHostUnicodeLed + urlLiteralPath + `)`)

	var candidates []string
	for _, c := range urlLiteralHostCases {
		candidates = append(candidates, c.src)
	}
	for b := byte(0x20); b < 0x7F; b++ {
		candidates = append(candidates,
			"https://"+string(b)+"ex.com/x",
			"https://ex"+string(b)+".com/x",
			"https://ex.com"+string(b)+"x")
	}

	for _, src := range candidates {
		if !isASCII(src) {
			continue
		}
		if got, want := urlLiteralAnchoredRe.FindString(src), goldmarkOnly.FindString(src); got != want {
			t.Errorf("%q: urlLiteralAnchoredRe gives %q, goldmark's own host rule gives %q",
				src, got, want)
		}
		if got := unicodeLedOnly.FindString(src); got != "" {
			t.Errorf("%q: the Unicode-led host must be unreachable for ASCII, matched %q",
				src, got)
		}
	}
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
