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
// TWO KNOWN GAPS, both left open deliberately because closing either widens
// past what was measured:
//
//   - A dotless host the transform still accepts through an EMPTY leading
//     segment ("https://.x" — the reference links it, this does not).
//   - The path. goldmark stops a literal at its own character class where
//     micromark stops it only at whitespace, so "https://ex.com~foo" links
//     through "ex.com" here and through "foo" there. That is a third
//     divergence, not this one, and widening the class would change every
//     literal's extent rather than only a rejected literal's verdict.
const (
	// urlLiteralHostDotted is goldmark's own host rule, and the reference's
	// decoded-text rule: a dot-separated host. Only its TLD class changes,
	// and only in CASE — goldmark spelled it `[a-z]+`, so "EX.COM" was not a
	// host at all. The class is written out as ASCII rather than folded with
	// `(?i)`; see urlLiteralScheme for why that distinction is not cosmetic.
	urlLiteralHostDotted = `[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z]+(?::\d+)?`
	// urlLiteralHost is the raw-source host rule, and the widening: on top of
	// the dotted form it accepts a DOTLESS host that starts alphanumeric
	// ("localhost:8080", "jira"), which is the tokenizer's half of the
	// reference and which an INTRANET name needs. The dotted alternative
	// comes FIRST so a host satisfying both keeps the extent it always had.
	urlLiteralHost = `(?:` + urlLiteralHostDotted +
		`|[a-zA-Z0-9][-a-zA-Z0-9@:%._\+~#=]{0,255})`
	// urlLiteralPath is goldmark's own optional path class, unchanged: a
	// literal's path is only entered through '/', '#' or '?'.
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
	// urlLiteralWWW is the scheme-less "www." literal, kept BYTE-IDENTICAL
	// to what this package matched before. goldmark completes the scheme for
	// it, so it is not a URL as written, and every caller of this package
	// treats it separately; widening it is a change of its own.
	urlLiteralWWW = `www\.[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+(?::\d+)?` + urlLiteralPath
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
