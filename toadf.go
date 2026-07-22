package adfast

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

// ToADF encodes the pivot AST into an ADF document — the encode half of
// the ADF side, the inverse of FromADF at the AST boundary. It runs
// convert.ToADF (which canonicalizes as it projects the tree onto ADF's
// flat mark arrays), applies the spec-level text-newline normalization,
// then any configured document transforms in order.
//
// The common md→adf conversion is ToADF(FromMarkdown(md)); a leading
// ast.Frontmatter node has no ADF form and drops.
//
// Options read: WithSmartLinks, WithCodeLanguages, WithUnsupportedKinds,
// WithPreserveListTightness, WithImageDimsResolver, WithAssetIDResolver,
// WithExtensions, WithDocTransforms, and WithDiagnostics (colwidths-orphan,
// decisions-orphan, unresolved-asset, unsupported-code-language,
// unsupported-in-product, raw-node, depth-exceeded).
func ToADF(n ast.Node, opts ...Option) adf.Doc {
	o := newOptions(opts)
	doc := convert.ToADF(n, o.convertOptions()...)
	// Spec-level newline normalization always runs, then caller transforms.
	doc = adf.NormalizeTextNewlines(doc)
	for _, t := range o.docTransforms {
		doc = t(doc)
	}
	return doc
}
