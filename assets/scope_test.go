package assets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestForScope_PerContainerMediaIDs: Jira binds attachments to one
// issue (Confluence to one page), so the same shared file pushed from
// two documents needs TWO media ids — one per container — while the
// local store keeps a single blob and friendly file. Each scoped view
// must miss on the other container's id, re-upload, and encode its own.
func TestForScope_PerContainerMediaIDs(t *testing.T) {
	mdDir := t.TempDir()
	dir := filepath.Join(mdDir, "assets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), tinyPNG(t, 2, 2), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	md := "![logo](assets/logo.png)\n"
	uploader := func(id string) Uploader {
		return UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
			results := make([]UploadResult, 0, len(batch))
			for _, p := range batch {
				results = append(results, UploadResult{Path: p.Path, MediaID: id})
			}
			return results, nil
		})
	}

	// Issue A pushes: upload happens, doc carries A's id.
	viewA := ForScope(store, "PROJ-1")
	docsA, err := PushPipeline(t.Context(), viewA, uploader(uuidA)).MarkdownToADFAll([]string{md})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMedia(docsA[0].Content, uuidA) {
		t.Error("issue A doc missing its media id")
	}

	// Issue B pushes the SAME file: the content is attached to A only,
	// so B's view re-lists it as pending, uploads again, and encodes
	// B's own id — never A's.
	viewB := ForScope(store, "PROJ-2")
	pending, err := viewB.Pending("")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0] != "assets/logo.png" {
		t.Fatalf("issue B pending = %v, want the shared file again", pending)
	}
	docsB, err := PushPipeline(t.Context(), viewB, uploader(uuidB)).MarkdownToADFAll([]string{md})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMedia(docsB[0].Content, uuidB) {
		t.Error("issue B doc missing its media id")
	}
	if hasMedia(docsB[0].Content, uuidA) {
		t.Error("issue B doc leaked issue A's media id")
	}

	// Scoped lookups stay separated; storage stayed deduplicated (one
	// blob, one friendly file, no -2 suffix).
	if id, ok := viewA.Lookup("", "assets/logo.png"); !ok || id != uuidA {
		t.Errorf("view A lookup: %q %v", id, ok)
	}
	if id, ok := viewB.Lookup("", "assets/logo.png"); !ok || id != uuidB {
		t.Errorf("view B lookup: %q %v", id, ok)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var friendly []string
	for _, e := range entries {
		if e.Name() != ".store" {
			friendly = append(friendly, e.Name())
		}
	}
	if len(friendly) != 1 || friendly[0] != "logo.png" {
		t.Errorf("friendly files = %v, want single deduplicated logo.png", friendly)
	}

	// Both scopes idle now: no further uploads.
	for _, view := range []Store{viewA, viewB} {
		if pending, pErr := view.Pending(""); pErr != nil || len(pending) != 0 {
			t.Errorf("pending after sync = %v (%v)", pending, pErr)
		}
	}
}

// TestForScope_LegacyUnscopedEntriesMatch: pre-scope index records
// (scope "") satisfy any scope — pulled assets keep working after an
// upgrade — but an exactly-scoped id wins over a legacy one.
func TestForScope_LegacyUnscopedEntriesMatch(t *testing.T) {
	mdDir := t.TempDir()
	store, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("", uuidA, "shot.png", tinyPNG(t, 2, 2)); err != nil { // unscoped legacy
		t.Fatal(err)
	}
	view := ForScope(store, "PROJ-9")
	if id, ok := view.Lookup("", "assets/shot.png"); !ok || id != uuidA {
		t.Errorf("legacy match: %q %v", id, ok)
	}
	if pending, pErr := view.Pending(""); pErr != nil || len(pending) != 0 {
		t.Errorf("legacy-satisfied file listed pending: %v (%v)", pending, pErr)
	}
	// A scoped id for the same content takes precedence in its scope.
	if _, err := view.Associate("PROJ-9", uuidB, "assets/shot.png"); err != nil {
		t.Fatal(err)
	}
	if id, ok := view.Lookup("", "assets/shot.png"); !ok || id != uuidB {
		t.Errorf("scoped precedence: %q %v, want %s", id, ok, uuidB)
	}
}

// TestForScope_AddAndResolveThroughView: Add through a scoped view
// records the bound scope (overriding any scope argument), Resolve
// works from every view (ids are globally unique), and lookups stay
// isolated per container.
func TestForScope_AddAndResolveThroughView(t *testing.T) {
	mdDir := t.TempDir()
	store, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	viewA := ForScope(store, "PROJ-1")
	viewB := ForScope(store, "PROJ-2")

	// The caller-supplied scope ("" here) is overridden by the bound one.
	asset, err := viewA.Add("", uuidA, "shot.png", tinyPNG(t, 2, 2))
	if err != nil {
		t.Fatal(err)
	}
	if asset.Path != "assets/shot.png" {
		t.Errorf("add through view: %+v", asset)
	}
	// Resolve is unscoped — both views (and the raw store) resolve it.
	for _, s := range []Store{viewA, viewB, store} {
		if got, ok := s.Resolve(uuidA); !ok || got.Path != "assets/shot.png" {
			t.Errorf("resolve: %+v %v", got, ok)
		}
	}
	// Lookup isolation: A sees its id, B misses (content only attached
	// in PROJ-1). Even an explicit foreign scope argument is overridden.
	if id, ok := viewA.Lookup("", "assets/shot.png"); !ok || id != uuidA {
		t.Errorf("view A lookup: %q %v", id, ok)
	}
	if _, ok := viewB.Lookup("", "assets/shot.png"); ok {
		t.Error("view B must not see PROJ-1's id")
	}
	if _, ok := viewB.Lookup("PROJ-1", "assets/shot.png"); ok {
		t.Error("bound scope must override the scope argument")
	}
	// The record landed under PROJ-1: the unscoped store confirms.
	if id, ok := store.Lookup("PROJ-1", "assets/shot.png"); !ok || id != uuidA {
		t.Errorf("store lookup in PROJ-1: %q %v", id, ok)
	}
}
