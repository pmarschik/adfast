package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The link TITLE — the advisory text a markdown link spells after its
// destination, [label](href "title") — used to leave the document on the
// ADF leg. MEASURED before the fix: [a](./a.md "T") through md → ADF → md
// came back as [a](./a.md), with nothing said about it.
//
// The ADF link mark has a title attribute of its own, so nothing about
// the format forced that. The attribute is spelled "title": it is in the
// published JSON schema of the shared ADF schema
// (@atlaskit/adf-schema@57.3.0, link_mark: attrs.title, type string,
// additionalProperties false) and on the Jira Cloud ADF reference page
// for the link mark ("Title for the URI", the equivalent of the title
// value of an HTML <a>). See docs/adf-coverage.md for the whole evidence
// trail, including the one question the schema does NOT settle: whether
// the products keep the attribute across a save.
//
// Every document below carries a TITLELESS link beside the titled one, so
// a "fix" that wrote a title onto every link mark, or that reached the
// round trip by dropping the destination, fails these rows too.

const (
	// The two rows every document here carries: the one under test and the
	// plain one that must come out exactly as it went in.
	titledLink   = `[spec](./spec.md "The Spec")`
	titlelessRow = `[guide](./guide.md)`

	// titledDoc places the row under test beside the untitled one.
	titledDoc = "See " + titledLink + " and " + titlelessRow + ".\n"
)

// The core measurement: a link title survives md → ADF → md.
func TestALinkTitleSurvivesTheADFLeg(t *testing.T) {
	md := titledDoc

	if got := roundTrip(t, md); got != md {
		t.Errorf("the link title did not survive md → ADF → md\n  in:  %q\n  out: %q", md, got)
	}
}

// The ADF shape, pinned by attribute name: the title rides on the link
// mark as "title", and a link that spelled none writes no attribute at
// all. Absent rather than empty is the half worth pinning — a "title": ""
// on every link mark adfast produces would be new content no author asked
// for, and a remote read would hand it straight back.
func TestALinkTitleIsSpelledTitleOnTheLinkMark(t *testing.T) {
	got := adfJSON(t, mdToADF(titledDoc))

	want := `{"attrs":{"href":"./spec.md","title":"The Spec"},"type":"link"}`
	if !strings.Contains(got, want) {
		t.Errorf("want the titled link mark\n  want %s\n  got  %s", want, got)
	}
	wantBare := `{"attrs":{"href":"./guide.md"},"type":"link"}`
	if !strings.Contains(got, wantBare) {
		t.Errorf("a titleless link must write no title attribute\n  want %s\n  got  %s", wantBare, got)
	}
}

// A title-carrying link has to be a FIXPOINT, not merely survive one
// trip: the output of the first trip is itself a titled link, so a second
// and a third have to leave the title exactly where the first put it
// rather than dropping, doubling or re-escaping it.
//
// The fixpoint has to sit AT the titled form, which is why the first
// assertion is here as well as in the survival test above: dropping the
// title on every trip is also a fixpoint, and was the behavior before the
// fix.
func TestALinkTitleIsAFixpointAcrossRepeatedTrips(t *testing.T) {
	md := titledDoc

	once := roundTrip(t, md)
	twice := roundTrip(t, once)
	thrice := roundTrip(t, twice)

	if once != md {
		t.Errorf("the fixpoint must be the titled form\n  in:  %q\n  out: %q", md, once)
	}
	if once != twice || twice != thrice {
		t.Errorf("a titled link is not a fixpoint:\n  once:   %q\n  twice:  %q\n  thrice: %q",
			once, twice, thrice)
	}
}

// Every reference spelling reaches the same place, because convert's
// linkref resolution already carried the definition's title onto the
// intermediate link node — the value existed and had nowhere to go. A
// remote read can only ever return the inline form, so that is what the
// trip produces.
func TestALinkTitleOnADefinitionSurvivesTheADFLeg(t *testing.T) {
	want := titledDoc

	for _, tt := range []struct{ name, md string }{
		{
			"shortcut reference",
			"See [spec] and " + titlelessRow + ".\n\n[spec]: ./spec.md \"The Spec\"\n",
		},
		{
			"collapsed reference",
			"See [spec][] and " + titlelessRow + ".\n\n[spec]: ./spec.md \"The Spec\"\n",
		},
		{
			"full reference",
			"See [spec][s] and " + titlelessRow + ".\n\n[s]: ./spec.md \"The Spec\"\n",
		},
	} {
		if got := roundTrip(t, tt.md); got != want {
			t.Errorf("%s: round-tripped to %q, want %q", tt.name, got, want)
		}
	}
}

// The other direction on its own: ADF that ARRIVES with a title attribute
// projects it back into the markdown link, and the decoder says nothing
// about it — before the fix the attribute was unmodeled, so it landed in
// Extra and raised an unknown-attr diagnostic on the way in.
func TestALinkMarkTitleReadsBackFromADF(t *testing.T) {
	doc := []byte(`{"version":1,"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"See "},` +
		`{"type":"text","text":"spec","marks":[{"type":"link","attrs":{"href":"./spec.md","title":"The Spec"}}]},` +
		`{"type":"text","text":" and "},` +
		`{"type":"text","text":"guide","marks":[{"type":"link","attrs":{"href":"./guide.md"}}]},` +
		`{"type":"text","text":"."}]}]}`)

	var codes []string
	got := adfToMD(doc, WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) }))

	if want := titledDoc; got != want {
		t.Errorf("the incoming title did not reach the markdown\n  got  %q\n  want %q", got, want)
	}
	if contains(codes, convert.CodeUnknownAttr) {
		t.Errorf("a modeled attribute must not report as unknown, got %v", codes)
	}
}

// The title travels on an inline-code label too. That path builds its
// mark set by hand rather than through buildMarks — the ADF code mark is
// exclusive, so it drops strong/em/strike — and a second copy of the link
// mark is a second place the title can be forgotten.
func TestALinkTitleSurvivesOnACodeLabel(t *testing.T) {
	md := "See [`spec`](./spec.md \"The Spec\") and [`guide`](./guide.md).\n"

	if got := roundTrip(t, md); got != md {
		t.Errorf("the link title did not survive on a code label\n  in:  %q\n  out: %q", md, got)
	}
}

// PIN, and a deliberate asymmetry rather than a gap this change forgot: a
// link wrapped around a BLOCK image keeps its destination on the media
// node's link mark, and that mark still carries no title.
//
// The media projection's decode is written twice — dialect's decode hooks
// and normalize.go's hand-written mirror of them — and only the pair of
// them can bring an attribute back to "[![alt](url)](href \"title\")". A
// title written on the encode side alone would be a loss dressed as a
// gain: it would sit in the ADF payload with neither leg reading it.
// Recorded as a remaining limit in docs/adf-coverage.md.
//
// The good case rides along: the IMAGE title is not affected, because the
// block form spells that one as a mediaSingle caption child, a sibling
// node rather than an attribute on the mark.
func TestALinkedBlockImageStillDropsTheLinkTitle(t *testing.T) {
	got := adfJSON(t, mdToADF("[![alt](https://x/a.png \"A caption\")](https://home/ \"Home\")\n"))

	if strings.Contains(got, `"title":"Home"`) {
		t.Errorf("the media link mark is not meant to carry a title yet:\n%s", got)
	}
	want := `"marks":[{"attrs":{"href":"https://home/"},"type":"link"}]`
	if !strings.Contains(got, want) {
		t.Errorf("want the titleless media link mark\n  want %s\n  got  %s", want, got)
	}
	if !strings.Contains(got, `{"type":"caption","content":[{"type":"text","text":"A caption"}]}`) {
		t.Errorf("the image title must still be the mediaSingle caption:\n%s", got)
	}
}
