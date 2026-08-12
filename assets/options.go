package assets

import (
	"context"

	adfast "github.com/pmarschik/adfast"
)

// MarkdownOptions bundles the md→ADF wiring for a store: asset-path →
// media-id resolution and local image dimension probing, so pushes keep
// attachment references intact. Path resolution is entirely the store's
// concern — a project-root or XDG-placed store resolves the same
// reference paths its Resolve emits.
func MarkdownOptions(store Store) []adfast.Option {
	return []adfast.Option{
		adfast.WithAssetIDResolver(IDResolver(store)),
		adfast.WithImageDimsResolver(store.Dims),
	}
}

// RenderOptions bundles the ADF→md wiring for a store: downloaded media
// render as local asset references.
//
// The store is asked about the media a document actually contains, one id at a
// time, rather than for its whole index. That matters because Resolve is not a
// passive read: an FSStore repairs the friendly file for the id it is asked
// about, next to the document being rendered. One index can serve every document
// in a repository, so rendering against the full map used to leave a copy of
// every asset the repository ever downloaded beside whichever document was
// being rendered.
func RenderOptions(store Store) []adfast.Option {
	return []adfast.Option{adfast.WithMediaAssetResolver(store.Resolve)}
}

// EnsureUploaded is the foolproof push-side entry point: it syncs
// pending assets through the uploader FIRST, then returns the markdown
// options wired to the now-complete store — encoding cannot observe an
// asset that could have been uploaded. Use it where a document is
// encoded for an actual push; pure conversions (diffs, previews) should
// use MarkdownOptions alone so they never trigger network I/O.
func EnsureUploaded(ctx context.Context, store Store, up Uploader) ([]adfast.Option, error) {
	if _, err := Sync(ctx, store, up); err != nil {
		return nil, err
	}
	return MarkdownOptions(store), nil
}
