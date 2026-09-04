package assets

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// reachingStore is a composed store that can READ a folder its worklist
// never offers — the shape an embedder builds when a document may reference
// an assets folder the store deliberately refuses to scan for. Pending stays
// the inner store's, so nothing here widens the worklist; only a path a
// document spells out reaches the second folder.
type reachingStore struct {
	Store
	reach Store
}

func (r reachingStore) Load(path string) ([]byte, error) {
	if content, err := r.Store.Load(path); err == nil {
		return content, nil
	}
	return r.reach.Load(path)
}

func (r reachingStore) Lookup(scope, path string) (string, bool) {
	if id, ok := r.Store.Lookup(scope, path); ok {
		return id, true
	}
	return r.reach.Lookup(scope, path)
}

func (r reachingStore) Associate(scope, mediaID, path string) (convert.MediaAsset, error) {
	if _, err := r.Store.Load(path); err == nil {
		return r.Store.Associate(scope, mediaID, path)
	}
	return r.reach.Associate(scope, mediaID, path)
}

// sidewaysProject lays out a project where a document in docs/ references a
// picture in a sibling's assets folder, and returns the composed store plus
// the document directory.
func sidewaysProject(t *testing.T) (store Store, docDir string) {
	t.Helper()
	root := t.TempDir()
	docDir = mustMkdir(t, filepath.Join(root, "docs"))
	own := mustMkdir(t, filepath.Join(docDir, "assets"))
	side := mustMkdir(t, filepath.Join(root, "drafts", "assets"))
	mustDo(t, os.WriteFile(filepath.Join(own, "one.png"), tinyPNG(t, 1, 1), 0o600))
	mustDo(t, os.WriteFile(filepath.Join(own, "scratch.png"), tinyPNG(t, 2, 2), 0o600))
	mustDo(t, os.WriteFile(filepath.Join(side, "wide.png"), tinyPNG(t, 3, 3), 0o600))
	return reachingStore{
		Store: mustStore(t, docDir),
		reach: mustStoreAt(t, filepath.Join(root, "drafts"), docDir),
	}, docDir
}

// FIX/DEFECT PROOF. A push used to intersect the document's references with
// the store's folder worklist, so a reference into a folder the worklist does
// not list dropped even though the store could read the file. The narrowing
// now starts at the references, so the sideways picture uploads.
//
// The GOOD cases share the document: a scratch file nobody references must
// still not upload, and a reference with no file behind it must not either.
func TestSyncOnEncodeUploadsAReferenceTheStoreReadsButDoesNotList(t *testing.T) {
	store, _ := sidewaysProject(t)

	var calls int
	up := mappedUploader(t, &calls, map[string]string{
		"assets/one.png":            uuidA,
		"../drafts/assets/wide.png": uuidB,
	})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![a](assets/one.png)\n\n![b](../drafts/assets/wide.png)\n\n![c](assets/gone.png)\n",
	})
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	wantMedia(t, docs[0], uuidA)
	wantMedia(t, docs[0], uuidB)
	// The unreferenced file is still the store's business, and still nobody's
	// upload.
	wantPending(t, store, "assets/scratch.png")
}

// FIX/DEFECT PROOF. A picture kept in a subdirectory of the assets folder is
// readable through the store, and the folder worklist never listed it — so the
// old intersection dropped it. The GOOD case is in the same document: the file
// that is not there still uploads nothing.
func TestSyncOnEncodeUploadsAReferenceBelowTheAssetsFolder(t *testing.T) {
	mdDir := t.TempDir()
	nested := mustMkdir(t, filepath.Join(mdDir, "assets", "diagrams"))
	mustDo(t, os.WriteFile(filepath.Join(nested, "flow.png"), tinyPNG(t, 4, 4), 0o600))
	store := mustStore(t, mdDir)

	var calls int
	up := mappedUploader(t, &calls, map[string]string{"assets/diagrams/flow.png": uuidA})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![a](assets/diagrams/flow.png)\n\n![b](assets/gone.png)\n",
	})
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	wantMedia(t, docs[0], uuidA)
}

// CONTRACT PIN. One spelling per file, no duplicates, sorted — so the batch an
// uploader receives does not depend on the order a document names its
// pictures in.
func TestPendingRefsNormalizesDeduplicatesAndSorts(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	for i, name := range []string{"one.png", "two.png"} {
		mustDo(t, os.WriteFile(filepath.Join(dir, name), tinyPNG(t, i+1, i+1), 0o600))
	}
	store := mustStore(t, mdDir)

	got := PendingRefs(store, "", []string{
		"assets/two.png",
		"./assets/one.png",
		"assets/one.png",
		"assets/sub/../one.png",
	})
	if want := []string{"assets/one.png", "assets/two.png"}; !slices.Equal(got, want) {
		t.Errorf("PendingRefs = %v, want %v", got, want)
	}
}

// CONTRACT PIN. A reference the store cannot read is left out rather than
// reported: a missing file, a remote URL, and a path that escapes the assets
// folder are all "no bytes to send", and the escape must stay refused —
// widening the worklist to the document's own references must not widen what
// a document may read.
func TestPendingRefsLeavesOutWhatTheStoreCannotRead(t *testing.T) {
	root := t.TempDir()
	mustDo(t, os.WriteFile(filepath.Join(root, "outside.png"), tinyPNG(t, 5, 5), 0o600))
	mdDir := mustMkdir(t, filepath.Join(root, "docs"))
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "here.png"), tinyPNG(t, 1, 1), 0o600))
	store := mustStore(t, mdDir)

	got := PendingRefs(store, "", []string{
		"assets/gone.png",
		"https://example.invalid/pic.png",
		"data:image/png;base64,AAAA",
		"../outside.png",
		"",
		"assets/here.png",
	})
	if want := []string{"assets/here.png"}; !slices.Equal(got, want) {
		t.Errorf("PendingRefs = %v, want %v", got, want)
	}
}

// CONTRACT PIN. A reference the store already holds a media id for is not
// pending — that is the half PendingRefs shares with Pending, and it is what
// keeps a second push from re-uploading everything.
func TestPendingRefsLeavesOutAReferenceThatAlreadyHasAMediaID(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	for i, name := range []string{"one.png", "two.png"} {
		mustDo(t, os.WriteFile(filepath.Join(dir, name), tinyPNG(t, i+1, i+1), 0o600))
	}
	store := mustStore(t, mdDir)
	mustAssociate(t, store, "", uuidA, "assets/one.png")

	got := PendingRefs(store, "", []string{"assets/one.png", "assets/two.png"})
	if want := []string{"assets/two.png"}; !slices.Equal(got, want) {
		t.Errorf("PendingRefs = %v, want %v", got, want)
	}
}

// CONTRACT PIN. Scope is the container the media ids belong to: an id minted
// for another container does not satisfy this one, so the reference is pending
// again here. A view from ForScope supplies the scope itself, which is why
// generic plumbing may pass "".
func TestPendingRefsIsPerScope(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "one.png"), tinyPNG(t, 1, 1), 0o600))
	store := mustStore(t, mdDir)
	mustAssociate(t, store, "PROJ-1", uuidA, "assets/one.png")

	if got := PendingRefs(store, "PROJ-1", []string{"assets/one.png"}); len(got) != 0 {
		t.Errorf("PendingRefs in the container that holds it = %v, want nothing", got)
	}
	view := ForScope(store, "PROJ-2")
	if got := PendingRefs(view, "", []string{"assets/one.png"}); !slices.Equal(got, []string{"assets/one.png"}) {
		t.Errorf("PendingRefs through a foreign container's view = %v, want the reference", got)
	}
}
