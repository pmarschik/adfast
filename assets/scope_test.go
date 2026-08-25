package assets

import (
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
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	mustDo(t, os.WriteFile(filepath.Join(dir, "logo.png"), tinyPNG(t, 2, 2), 0o600))
	store := mustStore(t, mdDir)
	md := "![logo](assets/logo.png)\n"

	// Issue A pushes: upload happens, doc carries A's id.
	viewA := ForScope(store, "PROJ-1")
	docsA := mustPushAll(t, PushPipeline(t.Context(), viewA, constantUploader(uuidA)), []string{md})
	wantMedia(t, docsA[0], uuidA)

	// Issue B pushes the SAME file: the content is attached to A only,
	// so B's view re-lists it as pending, uploads again, and encodes
	// B's own id — never A's.
	viewB := ForScope(store, "PROJ-2")
	wantPending(t, viewB, "assets/logo.png")
	docsB := mustPushAll(t, PushPipeline(t.Context(), viewB, constantUploader(uuidB)), []string{md})
	wantMedia(t, docsB[0], uuidB)
	wantNoMedia(t, docsB[0], uuidA)

	// Scoped lookups stay separated; storage stayed deduplicated (one
	// blob, one friendly file, no -2 suffix).
	wantLookup(t, viewA, "", "assets/logo.png", uuidA)
	wantLookup(t, viewB, "", "assets/logo.png", uuidB)
	wantFriendlyFiles(t, dir, "logo.png")

	// Both scopes idle now: no further uploads.
	wantPending(t, viewA)
	wantPending(t, viewB)
}

// TestForScope_LegacyUnscopedEntriesMatch: pre-scope index records
// (scope "") satisfy any scope — pulled assets keep working after an
// upgrade — but an exactly-scoped id wins over a legacy one.
func TestForScope_LegacyUnscopedEntriesMatch(t *testing.T) {
	store := mustStore(t, t.TempDir())
	mustAdd(t, store, uuidA, "shot.png", tinyPNG(t, 2, 2)) // unscoped legacy
	view := ForScope(store, "PROJ-9")
	wantLookup(t, view, "", "assets/shot.png", uuidA)
	// A legacy-satisfied file is not pending.
	wantPending(t, view)
	// A scoped id for the same content takes precedence in its scope.
	mustAssociate(t, view, "PROJ-9", uuidB, "assets/shot.png")
	wantLookup(t, view, "", "assets/shot.png", uuidB)
}

// TestForScope_AddAndResolveThroughView: Add through a scoped view
// records the bound scope (overriding any scope argument), Resolve
// works from every view (ids are globally unique), and lookups stay
// isolated per container.
func TestForScope_AddAndResolveThroughView(t *testing.T) {
	store := mustStore(t, t.TempDir())
	viewA := ForScope(store, "PROJ-1")
	viewB := ForScope(store, "PROJ-2")

	// The caller-supplied scope ("" here) is overridden by the bound one.
	wantPath(t, mustAdd(t, viewA, uuidA, "shot.png", tinyPNG(t, 2, 2)), "assets/shot.png")
	// Resolve is unscoped — both views (and the raw store) resolve it.
	for _, s := range []Store{viewA, viewB, store} {
		wantResolve(t, s, uuidA, "assets/shot.png")
	}
	// Lookup isolation: A sees its id, B misses (content only attached
	// in PROJ-1). Even an explicit foreign scope argument is overridden.
	wantLookup(t, viewA, "", "assets/shot.png", uuidA)
	wantNoLookup(t, viewB, "", "assets/shot.png")
	wantNoLookup(t, viewB, "PROJ-1", "assets/shot.png")
	// The record landed under PROJ-1: the unscoped store confirms.
	wantLookup(t, store, "PROJ-1", "assets/shot.png", uuidA)
}
