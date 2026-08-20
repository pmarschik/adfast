package assets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// TestSync_DeduplicatesByContent: two friendly files with identical
// bytes upload ONCE; both paths resolve to the same media id afterwards
// because Lookup is content-addressed.
func TestSync_DeduplicatesByContent(t *testing.T) {
	mdDir := t.TempDir()
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := tinyPNG(t, 2, 2)
	for _, name := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	var uploaded []string
	if _, syncErr := Sync(t.Context(), store, UploaderFunc(
		func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
			results := make([]UploadResult, 0, len(batch))
			for _, p := range batch {
				uploaded = append(uploaded, p.Path)
				results = append(results, UploadResult{Path: p.Path, MediaID: uuidA})
			}
			return results, nil
		},
	)); syncErr != nil {
		t.Fatal(syncErr)
	}
	if len(uploaded) != 1 {
		t.Fatalf("uploaded = %v, want one item for identical content", uploaded)
	}
	for _, path := range []string{"assets/a.png", "assets/b.png"} {
		if id, ok := store.Lookup("", path); !ok || id != uuidA {
			t.Errorf("%s → %q %v, want %s", path, id, ok, uuidA)
		}
	}
	pending, err := store.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("pending after dedup sync = %v", pending)
	}
}

// TestFSStoreSplit_SharedTruthPerDocView: the TRUE store (blobs +
// index) is shared under the project root while each document folder
// keeps its own friendly view with local reference paths. A view that
// never saw the asset materializes the friendly file from the blobs on
// Resolve.
func TestFSStoreSplit_SharedTruthPerDocView(t *testing.T) {
	root := t.TempDir()
	docA := filepath.Join(root, "docs", "a")
	docB := filepath.Join(root, "docs", "b")
	for _, d := range []string{docA, docB} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	viewA, err := NewFSStoreSplit(root, docA)
	if err != nil {
		t.Fatal(err)
	}
	viewB, err := NewFSStoreSplit(root, docB)
	if err != nil {
		t.Fatal(err)
	}

	// Download through view A: blob lands in the shared store, the
	// friendly file next to A's documents, reference path doc-local.
	asset, addErr := viewA.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4))
	if addErr != nil {
		t.Fatal(addErr)
	}
	if asset.Path != "assets/shot.png" {
		t.Errorf("view A path: %q", asset.Path)
	}
	blobs, dirErr := os.ReadDir(filepath.Join(root, "assets", ".store"))
	if dirErr != nil || len(blobs) != 2 { // blob + index.json
		t.Fatalf("shared blob dir: %v entries, err %v", len(blobs), dirErr)
	}
	if _, err := os.Lstat(filepath.Join(docA, "assets", "shot.png")); err != nil {
		t.Errorf("friendly file missing in view A: %v", err)
	}

	// View B resolves the same id through the shared index and
	// materializes its own friendly file.
	assetB, ok := viewB.Resolve(uuidA)
	if !ok || assetB.Path != "assets/shot.png" {
		t.Fatalf("view B resolve: %+v %v", assetB, ok)
	}
	if assetB.Width != 3 || assetB.Height != 4 {
		t.Errorf("view B dims: %dx%d", assetB.Width, assetB.Height)
	}
	if id, ok := viewB.Lookup("", "assets/shot.png"); !ok || id != uuidA {
		t.Errorf("view B lookup after materialize: %q %v", id, ok)
	}

	// Identical content added through view B deduplicates in the
	// shared blob store.
	if _, dupErr := viewB.Add("", uuidB, "copy.png", tinyPNG(t, 3, 4)); dupErr != nil {
		t.Fatal(dupErr)
	}
	blobs, dirErr = os.ReadDir(filepath.Join(root, "assets", ".store"))
	if dirErr != nil || len(blobs) != 2 {
		t.Errorf("blob store after duplicate add: %v entries, err %v", len(blobs), dirErr)
	}
}

// TestFormat_RewritesReferencesOnLayoutChange: re-rendering markdown
// with a store under a NEW layout rewrites the image destinations to
// the store's current reference paths — the formatter pipeline with the
// store wired is the markdown-rewrite facility.
func TestFormat_RewritesReferencesOnLayoutChange(t *testing.T) {
	root := t.TempDir()
	docDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docDir, 0o750); err != nil {
		t.Fatal(err)
	}
	local, err := NewFSStore(docDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := local.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	md := "# Title\n\nBefore text.\n\n![screen](assets/shot.png)\n\nAfter text.\n"

	// Same layout: the rewrite is a no-op.
	same := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(local, local)),
		adfast.WithPrettierFormat(), RewriteReferences(local, local),
	)
	if same != md {
		t.Errorf("same-layout format changed the document:\n%s", same)
	}

	// The assets folder physically moves to the project root (index
	// travels inside .store/): formatting with the relocated store
	// rewrites the reference while leaving everything else untouched.
	if mvErr := os.Rename(filepath.Join(docDir, "assets"), filepath.Join(root, "assets")); mvErr != nil {
		t.Fatal(mvErr)
	}
	shared, err := NewFSStoreAt(root, docDir)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(nil, shared)),
		adfast.WithPrettierFormat(), RewriteReferences(nil, shared),
	)
	want := "# Title\n\nBefore text.\n\n![screen](../assets/shot.png)\n\nAfter text.\n"
	if rewritten != want {
		t.Errorf("rewritten:\n%q\nwant:\n%q", rewritten, want)
	}
}

// TestWithStoreDir_PlacesBlobsInCustomTopLevelDir: the content-addressed
// blobs + index land in the configured store dir (a dedicated top-level
// ".asset-store") instead of the default hidden assets/.store, while the
// friendly symlink still lives in the doc's assets/ folder.
func TestWithStoreDir_PlacesBlobsInCustomTopLevelDir(t *testing.T) {
	root := t.TempDir()
	docDir := filepath.Join(root, "issues", "EPIC-1")
	if err := os.MkdirAll(docDir, 0o750); err != nil {
		t.Fatal(err)
	}
	store, err := NewFSStoreSplit(root, docDir, WithStoreDir(".asset-store"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("", "media-1", "shot.png", tinyPNG(t, 2, 2)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Blob + index live in <root>/.asset-store, NOT <root>/assets/.store.
	entries, globErr := filepath.Glob(filepath.Join(root, ".asset-store", "*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(entries) == 0 {
		t.Errorf("expected blobs under %s/.asset-store", root)
	}
	if _, err := os.Stat(filepath.Join(root, "assets", ".store")); !os.IsNotExist(err) {
		t.Errorf("did not expect the default assets/.store dir, stat err=%v", err)
	}
	// Friendly symlink still next to the doc.
	link := filepath.Join(docDir, "assets", "shot.png")
	if fi, lerr := os.Lstat(link); lerr != nil {
		t.Errorf("expected friendly file at %s: %v", link, lerr)
	} else if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink into the store", link)
	}
	// And it resolves back to the blob content.
	if b, rerr := store.Load("assets/shot.png"); rerr != nil || len(b) == 0 {
		t.Errorf("resolve via store failed: err=%v len=%d", rerr, len(b))
	}
}
