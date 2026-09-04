package assets

import (
	"path"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
)

// RewriteReferences returns the option that re-paths the local pictures
// of a document to a store's current reference paths — the markdown
// rewrite facility for layout changes (local folder → shared root,
// fused → split, …). All four spellings of a picture follow the file:
// ![alt](path), ::media, :::media and the inline :media chip. It composes
// with the prettier format mode, so user formatting survives:
//
//	out := adfast.ToMarkdown(adfast.FromMarkdown(md,
//		adfast.WithPrettierFormat(), assets.RewriteReferences(old, moved)),
//		adfast.WithPrettierFormat(), assets.RewriteReferences(old, moved))
//
// Each destination maps to its media id through the old store's
// content-addressed Lookup; when the referenced file no longer exists
// at the old path (the folder physically moved — pass nil for from), a
// unique-basename match against the new store's assets is the
// fallback. Destinations that map nowhere stay untouched.
//
// The canonical pipeline needs none of this: rendering with
// RenderOptions(store) always emits the store's current paths.
func RewriteReferences(from, to Store) adfast.Option {
	return adfast.WithASTTransforms(func(root ast.Node) {
		rewriteNodes(root, from, to)
	})
}

// rewriteNodes re-paths every local picture in the tree: the image nodes
// and the media directives, and the definitions a reference-style image
// resolves to — those hold the destination in their stead (see
// linkref.go). A media directive has no reference form to follow.
func rewriteNodes(root ast.Node, from, to Store) {
	rewriteImageNodes(root, from, to)
	for _, def := range imageRefDefinitions(root) {
		def.URL = rewrittenDest(def.URL, from, to)
	}
}

// rewriteImageNodes is rewriteNodes' recursion over the nodes that name a
// local file: the markdown image nodes, and the media directives.
//
// A picture has FOUR spellings — ![alt](path), ::media, :::media and the
// inline :media chip — and this walk used to match ast.Image alone, the
// same blind spot the upload scan had (collectImageNodes). So a layout
// change re-pathed the image form and left the three directive spellings
// pointing at where the file used to be: the encode resolved the stale
// path to nothing and shipped a media node with an EMPTY id, and the
// author was told the path could not be resolved with no hint that a
// layout change is what broke it. Two spellings of one picture in one
// document diverged silently.
//
// The attribute write is dialect's, not this walk's
// (dialect.RewriteMediaPath): the path lives in the directive's raw
// attribute map, and only the dialect knows which media kinds also bind
// it to a typed field. That function also answers for MORE directives
// than the upload scan's dialect.MediaLookupPath — a path beside a
// pinned id, or on external media, is not a lookup key but is still a
// path that moved. See its doc comment for why the two disagree.
func rewriteImageNodes(n ast.Node, from, to Store) {
	switch node := n.(type) {
	case *ast.Image:
		node.URL = rewrittenDest(node.URL, from, to)
	default:
		dialect.RewriteMediaPath(n, func(path string) string {
			return rewrittenDest(path, from, to)
		})
	}
	for _, c := range ast.Children(n) {
		rewriteImageNodes(c, from, to)
	}
}

// rewrittenDest answers the store's current path for one local
// destination, or the destination unchanged when it maps nowhere.
func rewrittenDest(url string, from, to Store) string {
	if url == "" || isRemoteURL(url) {
		return url
	}
	if p, ok := currentPath(url, from, to); ok {
		return p
	}
	return url
}

// currentPath maps an image destination to the new store's reference
// path for the same content.
func currentPath(url string, from, to Store) (string, bool) {
	if from != nil {
		if id, ok := from.Lookup("", url); ok {
			if asset, ok := to.Resolve(id); ok {
				return asset.Path, true
			}
		}
	}
	// Fallback for physically moved files: a unique basename match in
	// the new store.
	base, match, matches := path.Base(url), "", 0
	for _, asset := range to.Assets() {
		if path.Base(asset.Path) == base {
			match = asset.Path
			matches++
		}
	}
	if matches == 1 {
		return match, true
	}
	return "", false
}
