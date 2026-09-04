package assets

import (
	"context"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/convert"
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

// ReportMarkdownOptions is MarkdownOptions for a report — the md→ADF
// half of the pair ReportOptions completes. It resolves through
// Store.Report, so the document reads as the document it is even where
// its assets folder no longer holds the file: the id behind a reference
// and the reference's dimensions come from the blobs.
//
// Both halves of a round trip need it, not just the rendering one. A
// comparison puts the local markdown through md→ADF and the remote
// document through ADF→md, and a reference the local half cannot
// resolve is not the same node as the media the remote half renders —
// MEASURED, an image whose file was missing normalized to a LINK on the
// local side against an image on the remote one, a change nobody made.
// The repair used to hide that by putting the file back mid-comparison,
// which made the verdict depend on which side ran first.
func ReportMarkdownOptions(store Store) []adfast.Option {
	lookup := func(path string) (id string, ok bool) {
		store.Report(func() { id, ok = store.Lookup("", path) })
		return id, ok
	}
	dims := func(path string) (width, height int, ok bool) {
		store.Report(func() { width, height, ok = store.Dims(path) })
		return width, height, ok
	}
	return []adfast.Option{
		adfast.WithAssetIDResolver(lookup),
		adfast.WithImageDimsResolver(dims),
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

// ReportOptions is RenderOptions for a render nobody keeps: a diff, a
// preview, a dry run. It resolves media through Store.Report, so
// rendering a document does not repair the assets folder beside it.
//
// The distinction is not cosmetic. Rendering is how these commands
// decide whether a document CHANGED, and the repair is a change —
// reporting on a document whose assets folder somebody cleaned would
// silently put the files back, which is exactly what a caller running
// without permission to write must not do. The rendered markdown is
// the same either way: report mode still answers the reference path
// and the dimensions, from the blobs.
//
// Use RenderOptions where the result is written (a pull, a format), and
// this where it is only shown. Where the report also reads local
// markdown — a diff normalizes both sides — pair it with
// ReportMarkdownOptions, whose comment says why the pair has to match.
func ReportOptions(store Store) []adfast.Option {
	resolve := func(mediaID string) (asset convert.MediaAsset, ok bool) {
		store.Report(func() { asset, ok = store.Resolve(mediaID) })
		return asset, ok
	}
	return []adfast.Option{adfast.WithMediaAssetResolver(resolve)}
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
