package adfast

import (
	"fmt"
	"strings"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/markdown"
)

// FromMarkdown parses a Markdown string into the pivot AST — the parse
// half of the md side. It performs line-ending normalization, splits
// leading document metadata with a FrontmatterProvider (see
// WithFrontmatterProvider; a found block is kept as a leading
// ast.Frontmatter node), runs the goldmark parse with the dialect and
// directive extensions, and promotes directive names into their typed
// extension nodes.
//
// The result is the FAITHFUL parse tree, NOT the canonical form: it is
// the currency the To* primitives consume. It is a SINGLE parse for both
// directions — text values are fully decoded (the ADF currency) while the
// prettier formatter's literal escapes ride separately on ast.Text.Raw as
// escape provenance (see markdown.PreservedEscapes), so nothing collides.
// md→adf is ToADF(FromMarkdown(md)); the prettier md→md formatter is
// ToMarkdown(FromMarkdown(md), WithPrettierFormat(), …) — the format flag
// is read only on the render call and has NO parse-side effect, since the
// same FrontmatterProvider serves both directions.
//
// A FrontmatterProvider that reports FrontmatterMalformed (the document
// opens the convention but no valid block forms) keeps the whole source as
// body and emits a malformed-frontmatter diagnostic; the parse stays
// infallible.
//
// Options read: WithExtensions, WithFrontmatterProvider and WithDiagnostics
// (parse-recovered, malformed-frontmatter, depth-exceeded and
// span-marker-invalid notices).
func FromMarkdown(md string, opts ...Option) ast.Node {
	return parseMarkdownSource(md, newOptions(opts))
}

// parseMarkdownSource is FromMarkdown's body with the options already
// resolved and extra parse options appended, so a second facade entry
// (PlainTextOf, which parses with markdown.WithGenericDirectives) reuses the
// identical line-ending, frontmatter and diagnostics handling instead of
// restating it.
func parseMarkdownSource(md string, o options, extra ...markdown.ParseOption) ast.Node {
	// CommonMark line-ending normalization: remark treats a lone CR as a
	// line ending; goldmark does not, which would leave raw \r bytes inside
	// text nodes.
	if strings.ContainsRune(md, '\r') {
		md = strings.ReplaceAll(md, "\r\n", "\n")
		md = strings.ReplaceAll(md, "\r", "\n")
	}

	provider := o.frontmatter
	if provider == nil {
		provider = defaultFrontmatterProvider
	}
	front, source := "", md
	switch f, rest, outcome := provider(md); outcome {
	case FrontmatterFound:
		front, source = f, rest
	case FrontmatterMalformed:
		if o.diagnostics != nil {
			o.diagnostics(convert.Diagnostic{
				Code:    convert.CodeMalformedFrontmatter,
				Message: "document opens a frontmatter fence but does not close it validly; the block is kept as body",
			})
		}
	case FrontmatterAbsent:
		// No metadata block: the whole source is body (front stays "").
	}

	var root *ast.Root
	if strings.TrimSpace(source) == "" {
		root = &ast.Root{}
	} else {
		parseOpts := []markdown.ParseOption{}
		if len(o.extensions) > 0 {
			parseOpts = append(parseOpts, markdown.WithExtensions(o.extensions...))
		}
		parseOpts = append(parseOpts, parseNoticeOptions(o.diagnostics)...)
		parseOpts = append(parseOpts, extra...)
		var ok bool
		root, ok = markdown.Parse([]byte(source), parseOpts...).(*ast.Root)
		if !ok {
			// markdown.Parse always returns an *ast.Root; the guard keeps the
			// type assertion checked.
			root = &ast.Root{}
		}
	}
	if front != "" {
		root.Children = append([]ast.Node{&ast.Frontmatter{Value: front}}, root.Children...)
	}
	return root
}

// parseNoticeOptions wires a diagnostics sink into the parse-level
// notices (recovered parses, depth caps, invalid table span markers);
// a nil sink produces no options.
func parseNoticeOptions(sink func(convert.Diagnostic)) []markdown.ParseOption {
	if sink == nil {
		return nil
	}
	return []markdown.ParseOption{
		markdown.WithRecoverNotice(func() {
			sink(convert.Diagnostic{
				Code:    convert.CodeParseRecovered,
				Message: "goldmark parser panicked; recovered by re-parsing a normalized source",
			})
		}),
		markdown.WithDepthExceededNotice(func() {
			sink(convert.Diagnostic{
				Code:    convert.CodeDepthExceeded,
				Message: "markdown nesting exceeds the parse depth cap; deeper content truncated",
			})
		}),
		markdown.WithTableSpanNotice(func(marker string, row, col int) {
			direction := "no content cell to its right in the row"
			if marker == "^" {
				direction = "no spanning cell above its column"
			}
			sink(convert.Diagnostic{
				Code: convert.CodeSpanMarkerInvalid,
				Message: fmt.Sprintf(
					"table span marker %q at row %d, column %d has %s; kept as literal text",
					marker, row, col, direction,
				),
			})
		}),
	}
}
