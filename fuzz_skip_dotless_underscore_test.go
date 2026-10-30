package adfast

import "testing"

// A DOTLESS, SCHEME-PREFIXED HOST FUSED TO AN UNDERSCORE USED TO BE AN
// ACCEPTED-UNSTABLE SKIP SHAPE; THE REMARK LEG NOW RENDERS IT STABLY.
//
// goldmark really does linkify a dotless scheme-prefixed host in production
// ("http://0" — see barelinkform_test.go's "a dotless host" row), and its
// linkify pattern extends through a directly-fused '_'. Before the bare
// autolink round-trip check (see markdown/render_bare_autolink.go), the ADF
// leg wrote the link bare against the emphasis marker that followed it, and
// the reparse fused the two:
//
//	md     = "http://0#*0*"
//	first  = "http://0#_0_\n"
//	second = "http://0#_0\_\n"   (not stable — the '_' fuses onto the literal)
//
// That shape was covered by urlFusedWithUnderscore's dotless pass
// (urlFusedSchemeHostRe), a skip class for a hazard the renderer could not
// avoid. It can now: the renderer re-parses the bare candidate together with
// the '_' that follows it, sees the literal swallow the marker, and keeps the
// angle brackets, which no neighbor can fuse onto. The round trip is stable,
// so this pins the stable render rather than the skip.
// TestDotlessHostFusedUnderscoreLeavesNeighborsAlone below still pins the
// narrowness half of the skip class.
func TestDotlessHostFusedUnderscoreRoundTrips(t *testing.T) {
	t.Parallel()
	const md = "http://0#*0*"
	const wantFirst = "<http://0#>_0_\n"
	first := adfToMD(mdToADF(md))
	if first != wantFirst {
		t.Fatalf("first render of %q = %q, want %q (the bare form fuses the "+
			"emphasis marker onto the literal on reparse, so the link must keep "+
			"its angle brackets)", md, first, wantFirst)
	}
	if second := adfToMD(mdToADF(first)); second != first {
		t.Fatalf("round trip of %q is not stable: %q then %q", md, first, second)
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
