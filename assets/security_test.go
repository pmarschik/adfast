package assets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestSymlinkRead_PlantedLinkRejected: a symlink planted in assets/
// pointing at a file OUTSIDE the store must never be read — not by
// Lookup, not by Pending, not by Load or Dims, and Sync must not upload
// its target's content.
func TestSymlinkRead_PlantedLinkRejected(t *testing.T) {
	mdDir := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s3cr3t material"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "leak.png")); err != nil {
		t.Fatal(err)
	}
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := s.Lookup("", "assets/leak.png"); ok {
		t.Error("Lookup must not read through a planted symlink")
	}
	pending, err := s.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("Pending must exclude planted symlinks, got %v", pending)
	}
	if _, err := s.Load("assets/leak.png"); err == nil {
		t.Error("Load must reject a planted symlink")
	}
	if _, _, ok := s.Dims("assets/leak.png"); ok {
		t.Error("Dims must reject a planted symlink")
	}
	if _, err := s.Associate("", uuidA, "assets/leak.png"); err == nil {
		t.Error("Associate must reject a planted symlink")
	}
	uploaded := false
	if _, syncErr := Sync(t.Context(), s, UploaderFunc(
		func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
			uploaded = uploaded || len(batch) > 0
			return nil, nil
		},
	)); syncErr != nil {
		t.Fatal(syncErr)
	}
	if uploaded {
		t.Error("Sync must not upload a planted symlink's target")
	}

	// The store's OWN friendly symlinks (into .store/) keep working.
	if _, addErr := s.Add("", uuidA, "shot.png", tinyPNG(t, 2, 2)); addErr != nil {
		t.Fatal(addErr)
	}
	if fi, lerr := os.Lstat(filepath.Join(dir, "shot.png")); lerr != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture must be a store symlink, err=%v", lerr)
	}
	if _, ok := s.Lookup("", "assets/shot.png"); !ok {
		t.Error("store-created symlink must stay readable")
	}
	if content, loadErr := s.Load("assets/shot.png"); loadErr != nil || len(content) == 0 {
		t.Errorf("store-created symlink must load: %v", loadErr)
	}
	if w, h, ok := s.Dims("assets/shot.png"); !ok || w != 2 || h != 2 {
		t.Errorf("store-created symlink dims: %d %d %v", w, h, ok)
	}
}

// TestSymlinkRead_SplitStoreLinksAllowed: in the split layout the blob
// dir lives OUTSIDE the assets folder — the store's friendly symlinks
// point there and must keep working.
func TestSymlinkRead_SplitStoreLinksAllowed(t *testing.T) {
	root := t.TempDir()
	docDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docDir, 0o750); err != nil {
		t.Fatal(err)
	}
	s, err := NewFSStoreSplit(root, docDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := s.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	if id, ok := s.Lookup("", "assets/shot.png"); !ok || id != uuidA {
		t.Errorf("split-store symlink lookup: %q %v", id, ok)
	}
	if _, loadErr := s.Load("assets/shot.png"); loadErr != nil {
		t.Errorf("split-store symlink load: %v", loadErr)
	}
	if w, h, ok := s.Dims("assets/shot.png"); !ok || w != 3 || h != 4 {
		t.Errorf("split-store symlink dims: %d %d %v", w, h, ok)
	}
}

// TestSymlinkWrite_DanglingLinkNotFollowed: a dangling symlink planted
// at the friendly name must not be written through when Resolve tries
// to materialize the file — that would be an arbitrary file write.
func TestSymlinkWrite_DanglingLinkNotFollowed(t *testing.T) {
	mdDir := t.TempDir()
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := s.Add("", uuidA, "shot.png", tinyPNG(t, 2, 2)); addErr != nil {
		t.Fatal(addErr)
	}
	friendly := filepath.Join(mdDir, "assets", "shot.png")
	if rmErr := os.Remove(friendly); rmErr != nil {
		t.Fatal(rmErr)
	}
	victim := filepath.Join(t.TempDir(), "victim.txt")
	if linkErr := os.Symlink(victim, friendly); linkErr != nil {
		t.Fatal(linkErr)
	}

	if _, ok := s.Resolve(uuidA); ok {
		t.Error("Resolve must not materialize over a planted symlink")
	}
	if _, statErr := os.Lstat(victim); statErr == nil {
		t.Fatal("materialize wrote through the planted symlink")
	}
}

// TestIndexSanitization_CraftedEntriesIgnored: hand-written index.json
// records with traversal names or malformed hashes are dropped on load
// and on reload-merge.
func TestIndexSanitization_CraftedEntriesIgnored(t *testing.T) {
	mdDir := t.TempDir()
	blobDir := filepath.Join(mdDir, "assets", ".store")
	// A store constructed BEFORE the crafted index exists exercises the
	// reload-merge path later.
	early, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if mkErr := os.MkdirAll(blobDir, 0o750); mkErr != nil {
		t.Fatal(mkErr)
	}
	crafted := `{"media":{` +
		`"` + uuidA + `":{"hash":"0123456789abcdef","name":"../../evil"},` +
		`"` + uuidB + `":{"hash":"NOT-A-HASH","name":"fine.png"},` +
		`"` + uuidC + `":{"hash":"0123456789abcdef","name":"fine.png"}}}`
	if writeErr := os.WriteFile(filepath.Join(blobDir, "index.json"), []byte(crafted), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.media[uuidA]; ok {
		t.Error("traversal name must be dropped on load")
	}
	if _, ok := s.media[uuidB]; ok {
		t.Error("malformed hash must be dropped on load")
	}
	if _, ok := s.media[uuidC]; !ok {
		t.Error("well-formed entry must survive")
	}
	if _, ok := s.Resolve(uuidA); ok {
		t.Error("crafted entry must not resolve")
	}

	// Reload-merge path: the store constructed before the crafted index
	// existed must also drop the bad records when it merges.
	_ = early.Assets()
	if _, ok := early.media[uuidA]; ok {
		t.Error("traversal name must be dropped on reload-merge")
	}
	if _, ok := early.media[uuidB]; ok {
		t.Error("malformed hash must be dropped on reload-merge")
	}
	if _, ok := early.media[uuidC]; !ok {
		t.Error("well-formed entry must survive reload-merge")
	}
}

// TestSizeCap: Load refuses files over MaxAssetSize; Pending skips them
// (size-checked before any read).
func TestSizeCap(t *testing.T) {
	mdDir := t.TempDir()
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	huge := filepath.Join(dir, "huge.bin")
	f, err := os.Create(huge)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse file: size over the cap without writing the bytes.
	if truncErr := f.Truncate(MaxAssetSize + 1); truncErr != nil {
		t.Fatal(truncErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, loadErr := s.Load("assets/huge.bin"); loadErr == nil {
		t.Error("Load must refuse files over MaxAssetSize")
	}
	pending, err := s.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("Pending must skip oversized files, got %v", pending)
	}
}

// TestSentinelErrors: traversal and absolute-path rejections are
// distinguishable with errors.Is.
func TestSentinelErrors(t *testing.T) {
	mdDir := t.TempDir()
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, loadErr := s.Load("../outside.png"); !errors.Is(loadErr, ErrPathEscapes) {
		t.Errorf("escape error = %v, want ErrPathEscapes", loadErr)
	}
	if _, loadErr := s.Load("/etc/passwd"); !errors.Is(loadErr, ErrAbsolutePath) {
		t.Errorf("absolute error = %v, want ErrAbsolutePath", loadErr)
	}
	if _, assocErr := s.Associate("", uuidA, "../outside.png"); !errors.Is(assocErr, ErrPathEscapes) {
		t.Errorf("associate escape error = %v, want ErrPathEscapes", assocErr)
	}
}
