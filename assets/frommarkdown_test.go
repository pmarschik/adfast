package assets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// TestSyncOnEncode_SingleBatchReferencedOnly: two documents, three
// pending files — only the two referenced ones go up, in ONE batch, and
// both docs encode with media nodes afterwards.
func TestSyncOnEncode_SingleBatchReferencedOnly(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	for i, name := range []string{"one.png", "two.png", "scratch.png"} {
		mustDo(t, os.WriteFile(filepath.Join(dir, name), tinyPNG(t, i+1, i+1), 0o600))
	}
	store := mustStore(t, mdDir)

	var calls int
	up := mappedUploader(t, &calls, map[string]string{
		"assets/one.png": uuidA,
		"assets/two.png": uuidB,
	})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![a](assets/one.png)\n",
		"![b](assets/two.png)\n",
	})
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	if len(docs) != 2 {
		t.Fatalf("docs = %d", len(docs))
	}
	wantMedia(t, docs[0], uuidA)
	wantMedia(t, docs[1], uuidB)
	// The unreferenced scratch file must still be pending.
	wantPending(t, store, "assets/scratch.png")

	// Nothing new referenced: FromMarkdown must not call the uploader.
	calls = 0
	doc := PushPipeline(t.Context(), store, up).MarkdownToADF("![a](assets/one.png)\n")
	if calls != 0 {
		t.Errorf("uploader calls on already-synced doc = %d, want 0", calls)
	}
	wantMedia(t, doc, uuidA)
}

// TestSyncOnEncode_DotSlashReference: the only reference in the document
// is "./assets/x.png". The worklist says "assets/x.png", and the two
// spellings address one file, so it must upload and resolve.
func TestSyncOnEncode_DotSlashReference(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600))
	store := mustStore(t, mdDir)

	var calls int
	up := mappedUploader(t, &calls, map[string]string{"assets/one.png": uuidA})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![a](./assets/one.png)\n",
	})
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	wantMedia(t, docs[0], uuidA)
	wantPending(t, store)
}

// TestSyncOnEncode_MixedSpellingsUploadOnce: two documents name the same
// file with different spellings. It goes up once, and both resolve.
func TestSyncOnEncode_MixedSpellingsUploadOnce(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600))
	store := mustStore(t, mdDir)

	var calls int
	seen := 0
	up := UploaderFunc(func(ctx context.Context, batch []PendingAsset) ([]UploadResult, error) {
		seen += len(batch)
		return mappedUploader(t, &calls, map[string]string{"assets/one.png": uuidA}).Upload(ctx, batch)
	})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![a](./assets/one.png)\n",
		"![b](assets/one.png)\n",
	})
	if seen != 1 {
		t.Errorf("uploaded assets = %d, want 1", seen)
	}
	wantMedia(t, docs[0], uuidA)
	wantMedia(t, docs[1], uuidA)
}

// TestSyncOnEncode_ParentReference: a store whose assets folder sits
// above the document is addressed with "../assets/…", which is already
// the spelling Pending reports. It must keep working.
func TestSyncOnEncode_ParentReference(t *testing.T) {
	root := t.TempDir()
	dir := mustMkdir(t, filepath.Join(root, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600))
	docDir := mustMkdir(t, filepath.Join(root, "docs"))
	store := mustStoreAt(t, root, docDir)

	var calls int
	up := mappedUploader(t, &calls, map[string]string{"../assets/one.png": uuidA})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![a](../assets/one.png)\n",
	})
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	wantMedia(t, docs[0], uuidA)
}

// TestSyncOnEncode_ErrorHandling: MarkdownToADFAll aborts on an upload
// failure; the infallible MarkdownToADF downgrades it to a
// before-encode-failed diagnostic and proceeds.
func TestSyncOnEncode_ErrorHandling(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600))
	store := mustStore(t, mdDir)
	boom := errors.New("attachment API down")
	up := UploaderFunc(func(context.Context, []PendingAsset) ([]UploadResult, error) {
		return nil, boom
	})

	if _, err := PushPipeline(t.Context(), store, up).MarkdownToADFAll([]string{"![a](assets/one.png)\n"}); !errors.Is(err, boom) {
		t.Errorf("MarkdownToADFAll error = %v, want %v", err, boom)
	}

	var codes []string
	PushPipeline(t.Context(), store, up,
		adfast.WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })).
		MarkdownToADF("![a](assets/one.png)\n")
	if !hasCode(codes, "before-encode-failed") || !hasCode(codes, "unresolved-asset") {
		t.Errorf("diagnostics = %v, want before-encode-failed + unresolved-asset", codes)
	}
}

// TestSyncOnEncode_MediaSpellings: a picture has four spellings, and the
// upload scan must see the path in each one that spends it.
//
// ![alt](path) is the only one the scan used to match, so a hand-written
// ::media{path=…} never offered its file for upload: the store never
// learned the path, the resolve found nothing, and the media node
// shipped with an empty id. The three GOOD rows hold the other half of
// the rule — a path that the encode does NOT spend on a lookup must stay
// on the ground, because uploading it would leave an attachment the
// document never addresses.
//
// The two halves fail apart. Drop the directive arm of
// collectImageNodes and the three spellings that are not ![alt](path) go
// red, the GOOD rows staying green:
//
//	--- FAIL: TestSyncOnEncode_MediaSpellings/the_block_directive (0.00s)
//	    frommarkdown_test.go:240: uploader calls = 0, want 1 batch
//	    frommarkdown_test.go:245: lookup "assets/one.png" in scope "": no media id, want one of [b5773183-5f9a-481f-b1b8-8fe286bba8e9]
//	    frommarkdown_test.go:246: document has no media node for b5773183-5f9a-481f-b1b8-8fe286bba8e9
//	    frommarkdown_test.go:247: pending = [assets/one.png], want []
//	    (the same four in .../the_caption_directive and .../the_inline_chip)
//
// Drop dialect.MediaLookupPath's inert-source guard instead, so every
// path counts, and the complementary set goes red:
//
//	--- FAIL: TestSyncOnEncode_MediaSpellings/a_directive_pinning_an_explicit_id (0.00s)
//	    frommarkdown_test.go:229: uploader calls = 1, want 0 — this path is not a lookup key
//	    frommarkdown_test.go:232: pending = [], want [assets/one.png]
//	    (the same two in .../the_inline_chip_pinning_an_explicit_id, and in
//	    .../external_media, which also leaks a media node for the id it
//	    uploaded)
func TestSyncOnEncode_MediaSpellings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		md     string
		pinned string // the id the document addresses when it is not ours
		upload bool
	}{
		{name: "the image form", md: "![a](assets/one.png)\n", upload: true},
		{name: "the block directive", md: "::media[a]{path=assets/one.png}\n", upload: true},
		{
			name:   "the caption directive",
			md:     ":::media{path=assets/one.png}\nA caption.\n:::\n",
			upload: true,
		},
		{name: "the inline chip", md: "Before :media[a]{path=assets/one.png} after.\n", upload: true},
		{
			name:   "a directive pinning an explicit id",
			md:     "::media[a]{#" + uuidB + " path=assets/one.png}\n",
			upload: false,
			pinned: uuidB,
		},
		{
			name:   "the inline chip pinning an explicit id",
			md:     "Before :media[a]{#" + uuidB + " path=assets/one.png} after.\n",
			upload: false,
			pinned: uuidB,
		},
		{
			name:   "external media",
			md:     "::media[a]{type=external url=https://example.com/one.png path=assets/one.png}\n",
			upload: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mdDir := t.TempDir()
			dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
			mustDo(t, os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600))
			store := mustStore(t, mdDir)

			var calls int
			up := mappedUploader(t, &calls, map[string]string{"assets/one.png": uuidA})
			docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{tc.md})

			if !tc.upload {
				if calls != 0 {
					t.Errorf("uploader calls = %d, want 0 — this path is not a lookup key", calls)
				}
				// The file is untouched, so it is still on the worklist.
				wantPending(t, store, "assets/one.png")
				wantNoMedia(t, docs[0], uuidA)
				if tc.pinned != "" {
					wantMedia(t, docs[0], tc.pinned)
				}
				return
			}
			if calls != 1 {
				t.Errorf("uploader calls = %d, want 1 batch", calls)
			}
			// Collecting the path is only half the job: the upload has to
			// leave the store able to answer it, and this encode has to
			// carry the id it answered.
			wantLookup(t, store, "", "assets/one.png", uuidA)
			wantMedia(t, docs[0], uuidA)
			wantPending(t, store)
		})
	}
}

func hasCode(codes []string, code string) bool {
	return slices.Contains(codes, code)
}

// hasMedia reports whether a document addresses a media id, in either
// of the two ADF spellings: the block media node, and the inline chip.
// The question a caller asks is "does this document point at that
// attachment", and the answer must not depend on which of the two the
// author wrote.
func hasMedia(nodes []adf.Node, id string) bool {
	for _, n := range nodes {
		switch media := n.(type) {
		case *adf.Media:
			if media.ID == id {
				return true
			}
		case *adf.MediaInline:
			if media.ID == id {
				return true
			}
		}
		if hasMedia(adf.NodeContent(n), id) {
			return true
		}
	}
	return false
}
