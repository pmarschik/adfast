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
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"one.png", "two.png", "scratch.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), tinyPNG(t, i+1, i+1), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}

	var calls int
	up := UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		calls++
		ids := map[string]string{"assets/one.png": uuidA, "assets/two.png": uuidB}
		results := make([]UploadResult, 0, len(batch))
		for _, p := range batch {
			id, ok := ids[p.Path]
			if !ok {
				t.Errorf("unexpected upload of %s", p.Path)
				continue
			}
			results = append(results, UploadResult{Path: p.Path, MediaID: id})
		}
		return results, nil
	})

	docs, err := PushPipeline(t.Context(), store, up).MarkdownToADFAll([]string{
		"![a](assets/one.png)\n",
		"![b](assets/two.png)\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	if len(docs) != 2 {
		t.Fatalf("docs = %d", len(docs))
	}
	for i, id := range []string{uuidA, uuidB} {
		if !hasMedia(docs[i].Content, id) {
			t.Errorf("doc %d: no media node for %s", i, id)
		}
	}
	// The unreferenced scratch file must still be pending.
	pending, err := store.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0] != "assets/scratch.png" {
		t.Errorf("pending after = %v", pending)
	}

	// Nothing new referenced: FromMarkdown must not call the uploader.
	calls = 0
	doc := PushPipeline(t.Context(), store, up).MarkdownToADF("![a](assets/one.png)\n")
	if calls != 0 {
		t.Errorf("uploader calls on already-synced doc = %d, want 0", calls)
	}
	if !hasMedia(doc.Content, uuidA) {
		t.Error("re-encode lost the media node")
	}
}

// TestSyncOnEncode_ErrorHandling: MarkdownToADFAll aborts on an upload
// failure; the infallible MarkdownToADF downgrades it to a
// before-encode-failed diagnostic and proceeds.
func TestSyncOnEncode_ErrorHandling(t *testing.T) {
	mdDir := t.TempDir()
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
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
