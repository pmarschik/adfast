package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The whole asset store these probes run under: exactly one path resolves, so
// every probe can hold a resolvable and an unplaceable image side by side in
// one document and see the two fates in the same payload.
const (
	theKnownAsset   = "assets/shot.png"
	theKnownAssetID = "abc-123"
)

func storeWithOneAsset() Option {
	return WithAssetIDResolver(func(ref string) (string, bool) {
		return theKnownAssetID, ref == theKnownAsset
	})
}

// The bug this file exists for: a document whose only content was an image
// the asset store could not place converted to an EMPTY document. The image
// was the sole child of its paragraph, so dropping it emptied the paragraph,
// and an empty paragraph renders to nothing — the whole document gone, with a
// diagnostic the only trace.
//
// The good case rides along in the same document: an image the store DOES know
// still becomes block media, so the fix cannot have been to stop promoting.
func TestUnplaceableLoneImageDoesNotEmptyTheDocument(t *testing.T) {
	const md = "![shot](assets/shot.png)\n\n![diagram](assets/diagram.png \"a caption\")\n"

	got := adfJSON(t, mdToADF(md, storeWithOneAsset()))

	// What the old code produced for the unplaceable half: a paragraph with
	// no content at all.
	if strings.Contains(got, `{"type":"paragraph"}`) {
		t.Errorf("the unplaceable image emptied its paragraph:\n%s", got)
	}
	// The good case: the store knows this one, so it is real block media.
	wantGood := `{"type":"media","attrs":{"alt":"shot","collection":"","id":"abc-123","type":"file"}}`
	if !strings.Contains(got, wantGood) {
		t.Errorf("the resolvable image must still be block media\n  want %s\n  got  %s", wantGood, got)
	}
	// The unplaceable case: the picture is gone, the label is not — and the
	// image's title goes on the link mark's own title attribute, which is
	// where the caption of a picture ADF cannot hold now lands.
	wantDegraded := `{"type":"text","marks":[{"attrs":{"href":"assets/diagram.png","title":"a caption"},` +
		`"type":"link"}],"text":"diagram"}`
	if !strings.Contains(got, wantDegraded) {
		t.Errorf("the unplaceable image must keep its label and its title\n  want %s\n  got  %s", wantDegraded, got)
	}
}

// The same document through the whole md → ADF → md route: the old code
// rendered the unplaceable half away, so a one-image document came back as
// "\n". The route has to return both images' content now.
func TestUnplaceableLoneImageSurvivesTheRoundTrip(t *testing.T) {
	const md = "![shot](assets/shot.png)\n\n![diagram](assets/diagram.png)\n"
	store := storeWithOneAsset()

	doc := mdToADF(md, store)
	got := adfToMD(doc, WithMediaAssets(map[string]convert.MediaAsset{
		"abc-123": {Path: "assets/shot.png"},
	}))

	if got == "\n" || got == "" {
		t.Fatalf("the document converted away entirely, got %q", got)
	}
	if !strings.Contains(got, "assets/shot.png") {
		t.Errorf("the resolvable image must read back as its media, got %q", got)
	}
	if !strings.Contains(got, "[diagram](assets/diagram.png)") {
		t.Errorf("the unplaceable image must read back as its label, got %q", got)
	}
}

// Every block whose only content was such an image was emptied the same way,
// not just a paragraph: a table cell came back blank and a list item came back
// as a bare marker. Each probe carries the resolvable image in the same
// document, in the same kind of block.
func TestUnplaceableImageKeepsTheBlockItSatIn(t *testing.T) {
	store := storeWithOneAsset()

	t.Run("table cell", func(t *testing.T) {
		got := adfJSON(t, mdToADF(
			"| a | b |\n| - | - |\n| ![shot](assets/shot.png) | ![diagram](assets/diagram.png) |\n", store))

		if strings.Contains(got, `{"type":"tableCell","content":[{"type":"paragraph"}]}`) {
			t.Errorf("the unplaceable image emptied its cell:\n%s", got)
		}
		for _, want := range []string{"abc-123", `"text":"diagram"`} {
			if !strings.Contains(got, want) {
				t.Errorf("cell content %s lost:\n%s", want, got)
			}
		}
	})

	t.Run("list item", func(t *testing.T) {
		got := adfJSON(t, mdToADF(
			"- ![shot](assets/shot.png)\n- ![diagram](assets/diagram.png)\n", store))

		if strings.Contains(got, `{"type":"listItem","content":[{"type":"paragraph"}]}`) {
			t.Errorf("the unplaceable image emptied its list item:\n%s", got)
		}
		for _, want := range []string{"abc-123", `"text":"diagram"`} {
			if !strings.Contains(got, want) {
				t.Errorf("list content %s lost:\n%s", want, got)
			}
		}
	})
}

// The linked form lost twice over: the picture had no ADF node, and the link
// mark it should have carried had no node left to ride on either, so the
// destination went with it. The label is what keeps both alive — and the
// ENCLOSING destination is the one it carries, the reason degradeInlineImage
// gives for an external image.
func TestUnplaceableLinkedImageKeepsTheOuterDestination(t *testing.T) {
	const md = "[![the shot](assets/shot.png)](https://home.example/)\n\n" +
		"[![the logo](assets/logo.png)](https://home.example/)\n"

	var dropped []string
	got := adfJSON(t, mdToADF(md,
		storeWithOneAsset(),
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeLinkDestinationDropped {
				dropped = append(dropped, d.Message)
			}
		})))

	if len(dropped) != 0 {
		t.Errorf("the destination has a label to ride on now, want no %s diagnostic, got %v",
			convert.CodeLinkDestinationDropped, dropped)
	}
	// The good case: the store knows this one, so the destination rides on the
	// media node and nothing is lost at all.
	wantGood := `{"type":"media","attrs":{"alt":"the shot","collection":"","id":"abc-123","type":"file"},` +
		`"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`
	if !strings.Contains(got, wantGood) {
		t.Errorf("the resolvable linked image must keep the mark on its media node\n  want %s\n  got  %s",
			wantGood, got)
	}
	wantDegraded := `{"type":"text","marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}],` +
		`"text":"the logo"}`
	if !strings.Contains(got, wantDegraded) {
		t.Errorf("the unplaceable linked image must keep its label and the outer href\n  want %s\n  got  %s",
			wantDegraded, got)
	}
	if strings.Contains(got, "assets/logo.png") {
		t.Errorf("only one of the two hrefs can survive, and it is the outer one:\n%s", got)
	}
}

// An image with no destination at all is unplaceable for a second reason —
// there is nothing to resolve and nothing to link to — and it emptied the
// document just as silently. Its alt text is the content that stays, as plain
// text, because a link mark with no href is not a shape any product accepts.
func TestUnplaceableImageWithNoDestinationKeepsItsAlt(t *testing.T) {
	got := adfJSON(t, mdToADF("![shot](assets/shot.png)\n\n![just the words]()\n",
		storeWithOneAsset()))

	if strings.Contains(got, `{"type":"paragraph"}`) {
		t.Errorf("the image with no destination emptied its paragraph:\n%s", got)
	}
	if !strings.Contains(got, `{"type":"text","text":"just the words"}`) {
		t.Errorf("want the alt text kept unmarked, got:\n%s", got)
	}
	if !strings.Contains(got, "abc-123") {
		t.Errorf("the resolvable image must still be block media:\n%s", got)
	}
}

// With neither a destination nor alt text there is no content to keep and no
// asset to resolve later, so the node contributes nothing — and says nothing.
// A PIN on the one form that still converts away, so the fix above cannot
// creep into inventing a label out of an empty image.
func TestImageWithNothingInItStaysQuiet(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF("![]()\n",
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	if got != `{"type":"doc","content":[{"type":"paragraph"}],"version":1}` {
		t.Errorf("an image with nothing in it must contribute nothing, got:\n%s", got)
	}
	if len(codes) != 0 {
		t.Errorf("nothing was lost, so nothing to report, got %v", codes)
	}
}

// A PIN on the diagnostic: the code is what an upload flow keys off, and the
// consumer that classifies it separates it from the permanent losses because
// an upload makes the next encode find the id. Keeping the label must not have
// stopped the report — the PICTURE is still not on the page.
func TestUnplaceableImageStillReportsUnresolvedAsset(t *testing.T) {
	var messages []string
	mdToADF("![shot](assets/shot.png)\n\n![diagram](assets/diagram.png)\n",
		storeWithOneAsset(),
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeUnresolvedAsset {
				messages = append(messages, d.Message)
			}
		}))

	if len(messages) != 1 {
		t.Fatalf("want one %s diagnostic — for the unplaceable image only — got %v",
			convert.CodeUnresolvedAsset, messages)
	}
	// The sentence a consumer shows the author has to name both halves: the
	// picture that will not be on the page, and the label that will.
	for _, part := range []string{"assets/diagram.png", "picture is dropped", "stays as a link"} {
		if !strings.Contains(messages[0], part) {
			t.Errorf("the diagnostic must say %q, got %q", part, messages[0])
		}
	}
}

// The degradation has to be a fixed point or the round-trip idempotence
// invariant breaks, the same requirement the external inline form carries.
//
// The title travels now: the ADF link mark has a title attribute of its own
// (adf.Link.Title), so the caption the block form would have spelled as a
// mediaSingle caption child rides on the degraded link instead. The
// fixpoint is the interesting half — the degraded form is itself a titled
// link, so a second trip has to leave the title exactly where the first put
// it rather than dropping or doubling it.
func TestUnplaceableImageDegradationIsStable(t *testing.T) {
	once := roundTrip(t, "![diagram](assets/diagram.png \"a caption\")\n")
	twice := roundTrip(t, once)

	if once != twice {
		t.Errorf("degradation is not idempotent:\n first: %q\nsecond: %q", once, twice)
	}
	if once != "[diagram](assets/diagram.png \"a caption\")\n" {
		t.Errorf("unexpected degraded form %q", once)
	}
}

// The format leg must be untouched: convert.NormalizeFormat may reshape an
// author's syntax but never delete content, so an unplaceable image formats as
// the image it is — title and all. A PIN on the boundary between the two legs.
func TestUnplaceableImageFormatsUnchanged(t *testing.T) {
	const md = "![diagram](assets/diagram.png \"a caption\")\n"

	got := ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())

	if got != md {
		t.Errorf("the format leg must keep the image\n want %q\n  got %q", md, got)
	}
}
