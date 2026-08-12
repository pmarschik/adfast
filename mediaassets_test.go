package adfast

import (
	"slices"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// mediaAssetSpy is a WithMediaAssetResolver that answers for the ids it holds
// and records every id it was asked about, in order.
type mediaAssetSpy struct {
	held  map[string]convert.MediaAsset
	asked []string
}

func (s *mediaAssetSpy) resolve(id string) (convert.MediaAsset, bool) {
	s.asked = append(s.asked, id)
	a, ok := s.held[id]
	return a, ok
}

// downloadedMedia is a file media node whose recorded dimensions match the
// assets the tests hold, so a resolved id collapses to a plain image.
func downloadedMedia(id, alt string) adf.Node {
	return &adf.MediaSingle{
		Layout: ptrOf("align-start"),
		Content: []adf.Node{&adf.Media{
			Type: "file", ID: id, Alt: alt,
			Collection: ptrOf(""), Width: ptrOf(float64(8)), Height: ptrOf(float64(4)),
		}},
	}
}

// The whole point of the resolver form: a caller whose collection is large (or
// whose lookups cost something) is asked about the media in the document and
// nothing else.
func TestMediaAssetResolverAskedOnlyAboutTheMediaPresent(t *testing.T) {
	spy := &mediaAssetSpy{held: map[string]convert.MediaAsset{
		"id-here":      {Path: "assets/here.png", Width: 8, Height: 4, HasDim: true},
		"id-elsewhere": {Path: "assets/elsewhere.png", Width: 8, Height: 4, HasDim: true},
	}}

	md := adfToMD(doc(downloadedMedia("id-here", "here.png")),
		WithMediaAssetResolver(spy.resolve))

	if !strings.Contains(md, "![here.png](assets/here.png)") {
		t.Errorf("resolved media must render as a local image, got:\n%s", md)
	}
	if slices.Contains(spy.asked, "id-elsewhere") {
		t.Errorf("resolver asked about media the document does not contain: %v", spy.asked)
	}
}

// An id the resolver does not hold is not an error: that media keeps its
// directive, exactly as an absent WithMediaAssets entry leaves it.
func TestMediaAssetResolverMissKeepsTheDirective(t *testing.T) {
	spy := &mediaAssetSpy{}

	md := adfToMD(doc(downloadedMedia("id-unknown", "shot.png")),
		WithMediaAssetResolver(spy.resolve))

	if strings.Contains(md, "![") {
		t.Errorf("unresolved media must not render as an image, got:\n%s", md)
	}
	if !strings.Contains(md, "::media") {
		t.Errorf("unresolved media must stay a directive, got:\n%s", md)
	}
	if len(spy.asked) == 0 {
		t.Error("resolver must be asked about the media the document contains")
	}
}

// A resolver is allowed to be expensive, so the same id is resolved once per
// conversion however many times the walk (and normalization) looks it up.
func TestMediaAssetResolverAnswersAreMemoized(t *testing.T) {
	spy := &mediaAssetSpy{held: map[string]convert.MediaAsset{
		"id-twice": {Path: "assets/twice.png", Width: 8, Height: 4, HasDim: true},
	}}
	in := doc(
		downloadedMedia("id-twice", "twice.png"),
		downloadedMedia("id-twice", "twice.png"),
		downloadedMedia("id-gone", "gone.png"),
		downloadedMedia("id-gone", "gone.png"),
	)

	adfToMD(in, WithMediaAssetResolver(spy.resolve))

	for _, id := range []string{"id-twice", "id-gone"} {
		asked := 0
		for _, got := range spy.asked {
			if got == id {
				asked++
			}
		}
		if asked != 1 {
			t.Errorf("resolver asked about %s %d times, want 1 (asked: %v)", id, asked, spy.asked)
		}
	}
}

// Both option forms describe the same knowledge, so they compose: the map is
// the caller's already-known media, the resolver the rest.
func TestMediaAssetResolverFillsInWhatTheMapDoesNotCover(t *testing.T) {
	spy := &mediaAssetSpy{held: map[string]convert.MediaAsset{
		"id-lazy": {Path: "assets/lazy.png", Width: 8, Height: 4, HasDim: true},
	}}
	in := doc(
		downloadedMedia("id-eager", "eager.png"),
		downloadedMedia("id-lazy", "lazy.png"),
	)

	md := adfToMD(in,
		WithMediaAssets(map[string]convert.MediaAsset{
			"id-eager": {Path: "assets/eager.png", Width: 8, Height: 4, HasDim: true},
		}),
		WithMediaAssetResolver(spy.resolve))

	for _, want := range []string{"![eager.png](assets/eager.png)", "![lazy.png](assets/lazy.png)"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %s in:\n%s", want, md)
		}
	}
	if slices.Contains(spy.asked, "id-eager") {
		t.Errorf("resolver asked about media the map already covers: %v", spy.asked)
	}
}

// HasDim is derived for resolver replies the same way WithMediaAssets derives
// it, so a caller that predates the field is understood either way.
func TestMediaAssetResolverDerivesHasDim(t *testing.T) {
	spy := &mediaAssetSpy{held: map[string]convert.MediaAsset{
		"id-dims": {Path: "assets/dims.png", Width: 8, Height: 4},
	}}

	md := adfToMD(doc(downloadedMedia("id-dims", "dims.png")),
		WithMediaAssetResolver(spy.resolve))

	if !strings.Contains(md, "![dims.png](assets/dims.png)") {
		t.Errorf("dimensions must count as known without an explicit HasDim, got:\n%s", md)
	}
}

// Format mode reads the same options as the ADF decode (see ToMarkdown), so the
// store-aware media slimming works from a resolver too.
func TestFormatModeSlimsMediaViaResolver(t *testing.T) {
	spy := &mediaAssetSpy{held: map[string]convert.MediaAsset{
		"abc-123": {Path: "assets/shot.png", Width: 817, Height: 182, HasDim: true},
	}}
	md := `::media[shot.png]{#abc-123 collection height="182" layout="align-start" layoutWidth="671" type="file" width="817" widthType="pixel"}` + "\n"

	out := fmtMD(md, WithMediaAssetResolver(spy.resolve))

	if !strings.Contains(out, `path="assets/shot.png"`) {
		t.Errorf("expected the local path in:\n%s", out)
	}
	if strings.Contains(out, "#abc-123") {
		t.Errorf("expected the explicit id dropped in:\n%s", out)
	}
}
