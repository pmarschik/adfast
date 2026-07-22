package assets

import (
	"context"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// TestEnsureUploaded: the push-side entry point syncs pending assets
// FIRST and returns markdown options that already resolve the
// just-uploaded ids — encoding never observes an uploadable asset.
func TestEnsureUploaded(t *testing.T) {
	mdDir := t.TempDir()
	writeAsset(t, mdDir, "shot.png", tinyPNG(t, 2, 2))
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	up := UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		calls++
		results := make([]UploadResult, 0, len(batch))
		for _, p := range batch {
			results = append(results, UploadResult{Path: p.Path, MediaID: uuidA})
		}
		return results, nil
	})

	opts, err := EnsureUploaded(t.Context(), s, up)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("uploader calls = %d, want 1", calls)
	}
	doc := adfast.ToADF(adfast.FromMarkdown("![a](assets/shot.png)\n", opts...), opts...)
	if !hasMedia(doc.Content, uuidA) {
		t.Error("returned options must resolve the just-uploaded id")
	}

	// Nothing pending anymore: no further uploads on the next call.
	if _, err := EnsureUploaded(t.Context(), s, up); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("idle EnsureUploaded must not upload again, calls = %d", calls)
	}
}
