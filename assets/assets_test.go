package assets

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const (
	uuidA = "b5773183-5f9a-481f-b1b8-8fe286bba8e9"
	uuidB = "0a1b2c3d-1111-2222-3333-444455556666"
	uuidC = "9f8e7d6c-aaaa-bbbb-cccc-ddddeeeeffff"
)

func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStore_AddResolveLookup(t *testing.T) {
	mdDir := t.TempDir()
	s := mustStore(t, mdDir)
	content := tinyPNG(t, 12, 7)

	asset := mustAdd(t, s, uuidA, "we ird/../shot.png", content)
	wantAsset(t, asset, "assets/shot.png", 12, 7)
	// The friendly file is a symlink into the hidden store.
	wantSymlink(t, filepath.Join(mdDir, "assets", "shot.png"))
	// Idempotent re-add.
	wantPath(t, mustAdd(t, s, uuidA, "shot.png", content), asset.Path)
	// Identical content under a second media id reuses the friendly file.
	wantPath(t, mustAdd(t, s, uuidB, "other.png", content), "assets/shot.png")

	// A fresh store instance reads the persisted index.
	s2 := mustStore(t, mdDir)
	wantResolve(t, s2, uuidA, "assets/shot.png")
	wantLookupAny(t, s2, "", "assets/shot.png", uuidA, uuidB)
	wantNoLookup(t, s2, "", "../outside.png")
	if assets := s2.Assets(); len(assets) != 2 {
		t.Errorf("assets: %v", assets)
	}
}

func TestStore_NameConflictSuffix(t *testing.T) {
	s := mustStore(t, t.TempDir())
	mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 1, 1))
	// Different content, same suggested name → suffixed friendly name.
	wantPath(t, mustAdd(t, s, uuidB, "shot.png", tinyPNG(t, 2, 2)), "assets/shot-2.png")
}

func TestStore_LookupSurvivesRename(t *testing.T) {
	mdDir := t.TempDir()
	s := mustStore(t, mdDir)
	mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 1, 1))
	// User renames the friendly file; content-based lookup still maps it.
	dir := filepath.Join(mdDir, "assets")
	mustDo(t, os.Rename(filepath.Join(dir, "shot.png"), filepath.Join(dir, "renamed.png")))
	wantLookup(t, s, "", "assets/renamed.png", uuidA)
}

func TestStoreDims(t *testing.T) {
	s := mustStore(t, t.TempDir())
	mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 3, 4))
	wantDims(t, s, "assets/shot.png", 3, 4)
	wantNoDims(t, s, "../outside.png")
}

func TestStoreAtProjectRoot(t *testing.T) {
	// Project-root placement: the assets folder lives at the repo root
	// (found via an anchor entry) while the document sits two levels
	// down — reference paths must climb accordingly and resolve back.
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs", "guides"))
	mustDo(t, os.WriteFile(filepath.Join(root, ".assets-root"), nil, 0o600))
	found, ok := DiscoverRoot(docDir, ".assets-root")
	if !ok || found != root {
		t.Fatalf("DiscoverRoot: %q %v", found, ok)
	}
	s := mustStoreAt(t, found, docDir)
	wantPath(t, mustAdd(t, s, uuidA, "shot.png", tinyPNG(t, 3, 4)), "../../assets/shot.png")
	wantLookup(t, s, "", "../../assets/shot.png", uuidA)
	wantDims(t, s, "../../assets/shot.png", 3, 4)
	// A reference escaping the assets dir must not resolve.
	wantNoLookup(t, s, "", "../../secrets.txt")

	// A sibling document dir shares the same physical store.
	other := mustStoreAt(t, found, mustMkdir(t, filepath.Join(root, "notes")))
	wantResolve(t, other, uuidA, "../assets/shot.png")
}

func TestStore_MarkdownFirstFlow(t *testing.T) {
	// The user adds a file to assets/ and references it in markdown
	// BEFORE any upload: the store has no entry, so lookups must miss
	// (the encode side reports unresolved-asset); after the upload
	// assigns a media id, Add must adopt the existing file instead of
	// duplicating it under a suffixed name.
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "new-shot.png"), tinyPNG(t, 5, 5), 0o600))

	s := mustStore(t, mdDir)
	// An un-uploaded asset must not resolve to a media id.
	wantNoLookup(t, s, "", "assets/new-shot.png")
	// The upload worklist lists the file; symlinked/indexed content
	// must not appear on it.
	wantPending(t, s, "assets/new-shot.png")

	// Upload happened; the id comes back from the server. The existing
	// file must be adopted rather than duplicated.
	wantPath(t, mustAssociate(t, s, "", uuidA, "assets/new-shot.png"), "assets/new-shot.png")
	wantPending(t, s)
	if _, assocErr := s.Associate("", uuidB, "../outside.png"); assocErr == nil {
		t.Error("associate must reject path traversal")
	}
	if _, statErr := os.Lstat(filepath.Join(dir, "new-shot-2.png")); statErr == nil {
		t.Error("adoption must not create a suffixed duplicate")
	}
	wantLookup(t, s, "", "assets/new-shot.png", uuidA)
	wantAsset(t, mustResolve(t, s, uuidA), "assets/new-shot.png", 5, 5)
}
