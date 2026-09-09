package markdown

import (
	"regexp"
	"testing"

	"github.com/pmarschik/adfast/ast"
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
	// hostRejected is urlLiteralHostAccepted's verdict on src, inverted so
	// the common answer is the zero value. THE PATTERN IS ONLY HALF THE
	// ANSWER: every caller runs one of the two patterns above and then the
	// gate, so a row with a nonempty column and hostRejected set is prose in
	// the document even though the pattern matched. The rows that carry it
	// are the underscore hosts and the punctuation-led ones; that every other
	// row here leaves it false is what says the gate refuses those and
	// nothing else.
	hostRejected bool
	// textHostRejected is urlLiteralHostAcceptedInText's verdict on src, the
	// same way round. THE GATES SPLIT WHERE THE PATTERNS DO, so this column
	// is not redundant with the one above: the decoded-text gate is
	// mdast-util's isCorrectDomain, which demands an alphanumeric in each of
	// the host's last two segments, and micromark's tokenizer runs no such
	// test. The rows carrying textHostRejected without hostRejected are that
	// split, and they are the reference's own — see urlliteral.go's
	// urlLiteralHostAcceptedInText for the measured bodies.
	textHostRejected bool
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
	name:             "a dotless host",
	src:              "https://localhost:8080/x",
	raw:              "https://localhost:8080/x",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a dotless intranet host",
	src:              "https://jira/browse/X",
	raw:              "https://jira/browse/X",
	text:             "",
	textHostRejected: true,
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
	name:             "an uppercase scheme with a dotless host",
	src:              "HTTPS://LOCALHOST:8080/x",
	raw:              "HTTPS://LOCALHOST:8080/x",
	text:             "",
	textHostRejected: true,
}, {
	// CASE-INSENSITIVE, NOT CASE-FOLDED. Go's `(?i)` folds U+017F LATIN SMALL
	// LETTER LONG S onto "s", so `(?i:https?)` accepts this scheme and the
	// reference does not — measured, "See httpſ://ex.com/x end" comes back
	// unlinked. The pattern therefore spells the two ASCII cases out. A
	// change that shortens it back to `(?i:...)` passes every other row here.
	name:             "a long-s scheme is not the https scheme",
	src:              "httpſ://ex.com/x",
	raw:              "",
	text:             "",
	textHostRejected: true,
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
	name:             "a non-ASCII first host segment, raw only",
	src:              "https://例.com/x",
	raw:              "https://例.com/x",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a non-ASCII first host segment with no path",
	src:              "https://例.com",
	raw:              "https://例.com",
	text:             "",
	textHostRejected: true,
}, {
	// DOTLESS AND NON-ASCII AT ONCE. The tokenizer needs no dot, so this is
	// one address and not two rules meeting; measured, "See https://例/x end"
	// comes back "See <https://例/x> end".
	name:             "a dotless non-ASCII host",
	src:              "https://例/x",
	raw:              "https://例/x",
	text:             "",
	textHostRejected: true,
}, {
	// A WHOLLY NON-ASCII HOST, both segments, in a script that is not CJK —
	// the class is a negation and not a spot fix for one alphabet.
	name:             "a Cyrillic host and TLD",
	src:              "https://пример.рф/x",
	raw:              "https://пример.рф/x",
	text:             "",
	textHostRejected: true,
}, {
	// The ASCII host bytes still join a Unicode-led host: goldmark's own
	// class continues it, so a hyphen and a port behave as they always did.
	name:             "a non-ASCII host with an ASCII tail",
	src:              "https://例-x",
	raw:              "https://例-x",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a non-ASCII host with a port",
	src:              "https://例.com:8080/x",
	raw:              "https://例.com:8080/x",
	text:             "",
	textHostRejected: true,
}, {
	// THE NEGATIVES FOR THAT WIDENING, and the reason it is keyed on
	// urlLiteralHostRune rather than "any non-ASCII byte": micromark's
	// afterProtocol refuses `\p{P}`/`\p{S}` right after the "://", so
	// non-ASCII PUNCTUATION opens no host. Measured, both come back as prose
	// from the reference ("See https\\://—.com end").
	name:             "an em dash does not open a host",
	src:              "https://—.com",
	raw:              "",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a currency sign does not open a host",
	src:              "https://€.com",
	raw:              "",
	text:             "",
	textHostRejected: true,
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
	name:             "an em dash ends a Unicode-led host",
	src:              "https://例—x",
	raw:              "https://例",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a currency sign ends a Unicode-led host",
	src:              "https://例€x",
	raw:              "https://例",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a right double quote ends a Unicode-led host",
	src:              "https://例”x",
	raw:              "https://例",
	text:             "",
	textHostRejected: true,
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
	// THE HOST GATE, whose rows all keep their pattern match and lose the
	// document: an UNDERSCORE in either of the host's LAST TWO dot-separated
	// segments. micromark's domainAfter counts underscores per segment as it
	// consumes the domain and mdast-util's isCorrectDomain tests the last two
	// parts of a split host, so both halves of the reference read these as
	// prose, while goldmark's host class has always taken '_' as a byte.
	//
	// The pattern columns still hold the whole address on purpose. The rule
	// rejects an address rather than shortening one, and a character class
	// can only shorten — see urlLiteralHostAccepted — so the columns record
	// what the pattern reaches and hostRejected records what the document
	// gets.
	name:             "an underscore in a dotless host",
	src:              "https://ex_x",
	raw:              "https://ex_x",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	name:             "an underscore in the second-to-last segment",
	src:              "https://a_b.com/x",
	raw:              "https://a_b.com/x",
	text:             "https://a_b.com/x",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// The same rule reached through the Unicode-led host. The row is here so
	// that widening is measured against the gate too, rather than being
	// assumed to share the ASCII answer.
	name:             "an underscore after a non-ASCII host",
	src:              "https://例_x",
	raw:              "https://例_x",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// AN UNDERSCORE IN THE LAST SEGMENT, and the row that pays for the '_' in
	// urlLiteralHostDotted's TLD class. Without that byte the class ended the
	// dotted alternative at the TLD, so the pattern matched "https://ex.com"
	// and the WRONG HREF went out — a shortened address, which is the failure
	// mode the gate exists to avoid. With it the pattern reaches the whole
	// candidate and the gate refuses the whole candidate, which is what both
	// halves of the reference do. The byte is safe to add precisely because
	// any match that reaches it has an underscore in the last segment, so the
	// gate always refuses it.
	name:             "an underscore in the last segment",
	src:              "https://ex.com_x",
	raw:              "https://ex.com_x",
	text:             "https://ex.com_x",
	hostRejected:     true,
	textHostRejected: true,
}, {
	name:             "an underscore in the last segment with a path",
	src:              "https://ex.com_x/y",
	raw:              "https://ex.com_x/y",
	text:             "https://ex.com_x/y",
	hostRejected:     true,
	textHostRejected: true,
}, {
	name:             "an underscore in both of the last two segments",
	src:              "https://a_b.c_d.com",
	raw:              "https://a_b.c_d.com",
	text:             "https://a_b.c_d.com",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// LAST TWO, NOT ANY. This pair is what makes the rule a segment count
	// rather than "no underscore in a host": the underscore sits in the
	// second-to-last segment in one and the third-to-last in the other, and
	// only the first is refused. A gate that scanned the whole host would
	// pass every other row in this table and fail here.
	name:             "an underscore in the second-to-last segment of three",
	src:              "https://a.b_c.com",
	raw:              "https://a.b_c.com",
	text:             "https://a.b_c.com",
	hostRejected:     true,
	textHostRejected: true,
}, {
	name: "an underscore in the third-to-last segment is accepted",
	src:  "https://x_y.z.com",
	raw:  "https://x_y.z.com",
	text: "https://x_y.z.com",
}, {
	// THE TRIM IS PART OF THE RULE. goldmark drops a trailing run of
	// `? ! . , : * _ ~` off its match and micromark's domain ends before the
	// same run, so the host here is "ex" and the address links — testing the
	// untrimmed candidate would refuse one both recognizers accept. The
	// second row is the discriminator: trimming the dot leaves "a_b", which
	// is still an underscore in the last segment.
	name:             "a trailing underscore is trimmed off the host",
	src:              "https://ex_",
	raw:              "https://ex_",
	text:             "",
	textHostRejected: true,
}, {
	name:             "a trailing dot does not rescue an underscore",
	src:              "https://a_b.",
	raw:              "https://a_b.",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// The same trailing underscore in a DOTTED host, which is what the '_' in
	// the TLD class reaches: the pattern takes "com_" and the trim gives the
	// host back as "ex.com".
	name:             "a trailing underscore after a TLD is trimmed",
	src:              "https://ex.com_",
	raw:              "https://ex.com_",
	text:             "https://ex.com_",
	textHostRejected: true,
}, {
	// A PATH ENDS THE TRIM'S CLAIM, and this row is its discriminator against
	// the one above: micromark ends the domain before a punctuation run only
	// when the run reaches the end of the URL. Here "/y" follows, so the
	// underscore stays in the last segment and the address is prose — while
	// "https://ex_" two rows up loses it and links.
	name:             "an underscore before a path is not trimmed",
	src:              "https://ex_/y",
	raw:              "https://ex_/y",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// THE NEGATIVES, which is what keeps the widening from being "anything
	// after ://". The reference rejects each of these, measured.
	// Refused twice over, and the columns say which came first: the pattern
	// never opens a host on an underscore, and the gate would refuse the host
	// even if it did.
	name:             "a host starting on an underscore",
	src:              "https://_x",
	raw:              "",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// A HYPHEN-LED DOTLESS HOST IS REFUSED BY EVERY LAYER, and the gate columns
	// are the ones that changed here: micromark checks the character right
	// after the "://" on its own and refuses `\p{P}`/`\p{S}` there, while
	// isCorrectDomain wants two segments and this host has one. Measured, the
	// reference reads "see https://-x b" as prose from both recognizers. The
	// raw gate used to ACCEPT this address — nothing reached it, because the
	// pattern refused first, and then the '-' in urlLiteralHostDotted's TLD
	// class made "https://-.-" matchable and the gate's answer suddenly
	// mattered. Compare "a hyphen-led dotted host" below, which both
	// recognizers do link.
	name:             "a host starting on a hyphen",
	src:              "https://-x",
	raw:              "",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// THE EMPTY DOMAIN, which the gate refuses on its own account: the
	// reference's domain production must consume at least one character, so
	// a literal whose whole domain is a path opener or nothing at all has no
	// host to judge. Neither pattern reaches these, so the columns are the
	// first refusal and the gate is the second.
	name:             "a host starting on a slash",
	src:              "https:///x",
	raw:              "",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
}, {
	name:             "no host at all",
	src:              "https://",
	raw:              "",
	text:             "",
	hostRejected:     true,
	textHostRejected: true,
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
	// still needs one character. The reference rejects this too — measured,
	// "see https://. b" comes back as prose from both recognizers, its
	// splitUrl stripping the trailing dot and refusing the empty remainder.
	//
	// BOTH GATE COLUMNS ARE FALSE ON PURPOSE, and the pattern columns are why
	// that is safe: nothing reaches the gate, because neither pattern matches.
	// The gates say "accepted" here because isCorrectDomain does — "." splits
	// into two EMPTY segments, and the reference runs neither of its tests on
	// a falsy part, which is the same skip that links "https://.x" one row up.
	// Teaching the gate to refuse this address would be a rule the reference
	// does not have, bought for an input the pattern already refuses.
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
}, {
	// THE DIGIT TLD, at the pattern layer. Both recognizers of the reference
	// take a digit in the domain, so BOTH columns hold the whole address; the
	// class used to be `[a-zA-Z_]`, which ended the match at "https://ex.c"
	// and sent that shorter, wrong address out as the href. See
	// tlddigit_test.go for the document leg and the measured reference rows.
	name: "a digit in the TLD no longer truncates the literal",
	src:  "https://ex.c0m",
	raw:  "https://ex.c0m",
	text: "https://ex.c0m",
}, {
	name: "a digit in the TLD with a path and a port",
	src:  "https://ex.c0m:8080/p",
	raw:  "https://ex.c0m:8080/p",
	text: "https://ex.c0m:8080/p",
}, {
	// THE DIGIT AND THE '_' COMPOSE, and this row is why the digit could not
	// wait. The TLD class carries '_' so the gate can SEE an underscore in the
	// last segment and refuse the address; a digit ending the class one byte
	// early left the '_' trailing, the gate's trailing-punctuation trim
	// dropped it, and "https://ex.a" linked. Measured, the reference reads the
	// whole thing as prose. The pattern matches and the GATE refuses — which
	// is the pairing hostRejected exists to state.
	name:             "a digit after an underscore keeps the underscore in the last segment",
	src:              "https://ex.a_1",
	raw:              "https://ex.a_1",
	text:             "https://ex.a_1",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// THE HYPHEN TLD, at the pattern layer, and the same WRONG-HREF shape the
	// digit rows above carry: the class stopped at the '-', the dotted
	// alternative is tried FIRST, so its short match won outright and the
	// shorter host went out as the href while the text said otherwise. A
	// punycode TLD is the case that makes it user-facing rather than exotic —
	// every internationalized domain has one. Both recognizers of the
	// reference take the byte, so BOTH columns hold the whole address. See
	// TestHyphenInTLDKeepsTheWholeHost for the document leg and the measured
	// reference rows.
	name: "a hyphen in a punycode TLD no longer truncates the literal",
	src:  "https://ex.xn--p1ai",
	raw:  "https://ex.xn--p1ai",
	text: "https://ex.xn--p1ai",
}, {
	name: "a hyphen in both host segments",
	src:  "https://a-b.c-d",
	raw:  "https://a-b.c-d",
	text: "https://a-b.c-d",
}, {
	name: "a hyphen after a complete TLD",
	src:  "https://ex.com-x",
	raw:  "https://ex.com-x",
	text: "https://ex.com-x",
}, {
	// The hyphen at the two ends of the TLD, which the segment scanner reads
	// differently from the middle: leading, it is what the alphanumeric rule
	// looks past; trailing, it is also in goldmark's trailing-punctuation set
	// and so is what the raw gate's trim takes. Both link, measured, from both
	// recognizers.
	name: "a hyphen-led TLD",
	src:  "https://a.-b",
	raw:  "https://a.-b",
	text: "https://a.-b",
}, {
	name: "a hyphen-trailed TLD",
	src:  "https://a.b-",
	raw:  "https://a.b-",
	text: "https://a.b-",
}, {
	// THE NO-ALPHANUMERIC TLD IS WHERE THE TWO GATES PART, and these rows are
	// the reason the hyphen could not simply join the TLD class. mdast-util's
	// isCorrectDomain demands an alphanumeric in each of the host's last two
	// segments and micromark's domain production has no such test, so the
	// reference itself answers these two ways — measured, one body per row,
	// the decoded column being the same literal after a dangling "[ ":
	//
	//	"https://ex.--"  raw https://ex.--   decoded (no link)
	//	"https://ex.-"   raw https://ex.-    decoded (no link)
	//
	// Before the hyphen reached the TLD class the dotted alternative failed
	// here and the dotless one caught the address, which agreed with the raw
	// column by accident. Now the pattern matches and the SPLIT is carried by
	// the gates, which is what the two columns state.
	name:             "a TLD of nothing but hyphens",
	src:              "https://ex.--",
	raw:              "https://ex.--",
	text:             "https://ex.--",
	textHostRejected: true,
}, {
	name:             "a TLD of one hyphen",
	src:              "https://ex.-",
	raw:              "https://ex.-",
	text:             "https://ex.-",
	textHostRejected: true,
}, {
	// BOTH SEGMENTS PUNCTUATION-ONLY, and here the reference refuses on BOTH
	// sides: its tokenizer never reaches the domain because the character
	// right after the "://" is `\p{P}`, and isCorrectDomain refuses the "-"
	// segments. Measured, "see https://-.- b" is prose either way. This row is
	// the one the hyphen in the TLD class made matchable, so without the raw
	// gate's after-protocol check the widening would have INVENTED a link.
	name:             "a host of nothing but hyphens and a dot",
	src:              "https://-.-",
	raw:              "https://-.-",
	text:             "https://-.-",
	hostRejected:     true,
	textHostRejected: true,
}, {
	// THE SCHEME SET IS GFM's. goldmark's linkify extension adds "ftp://" and
	// the reference has no such scheme, so neither pattern here does. See
	// linkifyscheme_test.go for the document leg beside the four other bare
	// schemes that were never literals.
	name:             "an ftp scheme is not a bare URL",
	src:              "ftp://ex.com/f",
	raw:              "",
	text:             "",
	textHostRejected: true,
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
			if got := !urlLiteralHostAccepted(c.src); got != c.hostRejected {
				t.Errorf("urlLiteralHostAccepted(%q) = %v, want %v",
					c.src, !got, !c.hostRejected)
			}
			if got := !urlLiteralHostAcceptedInText(c.src); got != c.textHostRejected {
				t.Errorf("urlLiteralHostAcceptedInText(%q) = %v, want %v",
					c.src, !got, !c.textHostRejected)
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
//
// THE GATES CARRY THE SAME ORDERING, and the second leg is where that is
// stated. The raw-source gate is a UNION of the reference's two rules and the
// decoded-text gate is one of them, so an address the decoded gate accepts the
// raw gate must accept too. That is not decoration: the '-' in
// urlLiteralHostDotted's TLD class is safe only because the alphanumeric rule
// sits in the narrower gate, and moving that rule into the shared one — the
// obvious simplification, one gate instead of two — inverts the pair here
// while every row's pattern columns still pass.
func TestURLLiteralRawPatternIsTheWiderOne(t *testing.T) {
	t.Parallel()
	for _, c := range urlLiteralHostCases {
		if !c.textHostRejected && c.hostRejected {
			t.Errorf("%s: urlLiteralHostAcceptedInText accepts %q but urlLiteralHostAccepted refuses it",
				c.name, c.src)
		}
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
// every UNICODE host widening in this file, stated as a property rather than
// as a promise in a comment: on an ALL-ASCII candidate the raw pattern must
// answer exactly what the ASCII host alternatives answer, so no widening for a
// non-ASCII host can move a literal in an ASCII document.
//
// IT DOES NOT SPEAK FOR THE '_' IN urlLiteralHostDotted's TLD CLASS, which is
// the one place this package's ASCII host rule is deliberately not goldmark's,
// and which this test cannot see because goldmarkOnly is built from the same
// constant. That byte lengthens an ASCII extent on purpose —
// "https://ex.com_x" matches whole where goldmark stopped at "https://ex.com"
// — and what keeps it safe is not this property but urlLiteralHostAccepted,
// which refuses every candidate that reaches the byte. The urlLiteralHostCases
// rows carrying hostRejected are where that pair is measured.
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

// TestURLLiteralHostGateReadsThroughAnEscape covers the half of the host rule
// the candidate table cannot reach: a backslash INSIDE the address.
//
// A backslash is not a host byte, so it ends the host wherever it sits, and the
// shortened host it leaves can pass a rule the whole one fails. That is not a
// hypothetical — it is the formatter's own output. "https://例_x" is prose, the
// renderer escapes an underscore after a non-ASCII letter, and the escaped
// spelling "https://例\_x" leaves the host "例", which is dotless and
// underscore-free and would link. The document would gain a link to
// "https://例" by being formatted.
//
// The accepted rows are the discriminator: they carry the SAME escape shape and
// come back true, so the test says the verdict follows the escape-free host and
// not the presence of a backslash.
func TestURLLiteralHostGateReadsThroughAnEscape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		line     string
		accepted bool
	}{{
		name:     "a bare underscore host",
		line:     "https://ex_x b",
		accepted: false,
	}, {
		name:     "an escaped underscore in an ASCII host",
		line:     `https://ex\_x b`,
		accepted: false,
	}, {
		name:     "an escaped underscore in a non-ASCII host",
		line:     `https://例\_x b`,
		accepted: false,
	}, {
		name:     "an escaped underscore in the last segment",
		line:     `https://ex.com\_x b`,
		accepted: false,
	}, {
		// The same escape, and the host is fine once it is read whole: the
		// underscore is trailing, so the trim takes it.
		name:     "an escaped underscore that trims away",
		line:     `https://ex.com\_ b`,
		accepted: true,
	}, {
		name:     "an escaped dot inside an accepted host",
		line:     `https://ex.com\.x b`,
		accepted: true,
	}, {
		name:     "no escape at all",
		line:     "https://ex.com b",
		accepted: true,
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line := []byte(c.line)
			m := urlLiteralCandidate(line)
			if m == nil {
				t.Fatalf("urlLiteralCandidate(%q) found no literal to gate", c.line)
			}
			if got := urlLiteralHostAcceptedAt(line, len(m)); got != c.accepted {
				t.Errorf("urlLiteralHostAcceptedAt(%q, %d) = %v, want %v (candidate %q)",
					c.line, len(m), got, c.accepted, m)
			}
		})
	}
}

// A HYPHEN IN THE TLD USED TO TRUNCATE THE LITERAL, and the truncated string
// went out as the href — the same failure mode as the digit in the TLD
// (tlddigit_test.go), at the same site, urlLiteralHostDotted's TLD class.
// "https://ex.xn--p1ai" matched only as far as "https://ex.xn", so the link
// text named a punycode host and the link's target named a different one, and
// nothing in the rendered output says so. A punycode TLD is what makes this
// user-facing rather than exotic: every internationalized domain has one.
//
// Measured against the reference (remark-parse 11.0.0 + remark-gfm 4.0.1),
// both recognizers agreeing on every row, the "was" column being this package
// before the class moved:
//
//	"see https://ex.xn--p1ai here"  ref https://ex.xn--p1ai   was https://ex.xn
//	"see https://a-b.c-d b"         ref https://a-b.c-d       was https://a-b.c
//	"see https://ex.com-x b"        ref https://ex.com-x      was https://ex.com
//	"see www.ex.c-d here"           ref http://www.ex.c-d     was http://www.ex.c
//	"see https://a.b- b"            ref https://a.b-          was https://a.b
//	"see https://a.-b b"            ref https://a.-b          was https://a.-b
//
// THE BYTE ALONE WOULD HAVE BEEN A DEFECT OF ITS OWN, which is why this landed
// as a gate change first and a class widening second. A TLD of nothing but
// punctuation is where the reference's two recognizers PART — its tokenizer
// links "https://ex.--" and its transform refuses the same address — so a
// hyphen in a class shared by both patterns, with one gate shared by both,
// makes this package answer one way where the reference answers two.
// urlliteral.go's urlLiteralHostAcceptedInText carries those measurements; the
// "a TLD of nothing but hyphens" rows in urlLiteralHostCases pin the split, and
// TestURLLiteralRawPatternIsTheWiderOne pins the ordering between the gates.
func TestHyphenInTLDKeepsTheWholeHost(t *testing.T) {
	t.Parallel()
	const src = "See https://ex.xn--p1ai, https://a-b.c-d, https://ex.com-x," +
		" www.ex.c-d, https://a.b- and https://a.-b here.\n" +
		"See https://ex.com, www.ex.com and https://ex.co here.\n"
	want := []string{
		// Line one: the truncating shapes. TEXT AND HREF ARE ASSERTED
		// TOGETHER, which is the whole point — the defect was a row where the
		// two halves named different hosts, so a test reading only the href
		// would have passed on "https://ex.xn" just as happily.
		"https://ex.xn--p1ai|https://ex.xn--p1ai",
		"https://a-b.c-d|https://a-b.c-d",
		"https://ex.com-x|https://ex.com-x",
		"www.ex.c-d|http://www.ex.c-d",
		"https://a.b-|https://a.b-",
		"https://a.-b|https://a.-b",
		// Line two: THE GOOD CASES, in the same document so the boundary is
		// visible. A letters-only TLD, the scheme-less form and a short TLD all
		// linked before the class moved and still do — a widening that ran the
		// TLD on past the comma would pass every row above and break these.
		"https://ex.com|https://ex.com",
		"www.ex.com|http://www.ex.com",
		"https://ex.co|https://ex.co",
	}
	if got := linkVerdictsOf(Parse([]byte(src))); !equalStrings(got, want) {
		t.Errorf("links of\n%s\n got %v\nwant %v", src, got, want)
	}

	// The decoded-text recognizer — the text a dangling link label leaves
	// behind — reads urlLiteralRe, built from the same shared host rule. It
	// truncated identically, and it has to move with the raw one or the same
	// address is two different links depending on its surroundings.
	const dangling = "a [ https://ex.xn--p1ai b\n"
	wantDangling := []string{"https://ex.xn--p1ai|https://ex.xn--p1ai"}
	if got := linkVerdictsOf(Parse([]byte(dangling))); !equalStrings(got, wantDangling) {
		t.Errorf("links of %q = %v, want %v", dangling, got, wantDangling)
	}
}

// THE NO-ALPHANUMERIC TLD, at the document layer, pinned because it is the
// price of the row above and because the reference answers it TWO WAYS.
//
// Measured against the reference, one body per row, the second column being
// the same literal after a dangling "[ " so that only its decoded-text
// transform sees it:
//
//	"https://ex.--"  raw https://ex.--    decoded (no link)
//	"https://ex.-"   raw https://ex.-     decoded (no link)
//	"www..-"         raw http://www..-    decoded (no link)
//	"https://-.-"    raw (no link)        decoded (no link)
//
// So the split is the reference's own, and this test is where this package
// keeps it. The first three link in raw source and stay prose in decoded text;
// the fourth is refused everywhere, and it is the one the hyphen in the TLD
// class made MATCHABLE — without the after-protocol check in the raw gate the
// widening would have invented a link for it.
func TestNoAlphanumericTLDSplitsTheTwoRecognizers(t *testing.T) {
	t.Parallel()
	const raw = "See https://ex.--, https://ex.-, www..- and https://-.- here.\n"
	wantRaw := []string{
		"https://ex.--|https://ex.--",
		"https://ex.-|https://ex.-",
		"www..-|http://www..-",
	}
	if got := linkVerdictsOf(Parse([]byte(raw))); !equalStrings(got, wantRaw) {
		t.Errorf("links of %q = %v, want %v", raw, got, wantRaw)
	}
	for _, body := range []string{"https://ex.--", "https://ex.-", "www..-", "https://-.-"} {
		src := "a [ " + body + " b\n"
		if got := linkVerdictsOf(Parse([]byte(src))); len(got) != 0 {
			t.Errorf("links of %q = %v, want none", src, got)
		}
	}
}

// THE ROWS THE HYPHEN FIX COSTS, pinned so they are a decision and not a
// surprise. Every one of them is a host whose segments are punctuation only
// next to a TRAILING DOT, and every one is a corner of the PATH gap
// urlliteral.go's header describes and deliberately holds open — the pattern
// ends a match where the reference's recognizers run on to whitespace and trim
// afterwards, so a trailing dot lies beyond the match and the shifted segment
// window the gate would need never reaches it.
//
// Measured over a 2340-body host sweep against the reference (remark-parse
// 11.0.0 + remark-gfm 4.0.1), the change moves 532 of those bodies ONTO the
// reference and 8 off it, in three shapes:
//
//	body              ref raw          was raw         now raw
//	"https://a.-./y"  https://a.-./y   https://a.-./y  https://a.-
//	"https://-.a."    https://-.a      https://-.a     (no link)
//	"www.-.a."        http://www.-.a   http://www.-.a  http://www.-.a (raw)
//	                  http://www.-.a   http://www.-.a  (no link)      (decoded)
//
// The first is the dotted alternative winning SHORT over the dotless one, which
// is the same trade the digit in the TLD class made (tlddigit_test.go); the
// other two are the trailing dot. The '0' spellings of each — "https://0.-./y",
// "https://-.0.", "www.-.0." — behave identically, and 12 further bodies in the
// same family move from one wrong answer to another. Closing any of them means
// the path gap moves as one piece, with the trail rule.
func TestHyphenTLDResidueIsPinned(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		body    string
		raw     []string
		decoded []string
	}{{
		name: "a punctuation-only TLD before a path loses the path",
		body: "https://a.-./y",
		raw:  []string{"https://a.-|https://a.-"},
	}, {
		name: "a punctuation-only first segment with a trailing dot loses the link",
		body: "https://-.a.",
	}, {
		name:    "the scheme-less form of the same host keeps the raw link only",
		body:    "www.-.a.",
		raw:     []string{"www.-.a|http://www.-.a"},
		decoded: nil,
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			raw := "see " + c.body + " here\n"
			if got := linkVerdictsOf(Parse([]byte(raw))); !equalStrings(got, c.raw) {
				t.Errorf("links of %q = %v, want %v", raw, got, c.raw)
			}
			decoded := "a [ " + c.body + " b\n"
			if got := linkVerdictsOf(Parse([]byte(decoded))); !equalStrings(got, c.decoded) {
				t.Errorf("links of %q = %v, want %v", decoded, got, c.decoded)
			}
		})
	}
}

// TestURLLiteralUnderscoreHostStaysProse is the end-to-end half: one document
// holding every shape the host rule refuses next to the shapes it must not, so
// a gate that over-reached would fail here rather than pass quietly.
//
// THE SECOND LEG asserts that rendering the document neither adds a link nor
// takes one away, which is the property the gate has to hold on both sides of a
// round trip. It does NOT reach the escape hole urlLiteralHostAcceptedAt
// closes: this renderer escapes the colon as well ("https\\://例\\_x"), so the
// literal is gone before the host is read. The formatter in the root package
// escapes only the underscore, which is why that hole is measured there — see
// TestEscapedUnderscoreInAHostDoesNotLink.
func TestURLLiteralUnderscoreHostStaysProse(t *testing.T) {
	t.Parallel()
	const src = "see https://ex_x and https://ex.com_x and https://a_b.com/x" +
		" and https://例_x and https://x_y.z.com and www.a_b.com and www.a.A end\n"
	want := []string{"https://x_y.z.com", "http://www.a.A"}
	if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
		t.Errorf("link URLs of %q = %v, want %v", src, got, want)
	}
	rendered := Render(Parse([]byte(src)))
	if got := linkURLsOf(Parse([]byte(rendered))); !equalStrings(got, want) {
		t.Errorf("link URLs of the rendered form %q = %v, want %v", rendered, got, want)
	}
	if twice := Render(Parse([]byte(rendered))); twice != rendered {
		t.Errorf("not idempotent:\n once:  %q\n twice: %q", rendered, twice)
	}
}

// TestURLLiteralUnderscoreHostStaysProseAfterAColon covers the THIRD reader of
// the same pattern, which the two tests above cannot reach: colonURLParser.
//
// goldmark's linkify triggers on ' ', '*', '_', '~' and '(' only, so a literal
// glued to a preceding ':' — "link:https://…", which remark-gfm links — is this
// package's own inline parser and takes its own route to the pattern. A gate
// wired into the linkify parser alone leaves that route open, and every other
// test here still passes.
func TestURLLiteralUnderscoreHostStaysProseAfterAColon(t *testing.T) {
	t.Parallel()
	const src = "see link:https://ex_x and link:https://a_b.com/x" +
		" and link:https://ex.com end\n"
	want := []string{"https://ex.com"}
	if got := linkURLsOf(Parse([]byte(src))); !equalStrings(got, want) {
		t.Errorf("link URLs of %q = %v, want %v", src, got, want)
	}
}

// linkVerdictsOf is linkURLsOf with the link's TEXT alongside its URL, as
// "text|url". A wrong-href defect is exactly a row where those two disagree,
// so a test that reads only the URL cannot see one.
func linkVerdictsOf(root ast.Node) []string {
	var out []string
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if l, ok := n.(*ast.Link); ok {
			out = append(out, ast.PlainText(l.Children)+"|"+l.URL)
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	walk(root)
	return out
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
