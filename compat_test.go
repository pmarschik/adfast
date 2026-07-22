package adfast

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// Test-only composition helpers mirroring the pre-redesign one-call
// facade, so the existing suite exercises the new primitives through the
// exact old shapes. Behavior is identical to the deleted FromMarkdown
// (→adf.Doc), ToMarkdown(any), FromMarkdownAll, and FormatMarkdown.

func mdToADF(md string, opts ...Option) adf.Doc {
	return ToADF(FromMarkdown(md, opts...), opts...)
}

func adfToMD(v any, opts ...Option) string {
	o := newOptions(opts)
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
	return ToMarkdown(FromADF(doc, opts...), opts...)
}

func fmtMD(md string, opts ...Option) string {
	all := append([]Option{WithPrettierFormat()}, opts...)
	return ToMarkdown(FromMarkdown(md, all...), all...)
}
