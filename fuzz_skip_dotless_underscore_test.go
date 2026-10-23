package adfast

import (
	"strings"
	"testing"
)

// A DOTLESS, SCHEME-PREFIXED HOST IS THE SAME FUSED-UNDERSCORE HAZARD AS A
// DOTTED ONE, AND THE SKIP CLASS HAD A GAP FOR IT.
//
// goldmark really does linkify a dotless scheme-prefixed host in production
// ("http://0" — see markdown/barelinkform_test.go's "a dotless host" row),
// and its linkify pattern extends through a directly-fused '_' exactly as it
// does for a dotted host: the same attention-vs-literal hazard
// urlFusedWithUnderscore exists to recognize. But urlFusedWithUnderscore
// scanned only with urlLiteralRe — the harness's dotted-host approximation of
// the parser's literal recognizer — so a dotless candidate like "http://0"
// was never offered to the detector at all. The fuzzer read the unstable
// round trip below as a brand-new crasher instead of the already-accepted
// fused-underscore class:
//
//	md     = "http://0#*0*"
//	first  = "http://0#_0_\n"
//	second = "http://0#_0\_\n"   (not stable — the '_' fuses onto the literal)
//
// urlFusedSchemeHostRe is the fix: a second, narrower regexp scoped to
// urlFusedWithUnderscore alone, built from the same host and path character
// classes as urlLiteralRe with the dot requirement dropped. It does not widen
// urlLiteralRe itself, so every OTHER skip class that scans with urlLiteralRe
// (the escaped-scheme-colon class, the www-dot-punctuation class, …) is
// unaffected — TestDotlessHostFusedUnderscoreLeavesNeighborsAlone below is
// the control that pins that.
func TestDotlessHostFusedUnderscoreIsSkipped(t *testing.T) {
	t.Parallel()
	const md = "http://0#*0*"
	const wantFirst = "http://0#_0_\n"
	first := adfToMD(mdToADF(md))
	if first != wantFirst {
		t.Fatalf("first render of %q = %q, want %q", md, first, wantFirst)
	}
	second := adfToMD(mdToADF(first))
	// The instability is the premise; without it the skip below would be
	// covering nothing and this test would be vacuous.
	if second == first {
		t.Fatalf("round trip of %q is now stable at %q: this test exists to pin "+
			"an UNSTABLE shape as skipped, so a stable one makes it vacuous", md, first)
	}
	reason, skip := fuzzRoundTripSkip(md)
	if !skip {
		t.Fatalf("%q renders %q then %q and is NOT skipped: the fused-underscore "+
			"class must cover a dotless scheme-prefixed host exactly as it covers a "+
			"dotted one, or this shape is a fuzz crasher the renderer cannot fix "+
			"(goldmark's linkify fuses the marker onto the literal, exactly as it "+
			"does for a dotted host)", md, first, second)
	}
	if want := "underscore"; !strings.Contains(reason, want) {
		t.Errorf("%q is skipped as %q, want a reason containing %q: another class "+
			"reaching it first would make this test stop testing the gate it names",
			md, reason, want)
	}
}

// TestDotlessHostFusedUnderscoreLeavesNeighborsAlone is the narrowness half:
// the widening in urlFusedSchemeHostRe must catch the fused-underscore shape
// above and nothing more. A plain dotless literal with no underscore anywhere
// near it is not the hazard this class describes, and skipping it anyway
// would quietly stop measuring it — a blanket "dotless host" skip instead of
// a "dotless host fused with an underscore" one.
func TestDotlessHostFusedUnderscoreLeavesNeighborsAlone(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md string }{
		{"a bare dotless literal with no underscore in sight", "http://0"},
		{"a dotless literal followed by ordinary prose", "see http://0 there"},
		{"a dotless literal with a path, still no underscore", "http://0/a/b"},
		// The three contexts below DO contain a dotless literal glued to
		// an underscore in the rendered text, but none of them is the
		// fused-underscore hazard: a code span's content and a code
		// block's content are never offered to goldmark's linkify
		// extension at all (they are literal, not prose), and a link
		// title is markdown's own advisory-text attribute, re-emitted
		// verbatim — none of the three ever becomes a live emphasis
		// delimiter next to a live autolink. Scanning the raw rendered
		// string byte-for-byte cannot tell these apart from the prose
		// case the class exists to catch; proseOnlyText (see
		// urlFusedWithUnderscore) can, because it is built from the same
		// re-parse that decides whether linkify ever saw the text.
		{
			"a dotless literal fused with an underscore inside a code span",
			"`http://0_x`",
		},
		{
			"a dotless literal fused with an underscore inside a fenced code block",
			"```\nhttp://0_x\n```",
		},
		{
			"a dotless literal fused with an underscore inside a link title",
			`[a](http://example.com/real "http://0_x")`,
		},
		// A 3-backslash run in front of the underscore re-renders as the
		// same 3-backslash run (odd parity both times: the source's
		// third backslash escapes the '_', and the renderer's own
		// escaping of a literal backslash plus a literal '_' reproduces
		// exactly three). escapedByteAt's old fixed two-byte lookback
		// could only see "two backslashes immediately behind" and so
		// read this as NOT escaped — the opposite of a 1-backslash run's
		// (correctly escaped) classification — which would have flagged
		// this stable round trip as the hazard anyway. See
		// escapedByteAt's doc comment for the parity rule this pins.
		{
			"a dotless literal fused with a 3-backslash-escaped underscore",
			`a\\\_http://0`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			first := adfToMD(mdToADF(c.md))
			second := adfToMD(mdToADF(first))
			if first != second {
				t.Fatalf("round trip of %q is not stable (%q then %q): this input "+
					"belongs in the skipped half, not here", c.md, first, second)
			}
			if reason, skip := fuzzRoundTripSkip(c.md); skip {
				t.Errorf("%q is skipped as %q, but it round-trips cleanly: the "+
					"dotless-host widening reached past the fused-underscore shape "+
					"it describes", c.md, reason)
			}
		})
	}
}
