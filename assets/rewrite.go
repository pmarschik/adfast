package assets

import (
	"path"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/ast"
)

// RewriteReferences returns the option that re-paths local image
// destinations to a store's current reference paths — the markdown
// rewrite facility for layout changes (local folder → shared root,
// fused → split, …). It composes with the prettier format mode, so user
// formatting survives:
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

func rewriteNodes(n ast.Node, from, to Store) {
	if img, ok := n.(*ast.Image); ok && img.URL != "" && !isRemoteURL(img.URL) {
		if p, ok := currentPath(img.URL, from, to); ok && p != img.URL {
			img.URL = p
		}
	}
	for _, c := range ast.Children(n) {
		rewriteNodes(c, from, to)
	}
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
