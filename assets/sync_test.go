package assets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeAsset(t *testing.T, mdDir, name string, content []byte) {
	t.Helper()
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSync_BatchedLazyUpload(t *testing.T) {
	mdDir := t.TempDir()
	writeAsset(t, mdDir, "a.png", tinyPNG(t, 1, 1))
	writeAsset(t, mdDir, "b.png", tinyPNG(t, 2, 2))
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}

	var batches [][]PendingAsset
	up := UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		batches = append(batches, batch)
		results := make([]UploadResult, len(batch))
		ids := []string{uuidA, uuidB}
		for i, item := range batch {
			results[i] = UploadResult{Path: item.Path, MediaID: ids[i]}
		}
		return results, nil
	})

	associated, err := Sync(t.Context(), s, up)
	if err != nil {
		t.Fatal(err)
	}
	// Laziness folds everything into ONE batch call.
	if len(batches) != 1 || len(batches[0]) != 2 {
		t.Fatalf("expected one batch of two, got %d batches: %+v", len(batches), batches)
	}
	if batches[0][0].Name != "a.png" || len(batches[0][0].Content) == 0 {
		t.Errorf("batch item incomplete: %+v", batches[0][0])
	}
	if len(associated) != 2 {
		t.Errorf("associated: %+v", associated)
	}
	if pending, pendErr := s.Pending(""); pendErr != nil || len(pending) != 0 {
		t.Errorf("still pending: %v %v", pending, pendErr)
	}

	// Nothing pending → the uploader is not called at all.
	batches = nil
	if _, err := Sync(t.Context(), s, up); err != nil || batches != nil {
		t.Errorf("idle sync must not upload: %v %v", batches, err)
	}
}

func TestSync_PartialBatchKeepsProgress(t *testing.T) {
	mdDir := t.TempDir()
	writeAsset(t, mdDir, "ok.png", tinyPNG(t, 1, 1))
	writeAsset(t, mdDir, "fail.png", tinyPNG(t, 3, 3))
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("quota exceeded")
	up := UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		// Only the first item succeeded before the failure.
		return []UploadResult{{Path: batch[0].Path, MediaID: uuidA}}, wantErr
	})

	associated, err := Sync(t.Context(), s, up)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error not propagated: %v", err)
	}
	if len(associated) != 1 {
		t.Errorf("successful result must associate: %+v", associated)
	}
	pending, pendErr := s.Pending("")
	if pendErr != nil || len(pending) != 1 {
		t.Errorf("failed item must stay pending: %v %v", pending, pendErr)
	}
}

func TestStore_MultipleInstancesShareIndex(t *testing.T) {
	// Two documents (or two subsystems) over the same assets folder each
	// build their own FSStore; associations through one must be visible
	// to — and never clobbered by — the other.
	mdDir := t.TempDir()
	writeAsset(t, mdDir, "one.png", tinyPNG(t, 1, 1))
	writeAsset(t, mdDir, "two.png", tinyPNG(t, 2, 2))

	a, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}

	if _, assocErr := a.Associate("", uuidA, "assets/one.png"); assocErr != nil {
		t.Fatal(assocErr)
	}
	// Instance b was constructed before a's write — its own mutation
	// must merge, not overwrite, the index.
	if _, assocErr := b.Associate("", uuidB, "assets/two.png"); assocErr != nil {
		t.Fatal(assocErr)
	}

	fresh, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fresh.Resolve(uuidA); !ok {
		t.Error("association through instance a was lost")
	}
	if _, ok := fresh.Resolve(uuidB); !ok {
		t.Error("association through instance b was lost")
	}
	// Cross-instance visibility without reconstruction.
	if id, ok := a.Lookup("", "assets/two.png"); !ok || id != uuidB {
		t.Errorf("instance a must see b's association: %q %v", id, ok)
	}
	if pending, pendErr := a.Pending(""); pendErr != nil || len(pending) != 0 {
		t.Errorf("pending after both associations: %v %v", pending, pendErr)
	}
}
