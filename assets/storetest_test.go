package assets

import (
	"context"
	"os"
	"slices"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// The store tests all make the same two kinds of move: a setup step that
// must not fail, and an assertion that pins one store answer. Spelled
// out inline, each of them costs three or four lines of error plumbing,
// and a test body reads as a chain of `if err != nil` rather than as the
// sequence of facts it pins. The helpers below carry that plumbing so
// the tests carry only the behavior.
//
// Every helper is a t.Helper(), so a failure still points at the line in
// the test that asserted it.

// mustDo fails the test when err is non-nil. (A generic must(t, v, err)
// would be shorter still, but Go only forwards a multi-valued call into
// a single-argument call, so the store constructors and Add/Associate
// get one wrapper each below.)
func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// mustStore opens an FSStore over a markdown directory.
func mustStore(t *testing.T, mdDir string) *FSStore {
	t.Helper()
	s, err := NewFSStore(mdDir)
	mustDo(t, err)
	return s
}

// mustStoreAt opens an FSStore whose assets folder sits under
// assetsParent while the document lives in docDir.
func mustStoreAt(t *testing.T, assetsParent, docDir string, opts ...Option) *FSStore {
	t.Helper()
	s, err := NewFSStoreAt(assetsParent, docDir, opts...)
	mustDo(t, err)
	return s
}

// mustSplitStore opens an FSStore in the split layout: shared blobs
// under blobParent, the friendly view next to docDir's documents.
func mustSplitStore(t *testing.T, blobParent, docDir string, opts ...Option) *FSStore {
	t.Helper()
	s, err := NewFSStoreSplit(blobParent, docDir, opts...)
	mustDo(t, err)
	return s
}

// mustAdd stores content under a media id and returns the render asset.
// The scope argument is always "": a scoped add goes through a ForScope
// view, which overrides it with the bound scope anyway.
func mustAdd(t *testing.T, s Store, mediaID, suggestedName string, content []byte) convert.MediaAsset {
	t.Helper()
	asset, err := s.Add("", mediaID, suggestedName, content)
	mustDo(t, err)
	return asset
}

// mustAssociate binds an existing assets file to a media id and returns
// the render asset.
func mustAssociate(t *testing.T, s Store, scope, mediaID, path string) convert.MediaAsset {
	t.Helper()
	asset, err := s.Associate(scope, mediaID, path)
	mustDo(t, err)
	return asset
}

// mustPushAll runs a push pipeline over the markdown documents.
func mustPushAll(t *testing.T, p *adfast.Pipeline, mds []string) []adf.Doc {
	t.Helper()
	docs, err := p.MarkdownToADFAll(mds)
	mustDo(t, err)
	return docs
}

// mustMkdir creates dir with its parents and returns it.
func mustMkdir(t *testing.T, dir string) string {
	t.Helper()
	mustDo(t, os.MkdirAll(dir, 0o750))
	return dir
}

// mustResolve returns the render asset for a media id, failing when the
// store does not know it.
func mustResolve(t *testing.T, s Store, mediaID string) convert.MediaAsset {
	t.Helper()
	asset, ok := s.Resolve(mediaID)
	if !ok {
		t.Fatalf("media id %s does not resolve", mediaID)
	}
	return asset
}

// wantPath pins a render asset's markdown-relative reference path.
func wantPath(t *testing.T, got convert.MediaAsset, path string) {
	t.Helper()
	if got.Path != path {
		t.Errorf("reference path = %q, want %q", got.Path, path)
	}
}

// wantAsset pins a render asset's reference path and pixel dimensions.
func wantAsset(t *testing.T, got convert.MediaAsset, path string, w, h int) {
	t.Helper()
	wantPath(t, got, path)
	if got.Width != w || got.Height != h {
		t.Errorf("asset %q = %dx%d, want %dx%d", got.Path, got.Width, got.Height, w, h)
	}
}

// wantResolve pins the reference path a media id resolves to.
func wantResolve(t *testing.T, s Store, mediaID, path string) {
	t.Helper()
	wantPath(t, mustResolve(t, s, mediaID), path)
}

// wantLookup pins the media id a reference path maps back to.
func wantLookup(t *testing.T, s Store, scope, path, wantID string) {
	t.Helper()
	wantLookupAny(t, s, scope, path, wantID)
}

// wantLookupAny pins that a reference path maps back to one of wantIDs —
// for content that several media ids share.
func wantLookupAny(t *testing.T, s Store, scope, path string, wantIDs ...string) {
	t.Helper()
	id, ok := s.Lookup(scope, path)
	if !ok {
		t.Errorf("lookup %q in scope %q: no media id, want one of %v", path, scope, wantIDs)
		return
	}
	if !slices.Contains(wantIDs, id) {
		t.Errorf("lookup %q in scope %q = %q, want one of %v", path, scope, id, wantIDs)
	}
}

// wantNoLookup pins that a reference path maps to no media id.
func wantNoLookup(t *testing.T, s Store, scope, path string) {
	t.Helper()
	if id, ok := s.Lookup(scope, path); ok {
		t.Errorf("lookup %q in scope %q resolved to %q, want a miss", path, scope, id)
	}
}

// wantDims pins the intrinsic dimensions a reference path reports.
func wantDims(t *testing.T, s Store, path string, w, h int) {
	t.Helper()
	gotW, gotH, ok := s.Dims(path)
	if !ok {
		t.Errorf("dims %q: not probed, want %dx%d", path, w, h)
		return
	}
	if gotW != w || gotH != h {
		t.Errorf("dims %q = %dx%d, want %dx%d", path, gotW, gotH, w, h)
	}
}

// wantNoDims pins that a reference path reports no dimensions.
func wantNoDims(t *testing.T, s Store, path string) {
	t.Helper()
	if w, h, ok := s.Dims(path); ok {
		t.Errorf("dims %q = %dx%d, want a refusal", path, w, h)
	}
}

// wantLoad pins that a reference path loads non-empty content.
func wantLoad(t *testing.T, s Store, path string) {
	t.Helper()
	content, err := s.Load(path)
	if err != nil || len(content) == 0 {
		t.Errorf("load %q: %d bytes, err %v", path, len(content), err)
	}
}

// wantLoadRefused pins that a reference path does not load at all.
func wantLoadRefused(t *testing.T, s Store, path string) {
	t.Helper()
	if _, err := s.Load(path); err == nil {
		t.Errorf("load %q succeeded, want a refusal", path)
	}
}

// wantPending pins the store's upload worklist, in order. The scope is
// always "" — a per-container worklist is asked of a ForScope view,
// which supplies the bound scope itself.
func wantPending(t *testing.T, s Store, want ...string) {
	t.Helper()
	pending, err := s.Pending("")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !slices.Equal(pending, want) {
		t.Errorf("pending = %v, want %v", pending, want)
	}
}

// wantNothingToUpload pins that a Sync over the store hands the uploader
// no work — nothing on disk is upload-worthy.
func wantNothingToUpload(t *testing.T, s Store) {
	t.Helper()
	uploaded := false
	_, err := Sync(t.Context(), s, UploaderFunc(
		func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
			uploaded = uploaded || len(batch) > 0
			return nil, nil
		},
	))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if uploaded {
		t.Error("sync handed the uploader work, want nothing to upload")
	}
}

// wantSymlink pins that a physical path is a symlink.
func wantSymlink(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Errorf("lstat %s: %v", path, err)
		return
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%s is not a symlink", path)
	}
}

// wantExists pins that a physical path exists (following no symlink).
func wantExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

// wantSingleBlob pins that a blob store holds one blob and nothing but
// its index beside it — the shape that says content was not duplicated.
func wantSingleBlob(t *testing.T, dir string) {
	t.Helper()
	const want = 2 // the blob + index.json
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) != want {
		t.Errorf("%s holds %d entries, want %d", dir, len(entries), want)
	}
}

// wantFriendlyFiles pins the friendly (user-visible) file names in an
// assets folder, ignoring the hidden blob store.
func wantFriendlyFiles(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var friendly []string
	for _, e := range entries {
		if e.Name() != ".store" {
			friendly = append(friendly, e.Name())
		}
	}
	if !slices.Equal(friendly, want) {
		t.Errorf("friendly files = %v, want %v", friendly, want)
	}
}

// wantMedia pins that a document carries a media node for a media id.
func wantMedia(t *testing.T, doc adf.Doc, mediaID string) {
	t.Helper()
	if !hasMedia(doc.Content, mediaID) {
		t.Errorf("document has no media node for %s", mediaID)
	}
}

// wantNoMedia pins that a document carries no media node for a media id
// — the leak check for per-container ids.
func wantNoMedia(t *testing.T, doc adf.Doc, mediaID string) {
	t.Helper()
	if hasMedia(doc.Content, mediaID) {
		t.Errorf("document leaked a media node for %s", mediaID)
	}
}

// constantUploader answers every batch with the same media id — the
// stand-in for a product that binds one attachment per container.
func constantUploader(mediaID string) Uploader {
	return UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		results := make([]UploadResult, 0, len(batch))
		for _, p := range batch {
			results = append(results, UploadResult{Path: p.Path, MediaID: mediaID})
		}
		return results, nil
	})
}

// mappedUploader answers each pending path with the media id ids gives
// it, counts its calls in calls, and fails the test on a path it was not
// meant to see.
func mappedUploader(t *testing.T, calls *int, ids map[string]string) Uploader {
	t.Helper()
	return UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		*calls++
		results := make([]UploadResult, 0, len(batch))
		for _, p := range batch {
			id, ok := ids[p.Path]
			if !ok {
				t.Errorf("unexpected upload of %s", p.Path)
				continue
			}
			results = append(results, UploadResult{Path: p.Path, MediaID: id})
		}
		return results, nil
	})
}
