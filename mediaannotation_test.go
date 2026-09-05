package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The ANNOTATION mark is the third member of the media mark union
// (link | border | annotation): a Confluence inline-comment anchor
// sitting on a picture. MEASURED before this fix — every shape that can
// carry it lost it, with nothing said:
//
//	block external media   → "![alt](https://x/a.png)\n"        diags=[]
//	block file media       → "::media[shot]{#abc-123}\n"        diags=[]
//	mediaInline            → "See :media[shot]{#abc-123 …}.\n"  diags=[]
//	mediaGroup member      → "::media{#abc-123 group=\"true\"}" diags=[]
//
// A dropped anchor is not a cosmetic loss. The mark is what ties the
// picture to its comment thread, so pushing a body without it ORPHANS
// that thread on the remote — which is why the mark is carried rather
// than merely reported: a diagnostic would name the loss without
// preventing it, and the document would still come back wrong.
//
// It rides as annotationId/annotationType directive attributes, the way
// the border mark rides as borderColor/borderSize, and for the same
// reason: :annotation[…] wraps inline CONTENT, and a block media leaf is
// not content it can wrap. The id needs the compound name because a bare
// #id on a ::media node is already the media's own.
//
// Because there is no markdown ATTRIBUTE syntax on an image, an anchor
// also has to BLOCK the image form, exactly as a border does — a
// ![alt](url) has nowhere to put it. Every document below therefore
// carries an unannotated picture and a plain text link beside the
// annotated row, so a change that degrades every picture to a directive
// fails here too.

const (
	// The anchor under test, and the mark that carries it.
	testAnnotationID   = "ann-1"
	testAnnotationMark = `"marks":[{"type":"annotation","attrs":{"id":"ann-1",` +
		`"annotationType":"inlineComment"}}]`
)

// wantAnnotationAttrs is the attribute pair the directive forms spell.
func wantAnnotationAttrs() []string {
	return []string{`annotationId="ann-1"`, `annotationType="inlineComment"`}
}

// checkAnnotationAttrs asserts both attributes reached the markdown.
func checkAnnotationAttrs(t *testing.T, md string) {
	t.Helper()
	for _, want := range wantAnnotationAttrs() {
		if !strings.Contains(md, want) {
			t.Errorf("the anchor left the document; want %s in\n  %q", want, md)
		}
	}
}

// MUTATION A, the whole pre-fix decode: both mediaAnnotationAttrs calls
// removed (the block leaf and the inline chip) together with the
// mediaBlocksImage check. Five of the tests below fail, in every shape
// that can carry the mark, and the parity tripwire next door fails too —
// the format leg keeps the pair, so a pull → format → push cycle would
// hand the remote a document the ADF leg had already stripped. MEASURED:
//
//	--- FAIL: TestAnnotationOnMediaSurvivesAsADirectiveAttribute
//	    the anchor left the document; want annotationId="ann-1" in
//	      "![the logo](https://x/logo.png)\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	    the anchor left the document; want annotationType="inlineComment" in
//	      "![the logo](https://x/logo.png)\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	    want the block media directive, got
//	      "![the logo](https://x/logo.png)\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	    the image form cannot hold the anchor, yet it was used:
//	      "![the logo](https://x/logo.png)\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	--- FAIL: TestAnnotationOnFileMediaBlocksTheImageForm
//	    the anchor left the document; want annotationId="ann-1" in
//	      "![the logo](assets/logo.png)\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	    the attachment collapsed to an image and lost the anchor:
//	      "![the logo](assets/logo.png)\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	--- FAIL: TestAnnotationOnMediaInlineSurvives
//	    the anchor left the document; want annotationId="ann-1" in
//	      "see :media[the logo]{#abc-123 collection} here\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	--- FAIL: TestAnnotationOnAMediaGroupMemberSurvives
//	    the anchor left the document; want annotationId="ann-1" in
//	      "::media{#abc-123 group=\"true\"}\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	--- FAIL: TestAnnotationAndLinkOnOneMediaBothSurvive
//	    the anchor left the document; want annotationId="ann-1" in
//	      "[![the logo](https://x/logo.png)](https://home.example/ \"Home\")\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	    the destination lost href="https://home.example/" to the anchor:
//	      "[![the logo](https://x/logo.png)](https://home.example/ \"Home\")\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	--- FAIL: convert.TestMediaProjectionLegsAgree/annotated_media
//	    the two media projections disagree
//	      in:     "::media[alt]{type=external url=https://x/a.png annotationId=ann-1 annotationType=inlineComment}\n\n…"
//	      format: "::media[alt]{annotationId=\"ann-1\" annotationType=\"inlineComment\" type=\"external\" url=\"https://x/a.png\"}\n\n…"
//	      adf:    "![alt](https://x/a.png)\n\n…"
//
// Each annotationId line above is followed by the matching
// annotationType one over the same document, and three more parity rows
// (annotated file media, annotated media with href and hrefTitle,
// annotated caption container) disagree the same way; they are elided
// here only for length. The round trip fails too — its own mutation
// proof, narrowed to the encode side, sits above that test.
//
// Green under this mutation, honestly: the id-less good case (there was
// never anything to write) and the formatter test (a different leg, with
// its own proof below).
//
// MUTATION B, the blocking rule alone: only the mediaBlocksImage check
// removed, so the attributes are still derived but the picture reaches
// the image form first and there is nowhere to put them. Exactly the
// shapes that HAVE an image form fail — the block external picture, the
// store-resolvable attachment, and the link+anchor pair — while the
// mediaInline and mediaGroup tests stay green, because a chip and an
// attachment row were never images to begin with. MEASURED, the part
// that differs from A:
//
//	--- FAIL: convert.TestMediaProjectionLegsAgree/annotated_media_with_href_and_hrefTitle
//	    the two media projections disagree
//	      format: "::media[alt]{annotationId=\"ann-1\" annotationType=\"inlineComment\" href=\"https://home/\" hrefTitle=\"Home\" type=\"external\" url=\"https://x/a.png\"}\n\n…"
//	      adf:    "[![alt](https://x/a.png)](https://home/ \"Home\")\n\n…"
//
// That row is the whole argument for the blocking rule in one line: the
// destination and its title fit the markdown wrapper, the anchor does
// not, and a projection that takes the wrapper anyway silently drops it.

// An annotated external picture can no longer be a plain image: the
// image form has no room for the anchor, so the whole media falls back
// to the directive that does.
func TestAnnotationOnMediaSurvivesAsADirectiveAttribute(t *testing.T) {
	var codes []string
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			testAnnotationMark+`}]}`)),
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) }))

	checkAnnotationAttrs(t, md)
	if !strings.Contains(md, "::media[the logo]") {
		t.Errorf("want the block media directive, got\n  %q", md)
	}
	if strings.Contains(md, "![the logo](https://x/logo.png)") {
		t.Errorf("the image form cannot hold the anchor, yet it was used:\n  %q", md)
	}
	checkGoodRows(t, md)
	if len(codes) != 0 {
		t.Errorf("a form that loses nothing must report nothing, got %v", codes)
	}
}

// A store-resolvable attachment would otherwise read back as the image
// it came from, so the anchor has to override the store's answer too —
// the blocking rule is one predicate shared by both image paths.
func TestAnnotationOnFileMediaBlocksTheImageForm(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[`+
			`{"type":"media","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo",`+
			`"width":8,"height":4},`+testAnnotationMark+`}]}`)),
		testAssets())

	checkAnnotationAttrs(t, md)
	if strings.Contains(md, "![the logo](assets/logo.png)") {
		t.Errorf("the attachment collapsed to an image and lost the anchor:\n  %q", md)
	}
	checkGoodRows(t, md)
}

// Mid-sentence the picture is a mediaInline, which takes the same mark.
// It has no image form to be degraded out of — a :media[…] chip is
// already the directive — so only the attributes are new here.
func TestAnnotationOnMediaInlineSurvives(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"paragraph","content":[{"type":"text","text":"see "},`+
			`{"type":"mediaInline","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo"},`+
			testAnnotationMark+`},{"type":"text","text":" here"}]}`)))

	checkAnnotationAttrs(t, md)
	if !strings.Contains(md, ":media[the logo]") {
		t.Errorf("want the inline media directive, got\n  %q", md)
	}
	checkGoodRows(t, md)
}

// A mediaGroup member is an attachment row rather than a placed picture,
// so it takes the directive form regardless — and it also has an anchor
// to keep.
func TestAnnotationOnAMediaGroupMemberSurvives(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaGroup","content":[`+
			`{"type":"media","attrs":{"type":"file","id":"abc-123","collection":""},`+
			testAnnotationMark+`}]}`)))

	checkAnnotationAttrs(t, md)
	checkGoodRows(t, md)
}

// Two marks of the union on one node. The link would have been happy in
// the [![alt](url)](href) wrapper, but the anchor blocks that form, so
// BOTH have to reach the directive rather than one displacing the other.
func TestAnnotationAndLinkOnOneMediaBothSurvive(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			`"marks":[{"type":"link","attrs":{"href":"https://home.example/","title":"Home"}},`+
			`{"type":"annotation","attrs":{"id":"ann-1","annotationType":"inlineComment"}}]}]}`)))

	checkAnnotationAttrs(t, md)
	for _, want := range []string{`href="https://home.example/"`, `hrefTitle="Home"`} {
		if !strings.Contains(md, want) {
			t.Errorf("the destination lost %s to the anchor:\n  %q", want, md)
		}
	}
	checkGoodRows(t, md)
}

// An anchor with no id names no comment thread, so there is nothing to
// re-attach on the push and nothing worth degrading a picture for: it is
// not carried, and it does NOT block the image form. This is the same
// answer the inline projection gives — an :annotation with no id
// dissolves to its children — so both spellings of "an anchor without a
// thread" behave alike.
func TestAnAnnotationWithoutAnIDIsNotCarried(t *testing.T) {
	md := adfToMD(adfFromJSON(t, linkedDoc(
		`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`+
			`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},`+
			`"marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment"}}]}]}`)))

	if !strings.Contains(md, "![the logo](https://x/logo.png)") {
		t.Errorf("an id-less anchor must not degrade the picture:\n  %q", md)
	}
	if strings.Contains(md, "annotation") {
		t.Errorf("an id-less anchor has nothing to write, got\n  %q", md)
	}
	checkGoodRows(t, md)
}

// The acceptance criterion: ADF → md → ADF returns the same mark, in
// every shape that can carry one.
//
// MUTATION C, the ENCODE side alone: annotationMarkFromAttrs still runs
// in Media.EncodeADF and MediaInline.EncodeADF, but its result is
// discarded instead of appended to the mark set. The read side is
// untouched, so the directive in the middle of the trip is correct and
// only the way back is broken — the half a decode-only fix would have
// left behind. All four subtests fail, and the parity tripwire catches it
// as well, because the ADF leg now degrades the annotated directive back
// to a bare image. MEASURED:
//
//	--- FAIL: TestAnnotatedMediaRoundTripsBackToTheSameMark/external_media
//	    the anchor did not survive the round trip;
//	     want {"type":"media","attrs":{"alt":"the logo","type":"external","url":"https://x/logo.png"},"marks":[{"attrs":{"annotationType":"inlineComment","id":"ann-1"},"type":"annotation"}]}
//	      got {"type":"doc","content":[{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"alt":"the logo","type":"external","url":"https://x/logo.png"}}]},{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"alt":"just a picture","type":"external","url":"https://x/plain.png"}}]},{"type":"paragraph","content":[{"type":"text","marks":[{"attrs":{"href":"https://elsewhere.example/"},"type":"link"}],"text":"just a link"}]}],"version":1}
//	     via "::media[the logo]{annotationId=\"ann-1\" annotationType=\"inlineComment\" layout=\"center\" type=\"external\" url=\"https://x/logo.png\"}\n\n![just a picture](https://x/plain.png)\n\n[just a link](https://elsewhere.example/)\n"
//	    want exactly one anchor in the document:
//	      {"type":"doc","content":[{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"alt":"the logo","type":"external","url":"https://x/logo.png"}}]},…]}
//	--- FAIL: TestAnnotatedMediaRoundTripsBackToTheSameMark/a_store-resolvable_attachment
//	--- FAIL: TestAnnotatedMediaRoundTripsBackToTheSameMark/a_mediaInline
//	--- FAIL: TestAnnotatedMediaRoundTripsBackToTheSameMark/a_mediaGroup_member
//	--- FAIL: convert.TestMediaProjectionLegsAgree/annotated_media
//	    the two media projections disagree
//	      format: "::media[alt]{annotationId=\"ann-1\" annotationType=\"inlineComment\" type=\"external\" url=\"https://x/a.png\"}\n\n…"
//	      adf:    "![alt](https://x/a.png)\n\n…"
//
// The `via` line is the proof that this mutation is narrower than A: the
// markdown handed to the second half still spells the pair, so the loss
// is entirely on the way back in. The three other subtests fail with the
// same two messages over their own shapes, and three further parity rows
// disagree; elided for length.
func TestAnnotatedMediaRoundTripsBackToTheSameMark(t *testing.T) {
	const wantMark = `"marks":[{"attrs":{"annotationType":"inlineComment","id":"ann-1"},` +
		`"type":"annotation"}]`
	cases := []struct {
		name       string
		annotated  string
		wantADF    string
		opts       []Option
		roundtrips []Option
	}{
		{
			name: "external media",
			annotated: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` +
				`{"type":"media","attrs":{"type":"external","url":"https://x/logo.png","alt":"the logo"},` +
				testAnnotationMark + `}]}`,
			wantADF: `{"type":"media","attrs":{"alt":"the logo","type":"external",` +
				`"url":"https://x/logo.png"},` + wantMark + `}`,
		},
		{
			name: "a store-resolvable attachment",
			annotated: `{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[` +
				`{"type":"media","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo",` +
				`"width":8,"height":4},` + testAnnotationMark + `}]}`,
			opts:       []Option{testAssets()},
			roundtrips: testResolvers(),
			wantADF:    wantMark,
		},
		{
			name: "a mediaInline",
			annotated: `{"type":"paragraph","content":[{"type":"text","text":"see "},` +
				`{"type":"mediaInline","attrs":{"type":"file","id":"abc-123","collection":"","alt":"the logo"},` +
				testAnnotationMark + `},{"type":"text","text":" here"}]}`,
			wantADF: `{"type":"mediaInline","attrs":{"alt":"the logo","collection":"","id":"abc-123",` +
				`"type":"file"},` + wantMark + `}`,
		},
		{
			name: "a mediaGroup member",
			annotated: `{"type":"mediaGroup","content":[` +
				`{"type":"media","attrs":{"type":"file","id":"abc-123","collection":""},` +
				testAnnotationMark + `}]}`,
			wantADF: wantMark,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := adfToMD(adfFromJSON(t, linkedDoc(tc.annotated)), tc.opts...)
			back := adfJSON(t, mdToADF(md, append(tc.opts, tc.roundtrips...)...))

			if !strings.Contains(back, tc.wantADF) {
				t.Errorf("the anchor did not survive the round trip;\n want %s\n  got %s\n via %q",
					tc.wantADF, back, md)
			}
			// The bare picture beside it must come back bare — no anchor of
			// its own, and still an image rather than a directive.
			plain := `{"type":"media","attrs":{"alt":"just a picture","type":"external",` +
				`"url":"https://x/plain.png"}}`
			if !strings.Contains(back, plain) {
				t.Errorf("the plain image gained or lost something:\n  %s", back)
			}
			if strings.Count(back, `"type":"annotation"`) != 1 {
				t.Errorf("want exactly one anchor in the document:\n  %s", back)
			}
		})
	}
}

// The formatter is the other renderer of this shape, and it re-derives
// every directive attribute from the parsed node, so it has its own
// chance to drop the pair. It must also NOT collapse an annotated
// directive to an image, which is the mirror of the blocking rule on the
// decode leg.
//
// MUTATION D, the format-leg mirror alone: normalize.go's
// annotationFromAttrs made to report no id, which is the single read both
// the block leaf and the inline chip go through — and which takes the
// blocksImage check out with it, since an empty annotationID no longer
// blocks. Captured both before and after that read was extracted into
// its own function, with identical output. The ADF leg is untouched,
// so this is the mirror image of A — and it is the more damaging
// direction, because the formatter is contractually TOTAL: it may reshape
// an author's syntax but never delete it. Here it deletes an anchor the
// author had written by hand. MEASURED:
//
//	--- FAIL: TestFormatterKeepsTheAnnotatedMediaDirective
//	    the formatter changed the document:
//	     want "::media[the logo]{annotationId=\"ann-1\" annotationType=\"inlineComment\" layout=\"center\" type=\"external\" url=\"https://x/logo.png\"}\n"
//	      got "![the logo](https://x/logo.png)\n"
//	    the formatter changed the document:
//	     want "::media[the logo]{annotationId=\"ann-1\" annotationType=\"inlineComment\" href=\"https://home.example/\" layout=\"center\" type=\"external\" url=\"https://x/logo.png\"}\n"
//	      got "[![the logo](https://x/logo.png)](https://home.example/)\n"
//	    the formatter changed the document:
//	     want "see\n:media[the logo]{#abc-123 annotationId=\"ann-1\" annotationType=\"inlineComment\" collection}\nhere\n"
//	      got "see :media[the logo]{#abc-123 collection} here\n"
//	--- FAIL: convert.TestMediaProjectionLegsAgree/annotated_media
//	    the two media projections disagree
//	      in:     "::media[alt]{type=external url=https://x/a.png annotationId=ann-1 annotationType=inlineComment}\n\n…"
//	      format: "![alt](https://x/a.png)\n\n…"
//	      adf:    "::media[alt]{annotationId=\"ann-1\" annotationType=\"inlineComment\" layout=\"center\" type=\"external\" url=\"https://x/a.png\"}\n\n…"
//
// Note the third row: the chip loses the pair AND the sentence unwraps,
// because the attributes were what pushed it past the print width. All
// four parity rows disagree with format and adf swapped relative to A,
// which is what makes the tripwire a two-sided one. The two plain rows
// beside them stay green, so the mutation is a deletion of the anchor
// rather than a change of form for every picture.
func TestFormatterKeepsTheAnnotatedMediaDirective(t *testing.T) {
	// in is what the author wrote; want is the canonical form. They differ
	// on one thing only: an external media directive that spells no layout
	// has the required mediaSingle attribute completed with the schema
	// default, exactly as `:status[x]` gains color="neutral" and
	// `::linkEmbed[url]` gains layout="center" (the reference round-trip
	// rows for all three are cited on dialect's mediaSingleFromAttrs). The
	// FORM is what this test is named for, and the form is what the rows
	// pin: the directive survives, anchor and all, rather than collapsing
	// to a picture.
	for _, tc := range []struct{ in, want string }{
		{
			in: `::media[the logo]{annotationId="ann-1" annotationType="inlineComment" ` +
				`type="external" url="https://x/logo.png"}` + "\n",
			want: `::media[the logo]{annotationId="ann-1" annotationType="inlineComment" ` +
				`layout="center" type="external" url="https://x/logo.png"}` + "\n",
		},
		{
			in: `::media[the logo]{annotationId="ann-1" annotationType="inlineComment" ` +
				`href="https://home.example/" type="external" url="https://x/logo.png"}` + "\n",
			want: `::media[the logo]{annotationId="ann-1" annotationType="inlineComment" ` +
				`href="https://home.example/" layout="center" type="external" ` +
				`url="https://x/logo.png"}` + "\n",
		},
		// The chip plus the pair is longer than the print width, so the
		// canonical form has the sentence wrapped around it. This is that
		// form already, which makes the row a fixpoint check on the wrapped
		// spelling rather than on the one-line one. An inline chip has no
		// mediaSingle wrapper, so no layout reaches it.
		{
			in: "see\n" +
				`:media[the logo]{#abc-123 annotationId="ann-1" annotationType="inlineComment" collection}` +
				"\nhere\n",
			want: "see\n" +
				`:media[the logo]{#abc-123 annotationId="ann-1" annotationType="inlineComment" collection}` +
				"\nhere\n",
		},
		// The rows beside them: an unannotated picture stays an image, and
		// a plain link stays a link. Both are fixpoints.
		{in: wantPlainMedia + "\n", want: wantPlainMedia + "\n"},
		{in: wantPlainLink + "\n", want: wantPlainLink + "\n"},
	} {
		if got := fmtMD(tc.in); got != tc.want {
			t.Errorf("the formatter changed the document:\n want %q\n  got %q", tc.want, got)
		}
	}
}
