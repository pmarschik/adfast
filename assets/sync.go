package assets

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/pmarschik/adfast/convert"
)

// PendingAsset is one upload work item: a file referenced from markdown
// that has no media id yet.
type PendingAsset struct {
	// Path is the markdown-relative reference ("assets/shot.png").
	Path string
	// Name is the friendly filename ("shot.png").
	Name string
	// Content is the file's bytes — loaded so uploaders need no
	// filesystem access and the uploaded bytes match what was hashed.
	Content []byte
}

// UploadResult reports one uploaded asset: the product-assigned media id
// for a pending path.
type UploadResult struct {
	Path    string
	MediaID string
}

// Uploader performs the product-side upload. It is the pluggable seam
// for actual media management — implementations talk to Jira attachment
// or Confluence media APIs (or anything else). A single call receives
// the WHOLE batch so implementations can fold it into a bulk request;
// returning results for a subset is valid (those are associated, the
// rest stay pending for the next sync).
type Uploader interface {
	Upload(ctx context.Context, batch []PendingAsset) ([]UploadResult, error)
}

// UploaderFunc adapts a function to the Uploader interface.
type UploaderFunc func(ctx context.Context, batch []PendingAsset) ([]UploadResult, error)

// Upload implements Uploader.
func (f UploaderFunc) Upload(ctx context.Context, batch []PendingAsset) ([]UploadResult, error) {
	return f(ctx, batch)
}

// Sync is the lazy write-path: it collects everything pending, hands the
// batch to the uploader, and associates the assigned media ids. Nothing
// happens when nothing is pending. Successful results are associated
// even when the uploader also returns an error (partial-batch failures
// keep their progress; the failed remainder stays pending).
func Sync(ctx context.Context, store Store, up Uploader) (map[string]convert.MediaAsset, error) {
	paths, err := store.Pending("")
	if err != nil {
		return nil, err
	}
	return uploadPaths(ctx, store, up, paths)
}

// uploadPaths loads the given pending paths, hands them to the uploader
// as one batch, and associates the assigned media ids (partial results
// keep their progress). The batch is deduplicated by content: identical
// files upload once — with a content-addressed store like FSStore every
// duplicate path resolves to the same media id afterwards.
func uploadPaths(ctx context.Context, store Store, up Uploader, paths []string) (map[string]convert.MediaAsset, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	batch := make([]PendingAsset, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		content, loadErr := store.Load(p)
		if loadErr != nil {
			return nil, loadErr
		}
		hash := contentHash(content)
		if seen[hash] {
			continue
		}
		seen[hash] = true
		batch = append(batch, PendingAsset{Path: p, Name: filepath.Base(filepath.FromSlash(p)), Content: content})
	}
	results, upErr := up.Upload(ctx, batch)
	// Collect every association failure — a single bad result must not
	// mask the others (or the uploader's own error).
	errs := []error{upErr}
	associated := make(map[string]convert.MediaAsset, len(results))
	for _, r := range results {
		if r.MediaID == "" {
			continue
		}
		asset, assocErr := store.Associate("", r.MediaID, r.Path)
		if assocErr != nil {
			errs = append(errs, assocErr)
			continue
		}
		associated[r.MediaID] = asset
	}
	return associated, errors.Join(errs...)
}

// Load implements Store for FSStore: a validated read of a
// markdown-relative asset path — traversal-checked, symlink-vetted, and
// capped at MaxAssetSize.
func (s *FSStore) Load(path string) ([]byte, error) {
	content, err := s.readRef(path)
	if err != nil {
		return nil, fmt.Errorf("load asset: %w", err)
	}
	return content, nil
}
