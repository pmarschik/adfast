package assets

import "github.com/pmarschik/adfast/ast"

// Reference-style images ("![logo]" with "[logo]: ./logo.png" elsewhere)
// keep their destination on the definition, not on the image node, so the
// two walks in this package — the upload collector and the path rewriter —
// would look straight past them if they only matched ast.Image. Both
// therefore ask imageRefDefinitions which definitions an image in the
// document actually resolves to.
//
// It is the IMAGE references that matter, not every definition: a
// definition a link uses is a document the author pointed at, and
// uploading or re-pathing that would treat a link like an embed.

// imageRefDefinitions returns the definitions some reference-style image
// in the tree resolves to, in document order and without duplicates.
// Pairing is on the normalized label, and a label defined twice resolves
// to the first definition — CommonMark's rule, and the one the markdown
// parse already applied.
func imageRefDefinitions(root ast.Node) []*ast.Definition {
	used := map[string]bool{}
	byLabel := map[string]*ast.Definition{}
	var order []*ast.Definition
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		switch v := n.(type) {
		case *ast.ImageRef:
			used[v.Identifier()] = true
		case *ast.Definition:
			key := v.Identifier()
			if _, seen := byLabel[key]; !seen {
				byLabel[key] = v
				order = append(order, v)
			}
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	walk(root)
	out := make([]*ast.Definition, 0, len(order))
	for _, def := range order {
		if used[def.Identifier()] {
			out = append(out, def)
		}
	}
	return out
}
