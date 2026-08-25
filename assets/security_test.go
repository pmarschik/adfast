package assets

import (
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
	secret := filepath.Join(t.TempDir(), "secret.txt")
	mustDo(t, os.WriteFile(secret, []byte("s3cr3t material"), 0o600))
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.Symlink(secret, filepath.Join(dir, "leak.png")))
	s := mustStore(t, mdDir)

	// No read path reaches the planted link's target, and Sync finds
	// nothing to send.
	wantNoLookup(t, s, "", "assets/leak.png")
	wantPending(t, s)
	wantLoadRefused(t, s, "assets/leak.png")
	wantNoDims(t, s, "assets/leak.png")
	if _, err := s.Associate("", uuidA, "assets/leak.png"); err == nil {
		t.Error("Associate must reject a planted symlink")
	}
	wantNothingToUpload(t, s)

	// The store's OWN friendly symlinks (into .store/) keep working.
	mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 2, 2))
	wantSymlink(t, filepath.Join(dir, "shot.png"))
	wantLookup(t, s, "", "assets/shot.png", uuidA)
	wantLoad(t, s, "assets/shot.png")
	wantDims(t, s, "assets/shot.png", 2, 2)
}

// TestSymlinkRead_SplitStoreLinksAllowed: in the split layout the blob
// dir lives OUTSIDE the assets folder — the store's friendly symlinks
// point there and must keep working.
func TestSymlinkRead_SplitStoreLinksAllowed(t *testing.T) {
	root := t.TempDir()
	s := mustSplitStore(t, root, mustMkdir(t, filepath.Join(root, "docs")))
	mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 3, 4))
	wantLookup(t, s, "", "assets/shot.png", uuidA)
	wantLoad(t, s, "assets/shot.png")
	wantDims(t, s, "assets/shot.png", 3, 4)
}

// TestSymlinkWrite_DanglingLinkNotFollowed: a dangling symlink planted
// at the friendly name must not be written through when Resolve tries
// to materialize the file — that would be an arbitrary file write.
func TestSymlinkWrite_DanglingLinkNotFollowed(t *testing.T) {
	mdDir := t.TempDir()
	s := mustStore(t, mdDir)
	mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 2, 2))
	friendly := filepath.Join(mdDir, "assets", "shot.png")
	mustDo(t, os.Remove(friendly))
	victim := filepath.Join(t.TempDir(), "victim.txt")
	mustDo(t, os.Symlink(victim, friendly))

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
	// A store constructed BEFORE the crafted index exists exercises the
	// reload-merge path later.
	early := mustStore(t, mdDir)
	blobDir := mustMkdir(t, filepath.Join(mdDir, "assets", ".store"))
	crafted := `{"media":{` +
		`"` + uuidA + `":{"hash":"0123456789abcdef","name":"../../evil"},` +
		`"` + uuidB + `":{"hash":"NOT-A-HASH","name":"fine.png"},` +
		`"` + uuidC + `":{"hash":"0123456789abcdef","name":"fine.png"}}}`
	mustDo(t, os.WriteFile(filepath.Join(blobDir, "index.json"), []byte(crafted), 0o600))

	s := mustStore(t, mdDir)
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
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	f, err := os.Create(filepath.Join(dir, "huge.bin"))
	mustDo(t, err)
	// Sparse file: size over the cap without writing the bytes.
	mustDo(t, f.Truncate(MaxAssetSize+1))
	mustDo(t, f.Close())

	s := mustStore(t, mdDir)
	// Load refuses it, and Pending skips it (size-checked before any read).
	wantLoadRefused(t, s, "assets/huge.bin")
	wantPending(t, s)
}

// TestSentinelErrors: traversal and absolute-path rejections are
// distinguishable with errors.Is.
func TestSentinelErrors(t *testing.T) {
	s := mustStore(t, t.TempDir())
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
