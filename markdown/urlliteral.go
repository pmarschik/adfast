package markdown

import (
	"regexp"
	"strings"
)

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
// THE PATTERNS ARE NOT THE WHOLE ANSWER. urlLiteralHostAccepted, at the foot
// of this file, is the second half of it: micromark's domain rule refuses an
// underscore in either of the host's last two dot-separated segments, and
// that is a scanner over the segments rather than a character class, so no
// pattern here can carry it. Every caller runs the pattern and then the gate.
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
	//
	// THE TLD CLASS ALSO TAKES '_', and that byte is there for the GATE and
	// not for the pattern: urlLiteralHostAccepted refuses every host with an
	// underscore in its last two segments, so a TLD that uses this byte is
	// rejected as a whole address rather than linked. Without it the pattern
	// stopped at the underscore and the gate then saw a CLEAN shorter host —
	// "https://ex.com_x" matched "https://ex.com" and went out as that href,
	// where the reference reads the whole thing as prose. Reaching the
	// underscore is what lets the gate see it.
	//
	// IT IS ALSO WHY THE GATE STRIPS THE "www." PREFIX BEFORE IT TRIMS. The
	// byte lets this alternative match a domain that is nothing but
	// punctuation — "www.._" is the prefix, an empty segment and the TLD "_"
	// — and trimming that trailing run off the WHOLE literal leaves "www",
	// which has no underscore left to refuse. Found by the format fuzzer:
	// "*www..*" formats to "_www.._" and linked "http://www". The gate reads
	// the domain the reference reads, so an empty one is refused there.
	urlLiteralHostDotted = urlLiteralHostByte + `{0,256}\.` +
		`(?:[a-zA-Z_]|` + urlLiteralHostRune + `)+(?::\d+)?`
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
	// AN UNDERSCORE IN THE LAST TWO SEGMENTS is refused for this alternative
	// as for the two ASCII ones, and not here: the pattern still matches
	// "https://例_x" and urlLiteralHostAccepted is what rejects it. See that
	// function for why the rule cannot live in a pattern.
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
	// The `{2}` is the same per-letter class repeated, not a shortcut past
	// one: gocritic's regexpSimplify asks for it, and it is the spelling and
	// not the rule that changes.
	urlLiteralScheme = `(?:[hH][tT]{2}[pP][sS]?|[fF][tT][pP])://`
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
	//
	// THE PREFIX IS SPELLED PER LETTER, `[wW][wW][wW]\.`, the way
	// urlLiteralScheme spells its own case-insensitivity, because BOTH of the
	// reference's recognizers are case-insensitive here too. Measured against
	// the frozen reference:
	//
	//	"see WWW.ex.com b"   ref links "http://WWW.ex.com"
	//	"see WWW.x b"        ref links "http://WWW.x"
	//	"see Www.Ex.Com b"   ref links "http://Www.Ex.Com"
	//
	// THE PATTERN ALONE WAS NEVER THE GATE, which is why widening it needed a
	// parser to land with it. goldmark's linkify parser reaches its WWW
	// pattern only through a hard-coded
	// `bytes.HasPrefix(line, []byte("www."))`, spelled case-SENSITIVELY in
	// the extension and not configurable — unlike the schemed branch, whose
	// equivalent pre-gate IS configurable and which NewParser therefore opens
	// for both cases (see WithLinkifyAllowedProtocols there). A wider pattern
	// on its own would have left the raw recognizer refusing "WWW." while the
	// DECODED-TEXT scan started accepting it, which is the inversion
	// TestURLLiteralRawPatternIsTheWiderOne forbids for the schemed literal.
	// wwwCaseLinkifyParser in parser.go is the missing half: it claims
	// exactly the spellings goldmark's pre-gate refuses and runs this same
	// pattern on them, so the two recognizers keep one verdict.
	//
	// THE LOWERCASE SPELLING STILL GOES THROUGH GOLDMARK, untouched. The
	// widened letters only make the pattern reachable for a candidate the
	// stock parser would never have handed it.
	urlLiteralWWW = `[wW][wW][wW]\.(?:` + urlLiteralHostDotted +
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

// urlLiteralWWWAnchoredRe matches a SCHEME-LESS "www." literal at the head
// of the input, which is the form goldmark's linkify extension needs: it
// takes the pattern through WithLinkifyWWWRegexp and drops any match not at
// offset 0.
//
// IT EXISTS BECAUSE THE RAW RECOGNIZER HAD BEEN LEFT BEHIND. urlLiteralWWW
// widened the scheme-less host — a dotless "www.x" is a literal, as both of
// the reference's recognizers read it — but only relinkifyTexts, which
// scans DECODED TEXT, was built from it. goldmark kept its own
// `www\.…{1,256}\.[a-z]+`, so the two disagreed about the same bytes, and
// the disagreement was visible through the RAW-SPAN API: Source.Autolinks
// reports goldmark's verdict and nothing else, so "see www.x b" produced a
// link in the tree and NO autolink span at all, while "see www.ex.com b"
// produced both. A caller driving off spans — a linter naming a location,
// a rewriter splicing the source — saw the link vanish for exactly the
// hosts the widening was meant to add.
//
// WHY THE SAME PATTERN AND NOT A SECOND ONE: this is the raw-source side,
// and urlLiteralWWW is what the decoded-text side already uses, so pointing
// both at it is what makes the two verdicts one verdict. The scheme-less
// literal is the one shape where the two sides may share a pattern: unlike
// the schemed host, whose raw and decoded rules genuinely differ (see
// urlLiteralHost against urlLiteralHostDotted), the "www." prefix supplies
// the dot the decoded-text recognizer demands, so both halves of the
// reference accept the same set.
//
// CASE-INSENSITIVE IN THE PREFIX, which goldmark alone cannot use: it gates
// this pattern behind its own hard-coded
// `bytes.HasPrefix(line, []byte("www."))`, so the stock parser never reaches
// it with a "WWW.". wwwCaseLinkifyParser applies the same pattern to exactly
// those spellings, which is what keeps the raw and decoded-text recognizers
// on one verdict. See urlLiteralWWW.
var urlLiteralWWWAnchoredRe = regexp.MustCompile(`^(?:` + urlLiteralWWW + `)`)

// urlLiteralAnchoredRe matches a schemed literal at the head of the input,
// with the raw-source host rule. This is the form an inline parser needs:
// goldmark's linkify extension takes it through WithLinkifyURLRegexp and
// rejects any match not at offset 0, and colonURLParser applies it to the
// bytes right after its ':'.
var urlLiteralAnchoredRe = regexp.MustCompile(
	`^(?:` + urlLiteralScheme + urlLiteralHost + urlLiteralPath + `)`)

// urlLiteralSchemeRe strips the scheme off a candidate so the host can be
// read. It is the scheme pattern above and nothing else, anchored, so the
// two cannot drift; a plain search for "://" would also find one inside a
// path ("www.x/a://b").
var urlLiteralSchemeRe = regexp.MustCompile(`^(?:` + urlLiteralScheme + `)`)

// urlLiteralHostAccepted reports whether a candidate literal — the whole
// address the patterns above matched, scheme or "www." prefix included — has
// a host micromark's domain production accepts: NO UNDERSCORE IN EITHER OF
// THE LAST TWO dot-separated segments.
//
// WHY A FUNCTION AND NOT A PATTERN, which is the whole reason this rule sat
// unmodeled while the widenings around it landed. The rule is a scanner in
// both halves of the reference — micromark's domainAfter counts underscores
// per segment as it consumes the domain, and mdast-util's isCorrectDomain
// splits the host on '.' and tests the last two parts — and it REJECTS AN
// ADDRESS RATHER THAN SHORTENING ONE. A character class cannot do that. Spell
// the segments out in the pattern instead and the engine simply matches a
// shorter host: `https://ex_x` would stop before the underscore and go out as
// the href `https://ex`, and `www.a_b.com` as `http://www.a`, which replaces a
// wrong verdict with a wrong ADDRESS. Go's regexp has no lookahead to say
// "and nothing more of the host follows", so the verdict has to be taken
// after the match, once the whole candidate is in hand.
//
// IT IS NOT A PARITY NIT. The same hole loses content on the format leg, and
// that is what makes it worth the second pass: the formatter may pick '_' as
// the emphasis delimiter, '_' is a legal host byte, and the two decisions
// compose. "*www.*.A" formats to "_www._.A", whose bytes hold a "www."
// literal with the host "www._.A" — so the re-parse links it, the emphasis is
// gone, and a link the author never wrote is in the document. The host's last
// two segments are "_" and "A", so this rule is exactly what refuses it.
//
// THE END GOLDMARK WOULD PICK IS THE END TESTED, via isURLTrailPunct: the
// linkify extension trims a trailing run of `? ! . , : * _ ~` off its match,
// and micromark's domain does the same thing one step earlier — at a '.' or
// '_' it checks whether a trailing-punctuation run to the end of the URL
// starts there and ends the domain before it. So "https://ex_" is the host
// "ex" to both, and testing the untrimmed match would refuse an address both
// recognizers link.
//
// THE TRIM IS WHY THE DOMAIN IS READ THE REFERENCE'S WAY — the scheme or the
// "www." prefix off the FRONT first, and a trailing run gone only when no path
// follows. Both halves matter, and each was measured:
//
//   - Trimming the whole literal first lets the run eat back into the prefix.
//     "www.._" is a match — the TLD class takes '_' — and trimming it whole
//     leaves "www", a clean host with no underscore in sight, so the address
//     linked as "http://www". Stripping the prefix first leaves the domain
//     ".._", which trims to nothing, and an empty domain is no domain: neither
//     recognizer accepts one.
//   - Trimming a literal that HAS a path would refuse the reference's own
//     answer in reverse. micromark ends the domain before a punctuation run
//     only when the run reaches the END OF THE URL, so "https://ex_/y" keeps
//     its underscore and is prose, while "https://ex_" loses it and links.
func urlLiteralHostAccepted(literal string) bool {
	host := literal
	switch m := urlLiteralSchemeRe.FindString(host); {
	case m != "":
		host = host[len(m):]
	case hasWWWPrefix([]byte(host)):
		host = host[len(urlLiteralWWWPrefix):]
	}
	// The host ends where urlLiteralPath may open, and the trim applies to the
	// END OF THE URL — so a literal that HAS a path is not trimmed at all.
	// "https://ex_/y" keeps its underscore in the last segment, exactly as
	// micromark's domain does when the punctuation run does not reach the end.
	if i := strings.IndexAny(host, "/#?"); i >= 0 {
		host = host[:i]
	} else {
		// isURLTrailPunct is goldmark's own trailing set, applied here with
		// one difference that matters: trimURLLiteralEnd stops at the first
		// byte, because the linkify extension must leave SOME address behind,
		// while a domain that is nothing but punctuation is no domain at all
		// and the reference refuses it. Going all the way to empty is what
		// tells the two apart.
		i := len(host)
		for i > 0 && isURLTrailPunct(host[i-1]) {
			i--
		}
		host = host[:i]
	}
	if host == "" {
		return false
	}
	last := host
	rest := ""
	if i := strings.LastIndexByte(host, '.'); i >= 0 {
		last, rest = host[i+1:], host[:i]
	}
	if strings.Contains(last, "_") {
		return false
	}
	if i := strings.LastIndexByte(rest, '.'); i >= 0 {
		rest = rest[i+1:]
	}
	return !strings.Contains(rest, "_")
}

// urlLiteralCandidate returns the raw-source literal at the head of line, nil
// for none. The order is goldmark's own: the schemed pattern first, the
// scheme-less "www." one only when that found nothing.
func urlLiteralCandidate(line []byte) []byte {
	if m := urlLiteralAnchoredRe.Find(line); m != nil {
		return m
	}
	if hasWWWPrefix(line) {
		return urlLiteralWWWAnchoredRe.Find(line)
	}
	return nil
}

// urlLiteralWWWPrefix is goldmark's own scheme-less pre-gate, spelled once.
// It is the LOWERCASE spelling because that is the byte sequence goldmark's
// linkify extension tests for; hasWWWPrefix is what this package uses when
// the question is whether a candidate carries the prefix at all.
var urlLiteralWWWPrefix = []byte("www.")

// hasWWWPrefix reports whether line opens on the scheme-less "www." prefix in
// ANY case, which is how both halves of the reference read it (measured:
// "see WWW.x b" comes back linked as "http://WWW.x"). The comparison is ASCII
// folding written out rather than bytes.EqualFold, for the reason
// urlLiteralScheme spells its letters out: Unicode simple case folding puts
// the long s and the Kelvin sign in the same orbits as ASCII letters, and
// neither recognizer folds that far.
func hasWWWPrefix(line []byte) bool {
	if len(line) < len(urlLiteralWWWPrefix) {
		return false
	}
	return (line[0] == 'w' || line[0] == 'W') &&
		(line[1] == 'w' || line[1] == 'W') &&
		(line[2] == 'w' || line[2] == 'W') && line[3] == '.'
}

// urlLiteralHostEscapable lists the ASCII-punctuation bytes urlLiteralHostByte
// takes — the only bytes a backslash can hide that the host would otherwise
// have consumed.
const urlLiteralHostEscapable = `-@:%._+~#=`

// urlLiteralUnescaped returns line with every backslash that hides a host byte
// removed. It is a VIEW FOR A VERDICT and never for an extent: the offsets
// shift, so nothing may be spliced from it.
func urlLiteralUnescaped(line []byte) []byte {
	out := make([]byte, 0, len(line))
	for i := range line {
		if line[i] == '\\' && i+1 < len(line) &&
			strings.IndexByte(urlLiteralHostEscapable, line[i+1]) >= 0 {
			continue
		}
		out = append(out, line[i])
	}
	return out
}

// urlLiteralHostAcceptedAt reports whether the literal of matchLen bytes at the
// head of line has a host the rule accepts, and it takes the verdict from the
// ESCAPE-FREE VIEW whenever a backslash is what ended the match.
//
// AN ESCAPE MUST NOT DECIDE WHETHER A URL IS A LINK — the principle
// escapedLinkifyParser states for an escape in FRONT of a literal, which the
// host rule makes reachable from INSIDE one. A backslash is not a host byte,
// so it ends the host wherever it sits, and the shortened host it leaves can
// pass a rule the whole one fails. Measured, without this view: the formatter
// renders the plain text "https://例_x" as "https://例\_x" (it escapes an
// underscore after a non-ASCII letter), the re-parse reads the host as "例",
// the rule accepts a dotless host, and a link to "https://例" appears in a
// document whose source had none. Same bytes, two verdicts, and the format leg
// between them.
func urlLiteralHostAcceptedAt(line []byte, matchLen int) bool {
	cand := line[:matchLen]
	if matchLen < len(line) && line[matchLen] == '\\' {
		if v := urlLiteralCandidate(urlLiteralUnescaped(line)); v != nil {
			cand = v
		}
	}
	return urlLiteralHostAccepted(string(cand))
}

// findURLLiterals is urlLiteralRe.FindAllStringIndex with the host gate
// applied, which is the form the DECODED-TEXT scan needs. A rejected match is
// dropped whole rather than retried shorter — that is the reference's own
// behavior, whose transform returns false for the match instead of trimming
// the domain and looking again.
func findURLLiterals(s string) [][]int {
	var out [][]int
	for _, loc := range urlLiteralRe.FindAllStringIndex(s, -1) {
		if urlLiteralHostAccepted(s[loc[0]:loc[1]]) {
			out = append(out, loc)
		}
	}
	return out
}
