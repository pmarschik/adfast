package adfast

import (
	"strings"

	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/markdown"
)

// The leading-metadata boundary, exposed on its own. The vocabulary it
// speaks — FrontmatterProvider, FrontmatterOutcome and the built-in YAML
// convention — lives in options.go; this file holds the ONE function that
// runs a provider, and the entry point that lets a caller run it without
// a conversion.
//
// splitFrontmatterSource is that one function: FromMarkdown reaches it
// through parseMarkdownSource and SplitFrontmatter is a thin wrapper over
// it, so there is no second splitter to drift. That mattered before: the
// prettier formatter once installed a parse-side splitter of its own whose
// detection disagreed with the md→adf default on pathological delimiter
// shapes (see WithPrettierFormat), and the fix was to collapse the two
// into one path rather than to keep them in step by hand.

// FrontmatterSplit is what SplitFrontmatter found at the head of a
// Markdown document.
//
// Front and Body partition the document: Front+Body is the source with
// its line endings normalized to "\n" and any leading byte order mark
// peeled, whatever the Outcome. Nothing is dropped and nothing is added.
type FrontmatterSplit struct {
	// Front is the raw metadata block, its delimiters and trailing
	// newline included, exactly as ast.Frontmatter.Value carries it. It
	// is "" unless Outcome is FrontmatterFound — a block that did not
	// form validly stays in Body rather than being extracted.
	Front string
	// Body is the document with Front removed: the whole (normalized)
	// source when Outcome is not FrontmatterFound. This is the string
	// FromMarkdown hands to the Markdown parser.
	Body string
	// Outcome classifies what the provider found; see FrontmatterOutcome.
	// FrontmatterMalformed is the case a present-or-absent bool cannot
	// express, and the reason this field is here.
	Outcome FrontmatterOutcome
	// ByteOrderMark reports that the source opened with a UTF-8 byte
	// order mark. The mark is an encoding artifact rather than content,
	// so it is peeled before the provider sees the source (and before it
	// is counted as part of Body); FromMarkdown records the same fact on
	// ast.Root.ByteOrderMark and ToMarkdown writes it back. A caller
	// reassembling a document from Front and Body has to prepend
	// markdown.ByteOrderMark again when this is set.
	ByteOrderMark bool
}

// Found reports whether a metadata block was extracted, which is the only
// case where Front is non-empty. Prefer it to a Front != "" test: that
// test happens to be equivalent today but reads as a statement about the
// block's contents rather than about the boundary.
func (s FrontmatterSplit) Found() bool { return s.Outcome == FrontmatterFound }

// SplitFrontmatter answers "where does the leading metadata block end"
// for a Markdown source, without converting anything.
//
// It runs the SAME code path FromMarkdown runs — the byte order mark peel,
// the line-ending normalization, and then the FrontmatterProvider — so its
// answer is by construction the one a conversion or a format acts on. Use
// it instead of hand-rolling a "---" scan (which has to agree on a mark, a
// CR line ending, an indented fence, an unterminated block and a leading
// "---" that is really a thematic break) and instead of round-tripping the
// document through ADF just to read the block back off the tree.
//
// Options read: WithFrontmatterProvider (a caller-defined convention
// replaces the YAML default) and WithDiagnostics (a FrontmatterMalformed
// outcome emits the same malformed-frontmatter notice FromMarkdown emits,
// so a caller that already classifies diagnostics needs no second rule for
// it). Every other option is ignored: no parse and no conversion runs.
//
// A found block is opaque bytes. For structured access to a YAML one, hand
// Front to the frontmatter submodule (frontmatter.Parse); the core stays
// YAML-neutral.
func SplitFrontmatter(md string, opts ...Option) FrontmatterSplit {
	return splitFrontmatterSource(md, newOptions(opts))
}

// splitFrontmatterSource clears the two decoding artifacts that are not
// Markdown and then runs the configured FrontmatterProvider over what is
// left. It is the whole of FromMarkdown's pre-parse preamble, kept in one
// function so the exported split and the parse cannot disagree.
func splitFrontmatterSource(md string, o options) FrontmatterSplit {
	// A leading UTF-8 byte order mark is a decoding artifact, not content.
	// Peeled here, before the frontmatter provider and the goldmark parse
	// see the source: left in place it is ordinary text at the start of
	// line 1, so the first block misparses (a heading degrades to a
	// paragraph, a list's first item splits off, a fence is reflowed) and
	// a provider would need its own mark tolerance to find the block.
	// ast.Root carries the fact so the render prepends it again; adfast
	// does not silently change a document's encoding preamble.
	bom := strings.HasPrefix(md, markdown.ByteOrderMark)
	md = strings.TrimPrefix(md, markdown.ByteOrderMark)

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
	split := FrontmatterSplit{Body: md, ByteOrderMark: bom}
	switch f, rest, outcome := provider(md); outcome {
	case FrontmatterFound:
		split.Front, split.Body, split.Outcome = f, rest, FrontmatterFound
	case FrontmatterMalformed:
		// The opening bytes stay in Body (Front and Body keep the values
		// above), so a broken block is preserved rather than dropped; the
		// diagnostic is the only trace that it looked like one. A provider
		// that returns something other than ("", md) here is ignored on
		// purpose — the outcome, not the provider, decides.
		split.Outcome = FrontmatterMalformed
		if o.diagnostics != nil {
			o.diagnostics(convert.Diagnostic{
				Code:    convert.CodeMalformedFrontmatter,
				Message: "document opens a frontmatter fence but does not close it validly; the block is kept as body",
			})
		}
	case FrontmatterAbsent:
		// No metadata block: the whole source is body (Front stays "").
		split.Outcome = FrontmatterAbsent
	}
	return split
}
