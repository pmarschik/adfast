package assets

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/pmarschik/adfast/convert"
)

// Layered composes stores into one: reads consult each layer in order
// (first match wins) and per-file operations route to the first layer
// that knows the path, so different layers can own different files.
// Add — the download-direction write — goes to the first layer.
//
// The canonical composition is a document-local store over a shared
// project-root one:
//
//	local, _ := assets.NewFSStore(docDir)
//	root, ok := assets.DiscoverRoot(docDir, ".git")
//	shared, _ := assets.NewFSStoreAt(root, docDir)
//	store := assets.Layered(local, shared)
//
// Documents then resolve assets from either folder, new downloads land
// locally, and uploads (Pending is the ordered, deduplicated union)
// associate back into whichever layer holds the file.
func Layered(layers ...Store) Store {
	stack := layeredStore(layers)
	if !slices.ContainsFunc(layers, func(s Store) bool { _, ok := MetaOf(s); return ok }) {
		return stack
	}
	return &layeredMeta{layeredStore: stack}
}

type layeredStore []Store

// Resolve implements Store: first layer that resolves wins.
func (l layeredStore) Resolve(mediaID string) (convert.MediaAsset, bool) {
	for _, s := range l {
		if asset, ok := s.Resolve(mediaID); ok {
			return asset, true
		}
	}
	return convert.MediaAsset{}, false
}

// Lookup implements Store: first layer that knows the path wins.
func (l layeredStore) Lookup(scope, path string) (string, bool) {
	for _, s := range l {
		if id, ok := s.Lookup(scope, path); ok {
			return id, true
		}
	}
	return "", false
}

// Add implements Store: downloads land in the first layer.
func (l layeredStore) Add(scope, mediaID, suggestedName string, content []byte) (convert.MediaAsset, error) {
	if len(l) == 0 {
		return convert.MediaAsset{}, errors.New("layered store has no layers")
	}
	return l[0].Add(scope, mediaID, suggestedName, content)
}

// Assets implements Store: the merged map, earlier layers winning on
// media-id conflicts.
func (l layeredStore) Assets() map[string]convert.MediaAsset {
	out := map[string]convert.MediaAsset{}
	for _, i := range slices.Backward(l) {
		maps.Copy(out, i.Assets())
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Pending implements Store: the ordered union of the layers' worklists
// (deduplicated on the reference path).
func (l layeredStore) Pending(scope string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, s := range l {
		paths, err := s.Pending(scope)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// Associate implements Store: the association lands in the first layer
// that can load the path — the one owning the file.
func (l layeredStore) Associate(scope, mediaID, path string) (convert.MediaAsset, error) {
	for _, s := range l {
		if _, err := s.Load(path); err == nil {
			return s.Associate(scope, mediaID, path)
		}
	}
	return convert.MediaAsset{}, fmt.Errorf("associate: no layer holds %q", path)
}

// Load implements Store: first layer that can read the path.
func (l layeredStore) Load(path string) ([]byte, error) {
	var firstErr error
	for _, s := range l {
		content, err := s.Load(path)
		if err == nil {
			return content, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("load: no layer holds %q", path)
	}
	return nil, firstErr
}

// Dims implements Store: first layer that can probe the path.
func (l layeredStore) Dims(path string) (width, height int, ok bool) {
	for _, s := range l {
		if w, h, ok := s.Dims(path); ok {
			return w, h, true
		}
	}
	return 0, 0, false
}

// layeredMeta adds the Metadata face to a stack that has at least one
// layer carrying it — so MetaOf answers for the composition exactly when
// it answers for something inside it.
//
// Records are content-addressed, so a hash means the same thing in every
// layer, and reads take the first layer that knows it. Writes take the
// first layer that CAN take them: Put lands where a download would, and
// SetMeta goes to the layer already holding the content, because
// metadata for content a layer does not have is a record describing a
// picture nothing there can show.
type layeredMeta struct {
	layeredStore
}

// meta yields each layer's metadata face, in order.
func (l *layeredMeta) meta(yield func(Metadata) bool) {
	for _, s := range l.layeredStore {
		m, ok := MetaOf(s)
		if ok && !yield(m) {
			return
		}
	}
}

// Put implements Metadata: generated content lands in the first layer
// that can hold it, like a download.
func (l *layeredMeta) Put(suggestedName string, content []byte) (convert.MediaAsset, error) {
	for m := range l.meta {
		return m.Put(suggestedName, content)
	}
	return convert.MediaAsset{}, errors.New("layered store has no metadata layer")
}

// Meta implements Metadata: first layer that has the namespace.
func (l *layeredMeta) Meta(hash, ns string) (json.RawMessage, bool) {
	for m := range l.meta {
		if raw, ok := m.Meta(hash, ns); ok {
			return raw, true
		}
	}
	return nil, false
}

// SetMeta implements Metadata: the layer that holds the content takes
// the write.
func (l *layeredMeta) SetMeta(hash, ns string, value json.RawMessage) error {
	var firstErr error
	for m := range l.meta {
		err := m.SetMeta(hash, ns, value)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		firstErr = errors.New("layered store has no metadata layer")
	}
	return firstErr
}

// MetaHashes implements Metadata: the sorted union across the layers.
func (l *layeredMeta) MetaHashes(ns string) []string {
	var out []string
	for m := range l.meta {
		out = append(out, m.MetaHashes(ns)...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// HashOf implements Metadata: first layer that can read the path.
func (l *layeredMeta) HashOf(path string) (string, bool) {
	for m := range l.meta {
		if hash, ok := m.HashOf(path); ok {
			return hash, true
		}
	}
	return "", false
}

// NameOf implements Metadata: first layer that knows the content.
func (l *layeredMeta) NameOf(hash string) (string, bool) {
	for m := range l.meta {
		if name, ok := m.NameOf(hash); ok {
			return name, true
		}
	}
	return "", false
}
