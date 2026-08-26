package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNoAsset says the store holds nothing under a content hash: no index
// record, and no blob either. It is what Forget returns instead of
// reporting a removal it did not make.
var ErrNoAsset = errors.New("no asset with that content hash")

// Pruner is the removal face of a Store: the one operation that takes
// content out again.
//
// It is separate from Store for the reason every other face here is. The
// media pipeline only ever adds — it resolves an id, it uploads a path,
// it records what it stored — and a store that cannot forget is exactly
// right for it. Deciding that content is unused is a question about a
// whole project, which no store can answer: it is keyed by content, so
// nothing in it says which document put it there, and two documents
// publishing one picture are meant to be one blob. So the caller decides
// what is unused, and this is what it says so to.
//
// Reach it with PrunerOf. FSStore implements it; a wrapper that does not
// forward it is a store a caller must not prune through, which is the
// safe way round.
type Pruner interface {
	// Forget removes one asset by content hash: the index record, the
	// content-addressed blob, and the friendly file of this store's own
	// assets folder when it is the link that reached the blob. It returns
	// what it removed, or ErrNoAsset when the store held neither half.
	Forget(hash string) (Forgotten, error)
}

// Forgotten says what one removal took away. A field left empty is a
// half the store did not hold: an index record describing content whose
// blob is already gone forgets a record and no bytes.
type Forgotten struct {
	// Hash is the content hash asked for.
	Hash string
	// Name is the friendly filename the record carried, empty when there
	// was no record.
	Name string
	// Blob is the content-addressed file removed, absolute. Empty when
	// the store had no blob for the hash.
	Blob string
	// Friendly is the file removed from this store's own assets folder,
	// absolute. Empty unless that folder reached the blob through a link,
	// which for a shared store is the usual case: the friendly files live
	// beside the documents, in folders this store never sees.
	Friendly string
	// Size is the blob's size in bytes, 0 when Blob is empty.
	Size int64
	// Record says whether an index record was removed.
	Record bool
}

// PrunerOf returns the removal face of a store, if it has one. A store
// that does not can still be read and written; it just cannot be pruned,
// and a caller that wanted to has to say what it does instead.
func PrunerOf(s Store) (Pruner, bool) {
	p, ok := s.(Pruner)
	return p, ok
}

// Forget implements Pruner.
//
// The blob goes before the record, so a failure between the two leaves a
// record whose content is gone rather than content nothing names. The
// first is a state the index already describes and the next Forget
// finishes; the second is a file no reader of the store can see.
func (s *FSStore) Forget(hash string) (Forgotten, error) {
	if !storeHashRe.MatchString(hash) {
		return Forgotten{}, fmt.Errorf("asset hash %q: %w", hash, ErrNoAsset)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()

	record, known := s.records[hash]
	blob, held := s.blobFiles()[hash]
	if !known && !held {
		return Forgotten{}, fmt.Errorf("%s: %w", hash, ErrNoAsset)
	}

	out := Forgotten{Hash: hash, Name: record.Name, Record: known}
	if held {
		out.Friendly = s.forgetFriendly(record.Name, filepath.Base(blob.path))
		if err := os.Remove(blob.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Forgotten{}, fmt.Errorf("remove asset content: %w", err)
		}
		out.Blob, out.Size = blob.path, blob.size
	}
	if known {
		delete(s.records, hash)
		if err := s.saveIndex(); err != nil {
			return out, err
		}
	}
	return out, nil
}

// forgetFriendly removes this store's own friendly file when it is the
// link that reached the blob, and returns what it removed.
//
// Only a link, and only one pointing at this blob. A regular file of the
// same name is content somebody put there by hand — the markdown-first
// flow createFriendly adopts — and removing that would take away the
// only copy there is over a decision about the store's copy.
func (s *FSStore) forgetFriendly(name, storeName string) string {
	if name == "" || !s.friendlyMatches(name, storeName) {
		return ""
	}
	full, err := s.securePath(false, name)
	if err != nil || os.Remove(full) != nil {
		return ""
	}
	return full
}
