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
	a, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	const goroutines = 8
	const iterations = 6
	contents := make([][]byte, goroutines)
	for g := range contents {
		contents[g] = tinyPNG(t, g+1, 1)
	}

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			store := a
			if g%2 == 1 {
				store = b
			}
			for i := range iterations {
				id := fmt.Sprintf("%08d-0000-4000-8000-%012d", g, i)
				if _, addErr := store.Add("", id, fmt.Sprintf("g%d.png", g), contents[g]); addErr != nil {
					t.Errorf("g%d add %d: %v", g, i, addErr)
				}
				if _, ok := store.Resolve(id); !ok {
					t.Errorf("g%d resolve %d missed own add", g, i)
				}
				store.Lookup("", fmt.Sprintf("assets/g%d.png", g))
				assocID := fmt.Sprintf("%08d-1111-4000-8000-%012d", g, i)
				if _, assocErr := store.Associate("", assocID, "assets/seed.png"); assocErr != nil {
					t.Errorf("g%d associate %d: %v", g, i, assocErr)
				}
			}
		})
	}
	wg.Wait()

	// A fresh instance sees every goroutine's final state.
	fresh, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	for g := range goroutines {
		id := fmt.Sprintf("%08d-0000-4000-8000-%012d", g, iterations-1)
		if _, ok := fresh.Resolve(id); !ok {
			t.Errorf("g%d's last add is missing from the merged index", g)
		}
	}
}

// TestFSStore_InterleavedInstancesMergeIndex pins the reload-merge
// semantics: instance A loads (empty index), instance B adds, then A
// adds — A's write must merge B's entry, not clobber it.
func TestFSStore_InterleavedInstancesMergeIndex(t *testing.T) {
	mdDir := t.TempDir()
	a, err := NewFSStore(mdDir) // A loads before B writes anything
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := b.Add("", uuidB, "b.png", tinyPNG(t, 2, 2)); addErr != nil {
		t.Fatal(addErr)
	}
	if _, addErr := a.Add("", uuidA, "a.png", tinyPNG(t, 1, 1)); addErr != nil {
		t.Fatal(addErr)
	}
	fresh, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
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
