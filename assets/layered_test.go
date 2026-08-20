package assets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestLayered_LocalOverSharedRoot: the canonical composition — a
// document-local store layered over a project-root one. Reads hit
// whichever layer owns the file, downloads land locally, and uploads
// associate into the owning layer.
// layeredFixture is the canonical composition: a document-local store
// over a shared project-root one.
func layeredFixture(t *testing.T) (store Store, shared *FSStore, sharedAssets string) {
	t.Helper()
	root := t.TempDir()
	docDir := filepath.Join(root, "docs")
	sharedAssets = filepath.Join(root, "assets")
	for _, d := range []string{filepath.Join(docDir, "assets"), sharedAssets} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	local, err := NewFSStore(docDir)
	if err != nil {
		t.Fatal(err)
	}
	shared, err = NewFSStoreAt(root, docDir)
	if err != nil {
		t.Fatal(err)
	}
	return Layered(local, shared), shared, sharedAssets
}

// TestLayered_ReadsRouteToOwningLayer: a shared asset resolves through
// the composition with the root-relative reference path; downloads land
// in the FIRST (document-local) layer.
func TestLayered_ReadsRouteToOwningLayer(t *testing.T) {
	store, shared, _ := layeredFixture(t)
	if _, err := shared.Add("", uuidA, "logo.png", tinyPNG(t, 2, 2)); err != nil {
		t.Fatal(err)
	}
	if asset, ok := store.Resolve(uuidA); !ok || asset.Path != "../assets/logo.png" {
		t.Errorf("shared resolve: %+v %v", asset, ok)
	}
	if id, ok := store.Lookup("", "../assets/logo.png"); !ok || id != uuidA {
		t.Errorf("shared lookup: %q %v", id, ok)
	}
	if w, h, ok := store.Dims("../assets/logo.png"); !ok || w != 2 || h != 2 {
		t.Errorf("shared dims: %d %d %v", w, h, ok)
	}
	if asset, err := store.Add("", uuidB, "shot.png", tinyPNG(t, 3, 3)); err != nil || asset.Path != "assets/shot.png" {
		t.Errorf("add: %+v %v", asset, err)
	}
	all := store.Assets()
	for _, id := range []string{uuidA, uuidB} {
		if _, ok := all[id]; !ok {
			t.Errorf("Assets() missing %s", id)
		}
	}
}

// TestLayered_AssociateLandsInOwningLayer: a markdown-first file in the
// shared folder is pending through the union and associates back into
// the shared layer.
func TestLayered_AssociateLandsInOwningLayer(t *testing.T) {
	store, shared, sharedAssets := layeredFixture(t)
	if err := os.WriteFile(filepath.Join(sharedAssets, "chart.png"), tinyPNG(t, 4, 4), 0o600); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0] != "../assets/chart.png" {
		t.Fatalf("pending = %v", pending)
	}
	if _, err := Sync(t.Context(), store, UploaderFunc(
		func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
			if len(batch) != 1 || batch[0].Path != "../assets/chart.png" {
				t.Errorf("batch = %+v", batch)
			}
			return []UploadResult{{Path: batch[0].Path, MediaID: uuidC}}, nil
		},
	)); err != nil {
		t.Fatal(err)
	}
	if id, ok := shared.Lookup("", "../assets/chart.png"); !ok || id != uuidC {
		t.Errorf("association landed in the wrong layer: %q %v", id, ok)
	}
}
