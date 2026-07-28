package assets

import (
	"context"
	"strings"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/ast"
)

// SyncOnEncode makes a Pipeline trigger the uploader itself: as a
// BeforeEncode hook it receives the parsed documents, collects the local
// assets they reference, and uploads the pending ones as ONE batch
// before anything encodes. Only referenced assets go up — a scratch file
// in the assets folder stays pending. Wire it with PushPipeline (or
// adfast.WithBeforeEncode directly).
//
// With Pipeline.MarkdownToADFAll an upload failure aborts the
// conversion; the infallible Pipeline.MarkdownToADF downgrades it to a
// "before-encode-failed" diagnostic. Pure conversions (diffs, previews)
// should use MarkdownOptions alone so they never touch the network.
func SyncOnEncode(ctx context.Context, store Store, up Uploader) adfast.BeforeEncode {
	return func(docs []ast.Node) error {
		return syncReferenced(ctx, store, up, docs)
	}
}

// PushPipeline builds the full push-side pipeline: MarkdownOptions (plus
// any extra options) with SyncOnEncode wired as a BeforeEncode hook, so
// its MarkdownToADF/MarkdownToADFAll upload referenced pending assets in
// one batch before encoding.
func PushPipeline(ctx context.Context, store Store, up Uploader, extra ...adfast.Option) *adfast.Pipeline {
	return adfast.NewPipeline(
		adfast.WithPipelineOptions(append(MarkdownOptions(store), extra...)...),
		adfast.WithBeforeEncode(SyncOnEncode(ctx, store, up)),
	)
}

// SyncMarkdown uploads the pending assets referenced by a single markdown
// document as one batch. It parses md, collects its local image references,
// and uploads the intersection with the store's pending worklist — a
// convenience for callers that encode one markdown string at a time and
// cannot use a Pipeline/BeforeEncode hook. Nothing uploads when the document
// references no pending assets.
func SyncMarkdown(ctx context.Context, store Store, up Uploader, md string) error {
	return syncReferenced(ctx, store, up, []ast.Node{adfast.FromMarkdown(md)})
}

// syncReferenced uploads the intersection of the documents' local image
// references and the store's pending worklist as one batch.
func syncReferenced(ctx context.Context, store Store, up Uploader, docs []ast.Node) error {
	pending, err := store.Pending("")
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	referenced := map[string]bool{}
	for _, doc := range docs {
		collectLocalImages(doc, referenced)
	}
	var paths []string
	for _, p := range pending {
		if referenced[p] {
			paths = append(paths, p)
		}
	}
	_, err = uploadPaths(ctx, store, up, paths)
	return err
}

// collectLocalImages walks a parsed document for image destinations that
// are local paths (not URLs) — the references an upload could resolve.
func collectLocalImages(n ast.Node, out map[string]bool) {
	if img, ok := n.(*ast.Image); ok && img.URL != "" && !isRemoteURL(img.URL) {
		out[img.URL] = true
	}
	for _, c := range ast.Children(n) {
		collectLocalImages(c, out)
	}
}

func isRemoteURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "data:")
}
