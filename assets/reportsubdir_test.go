package assets

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReportRefusesAReferenceIntoAnAssetsSubdirectory: report mode
// answers a path-keyed read for a friendly file the folder no longer
// holds by reaching for the record carrying that NAME. The name alone is
// not the reference: assets/sub/shot.png is a different file from
// assets/shot.png, and the repairing twin says so — it declines rather
// than create a file at a path no caller named. Report mode used to
// answer anyway, which made it strictly MORE permissive than the mode it
// exists to mirror, and every consumer keyed on that difference read a
// dead link as live: it vanished from the broken-reference report, and
// prune counted the top-level blob as still referenced and never
// reclaimed it.
//
// The top-level reference is asserted in the same scope, because a
// containment check that simply refused everything would satisfy the
// refusals above while destroying the whole point of the mode.
func TestReportRefusesAReferenceIntoAnAssetsSubdirectory(t *testing.T) {
	store, friendly := cleanedDoc(t)
	assetsDir := filepath.Dir(friendly)
	mustMkdir(t, filepath.Join(assetsDir, "sub"))

	const deep = "assets/sub/shot.png"
	store.Report(func() {
		wantNoLookup(t, store, "", deep)
		wantNoDims(t, store, deep)
		wantLoadRefused(t, store, deep)
		if hash, ok := store.HashOf(deep); ok {
			t.Errorf("HashOf(%q) = %q while reporting, want a refusal", deep, hash)
		}

		// GOOD case: the reference the record actually stands for still
		// reads from the blob, which is what the mode is for.
		wantLookup(t, store, "", "assets/shot.png", uuidA)
		wantDims(t, store, "assets/shot.png", cleanedW, cleanedH)
		wantLoad(t, store, "assets/shot.png")
	})
	wantAbsent(t, friendly)

	// Outside the scope the repairing twin says the same thing about both
	// references — that equality is the claim under test — and it invents
	// no file in the subdirectory.
	wantNoLookup(t, store, "", deep)
	wantNoDims(t, store, deep)
	wantLoadRefused(t, store, deep)
	if hash, ok := store.HashOf(deep); ok {
		t.Errorf("HashOf(%q) = %q outside a report, want a refusal", deep, hash)
	}
	wantAbsent(t, filepath.Join(assetsDir, "sub", "shot.png"))

	wantLookup(t, store, "", "assets/shot.png", uuidA)
	wantDims(t, store, "assets/shot.png", cleanedW, cleanedH)
	wantLoad(t, store, "assets/shot.png")
	wantExists(t, friendly)
}

// TestReportReadsACleanedFileThroughAParentRelativeRefPrefix: a
// project-root store reaches its assets folder through ../ segments, so
// every reference it hands out is a path that leaves the document
// directory. A containment check written against the fused layout alone
// would reject those and silently disable report mode for the whole
// shared-folder placement, so the ordinary read is pinned here in that
// layout — with the subdirectory refusal beside it, so the pin cannot be
// met by a store that answers everything either.
func TestReportReadsACleanedFileThroughAParentRelativeRefPrefix(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs", "deep", "nested"))
	store := mustStoreAt(t, root, docDir)
	asset := mustAdd(t, store, uuidA, "shot.png", tinyPNG(t, cleanedW, cleanedH))
	wantPath(t, asset, "../../../assets/shot.png")
	friendly := filepath.Join(root, Dir, "shot.png")
	mustDo(t, os.Remove(friendly))
	mustMkdir(t, filepath.Join(root, Dir, "sub"))

	store.Report(func() {
		wantLookup(t, store, "", asset.Path, uuidA)
		wantDims(t, store, asset.Path, cleanedW, cleanedH)
		wantLoad(t, store, asset.Path)

		const deep = "../../../assets/sub/shot.png"
		wantNoLookup(t, store, "", deep)
		wantNoDims(t, store, deep)
		wantLoadRefused(t, store, deep)
	})
	wantAbsent(t, friendly)
}
