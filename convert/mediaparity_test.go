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
// Facts about the mirror, measured while writing this file, that the
// reader of either copy cannot see from one side alone:
//
//   - singleBlocksImage differs, and must: dialect also tests
//     adf.HasExtra for a layout or width whose JSON value is not the type
//     the field holds. A directive attribute is always a string, so the
//     fmtMedia side has nothing to test.
//   - the leaf CONSTRUCTOR is no longer doubled. normalize.go's
//     mediaLeafNode used to end in a hand-written copy of dialect's
//     unexported newMedia, field for field, so a new attribute on
//     dialect.Media needed two edits and the compiler named neither;
//     both copies now call dialect.NewMedia.
//   - the blocking predicate the file and external image paths share
//     (occurrenceKey, non-empty collection, border) used to be restated
//     inside mediaAsImage AND inside fileMediaAsImage here, so it lived
//     in three places against dialect's one. normalize.go now factors it
//     into fmtMedia.blocksImage, the counterpart of dialect's
//     mediaBlocksImage. The two are still separate functions: they read
//     different types (adf.Media + adf.MediaSingle against fmtMedia),
//     and the only shape both could call is one taking the four answers
//     as booleans — which would move the actual checks back out to the
//     call sites and lose exactly what factoring them in bought.
//   - the STORE RESOLUTION is no longer doubled either, and was worse
//     than doubled: recovering the media id and the intrinsic size a
//     path-addressed directive leaves out was spelled at dialect's
//     ::media encode site only. :::media never did it, and the format
//     leg looked assets up by id alone. Both legs now call
//     internal/mediasrc, and on the ADF leg the recovery sits inside
//     mediaFromAttrs, where no encode site can skip it.

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
		// Reconciled drift, and the row that proves it: this used to sit
		// in TestMediaProjectionLegsDivergeAsMeasured because
		// WithPreserveLocalImages reached only the ADF leg — the format
		// copy hardcoded an absolute-http test and the normalizer never
		// received the flag, so the ADF leg rendered ![alt](img/a.png)
		// and the format leg kept the directive. Both legs now ask
		// mediaurl.ProjectsToImage.
		{
			"relative external media under WithPreserveLocalImages",
			"::media[alt]{type=external url=img/a.png}",
			[]Option{WithPreserveLocalImages()},
		},
		{"file media in the store", "::media[alt]{id=AID width=10 height=20}", assets},
		// Reconciled drift, the second row to move up out of the
		// divergence table: a path-addressed directive spells neither id
		// nor size, and only the ADF leg recovered them from the store, so
		// one directive was a picture on one leg and an opaque directive
		// on the other. Both legs now ask mediasrc.ID/mediasrc.Dims.
		{"file media addressed by path", "::media[alt]{path=assets/a.png}", assets},
		{"plain caption on path-addressed media", ":::media[alt]{path=assets/a.png}\nA caption\n:::", assets},
		{"rich caption on path-addressed media", ":::media[alt]{path=assets/a.png}\nA **bold** caption\n:::", assets},
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
		//
		// Empty, and that is the point of the table. Both rows this group
		// has held so far — external media under WithPreserveLocalImages,
		// and file media addressed by path — are reconciled and now live
		// in parityAgreeing; see the comments on them there. A new row
		// belongs here only as a measurement, until the missing rule moves
		// into a function both legs call.
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

// TestMediaFormatLegHonorsPreserveLocalImages pins the format leg's
// exact bytes for a document-relative external media, with and without
// WithPreserveLocalImages, in one document.
//
// The "with" case is a DEFECT PROOF: it fails on the pre-fix normalizer,
// which hardcoded an absolute-http test and had no field for the option,
// so the flag could not reach the decision at all and a caller could not
// tell from the output which leg had run.
//
// The "without" case is a PRESERVED-BEHAVIOR PIN: it passes on the
// pre-fix normalizer too. It is not evidence of a defect; it is the
// guard that threading the option in did not turn the default on, since
// the default is what keeps a relative external url lossless across
// re-encode.
func TestMediaFormatLegHonorsPreserveLocalImages(t *testing.T) {
	t.Parallel()
	const row = "::media[alt]{type=external url=img/a.png}"
	good := "\n\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
	cases := []struct {
		name string
		want string
		kind string
		opts []Option
	}{
		{
			name: "without the option the directive stays",
			want: "::media[alt]{type=\"external\" url=\"img/a.png\"}" + good,
			kind: "preserved-behavior PIN",
			opts: nil,
		},
		{
			name: "with the option the picture reaches image form",
			want: "![alt](img/a.png)" + good,
			kind: "defect proof",
			opts: []Option{WithPreserveLocalImages()},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(row)
			if got := parityFormatLeg(md, c.opts...); got != c.want {
				t.Errorf("%s: format leg output changed\n  in:   %q\n  got:  %q\n  want: %q",
					c.kind, md, got, c.want)
			}
		})
	}
}

// TestMediaFormatLegKeepsThePathAttribute pins the ONE media asymmetry
// between the legs that is not drift and must not be reconciled: for a
// store-known asset the format leg emits the markdown-relative path=
// where dialect's leaf prefers id= (mediaSourceAttrs). The format leg is
// total — it may not delete an author's path and make the document
// depend on a store lookup to say where the picture is — while the ADF
// leg addresses the attachment by media id because that is what ADF
// holds.
//
// This is a PRESERVED-BEHAVIOR PIN: it passes before and after the
// preserveLocalImages fix, and exists so a later attempt to collapse the
// two projections into one function fails loudly here.
func TestMediaFormatLegKeepsThePathAttribute(t *testing.T) {
	t.Parallel()
	good := "\n\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
	// Richer than an image (a border), so the leaf form is what renders
	// and its source attributes are visible.
	row := "::media[alt]{id=AID width=10 height=20 borderColor=#000 borderSize=2}"
	want := "::media[alt]{borderColor=\"#000\" borderSize=\"2\" path=\"assets/a.png\"}" + good
	got := parityFormatLeg(parityDoc(row), parityAssetOpts()...)
	if got != want {
		t.Errorf("the format leg's path-over-id preference changed\n  in:   %q\n  got:  %q\n  want: %q",
			row, got, want)
	}
	if strings.Contains(got, "id=") {
		t.Errorf("the format leg emitted an id= it should have resolved to a path=: %q", got)
	}
}

// TestPathAddressedMediaResolvesOnBothLegs pins the exact bytes both
// legs give a media directive that spells only a path — the form a
// pulled document uses, since an attachment downloaded next to the
// markdown needs no id and no size in the text.
//
// parityAgreeing already fails when the two legs disagree; this test
// exists because agreement is not the whole property. Both legs could
// agree on losing the picture, so the picture is pinned here.
//
// Two mutations, one per leg. Dropping the mediasrc calls from the
// normalizer's mediaShape — the format leg's pre-fix state, an asset
// lookup by id alone:
//
//	--- FAIL: TestPathAddressedMediaResolvesOnBothLegs/the_leaf_reaches_image_form
//	    defect proof (format leg): format leg
//	      in:   "::media[alt]{path=assets/a.png}…"
//	      got:  "::media[alt]{path=\"assets/a.png\"}…"
//	      want: "![alt](assets/a.png)…"
//	    …/a_plain_caption_reaches_image_form likewise, plus
//	    TestMediaProjectionLegsAgree on both path rows.
//
// Building the caption carrier's leaf without the store — the ADF leg's
// pre-fix state, since the recovery was written at the ::media site and
// not inside mediaFromAttrs:
//
//	--- FAIL: TestPathAddressedMediaResolvesOnBothLegs/a_rich_caption_keeps_the_source
//	    defect proof (adf leg): adf leg
//	      in:   ":::media[alt]{path=assets/a.png}\nA **bold** caption\n:::…"
//	      got:  ":::media[alt]\nA **bold** caption\n:::…"
//	      want: ":::media[alt]{path=\"assets/a.png\"}\nA **bold** caption\n:::…"
//
// The "got" there is the whole point: the payload held a media node with
// an empty id, an attachment no Atlassian product can address.
func TestPathAddressedMediaResolvesOnBothLegs(t *testing.T) {
	t.Parallel()
	assets := parityAssetOpts()
	good := "\n\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
	cases := []struct {
		name string
		row  string
		want string
		kind string
	}{
		{
			// The format leg looked the asset up by id alone and kept an
			// opaque directive where the ADF leg produced a picture.
			name: "the leaf reaches image form",
			row:  "::media[alt]{path=assets/a.png}",
			want: "![alt](assets/a.png)" + good,
			kind: "defect proof (format leg)",
		},
		{
			// Same on the caption carrier, whose plain-text caption becomes
			// the image title.
			name: "a plain caption reaches image form",
			row:  ":::media[alt]{path=assets/a.png}\nA caption\n:::",
			want: "![alt](assets/a.png \"A caption\")" + good,
			kind: "defect proof (format leg)",
		},
		{
			// This one is an ADF-leg defect: MediaCaption.EncodeADF built
			// the leaf without the store resolution, so the payload carried
			// a media node with an empty id — an attachment Atlassian
			// cannot address — and reading it back left a bare :::media
			// with no source at all.
			name: "a rich caption keeps the source",
			row:  ":::media[alt]{path=assets/a.png}\nA **bold** caption\n:::",
			want: ":::media[alt]{path=\"assets/a.png\"}\nA **bold** caption\n:::" + good,
			kind: "defect proof (adf leg)",
		},
		{
			// The good case for all three: a path-addressed media too rich
			// for an image still keeps its path, so the fix recovered the
			// source without collapsing everything into a picture.
			name: "a bordered leaf keeps the directive",
			row:  "::media[alt]{path=assets/a.png borderColor=#000 borderSize=2}",
			want: "::media[alt]{borderColor=\"#000\" borderSize=\"2\" path=\"assets/a.png\"}" + good,
			kind: "good case",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(c.row)
			if got := parityFormatLeg(md, assets...); got != c.want {
				t.Errorf("%s: format leg\n  in:   %q\n  got:  %q\n  want: %q", c.kind, md, got, c.want)
			}
			if got := parityADFLeg(md, assets...); got != c.want {
				t.Errorf("%s: adf leg\n  in:   %q\n  got:  %q\n  want: %q", c.kind, md, got, c.want)
			}
		})
	}
}

// TestNoOpResizeDoesNotBlockTheImageForm pins the resolution of a
// contradiction inside the media projection that the path resolution
// above made reachable: mediaOmissions drops a pixel display width equal
// to the picture's intrinsic width as a redundant no-op, while
// singleBlocksImage counted the same attribute as a reason the picture
// could not take the plain image form. A media node with both facts
// therefore rendered as a directive that no longer spelled the width —
// and that directive, read again, was a picture. The formatter changed
// its own output.
//
// One predicate now answers both questions, so a display width the text
// omits cannot also be a display width the text depends on.
//
// Mutation: with singleBlocksImage blocking on any layoutWidth again
// (both copies), the fix row and the fixpoint both report the picture
// stuck in directive form, one pass short of the image:
//
//	--- FAIL: TestNoOpResizeDoesNotBlockTheImageForm/an_intrinsic-width_pixel_resize_is_not_a_resize
//	    fix: format leg
//	      in:   "::media[alt]{id=AID width=10 height=20 layoutWidth=10 widthType=pixel}…"
//	      got:  "::media[alt]{path=\"assets/a.png\"}…"
//	      want: "![alt](assets/a.png)…"
//	    fix: adf leg — the same got/want.
//	--- FAIL: TestMediaFormatLegIsAFixpoint/no-op_resize
//	    formatting the formatter's own media output changed it
//	      in:    "::media[alt]{id=AID width=10 height=20 layoutWidth=10 widthType=pixel}…"
//	      once:  "::media[alt]{path=\"assets/a.png\"}…"
//	      twice: "![alt](assets/a.png)…"
func TestNoOpResizeDoesNotBlockTheImageForm(t *testing.T) {
	t.Parallel()
	assets := parityAssetOpts()
	good := "\n\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
	cases := []struct {
		name string
		row  string
		want string
		kind string
	}{
		{
			name: "an intrinsic-width pixel resize is not a resize",
			row:  "::media[alt]{id=AID width=10 height=20 layoutWidth=10 widthType=pixel}",
			want: "![alt](assets/a.png)" + good,
			kind: "fix",
		},
		{
			name: "a narrower display width still blocks",
			row:  "::media[alt]{id=AID width=10 height=20 layoutWidth=5 widthType=pixel}",
			want: "::media[alt]{layoutWidth=\"5\" path=\"assets/a.png\" widthType=\"pixel\"}" + good,
			kind: "good case",
		},
		{
			name: "the same number in percent still blocks",
			row:  "::media[alt]{id=AID width=10 height=20 layoutWidth=10 widthType=percentage}",
			want: "::media[alt]{layoutWidth=\"10\" path=\"assets/a.png\" widthType=\"percentage\"}" + good,
			kind: "good case",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			md := parityDoc(c.row)
			if got := parityFormatLeg(md, assets...); got != c.want {
				t.Errorf("%s: format leg\n  in:   %q\n  got:  %q\n  want: %q", c.kind, md, got, c.want)
			}
			if got := parityADFLeg(md, assets...); got != c.want {
				t.Errorf("%s: adf leg\n  in:   %q\n  got:  %q\n  want: %q", c.kind, md, got, c.want)
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
