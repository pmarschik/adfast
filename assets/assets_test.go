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
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	content := tinyPNG(t, 12, 7)

	asset, err := s.Add("", uuidA, "we ird/../shot.png", content)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Path != "assets/shot.png" || asset.Width != 12 || asset.Height != 7 {
		t.Fatalf("asset %+v", asset)
	}
	// The friendly file is a symlink into the hidden store.
	if fi, lstatErr := os.Lstat(filepath.Join(mdDir, "assets", "shot.png")); lstatErr != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink, err=%v", lstatErr)
	}
	// Idempotent re-add.
	if again, addErr := s.Add("", uuidA, "shot.png", content); addErr != nil || again.Path != asset.Path {
		t.Errorf("re-add: %+v %v", again, addErr)
	}
	// Identical content under a second media id reuses the friendly file.
	if b, addErr := s.Add("", uuidB, "other.png", content); addErr != nil || b.Path != "assets/shot.png" {
		t.Errorf("dedup: %+v %v", b, addErr)
	}

	// A fresh store instance reads the persisted index.
	s2, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s2.Resolve(uuidA); !ok || got.Path != "assets/shot.png" {
		t.Errorf("resolve: %+v %v", got, ok)
	}
	if id, ok := s2.Lookup("", "assets/shot.png"); !ok || (id != uuidA && id != uuidB) {
		t.Errorf("lookup: %q %v", id, ok)
	}
	if _, ok := s2.Lookup("", "../outside.png"); ok {
		t.Error("path traversal must not resolve")
	}
	if assets := s2.Assets(); len(assets) != 2 {
		t.Errorf("assets: %v", assets)
	}
}

func TestStore_NameConflictSuffix(t *testing.T) {
	mdDir := t.TempDir()
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := s.Add("", uuidA, "shot.png", tinyPNG(t, 1, 1)); addErr != nil {
		t.Fatal(addErr)
	}
	// Different content, same suggested name → suffixed friendly name.
	b, err := s.Add("", uuidB, "shot.png", tinyPNG(t, 2, 2))
	if err != nil {
		t.Fatal(err)
	}
	if b.Path != "assets/shot-2.png" {
		t.Errorf("suffix: %+v", b)
	}
}

func TestStore_LookupSurvivesRename(t *testing.T) {
	mdDir := t.TempDir()
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("", uuidA, "shot.png", tinyPNG(t, 1, 1)); err != nil {
		t.Fatal(err)
	}
	// User renames the friendly file; content-based lookup still maps it.
	dir := filepath.Join(mdDir, "assets")
	if err := os.Rename(filepath.Join(dir, "shot.png"), filepath.Join(dir, "renamed.png")); err != nil {
		t.Fatal(err)
	}
	if id, ok := s.Lookup("", "assets/renamed.png"); !ok || id != uuidA {
		t.Errorf("lookup after rename: %q %v", id, ok)
	}
}

func TestStoreDims(t *testing.T) {
	mdDir := t.TempDir()
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); err != nil {
		t.Fatal(err)
	}
	if w, h, ok := s.Dims("assets/shot.png"); !ok || w != 3 || h != 4 {
		t.Errorf("dims: %d %d %v", w, h, ok)
	}
	if _, _, ok := s.Dims("../outside.png"); ok {
		t.Error("path traversal must not resolve")
	}
}

func TestStoreAtProjectRoot(t *testing.T) {
	// Project-root placement: the assets folder lives at the repo root
	// (found via an anchor entry) while the document sits two levels
	// down — reference paths must climb accordingly and resolve back.
	root := t.TempDir()
	docDir := filepath.Join(root, "docs", "guides")
	if err := os.MkdirAll(docDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".assets-root"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	found, ok := DiscoverRoot(docDir, ".assets-root")
	if !ok || found != root {
		t.Fatalf("DiscoverRoot: %q %v", found, ok)
	}
	s, err := NewFSStoreAt(found, docDir)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := s.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4))
	if err != nil {
		t.Fatal(err)
	}
	if asset.Path != "../../assets/shot.png" {
		t.Errorf("reference path: %q", asset.Path)
	}
	if id, ok := s.Lookup("", "../../assets/shot.png"); !ok || id != uuidA {
		t.Errorf("lookup: %q %v", id, ok)
	}
	if w, h, ok := s.Dims("../../assets/shot.png"); !ok || w != 3 || h != 4 {
		t.Errorf("dims: %d %d %v", w, h, ok)
	}
	if _, ok := s.Lookup("", "../../secrets.txt"); ok {
		t.Error("reference escaping the assets dir must not resolve")
	}
	// A sibling document dir shares the same physical store.
	otherDir := filepath.Join(root, "notes")
	if mkErr := os.MkdirAll(otherDir, 0o750); mkErr != nil {
		t.Fatal(mkErr)
	}
	other, err := NewFSStoreAt(found, otherDir)
	if err != nil {
		t.Fatal(err)
	}
	if asset, ok := other.Resolve(uuidA); !ok || asset.Path != "../assets/shot.png" {
		t.Errorf("sibling resolve: %+v %v", asset, ok)
	}
}

func TestStore_MarkdownFirstFlow(t *testing.T) {
	// The user adds a file to assets/ and references it in markdown
	// BEFORE any upload: the store has no entry, so lookups must miss
	// (the encode side reports unresolved-asset); after the upload
	// assigns a media id, Add must adopt the existing file instead of
	// duplicating it under a suffixed name.
	mdDir := t.TempDir()
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := tinyPNG(t, 5, 5)
	if err := os.WriteFile(filepath.Join(dir, "new-shot.png"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Lookup("", "assets/new-shot.png"); ok {
		t.Error("un-uploaded asset must not resolve to a media id")
	}

	// The upload worklist lists the file; symlinked/indexed content
	// must not appear on it.
	pending, err := s.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0] != "assets/new-shot.png" {
		t.Errorf("pending: %v", pending)
	}

	// Upload happened; the id comes back from the server.
	asset, err := s.Associate("", uuidA, "assets/new-shot.png")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Path != "assets/new-shot.png" {
		t.Errorf("existing file must be adopted, got %q", asset.Path)
	}
	if again, pendErr := s.Pending(""); pendErr != nil || len(again) != 0 {
		t.Errorf("pending after associate: %v %v", again, pendErr)
	}
	if _, assocErr := s.Associate("", uuidB, "../outside.png"); assocErr == nil {
		t.Error("associate must reject path traversal")
	}
	if _, statErr := os.Lstat(filepath.Join(dir, "new-shot-2.png")); statErr == nil {
		t.Error("adoption must not create a suffixed duplicate")
	}
	if id, ok := s.Lookup("", "assets/new-shot.png"); !ok || id != uuidA {
		t.Errorf("post-upload lookup: %q %v", id, ok)
	}
	if got, ok := s.Resolve(uuidA); !ok || got.Path != "assets/new-shot.png" || got.Width != 5 {
		t.Errorf("resolve: %+v %v", got, ok)
	}
}
