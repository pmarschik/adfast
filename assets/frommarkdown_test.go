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

func hasCode(codes []string, code string) bool {
	return slices.Contains(codes, code)
}

func hasMedia(nodes []adf.Node, id string) bool {
	for _, n := range nodes {
		if m, ok := n.(*adf.Media); ok && m.ID == id {
			return true
		}
		if hasMedia(adf.NodeContent(n), id) {
			return true
		}
	}
	return false
}
