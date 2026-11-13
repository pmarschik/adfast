package adfast

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The first two tests are PRESERVED-BEHAVIOR PINS that hold
// WithPreserveLocalImages' doc comment to the code: off, the picture
// drops but the LABEL survives as a link to the path; on, an image on
// its own line becomes external media carrying the path.
//
// The rest pin the option's reach into the other media forms. It used
// to stop at the own-line image, because mediaInline had no url to hold
// a path and the ::media directives spent their path as a store key and
// nothing else: an inline image, an inline :media{path} chip and a
// block ::media{path} directive all lost the path under the option
// exactly as they did without it. Now each one keeps it as an external
// url, and each of those tests also runs the option OFF, so a passing
// test cannot be the option failing to change anything.

// pliADF encodes md and returns the ADF wire JSON plus the diagnostic
// codes, with the option on or off.
func pliADF(t *testing.T, md string, preserve bool) (wire string, codes []string) {
	t.Helper()
	opts := []Option{WithDiagnostics(func(d convert.Diagnostic) {
		codes = append(codes, d.Code)
	})}
	if preserve {
		opts = append(opts, WithPreserveLocalImages())
	}
	b, err := json.Marshal(mdToADF(md, opts...))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b), codes
}

func TestPreserveLocalImages_OffKeepsTheLabelAsALinkToThePath(t *testing.T) {
	const src = "![sketch](assets/x.png)\n"

	got, codes := pliADF(t, src, false)

	// The claim under test is that the label SURVIVES. Assert it is
	// there rather than only that the media node is absent, which any
	// empty document would satisfy.
	if !strings.Contains(got, `"text":"sketch"`) {
		t.Errorf("the alt text did not survive the degradation:\n%s", got)
	}
	if !strings.Contains(got, `"href":"assets/x.png"`) {
		t.Errorf("the path did not survive as a link destination:\n%s", got)
	}
	if strings.Contains(got, `"media"`) {
		t.Errorf("off, the local image must not reach a media node:\n%s", got)
	}
	if len(codes) != 1 || codes[0] != convert.CodeUnresolvedAsset {
		t.Errorf("diagnostics = %v, want exactly [%s]", codes, convert.CodeUnresolvedAsset)
	}
}

func TestPreserveLocalImages_OnPromotesAnOwnLineImageToExternalMedia(t *testing.T) {
	const src = "![sketch](assets/x.png)\n"

	got, codes := pliADF(t, src, true)

	if !strings.Contains(got, `"type":"external"`) || !strings.Contains(got, `"url":"assets/x.png"`) {
		t.Errorf("the option did not promote the image to external media:\n%s", got)
	}
	if len(codes) != 0 {
		t.Errorf("diagnostics = %v, want none: the path traveled, so nothing was lost", codes)
	}
}

// pliRoundTrip runs md through ToADF and back to markdown, then through
// ToADF again, with the option on, and fails unless the second encode
// equals the first: the path must be a FIXED POINT of the projection,
// not just survive one leg of it.
func pliRoundTrip(t *testing.T, md string) (back string) {
	t.Helper()
	opts := []Option{WithPreserveLocalImages()}
	first := mdToADF(md, opts...)
	back = ToMarkdown(FromADF(first, opts...), opts...)
	second := mdToADF(back, opts...)
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("md → adf → md → adf moved the document\nfirst:  %s\nsecond: %s\nvia markdown:\n%s", a, b, back)
	}
	if again := ToMarkdown(FromADF(second, opts...), opts...); again != back {
		t.Errorf("md → adf → md is not idempotent\nonce:  %q\ntwice: %q", back, again)
	}
	return back
}

func TestPreserveLocalImages_KeepsAnInlineImageAsExternalMediaInline(t *testing.T) {
	const src = "see ![sketch](assets/x.png) here\n"

	off, offCodes := pliADF(t, src, false)
	on, onCodes := pliADF(t, src, true)

	// Off is unchanged: the label stays as a link to the path, and the
	// loss is reported.
	if strings.Contains(off, `"mediaInline"`) || !strings.Contains(off, `"href":"assets/x.png"`) {
		t.Errorf("off, an unplaceable inline image must degrade to a link to its path:\n%s", off)
	}
	if len(offCodes) != 1 || offCodes[0] != convert.CodeUnresolvedAsset {
		t.Errorf("off: diagnostics = %v, want exactly [%s]", offCodes, convert.CodeUnresolvedAsset)
	}
	// On, the picture itself travels, mid-sentence.
	const want = `{"type":"mediaInline","attrs":{"alt":"sketch","type":"external","url":"assets/x.png"}}`
	if !strings.Contains(on, want) {
		t.Errorf("on: want %s in\n%s", want, on)
	}
	if len(onCodes) != 0 {
		t.Errorf("on: diagnostics = %v, want none: the path traveled", onCodes)
	}
	if back := pliRoundTrip(t, src); back != src {
		t.Errorf("on: md → adf → md = %q, want the source %q back", back, src)
	}
}

func TestPreserveLocalImages_KeepsALinkedInlineImageAndItsLink(t *testing.T) {
	const src = "see [![sketch](assets/x.png)](https://example.com/full) here\n"

	on, _ := pliADF(t, src, true)
	if !strings.Contains(on, `"url":"assets/x.png"`) || !strings.Contains(on, `"href":"https://example.com/full"`) {
		t.Errorf("on: the linked inline image lost its path or its link:\n%s", on)
	}
	if back := pliRoundTrip(t, src); back != src {
		t.Errorf("on: md → adf → md = %q, want the source %q back", back, src)
	}
}

func TestPreserveLocalImages_KeepsAnInlineMediaChipPath(t *testing.T) {
	const src = "see :media[sketch]{path=\"assets/x.png\"} here\n"

	off, offCodes := pliADF(t, src, false)
	on, onCodes := pliADF(t, src, true)

	if strings.Contains(off, `assets/x.png`) {
		t.Errorf("off: the chip's path has no slot without the option, yet it traveled:\n%s", off)
	}
	if len(offCodes) != 1 || offCodes[0] != convert.CodeUnresolvedAsset {
		t.Errorf("off: diagnostics = %v, want exactly [%s]", offCodes, convert.CodeUnresolvedAsset)
	}
	const want = `{"type":"mediaInline","attrs":{"alt":"sketch","type":"external","url":"assets/x.png"}}`
	if !strings.Contains(on, want) {
		t.Errorf("on: want %s in\n%s", want, on)
	}
	if len(onCodes) != 0 {
		t.Errorf("on: diagnostics = %v, want none: the path traveled", onCodes)
	}
	// The chip comes back as the image it is equivalent to, and that is
	// the fixed point.
	if back := pliRoundTrip(t, src); back != "see ![sketch](assets/x.png) here\n" {
		t.Errorf("on: md → adf → md = %q", back)
	}
}

func TestPreserveLocalImages_KeepsABlockMediaDirectivePath(t *testing.T) {
	for _, src := range []string{
		"::media[sketch]{path=\"assets/x.png\"}\n",
		":::media[sketch]{path=\"assets/x.png\"}\nA caption.\n:::\n",
	} {
		off, offCodes := pliADF(t, src, false)
		on, onCodes := pliADF(t, src, true)

		if strings.Contains(off, `assets/x.png`) || !strings.Contains(off, `"type":"file"`) {
			t.Errorf("off: want a file media with no path for %q:\n%s", src, off)
		}
		if len(offCodes) != 1 || offCodes[0] != convert.CodeUnresolvedAsset {
			t.Errorf("off: diagnostics = %v for %q, want exactly [%s]", offCodes, src, convert.CodeUnresolvedAsset)
		}
		const want = `{"type":"media","attrs":{"alt":"sketch","type":"external","url":"assets/x.png"}}`
		if !strings.Contains(on, want) {
			t.Errorf("on: want %s for %q in\n%s", want, src, on)
		}
		if len(onCodes) != 0 {
			t.Errorf("on: diagnostics = %v for %q, want none: the path traveled", onCodes, src)
		}
		if back := pliRoundTrip(t, src); !strings.Contains(back, "assets/x.png") {
			t.Errorf("on: md → adf → md lost the path of %q: %q", src, back)
		}
	}
}

// The option keeps only a path NOTHING ELSE addresses. A store id, an
// explicit id, an explicit url and a non-file type each win over it, so
// turning the option on never re-points a picture that already has a
// source.
func TestPreserveLocalImages_YieldsToEveryOtherSource(t *testing.T) {
	store := WithAssetIDResolver(func(path string) (string, bool) {
		return "store-id", path == "assets/known.png"
	})
	cases := []struct{ name, src, want string }{
		{"store id, block", "::media[k]{path=\"assets/known.png\"}\n", `"id":"store-id"`},
		{"store id, chip", "a :media[k]{path=\"assets/known.png\"} b\n", `"id":"store-id"`},
		{"store id, inline image", "a ![k](assets/known.png) b\n", `"id":"store-id"`},
		{"explicit id", "::media[k]{id=\"pinned\" path=\"assets/x.png\"}\n", `"id":"pinned"`},
		{"explicit url", "a :media[k]{type=\"external\" url=\"https://e/k.png\" path=\"assets/x.png\"} b\n", `"url":"https://e/k.png"`},
		{"link type", "::media[k]{type=\"link\" path=\"assets/x.png\"}\n", `"type":"link"`},
	}
	for _, c := range cases {
		b, err := json.Marshal(mdToADF(c.src, store, WithPreserveLocalImages()))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := string(b)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: want %s in\n%s", c.name, c.want, got)
		}
		if strings.Contains(got, `"url":"assets/`) {
			t.Errorf("%s: the option re-pointed a sourced picture at its path:\n%s", c.name, got)
		}
	}
}

// An external chip's url is part of the node: the encode writes it with
// or without the option, and the decode reads it back. Without the
// option a relative url stays the directive, because the image form
// would re-encode to a link.
func TestMediaInline_ExternalURLRoundTrips(t *testing.T) {
	const src = "a :media[k]{type=\"external\" url=\"rel/k.png\"} b\n"
	const want = `{"type":"mediaInline","attrs":{"alt":"k","type":"external","url":"rel/k.png"}}`

	got, _ := pliADF(t, src, false)
	if !strings.Contains(got, want) {
		t.Errorf("want %s in\n%s", want, got)
	}
	if back := ToMarkdown(FromADF(mdToADF(src))); back != src {
		t.Errorf("md → adf → md = %q, want the source %q back", back, src)
	}
	// The format leg keeps the url too, so it encodes to the same node.
	formatted := ToMarkdown(FromMarkdown(src, WithPrettierFormat()), WithPrettierFormat())
	if f, _ := pliADF(t, formatted, false); f != got {
		t.Errorf("format leg moved the encode\nformatted: %q\nadf: %s\nwant: %s", formatted, f, got)
	}
}
