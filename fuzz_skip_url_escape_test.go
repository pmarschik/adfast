package adfast

import "testing"

// THE ESCAPED-URL SKIP CLASSES ARE KEYED ON RENDERED BYTES, AND RENDERED
// BYTES MOVE. Each of the three classes in skipRenderedURLClasses says "a
// backslash escape next to a URL literal", and each carries a probe input in
// its comment. The probes are the contract: a class whose own probe no longer
// reaches it is not a narrow class, it is an absent one, and the fuzzer then
// reports its whole family as fresh crashers.
//
// That is exactly what had happened. escapedColonURLRe and escapedPunctURLRe
// were written against a render that spells the literal "www.", and the
// remark unsafe table's '.'-after-'w' row (dotAfterWwwEscapes) later made the
// renderer spell the same literal "www\." — so ":www.0.a" rendered to
// "\:www\.0.a", which the patterns no longer matched. Both classes covered
// nothing, and both of the round trips they exist to excuse came back as
// fuzz failures.
//
// The test below is in two halves, and BOTH halves are load-bearing:
//
//   - THE PROBES MUST STILL BE UNSTABLE AND MUST STILL BE SKIPPED. A skip
//     whose input round-trips cleanly is stale in the other direction and
//     should be deleted rather than kept, so the instability is asserted
//     alongside the skip. If the round trip is ever made stable — it cannot
//     be today; see the reference measurement on the class — this half fails
//     and says which class to remove.
//   - THE SHAPES NEXT DOOR MUST NOT BE SKIPPED. Widening a pattern until the
//     probes match is trivial and worthless: the same widening can swallow
//     every neighboring input, and a skip that hides a real crasher costs
//     more than the one it excuses. The second half pins the stable
//     neighbors — a dotless host the decoded-text gate refuses, a literal
//     that linkified on the FIRST round and therefore needs no escape — as
//     inputs the fuzzer still measures.
func TestEscapedURLSkipClassesCoverTheirProbes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		md     string
		first  string
		reason string
	}{{
		name:   "escaped colon before a www literal whose dot is escaped too",
		md:     ":www.0",
		first:  "\\:www\\.0\n",
		reason: "escaped colon before URL literal; remark is equally unstable",
	}, {
		// The probe named in escapedColonURLRe's own skip comment.
		name:   "the class's documented probe",
		md:     ":www.0.a",
		first:  "\\:www\\.0.a\n",
		reason: "escaped colon before URL literal; remark is equally unstable",
	}, {
		name:   "a colon mid-line, where the directive steal is the same",
		md:     "z:www.a.b c",
		first:  "z\\:www\\.a.b c\n",
		reason: "escaped colon before URL literal; remark is equally unstable",
	}, {
		// The probe named in escapedPunctURLRe's own skip comment.
		name:   "escaped underscore before the same literal",
		md:     ":0_www.0.a",
		first:  ":0\\_www\\.0.a\n",
		reason: "escaped punctuation before URL literal; remark is equally unstable",
	}, {
		name:   "escaped colon inside the scheme, emphasized",
		md:     "*http://*0.0**",
		first:  "_http\\://0.0_\n",
		reason: "escaped colon inside URL scheme; remark is equally unstable",
	}, {
		name:   "the same with a named host",
		md:     "*https://*ex.com**",
		first:  "_https\\://ex.com_\n",
		reason: "escaped colon inside URL scheme; remark is equally unstable",
	}, {
		// The host ".0" has an EMPTY first segment, which isCorrectDomain
		// accepts and an alphanumeric-anchored host pattern does not. The
		// fuzzer reaches this one in about two seconds, so a scheme class
		// keyed on a pattern is a class that lasts one fuzz run.
		name:   "escaped colon before a host whose first segment is empty",
		md:     "*http://.*0**",
		first:  "_http\\://.0_\n",
		reason: "escaped colon inside URL scheme; remark is equally unstable",
	}, {
		// The dot-escaping unsafe row is `before: '[Ww]'`, so a mixed-case
		// "www" renders the same "\." and re-links the same way. The fuzzer
		// reached this one in 77s against a lowercase-only pattern.
		name:   "mixed-case www literal",
		md:     ":wwW.0",
		first:  "\\:wwW\\.0\n",
		reason: "escaped colon before URL literal; remark is equally unstable",
	}, {
		name:   "mixed-case www literal behind an escaped underscore",
		md:     ":0_wWw.0.a",
		first:  ":0\\_wWw\\.0.a\n",
		reason: "escaped punctuation before URL literal; remark is equally unstable",
	}, {
		// The colon-escaping unsafe row is `before: '[ps]'`, so the escape
		// still appears when only the scheme's LAST letter is lowercase.
		name:   "escaped colon after a mixed-case scheme ending in p",
		md:     "*HTTp://*0.0**",
		first:  "_HTTp\\://0.0_\n",
		reason: "escaped colon inside URL scheme; remark is equally unstable",
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			first := adfToMD(mdToADF(c.md))
			if first != c.first {
				t.Fatalf("first render of %q = %q, want %q: the probe no longer "+
					"produces the bytes its class is keyed on", c.md, first, c.first)
			}
			second := adfToMD(mdToADF(first))
			if first == second {
				t.Errorf("round trip of %q is stable at %q: the skip class %q is "+
					"no longer needed and should be deleted, not widened",
					c.md, first, c.reason)
			}
			reason, skip := skipRenderedClasses(first)
			if !skip {
				t.Fatalf("first render %q of %q is not skipped: the class is keyed "+
					"on bytes the renderer no longer writes", first, c.md)
			}
			if reason != c.reason {
				t.Errorf("first render %q of %q skipped as %q, want %q",
					first, c.md, reason, c.reason)
			}
		})
	}
}

// TestEscapedURLSkipClassesLeaveTheStableNeighborsAlone is the other half of
// the pin above: every input here carries a backslash escape next to URL-ish
// bytes and is nonetheless a clean round trip, so a skip on any of them buys
// the probes' coverage by blinding the fuzzer instead of by describing the
// class.
//
// The dotless hosts are the ones that decide the scheme class's shape.
// "https\://x" keeps its escape through the re-parse because
// mdast-util-gfm-autolink-literal's isCorrectDomain demands two dot-separated
// segments and "x" is one — the split urlliteral.go documents between the
// raw-source recognizer and the decoded-text one. So the escaped-scheme class
// cannot key on the escape alone, and this test fails if it starts to.
func TestEscapedURLSkipClassesLeaveTheStableNeighborsAlone(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md string }{
		{"dotless host keeps its escape", "https\\://x"},
		{"dotless host inside emphasis", "*http://*x**"},
		{"schemed literal that linkified on the first round", "https\\://ex.com"},
		{"www literal that linkified on the first round", "a www.0.a b"},
		{"a directive name with no dot after it", ":www"},
		{"escaped colon before a literal the first round already linked", "\\:www.0"},
		{"mixed-case dotless host keeps its escape", "HTTPs\\://x"},
		{"mixed-case dotless host inside emphasis", "*HTTp://*x**"},
		{"a mixed-case scheme that is not stolen by a directive", ":HTTP://0.0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			first := adfToMD(mdToADF(c.md))
			second := adfToMD(mdToADF(first))
			if first != second {
				t.Fatalf("round trip of %q is not stable (%q then %q): this input "+
					"belongs in the skipped half, not here", c.md, first, second)
			}
			if reason, skip := skipRenderedClasses(first); skip {
				t.Errorf("first render %q of %q is skipped as %q, but it round-trips "+
					"cleanly: the class widened past the shapes it describes",
					first, c.md, reason)
			}
		})
	}
}
