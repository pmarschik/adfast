package assets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mustPruner returns the removal face of a store, failing when it has
// none.
func mustPruner(t *testing.T, s Store) Pruner {
	t.Helper()
	p, ok := PrunerOf(s)
	if !ok {
		t.Fatal("store has no pruner")
	}
	return p
}

// TestForgetRemovesTheRecordTheBlobAndTheLink pins what one removal
// takes away: after it, nothing in the store names the content and
// nothing on disk holds it.
func TestForgetRemovesTheRecordTheBlobAndTheLink(t *testing.T) {
	dir := t.TempDir()
	s := mustStore(t, dir)
	mustAdd(t, s, "media-1", "shot.png", []byte("picture"))
	rec := recordFor(t, mustCatalog(t, s).Records(), "shot.png")
	friendly := filepath.Join(dir, Dir, "shot.png")

	gone, err := mustPruner(t, s).Forget(rec.Hash)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if gone.Name != "shot.png" || !gone.Record || gone.Size != int64(len("picture")) {
		t.Errorf("Forget reported %+v, want the named record and its bytes", gone)
	}
	if gone.Blob != rec.Blob || gone.Friendly != friendly {
		t.Errorf("Forget removed %q and %q, want %q and %q", gone.Blob, gone.Friendly, rec.Blob, friendly)
	}
	for _, path := range []string{rec.Blob, friendly} {
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("%s is still there", path)
		}
	}
	if recs := mustCatalog(t, s).Records(); len(recs) != 0 {
		t.Errorf("the store still lists %+v", recs)
	}
}

// TestForgetSurvivesReopening pins that the removal reached the index on
// disk, not only the copy in memory.
func TestForgetSurvivesReopening(t *testing.T) {
	dir := t.TempDir()
	s := mustStore(t, dir)
	mustAdd(t, s, "media-1", "shot.png", []byte("picture"))
	mustAdd(t, s, "media-2", "chart.svg", []byte("drawing"))

	if _, err := mustPruner(t, s).Forget(ContentHash([]byte("picture"))); err != nil {
		t.Fatalf("Forget: %v", err)
	}

	recs := mustCatalog(t, mustStore(t, dir)).Records()
	if len(recs) != 1 || recs[0].Name != "chart.svg" {
		t.Fatalf("a reopened store lists %+v, want only chart.svg", recs)
	}
}

// TestForgetOfAnUnknownHashRemovesNothing pins the answer to a hash the
// store never held. A removal that reported success for content nobody
// stored would let a caller believe it had cleaned up something.
func TestForgetOfAnUnknownHashRemovesNothing(t *testing.T) {
	s := mustStore(t, t.TempDir())
	mustAdd(t, s, "media-1", "shot.png", []byte("picture"))

	for _, hash := range []string{ContentHash([]byte("never stored")), "not-a-hash", ""} {
		if _, err := mustPruner(t, s).Forget(hash); !errors.Is(err, ErrNoAsset) {
			t.Errorf("Forget(%q) error = %v, want ErrNoAsset", hash, err)
		}
	}
	if recs := mustCatalog(t, s).Records(); len(recs) != 1 {
		t.Errorf("the store lists %+v, want the one record it had", recs)
	}
}

// TestForgetFinishesARecordWhoseBlobIsGone pins the half-removed state
// the write order can leave behind: the record is still listed, and
// forgetting it is what finishes the job.
func TestForgetFinishesARecordWhoseBlobIsGone(t *testing.T) {
	dir := t.TempDir()
	s := mustStore(t, dir)
	mustAdd(t, s, "media-1", "shot.png", []byte("picture"))
	rec := recordFor(t, mustCatalog(t, s).Records(), "shot.png")
	mustDo(t, os.Remove(rec.Blob))

	gone, err := mustPruner(t, s).Forget(rec.Hash)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if gone.Blob != "" || gone.Size != 0 || !gone.Record {
		t.Errorf("Forget reported %+v, want a record and no bytes", gone)
	}
	if recs := mustCatalog(t, s).Records(); len(recs) != 0 {
		t.Errorf("the store still lists %+v", recs)
	}
}

// TestForgetLeavesAPlainFileAlone pins the one friendly file a removal
// must not touch. A regular file in assets/ is content somebody put
// there by hand — the markdown-first flow the store adopts rather than
// copies — so it is the only copy there is, and the store's decision
// about its own blob says nothing about it.
func TestForgetLeavesAPlainFileAlone(t *testing.T) {
	dir := t.TempDir()
	docs := mustMkdir(t, filepath.Join(dir, Dir))
	plain := filepath.Join(docs, "shot.png")
	mustDo(t, os.WriteFile(plain, []byte("picture"), 0o600))

	s := mustStore(t, dir)
	mustAdd(t, s, "media-1", "shot.png", []byte("picture"))
	rec := recordFor(t, mustCatalog(t, s).Records(), "shot.png")

	gone, err := mustPruner(t, s).Forget(rec.Hash)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if gone.Friendly != "" {
		t.Errorf("Forget removed %q, want the hand-placed file left alone", gone.Friendly)
	}
	if body, err := os.ReadFile(plain); err != nil || string(body) != "picture" {
		t.Errorf("reading %s: %q, err %v", plain, body, err)
	}
}

// TestForgetNamesNoFriendlyFileItCannotSee pins the shared case, and
// with it the limit of what a store can clean up.
//
// A project's friendly files live beside its documents, one folder per
// document, and the store that owns the blobs is not any of them. So a
// removal through the shared store takes the content and reports no
// friendly file — the links in the document folders are left dangling,
// for the caller that knows where the documents are to remove. Saying
// so is the honest half: a report claiming the friendly side was cleaned
// would be wrong exactly where it matters.
func TestForgetNamesNoFriendlyFileItCannotSee(t *testing.T) {
	dir := t.TempDir()
	docs := mustMkdir(t, filepath.Join(dir, "docs"))
	doc := mustSplitStore(t, dir, docs, WithStoreDir(".asset-store"))
	mustAdd(t, doc, "media-1", "shot.png", []byte("picture"))

	shared := mustStoreAt(t, dir, dir, WithStoreDir(".asset-store"))
	rec := recordFor(t, mustCatalog(t, shared).Records(), "shot.png")
	gone, err := mustPruner(t, shared).Forget(rec.Hash)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if gone.Blob != rec.Blob || gone.Friendly != "" {
		t.Errorf("Forget removed %q and friendly %q, want the blob and nothing else", gone.Blob, gone.Friendly)
	}
	if _, err := os.Lstat(filepath.Join(docs, Dir, "shot.png")); err != nil {
		t.Errorf("the document's link is gone, so the report understated what it removed: %v", err)
	}
}
