package adfast

import (
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/markdown"
)

// ToMarkdown renders the pivot AST to a Markdown string — the render half
// of the md side, the inverse of FromMarkdown at the AST boundary. By
// default it is a pure projection of the tree it is given (adf→md is
// ToMarkdown(FromADF(doc))): the faithful AST renders faithfully, so an
// ADF-shaped tree that cannot arise from a Markdown parse still round
// trips.
//
// WithPrettierFormat switches on the prettier md→md formatter: the tree is
// first canonicalized with convert.Normalize (the single canonicalization
// pass) and text serialization uses prettier's rules. That is applied
// only in this mode — the plain adf→md render must not disturb the tree —
// so the format composition is
// ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat(),
// WithPrintWidth(w)).
//
// Options read: WithPrintWidth, WithNoWrap, WithBlockSeparator,
// WithPrettierFormat, WithoutSignificantSpaceEscapes, and (in the format
// mode only) the convert.Normalize
// options WithSmartLinks, WithMediaAssets or WithMediaAssetResolver,
// WithCodeLanguages, WithExtensions
// and WithDiagnostics.
func ToMarkdown(n ast.Node, opts ...Option) string {
	o := newOptions(opts)
	if o.prettier {
		n = convert.Normalize(n, o.convertOptions()...)
		// AST transforms are the formatter's content-rewrite seam (see
		// WithASTTransforms); they run only in the prettier-format mode,
		// on the canonicalized tree, matching the documented contract.
		for _, t := range o.astTransforms {
			t(n)
		}
	}
	return markdown.Render(n, o.renderOptions()...)
}

// renderOptions builds the markdown render options from the resolved
// facade options.
func (o *options) renderOptions() []markdown.RenderOption {
	var out []markdown.RenderOption
	if o.printWidth != nil {
		out = append(out, markdown.WithPrintWidth(*o.printWidth))
	}
	if o.noWrap {
		out = append(out, markdown.WithNoWrap())
	}
	if o.blockSep != nil {
		out = append(out, markdown.WithBlockSeparator(*o.blockSep))
	}
	if o.prettier {
		out = append(out, markdown.WithPrettierText())
	}
	if o.noSpaceEscapes {
		out = append(out, markdown.WithoutSignificantSpaceEscapes())
	}
	return out
}
