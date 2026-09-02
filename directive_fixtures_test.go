package adfast

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// The fixtures in testdata/directive_fixtures.json are generated from the
// reference implementations (remark-parse + remark-directive + remark-gfm
// and prettier) — remark is the parity reference for directive behavior.
type directiveFixtures struct {
	Markdown []struct {
		Md        string          `json:"md"`
		Roundtrip string          `json:"roundtrip"`
		Adf       json.RawMessage `json:"adf"`
	} `json:"markdown"`
	Adf []struct {
		Md  string          `json:"md"`
		Adf json.RawMessage `json:"adf"`
	} `json:"adf"`
}

func loadDirectiveFixtures(t *testing.T) directiveFixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/directive_fixtures.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var f directiveFixtures
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("unmarshal fixtures: %v", err)
	}
	if len(f.Markdown) == 0 || len(f.Adf) == 0 {
		t.Fatal("fixtures empty")
	}
	return f
}

// normalizeAdfJSON re-marshals fixture ADF through the typed adf.Doc so that both
// sides use identical key ordering and field presence.
func normalizeAdfJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal adf: %v", err)
	}
	doc, ok := adf.DecodeDoc(v)
	if !ok {
		t.Fatalf("adf.DecodeDoc failed for %s", raw)
	}
	return marshalDoc(t, doc)
}

func marshalDoc(t *testing.T, doc adf.Doc) string {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	return string(b)
}

// TestDirectiveFixtures_FromMarkdown asserts that Go's FromMarkdown produces
// fixtureMediaAssets is the fixed media-asset environment both the Go
// pipeline and the reference fixture generator run the probes under.
var fixtureMediaAssets = map[string]convert.MediaAsset{
	"b5773183-5f9a-481f-b1b8-8fe286bba8e9": {Path: "assets/shot.png", Width: 2308, Height: 551},
	"0a1b2c3d-1111-2222-3333-444455556666": {Path: "assets/old.png", Width: 100, Height: 60},
	"9f8e7d6c-aaaa-bbbb-cccc-ddddeeeeffff": {Path: "assets/spec.pdf"},
}

// fixtureAssetID mirrors the generator's resolver of the same name.
func fixtureAssetID(path string) (string, bool) {
	for id, a := range fixtureMediaAssets {
		if a.Path == path {
			return id, true
		}
	}
	return "", false
}

// fixtureImageDims mirrors the generator's resolver of the same name.
func fixtureImageDims(path string) (width, height int, ok bool) {
	for _, a := range fixtureMediaAssets {
		if a.Path == path && a.Width > 0 {
			return a.Width, a.Height, true
		}
	}
	return 0, 0, false
}

// the exact ADF the remark reference pipeline produces for every directive probe.
func TestDirectiveFixtures_FromMarkdown(t *testing.T) {
	fixtures := loadDirectiveFixtures(t)
	for _, f := range fixtures.Markdown {
		want := normalizeAdfJSON(t, f.Adf)
		got := marshalDoc(t, mdToADF(f.Md, WithImageDimsResolver(fixtureImageDims), WithAssetIDResolver(fixtureAssetID), WithSmartLinks(jiraTestSmartLinks)))
		if got != want {
			t.Errorf("mdToADF(%q) diverged from the remark reference corpus\n got: %s\nwant: %s", f.Md, got, want)
		}
	}
}

// TestDirectiveFixtures_RoundTripStable asserts that Go's own render→parse→
// render cycle is stable for every directive probe (byte-level equality with
// the reference corpus's rendered markdown is not required — escaping breadth
// differs by design — but each side must be self-consistent).
func TestDirectiveFixtures_RoundTripStable(t *testing.T) {
	fixtures := loadDirectiveFixtures(t)
	for _, f := range fixtures.Markdown {
		doc := mdToADF(f.Md, WithImageDimsResolver(fixtureImageDims), WithAssetIDResolver(fixtureAssetID))
		first := adfToMD(doc, WithMediaAssets(fixtureMediaAssets))
		second := adfToMD(mdToADF(first, WithImageDimsResolver(fixtureImageDims), WithAssetIDResolver(fixtureAssetID)), WithMediaAssets(fixtureMediaAssets))
		if first != second {
			t.Errorf("round-trip unstable for %q:\nfirst:  %q\nsecond: %q", f.Md, first, second)
		}
	}
}

// TestDirectiveFixtures_ToMarkdown asserts byte-exact parity with the remark
// reference corpus for rendering ADF documents (panel layouts and
// colon-escaping contexts).
func TestDirectiveFixtures_ToMarkdown(t *testing.T) {
	fixtures := loadDirectiveFixtures(t)
	for _, f := range fixtures.Adf {
		var v map[string]any
		if err := json.Unmarshal(f.Adf, &v); err != nil {
			t.Fatalf("unmarshal adf: %v", err)
		}
		got := adfToMD(v, WithMediaAssets(fixtureMediaAssets), WithSmartLinks(jiraTestSmartLinks))
		if got != f.Md {
			t.Errorf("ToMarkdown diverged from the remark reference corpus for %s\n got: %q\nwant: %q", f.Adf, got, f.Md)
		}
	}
}
