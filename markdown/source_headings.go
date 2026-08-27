package markdown

import (
	"slices"

	gast "github.com/yuin/goldmark/ast"
)

// Heading is one heading of a Markdown source: its level, the bytes of its
// text as written, and the bytes the heading occupies.
//
// ATX (`## Title`) and setext (`Title` over `-----`) headings are both
// reported, and a consumer cannot tell them apart from a Heading alone —
// deliberately, because "which headings are there, and where" is the fact
// this view supplies. Which of them is a title, a section, or a field is
// the consumer's policy.
//
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type Heading struct {
	// Span covers the heading's WHOLE LINES: it starts at the first byte
	// of the line the heading opens on and ends just past the newline of
	// the line it ends on — for a setext heading, the underline line, not
	// the text line. Inside a blockquote or a list item the container's
	// prefix on those lines is part of the span.
	//
	// This is the same over-including rule Source.CodeSpans documents, and
	// it is deliberate here for a second reason: the consumer this view
	// exists for splits a document into the runs BETWEEN headings, and a
	// span that stopped short of the line's end would leave a fragment of
	// the heading's own syntax at the head of the following section.
	Span Span
	// Text covers the heading's text exactly as goldmark reads it, and is
	// TIGHT: the `#` markers, the space after them, an ATX heading's
	// optional closing `###` run, the surrounding whitespace, and a setext
	// heading's underline are all outside it. Slicing it yields the raw
	// written text — inline markup included, nothing transformed — which
	// is what a consumer that must round-trip a heading verbatim needs.
	//
	// A heading with no text at all (`#`) has an empty Text positioned
	// where its text would go, so an Edit there inserts one.
	Text Span
	// Level is 1 through 6.
	Level int
}

// Headings returns every heading in src, in document order. See
// Source.Headings for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func Headings(src []byte) []Heading { return NewSource(src).Headings() }

// Headings returns every heading in the source, in document order.
//
// What counts as a heading is goldmark's verdict, so it is the same verdict
// the conversion path reaches. A `#` line inside a code block is not a
// heading and does not appear; neither does `#no-space`, which CommonMark
// reads as a paragraph. A setext heading IS one, which the line scanners
// this view replaces never saw at all.
//
// Headings never overlap, and neither Span nor Text is ever empty except
// for the documented empty-text case.
func (s *Source) Headings() []Heading {
	if s.headingsDone {
		return s.headings
	}
	s.headings, s.headingsDone = collectHeadings(s.doc, s.src), true
	return s.headings
}

// collectHeadings walks doc for headings and resolves each to its written
// extent.
func collectHeadings(doc gast.Node, src []byte) []Heading {
	var out []Heading
	for _, n := range blockNodes(doc) {
		h, ok := n.(*gast.Heading)
		if !ok {
			continue
		}
		if got, ok := headingSpan(h, src); ok {
			out = append(out, got)
		}
	}
	slices.SortFunc(out, func(a, b Heading) int { return a.Span.Start - b.Span.Start })
	return out
}

// headingSpan resolves one heading to its whole-line extent and its tight
// text extent.
//
// goldmark sets a heading's Pos to the first byte of its opening content —
// the first `#` of an ATX marker, the first text byte of a setext heading —
// and its Lines() to the text alone, with the markers, the padding, and an
// ATX closing run already stripped. So the text needs no scanning; only the
// two ends of the block do.
func headingSpan(h *gast.Heading, src []byte) (Heading, bool) {
	pos := h.Pos()
	if pos < 0 || pos >= len(src) {
		return Heading{}, false
	}
	out := Heading{Level: h.Level}
	stop := lineEndAfter(src, pos)
	if start, end, ok := nodeSpan(h); ok {
		out.Text = Span{Start: start, Stop: end}
		if end > stop {
			stop = wholeLineEnd(src, end)
		}
	} else {
		// An ATX heading with no text carries no content line at all, so
		// there is nothing to measure: the text goes after the marker and
		// the padding that follows it.
		at := atxTextStart(src, pos)
		out.Text = Span{Start: at, Stop: at}
	}
	if !isATXMarker(src, pos, h.Level) {
		// A setext heading's underline is a line of its own that goldmark
		// consumes without recording. It is always the line right after
		// the text, so widening over it needs no search — only the check
		// that the line really is one, which also proves the heading is
		// setext rather than an ATX heading followed by a thematic break.
		if end := lineEndAfter(src, stop); isSetextUnderline(src[stop:end], h.Level) {
			stop = end
		}
	}
	out.Span = Span{Start: lineStartBefore(src, pos), Stop: stop}
	return out, true
}

// isATXMarker reports whether src at pos opens an ATX heading of the given
// level: a run of exactly level `#` characters, terminated by a space, a
// tab, the end of the line, or the end of the source.
//
// It is the discriminator between the two heading forms, and the run length
// alone is not enough: `#foo` over `---` is a SETEXT heading whose text
// happens to start with a `#`, because a `#` run with no space after it
// opens nothing.
func isATXMarker(src []byte, pos, level int) bool {
	run := 0
	for i := pos; i < len(src) && src[i] == '#'; i++ {
		run++
	}
	if run != level || run < 1 || run > 6 {
		return false
	}
	at := pos + run
	if at >= len(src) {
		return true
	}
	switch src[at] {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// atxTextStart returns the offset where an empty ATX heading's text would
// begin: past the `#` run and the padding after it, and never past the end
// of the line.
func atxTextStart(src []byte, pos int) int {
	at := pos
	for at < len(src) && src[at] == '#' {
		at++
	}
	for at < len(src) && (src[at] == ' ' || src[at] == '\t') {
		at++
	}
	return at
}

// isSetextUnderline reports whether line is the underline of a setext
// heading of the given level: a run of `=` (level 1) or `-` (level 2)
// behind the container prefix, with nothing but whitespace after it.
func isSetextUnderline(line []byte, level int) bool {
	var marker byte
	switch level {
	case 1:
		marker = '='
	case 2:
		marker = '-'
	default:
		return false
	}
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '>') {
		i++
	}
	run := 0
	for i < len(line) && line[i] == marker {
		run++
		i++
	}
	if run == 0 {
		return false
	}
	for ; i < len(line); i++ {
		switch line[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return false
		}
	}
	return true
}
