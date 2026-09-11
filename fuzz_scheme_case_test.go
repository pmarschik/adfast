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

// AN UPPERCASE SCHEME LETTER BEFORE THE COLON IS A LIVE, UNSKIPPED FUZZ
// FAILURE, AND IT MUST STAY THAT WAY UNTIL THE RENDERER WRITES THE ESCAPE.
//
// The mechanism, measured: remark-stringify's unsafe table escapes the colon
// of a bare scheme with the row `{character: ':', before: '[ps]', after:
// '\/'}`, and that row is CASE-SENSITIVE. adfast ports it in
// markdown.mdRenderer.colonBeforeSlashEscapes. So for an unlinked text node
// whose literal is spelled with a capital last scheme letter — "httP://0" —
// no escape is written, the bytes go out bare, and the next parse linkifies
// them: a plain text node becomes a link. That is semantic loss, not a byte
// nit, and it is why these inputs are errors rather than skips.
//
// The reference does not excuse it. Probed as a POST-LOSS tree (the mdast
// built by hand, stringified — the reference has no ADF hop to lose the link
// mark in, so processSync on the source text would answer a different
// question), at unified 11.0.3 / remark-parse 11.0.0 / remark-gfm 4.0.0 /
// remark-directive 3.0.0 / remark-stringify 11.0.0:
//
//	{"name":"em[text http://, text 0]","s1":"*http\\://0*\n","s2":"*http\\://0*\n","stable":true}
//	{"name":"em[text httP://, text 0]","s1":"*httP://0*\n","s2":"*<httP://0>*\n","stable":false}
//	{"name":"text httP://0","s1":"httP://0\n","s2":"<httP://0>\n","stable":false}
//	{"name":"text HTTPS://x","s1":"HTTPS://x\n","s2":"<HTTPS://x>\n","stable":false}
//
// The same tree one letter's case apart: the reference PROVES its own escape
// works and merely fails to write it. "remark is equally unstable" is
// therefore not available as a skip reason for this class — and note that the
// reference converges on the angle form on the SECOND pass (s3 == s2), so it
// is the 1->2 transition that diverges, not a permanent oscillation.
//
// THE WRONG FIX IS CHEAP AND THIS TEST EXISTS TO BLOCK IT. urlLiteralRe, the
// test-side recognizer that drives the "unlinked URL literal in text" skip,
// is lowercase-only. Adding (?i) to it silences every row below in one byte,
// and silences them by making the fuzzer stop looking rather than by making
// the renderer correct. The stable half is the control: those inputs prove the
// escape exists and fires, so a blanket skip of "scheme-looking text" hides a
// class that is already half-working.
//
// This is a RED INVENTORY, not a green pin: when the escape row is made
// case-insensitive, the "second" bytes below change and this test fails. That
// failure is the deliverable — it names every input to re-measure and move
// into the stable half, and it is cheaper than discovering the set again from
// a fuzz run.
func TestUppercaseSchemeColonIsALiveFuzzFailureNotASkipClass(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, md, first, second string
	}{{
		// The literal exists only ACROSS the ADF text-node seam: the
		// emphasis boundary splits it into two same-marked text nodes
		// ("httP://" and "0"), so no per-node recognizer can see a host
		// here, and the crude two-byte unsafe row is the only rule that
		// still fires. The fuzzer reaches this one on a warm corpus.
		name:   "emphasis boundary splits the literal, uppercase last scheme letter",
		md:     `*httP://*0**`,
		first:  "_httP://0_\n",
		second: "_<httP://0>_\n",
	}, {
		name:   "an escaped colon the uppercase render drops",
		md:     `HTTPS\://x`,
		first:  "HTTPS://x\n",
		second: "<HTTPS://x>\n",
	}, {
		name:   "after an escaped bracket, where the escape is written and the colon's is not",
		md:     `[ httP://0`,
		first:  "\\[ httP://0\n",
		second: "\\[ <httP://0>\n",
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if reason, skip := fuzzRoundTripSkip(c.md); skip {
				t.Fatalf("%q is skipped as %q: this class loses a link mark on "+
					"re-parse and the reference writes the escape for the "+
					"lowercase spelling of the same tree, so it must be measured, "+
					"not swallowed", c.md, reason)
			}
			first := adfToMD(mdToADF(c.md))
			second := adfToMD(mdToADF(first))
			if first != c.first || second != c.second {
				t.Errorf("round trip of %q = %q then %q, want %q then %q: if the "+
					"colon escape is now written for an uppercase scheme letter, "+
					"this row is FIXED — move it to the stable test below",
					c.md, first, second, c.first, c.second)
			}
		})
	}
}

// TestLowercaseSchemeColonIsEscapedAndStable is the control for the inventory
// above and the reason a blanket skip cannot be the answer: the very same
// shapes, spelled with a lowercase last scheme letter, already get their
// colon escaped and already round-trip. Whatever silences the red half must
// leave these measured and stable.
func TestLowercaseSchemeColonIsEscapedAndStable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		{"emphasis boundary splits the literal", `*http://*0**`, "_http\\://0_\n"},
		{"an escaped colon that survives the render", `https\://x`, "https\\://x\n"},
		{"after an escaped bracket", `[ http://0`, "\\[ http\\://0\n"},
		// The unsafe row is `before: '[ps]'`, so it is the LAST scheme letter
		// that decides, not the scheme's overall case: "HTTPs" is escaped and
		// "httP" is not. A fix that keys on "the scheme is uppercase" instead
		// of on the one byte the row reads gets this row wrong.
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
				t.Fatalf("first render of %q = %q, want %q", c.md, first, c.want)
			}
			if second := adfToMD(mdToADF(first)); second != first {
				t.Errorf("round trip of %q is not stable (%q then %q): the colon "+
					"escape no longer holds for a lowercase scheme", c.md, first, second)
			}
		})
	}
}
