package markdown

import (
	"slices"

	gast "github.com/yuin/goldmark/ast"
)

// InlineCodeSpans returns the byte span of every inline code span in src.
// See Source.InlineCodeSpans for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func InlineCodeSpans(src []byte) Spans { return NewSource(src).InlineCodeSpans() }

// InlineCodeSpans returns the byte span of every inline code span — the
// backtick kind, `like this` — in document order, non-overlapping.
//
// This is the INLINE half of what CodeSpans reports as blocks, and the two
// are separate views because a caller wants different things from them: a
// block span names lines nothing may touch, an inline span names a run
// inside a line. Together they are every byte a document means literally.
//
// A span is TIGHT, and covers the WRITTEN form: the opening backtick run,
// the content, and the closing run. It is tight for the reason
// Source.Images documents — an inline lives inside prose, and a span
// widened to its line would name the sentence around it. The delimiters are
// inside the span because a rewriter asking "may I touch this byte" must be
// told yes for prose and no for the backticks that make it code.
//
// A code span that runs over a line break is one span, and it covers the
// bytes between the two lines as written — a blockquote's `>` and a list
// item's indent included. That over-includes container syntax by a few
// bytes, which is the same safe direction CodeSpans over-includes in: no
// prose lives there.
//
// What counts as inline code is goldmark's verdict, so it is the verdict
// the conversion path reaches. A backtick-parity count over a line
// disagrees with it in both directions — a run of three backticks closes
// only a run of three, an escaped backtick opens nothing, and a span may
// cross a line the count never leaves — and the disagreements that
// UNDER-report are the ones that let a rewriter into code.
func (s *Source) InlineCodeSpans() Spans {
	if s.inlineCodeDone {
		return s.inlineCode
	}
	s.inlineCode, s.inlineCodeDone = collectInlineCodeSpans(s.doc, s.src), true
	return s.inlineCode
}

// collectInlineCodeSpans walks doc for code spans and resolves each to its
// written extent.
//
// Like collectImages this descends into inline subtrees, and is safe for the
// reason blockNodes documents: a text directive's label is parsed against its
// own detached source, so its root is not attached to this tree and cannot
// contribute a foreign offset.
func collectInlineCodeSpans(doc gast.Node, src []byte) Spans {
	var out Spans
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if code, ok := c.(*gast.CodeSpan); ok {
				if sp, ok := inlineCodeSpan(code, src); ok {
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

// inlineCodeSpan resolves one code span to its written extent.
//
// goldmark records no position on the node itself: a CodeSpan carries only
// its content, as raw text segments, and the delimiter runs are consumed
// without being kept. So the content is the anchor and the two runs are
// re-read from the source around it.
//
// The runs are found rather than assumed, and the span falls back to the
// content alone when they cannot be: a guard built on the content is still
// a correct guard, and a delimiter offset that is a guess is not.
func inlineCodeSpan(n *gast.CodeSpan, src []byte) (Span, bool) {
	first, isFirstText := n.FirstChild().(*gast.Text)
	last, isLastText := n.LastChild().(*gast.Text)
	if !isFirstText || !isLastText {
		// An empty code span (`` `` ``) keeps no segment at all. It holds no
		// byte a rewriter could touch, so there is nothing to report.
		return Span{}, false
	}
	content := Span{Start: first.Segment.Start, Stop: last.Segment.Stop}
	if content.Start < 0 || content.Stop > len(src) || content.Len() < 0 {
		return Span{}, false
	}
	open, opened := backtickRunBefore(src, content.Start)
	closing, closed := backtickRunAfter(src, content.Stop)
	// goldmark closes a run only on one of the same length, so unequal runs
	// mean the scan found something that is not this span's delimiters.
	if !opened || !closed || content.Start-open != closing-content.Stop {
		return content, true
	}
	return Span{Start: open, Stop: closing}, true
}

// backtickRunBefore returns the offset the delimiter run before at begins on.
//
// At most ONE space or newline can sit between the run and the content: the
// parser trims a single one from each end, and only when both ends have one.
// Anything else the content begins with is content.
func backtickRunBefore(src []byte, at int) (int, bool) {
	i := at
	if i > 0 && (src[i-1] == ' ' || src[i-1] == '\n') {
		i--
	}
	run := i
	for run > 0 && src[run-1] == '`' {
		run--
	}
	return run, run < i
}

// backtickRunAfter returns the offset one past the delimiter run that follows
// at, under the same one-character rule backtickRunBefore documents.
func backtickRunAfter(src []byte, at int) (int, bool) {
	i := at
	if i < len(src) && (src[i] == ' ' || src[i] == '\n') {
		i++
	}
	run := i
	for run < len(src) && src[run] == '`' {
		run++
	}
	return run, run > i
}
