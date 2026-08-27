package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/pmarschik/adfast/convert"
)

// Metadata is the optional half of a Store: the index records addressed
// by content rather than by media id, and the metadata namespaces an
// embedder keeps on them.
//
// It is separate from Store because Store is the media pipeline — the
// download/upload round trip, which only ever deals in ids — and this is
// what an embedder needs when it PRODUCES an asset instead of receiving
// one. A rendered diagram has content and provenance long before it has
// a media id, so it cannot be recorded through Add, and what drew it is
// not something the store could model without knowing about diagrams.
// Both problems disappear once content can be stored on its own and
// carry an opaque bag alongside it.
//
// A metadata namespace is the embedder's: the store carries, merges and
// writes it back verbatim, and never looks inside. Namespace it after
// the concern that owns it ("diagram"), not after the tool.
//
// Reach it with MetaOf; FSStore implements it, and the Layered and
// ForScope wrappers forward it.
type Metadata interface {
	// Put stores content under a friendly name derived from
	// suggestedName, with no media id — the write direction for an asset
	// the embedder generated rather than downloaded. An upload later
	// binds an id to the same content through Associate. It returns the
	// render asset; ContentHash(content) is the key the record is under.
	Put(suggestedName string, content []byte) (convert.MediaAsset, error)
	// Meta returns the metadata recorded under ns for the content with
	// this hash.
	Meta(hash, ns string) (json.RawMessage, bool)
	// SetMeta records metadata under ns for the content with this hash,
	// replacing whatever was there. A nil value removes the namespace.
	// It fails for content the store does not hold.
	SetMeta(hash, ns string, value json.RawMessage) error
	// MetaHashes lists, sorted, the content hashes carrying metadata in
	// ns — the embedder's own worklist over the store.
	MetaHashes(ns string) []string
	// HashOf is the content hash of a referenced markdown-relative path,
	// which is how a caller holding a file name asks about its record.
	HashOf(path string) (string, bool)
	// NameOf is the friendly filename recorded for a content hash, the
	// reverse of HashOf for a caller that knows the content but not
	// where this folder reaches it.
	NameOf(hash string) (string, bool)
}

// MetaOf returns the metadata face of a store, if it has one. A store
// that does not carries assets but no provenance, and a caller that
// needs provenance has to say what it does without one.
func MetaOf(s Store) (Metadata, bool) {
	m, ok := s.(Metadata)
	return m, ok
}

// ContentHash is the key an index record is stored under: the
// 16-hex-digit sha256 prefix that also names the blob. Exported so a
// caller can name a record for Meta/SetMeta from the bytes it already
// holds, without a round trip through the filesystem.
func ContentHash(content []byte) string { return hashContent(content) }

// ErrUnknownContent reports metadata addressed to content the store does
// not hold. Store the content first — a record with metadata and no blob
// would describe a picture nothing can show.
var ErrUnknownContent = errors.New("no asset with that content hash")

// Put implements Metadata.
func (s *FSStore) Put(suggestedName string, content []byte) (convert.MediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	_, name, err := s.store(suggestedName, content)
	if err != nil {
		return convert.MediaAsset{}, err
	}
	if err := s.saveIndex(); err != nil {
		return convert.MediaAsset{}, err
	}
	return s.mustAsset(name), nil
}

// Meta implements Metadata.
func (s *FSStore) Meta(hash, ns string) (json.RawMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	raw, ok := s.records[hash].Meta[ns]
	return compactJSON(raw), ok
}

// compactJSON is the canonical form a namespace is stored and handed
// back in. It exists because writing the index with MarshalIndent
// re-indents the embedder's raw bytes to sit in the file, so a value
// read back after a save would otherwise not be the value that was
// written — the same JSON, spelled differently, which is the kind of
// difference that shows up as a diff nobody made.
func compactJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return raw
	}
	return buf.Bytes()
}

// SetMeta implements Metadata.
func (s *FSStore) SetMeta(hash, ns string, value json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	record, ok := s.records[hash]
	if _, had := record.Meta[ns]; !had && value == nil {
		// Removing what is not there is the state the caller asked for,
		// whether or not the content itself is known here.
		return nil
	}
	if !ok {
		return fmt.Errorf("set %s metadata for %s: %w", ns, hash, ErrUnknownContent)
	}
	if value == nil {
		delete(record.Meta, ns)
	} else {
		if record.Meta == nil {
			record.Meta = map[string]json.RawMessage{}
		}
		record.Meta[ns] = compactJSON(value)
	}
	s.records[hash] = record
	return s.saveIndex()
}

// MetaHashes implements Metadata.
func (s *FSStore) MetaHashes(ns string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	var out []string
	for hash, record := range s.records {
		if _, ok := record.Meta[ns]; ok {
			out = append(out, hash)
		}
	}
	slices.Sort(out)
	return out
}

// HashOf implements Metadata.
func (s *FSStore) HashOf(path string) (string, bool) {
	content, err := s.readRef(path)
	if err != nil {
		return "", false
	}
	return hashContent(content), true
}

// NameOf implements Metadata.
func (s *FSStore) NameOf(hash string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	record, ok := s.records[hash]
	if !ok || record.Name == "" {
		return "", false
	}
	return record.Name, true
}
