package convert

// The media projection — a media node becoming a plain markdown image,
// and everything richer than an image staying a ::media directive — is
// implemented TWICE, and the two copies have to agree.
//
// One copy is the ADF leg. dialect's Media.EncodeADF builds the
// adf.Media (plus its mediaSingle wrapper), and dialect's decode hooks
// read one back: mediaFromAttrs, singleBlocksImage, mediaBlocksImage,
// imageParagraph, setImageTitle, mediaLeafNode. The other copy is
// normalize.go, which performs the same projection on the pivot AST
// without going through ADF at all, over its own fmtMedia shape, written
// out function for function by hand — the comments there say "mirrors
// dialect's …" on each one, and setImageTitle's two bodies are
// byte-identical.
//
// Nothing made them agree. A linked-image fix landed on the ADF leg
// alone, so the format leg went on deleting the link wrapped around an
// image: formatting the pull's own output dropped the very destination
// the decode had just restored, and the formatter was not a fixpoint on
// its own output. That was patched copy by copy. The drift MECHANISM
// survived the patch, because every media change still has to be made
// twice, and only the person making it remembers there is a second
// place.
//
// This file is the tripwire for the next one. Normalize's package
// comment states the invariant the mirror owes: the format render is
// byte-for-byte what routing the parse AST through ADF and back
// produces. So for a media document the two legs must render the same
// markdown, and a media change made to one copy alone breaks
// TestMediaProjectionLegsAgree.
//
// Every document carries an ordinary image and an ordinary text link
// beside the row under test, so a change that "fixes" one media shape by
// over-normalizing the plain ones fails here too.
//
// Three facts about the mirror, measured while writing this file, that
// the reader of either copy cannot see from one side alone:
//
//   - mediaBlocksImage has NO counterpart here. dialect factors the
//     three checks the file and external image paths share
//     (occurrenceKey, non-empty collection, border) into it; normalize.go
//     restates all three inside mediaAsImage AND inside fileMediaAsImage,
//     so that predicate lives in three places, not two.
//   - normalize.go's mediaLeafNode ends in a hand-written copy of
//     dialect's newMedia constructor, field for field, because newMedia is
//     unexported. A new field on dialect.Media has to be added twice.
//   - singleBlocksImage differs, and must: dialect also tests
//     adf.HasExtra for a layout or width whose JSON value is not the type
//     the field holds. A directive attribute is always a string, so the
//     fmtMedia side has nothing to test.

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// The two rows that ride along in every document and must come out
// untouched, whatever the row under test does.
const (
	parityGoodImage = "![just a picture](https://x/plain.png)"
	parityGoodLink  = "[just a link](https://elsewhere.example/)"
)

// parityDoc places the row under test above the two unaffected rows.
func parityDoc(row string) string {
	return row + "\n\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
}

// parityFormatLeg is the prettier md→md formatter's composition, the
// leg normalize.go's own media projection serves:
// Render∘NormalizeFormat∘Parse.
func parityFormatLeg(md string, opts ...Option) string {
	root := markdown.Parse([]byte(md))
	return markdown.Render(NormalizeFormat(root, opts...), markdown.WithPrettierText())
}

// parityADFLeg is the md→ADF→md round trip, the leg dialect's encode and
// decode hooks serve. It renders with the same options as the format leg
// so only the projection can differ.
func parityADFLeg(md string, opts ...Option) string {
	root := markdown.Parse([]byte(md))
	return markdown.Render(FromADF(ToADF(root, opts...), opts...), markdown.WithPrettierText())
}

// parityAssetOpts is a conversion that knows one downloaded attachment,
// by id and by path: the configuration under which a file media node has
// an image form at all.
func parityAssetOpts() []Option {
	return []Option{
		WithMediaAssets(map[string]MediaAsset{
			"AID": {Path: "assets/a.png", Width: 10, Height: 20, HasDim: true},
		}),
		WithAssetIDResolver(func(path string) (string, bool) {
			return "AID", path == "assets/a.png"
		}),
		WithImageDimsResolver(func(path string) (int, int, bool) {
			return 10, 20, path == "assets/a.png"
		}),
	}
}

// parityCase is one media shape and the option set it needs.
type parityCase struct {
	name string
	row  string
	opts []Option
}

// parityAgreeing is the corpus the two projections must render
// identically. Between them the rows reach all six mirrored functions:
// mediaFromAttrs (every directive row), singleBlocksImage (the layout
// and width rows), mediaBlocksImage (border, occurrenceKey, collection),
// imageParagraph (the image and href rows), setImageTitle (the caption
// rows) and mediaLeafNode (every row that stays a directive).
func parityAgreeing() []parityCase {
	assets := parityAssetOpts()
	return []parityCase{
		{"external image", "![alt](https://x/a.png)", nil},
		{"linked external image", "[![alt](https://x/a.png)](https://home/)", nil},
		{"external media", "::media[alt]{type=external url=https://x/a.png}", nil},
		{"external media with href", "::media[alt]{type=external url=https://x/a.png href=https://home/}", nil},
		{"external media, relative url", "::media[alt]{type=external url=img/a.png}", nil},
		{"file media in the store", "::media[alt]{id=AID width=10 height=20}", assets},
		{"file media with href", "::media[alt]{id=AID width=10 height=20 href=https://home/}", assets},
		{"bordered media", "::media[alt]{type=external url=https://x/a.png borderColor=#000 borderSize=2}", nil},
		{"wide layout", "::media[alt]{type=external url=https://x/a.png layout=wide}", nil},
		{"no-op resize", "::media[alt]{id=AID width=10 height=20 layoutWidth=10 widthType=pixel}", assets},
		{"occurrence key", "::media[alt]{type=external url=https://x/a.png occurrenceKey=k}", nil},
		{"non-empty collection", "::media[alt]{id=AID collection=c width=10 height=20}", assets},
		{"the file default layout", "::media[alt]{id=AID width=10 height=20 layout=align-start}", assets},
		{"a layout the image cannot hold", "::media[alt]{id=AID width=10 height=20 layout=center}", assets},
		{"plain caption becomes the title", ":::media[alt]{type=external url=https://x/a.png}\nA caption\n:::", nil},
		{"rich caption keeps the container", ":::media[alt]{type=external url=https://x/a.png}\nA **bold** caption\n:::", nil},
		{"caption and href together", ":::media[alt]{type=external url=https://x/a.png href=https://home/}\nA caption\n:::", nil},
		{"caption on store media", ":::media[alt]{id=AID width=10 height=20}\nA caption\n:::", assets},
		{"a media group", "::media[a]{id=X group=true}\n\n::media[b]{id=Y group=true}", nil},
	}
}

// TestMediaProjectionLegsAgree is the drift detector: for a media
// document the format leg's own projection and the ADF round trip must
// render the same markdown. Breaking either copy of any of the six
// mirrored functions breaks this.
func TestMediaProjectionLegsAgree(t *testing.T) {
	t.Parallel()
	for _, c := range parityAgreeing() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(c.row)
			format := parityFormatLeg(md, c.opts...)
			viaADF := parityADFLeg(md, c.opts...)
			if format != viaADF {
				t.Errorf("the two media projections disagree\n  in:     %q\n  format: %q\n  adf:    %q",
					md, format, viaADF)
			}
			parityCheckGoodRows(t, "format", format)
			parityCheckGoodRows(t, "adf", viaADF)
		})
	}
}

// parityCheckGoodRows asserts the two unaffected rows, so a failure in
// the row under test is visibly a failure of that row alone.
func parityCheckGoodRows(t *testing.T, leg, md string) {
	t.Helper()
	for _, want := range []string{parityGoodImage, parityGoodLink} {
		if !strings.Contains(md, want) {
			t.Errorf("%s leg changed the plain row beside it; want %q in\n  %q", leg, want, md)
		}
	}
}

// parityDivergence is one shape the two legs do NOT render alike, with
// both renders pinned.
type parityDivergence struct {
	name   string
	row    string
	format string
	adf    string
	why    string
	opts   []Option
}

// TestMediaProjectionLegsDivergeAsMeasured pins the shapes the two legs
// render differently, so the set cannot widen in silence and a
// reconciliation cannot pass unnoticed.
//
// The first group is divergence BY CONTRACT, and belongs here forever:
// the ADF encode may drop what ADF has no node for, while the format leg
// is total — a formatter may reshape an author's syntax but never delete
// it. Each row is the documented behavior in docs/adf-coverage.md
// ("Reading one back" and the lossy-encode table above it).
//
// The second group is DRIFT, found by measuring the two copies against
// each other. These rows are pinned as measured, not as intended: the
// format copy of the projection is missing a rule the ADF copy has.
// Reconciling one means moving its row up into parityAgreeing, which is
// exactly the failure this table produces when someone does.
func TestMediaProjectionLegsDivergeAsMeasured(t *testing.T) {
	t.Parallel()
	assets := parityAssetOpts()
	good := "\n\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
	cases := []parityDivergence{
		// --- by contract ---
		{
			name:   "an image no store can place",
			row:    "![alt](img/a.png)",
			format: "![alt](img/a.png)" + good,
			adf:    "[alt](img/a.png)" + good,
			why: "ADF addresses an attachment by media id, so an image the store " +
				"cannot map has no node for the picture and degrades to its label as " +
				"a link (unresolved-asset). The formatter may not delete the picture.",
		},
		{
			name:   "a link around an unplaceable image",
			row:    "[![alt](img/a.png)](https://home/)",
			format: "[![alt](img/a.png)](https://home/)" + good,
			adf:    "[alt](https://home/)" + good,
			why: "Same drop, and with two candidate hrefs and one link mark the " +
				"enclosing destination wins, because that is the one the reader clicks.",
		},
		{
			name:   "a mid-sentence external image",
			row:    "text ![alt](https://x/a.png) tail",
			format: "text ![alt](https://x/a.png) tail" + good,
			adf:    "text [alt](https://x/a.png) tail" + good,
			why: "ADF has no inline external image at all: mediaInline addresses an " +
				"uploaded attachment by id and has no external variant " +
				"(inline-image-degraded).",
		},
		{
			name:   "a link around a mid-sentence external image",
			row:    "text [![alt](https://x/a.png)](https://home/) tail",
			format: "text [![alt](https://x/a.png)](https://home/) tail" + good,
			adf:    "text [alt](https://home/) tail" + good,
			why:    "The same degradation, and again the enclosing destination wins the href.",
		},
		// --- drift: the format copy is missing a rule the ADF copy has ---
		{
			name:   "relative external media under WithPreserveLocalImages",
			row:    "::media[alt]{type=external url=img/a.png}",
			opts:   []Option{WithPreserveLocalImages()},
			format: "::media[alt]{type=\"external\" url=\"img/a.png\"}" + good,
			adf:    "![alt](img/a.png)" + good,
			why: "dialect's mediaAsImage takes a preserveLocal parameter and lets a " +
				"document-relative external url reach the plain image form under the " +
				"opt-in; normalize.go's copy hardcodes an http(s) prefix test and the " +
				"normalizer never receives the flag. Reconciling this means threading " +
				"the option into the normalizer.",
		},
		{
			name:   "file media addressed by path",
			row:    "::media[alt]{path=assets/a.png}",
			opts:   assets,
			format: "::media[alt]{path=\"assets/a.png\"}" + good,
			adf:    "![alt](assets/a.png)" + good,
			why: "Media.EncodeADF resolves a path-only directive back to its id and " +
				"dimensions through the store (AssetID, AssetDims), so the ADF leg " +
				"finds the asset and collapses to an image; normalize.go's " +
				"fileMediaAsImage looks the asset up by id alone and gives up when the " +
				"directive carries only a path.",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(c.row)
			if got := parityFormatLeg(md, c.opts...); got != c.format {
				t.Errorf("format leg changed\n  in:   %q\n  got:  %q\n  want: %q\n  %s", md, got, c.format, c.why)
			}
			if got := parityADFLeg(md, c.opts...); got != c.adf {
				t.Errorf("adf leg changed\n  in:   %q\n  got:  %q\n  want: %q\n  %s", md, got, c.adf, c.why)
			}
		})
	}
}

// TestMediaFormatLegIsAFixpoint: the prettier formatter must be
// idempotent on media, its own output included. This is the property the
// linked-image drift broke — the format leg deleted the link the decode
// had just written, so formatting a pulled document changed it and
// formatting it again changed it back — so the corpus leads with a link
// wrapped around an image, in every spelling that reaches media plus a
// link mark.
func TestMediaFormatLegIsAFixpoint(t *testing.T) {
	t.Parallel()
	assets := parityAssetOpts()
	cases := append(parityAgreeing(),
		parityCase{"a link around an image", "[![alt](https://x/a.png)](https://home/)", nil},
		parityCase{"a link around a titled image", "[![alt](https://x/a.png \"t\")](https://home/)", nil},
		parityCase{"a link around a reference image", "[![alt][r]](https://home/)\n\n[r]: https://x/a.png", nil},
		parityCase{"a link around a store image", "[![alt](assets/a.png)](https://home/)", assets},
		parityCase{"a link around an unplaceable image", "[![alt](img/a.png)](https://home/)", nil},
		parityCase{"a bordered media with an href", "::media[alt]{type=external url=https://x/a.png borderColor=#000 href=https://home/}", nil},
	)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(c.row)
			once := parityFormatLeg(md, c.opts...)
			twice := parityFormatLeg(once, c.opts...)
			if once != twice {
				t.Errorf("formatting the formatter's own media output changed it\n  in:    %q\n  once:  %q\n  twice: %q",
					md, once, twice)
			}
			parityCheckGoodRows(t, "format", once)
		})
	}
}
