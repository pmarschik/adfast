package convert

// The mediaSingle wrapper's layout attribute, pinned shape by shape, and
// the three sites that have to agree about it.
//
// layout is REQUIRED on mediaSingle: Jira's ADF node reference marks it
// so, and Atlassian's adf-schema declares it `layout: { default:
// 'center' }` (media-single.ts, the file docs/adf-coverage.md already
// cites for the node). A directive that spells no layout therefore still
// has to encode one, and which value stands in depends on the media
// type:
//
//   - FILE media takes "align-start", the attachment default, and the
//     decode omits it again, so the directive form stays terse and the
//     round trip is an identity.
//   - Every OTHER type takes "center", the schema default, and the decode
//     KEEPS it. So a round trip completes an external media directive
//     that omitted the attribute, writing the canonical form back.
//
// WHY THAT ASYMMETRY, measured rather than assumed. The oracle is
// testdata/directive_fixtures.json, generated from the reference
// implementations, and it pins the contrast in two adjacent rows that
// differ in nothing but the media type:
//
//   - ADF→md, file media carrying `layout: "align-start"`, renders
//     `::media[image-20260212-125503.png]{layoutWidth="686"
//     path="assets/shot.png" widthType="pixel"}` — the attribute is
//     ELIDED. Three more attachment rows elide it the same way.
//   - ADF→md, external media carrying `layout: "center"`, renders
//     `::media[shot]{height="50" layout="center" type="external"
//     url="https://example.com/i.png" width="100"}` — the attribute is
//     SPELLED. Offered exactly the elision it performs one row earlier,
//     the reference declines for external media.
//
// So eliding center for external media is not "the same terseness"; it
// is a corpus row. And the md→ADF direction pins the completion itself:
// a file directive that omits the layout
// (`::media[shot.png]{#abc collection …}`) encodes WITH `layout:
// "align-start"`. Across both directions the corpus holds twelve
// mediaSingle nodes and NOT ONE of them lacks the attribute, which is
// the whole decision in one sentence — the reference never emits a
// layout-less mediaSingle, so a directive form that did was the
// under-spelling one.
//
// A ROUND TRIP MAY THEREFORE ADD THIS ATTRIBUTE. That reads at first
// like a breach of the general rule ("a format may no more add to a
// document than delete from one"), so it is worth saying where the rule
// actually stands: adding a schema-defaulted attribute is what the
// REFERENCE does, and the corpus says so in the one field that can
// answer it. Each markdown fixture carries the reference's own md→ADF→md
// `roundtrip` beside the source, and two rows there gain an attribute
// the author never wrote:
//
//	`::linkEmbed[https://example.com/embed]`
//	  roundtrip → `::linkEmbed[https://example.com/embed]{layout="center"}`
//	`:status[no color]`
//	  roundtrip → `:status[no color]{color="neutral"}`
//
// So the reference ADDS — it does not preserve what it was handed and it
// does not drop. adfast follows on both constructs at every leg, the
// FORMATTER included: `:status[no color]` formats to
// `:status[no color]{color="neutral"}` today. mediaSingle was the one
// wrapper left out of a rule the dialect already followed twice, and the
// rule the formatter is actually held to is its stated contract —
// semantic coherence and idempotence, not byte identity with its input.
// What a format may never do is DELETE, and completing a REQUIRED
// attribute is the opposite of that.
//
// WHAT IT COST TO LEAVE IT OUT: a hand-written
// `::media[alt]{type=external url=…}` encoded a mediaSingle with no
// layout at all. The push was accepted only because the receiving
// ProseMirror schema supplies the default; a stricter validator rejects
// it. Closing it also closes the one exception Normalize's package
// comment used to carry, so ToADF is now invariant under the pass with
// no qualification.
//
// THE THREE SITES, all of which this file pins:
//
//	encode     dialect's mediaSingleFromAttrs — directive attrs → layout
//	decode     dialect's mediaSingleAttrs     — layout → directive attrs
//	format leg this package's applySingle     — the mirror of encode, so
//	                                            Render∘NormalizeFormat
//	                                            agrees with md→ADF→md
//
// TestMediaSingleLayoutIsSpelledAtEveryLeg mutates each independently.

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/markdown"
)

// wrapperLayoutCase is one media row with BOTH encodes of its first
// block pinned: raw is ToADF(parse), norm is ToADF(Normalize(parse)).
// Every row's two literals are now EQUAL — that equality is the
// invariant, and the test asserts it as well as the literals, so a row
// cannot be re-pinned into disagreement by editing one side.
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

// wrapperLayoutCases is the shape map: the rows that used to be the gap,
// then the rows that always agreed.
func wrapperLayoutCases() []wrapperLayoutCase {
	return append(wrapperLayoutClosedCases(), wrapperLayoutAgreeingCases()...)
}

// wrapperLayoutClosedCases is every way an external media directive that
// omits a layout can still reach the image form — bare leaf, href, plain
// caption. These three were the measured gap: the image form spelled
// center and the directive form spelled nothing. Both spell it now.
func wrapperLayoutClosedCases() []wrapperLayoutCase {
	const extLeaf = wrapperLayoutExtLeaf
	const centered = `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`
	return []wrapperLayoutCase{
		{
			name: "external media with no layout",
			row:  "::media[alt]{type=external url=https://x/a.png}",
			raw:  centered + extLeaf + `]}`,
			norm: centered + extLeaf + `]}`,
			why: "the shape the gap was about: the pass degrades it to an ast.Image, " +
				"and the directive form now spells the same schema default the image " +
				"form always did.",
		},
		{
			name: "external media with no layout, linked",
			row:  "::media[alt]{type=external url=https://x/a.png href=https://home/}",
			raw: centered + `{"type":"media","attrs":` +
				`{"alt":"alt","type":"external","url":"https://x/a.png"},"marks":` +
				`[{"attrs":{"href":"https://home/"},"type":"link"}]}]}`,
			norm: centered + `{"type":"media","attrs":` +
				`{"alt":"alt","type":"external","url":"https://x/a.png"},"marks":` +
				`[{"attrs":{"href":"https://home/"},"type":"link"}]}]}`,
			why: "the href reaches the [![alt](url)](href) image form, so the same " +
				"degrade happens and both sides now carry the same wrapper attribute.",
		},
		{
			name: "external media with no layout, plain caption",
			row:  ":::media[alt]{type=external url=https://x/a.png}\nA caption\n:::",
			raw: centered + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A caption"}]}]}`,
			norm: centered + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A caption"}]}]}`,
			why: "a plain caption becomes the image title, so the container reaches " +
				"the image form too and the caption child rides along unchanged.",
		},
	}
}

// wrapperLayoutAgreeingCases is the neighborhood: one row for each
// distinct reason the two encodes agree, so a reader can see that the
// attribute is now present for EVERY media shape that has a wrapper, and
// that the value depends only on the type and on what the author spelled.
func wrapperLayoutAgreeingCases() []wrapperLayoutCase {
	assets := parityAssetOpts()
	const extLeaf, fileLeaf = wrapperLayoutExtLeaf, wrapperLayoutFileLeaf
	const centered = `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[`
	const alignStart = `{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[`
	return []wrapperLayoutCase{
		{
			name: "external media that spells layout=center",
			row:  "::media[alt]{type=external url=https://x/a.png layout=center}",
			raw:  centered + extLeaf + `]}`,
			norm: centered + extLeaf + `]}`,
			why: "the author spelled the default explicitly. This row and the bare one " +
				"above now encode identically, which is the point: the canonical form " +
				"spells the layout, so spelling it changes nothing.",
		},
		{
			name: "external media that spells a non-default layout",
			row:  "::media[alt]{type=external url=https://x/a.png layout=wide}",
			raw:  `{"type":"mediaSingle","attrs":{"layout":"wide"},"content":[` + extLeaf + `]}`,
			norm: `{"type":"mediaSingle","attrs":{"layout":"wide"},"content":[` + extLeaf + `]}`,
			why: "the UPPER FENCE: an author's layout must beat the stand-in, and a " +
				"layout an image cannot hold also blocks the projection, so there is " +
				"no degrade (same for full-width and the wrap/align placements).",
		},
		{
			name: "external media with a rich caption",
			row:  ":::media[alt]{type=external url=https://x/a.png}\nA **bold** caption\n:::",
			raw: centered + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A "},` +
				`{"type":"text","marks":[{"type":"strong"}],"text":"bold"},` +
				`{"type":"text","text":" caption"}]}]}`,
			norm: centered + extLeaf +
				`,{"type":"caption","content":[{"type":"text","text":"A "},` +
				`{"type":"text","marks":[{"type":"strong"}],"text":"bold"},` +
				`{"type":"text","text":" caption"}]}]}`,
			why: "marked-up caption text has no image-title form, so the container " +
				"stays a directive on both sides — and the completion reaches it " +
				"anyway, because it is the ENCODE that supplies the layout and not " +
				"the degrade.",
		},
		{
			name: "external media that also spells a path",
			row:  "::media[alt]{path=assets/shot.png type=external url=https://x/a.png}",
			raw:  centered + extLeaf + `]}`,
			norm: centered + extLeaf + `]}`,
			why: "an ast.Image has one destination, so a spelled path blocks the " +
				"projection (mediaexternalpath_test.go). It keeps the directive form " +
				"and the directive form now carries the layout too.",
		},
		{
			name: "file media addressed by id, no layout",
			row:  "::media[alt]{id=AID width=10 height=20}",
			raw:  alignStart + fileLeaf + `]}`,
			norm: alignStart + fileLeaf + `]}`,
			opts: assets,
			why: "the LOWER FENCE: file media keeps taking align-start, not center. " +
				"The reference elides align-start on the way back, so this shape's " +
				"round trip stays an identity while the external one completes.",
		},
		{
			name: "file media addressed by path, no layout",
			row:  "::media[alt]{path=assets/a.png}",
			raw:  alignStart + fileLeaf + `]}`,
			norm: alignStart + fileLeaf + `]}`,
			opts: assets,
			why:  "same fence, reached through the store rather than a spelled id.",
		},
		{
			name: "a media type with no image form at all",
			row:  "::media[alt]{type=link url=https://x/a.png}",
			raw: centered + `{"type":"media","attrs":` +
				`{"alt":"alt","type":"link","url":"https://x/a.png"}}]}`,
			norm: centered + `{"type":"media","attrs":` +
				`{"alt":"alt","type":"link","url":"https://x/a.png"}}]}`,
			why: "a type that is neither file nor external has no image projection, so " +
				"it never degrades — and it still needs the required attribute, so it " +
				"takes the schema default like every other non-file type.",
		},
		{
			name: "a picture already in image form",
			row:  "![alt](https://x/a.png)",
			raw:  centered + extLeaf + `]}`,
			norm: centered + extLeaf + `]}`,
			why: "the row the reference corpus pins directly: an ordinary markdown " +
				"image encodes WITH layout center. The directive rows above now " +
				"encode identically to it, which is what conformance to that row means.",
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
			why: "an attachment strip has no wrapper to carry a layout, so the " +
				"completion cannot reach it and must not invent one.",
		},
	}
}

// TestToADFWrapperLayoutUnderNormalize pins both encodes of every row in
// the shape map and asserts they are EQUAL, which is the invariant
// Normalize's package comment states without qualification now that the
// wrapper layout is spelled on both sides.
//
// It bites in both directions. Making the two sides disagree fails the
// equality check; changing what either spells fails the pinned literal.
// The per-leg mutations are in
// TestMediaSingleLayoutIsSpelledAtEveryLeg below.
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
			if raw != norm {
				t.Errorf("ToADF is not invariant under Normalize for this row"+
					"\n  in:   %q\n  ToADF:      %s\n  normalized: %s\n  %s",
					md, raw, norm, c.why)
			}
			wrapperLayoutCheckGoodRows(t, rawDoc, normDoc)
			wrapperLayoutCheckSameDocument(t, md, rawDoc, normDoc, c.opts)
		})
	}
}

// layoutLegCase is one row measured at all three sites at once: the ADF
// the encode builds, the markdown the decode reads back out of it, and
// the markdown the format leg produces without going through ADF. Any
// one site left behind shows up as one of the three fields.
type layoutLegCase struct {
	name string
	row  string
	adf  string // the wrapper's attrs, as encoded
	back string // the row as md→ADF→md renders it
	form string // the row as Render∘NormalizeFormat renders it
	why  string
	opts []Option
}

// TestMediaSingleLayoutIsSpelledAtEveryLeg is the three-site pin, and the
// FENCE at both ends of the rule: an author's explicit layout still wins
// over the stand-in, file media still takes align-start rather than
// center, and only a non-file media that spelled nothing gets completed.
//
// This is a DEFECT PROOF for the external rows, and each arm was mutated
// on its own so the three fields are shown to be independent.
//
// MUTATION 1, the ENCODE arm: mediaSingleFromAttrs's `default:
// single.Layout = new("center")` deleted, leaving the file arm and
// nothing after it — the shape the code had before this rule landed.
// The bare row fails on the encode field alone, because with no layout
// on the wrapper nothing blocks the image projection and both renders
// are the plain picture either way; the bordered row, which keeps the
// directive form, fails on the encode AND the decode field. The format
// field stays green in both, which is what makes the mutation
// single-legged. MEASURED:
//
//	--- FAIL: TestMediaSingleLayoutIsSpelledAtEveryLeg/bare_external_media_is_completed
//	    encode: the mediaSingle wrapper's attrs changed
//	      in:   "::media[alt]{type=external url=https://x/a.png}\n"
//	      got:  <the wrapper carries NO attrs at all>
//	      want: {"layout":"center"}
//	--- FAIL: TestMediaSingleLayoutIsSpelledAtEveryLeg/bordered_external_media_is_completed_and_spells_it
//	    encode: the mediaSingle wrapper's attrs changed
//	      got:  <the wrapper carries NO attrs at all>
//	      want: {"layout":"center"}
//	    decode: the md→ADF→md render changed
//	      got:  "::media[alt]{borderColor=\"#000\" borderSize=\"2\" type=\"external\" url=\"https://x/a.png\"}\n"
//	      want: "::media[alt]{borderColor=\"#000\" borderSize=\"2\" layout=\"center\" type=\"external\" url=\"https://x/a.png\"}\n"
//
// MUTATION 2, the FORMAT-LEG arm alone: applySingle's `default:
// m.layout = new("center")` deleted, so the leg sets m.layout only when
// the author spelled one. The ADF leg is untouched, so only the format
// field moves — and it is the direction that would put the two
// projections back into disagreement, which is why the format leg
// spells the attribute rather than staying silent. MEASURED:
//
//	--- FAIL: TestMediaSingleLayoutIsSpelledAtEveryLeg/bordered_external_media_is_completed_and_spells_it
//	    format leg: the Render∘NormalizeFormat render changed
//	      got:  "::media[alt]{borderColor=\"#000\" borderSize=\"2\" type=\"external\" url=\"https://x/a.png\"}\n"
//	      want: "::media[alt]{borderColor=\"#000\" borderSize=\"2\" layout=\"center\" type=\"external\" url=\"https://x/a.png\"}\n"
//	--- FAIL: TestMediaProjectionLegsAgree/bordered_media
//	    the two media projections disagree
//	      format: "::media[alt]{borderColor=\"#000\" borderSize=\"2\" type=\"external\" url=\"https://x/a.png\"}\n\n…"
//	      adf:    "::media[alt]{borderColor=\"#000\" borderSize=\"2\" layout=\"center\" type=\"external\" url=\"https://x/a.png\"}\n\n…"
//
// MUTATION 3, the DECODE arm, which is the OTHER candidate answer and
// the reason this rule is asymmetric: mediaSingleAttrs made to elide
// center for a non-file media the way it elides align-start for a file
// one. That makes the round trip an identity and would let the format
// leg stay silent — and the reference corpus rejects it outright:
//
//	--- FAIL: TestDirectiveFixtures_ToMarkdown
//	    ToMarkdown diverged from the remark reference corpus for
//	    mediaSingle{layout: "center"} → media{type: "external", …}
//	     got: "::media[shot]{height=\"50\" type=\"external\" url=\"https://example.com/i.png\" width=\"100\"}\n"
//	    want: "::media[shot]{height=\"50\" layout=\"center\" type=\"external\" url=\"https://example.com/i.png\" width=\"100\"}\n"
//
// The file rows and the explicit-layout rows are preserved-behavior
// PINS: they pass on both implementations, and they are here so that a
// future "simplification" of the rule into "always center" or "never
// complete" cannot pass.
func TestMediaSingleLayoutIsSpelledAtEveryLeg(t *testing.T) {
	t.Parallel()
	for _, c := range mediaSingleLayoutLegCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := c.row + "\n"
			if got := layoutWrapperAttrs(t, ToADF(markdown.Parse([]byte(md)), c.opts...)); got != c.adf {
				t.Errorf("encode: the mediaSingle wrapper's attrs changed"+
					"\n  in:   %q\n  got:  %s\n  want: %s\n  %s", md, got, c.adf, c.why)
			}
			if got := parityADFLeg(md, c.opts...); got != c.back {
				t.Errorf("decode: the md→ADF→md render changed"+
					"\n  in:   %q\n  got:  %q\n  want: %q\n  %s", md, got, c.back, c.why)
			}
			if got := parityFormatLeg(md, c.opts...); got != c.form {
				t.Errorf("format leg: the Render∘NormalizeFormat render changed"+
					"\n  in:   %q\n  got:  %q\n  want: %q\n  %s", md, got, c.form, c.why)
			}
		})
	}
}

// mediaSingleLayoutLegCases holds a FIX row and a fence row for each arm
// of the rule, so no arm can be removed without a failure and none can
// be widened over its neighbor.
func mediaSingleLayoutLegCases() []layoutLegCase {
	assets := parityAssetOpts()
	const ext = "type=external url=https://x/a.png"
	return []layoutLegCase{
		{
			name: "bare external media is completed",
			row:  "::media[alt]{" + ext + "}",
			adf:  `{"layout":"center"}`,
			back: "![alt](https://x/a.png)\n",
			form: "![alt](https://x/a.png)\n",
			why: "THE FIX, in the shape that degrades to an image: the wrapper carries " +
				"the required attribute, and because center is what the image form " +
				"means, the markdown both legs render is the plain picture.",
		},
		{
			name: "bordered external media is completed and spells it",
			row:  "::media[alt]{" + ext + " borderColor=#000 borderSize=2}",
			adf:  `{"layout":"center"}`,
			back: `::media[alt]{borderColor="#000" borderSize="2" layout="center" ` +
				`type="external" url="https://x/a.png"}` + "\n",
			form: `::media[alt]{borderColor="#000" borderSize="2" layout="center" ` +
				`type="external" url="https://x/a.png"}` + "\n",
			why: "THE FIX where the directive SURVIVES, so the completion is visible in " +
				"the markdown: a border blocks the image form, the decode spells the " +
				"layout for external media, and the format leg must spell it too or " +
				"the two projections disagree.",
		},
		{
			name: "an author's explicit layout beats the stand-in",
			row:  "::media[alt]{" + ext + " layout=wide}",
			adf:  `{"layout":"wide"}`,
			back: `::media[alt]{layout="wide" type="external" url="https://x/a.png"}` + "\n",
			form: `::media[alt]{layout="wide" type="external" url="https://x/a.png"}` + "\n",
			why: "THE UPPER FENCE, a preserved-behavior PIN: the stand-in may only fill " +
				"an ABSENT attribute. A completion that overwrote the author's wide " +
				"with center would silently re-place the picture.",
		},
		{
			name: "file media takes align-start, not center",
			row:  "::media[alt]{id=AID width=10 height=20}",
			adf:  `{"layout":"align-start"}`,
			back: "![alt](assets/a.png)\n",
			form: "![alt](assets/a.png)\n",
			opts: assets,
			why: "THE LOWER FENCE, a preserved-behavior PIN: the attachment default is " +
				"align-start and the decode elides it, per the reference's attachment " +
				"rows. A rule collapsed to 'always center' fails here.",
		},
		{
			name: "bordered file media keeps align-start unspelled",
			row:  "::media[alt]{id=AID width=10 height=20 borderColor=#000 borderSize=2}",
			adf:  `{"layout":"align-start"}`,
			back: `::media[alt]{borderColor="#000" borderSize="2" path="assets/a.png"}` + "\n",
			form: `::media[alt]{borderColor="#000" borderSize="2" path="assets/a.png"}` + "\n",
			opts: assets,
			why: "the lower fence where the directive SURVIVES, so the ABSENCE of the " +
				"attribute is visible in the markdown: align-start is completed on the " +
				"encode and elided again on the way back, so a file directive stays " +
				"terse while the bordered external row above gains the attribute. " +
				"(The id and the size give way to path= because the store can recover " +
				"both; that is mediaSourceAttrs and not this rule.)",
		},
	}
}

// layoutWrapperAttrs returns the JSON of the first block's OWN attrs
// object, so an encode assertion names the wrapper and not the whole
// payload.
//
// It stops at the wrapper's "content", because the media leaf inside has
// an attrs object too: a search that ran past it would report the LEAF's
// attributes as the wrapper's, and the failure for a wrapper that has
// none at all — which is exactly the defect this file is about — would
// print a plausible-looking object instead of saying the attribute is
// missing. Measured, by deleting the truncation and re-running MUTATION
// 1 above:
//
//	encode: the mediaSingle wrapper's attrs changed
//	  got:  {"alt":"alt","type":"external","url":"https://x/a.png"}
//	  want: {"layout":"center"}
//
// The VERDICT is the same either way — the row fails on both variants,
// and on the fixed implementation every mediaSingle carries an attrs
// object that precedes its content, so the two variants are
// indistinguishable on a green run. What the truncation buys is a
// truthful transcript, which is the whole value of a mutation this file
// records for the next reader.
func layoutWrapperAttrs(t *testing.T, doc adf.Doc) string {
	t.Helper()
	block := wrapperLayoutFirstBlock(t, doc)
	if c := strings.Index(block, `"content":`); c >= 0 {
		block = block[:c]
	}
	const key = `"attrs":`
	i := strings.Index(block, key)
	if i < 0 {
		return "<the wrapper carries NO attrs at all>"
	}
	rest := block[i+len(key):]
	depth := 0
	for j := range len(rest) {
		switch rest[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return rest[:j+1]
			}
		}
	}
	t.Fatalf("the attrs object of %s is unterminated", block)
	return ""
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

// wrapperLayoutCheckSameDocument keeps the two encodes readable as ONE
// document: whatever each spells, reading them back must produce the
// same markdown. It survives the fix as a stronger statement than it
// was — the two payloads are now byte-equal, so this can only fail if a
// change makes them diverge again.
func wrapperLayoutCheckSameDocument(t *testing.T, md string, rawDoc, normDoc adf.Doc, opts []Option) {
	t.Helper()
	raw := markdown.Render(FromADF(rawDoc, opts...), markdown.WithPrettierText())
	norm := markdown.Render(FromADF(normDoc, opts...), markdown.WithPrettierText())
	if raw != norm {
		t.Errorf("the two encodes read back as DIFFERENT documents"+
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
