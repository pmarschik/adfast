package adfast

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

// Pipeline is a configure-once bundle that carries one set of facade
// options across BOTH conversion directions and exposes the composed
// one-shot conveniences for the common paths. The package-level
// primitives (FromMarkdown, FromADF, ToADF, ToMarkdown) stay pure and
// take per-call options; a Pipeline registers the cross-cutting options
// once — a smart-link scheme, an extension bundle, a diagnostics sink —
// so the parse/encode and decode/render halves cannot drift apart, and
// adds the batched md→ADF flow (BeforeEncode hooks between parse and
// encode) that the primitives deliberately omit.
//
// The zero value is valid and behaves like the primitives without
// options. A Pipeline is immutable after construction and safe for
// concurrent use.
type Pipeline struct {
	opts  []Option
	hooks []BeforeEncode
}

// PipelineOption configures NewPipeline.
type PipelineOption func(*Pipeline)

// NewPipeline builds a Pipeline from the given options.
func NewPipeline(opts ...PipelineOption) *Pipeline {
	p := &Pipeline{}
	for _, o := range opts {
		o(p)
	}
	return p
}

// WithPipelineOptions registers facade options applied to every
// conversion the Pipeline runs, in both directions (each primitive reads
// the subset that applies to it; see Option).
func WithPipelineOptions(opts ...Option) PipelineOption {
	return func(p *Pipeline) { p.opts = append(p.opts, opts...) }
}

// BeforeEncode observes (and may act on) the parsed pivot ASTs of one
// MarkdownToADF/MarkdownToADFAll call before any document encodes — the
// event seam consumers hook side effects into. The assets package is one
// such consumer: assets.SyncOnEncode uploads every referenced pending
// asset as a single batch here. Hooks receive ALL documents of the call,
// so cross-document work batches naturally.
type BeforeEncode func(docs []ast.Node) error

// WithBeforeEncode appends hooks that fire on the parsed ASTs between
// FromMarkdown and ToADF. MarkdownToADF downgrades a hook error to a
// "before-encode-failed" diagnostic (the conversion is infallible);
// MarkdownToADFAll returns it.
func WithBeforeEncode(hooks ...BeforeEncode) PipelineOption {
	return func(p *Pipeline) { p.hooks = append(p.hooks, hooks...) }
}

// MarkdownToADF converts one Markdown document to ADF under the
// pipeline's configuration: parse, fire the BeforeEncode hooks on the
// parsed AST, then encode. A hook error becomes a "before-encode-failed"
// diagnostic and the conversion proceeds.
func (p *Pipeline) MarkdownToADF(md string) adf.Doc {
	root := FromMarkdown(md, p.opts...)
	if err := p.fireHooks([]ast.Node{root}); err != nil {
		if sink := newOptions(p.opts).diagnostics; sink != nil {
			sink(convert.Diagnostic{
				Code:    convert.CodeBeforeEncodeFailed,
				Message: "before-encode hook failed: " + err.Error(),
			})
		}
	}
	return ToADF(root, p.opts...)
}

// MarkdownToADFAll converts many documents in ONE call: every source is
// parsed first, the BeforeEncode hooks run once over the full set of
// parsed documents (e.g. uploading every referenced asset as a single
// batch), then each document encodes. A hook error aborts before any
// encoding. Documents are returned in input order.
func (p *Pipeline) MarkdownToADFAll(mds []string) ([]adf.Doc, error) {
	roots := make([]ast.Node, len(mds))
	for i, md := range mds {
		roots[i] = FromMarkdown(md, p.opts...)
	}
	if err := p.fireHooks(roots); err != nil {
		return nil, err
	}
	docs := make([]adf.Doc, len(mds))
	for i := range roots {
		docs[i] = ToADF(roots[i], p.opts...)
	}
	return docs, nil
}

// ADFToMarkdown converts an ADF document to Markdown under the pipeline's
// configuration (decode then render).
func (p *Pipeline) ADFToMarkdown(doc adf.Doc) string {
	return ToMarkdown(FromADF(doc, p.opts...), p.opts...)
}

// ADFBytesToMarkdown decodes ADF JSON bytes (or any decoded ADF value)
// and converts the document to Markdown. A value that is not an ADF
// document reports a "decode-failed" diagnostic (when a sink is
// configured) and renders "".
func (p *Pipeline) ADFBytesToMarkdown(v any) string {
	o := newOptions(p.opts)
	doc, ok := adf.DecodeDocOpts(v, adf.DecodeOptions{Diagnostics: o.diagnostics})
	if !ok {
		if o.diagnostics != nil {
			o.diagnostics(convert.Diagnostic{
				Code:    convert.CodeDecodeFailed,
				Message: "value could not be decoded into an ADF document; rendering empty output",
			})
		}
		return ""
	}
	return p.ADFToMarkdown(doc)
}

// Format reformats a Markdown document with prettier's prose-wrap rules
// under the pipeline's options; per-call options (WithPrintWidth, …) come
// after and may add to them. It is the composition
// ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat(), …).
func (p *Pipeline) Format(md string, opts ...Option) string {
	all := make([]Option, 0, len(p.opts)+len(opts)+1)
	all = append(all, WithPrettierFormat())
	all = append(all, p.opts...)
	all = append(all, opts...)
	return ToMarkdown(FromMarkdown(md, all...), all...)
}

func (p *Pipeline) fireHooks(docs []ast.Node) error {
	var live []ast.Node
	for _, d := range docs {
		if r, ok := d.(*ast.Root); ok && len(r.Children) == 0 {
			continue
		}
		live = append(live, d)
	}
	for _, h := range p.hooks {
		if err := h(live); err != nil {
			return err
		}
	}
	return nil
}
