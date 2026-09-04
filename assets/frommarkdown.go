package assets

import (
	"context"
	"maps"
	"path"
	"path/filepath"
	"slices"
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
// and uploads the ones PendingRefs keeps — a convenience for callers that
// encode one markdown string at a time and cannot use a Pipeline/BeforeEncode
// hook. Nothing uploads when the document references no pending assets.
func SyncMarkdown(ctx context.Context, store Store, up Uploader, md string) error {
	return syncReferenced(ctx, store, up, []ast.Node{adfast.FromMarkdown(md)})
}

// syncReferenced uploads the documents' local image references that the
// store can read and holds no media id for, as one batch.
//
// The narrowing is PendingRefs — the documents' own references asked about
// one by one — rather than an intersection with Store.Pending. A folder
// worklist can only offer what it lists, and a composed store may reach
// further than it lists on purpose; starting from the references keeps a
// picture whose file the store can read out of the "dropped, no media id"
// case it never belonged in.
func syncReferenced(ctx context.Context, store Store, up Uploader, docs []ast.Node) error {
	referenced := map[string]bool{}
	for _, doc := range docs {
		collectLocalImages(doc, referenced)
	}
	if len(referenced) == 0 {
		return nil
	}
	_, err := uploadPaths(ctx, store, up, PendingRefs(store, "", slices.Sorted(maps.Keys(referenced))))
	return err
}

// collectLocalImages walks a parsed document for image destinations that
// are local paths (not URLs) — the references an upload could resolve.
// The destinations are normalized, because the worklist they meet is.
//
// A reference-style image keeps its destination on the definition, so the
// definitions its labels resolve to are collected too (see linkref.go);
// without that pass "![logo]" would never upload its file.
func collectLocalImages(root ast.Node, out map[string]bool) {
	collectImageNodes(root, out)
	for _, def := range imageRefDefinitions(root) {
		collectLocalDest(def.URL, out)
	}
}

// collectImageNodes is collectLocalImages' recursion over the inline
// image nodes.
func collectImageNodes(n ast.Node, out map[string]bool) {
	if img, ok := n.(*ast.Image); ok {
		collectLocalDest(img.URL, out)
	}
	for _, c := range ast.Children(n) {
		collectImageNodes(c, out)
	}
}

// collectLocalDest records one destination when it is a local path.
func collectLocalDest(url string, out map[string]bool) {
	if url != "" && !isRemoteURL(url) {
		out[normalizeRef(url)] = true
	}
}

// normalizeRef reduces a reference path to one spelling per file, so an
// author's "./assets/x.png" meets the worklist's "assets/x.png". Pending
// reports the path the store builds itself — always clean — while a
// document says whatever its author typed.
func normalizeRef(ref string) string {
	return path.Clean(filepath.ToSlash(ref))
}

func isRemoteURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "data:")
}
