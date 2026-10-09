package adfast

import (
	"strings"
	"testing"
)

// fuzzRoundTripSkip mirrors FuzzRoundTripIdempotent's skip ladder, stage for
// stage, so a test can ask the question the fuzz target asks: would this input
// be measured, or swallowed? Keep it in step with the target — a stage missing
// here reports "not skipped" for an input the fuzzer never reaches, which is
// the one wrong answer this helper can give.
func fuzzRoundTripSkip(md string) (reason string, skip bool) {
	if reason, skip := skipRawInput(md); skip {
		return reason, true
	}
	doc := mdToADF(md)
	if reason, skip := skipDocClasses(doc); skip {
		return reason, true
	}
	first := adfToMD(doc)
	if hasTabText(doc.Content) && !strings.Contains(first, "\t") {
		return "wrapped tab in prose; the reference pipeline is equally unstable", true
	}
	if reason, skip := skipDocLinkClasses(doc); skip {
		return reason, true
	}
	return skipRenderedClasses(first)
}

// THE COLON ESCAPE IS WRITTEN IN EITHER CASE, AND THAT IS A DELIBERATE
// DIVERGENCE FROM THE REFERENCE'S TABLE.
//
// remark-stringify's unsafe table escapes the colon of a bare scheme with the
// row `{character: ':', before: '[ps]', after: '\/'}`, and that row is
// CASE-SENSITIVE. markdown.mdRenderer.colonBeforeSlashEscapes ports the row
// but folds the byte it reads, so an uppercase final scheme letter is escaped
// too. Every row below is a shape that divergence fixed or a control that
// proves it did not overreach; all of them must stay MEASURED, not skipped.
//
// THIS TEST WAS A RED INVENTORY OF LIVE FUZZ FAILURES and the first three rows
// are what it inventoried. Before the fold, an unlinked text node spelled
// "httP://0" got no escape, went out bare, and the next parse LINKIFIED it: a
// plain text node became a link. That is semantic loss, not a byte nit, which
// is why it could not be answered with a skip class:
//
//	round-trip not idempotent for "*httP://*0**":
//	    first:  "_httP://0_"
//	    second: "_<httP://0>_"
//
// THE REFERENCE DID NOT EXCUSE IT. Probed as a POST-LOSS tree (the mdast built
// by hand, stringified — the reference has no ADF hop to lose the link mark in,
// so processSync on the source text would answer a different question), at
// unified 11.0.3 / remark-parse 11.0.0 / remark-gfm 4.0.0 / remark-directive
// 3.0.0 / remark-stringify 11.0.0:
//
//	{"name":"em[text http://, text 0]","s1":"*http\\://0*\n","s2":"*http\\://0*\n","stable":true}
//	{"name":"em[text httP://, text 0]","s1":"*httP://0*\n","s2":"*<httP://0>*\n","stable":false}
//	{"name":"text httP://0","s1":"httP://0\n","s2":"<httP://0>\n","stable":false}
//	{"name":"text HTTPS://x","s1":"HTTPS://x\n","s2":"<HTTPS://x>\n","stable":false}
//
// The same tree one letter's case apart: the reference PROVES its own escape
// works and merely fails to write it. "remark is equally unstable" was
// therefore never available as a skip reason for this class, and writing the
// escape for the uppercase spelling is closer to the table's intent than the
// table's own spelling is. The cost of the divergence is two rows in
// markdown.colonSlashCases, both colons that could never have linkified.
//
// THE WRONG FIX IS CHEAP AND THIS TEST STILL EXISTS TO BLOCK IT. urlLiteralRe,
// the test-side recognizer that drives the "unlinked URL literal in text"
// skip, is lowercase-only. Adding (?i) to it silences every row below in one
// byte, and silences them by making the fuzzer stop looking rather than by
// keeping the renderer correct. The skip assertion in each subtest is what
// refuses that: these inputs round-trip, so a blanket skip of
// "scheme-looking text" would hide a class that works.
func TestSchemeColonIsEscapedAndStableInEitherCase(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		// THE THREE SHAPES THE FOLD FIXED. The first is the one the fuzzer
		// reaches on a warm corpus, and it is the reason no narrower
		// recognizer-based rule could work: the literal exists only ACROSS
		// the ADF text-node seam, because the emphasis boundary splits it
		// into two same-marked text nodes ("httP://" and "0"). No per-node
		// recognizer sees a host there, so the crude two-byte unsafe row is
		// the only rule that still fires.
		{"emphasis boundary splits the literal, uppercase last scheme letter", `*httP://*0**`, "_httP\\://0_\n"},
		{"an authored escape the uppercase render used to drop", `HTTPS\://x`, "HTTPS\\://x\n"},
		{"after an escaped bracket, where both escapes are now written", `[ httP://0`, "\\[ httP\\://0\n"},

		// THE LOWERCASE SIBLINGS, which are the controls: these already
		// round-tripped before the fold, so they prove the escape existed
		// and fired, and they are why a blanket skip could not be the
		// answer. Whatever changes the rows above must leave these alone.
		{"emphasis boundary splits the literal", `*http://*0**`, "_http\\://0_\n"},
		{"an escaped colon that survives the render", `https\://x`, "https\\://x\n"},
		{"after an escaped bracket", `[ http://0`, "\\[ http\\://0\n"},

		// THE ROW THAT PINS WHICH BYTE THE RULE READS. The unsafe row is
		// `before: '[ps]'` — one character — so it is the LAST scheme letter
		// that decides, not the scheme's overall case. Before the fold
		// "HTTPs" was escaped and "httP" was not; after it both are, and a
		// fix keyed on "the scheme is uppercase" instead of on the one byte
		// in front of the colon gets this row wrong in either direction.
		{"a mixed-case scheme whose last letter is lowercase", `*HTTPs://*0**`, "_HTTPs\\://0_\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if reason, skip := fuzzRoundTripSkip(c.md); skip {
				t.Fatalf("%q is skipped as %q, but it round-trips cleanly: the skip "+
					"class widened past the shapes it describes", c.md, reason)
			}
			first := adfToMD(mdToADF(c.md))
			if first != c.want {
				t.Fatalf("first render of %q = %q, want %q: the colon escape must be "+
					"written for either case of the last scheme letter", c.md, first, c.want)
			}
			if second := adfToMD(mdToADF(first)); second != first {
				t.Errorf("round trip of %q is not stable (%q then %q): the colon "+
					"escape no longer holds", c.md, first, second)
			}
		})
	}
}

// A DOTTED HOST IS A DIFFERENT CLASS, AND THE FOLD HANDED IT A SECOND MEMBER.
//
// With a host the autolink literal accepts, writing the escape does not buy a
// round trip: the decoded-text linkify (relinkifyTexts, mirroring
// mdast-util-gfm-autolink-literal's transform) reads the text the escape
// produced and never sees the backslash, so the second pass links it anyway
// and writes the angle autolink. The reference degrades in the same two steps
// on the same shape, which is what makes this a legitimate skip —
// skipRenderedClasses takes it via escapedSchemeColonIgnored, and the rows
// above are the control proving that skip does not reach a host-less literal.
//
// THIS TEST IS WHAT MAKES escapedSchemeColonRe's `(?i)` LOAD-BEARING RATHER
// THAN DECORATIVE, and it could not be written before the fold landed: with a
// case-sensitive renderer the uppercase spelling never produced an escape for
// the pattern to miss, so widening the pattern was an unmutatable no-op. Now
// the pair below is the same tree one letter's case apart, both unstable for
// the identical reason, and a lowercase-only gate covers the first and hands
// the second to the fuzzer as a fresh crasher. Narrow escapedSchemeColonRe
// back to `[ps]\\://` and the uppercase row here fails.
func TestDottedHostSchemeColonIsSkippedInEitherCase(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, first, second string }{
		{"an uppercase last scheme letter", `*httP://*0.0**`, "_httP\\://0.0_\n", "_<httP://0.0>_\n"},
		{"a lowercase last scheme letter", `*HTTp://*0.0**`, "_HTTp\\://0.0_\n", "_<HTTp://0.0>_\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			first := adfToMD(mdToADF(c.md))
			second := adfToMD(mdToADF(first))
			if first != c.first || second != c.second {
				t.Fatalf("round trip of %q = %q then %q, want %q then %q", c.md, first, second, c.first, c.second)
			}
			// The instability is the premise; without it the skip below
			// would be covering nothing and this row would be vacuous.
			if second == first {
				t.Fatalf("round trip of %q is now stable at %q: this row exists to pin "+
					"an UNSTABLE shape as skipped, so a stable one makes it vacuous", c.md, first)
			}
			reason, skip := fuzzRoundTripSkip(c.md)
			if !skip {
				t.Fatalf("%q renders %q then %q and is NOT skipped: the escaped-scheme-colon "+
					"class must cover either case of the byte in front of the colon, or this "+
					"shape is a fuzz crasher the renderer cannot fix (the escape is written "+
					"and the next parse ignores it, exactly as in the reference)", c.md, first, second)
			}
			if want := "escaped colon inside URL scheme"; !strings.Contains(reason, want) {
				t.Errorf("%q is skipped as %q, want the %q class: another class reaching it "+
					"first would make this row stop testing the gate it names", c.md, reason, want)
			}
		})
	}
}
