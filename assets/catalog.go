package assets

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Catalog is the enumeration face of a Store: everything the store
// holds, as data, in one pass.
//
// It is separate from Store because Store is the media pipeline, and the
// pipeline only ever asks about one asset at a time — an id it is
// resolving, a path it is uploading. Assets comes closest to an
// inventory and is not one: it is keyed by media id, so it can only
// report the assets an upload has already been through. Content that was
// generated rather than downloaded (a rendered diagram), or added to the
// folder and not yet pushed, is in the store, is reachable by name, and
// is invisible there. A tool that has to answer "what is in here, and is
// any of it unused" needs the records themselves.
//
// It is separate from Metadata for the same reason in the other
// direction: Metadata addresses ONE record at a time, by a hash or a
// path the caller already has. MetaHashes lists a namespace, which is an
// embedder's worklist over its own concern, not an inventory of the
// store.
//
// Reach it with CatalogOf; FSStore implements it, and the Layered and
// ForScope wrappers forward it.
type Catalog interface {
	// Records lists every asset in the store, sorted by content hash. The
	// slice and the records in it are the caller's — mutating them does
	// not reach the store.
	Records() []Record
}

// Record is one asset in the store, as a caller reading the index sees
// it: the whole truth about one piece of content in one value.
//
// It is a struct rather than the on-disk assetRecord because the two
// answer to different things. assetRecord is a file format, free to grow
// fields and change how they are spelled; this is what a caller was
// promised. The content hash is a field here rather than a map key
// because a record travels on its own once it leaves the index, and a
// hash it cannot state is a record nothing can ask a second question
// about.
type Record struct {
	// Meta is the embedder metadata, one entry per namespace, exactly as
	// it sits in the index. Nil when the record carries none.
	Meta map[string]json.RawMessage
	// Hash is the content hash the record is keyed by, which also names
	// the blob.
	Hash string
	// Name is the friendly filename an assets folder reaches the content
	// under, without any directory part.
	Name string
	// Blob is where the content-addressed file lives, absolute. Empty for
	// a store that is not filesystem-backed, and for a record whose blob
	// is gone — an index entry describing content nothing can show.
	Blob string
	// IDs are the media ids pointing at this content, one per visibility
	// scope it was uploaded into, sorted. Empty for content that has
	// never been uploaded.
	IDs []MediaRef
	// Size is the blob's size in bytes, 0 when Blob is empty.
	Size int64
}

// MediaRef is one media id pointing at a record's content. Scope is the
// visibility container the id is valid in (a Jira issue key, a
// Confluence page id); empty for an unscoped record.
type MediaRef struct {
	ID    string
	Scope string
}

// CatalogOf returns the enumeration face of a store, if it has one. A
// store that does not can still be read asset by asset; it just cannot
// say what it holds, so a caller that needs an inventory has to say what
// it does without one.
func CatalogOf(s Store) (Catalog, bool) {
	c, ok := s.(Catalog)
	return c, ok
}

// Records implements Catalog.
func (s *FSStore) Records() []Record {
	blobs := s.blobFiles()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	out := make([]Record, 0, len(s.records))
	for hash, rec := range s.records {
		r := Record{
			Hash: hash,
			Name: rec.Name,
			IDs:  make([]MediaRef, 0, len(rec.IDs)),
			Meta: maps.Clone(rec.Meta),
		}
		for _, ref := range rec.IDs {
			// The conversion is deliberate: it compiles only while the
			// exported shape still says everything the on-disk one does, so
			// a field added to mediaRef has to be answered for here.
			r.IDs = append(r.IDs, MediaRef(ref))
		}
		if blob, ok := blobs[hash]; ok {
			r.Blob, r.Size = blob.path, blob.size
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Record) int { return strings.Compare(a.Hash, b.Hash) })
	return out
}

// blobFile is a content-addressed file found on disk.
type blobFile struct {
	path string
	size int64
}

// blobFiles maps content hash to the blob directory's file for it.
//
// The directory is read rather than each name derived, because a blob is
// named for its hash plus the extension of whatever the friendly name
// was when the content was first stored, and the friendly name can be
// changed afterwards. Deriving the file from the record would then miss
// a blob that is sitting right there, and report content the store holds
// as content it lost. Reading the directory asks the only thing that
// knows.
func (s *FSStore) blobFiles() map[string]blobFile {
	entries, err := os.ReadDir(s.blobDir())
	if err != nil {
		return nil
	}
	out := make(map[string]blobFile, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		hash := strings.TrimSuffix(name, filepath.Ext(name))
		if !storeHashRe.MatchString(hash) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out[hash] = blobFile{path: filepath.Join(s.blobDir(), name), size: info.Size()}
	}
	return out
}

// layeredCatalog merges the layers' inventories: one record per content
// hash, the first layer holding it winning, which is the order every
// other read uses. Layers over one shared index therefore report it
// once, and layers over different stores report the union.
type layeredCatalog struct {
	layers []Catalog
}

// Records implements Catalog.
func (l layeredCatalog) Records() []Record {
	seen := map[string]bool{}
	var out []Record
	for _, c := range l.layers {
		for _, r := range c.Records() {
			if seen[r.Hash] {
				continue
			}
			seen[r.Hash] = true
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b Record) int { return strings.Compare(a.Hash, b.Hash) })
	return out
}

// catalogsOf is the enumerable layers of a stack, in order, and whether
// there are any.
func catalogsOf(layers []Store) ([]Catalog, bool) {
	var out []Catalog
	for _, s := range layers {
		if c, ok := CatalogOf(s); ok {
			out = append(out, c)
		}
	}
	return out, len(out) > 0
}
