package assets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// mustMeta is the metadata face of a store that must have one.
func mustMeta(t *testing.T, s Store) Metadata {
	t.Helper()
	m, ok := MetaOf(s)
	if !ok {
		t.Fatalf("store %T has no metadata face", s)
	}
	return m
}

// mustPut stores generated content with no media id.
func mustPut(t *testing.T, m Metadata, name string, content []byte) string {
	t.Helper()
	asset, err := m.Put(name, content)
	if err != nil {
		t.Fatalf("Put(%q): %v", name, err)
	}
	return asset.Path
}

// TestPut_GeneratedContentIsAStoreSymlink: an asset the embedder
// produced enters the store the same way a download does — one blob,
// with the friendly name linked to it — even though no media id exists
// yet. A later upload binds the id without a second copy appearing.
func TestPut_GeneratedContentIsAStoreSymlink(t *testing.T) {
	mdDir := t.TempDir()
	s := mustStore(t, mdDir)
	content := tinyPNG(t, 5, 6)

	if got := mustPut(t, mustMeta(t, s), "chart.png", content); got != "assets/chart.png" {
		t.Errorf("Put path = %q, want assets/chart.png", got)
	}
	friendly := filepath.Join(mdDir, "assets", "chart.png")
	wantSymlink(t, friendly)
	wantEntryCount(t, filepath.Join(mdDir, "assets", ".store"), 2) // the blob + index.json

	// No id yet, so it is on the upload worklist and resolves to nothing.
	wantPending(t, s, "assets/chart.png")
	wantNoLookup(t, s, "", "assets/chart.png")

	// The upload binds an id to the content already stored; the friendly
	// file stays the same link and no duplicate blob appears.
	wantPath(t, mustAssociate(t, s, "", uuidA, "assets/chart.png"), "assets/chart.png")
	wantSymlink(t, friendly)
	wantEntryCount(t, filepath.Join(mdDir, "assets", ".store"), 2)
	wantLookup(t, s, "", "assets/chart.png", uuidA)
	wantPending(t, s)
}

// TestPut_IdenticalContentSharesOneRecord: two documents generating the
// same picture deduplicate to one blob and one record, which is what
// keying the index by content rather than by media id buys.
func TestPut_IdenticalContentSharesOneRecord(t *testing.T) {
	root := t.TempDir()
	content := tinyPNG(t, 2, 3)
	blobs := filepath.Join(root, ".asset-store")

	var paths []string
	for _, doc := range []string{"one", "two"} {
		docDir := mustMkdir(t, filepath.Join(root, doc))
		s := mustSplitStore(t, root, docDir, WithStoreDir(".asset-store"))
		paths = append(paths, mustPut(t, mustMeta(t, s), "chart.png", content))
		wantSymlink(t, filepath.Join(docDir, "assets", "chart.png"))
	}
	if paths[0] != "assets/chart.png" || paths[1] != "assets/chart.png" {
		t.Errorf("paths = %v, want both assets/chart.png", paths)
	}
	wantEntryCount(t, blobs, 2) // one blob for both documents + index.json

	shared := mustSplitStore(t, root, filepath.Join(root, "one"), WithStoreDir(".asset-store"))
	if got := len(shared.records); got != 1 {
		t.Errorf("records = %d, want 1 shared record", got)
	}
}

// TestMeta_RoundTripAndRemoval: metadata is carried verbatim under its
// namespace, listed by MetaHashes, and removed by a nil value.
func TestMeta_RoundTripAndRemoval(t *testing.T) {
	s := mustStore(t, t.TempDir())
	m := mustMeta(t, s)
	content := tinyPNG(t, 1, 1)
	mustPut(t, m, "chart.png", content)
	hash := ContentHash(content)

	const ns = "diagram"
	value := json.RawMessage(`{"lang":"d2","source":"a -> b"}`)
	if err := m.SetMeta(hash, ns, value); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	got, ok := m.Meta(hash, ns)
	if !ok || string(got) != string(value) {
		t.Errorf("Meta = %q, %v; want %q, true", got, ok, value)
	}
	if hashes := m.MetaHashes(ns); !slices.Equal(hashes, []string{hash}) {
		t.Errorf("MetaHashes = %v, want [%s]", hashes, hash)
	}
	if hashes := m.MetaHashes("other"); len(hashes) != 0 {
		t.Errorf("MetaHashes(other) = %v, want none", hashes)
	}

	if err := m.SetMeta(hash, ns, nil); err != nil {
		t.Fatalf("SetMeta(nil): %v", err)
	}
	if _, ok := m.Meta(hash, ns); ok {
		t.Error("Meta survived removal")
	}
	if hashes := m.MetaHashes(ns); len(hashes) != 0 {
		t.Errorf("MetaHashes after removal = %v, want none", hashes)
	}
	// The asset itself is untouched by its metadata coming and going.
	wantPending(t, s, "assets/chart.png")
}

// TestMeta_UnknownContentRefused: metadata for content the store does
// not hold would describe a picture nothing can show.
func TestMeta_UnknownContentRefused(t *testing.T) {
	m := mustMeta(t, mustStore(t, t.TempDir()))
	err := m.SetMeta("0123456789abcdef", "diagram", json.RawMessage(`{}`))
	if !errors.Is(err, ErrUnknownContent) {
		t.Errorf("SetMeta on unknown content = %v, want ErrUnknownContent", err)
	}
	// Removing what was never there is not an error — it is the state
	// the caller asked for.
	if err := m.SetMeta("0123456789abcdef", "diagram", nil); err != nil {
		t.Errorf("SetMeta(nil) on unknown content = %v, want nil", err)
	}
}

// TestMeta_SurvivesAnotherWriter: a second instance over the same index
// keeps the metadata it did not write, the way it keeps ids it did not
// record.
func TestMeta_SurvivesAnotherWriter(t *testing.T) {
	mdDir := t.TempDir()
	content := tinyPNG(t, 4, 4)
	hash := ContentHash(content)

	// Opened first, so it holds a pre-metadata view of the record.
	early := mustStore(t, mdDir)

	late := mustStore(t, mdDir)
	mustPut(t, mustMeta(t, late), "chart.png", content)
	value := json.RawMessage(`{"lang":"d2"}`)
	if err := mustMeta(t, late).SetMeta(hash, "diagram", value); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	// early merges the record on its next read and must not drop the
	// namespace when it writes the index back.
	mustAdd(t, early, uuidA, "chart.png", content)
	reopened := mustStore(t, mdDir)
	got, ok := mustMeta(t, reopened).Meta(hash, "diagram")
	if !ok || string(got) != string(value) {
		t.Errorf("Meta after another writer = %q, %v; want %q, true", got, ok, value)
	}
	wantLookup(t, reopened, "", "assets/chart.png", uuidA)
}

// TestMeta_ThroughWrappers: ForScope and Layered pass the metadata face
// through, and a store without one does not grow a broken face by being
// wrapped.
func TestMeta_ThroughWrappers(t *testing.T) {
	mdDir := t.TempDir()
	s := mustStore(t, mdDir)
	content := tinyPNG(t, 2, 2)
	hash := ContentHash(content)
	mustPut(t, mustMeta(t, s), "chart.png", content)

	for name, wrapped := range map[string]Store{
		"ForScope": ForScope(s, "PROJ-1"),
		"Layered":  Layered(s),
		"both":     ForScope(Layered(s), "PROJ-1"),
	} {
		t.Run(name, func(t *testing.T) {
			m := mustMeta(t, wrapped)
			if err := m.SetMeta(hash, name, json.RawMessage(`{"through":true}`)); err != nil {
				t.Fatalf("SetMeta: %v", err)
			}
			if _, ok := m.Meta(hash, name); !ok {
				t.Error("Meta did not reach the wrapped store")
			}
			if got, ok := m.HashOf("assets/chart.png"); !ok || got != hash {
				t.Errorf("HashOf = %q, %v; want %q, true", got, ok, hash)
			}
			if got, ok := m.NameOf(hash); !ok || got != "chart.png" {
				t.Errorf("NameOf = %q, %v; want chart.png, true", got, ok)
			}
		})
	}

	if _, ok := MetaOf(ForScope(storeWithoutMeta{}, "PROJ-1")); ok {
		t.Error("a scoped view claims metadata its store does not have")
	}
	if _, ok := MetaOf(Layered(storeWithoutMeta{})); ok {
		t.Error("a layered stack claims metadata no layer has")
	}
}

// TestIndexDeterminism: independent writers produce identical bytes, so
// the index does not churn a working copy — the ids of a record are
// sorted rather than left in insertion order.
func TestIndexDeterminism(t *testing.T) {
	content := tinyPNG(t, 3, 3)
	var written []string
	for _, order := range [][]string{{uuidA, uuidB, uuidC}, {uuidC, uuidA, uuidB}} {
		mdDir := t.TempDir()
		s := mustStore(t, mdDir)
		for _, id := range order {
			mustAdd(t, s, id, "shot.png", content)
		}
		if err := mustMeta(t, s).SetMeta(ContentHash(content), "diagram", json.RawMessage(`{"lang":"d2"}`)); err != nil {
			t.Fatalf("SetMeta: %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(mdDir, "assets", ".store", "index.json"))
		mustDo(t, err)
		written = append(written, string(raw))
	}
	if written[0] != written[1] {
		t.Errorf("index depends on insertion order:\n%s\n--- vs ---\n%s", written[0], written[1])
	}
}

// storeWithoutMeta is a Store with no metadata face, for the wrappers.
type storeWithoutMeta struct{ Store }
