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
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	content := tinyPNG(t, 2, 2)
	for _, name := range []string{"a.png", "b.png"} {
		mustDo(t, os.WriteFile(filepath.Join(dir, name), content, 0o600))
	}
	store := mustStore(t, mdDir)
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
		wantLookup(t, store, "", path, uuidA)
	}
	wantPending(t, store)
}

// TestFSStoreSplit_SharedTruthPerDocView: the TRUE store (blobs +
// index) is shared under the project root while each document folder
// keeps its own friendly view with local reference paths. A view that
// never saw the asset materializes the friendly file from the blobs on
// Resolve.
func TestFSStoreSplit_SharedTruthPerDocView(t *testing.T) {
	root := t.TempDir()
	docA := mustMkdir(t, filepath.Join(root, "docs", "a"))
	docB := mustMkdir(t, filepath.Join(root, "docs", "b"))
	viewA := mustSplitStore(t, root, docA)
	viewB := mustSplitStore(t, root, docB)
	blobDir := filepath.Join(root, "assets", ".store")

	// Download through view A: blob lands in the shared store, the
	// friendly file next to A's documents, reference path doc-local.
	wantPath(t, mustAdd(t, viewA, uuidA, "shot.png", tinyPNG(t, 3, 4)), "assets/shot.png")
	wantEntryCount(t, blobDir, 2) // blob + index.json
	wantExists(t, filepath.Join(docA, "assets", "shot.png"))

	// View B resolves the same id through the shared index and
	// materializes its own friendly file.
	wantAsset(t, mustResolve(t, viewB, uuidA), "assets/shot.png", 3, 4)
	wantLookup(t, viewB, "", "assets/shot.png", uuidA)

	// Identical content added through view B deduplicates in the
	// shared blob store.
	mustAdd(t, viewB, uuidB, "copy.png", tinyPNG(t, 3, 4))
	wantEntryCount(t, blobDir, 2)
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
