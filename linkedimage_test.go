package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// A linked image — a logo that links home, a badge that links to a build — used
// to reach ADF as a picture with no destination: the outer href vanished with
// nothing said about it, so a push published an unclickable image. ADF does
// carry it: the mark set of media and mediaInline is
// (link | border | annotation), so the destination belongs on the media node.
//
// Every document below carries an UNAFFECTED plain image and plain link beside
// the linked one, so a change that "fixes" the linked form by mangling either
// of the ordinary ones fails here too.
const linkedImageDoc = "[![the logo](https://x/logo.png)](https://home.example/)\n\n" +
	"![just a picture](https://x/plain.png)\n\n" +
	"[just a link](https://elsewhere.example/)\n"

// linkedImageRefDoc says the same thing in the reference form, which is the
// spelling the defect was found in. Both halves may be a reference, so both are
// written as one here.
const linkedImageRefDoc = "[![the logo][logo]][home]\n\n" +
	"![just a picture](https://x/plain.png)\n\n" +
	"[just a link](https://elsewhere.example/)\n\n" +
	"[logo]: https://x/logo.png\n[home]: https://home.example/\n"

// The block form loses nothing: the picture stays block media and the
// destination rides on the media node as the link mark ADF puts there.
func TestLinkedImageKeepsTheOuterDestination(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF(linkedImageDoc,
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	want := `{"type":"media","attrs":{"alt":"the logo","type":"external",` +
		`"url":"https://x/logo.png"},"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`
	if !strings.Contains(got, want) {
		t.Errorf("want the linked media\n  %s\ngot\n  %s", want, got)
	}
	// The plain image beside it must stay a bare media node…
	plain := `{"type":"media","attrs":{"alt":"just a picture","type":"external","url":"https://x/plain.png"}}`
	if !strings.Contains(got, plain) {
		t.Errorf("the plain image changed; want\n  %s\ngot\n  %s", plain, got)
	}
	// …and the plain link a text node with a link mark.
	link := `{"type":"text","marks":[{"attrs":{"href":"https://elsewhere.example/"},"type":"link"}],` +
		`"text":"just a link"}`
	if !strings.Contains(got, link) {
		t.Errorf("the plain link changed; want\n  %s\ngot\n  %s", link, got)
	}
	if len(codes) != 0 {
		t.Errorf("a form that loses nothing must report nothing, got %v", codes)
	}
}

// The reference form is the same document, so it must produce the same ADF —
// including the same two unaffected neighbors.
func TestLinkedImageReferenceFormMatchesTheInlineForm(t *testing.T) {
	inline := adfJSON(t, mdToADF(linkedImageDoc))
	reference := adfJSON(t, mdToADF(linkedImageRefDoc))

	if inline != reference {
		t.Errorf("the two forms of one linked image diverge:\n inline: %s\n    ref: %s", inline, reference)
	}
}

// An unreferenced definition reports its dropped destination; a definition the
// linked image resolved must NOT, or the fix would trade one wrong warning for
// another.
func TestLinkedImageReferenceMarksBothDefinitionsUsed(t *testing.T) {
	var codes []string
	mdToADF(linkedImageRefDoc,
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) }))

	if contains(codes, convert.CodeUnusedDefinitionDropped) {
		t.Errorf("both definitions are referenced by the linked image, got %v", codes)
	}
}

// A path the asset store knows becomes the attachment's own media node, exactly
// as the bare image does, and the destination goes on with it.
func TestStoreResolvableLinkedImageKeepsTheDestination(t *testing.T) {
	got := adfJSON(t, mdToADF("[![the logo](assets/logo.png)](https://home.example/)\n",
		WithAssetIDResolver(func(ref string) (string, bool) {
			return "abc-123", ref == "assets/logo.png"
		})))

	want := `{"type":"media","attrs":{"alt":"the logo","collection":"","id":"abc-123","type":"file"},` +
		`"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`
	if !strings.Contains(got, want) {
		t.Errorf("want the linked attachment media\n  %s\ngot\n  %s", want, got)
	}
}

// Mid-sentence, a store-resolvable linked image is a mediaInline — which takes
// the same link mark, so this form loses nothing either.
func TestLinkedMediaInlineKeepsTheDestination(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF("see [![the logo](assets/logo.png)](https://home.example/) here\n",
		WithAssetIDResolver(func(ref string) (string, bool) {
			return "abc-123", ref == "assets/logo.png"
		}),
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	want := `{"type":"mediaInline","attrs":{"alt":"the logo","collection":"","id":"abc-123","type":"file"},` +
		`"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`
	if !strings.Contains(got, want) {
		t.Errorf("want the linked mediaInline\n  %s\ngot\n  %s", want, got)
	}
	if len(codes) != 0 {
		t.Errorf("a form that loses nothing must report nothing, got %v", codes)
	}
}

// Mid-sentence with an EXTERNAL image there is no inline media at all to hang a
// mark on, so the degraded link keeps the destination the reader clicks and the
// image URL is what leaves — said out loud, not silently.
func TestExternalLinkedImageInlineKeepsTheOuterHrefAndReportsTheLoss(t *testing.T) {
	var messages []string
	got := adfJSON(t, mdToADF("see [![the logo](https://x/logo.png)](https://home.example/) here\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeInlineImageDegraded {
				messages = append(messages, d.Message)
			}
		})))

	href := `{"type":"text","marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}],` +
		`"text":"the logo"}`
	if !strings.Contains(got, href) {
		t.Errorf("the degraded link must point at the outer destination, got\n  %s", got)
	}
	if strings.Contains(got, "https://x/logo.png") {
		t.Errorf("the image URL cannot also be the href; only one can survive:\n  %s", got)
	}
	if len(messages) != 1 {
		t.Fatalf("want one degraded-image diagnostic, got %v", messages)
	}
	for _, part := range []string{"https://x/logo.png", "https://home.example/", "dropped"} {
		if !strings.Contains(messages[0], part) {
			t.Errorf("the diagnostic must name %q; got %q", part, messages[0])
		}
	}
}

// The last way a destination can leave: the label converts to nothing ANYWHERE
// in it, so an ADF link mark has no node to ride on. An image with neither a
// destination nor alt text is the one markdown form that reaches it — an image
// the store cannot place still leaves its label behind for the mark (see
// TestUnplaceableLinkedImageKeepsTheOuterDestination). The href still may not
// disappear in silence.
func TestEmptyLabelledLinkReportsTheDroppedDestination(t *testing.T) {
	var messages []string
	mdToADF("[![]()](https://home.example/)\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeLinkDestinationDropped {
				messages = append(messages, d.Message)
			}
		}))

	if len(messages) != 1 {
		t.Fatalf("want one %s diagnostic, got %v", convert.CodeLinkDestinationDropped, messages)
	}
	if !strings.Contains(messages[0], "https://home.example/") {
		t.Errorf("the diagnostic must name the dropped destination, got %q", messages[0])
	}
}

// A link with no destination has nothing to carry, and a link mark without an
// href is not a shape any product accepts, so the empty form adds no mark.
func TestLinkedImageWithAnEmptyDestinationAddsNoMark(t *testing.T) {
	got := adfJSON(t, mdToADF("[![the logo](https://x/logo.png)]()\n"))

	if strings.Contains(got, `"marks"`) {
		t.Errorf("an empty destination must add no mark, got\n  %s", got)
	}
}
