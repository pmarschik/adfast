package assets

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/convert"
)

// cleanedRef is the reference the push rows below all write.
const cleanedRef = "assets/shot.png"

// offeringUploader records every path it is handed and answers each with
// the same media id — the "what did the push actually offer" probe.
func offeringUploader(offered *[]string, mediaID string) Uploader {
	return UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		out := make([]UploadResult, 0, len(batch))
		for _, p := range batch {
			*offered = append(*offered, p.Path)
			out = append(out, UploadResult{Path: p.Path, MediaID: mediaID})
		}
		return out, nil
	})
}

// TestPushOffersAnAssetWhoseBlobTheStoreStillHolds: a push used to lose a
// picture whose bytes the store still had. The friendly file under
// assets/ was gone — cleaned up, or a checkout that never fetched it —
// while the blob and its index record stayed. Every worklist a push
// builds is keyed by PATH (PendingRefs asks the store about each
// reference the document makes), and the path-keyed read went through the
// friendly file alone: the read missed, the file was never offered for
// upload, nothing ever attached a media id to it, and the encode dropped
// the picture from the payload with "not in the asset store" — sending
// the author off to re-add an asset the store already held.
//
// Reading BY MEDIA ID had repaired that folder all along (assetOf →
// materialize). The fix is the path-keyed twin of the same repair, so the
// rows here are the four fates a reference can have, and only one of them
// is the bug.
func TestPushOffersAnAssetWhoseBlobTheStoreStillHolds(t *testing.T) {
	for name, tc := range map[string]struct {
		wantOffered []string
		// recorded places the asset through the OTHER page's view, so the
		// media id on it is one that cannot travel to this push.
		recorded bool
		// file says whether assets/shot.png exists when the push runs.
		file        bool
		wantPicture bool
	}{
		// GOOD: the markdown-first flow — the author drops a file into
		// assets/ and references it before anything is uploaded.
		"the author dropped the file in": {
			file: true, wantOffered: []string{cleanedRef}, wantPicture: true,
		},
		// GOOD: the file is there and carries another container's media id.
		// That id cannot travel here, so the file still uploads.
		"the file is there under another page's id": {
			recorded: true, file: true, wantOffered: []string{cleanedRef}, wantPicture: true,
		},
		// THE FIX: same store, same blob, same record — only the friendly
		// file is missing.
		"the file is gone and the blob is held": {
			recorded: true, wantOffered: []string{cleanedRef}, wantPicture: true,
		},
		// GOOD: an asset genuinely not in the store — no file, no blob, no
		// record. Nothing can upload it and the picture is dropped, which
		// is the case the message the row above no longer produces is for.
		"the store has nothing for it": {},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			docDir := mustMkdir(t, filepath.Join(root, "docs", "a"))
			store := mustSplitStore(t, root, docDir)
			friendly := filepath.Join(docDir, "assets", "shot.png")
			switch {
			case tc.recorded:
				mustAdd(t, ForScope(store, "PAGE-1"), uuidA, "shot.png", tinyPNG(t, 3, 4))
			case tc.file:
				writeAsset(t, docDir, "shot.png", tinyPNG(t, 3, 4))
			}
			if tc.recorded && !tc.file {
				mustDo(t, os.Remove(friendly))
			}

			var offered, diags []string
			sink := adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d.Message) })
			pipe := PushPipeline(t.Context(), ForScope(store, "PAGE-2"), offeringUploader(&offered, uuidB), sink)
			docs := mustPushAll(t, pipe, []string{"![one](" + cleanedRef + ")\n"})

			if !slices.Equal(offered, tc.wantOffered) {
				t.Errorf("uploader offered %v, want %v", offered, tc.wantOffered)
			}
			if !tc.wantPicture {
				wantNoMedia(t, docs[0], uuidB)
				wantDropReported(t, diags)
				wantAbsent(t, friendly) // nothing held for it, nothing invented
				return
			}
			wantMedia(t, docs[0], uuidB)
			if len(diags) != 0 {
				t.Errorf("the picture was placed but the encode still complained: %v", diags)
			}
			// The repair is a repair: the file the reference names is there
			// again, exactly as reading by media id leaves it.
			wantExists(t, friendly)
		})
	}
}

// wantDropReported pins the one diagnostic an encode emits for a picture
// it could not place — the message this bug was reported through.
func wantDropReported(t *testing.T, diags []string) {
	t.Helper()
	if len(diags) != 1 || !strings.Contains(diags[0], "has no media id (not in the asset store)") {
		t.Errorf("diagnostics = %v, want the one unplaceable-image message", diags)
	}
}

// TestTheRepairRefusesWhatItCannotKnow: the path-keyed repair puts a file
// back, so its refusals are the interesting half — a repair that guesses
// writes a wrong file into the working copy and then answers about it as
// fact. It declines wherever the repair by media id declines, plus one
// case only a path has: a reference is a PATH, and a record's friendly
// name is a bare name at the assets folder's top level, so a basename
// that happens to match says nothing about a reference below it.
func TestTheRepairRefusesWhatItCannotKnow(t *testing.T) {
	t.Run("an ambiguous friendly name", func(t *testing.T) {
		// One blob store, two document folders — the same friendly name can
		// stand for different content in each, and the name alone cannot say
		// which one the cleaned document meant.
		root := t.TempDir()
		docA := mustMkdir(t, filepath.Join(root, "docs", "a"))
		docB := mustMkdir(t, filepath.Join(root, "docs", "b"))
		viewA := mustSplitStore(t, root, docA)
		viewB := mustSplitStore(t, root, docB)
		mustAdd(t, viewA, uuidA, "shot.png", tinyPNG(t, 3, 4))
		mustAdd(t, viewB, uuidB, "shot.png", tinyPNG(t, 5, 6))
		friendly := filepath.Join(docA, "assets", "shot.png")
		mustDo(t, os.Remove(friendly))

		wantNoLookup(t, viewA, "", cleanedRef)
		wantNoDims(t, viewA, cleanedRef)
		wantLoadRefused(t, viewA, cleanedRef)
		wantAbsent(t, friendly)
	})

	t.Run("a reference into a subdirectory", func(t *testing.T) {
		root := t.TempDir()
		docDir := mustMkdir(t, filepath.Join(root, "docs", "a"))
		store := mustSplitStore(t, root, docDir)
		mustAdd(t, store, uuidA, "chart.png", tinyPNG(t, 3, 4))
		flat := filepath.Join(docDir, "assets", "chart.png")
		mustDo(t, os.Remove(flat))

		const nested = "assets/sub/chart.png"
		wantNoLookup(t, store, "", nested)
		wantNoDims(t, store, nested)
		wantLoadRefused(t, store, nested)
		// Silent on disk too: the record's file is not restored on the
		// strength of a path that never named it.
		wantAbsent(t, flat)

		// GOOD case: the refusal is about the path, not about the record —
		// the reference the record IS for still reads, and repairs.
		wantLoad(t, store, "assets/chart.png")
		wantExists(t, flat)
	})
}
