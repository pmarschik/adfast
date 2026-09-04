package convert

// The one measured exception to "ToADF is invariant under Normalize",
// pinned shape by shape.
//
// Where the pass degrades a ::media directive to an ast.Image, the two
// encodes can differ by ONE byte-level fact: the mediaSingle wrapper's
// layout attribute. The image form spells the placement — center for
// external media (ast_to_adf.go's loneImageMedia), align-start for a
// downloaded attachment (attachmentImageToMedia) — while the directive
// form (dialect's mediaSingleFromAttrs) re-infers align-start for a
// FILE-type directive that carries no layout and spells nothing at all
// for any other type. So the gap is exactly one shape: EXTERNAL media
// that omits a layout and still reaches the image form. Every other
// media shape agrees, and the table below says why for each.
//
// This file is not about the format leg, so it is not in
// mediaparity_test.go's divergence table: that table compares the two
// projections' rendered MARKDOWN, and both legs render these rows
// identically. What differs here is the JSON a push would send, on the
// encode leg alone.
//
// WHICH FORM IS WRONG, measured rather than assumed. center is not the
// image form's invention:
//
//   - Atlassian's adf-schema declares the attribute as
//     `layout: { default: 'center' }` on mediaSingle (media-single.ts,
//     the file docs/adf-coverage.md already cites for the node), and its
//     DOM parse reads `dom.getAttribute('data-layout') || 'center'`.
//   - Jira's ADF node reference marks attrs.layout REQUIRED for
//     mediaSingle, with no documented default.
//   - testdata/directive_fixtures.json, generated from the reference
//     implementations, pins `![shot](https://example.com/i.png)` →
//     mediaSingle{"layout":"center"} in the md→ADF direction, and pins
//     the reverse direction KEEPING layout="center" on an external
//     ::media directive.
//
// So the directive form is the under-spelling one, and closing the gap
// belongs on that side — in dialect's mediaSingleFromAttrs, whose
// `else if media.Type == "file"` branch already performs exactly this
// re-inference for one type. Measured, that one line does close the gap
// for every row below. It cannot land alone, though: the ADF round trip
// then writes layout="center" into four documents whose author never
// spelled one (TestMediaProjectionLegsAgree's bordered-media,
// relative-url, annotated-caption and occurrence-key rows all fail with
// the attribute added), and a round trip may no more ADD to an author's
// document than delete from it. Pairing it with the mirrored omission on
// the decode side — the counterpart of the align-start omission in
// dialect's mediaSingleAttrs and in this package's mirror of it — removes
// that, but then the reference corpus disagrees: the reference keeps
// layout="center" on an external directive, so the fixture above fails
// instead. Whoever picks this up has to settle that with the reference
// first; until then the invariant is qualified where it is stated (see
// Normalize's package comment) and pinned here.

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/markdown"
)

// wrapperLayoutCase is one media row with BOTH encodes of its first
// block pinned: raw is ToADF(parse), norm is ToADF(Normalize(parse)).
// Equal literals are the rows where the invariant holds; unequal
// literals are the gap.
type wrapperLayoutCase struct {
	name string
	row  string
	raw  string
	norm string
	why  string
	opts []Option
}

// The two media leaves the rows below share, spelled once so a row reads
// as its wrapper and nothing else — the wrapper is the whole subject.
const (
	wrapperLayoutExtLeaf = `{"type":"media","attrs":` +
		`{"alt":"alt","type":"external","url":"https://x/a.png"}}`
	wrapperLayoutFileLeaf = `{"type":"media","attrs":` +
		`{"alt":"alt","collection":"","height":20,"id":"AID","type":"file","width":10}}`
)

// wrapperLayoutCases is the shape map: the gap, then the rows that must
// keep agreeing.
func wrapperLayoutCases() []wrapperLayoutCase {
	return append(wrapperLayoutGapCases(), wrapperLayoutAgreeingCases()...)
}

// wrapperLayoutGapCases is every way an external media directive that
// omits a layout can still reach the image form — bare leaf, href, plain
// caption — and therefore every shape the invariant does not hold for.
func wrapperLayoutGapCases() []wrapperLayoutCase {
	const extLeaf = wrapperLayoutExtLeaf
	return []wrapperLayoutCase{
		{
			name: "external media with no layout",
			row:  "::media[alt]{type=external url=https://x/a.png}",
			raw:  `{"type":"mediaSingle","content":[` + extLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + extLeaf + `]}`,
			why: "the shape the gap is about: the pass degrades it to an ast.Image, " +
				"which spells the schema default the directive left unspelled.",
		},
		{
			name: "external media with no layout, linked",
			row:  "::media[alt]{type=external url=https://x/a.png href=https://home/}",
			raw: `{"type":"mediaSingle","content":[{"type":"media","attrs":` +
				`{"alt":"alt","type":"external","url":"https://x/a.png"},"marks":` +
				`[{"attrs":{"href":"https://home/"},"type":"link"}]}]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":` +
				`[{"type":"media","attrs":` +
				`{"alt":"alt","type":"external","url":"https://x/a.png"},"marks":` +
				`[{"attrs":{"href":"https://home/"},"type":"link"}]}]}`,
			why: "the href reaches the [![alt](url)](href) image form, so the same " +
				"degrade happens and the same wrapper attribute appears.",
		},
		{
			name: "external media with no layout, plain caption",
			row:  ":::media[alt]{type=external url=https://x/a.png}\nA caption\n:::",
			raw: `{"type":"mediaSingle","content":[` + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A caption"}]}]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A caption"}]}]}`,
			why: "a plain caption becomes the image title, so the container reaches " +
				"the image form too and the caption child rides along unchanged.",
		},
	}
}

// wrapperLayoutAgreeingCases is the neighborhood of the gap: one row for
// each distinct reason the invariant DOES hold, so a reader can see the
// gap is one omission covering one media type and not a property of the
// media projection.
func wrapperLayoutAgreeingCases() []wrapperLayoutCase {
	assets := parityAssetOpts()
	const extLeaf, fileLeaf = wrapperLayoutExtLeaf, wrapperLayoutFileLeaf
	return []wrapperLayoutCase{
		{
			name: "external media that spells layout=center",
			row:  "::media[alt]{type=external url=https://x/a.png layout=center}",
			raw:  `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + extLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + extLeaf + `]}`,
			why: "the author spelled the default, so the directive form emits the " +
				"attribute and the image form agrees with it. This is the neighbor " +
				"that localizes the gap to the OMISSION and not to external media.",
		},
		{
			name: "external media that spells a non-default layout",
			row:  "::media[alt]{type=external url=https://x/a.png layout=wide}",
			raw:  `{"type":"mediaSingle","attrs":{"layout":"wide"},"content":[` + extLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"wide"},"content":[` + extLeaf + `]}`,
			why: "a layout an image cannot hold BLOCKS the projection, so there is no " +
				"degrade and nothing to disagree about (same for full-width and the " +
				"wrap/align placements).",
		},
		{
			name: "external media with a rich caption",
			row:  ":::media[alt]{type=external url=https://x/a.png}\nA **bold** caption\n:::",
			raw: `{"type":"mediaSingle","content":[` + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A "},` +
				`{"type":"text","marks":[{"type":"strong"}],"text":"bold"},` +
				`{"type":"text","text":" caption"}]}]}`,
			norm: `{"type":"mediaSingle","content":[` + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A "},` +
				`{"type":"text","marks":[{"type":"strong"}],"text":"bold"},` +
				`{"type":"text","text":" caption"}]}]}`,
			why: "marked-up caption text has no image-title form, so the container " +
				"stays a directive and the wrapper stays bare on both sides. The " +
				"plain-caption row above is the same shape with the block removed.",
		},
		{
			name: "external media that also spells a path",
			row:  "::media[alt]{path=assets/shot.png type=external url=https://x/a.png}",
			raw:  `{"type":"mediaSingle","content":[` + extLeaf + `]}`,
			norm: `{"type":"mediaSingle","content":[` + extLeaf + `]}`,
			why: "an ast.Image has one destination, so a spelled path blocks the " +
				"projection (mediaexternalpath_test.go). Blocking is what made this " +
				"shape invariant; the rows above are the ones it deliberately left.",
		},
		{
			name: "file media addressed by id, no layout",
			row:  "::media[alt]{id=AID width=10 height=20}",
			raw:  `{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[` + fileLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[` + fileLeaf + `]}`,
			opts: assets,
			why: "the DIRECTIVE form re-infers align-start for a file-type directive " +
				"that omits the layout, and the image form spells the same. This row " +
				"is the proof that the gap is an omission covering one type only, not " +
				"a decision about the media projection.",
		},
		{
			name: "file media addressed by path, no layout",
			row:  "::media[alt]{path=assets/a.png}",
			raw:  `{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[` + fileLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[` + fileLeaf + `]}`,
			opts: assets,
			why:  "same re-inference, reached through the store rather than a spelled id.",
		},
		{
			name: "a media type with no image form at all",
			row:  "::media[alt]{type=link url=https://x/a.png}",
			raw: `{"type":"mediaSingle","content":[{"type":"media","attrs":` +
				`{"alt":"alt","type":"link","url":"https://x/a.png"}}]}`,
			norm: `{"type":"mediaSingle","content":[{"type":"media","attrs":` +
				`{"alt":"alt","type":"link","url":"https://x/a.png"}}]}`,
			why: "a type that is neither file nor external has no image projection, so " +
				"it keeps the bare wrapper on both sides. The gap needs BOTH the " +
				"omission and a reachable image form.",
		},
		{
			name: "a picture already in image form",
			row:  "![alt](https://x/a.png)",
			raw:  `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + extLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[` + extLeaf + `]}`,
			why: "the row the reference corpus pins: an ordinary markdown image encodes " +
				"WITH layout center, which is why the image form is the conformant one.",
		},
		{
			name: "a media group, which has no mediaSingle",
			row:  "::media[a]{id=X group=true}\n\n::media[b]{id=Y group=true}",
			raw: `{"type":"mediaGroup","content":[{"type":"media","attrs":` +
				`{"alt":"a","collection":"","id":"X","type":"file"}},{"type":"media","attrs":` +
				`{"alt":"b","collection":"","id":"Y","type":"file"}}]}`,
			norm: `{"type":"mediaGroup","content":[{"type":"media","attrs":` +
				`{"alt":"a","collection":"","id":"X","type":"file"}},{"type":"media","attrs":` +
				`{"alt":"b","collection":"","id":"Y","type":"file"}}]}`,
			why: "an attachment strip has no wrapper to carry a layout, so it cannot " +
				"be reached by any fix to this attribute.",
		},
	}
}

// TestToADFWrapperLayoutUnderNormalize pins both encodes of every row in
// the shape map, so the gap can neither widen nor close in silence.
//
// This is a MEASUREMENT PIN, not a defect proof: nothing here changed
// behavior. Its job is to hold the counterexample that Normalize's
// package comment now names, next to the neighbors that localize it —
// the same row with the layout spelled, the same row as a file
// attachment, the same row with a path — so a reader can see the gap is
// one omission for one type and not a property of media.
//
// It bites in both directions. Making the two sides AGREE fails it (the
// pinned norm or raw literal stops matching), whichever side is changed
// to do that; making them disagree differently fails it too. The
// mutations that were run against it: dialect's mediaSingleFromAttrs
// re-inferring center for non-file media (raw gains the attribute),
// ast_to_adf.go's loneImageMedia dropping center (norm loses it), and
// this package's own media projection refusing to degrade a layout-less
// external media (norm loses it). All three fail this test.
func TestToADFWrapperLayoutUnderNormalize(t *testing.T) {
	t.Parallel()
	for _, c := range wrapperLayoutCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(c.row)
			// Normalize rewrites in place, so each leg gets its own parse.
			rawDoc := ToADF(markdown.Parse([]byte(md)), c.opts...)
			normDoc := ToADF(Normalize(markdown.Parse([]byte(md)), c.opts...), c.opts...)
			raw := wrapperLayoutFirstBlock(t, rawDoc)
			norm := wrapperLayoutFirstBlock(t, normDoc)
			if raw != c.raw {
				t.Errorf("ToADF(parse) changed for the row under test"+
					"\n  in:   %q\n  got:  %s\n  want: %s\n  %s", md, raw, c.raw, c.why)
			}
			if norm != c.norm {
				t.Errorf("ToADF(Normalize(parse)) changed for the row under test"+
					"\n  in:   %q\n  got:  %s\n  want: %s\n  %s", md, norm, c.norm, c.why)
			}
			wrapperLayoutCheckGoodRows(t, rawDoc, normDoc)
			wrapperLayoutCheckSameDocument(t, md, rawDoc, normDoc, c.opts)
		})
	}
}

// wrapperLayoutCheckGoodRows is the GOOD row for every row in the table:
// parityDoc puts an ordinary image and an ordinary text link after the
// row under test, and those two blocks must encode identically on both
// legs. A "fix" to the wrapper attribute that reached a plain picture
// would show up here rather than as a pinned literal three rows away.
func wrapperLayoutCheckGoodRows(t *testing.T, rawDoc, normDoc adf.Doc) {
	t.Helper()
	const wantBlocks = 3 // the row under test, the plain image, the plain link
	if len(rawDoc.Content) != wantBlocks || len(normDoc.Content) != wantBlocks {
		t.Fatalf("the document lost or gained a block: raw=%d norm=%d, want %d",
			len(rawDoc.Content), len(normDoc.Content), wantBlocks)
	}
	for i := 1; i < wantBlocks; i++ {
		raw, norm := adfNodeJSON(t, rawDoc.Content[i]), adfNodeJSON(t, normDoc.Content[i])
		if raw != norm {
			t.Errorf("the pass changed the plain row beside the one under test"+
				"\n  block %d raw:  %s\n  block %d norm: %s", i, raw, i, norm)
		}
	}
}

// wrapperLayoutCheckSameDocument is what keeps this a BYTE gap and not a
// rendering one: whatever the two encodes spell, reading them back must
// produce the same markdown. center is the schema's default for the
// attribute, so a wrapper that omits it and a wrapper that spells it
// describe one placement — and adfast's own decode agrees, which is why
// nothing a remote renders is at stake and the invariant could be
// qualified rather than forced.
func wrapperLayoutCheckSameDocument(t *testing.T, md string, rawDoc, normDoc adf.Doc, opts []Option) {
	t.Helper()
	raw := markdown.Render(FromADF(rawDoc, opts...), markdown.WithPrettierText())
	norm := markdown.Render(FromADF(normDoc, opts...), markdown.WithPrettierText())
	if raw != norm {
		t.Errorf("the two encodes read back as DIFFERENT documents, so the gap is not "+
			"confined to the wrapper's placement bytes"+
			"\n  in:   %q\n  raw:  %q\n  norm: %q", md, raw, norm)
	}
}

// wrapperLayoutFirstBlock returns the JSON of the block holding the row
// under test, so a mismatch names that block instead of the whole
// payload.
func wrapperLayoutFirstBlock(t *testing.T, doc adf.Doc) string {
	t.Helper()
	if len(doc.Content) == 0 {
		t.Fatalf("the encode produced no content: %s", adfJSON(t, doc))
	}
	return adfNodeJSON(t, doc.Content[0])
}
