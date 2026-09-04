package assets

import (
	"os"
	"path/filepath"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/convert"
)

// Report is part of Store, so the minimum Store in catalog_test.go needs
// it too. It lives here, with the rest of the mode, because it is the
// mode that put it there: a bare store keeps no files and has nothing to
// suppress, so running fn is the whole of its report scope.
func (bareStore) Report(fn func()) { fn() }

// cleanedW and cleanedH are the dimensions of the picture cleanedDoc records.
const (
	cleanedW = 3
	cleanedH = 4
)

// cleanedDoc is a document whose asset the store knows and whose assets
// folder no longer holds the friendly file — somebody cleaned it, or the
// document was checked out without it. It returns the store and the
// friendly path that is now absent.
func cleanedDoc(t *testing.T) (store *FSStore, friendly string) {
	t.Helper()
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs", "a"))
	store = mustSplitStore(t, root, docDir)
	// The size is asserted on, not varied: it is what says the answer came
	// from the blob rather than from a default.
	mustAdd(t, store, uuidA, "shot.png", tinyPNG(t, cleanedW, cleanedH))
	friendly = filepath.Join(docDir, "assets", "shot.png")
	mustDo(t, os.Remove(friendly))
	return store, friendly
}

// wantAbsent pins that nothing is at a path — the disk-write check.
func wantAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s exists, want nothing written there (lstat err = %v)", path, err)
	}
}

// reportResolve resolves a media id inside a report scope.
func reportResolve(s Store, mediaID string) (convert.MediaAsset, bool) {
	var asset convert.MediaAsset
	var ok bool
	s.Report(func() { asset, ok = s.Resolve(mediaID) })
	return asset, ok
}

// TestReportResolvesFromTheBlobInsteadOfCreatingTheFriendlyFile: a
// reporting Resolve answers the reference path AND the dimensions a
// repairing one would, and leaves the folder alone. Both halves matter
// on their own: a report that writes changes the tree it reports on, and
// a report that answers "unknown" renders the media as an unresolved
// reference — quiet and wrong.
func TestReportResolvesFromTheBlobInsteadOfCreatingTheFriendlyFile(t *testing.T) {
	store, friendly := cleanedDoc(t)

	asset, ok := reportResolve(store, uuidA)
	if !ok {
		t.Fatal("reporting Resolve does not answer for a recorded media id")
	}
	wantAsset(t, asset, "assets/shot.png", cleanedW, cleanedH)
	wantAbsent(t, friendly)

	// The repair itself is not the bug and stays: outside a report scope
	// the same call still puts the file back, which is what a pull and a
	// format depend on.
	wantAsset(t, mustResolve(t, store, uuidA), "assets/shot.png", cleanedW, cleanedH)
	wantExists(t, friendly)
}

// TestReportEndsWithItsScope: the mode is scoped to fn and nests, so a
// report inside a report does not leave the store read-only afterwards.
func TestReportEndsWithItsScope(t *testing.T) {
	store, friendly := cleanedDoc(t)

	store.Report(func() {
		store.Report(func() {
			if _, ok := store.Resolve(uuidA); !ok {
				t.Error("nested report scope stopped answering")
			}
		})
		if store.reporting() != true {
			t.Error("the inner scope closing ended the outer one")
		}
		wantAbsent(t, friendly)
	})
	if store.reporting() {
		t.Error("the store is still reporting after its scope closed")
	}
	mustResolve(t, store, uuidA)
	wantExists(t, friendly)
}

// TestReportAnswersPathKeyedReadsFromTheBlob: reading is not only by
// media id. Deciding whether a document changed also asks the store
// about the paths the document names — the id behind a reference, its
// dimensions, its bytes — and those go through the friendly file too. In
// a report scope they come from the blob, so a document whose assets
// folder was cleaned still reads as the document it is.
func TestReportAnswersPathKeyedReadsFromTheBlob(t *testing.T) {
	store, friendly := cleanedDoc(t)

	store.Report(func() {
		wantLookup(t, store, "", "assets/shot.png", uuidA)
		wantDims(t, store, "assets/shot.png", cleanedW, cleanedH)
		wantLoad(t, store, "assets/shot.png")
		if _, ok := store.HashOf("assets/shot.png"); !ok {
			t.Error("HashOf found no record for a recorded name")
		}

		// GOOD case: report mode answers for the store's own records, not
		// for any name a document happens to write.
		wantNoLookup(t, store, "", "assets/never.png")
		wantNoDims(t, store, "assets/never.png")
		wantLoadRefused(t, store, "assets/never.png")
	})
	wantAbsent(t, friendly)

	// And outside a report scope the reads stay as they were: a path with
	// no file behind it is a miss, because these reads answer from the
	// file the document names.
	wantNoLookup(t, store, "", "assets/shot.png")
	wantNoDims(t, store, "assets/shot.png")
	wantLoadRefused(t, store, "assets/shot.png")
	wantAbsent(t, friendly)
}

// TestReportRefusesAnAmbiguousFriendlyName: a friendly name is unique
// within one folder, so two folders sharing a blob store can record the
// same name for different content. Asked about that name with no file to
// read, a report cannot know which content the document meant and says
// nothing — a guess reported as fact is worse than a miss. Resolution by
// media id is unaffected: an id names the content.
func TestReportRefusesAnAmbiguousFriendlyName(t *testing.T) {
	root := t.TempDir()
	docA := mustMkdir(t, filepath.Join(root, "docs", "a"))
	docB := mustMkdir(t, filepath.Join(root, "docs", "b"))
	viewA := mustSplitStore(t, root, docA)
	viewB := mustSplitStore(t, root, docB)
	wantPath(t, mustAdd(t, viewA, uuidA, "shot.png", tinyPNG(t, 3, 4)), "assets/shot.png")
	wantPath(t, mustAdd(t, viewB, uuidB, "shot.png", tinyPNG(t, 5, 6)), "assets/shot.png")
	friendly := filepath.Join(docA, "assets", "shot.png")
	mustDo(t, os.Remove(friendly))

	viewA.Report(func() {
		wantNoLookup(t, viewA, "", "assets/shot.png")
		wantNoDims(t, viewA, "assets/shot.png")
		asset, ok := viewA.Resolve(uuidA)
		if !ok {
			t.Fatal("an ambiguous name must not stop resolution by media id")
		}
		wantAsset(t, asset, "assets/shot.png", cleanedW, cleanedH)
	})
	wantAbsent(t, friendly)
}

// TestReportLeavesAnOccupiedFriendlyPathAlone: materialize refuses to
// write over or through whatever already sits at the friendly path, and
// a report says what a repairing run would have — so it refuses the same
// record rather than claiming a path the store would not have made.
func TestReportLeavesAnOccupiedFriendlyPathAlone(t *testing.T) {
	store, friendly := cleanedDoc(t)
	mustDo(t, os.Symlink(filepath.Join(filepath.Dir(friendly), "gone.png"), friendly))

	if asset, ok := reportResolve(store, uuidA); ok {
		t.Errorf("reporting Resolve claimed %q over an occupied path", asset.Path)
	}
	// The same refusal outside the scope, and the planted link survives
	// both: neither run writes through it.
	if _, ok := store.Resolve(uuidA); ok {
		t.Error("Resolve claimed an occupied friendly path")
	}
	wantSymlink(t, friendly)
}

// TestReportForwardsThroughTheWrappers: report mode is only sound if
// EVERY wrapper passes it down — a view or a stack that forgot to would
// write on exactly the path the mode exists to keep read-only, and the
// caller has no way to see it. The composition here is the one an
// embedder builds: a scoped view over a stack of stores.
func TestReportForwardsThroughTheWrappers(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs", "a"))
	inner := mustSplitStore(t, root, docDir)
	mustAdd(t, inner, uuidA, "shot.png", tinyPNG(t, 3, 4))
	friendly := filepath.Join(docDir, "assets", "shot.png")
	mustDo(t, os.Remove(friendly))

	for name, store := range map[string]Store{
		"scoped":                                ForScope(inner, "PROJ-1"),
		"layered":                               Layered(inner),
		"scoped layered":                        ForScope(Layered(inner), "PROJ-1"),
		"layered scoped":                        Layered(ForScope(inner, "PROJ-1")),
		"layered with an unrelated first layer": Layered(bareStore{}, inner),
	} {
		t.Run(name, func(t *testing.T) {
			asset, ok := reportResolve(store, uuidA)
			if !ok {
				t.Fatal("reporting Resolve does not answer through this wrapper")
			}
			wantAsset(t, asset, "assets/shot.png", cleanedW, cleanedH)
			wantAbsent(t, friendly)
		})
	}
}

// TestReportRoundTripSurvivesAMissingFriendlyFile: the shape a
// comparison actually has — local markdown through md→ADF, the remote
// document through ADF→md — over a document whose assets folder no
// longer holds the file. Both halves must place the same node, or the
// two sides differ over a file rather than over content, and nothing
// the user did explains the change. Neither half may write.
func TestReportRoundTripSurvivesAMissingFriendlyFile(t *testing.T) {
	store, friendly := cleanedDoc(t)
	const local = "![one](assets/shot.png)\n"

	parseOpts := ReportMarkdownOptions(store)
	renderOpts := ReportOptions(store)
	doc := adfast.ToADF(adfast.FromMarkdown(local, parseOpts...), parseOpts...)
	if !hasMedia(doc.Content, uuidA) {
		t.Fatal("the local half did not recognize the reference as media")
	}
	got := adfast.ToMarkdown(adfast.FromADF(doc, renderOpts...), renderOpts...)
	if got != local {
		t.Errorf("round trip = %q, want %q", got, local)
	}
	wantAbsent(t, friendly)
}

// TestReportOptionsRenderWhatRenderOptionsRenderWithoutTheRepair: the
// two option sets produce the same markdown for the same document, and
// only one of them touches the folder. That equality is the whole claim
// — a diff or a dry run switches to ReportOptions to stop writing, and
// must not start reporting something else.
func TestReportOptionsRenderWhatRenderOptionsRenderWithoutTheRepair(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs", "a"))
	store := mustSplitStore(t, root, docDir)
	mustAdd(t, store, uuidA, "shot.png", tinyPNG(t, 3, 4))
	mdOpts := MarkdownOptions(store)
	doc := adfast.ToADF(adfast.FromMarkdown("![one](assets/shot.png)\n", mdOpts...), mdOpts...)
	if !hasMedia(doc.Content, uuidA) {
		t.Fatalf("test setup: the document must reference %s", uuidA)
	}
	friendly := filepath.Join(docDir, "assets", "shot.png")
	mustDo(t, os.Remove(friendly))

	reportOpts := ReportOptions(store)
	reported := adfast.ToMarkdown(adfast.FromADF(doc, reportOpts...), reportOpts...)
	wantAbsent(t, friendly)

	renderOpts := RenderOptions(store)
	rendered := adfast.ToMarkdown(adfast.FromADF(doc, renderOpts...), renderOpts...)
	wantExists(t, friendly)

	if reported != rendered {
		t.Errorf("reported = %q, rendered = %q — a report must say the same thing", reported, rendered)
	}
	if want := "![one](assets/shot.png)\n"; reported != want {
		t.Errorf("reported = %q, want %q", reported, want)
	}
}

// TestReportStillWritesWhatWritesAreFor: the mode suppresses the REPAIR
// that a READ performs, not writing as such. Add, Put and Associate keep
// their effect inside a report scope, and that is load-bearing rather
// than incidental: a caller reports over a document and then acts on what
// it found — a restore-missing-assets action, a dry-run leg that still
// records what it downloaded — inside the same scope. A "report means the
// store is read-only" reading of the mode is the plausible tidy-up, and
// it is silent: Add returning a zero asset and no error leaves both this
// package's suite and the consumer's fully green, because nothing else
// here writes while reporting. So the mode's write half is pinned
// explicitly.
//
// Associate is not given its own row on purpose: it reads the reference
// and then delegates to Add, so the row below is its write path too. Put
// is asserted because it reaches the blob store by its own route
// (assets/meta.go), without a media id.
func TestReportStillWritesWhatWritesAreFor(t *testing.T) {
	store, cleaned := cleanedDoc(t)
	assetsDir := filepath.Dir(cleaned)

	const addedW, addedH = 5, 6
	var added, put convert.MediaAsset
	store.Report(func() {
		added = mustAdd(t, store, uuidB, "added.png", tinyPNG(t, addedW, addedH))
		var err error
		put, err = store.Put("put.png", tinyPNG(t, addedW, addedH))
		mustDo(t, err)
	})

	wantAsset(t, added, "assets/added.png", addedW, addedH)
	wantExists(t, filepath.Join(assetsDir, "added.png"))
	if put.Path == "" {
		t.Error("Put inside a report scope answered no path")
	}

	// And the record outlives the scope: a write during a report is a
	// write, not a scratch answer that unwinds with fn.
	wantAsset(t, mustResolve(t, store, uuidB), "assets/added.png", addedW, addedH)
}
