package markdown

import "regexp"

// WHICH BYTES ARE A BARE URL. Three code paths in this package have to agree
// on that answer or a round trip is unstable: goldmark's linkify extension
// (configured in NewParser), colonURLParser for a URL written right after a
// ':', and relinkifyTexts for the decoded text goldmark skipped because it
// was inside a dangling link label. They used to hold three near-copies of
// goldmark's own urlRegexp; the patterns below are the single answer, and
// each of the three is built from them.
//
// THE SHAPE IS GFM's AUTOLINK LITERAL, as the reference implementation
// adfast round-trips against reads it. That reference has TWO recognizers,
// and — this is the part that decides the patterns below — THEIR HOST RULES
// DIFFER:
//
//   - micromark-extension-gfm-autolink-literal's tokenizer reads the RAW
//     SOURCE. It lowercases the scheme before testing it, requires one
//     character after "://" that is neither whitespace nor Unicode
//     punctuation, and requires NO dot in the host. Its own source says so:
//     "Note: that's GH says a dot is needed, but it's not true".
//   - mdast-util-gfm-autolink-literal's post-parse transform reads the
//     DECODED TEXT the tokenizer left behind — the text an escape produced,
//     among others. It is case-insensitive too, but its isCorrectDomain
//     DEMANDS two dot-separated host segments.
//
// That split is measurable and it is not cosmetic. "https\://ex.com" comes
// back from the reference as "<https://ex.com>" — the escape is undone,
// because the decoded text passes the transform's dotted-host test — while
// "https\://x" comes back with the escape intact, because a dotless host
// fails it. So the two patterns here keep the same split: the RAW-SOURCE
// parsers take urlLiteralAnchoredRe, whose host may be dotless, and the
// DECODED-TEXT scan takes urlLiteralRe, whose host may not.
//
// ONE KNOWN GAP, left open deliberately because closing it widens past what
// was measured: THE PATH. goldmark's path must open on '/', '#' or '?' and
// then stays inside goldmark's own character class, where BOTH of the
// reference's recognizers run the path to the next whitespace and trim a
// trailing-punctuation run afterwards. So a literal ends early here wherever
// the next character is neither whitespace nor a path character, and the
// truncated address goes out as the href. Measured against the frozen
// reference, whole bodies:
//
//	"https://ex.com~foo"     ref "https://ex.com~foo"     here "https://ex.com"
//	"https://ex.com,x"       ref "https://ex.com,x"       here "https://ex.com"
//	"https://ex.com—x"       ref "https://ex.com—x"       here "https://ex.com"
//	"https://ex.com€x"       ref "https://ex.com€x"       here "https://ex.com"
//	"https://ex.com/a|b"     ref "https://ex.com/a|b"     here "https://ex.com/a"
//	"https://ex.com/a—b"     ref "https://ex.com/a—b"     here "https://ex.com/a"
//	"https://www.點看.com"    ref "https://www.點看.com"    here "https://www.點看"
//
// THIS ONE IS NOT A CLASS WIDENING, which is why it is still open. The
// reference's stop rule is an algorithm, not a character set: the tokenizer
// consumes to whitespace and then LOOKS AHEAD over a trailing run of
// `! " ' ) * , . : ; ? _ ~`, a `&…;` character reference and a `]` not
// followed by '(' or '[', while the transform trims `[!"&'),.:;<>?\]}]+$`
// with its own paren-balancing pass — two different trail rules for the two
// recognizers. A character class cannot express either, so closing this needs
// a scanner plus a trim, and it moves the extent of literals that already
// link rather than the verdict of literals that do not. Measured over both
// repositories' Markdown, a "path runs to whitespace" candidate moves 3 of
// 212 literals; over the ADF and Markdown fixtures, 31 of 93.
//
// The non-ASCII half of the gap ("—x", "€x") measures as zero corpus movement
// on its own and would fit the negation discipline described below, but it is
// held back with the rest on purpose: unlike the host widenings, which turn a
// WRONG href into the right one, a longer path REPLACES a right href with a
// longer one that merely agrees with the reference. Measured, the reference
// links "https://ex.com的页面。" whole out of "参见https://ex.com的页面。" while
// the address the author meant is "https://ex.com". So the path moves as one
// piece, with the trail rule, or not at all.
const (
	// urlLiteralHostDotted is goldmark's own host rule, and the reference's
	// decoded-text rule: a dot-separated host. Only its TLD class changes,
	// and only in CASE — goldmark spelled it `[a-z]+`, so "EX.COM" was not a
	// host at all. The class is written out as ASCII rather than folded with
	// `(?i)`; see urlLiteralScheme for why that distinction is not cosmetic.
	//
	// THE LEADING SEGMENT MAY BE EMPTY, which goldmark's `{1,256}` forbade.
	// The reference's decoded-text host is `[-.\w]+`, so it may open on a
	// dot, and its isCorrectDomain then SKIPS an empty segment rather than
	// rejecting it (a falsy part fails neither of its two tests). Measured
	// against the frozen reference, "See https://.x end" comes back as
	// "See <https://.x> end" while goldmark left it as prose, which is what
	// an author writing "https://.internal/x" gets. An all-empty host is
	// still rejected: the TLD class needs one character, so "https://." and
	// "https://" match neither alternative, and the reference rejects both
	// too — its splitUrl strips the trailing dot and then refuses the empty
	// remainder.
	//
	// THE TLD CLASS ALSO TAKES A NON-ASCII LETTER, urlLiteralHostRune. Both
	// halves of the reference run "https://ex.coſ/x" to the end of the word;
	// goldmark's ASCII `[a-zA-Z]+` stopped at "ex.co" and emitted THAT as the
	// href, which is the one shape in this file where the wrong URL went out
	// rather than none.
	urlLiteralHostDotted = urlLiteralHostByte + `{0,256}\.` +
		`(?:[a-zA-Z]|` + urlLiteralHostRune + `)+(?::\d+)?`
	// urlLiteralHostByte is goldmark's own host character class, spelled once
	// so the three host alternatives below cannot drift apart. The bytes are
	// goldmark's, unchanged.
	urlLiteralHostByte = `[-a-zA-Z0-9@:%._\+~#=]`
	// urlLiteralHostRune is one NON-ASCII character the reference's
	// raw-source domain consumes: micromark's tokenizer takes any code that
	// is neither whitespace nor `\p{P}`/`\p{S}` (its unicodePunctuation is
	// `/\p{P}|\p{S}/u`), so a letter, a digit or a combining mark from any
	// script joins the host and a dash, a quote or a currency sign does not.
	//
	// ASCII IS DELIBERATELY EXCLUDED, and that is the whole point of writing
	// the class as a negation with `\x00-\x7F` in it. The ASCII half of every
	// host and path class in this file stays byte-for-byte what goldmark
	// shipped, so no ASCII document's literal changes extent; only the
	// non-ASCII half, which goldmark had no rule for at all, gains one.
	//
	// THE TWO RECOGNIZERS REACH THE SAME EXTENT BY DIFFERENT ROUTES, and this
	// class stands in for both. The tokenizer puts "ſ" in the HOST; the
	// decoded-text transform's host is ASCII-only (`[-.\w]+`), so it puts
	// "ſ" in its PATH instead, which is `[^ \t\r\n]*`. Measured against the
	// frozen reference, both routes answer "https://ex.coſ/x" whole:
	//
	//	"See https://ex.coſ/x end"    ->  "See <https://ex.coſ/x> end"
	//	"a [ https://ex.coſ/x b"      ->  "a \\[ <https://ex.coſ/x> b"
	//
	// One class in the shared host therefore matches both, and it keeps the
	// invariant TestURLLiteralRawPatternIsTheWiderOne states. Where the two
	// decompositions DO answer differently is the path gap above: the
	// reference's transform reads "https://ex.coſ~x" whole through its
	// permissive path, and this stops at "ex.coſ".
	urlLiteralHostRune = `[^\x00-\x7F\p{Z}\p{P}\p{S}]`
	// urlLiteralHostUnicodeLed is the RAW-SOURCE-ONLY widening for a host
	// whose FIRST character is non-ASCII: an IDN address, "https://例.com/x"
	// or "https://пример.рф/x". goldmark's two host alternatives both open on
	// an ASCII byte, so such an address was not a host at all and came out as
	// prose. The reference's raw-source tokenizer links it whole: its
	// afterProtocol gate refuses only whitespace and `\p{P}`/`\p{S}` right
	// after the "://", and its domain then consumes any rune that is neither.
	//
	// RAW ONLY, and that asymmetry is the reference's own, measured both ways.
	// Its tokenizer links the address whole; its decoded-text transform
	// REJECTS it outright, because that recognizer's host is the ASCII
	// `[-.\w]+` and needs one such character right after the "://":
	//
	//	"See https://例.com/x end"  ->  "See <https://例.com/x> end"
	//	"a [ https://例.com/x b"    ->  "a \\[ https\\://例.com/x b"
	//
	// So this alternative joins urlLiteralHost and NOT urlLiteralHostDotted,
	// and urlLiteralRe keeps refusing the address.
	//
	// IT CANNOT MATCH AN ASCII ADDRESS, which is what makes it safe to add:
	// the leading urlLiteralHostRune excludes `\x00-\x7F`, so for a host that
	// opens on an ASCII byte this alternative fails at its first character
	// and the pattern falls through to the two goldmark alternatives
	// byte-for-byte. Measured over the parity corpus, its cassette and both
	// repositories' Markdown, it changes the extent of ZERO literals that
	// link today; it only turns prose into a link.
	//
	// A NON-ASCII SEGMENT THAT IS NOT THE FIRST STILL TRUNCATES, deliberately.
	// "https://www.點看.com" (micromark's own example) links whole in BOTH
	// halves of the reference and comes out here as "https://www.點看", and
	// "https://a.coſ.com/x" likewise stops at "https://a.coſ". Both are the
	// PATH gap above rather than a host rule: the reference's transform reads
	// them as an ASCII host ("www.", "a.co") plus its whitespace-terminated
	// path. Widening the raw host to reach them alone would make the raw
	// pattern outrun the decoded-text one and break the same-extent invariant
	// TestURLLiteralRawPatternIsTheWiderOne states.
	//
	// AN UNDERSCORE IN THE LAST TWO SEGMENTS is the one shape this alternative
	// takes and the reference does not ("https://例_x"): micromark's
	// domainAfter refuses it. The ASCII alternatives have carried the same
	// hole since goldmark ("https://ex.com_x" is prose in both halves of the
	// reference and "https://ex.com" here), so modeling the rule belongs to
	// that divergence and not to this one.
	urlLiteralHostUnicodeLed = urlLiteralHostRune +
		`(?:` + urlLiteralHostByte + `|` + urlLiteralHostRune + `)*`
	// urlLiteralHost is the raw-source host rule, and the widening: on top of
	// the dotted form it accepts a DOTLESS host that starts alphanumeric
	// ("localhost:8080", "jira"), which is the tokenizer's half of the
	// reference and which an INTRANET name needs, and the Unicode-led form
	// above. The dotted alternative comes before the dotless one so a host
	// satisfying both keeps the extent it always had; the Unicode-led one is
	// first because it is the only alternative that can open on a non-ASCII
	// character, so it is unreachable for every address the other two accept.
	urlLiteralHost = `(?:` + urlLiteralHostUnicodeLed + `|` + urlLiteralHostDotted +
		`|[a-zA-Z0-9]` + urlLiteralHostByte + `{0,255})`
	// urlLiteralPath is goldmark's own optional path class, unchanged: a
	// literal's path is only entered through '/', '#' or '?' and then stays
	// inside this class. That is the one open gap this file's header
	// describes, and the rows pinned in urlliteral_test.go measure it.
	urlLiteralPath = `(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`
	// urlLiteralScheme is the scheme set goldmark's linkify extension
	// accepts, now matched WITHOUT REGARD TO CASE, which is the reference's
	// rule in BOTH its recognizers and was the whole of the "HTTPS://EX.COM"
	// gap. "ftp" is goldmark's addition to GFM and stays: it is an accepted
	// divergence of its own, and coupling the host rule to the scheme would
	// add a second, undocumented asymmetry.
	//
	// SPELLED OUT PER LETTER, NOT `(?i:https?|ftp)`, and that is a measured
	// difference rather than a style. Go's `(?i)` applies Unicode simple case
	// folding, which puts the long s U+017F in the same orbit as "s" — so the
	// folded form accepts "httpſ://ex.com/x" as a URL. The reference accepts
	// neither of its two recognizers' way: micromark lowercases with
	// JavaScript's .toLowerCase(), which leaves U+017F alone, and the
	// transform's /i canonicalization explicitly refuses to fold a non-ASCII
	// character onto an ASCII one. Measured against the frozen reference,
	// "See httpſ://ex.com/x end" comes back unlinked. Neither is the scheme a
	// candidate for ast.NormalizeLabel's fold, which is a Unicode FULL fold
	// over arbitrary label text and would widen this gate further still.
	urlLiteralScheme = `(?:[hH][tT][tT][pP][sS]?|[fF][tT][pP])://`
	// urlLiteralWWW is the scheme-less "www." literal. goldmark completes the
	// scheme for it, so it is not a URL as written, and every caller of this
	// package treats it separately.
	//
	// THE "www." PREFIX IS ITSELF THE DOT the host rule demands, and that is
	// the whole of the widening here. goldmark spelled the rest of the host
	// as `[-a-zA-Z0-9@:%._+~#=]{1,256}\.[a-z]+`, so it wanted a SECOND dot
	// after the prefix and read "www.x" as prose. Both of the reference's
	// recognizers link it:
	//
	//   - micromark's tokenizer needs no dot in a domain at all (its own
	//     source says so), so "www." plus one domain character is a literal.
	//   - mdast-util-gfm-autolink-literal's transform splits the whole host
	//     on '.' and demands two segments — and "www" is the first of them,
	//     so "www.x" satisfies it where a bare "x" would not.
	//
	// Measured against the frozen reference, "see www.x b" comes back with
	// the host linked while this package left the line as prose. That is a
	// LINK-VERSUS-TEXT divergence rather than a shifted extent: a canonical
	// diff over a document holding a bare "www." host disagreed about the
	// payload, and a push sent prose where the reference sends a link.
	//
	// THE REMAINDER IS THE SCHEMED HOST RULE, reused rather than respelled,
	// so the dotted and dotless forms cannot drift apart from the schemed
	// literal's. Taking urlLiteralHostDotted FIRST is what keeps every
	// address that already linked at the extent it already had: '~' is a
	// host byte, so a lone dotless alternative would read "www.ex.com~foo"
	// whole, where the dotted alternative stops at "www.ex.com" and leaves
	// the tilde to the path gap this file's header describes.
	//
	// urlLiteralHostUnicodeLed is deliberately NOT among the alternatives.
	// The prefix is ASCII, so the character right after it can never be the
	// FIRST of the host — that position is 'w' — and the segment after the
	// prefix is a middle segment, which the header records as truncating in
	// the schemed form too ("https://www.點看.com"). Adding it here would
	// move that row for the scheme-less spelling alone.
	urlLiteralWWW = `www\.(?:` + urlLiteralHostDotted +
		`|[a-zA-Z0-9]` + urlLiteralHostByte + `{0,255})` + urlLiteralPath
)

// urlLiteralRe matches a GFM literal-autolink URL anywhere in a string,
// either form, with the DOTTED host rule. Used by relinkifyTexts, which
// scans decoded text rather than a reader — the reference's own
// decoded-text recognizer wants a dotted host too — so it is deliberately
// unanchored and deliberately narrower than urlLiteralAnchoredRe.
var urlLiteralRe = regexp.MustCompile(
	`(?:` + urlLiteralScheme + urlLiteralHostDotted + urlLiteralPath +
		`|` + urlLiteralWWW + `)`)

// urlLiteralAnchoredRe matches a schemed literal at the head of the input,
// with the raw-source host rule. This is the form an inline parser needs:
// goldmark's linkify extension takes it through WithLinkifyURLRegexp and
// rejects any match not at offset 0, and colonURLParser applies it to the
// bytes right after its ':'.
var urlLiteralAnchoredRe = regexp.MustCompile(
	`^(?:` + urlLiteralScheme + urlLiteralHost + urlLiteralPath + `)`)
