package adfast

import "testing"

// The delimiter an emphasis is written with.
//
// '_' has no intraword form in CommonMark, so the renderer used to answer an
// intraword emphasis by hex-encoding the letters around the marker: "a*b*c"
// went out as "&#x61;_&#x62;_&#x63;". Both references write "a*b*c" — prettier
// 3.8.1 by switching to '*' when a word touches the emphasis,
// mdast-util-to-markdown by preferring '*' outright — so the renderer now
// takes '*' wherever '_' would otherwise force the encoding.
//
// "Wherever" is the whole difficulty, and the two run-guard rows below are
// FuzzRoundTripIdempotent finds: '*' is the character strong is ALSO written
// with, so a '*' emphasis marker can be swallowed by a neighboring asterisk
// run in a way the flanking checks — which only classify the neighbor as
// whitespace, punctuation or neither — cannot see.
//
// Every row asserts the rendered markdown AND that re-rendering it is a
// fixpoint, because the defect the guards exist for showed up as a second
// render that disagreed with the first.
func TestEmphasisDelimiterChoice(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{{
		// GOOD case: the whole point of preferring '*'. Before the
		// delimiter choice this rendered "&#x61;_&#x62;_&#x63;".
		name: "intraword emphasis takes the asterisk",
		src:  "a*b*c",
		want: "a*b*c\n",
	}, {
		// GOOD case, non-ASCII: the word class is Unicode's, not ASCII's.
		name: "intraword emphasis takes the asterisk across non-ASCII letters",
		src:  "ä*b*ü",
		want: "ä*b*ü\n",
	}, {
		// PIN: an emphasis '_' can carry keeps '_'. prettier writes the
		// spaced form with underscores and so does this renderer; the
		// delimiter choice must not turn every emphasis into '*'.
		name: "spaced emphasis keeps the underscore",
		src:  "a _b_ c",
		want: "a _b_ c\n",
	}, {
		// GOOD case for the run guard: a strong nearby but not touching
		// the emphasis marker leaves the '*' choice intact.
		name: "asterisk survives a strong that does not touch the marker",
		src:  "a*b*c**d**",
		want: "a*b*c**d**\n",
	}, {
		// FIX (FuzzRoundTripIdempotent, seed 8da4a85e612bbb75). The
		// emphasis ends in a strong and is followed by another, so a '*'
		// closer would sit between two '**' runs. Taking it wrote
		// "0*0**0*****0**\*", whose five-asterisk run re-parsed as a
		// different tree; the second render then produced
		// "0\*0**0**\***0**\*". The guard falls back to '_', which is a
		// different character from '*' and cannot merge with either run.
		name: "emphasis between strong markers falls back to the underscore",
		src:  "0*0****0*0***",
		want: "&#x30;_&#x30;**0**_**0**\\*\n",
	}, {
		// FIX (FuzzRoundTripIdempotent, seed 0ecb3d2fe918a3d2). The
		// emphasis sits inside a strong, so the flanking scratch render
		// has to see '*' as the preceding rune — it used to see a
		// hardcoded '_' stand-in and pick '*' where the real render picks
		// '_'. The strong's trail was then read as '0' where the render
		// wrote ';', its closing "**" was judged flankable when it was
		// not, and the trailing text lost its encoding: the first render
		// wrote "**_&#x30;_&#x30;**0" and the re-parse dropped both marks.
		name: "emphasis inside a strong measures against the strong marker",
		src:  "***0*0**0",
		want: "**_&#x30;_&#x30;**&#x30;\n",
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := adfToMD(mdToADF(tc.src))
			if got != tc.want {
				t.Errorf("render of %q = %q, want %q", tc.src, got, tc.want)
			}
			if again := adfToMD(mdToADF(got)); again != got {
				t.Errorf("render of %q is not a fixpoint:\nfirst:  %q\nsecond: %q", tc.src, got, again)
			}
		})
	}
}
