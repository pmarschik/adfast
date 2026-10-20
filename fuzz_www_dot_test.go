package adfast

import (
	"strings"
	"testing"
)

// DOTAFTERWWWESCAPES READS ONE BYTE OF LOOKAHEAD, AND A SECOND DOT IS PAST IT.
//
// The reference's unsafe table row for a '.' after "www" reads `after:
// '[\-.\w]'` — one byte, same as the port. Neither side can see that a LATER
// dot, past a byte the row rejects, still completes a host GFM's www-autolink
// grammar accepts: "www.#." has no escape to write (the after-class rejects
// '#'), and "www..0" has an escape that the decoded-text linkify
// (relinkifyTexts) reads straight past, because the backslash is already
// decoded away by the time that pass runs. Both shapes relink on the next
// parse, and the reference degrades on the identical bytes — see
// wwwDotPunctRe and wwwEscapedDotIgnored for the measurement each cites.
//
// THE CHARACTER SET IN wwwDotPunctRe IS A MEASURED SET, NOT A GUESS. Every
// ASCII punctuation byte between "www." and the second dot was checked
// against the reference individually. Most of them (the controls below)
// round-trip cleanly in BOTH implementations; only '#', '%', '+', '=' relink
// in both. Widening the class to "any punctuation" would skip the controls,
// which is why they are pinned here as must-NOT-skip rows.
func TestWwwHostSecondDotIsSkippedInEitherShape(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, first, second string }{
		// THE UNESCAPED SHAPE: no byte in dotAfterWwwEscapes's after-class
		// sits between the dots, so no escape is ever written.
		{"a lone unescapable byte before the second dot", `*www.#.*`, "_www.#._\n", "_[www.#](http://www.#)._\n"},
		{"the uppercase host spelling", `*WWW.#.*`, "_WWW.#._\n", "_[WWW.#](http://WWW.#)._\n"},
		{"a different unescapable byte", `*www.%.*`, "_www.%._\n", "_[www.%](http://www.%)._\n"},
		{"a third unescapable byte", `*www.+.*`, "_www.+._\n", "_[www.+](http://www.+)._\n"},
		{"a fourth unescapable byte", `*www.=.*`, "_www.=._\n", "_[www.=](http://www.=)._\n"},
		{"trailing content past the second dot", `*www.#..*`, "_www.#.._\n", "_[www.#](http://www.#).._\n"},

		// THE ESCAPED SHAPE: the first dot IS in the after-class (another
		// '.'), so dotAfterWwwEscapes writes the backslash — and the
		// decoded-text linkify ignores it anyway once a valid second label
		// follows.
		{"a chained dot then an alphanumeric label", `*www..*0**`, "_www\\..0_\n", "_[www..0](http://www..0)_\n"},
		{"the uppercase host spelling, chained dot", `*WWW..*0**`, "_WWW\\..0_\n", "_[WWW..0](http://WWW..0)_\n"},
		{"three chained dots before the label", `*www...*0**`, "_www\\...0_\n", "_[www...0](http://www...0)_\n"},
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
				t.Fatalf("%q renders %q then %q and is NOT skipped: the www-host-second-dot "+
					"class must cover this shape, or it is a fuzz crasher the renderer cannot "+
					"fix (the one-byte-wide escape rule misses a later dot, exactly as in the "+
					"reference)", c.md, first, second)
			}
			if want := "www host"; !strings.Contains(reason, want) {
				t.Errorf("%q is skipped as %q, want the %q class: another class reaching "+
					"it first would make this row stop testing the gate it names", c.md, reason, want)
			}
		})
	}
}

// NEITHER SHAPE OVERREACHES, AND THESE ROWS ARE WHAT PROVE IT.
//
// wwwDotPunctRe is a measured set of four bytes, not "any punctuation
// between two dots" — the rows below use other punctuation in the identical
// position and stay stable in both this renderer and the reference, so they
// must stay measured. wwwEscapedDotIgnored is a parser oracle rather than a
// second pattern for the same reason escapedSchemeColonIgnored is one: "-"
// alone is not a valid second label (isCorrectDomain/micromark's domain
// production reject it), so the escape in "www\.-." never had anything to
// lose, and the oracle must say so rather than skip on the shape of the
// bytes alone.
func TestWwwHostWithoutASecondValidLabelIsNotSkipped(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md, want string }{
		{"a byte the autolink grammar rejects as a label", `*www.!.*`, "_www.!._\n"},
		{"an escaped dot with no valid label after it", `*www.-.*0**`, "_www\\.-.0_\n"},
		{"no second dot at all", `www.#`, "www.#\n"},
		{"a word character directly after the first unescapable byte", `www.#x`, "www.#x\n"},
		{"a space before any second dot", `www.# x`, "www.# x\n"},
		{"plain prose with two unrelated dots", `a b.c d`, "a b.c d\n"},
		{"an ordinary linkable host, single dot", `see www.0 b`, "see [www.0](http://www.0) b\n"},
		{"a host that already links at parse time, not render time", `*www..0*`, "_[www..0](http://www..0)_\n"},
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
				t.Errorf("round trip of %q is not stable (%q then %q)", c.md, first, second)
			}
		})
	}
}
