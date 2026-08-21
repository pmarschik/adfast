package adfast

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

// FromADF converts an ADF document into the pivot AST — the decode half
// of the ADF side, the inverse of ToADF at the AST boundary. It rebuilds
// the nested inline mark wrappers ADF stores as flat arrays and lifts the
// known dialect shapes into their typed nodes.
//
// The common adf→md conversion is ToMarkdown(FromADF(doc)). JSON bytes or
// a decoded map first go through adf.DecodeDoc (or the Pipeline byte
// helper), which owns the decode-failed/unknown-node/unknown-mark/
// unknown-attr diagnostics.
//
// Options read: WithADFTransforms (applied to the document first),
// WithExtensions, WithSmartLinks (KeyFromURL card labels), WithMediaAssets
// or WithMediaAssetResolver, and WithDiagnostics (the raw-node projection
// notice).
func FromADF(doc adf.Doc, opts ...Option) ast.Node {
	o := newOptions(opts)
	// Caller transforms first: they rewrite product-specific shapes into
	// the ones the conversion understands (see WithADFTransforms).
	for _, t := range o.adfTransforms {
		doc = t(doc)
	}
	return convert.FromADF(doc, o.convertOptions()...)
}
