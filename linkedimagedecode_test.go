package adfast

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The mirror of linkedimage_test.go, for the other direction. Writing a linked
// image reached ADF as media plus a link mark; READING that document back
// dropped the mark, so the destination left in silence and a pull → push cycle
// stripped the href off a picture that already had one.
//
// Every document below carries an UNAFFECTED bare media node and an ordinary
// text link beside the linked one, so a change that "fixes" the linked form by
// mangling either of the plain ones fails here too.

// adfFromJSON decodes a literal ADF document, the shape a pull hands the decode
// leg, into the generic value adfToMD accepts.
func adfFromJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("bad test document: %v", err)
	}
	return doc
}

const (
	// The destination the picture points at, and the mark that carries it.
	testHome     = "https://home.example/"
	testLinkMark = `"marks":[{"type":"link","attrs":{"href":"https://home.example/"}}]`

	// The two rows that must survive untouched in every document.
	goodPlainMedia = `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` +
		`{"type":"media","attrs":{"type":"external","url":"https://x/plain.png","alt":"just a picture"}}]}`
	goodPlainLink = `{"type":"paragraph","content":[{"type":"text","text":"just a link",` +
		`"marks":[{"type":"link","attrs":{"href":"https://elsewhere.example/"}}]}]}`

	wantPlainMedia = "![just a picture](https://x/plain.png)"
	wantPlainLink  = "[just a link](https://elsewhere.example/)"
)

// linkedDoc assembles a document from the linked row plus the two good rows.
func linkedDoc(linked string) string {
	return `{"type":"doc","version":1,"content":[` +
		linked + `,` + goodPlainMedia + `,` + goodPlainLink + `]}`
}

// checkGoodRows asserts the two unaffected rows, so a failure in the linked row
// is visibly a failure of the linked row alone.
func checkGoodRows(t *testing.T, md string) {
	t.Helper()
	if !strings.Contains(md, wantPlainMedia) {
		t.Errorf("the plain image beside it changed; want %q in\n  %q", wantPlainMedia, md)
	}
	if !strings.Contains(md, wantPlainLink) {
		t.Errorf("the plain link beside it changed; want %q in\n  %q", wantPlainLink, md)
	}
}

// The mark sits on the media node, where the schema puts it and where the
// markdown leg writes it, so that is the first place decode looks.
func TestLinkOnMediaDecodesToTheLinkedImageForm(t *testing.T) {
	var codes []string
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			testLinkMark+`}]}`)),
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) }))

	want := "[![the logo](https://x/logo.png)](" + testHome + ")"
	if !strings.Contains(md, want) {
		t.Errorf("the destination left the document; want %q in\n  %q", want, md)
	}
	checkGoodRows(t, md)
	if len(codes) != 0 {
		t.Errorf("a form that loses nothing must report nothing, got %v", codes)
	}
}

// A mark on the mediaSingle wrapper is the placement the schema also allows.
// Decode tolerates it rather than dropping a destination on a technicality.
func TestLinkOnMediaSingleIsTolerated(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},`+testLinkMark+`,"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"}}]}`)))

	want := "[![the logo](https://x/logo.png)](" + testHome + ")"
	if !strings.Contains(md, want) {
		t.Errorf("the wrapper placement was ignored; want %q in\n  %q", want, md)
	}
	checkGoodRows(t, md)
}

// When both placements carry a mark and they disagree, the media node wins: it
// is the placement the reference documents and the one the markdown leg writes,
// so the wrapper is only ever the fallback.
func TestLinkOnBothPlacementsPrefersTheMediaNode(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},`+
			`"marks":[{"type":"link","attrs":{"href":"https://wrapper.example/"}}],"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			`"marks":[{"type":"link","attrs":{"href":"https://media.example/"}}]}]}`)))

	if !strings.Contains(md, "](https://media.example/)") {
		t.Errorf("the media placement must win, got\n  %q", md)
	}
	if strings.Contains(md, "wrapper.example") {
		t.Errorf("only one destination can survive; the wrapper is not it:\n  %q", md)
	}
	checkGoodRows(t, md)
}

// A store-resolvable attachment reads back as the image it came from, and the
// destination rides along.
func TestLinkOnFileMediaDecodesToTheLinkedImageForm(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[`+
			`{"type":"media","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo",`+
			`"width":8,"height":4},`+testLinkMark+`}]}`)),
		testAssets())

	want := "[![the logo](assets/logo.png)](" + testHome + ")"
	if !strings.Contains(md, want) {
		t.Errorf("the destination left the attachment; want %q in\n  %q", want, md)
	}
	checkGoodRows(t, md)
}

// Mid-sentence the picture is a mediaInline, which takes the same mark, so the
// linked wrapper appears inside the sentence.
func TestLinkOnMediaInlineDecodesToTheLinkedImageForm(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"paragraph","content":[{"type":"text","text":"see "},`+
			`{"type":"mediaInline","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo"},`+
			testLinkMark+`},{"type":"text","text":" here"}]}`)),
		testAssets())

	want := "see [![the logo](assets/logo.png)](" + testHome + ") here"
	if !strings.Contains(md, want) {
		t.Errorf("the inline destination left the sentence; want %q in\n  %q", want, md)
	}
	checkGoodRows(t, md)
}

// A media node the image form cannot hold falls back to the ::media directive,
// which has no room for a markdown link — so the destination rides as an
// attribute, the way the border mark already does.
func TestLinkedMediaDirectiveCarriesTheHrefAttribute(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"wide"},"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			testLinkMark+`}]}`)))

	if !strings.Contains(md, `href="`+testHome+`"`) {
		t.Errorf("the directive dropped the destination, got\n  %q", md)
	}
	if !strings.Contains(md, "::media[the logo]") {
		t.Errorf("want the block media directive, got\n  %q", md)
	}
	checkGoodRows(t, md)
}

// The same for the inline directive, the form a mediaInline takes when no asset
// store can turn the id into a path.
func TestLinkedMediaInlineDirectiveCarriesTheHrefAttribute(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"paragraph","content":[{"type":"text","text":"see "},`+
			`{"type":"mediaInline","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo"},`+
			testLinkMark+`},{"type":"text","text":" here"}]}`)))

	if !strings.Contains(md, `href="`+testHome+`"`) {
		t.Errorf("the inline directive dropped the destination, got\n  %q", md)
	}
	if !strings.Contains(md, ":media[the logo]") {
		t.Errorf("want the inline media directive, got\n  %q", md)
	}
	checkGoodRows(t, md)
}

// A mediaGroup member is an attachment row rather than a placed picture, so it
// takes the directive form too — and it also has a destination to keep.
func TestLinkedMediaGroupMemberCarriesTheHrefAttribute(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaGroup","content":[`+
			`{"type":"media","attrs":{"type":"file","id":"abc-123","collection":""},`+
			testLinkMark+`}]}`)))

	if !strings.Contains(md, `href="`+testHome+`"`) {
		t.Errorf("the group member dropped the destination, got\n  %q", md)
	}
	checkGoodRows(t, md)
}

// A caption becomes the image title, which lives on the image INSIDE the link
// wrapper — so the caption and the destination have to coexist, not displace
// each other.
func TestCaptionedLinkedMediaKeepsBothTheCaptionAndTheDestination(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			testLinkMark+`},{"type":"caption","content":[{"type":"text","text":"a caption"}]}]}`)))

	want := `[![the logo](https://x/logo.png "a caption")](` + testHome + `)`
	if !strings.Contains(md, want) {
		t.Errorf("want the captioned linked image\n  %s\ngot\n  %q", want, md)
	}
	checkGoodRows(t, md)
}

// The acceptance criterion: a document with a link mark on media, on
// mediaSingle and on mediaInline survives ADF → md → ADF with the href intact.
func TestLinkedMediaRoundTripsBackToTheSameMark(t *testing.T) {
	cases := []struct {
		name    string
		linked  string
		wantADF string
		opts    []Option
	}{
		{
			name: "on the media node",
			linked: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` +
				`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},` +
				testLinkMark + `}]}`,
			wantADF: `{"type":"media","attrs":{"alt":"the logo","type":"external",` +
				`"url":"https://x/logo.png"},"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`,
		},
		{
			name: "on the mediaSingle wrapper",
			linked: `{"type":"mediaSingle","attrs":{"layout":"center"},` + testLinkMark + `,"content":[` +
				`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"}}]}`,
			wantADF: `{"type":"media","attrs":{"alt":"the logo","type":"external",` +
				`"url":"https://x/logo.png"},"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`,
		},
		{
			name: "on a mediaInline",
			linked: `{"type":"paragraph","content":[{"type":"text","text":"see "},` +
				`{"type":"mediaInline","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo"},` +
				testLinkMark + `},{"type":"text","text":" here"}]}`,
			opts: []Option{testAssets()},
			wantADF: `{"type":"mediaInline","attrs":{"alt":"the logo","collection":"","id":"abc-123",` +
				`"type":"file"},"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]}`,
		},
		{
			name: "on a media node the image form cannot hold",
			linked: `{"type":"mediaSingle","attrs":{"layout":"wide"},"content":[` +
				`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},` +
				testLinkMark + `}]}`,
			wantADF: `"marks":[{"attrs":{"href":"https://home.example/"},"type":"link"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := adfToMD(adfFromJSON(t, linkedDoc(tc.linked)), tc.opts...)
			back := adfJSON(t, mdToADF(md, append(tc.opts, testResolvers()...)...))

			if !strings.Contains(back, tc.wantADF) {
				t.Errorf("the href did not survive the round trip;\n want %s\n  got %s\n via %q",
					tc.wantADF, back, md)
			}
			// The bare picture beside it must still come back bare.
			plain := `{"type":"media","attrs":{"alt":"just a picture","type":"external",` +
				`"url":"https://x/plain.png"}}`
			if !strings.Contains(back, plain) {
				t.Errorf("the plain image gained or lost something:\n  %s", back)
			}
			if !strings.Contains(back, `"text":"just a link"`) {
				t.Errorf("the plain link left the document:\n  %s", back)
			}
		})
	}
}

// The formatter is the other renderer of this shape. It used to delete a link
// wrapped around an image, which would have quietly undone the fix on any
// document that passed through a format step.
func TestFormatterKeepsTheLinkedImageForm(t *testing.T) {
	for _, md := range []string{
		"[![the logo](https://x/logo.png)](" + testHome + ")\n",
		"[![the logo](https://x/logo.png \"a caption\")](" + testHome + ")\n",
		"see [![the logo](assets/logo.png)](" + testHome + ") here\n",
		"![just a picture](https://x/plain.png)\n",
		"[just a link](https://elsewhere.example/)\n",
		// The directive forms carry the destination as an attribute, and the
		// formatter re-derives every attribute from the parsed node, so it
		// has its own chance to drop it.
		`::media[the logo]{href="https://home.example/" layout="wide" type="external" url="https://x/logo.png"}` + "\n",
		`see :media[the logo]{#abc-123 collection href="https://home.example/"} here` + "\n",
	} {
		if got := fmtMD(md); got != md {
			t.Errorf("the formatter changed the document:\n want %q\n  got %q", md, got)
		}
	}
}

// The formatter also canonicalizes the OTHER way: a ::media directive whose
// media the image form can hold collapses to that image — and the collapse has
// to take the destination with it, or formatting a hand-written directive would
// drop the href.
func TestFormatterCollapsesTheLinkedMediaDirectiveToTheLinkedImage(t *testing.T) {
	md := `::media[the logo]{href="https://home.example/" type="external" url="https://x/logo.png"}` + "\n\n" +
		`::media[just a picture]{type="external" url="https://x/plain.png"}` + "\n\n" +
		wantPlainLink + "\n"

	got := fmtMD(md)

	want := "[![the logo](https://x/logo.png)](" + testHome + ")"
	if !strings.Contains(got, want) {
		t.Errorf("the collapse dropped the destination; want %q in\n  %q", want, got)
	}
	checkGoodRows(t, got)
}

// The captioned container collapses the same way, and the caption becomes the
// title of the image INSIDE the link — so both have to arrive.
func TestFormatterCollapsesTheCaptionedLinkedMediaDirective(t *testing.T) {
	md := `:::media[the logo]{href="https://home.example/" type="external" url="https://x/logo.png"}` +
		"\na caption\n:::\n\n" + wantPlainMedia + "\n\n" + wantPlainLink + "\n"

	got := fmtMD(md)

	want := `[![the logo](https://x/logo.png "a caption")](` + testHome + `)`
	if !strings.Contains(got, want) {
		t.Errorf("want the captioned linked image\n  %s\ngot\n  %q", want, got)
	}
	checkGoodRows(t, got)
}

// testAssets is the store that turns the test attachment id back into a path,
// the condition under which media reads back as an image at all.
func testAssets() Option {
	return WithMediaAssets(map[string]convert.MediaAsset{
		"abc-123": {Path: "assets/logo.png", Width: 8, Height: 4, HasDim: true},
	})
}

// testResolvers is the same mapping for the opposite leg, so a round trip can
// return to the attachment it started as.
func testResolvers() []Option {
	return []Option{
		WithAssetIDResolver(func(ref string) (string, bool) {
			return "abc-123", ref == "assets/logo.png"
		}),
		WithImageDimsResolver(func(ref string) (int, int, bool) {
			return 8, 4, ref == "assets/logo.png"
		}),
	}
}
