package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestFSStore_ConcurrentUse stresses Add/Lookup/Resolve/Associate from
// multiple goroutines against one FSStore plus a second instance over
// the same directory — the intra-process mutex and the reload-merge
// index discipline must hold under the race detector.
func TestFSStore_ConcurrentUse(t *testing.T) {
	mdDir := t.TempDir()
	writeAsset(t, mdDir, "seed.png", tinyPNG(t, 9, 9))
	a := mustStore(t, mdDir)
	b := mustStore(t, mdDir)
	const goroutines = 8
	const iterations = 6

	var wg sync.WaitGroup
	for g := range goroutines {
		// Odd goroutines drive the second instance over the same
		// directory, so the reload-merge path runs concurrently too.
		store := Store(a)
		if g%2 == 1 {
			store = b
		}
		content := tinyPNG(t, g+1, 1)
		wg.Go(func() { hammerStore(t, store, g, iterations, content) })
	}
	wg.Wait()

	// A fresh instance sees every goroutine's final state.
	fresh := mustStore(t, mdDir)
	for g := range goroutines {
		if _, ok := fresh.Resolve(addedID(g, iterations-1)); !ok {
			t.Errorf("g%d's last add is missing from the merged index", g)
		}
	}
}

// hammerStore runs one goroutine's share of the concurrent workload:
// add, resolve one's own add, look the friendly path up, and associate a
// second id with the shared seed file — repeated.
func hammerStore(t *testing.T, store Store, g, iterations int, content []byte) {
	t.Helper()
	for i := range iterations {
		id := addedID(g, i)
		if _, err := store.Add("", id, fmt.Sprintf("g%d.png", g), content); err != nil {
			t.Errorf("g%d add %d: %v", g, i, err)
		}
		if _, ok := store.Resolve(id); !ok {
			t.Errorf("g%d resolve %d missed own add", g, i)
		}
		store.Lookup("", fmt.Sprintf("assets/g%d.png", g))
		if _, err := store.Associate("", associatedID(g, i), "assets/seed.png"); err != nil {
			t.Errorf("g%d associate %d: %v", g, i, err)
		}
	}
}

// addedID and associatedID are the two disjoint media-id families the
// concurrent workload writes — one per Add, one per Associate.
func addedID(g, i int) string { return fmt.Sprintf("%08d-0000-4000-8000-%012d", g, i) }

func associatedID(g, i int) string { return fmt.Sprintf("%08d-1111-4000-8000-%012d", g, i) }

// TestFSStore_InterleavedInstancesMergeIndex pins the reload-merge
// semantics: instance A loads (empty index), instance B adds, then A
// adds — A's write must merge B's entry, not clobber it.
func TestFSStore_InterleavedInstancesMergeIndex(t *testing.T) {
	mdDir := t.TempDir()
	a := mustStore(t, mdDir) // A loads before B writes anything
	b := mustStore(t, mdDir)
	mustAdd(t, b, uuidB, "b.png", tinyPNG(t, 2, 2))
	mustAdd(t, a, uuidA, "a.png", tinyPNG(t, 1, 1))
	fresh := mustStore(t, mdDir)
	if _, ok := fresh.Resolve(uuidB); !ok {
		t.Error("B's entry was clobbered by A's later write")
	}
	if _, ok := fresh.Resolve(uuidA); !ok {
		t.Error("A's entry is missing")
	}
	// Both friendly files exist independently.
	for _, name := range []string{"a.png", "b.png"} {
		if _, statErr := os.Lstat(filepath.Join(mdDir, "assets", name)); statErr != nil {
			t.Errorf("friendly file %s: %v", name, statErr)
		}
	}
}
