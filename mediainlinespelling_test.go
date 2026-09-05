package adfast_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// The bug this file exists for: a `:media` chip that ended up carrying no
// attribute and no label had NO markdown spelling that read back as the
// chip.
//
// The renderer wrote such a node as the bare name `:media`, and a bare
// KNOWN inline directive name is prose — that is the bare-name rule in
// `dialect/bare.go`, which is correct and is what stops `see :media in the
// log` from eating a word. So the render was not the node it came from:
// the next parse read the word, and the render after THAT escaped the
// colon to keep the word stable. Measured before the fix, each row being
// `adfToMD(mdToADF(x))` applied until it stops moving:
//
//	":media[ ]"   converged@2  [":media\n"   "\\:media\n"]
//	"0:media{0}"  converged@2  ["0:media\n"  "0\\:media\n"]
//
// Every sibling shape settled in ONE pass, so this was the round-trip
// idempotence invariant broken; a second format or diff of an unchanged
// document reported a change. Worse than the churn, the two passes are a
// media node silently becoming a word.
//
// Two spellings reach the payload-less node and neither is exotic. A
// whitespace-only label is dropped at parse, so `:media[ ]` promotes to a
// chip with nothing on it; an attribute the chip cannot read is dropped on
// the ADF leg, so `:media{0}` re-derives to the same. The third producer
// is the product itself: ADF carries a mediaInline with neither id nor
// collection, and its decode had no attribute left to write.
//
// The repair is to spell the default type rather than nothing, which is
// what the format leg has written since the same rule landed there
// (`convert.unbareSourcelessChip`). It adds no grammar — `:media{type=file}`
// already parsed — and costs nothing in ADF, because the encode infers
// "file" when no type is written.
//
// Every row below carries a GOOD CASE in the same document: a labeled
// chip, which must keep its bare-label spelling, and a second directive
// kind, which must keep its own. A repair that reached further than the
// payload-less chip fails here.
func TestPayloadlessMediaChipKeepsASpellingThatReadsBack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro. The label holds one space, which the parse
			// drops, so the promoted chip carries nothing at all.
			name: "whitespace-only label",
			md:   ":media[ ] and :media[x] and :status[Done]{color=green}",
			want: ":media{type=\"file\"} and :media[x] and :status[Done]{color=\"green\"}\n",
		},
		{
			// The second repro, and a different mover: the attribute is
			// real, so the bare-name rule never fires, but ADF has nowhere
			// to put "0" and the chip comes back empty.
			name: "attribute the chip cannot read",
			md:   "0:media{0} and :media[x] and :mention[Bob]{id=1}",
			want: "0:media{type=\"file\"} and :media[x] and :mention[Bob]{#1}\n",
		},
		{
			// The default spelled explicitly is the same node, so it takes
			// the same repair instead of collapsing to the bare name.
			name: "the default type spelled out",
			md:   ":media{type=file} and :media[x]",
			want: ":media{type=\"file\"} and :media[x]\n",
		},
		{
			// A chip that addresses something never had the problem and
			// must not gain a filler attribute.
			name: "an addressed chip is left alone",
			md:   ":media{#abc} and :media[x]",
			want: ":media{#abc} and :media[x]\n",
		},
		{
			// A non-file type already terminates the form; the repair must
			// write the type the node has, not the default.
			name: "a non-file type is kept as it is",
			md:   ":media{type=external} and :media[x]",
			want: ":media{type=\"external\"} and :media[x]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			first := roundTripMarkdown(tt.md)
			if first != tt.want {
				t.Fatalf("first render = %q, want %q", first, tt.want)
			}
			if second := roundTripMarkdown(first); second != first {
				t.Fatalf("the round trip must settle in ONE pass:\n first:  %q\n second: %q", first, second)
			}
		})
	}
}

// The product's own shape, from the ADF side: a mediaInline with neither
// id nor collection is what an unadorned inline attachment looks like
// coming back, and its decode is the third producer of the payload-less
// chip (convert's VisitMediaInline).
//
// It used to come back as the word `:media`, so a pull wrote a word into
// the file and the next push sent text where a media node had been —
// silent, because both sides then agreed on the text. It now keeps a
// spelling, so the node survives the trip.
//
// The good cases ride the same document: an addressed chip beside it,
// which keeps its own spelling, and a status, which is a different kind.
func TestSourcelessChipFromADFRoundTripsThroughMarkdown(t *testing.T) {
	t.Parallel()
	const src = `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"mediaInline","attrs":{"type":"file"}},` +
		`{"type":"text","text":" and "},` +
		`{"type":"mediaInline","attrs":{"type":"file","id":"abc-123","collection":"c1"}},` +
		`{"type":"text","text":" and "},` +
		`{"type":"status","attrs":{"text":"Done","color":"green"}}]}]}`
	var doc adf.Doc
	if err := json.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	first := adfast.ToMarkdown(adfast.FromADF(doc))
	const want = ":media{type=\"file\"} and :media{#abc-123 collection=\"c1\"} and\n" +
		":status[Done]{color=\"green\"}\n"
	if first != want {
		t.Fatalf("render = %q, want %q", first, want)
	}
	// The point of the spelling: the chip is still a chip after the trip.
	// A word here is the defect this test exists for.
	back, err := json.Marshal(adfast.ToADF(adfast.FromMarkdown(first)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantNode = `{"type":"mediaInline","attrs":{"type":"file"}}`
	if !strings.Contains(string(back), wantNode) {
		t.Errorf("the chip must survive the trip\n  want %s\n  in   %s", wantNode, back)
	}
	if second := roundTripMarkdown(first); second != first {
		t.Fatalf("the round trip must settle in ONE pass:\n first:  %q\n second: %q", first, second)
	}
}

// A mediaInline may arrive with no attrs object at all, so the decode
// carries an EMPTY type through. The encode reads an absent type as
// "file", so that is the only default the repair may write: anything else
// would not survive its own round trip, and the document would need two
// passes again for a new reason.
func TestSourcelessChipWithNoTypeAtAllTakesTheEncodesDefault(t *testing.T) {
	t.Parallel()
	const src = `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"mediaInline"}]}]}`
	var doc adf.Doc
	if err := json.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	first := adfast.ToMarkdown(adfast.FromADF(doc))
	if want := ":media{type=\"file\"}\n"; first != want {
		t.Fatalf("render = %q, want %q", first, want)
	}
	if second := roundTripMarkdown(first); second != first {
		t.Fatalf("the round trip must settle in ONE pass:\n first:  %q\n second: %q", first, second)
	}
}
