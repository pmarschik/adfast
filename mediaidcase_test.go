package adfast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// The bug this file exists for: one attachment got TWO media ids,
// depending on which markdown spelling named it. The two image spellings
// lowercased whatever the asset store answered; the three ::media
// directive spellings passed it through. Measured against a store
// answering "ABC-123" for assets/logo.png:
//
//	![the logo](assets/logo.png)               -> "id":"abc-123"
//	::media[the logo]{path="assets/logo.png"}  -> "id":"ABC-123"
//
// Only one of those addresses the attachment on the remote, and nothing
// said which.
//
// Which side is right is settled by what a media id IS: the store's name
// for a piece of content, handed to adfast to carry into the payload.
// adfast's own store says so in as many words — it folds case to MATCH an
// id and records "whatever case the product handed over" (assets.idKey) —
// and the reverse leg looks an id up as a plain map key
// (convert.mediaAssets.lookup, an exact match). So a case the encode
// invents is a case nothing can look up again, and the round trip below
// is the proof: an id adfast DECODED came back different from the encode.
//
// The fold is therefore removed rather than added to the directive legs.
// Atlassian media ids are lowercase UUIDs in practice, so folding them
// bought nothing while costing the fidelity a store that names assets
// otherwise depends on.
//
// Every probe below carries a lowercase id beside the mixed-case one, so
// a "fix" that mangled the ordinary Atlassian case would fail here too.

// mixedCaseStore answers a mixed-case id for one path and a lowercase id
// for another, so both fates ride in one document.
func mixedCaseStore() []Option {
	ids := map[string]string{
		"assets/logo.png":  "ABC-123",
		"assets/plain.png": "def-456",
	}
	return []Option{
		WithAssetIDResolver(func(ref string) (string, bool) {
			id, ok := ids[ref]
			return id, ok
		}),
		WithImageDimsResolver(func(ref string) (int, int, bool) {
			_, ok := ids[ref]
			return 10, 20, ok
		}),
	}
}

// Every spelling of "this attachment" must write the id the store
// answered, byte for byte.
func TestStoreMediaIDTravelsVerbatimFromEverySpelling(t *testing.T) {
	cases := []struct {
		name string
		md   string
	}{
		{"a block image", "![%s](%s)\n"},
		{"an inline image", "text ![%s](%s) more\n"},
		{"a media directive", "::media[%s]{path=%s}\n"},
		{"a caption-carrying media directive", ":::media[%s]{path=%s}\nA caption\n:::\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The mixed-case id is what regressed; the lowercase one is the
			// ordinary Atlassian shape and must be untouched either way.
			md := fmt.Sprintf(c.md, "the logo", "assets/logo.png") +
				"\n" + fmt.Sprintf(c.md, "plain", "assets/plain.png")

			got := adfJSON(t, mdToADF(md, mixedCaseStore()...))

			if !strings.Contains(got, `"id":"ABC-123"`) {
				t.Errorf("the store answered ABC-123 and the payload must say so\n  got %s", got)
			}
			if strings.Contains(got, `"id":"abc-123"`) {
				t.Errorf("the id was case-folded; the store's name for the attachment is ABC-123\n  got %s", got)
			}
			if !strings.Contains(got, `"id":"def-456"`) {
				t.Errorf("an already-lowercase id must survive unchanged\n  got %s", got)
			}
		})
	}
}

// An explicit id on the directive is the author's own and was never
// folded; it is pinned here so the two spellings cannot drift apart again
// from the other side either.
func TestExplicitMediaIDTravelsVerbatim(t *testing.T) {
	got := adfJSON(t, mdToADF("::media[the logo]{id=ABC-123}\n\n::media[plain]{id=def-456}\n",
		mixedCaseStore()...))

	for _, want := range []string{`"id":"ABC-123"`, `"id":"def-456"`} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in\n  %s", want, got)
		}
	}
}

// The full circle, and the reason the pass-through side wins: adfast
// DECODES a media id into a local image reference and then ENCODES that
// reference back through the store. The id it returns has to be the id it
// was given, or the payload names an attachment the page does not have.
func TestMediaIDSurvivesTheADFRoundTrip(t *testing.T) {
	const id = "ABC-123"
	in := doc(&adf.MediaSingle{
		Layout: new("align-start"),
		Content: []adf.Node{&adf.Media{
			Type: "file", ID: id, Alt: "the logo",
			Collection: new(""), Width: new(float64(10)), Height: new(float64(20)),
		}},
	})

	md := adfToMD(in, WithMediaAssets(map[string]convert.MediaAsset{
		id: {Path: "assets/logo.png", Width: 10, Height: 20, HasDim: true},
	}))
	if !strings.Contains(md, "![the logo](assets/logo.png)") {
		t.Fatalf("the store knows this id, so the markdown must be a local image, got %q", md)
	}

	got := adfJSON(t, mdToADF(md, mixedCaseStore()...))
	if !strings.Contains(got, `"id":"`+id+`"`) {
		t.Errorf("ADF -> md -> ADF must return the id it was handed (%s)\n  got %s", id, got)
	}
}
