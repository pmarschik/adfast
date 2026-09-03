package markdown

import (
	"slices"

	gast "github.com/yuin/goldmark/ast"
)

// HTMLSpans returns the byte span of every HTML block in src. See
// Source.HTMLSpans for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
//
// Inline HTML is a separate view: see InlineHTMLSpans.
func HTMLSpans(src []byte) Spans { return NewSource(src).HTMLSpans() }

// HTMLSpans returns the byte span of every HTML block — every CommonMark
// block whose content is raw HTML, a `<!-- comment -->` on its own line
// included — in document order, non-overlapping.
//
// This is the BLOCK half of the HTML surface, and InlineHTMLSpans is the
// inline half, split for the reason CodeSpans and InlineCodeSpans are: a
// block span names lines nothing may touch, an inline span names a run
// inside a line. The two together are every byte of a document the parser
// hands to an HTML renderer verbatim.
//
// A span covers WHOLE LINES, exactly as CodeSpans does and for the same
// reason. It starts at the first byte of the line the block opens on and
// ends just past the newline of the line it closes on, so it includes a
// blockquote's `>` and a list item's indent on those lines. It also
// includes any text written AFTER the closer on the closing line, because
// CommonMark says that text is part of the block: "<!-- x --> https://y"
// is one HTML block and the URL in it is not prose. Over-including a few
// bytes of container syntax is the safe direction for the only thing these
// spans are for — a rewriter deciding what NOT to touch — and no prose can
// live in the bytes it over-includes, because an HTML block starts a block
// and only its container's prefix can precede it on the line.
//
// What counts as an HTML block is goldmark's verdict, so it is the verdict
// the conversion path reaches. A pattern that looks for `<!--` and the next
// `-->` disagrees in both directions: a comment written inside prose is
// INLINE HTML and not a block, a blank line ends a `<div>` block before its
// closing tag (so the paragraph between them IS prose), and two comment
// lines with nothing between them are two blocks rather than one.
func (s *Source) HTMLSpans() Spans {
	if s.htmlDone {
		return s.html
	}
	s.html, s.htmlDone = collectHTMLSpans(s.doc, s.src), true
	return s.html
}

// collectHTMLSpans walks doc for HTML blocks and widens each to whole lines.
func collectHTMLSpans(doc gast.Node, src []byte) Spans {
	var out Spans
	for _, n := range blockNodes(doc) {
		block, isHTML := n.(*gast.HTMLBlock)
		if !isHTML {
			continue
		}
		if sp, ok := htmlBlockLineSpan(block, src); ok {
			out = append(out, sp)
		}
	}
	slices.SortFunc(out, func(a, b Span) int { return a.Start - b.Start })
	return clipOverlaps(out)
}

// htmlBlockLineSpan widens one HTML block to its whole-line source extent.
//
// The written extent comes from htmlBlockSpan, the same helper the
// conversion path reads a block's text with, so this view and the text it
// guards cannot disagree about where a block is.
func htmlBlockLineSpan(n *gast.HTMLBlock, src []byte) (Span, bool) {
	start, end := htmlBlockSpan(n)
	if start < 0 || end < start || end > len(src) {
		return Span{}, false
	}
	sp := Span{Start: lineStartBefore(src, start), Stop: wholeLineEnd(src, end)}
	if sp.Len() <= 0 {
		// A block that kept no line covers no byte a rewriter could touch,
		// so there is nothing to report.
		return Span{}, false
	}
	return sp, true
}

// InlineHTMLSpans returns the byte span of every run of inline HTML in src.
// See Source.InlineHTMLSpans for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
//
// HTML blocks are a separate view: see HTMLSpans.
func InlineHTMLSpans(src []byte) Spans { return NewSource(src).InlineHTMLSpans() }

// InlineHTMLSpans returns the byte span of every run of inline HTML — a tag
// written inside prose, `<span>` or `</span>`, and a `<!-- comment -->`
// written inside prose — in document order, non-overlapping.
//
// A span is TIGHT, and covers the tag or the comment as written and nothing
// around it. It is tight for the reason Source.InlineCodeSpans is: an inline
// lives inside a sentence, and a span widened to its line would name the
// sentence. So the CONTENT BETWEEN two tags is not covered — "<span>text
// </span>" reports the two tags and leaves the words between them to the
// prose views, which is what a rewriter wants: the words are prose and the
// tags are not.
//
// A comment is the case that matters most to a rewriter, and it is why a
// caller usually wants this view together with HTMLSpans. Text an author
// commented out reads as prose to any pattern over the raw source, so a
// rewriter without this view edits the one place in a document whose whole
// point is that it is not part of it — and the edit is silent, because the
// rendered output does not change.
//
// A run that crosses a line break is ONE span, and it covers the bytes
// between the two lines as written — a blockquote's `>` and a list item's
// indent included — the same safe over-inclusion Source.InlineCodeSpans
// documents for a code span crossing a line.
//
// What counts as inline HTML is goldmark's verdict, so it is the verdict the
// conversion path reaches: a `<span>` inside inline code or inside a code
// block is not inline HTML and is not reported here (CodeSpans and
// InlineCodeSpans are the views that cover those bytes), and neither is a
// `<` that opens nothing.
func (s *Source) InlineHTMLSpans() Spans {
	if s.inlineHTMLDone {
		return s.inlineHTML
	}
	s.inlineHTML, s.inlineHTMLDone = collectInlineHTMLSpans(s.doc, s.src), true
	return s.inlineHTML
}

// collectInlineHTMLSpans walks doc for raw inline HTML and resolves each run
// to its written extent.
//
// Like collectInlineCodeSpans this descends into inline subtrees, and is
// safe for the reason blockNodes documents: a text directive's label is
// parsed against its own detached source, so its root is not attached to
// this tree and cannot contribute a foreign offset.
func collectInlineHTMLSpans(doc gast.Node, src []byte) Spans {
	var out Spans
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if raw, isRaw := c.(*gast.RawHTML); isRaw {
				if sp, ok := rawHTMLSpan(raw, src); ok {
					out = append(out, sp)
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Span) int { return a.Start - b.Start })
	return clipOverlaps(out)
}

// rawHTMLSpan resolves one run of inline HTML to its written extent.
//
// goldmark records the run as text SEGMENTS, one per source line, and this
// takes the first segment's start through the last one's stop. The bytes
// between two segments are the line break and the container prefix after it,
// which is the over-inclusion InlineHTMLSpans documents.
func rawHTMLSpan(n *gast.RawHTML, src []byte) (Span, bool) {
	if n.Segments.Len() == 0 {
		return Span{}, false
	}
	sp := Span{
		Start: n.Segments.At(0).Start,
		Stop:  n.Segments.At(n.Segments.Len() - 1).Stop,
	}
	if sp.Start < 0 || sp.Stop > len(src) || sp.Len() <= 0 {
		return Span{}, false
	}
	return sp, true
}
