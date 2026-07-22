package assets

import (
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
	return layeredStore(layers)
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
