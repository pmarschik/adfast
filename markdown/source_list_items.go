package markdown

import (
	"slices"

	gast "github.com/yuin/goldmark/ast"
)

// ListItem is one list item of a Markdown source: the lines it occupies, the
// bytes of its content as written, how deep it is nested, and whether the
// list holding it is ordered.
//
// Bullet and ordered items are both reported, and so is an item of a list
// nested inside another item, inside a blockquote, or inside any other
// container. Which of them counts — a top-level item only, every item, an
// unordered one only — is the consumer's policy, and Depth and Ordered are
// what it decides on.
//
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type ListItem struct {
	// Span covers the item's WHOLE LINES: it starts at the first byte of the
	// line the marker is written on and ends just past the newline of the
	// last line the item owns — a continuation line, a second paragraph, and
	// a fenced block's closing fence included. Inside a blockquote or an
	// outer list item the container's prefix on those lines is part of it.
	//
	// This is the same over-including rule Source.CodeSpans and
	// Source.Headings document, and for the same reason: a consumer of a
	// block span wants the lines the block owns, so that the runs BETWEEN
	// two spans hold no fragment of either one's syntax.
	//
	// A NESTED item's Span lies inside the Span of every item above it. That
	// is the one case in which two spans from this view overlap, and it is
	// the same containment Source.Images documents for an image written
	// inside another image's alt text.
	Span Span
	// Text covers the item's content as written, and is TIGHT: the marker,
	// the padding after it, and the newline ending the item's last line are
	// all outside it. Slicing it yields the raw written text — inline markup,
	// a task checkbox, and a nested list's own source included, nothing
	// transformed — which is what a consumer that must round-trip an item
	// verbatim needs.
	//
	// An item spanning more than one line keeps the bytes BETWEEN those lines
	// as written, so the continuation indent and an enclosing blockquote's
	// `>` are inside Text. That is the same as-written rule
	// Source.InlineCodeSpans documents for a code span crossing a line: a
	// consumer wanting one line takes the first, and a consumer splicing
	// wants the prefix left where it is.
	//
	// An item with no content at all (`-`) has an empty Text positioned where
	// its content would go, so an Edit there inserts some.
	Text Span
	// Depth is 0 for an item of a list that no other list contains, and one
	// more for each list above it. A container between the two lists — a
	// blockquote written inside a list item, say — does not reset it: what
	// Depth counts is enclosing LISTS, because that is what a consumer
	// deciding whether an item is a sub-item is asking about.
	Depth int
	// Ordered reports whether the list holding the item is ordered, so that a
	// consumer can tell `- a` from `1. a` without re-reading the marker. The
	// marker character and the number are not reported: they are a rendering
	// decision, and a consumer that needs them has Span to read.
	Ordered bool
}

// ListItems returns every list item in src, in document order. See
// Source.ListItems for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func ListItems(src []byte) []ListItem { return NewSource(src).ListItems() }

// ListItems returns every list item in the source, in document order.
//
// What counts as a list item is goldmark's verdict, so it is the same verdict
// the conversion path reaches. A `- ` line inside a fenced or an indented code
// block is not a list item and does not appear; neither is `- - -`, which
// CommonMark reads as a thematic break, nor `-item`, which has no space after
// its marker. An ordered item, a `+` item, and an item whose content starts on
// the line below its marker ARE items — the line scanners this view replaces
// saw none of them.
//
// Both directions of that disagreement corrupt a document that is read field
// by field: a bullet quoted inside a code block becomes a phantom value on the
// next write, and an ordered list under the same heading contributes nothing
// and falls through to whatever the caller does with unlisted text.
//
// Items are ordered by their first byte, so an item of a nested list follows
// the item that contains it.
func (s *Source) ListItems() []ListItem {
	if s.listItemsDone {
		return s.listItems
	}
	s.listItems, s.listItemsDone = collectListItems(s.doc, s.src), true
	return s.listItems
}

// collectListItems walks doc for list items and resolves each to its written
// extent.
//
// The depth is carried down rather than counted back up from the node,
// because it is the walk that knows which lists it has already entered. An
// item's own list does not deepen it — the items of a top-level list are
// depth 0 — so the counter rises when the walk descends INTO an item, which
// is the only place a further list can begin.
func collectListItems(doc gast.Node, src []byte) []ListItem {
	starts := blockStarts(doc)
	var out []ListItem
	var walk func(n gast.Node, depth int)
	walk = func(n gast.Node, depth int) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			// Inline subtrees are skipped whole for the reason blockNodes
			// documents: a text directive's label is parsed against its own
			// DETACHED source, so an offset found under one refers to a
			// different buffer and must never reach a span.
			if c.Type() == gast.TypeInline {
				continue
			}
			if li, ok := c.(*gast.ListItem); ok {
				if got, ok := listItemSpan(li, src, depth, starts); ok {
					out = append(out, got)
				}
				walk(c, depth+1)
				continue
			}
			walk(c, depth)
		}
	}
	walk(doc, 0)
	slices.SortStableFunc(out, func(a, b ListItem) int { return a.Span.Start - b.Span.Start })
	return out
}

// blockStarts returns the ascending first-byte offsets of doc's block
// descendants, which is what withClosingFence needs to reject a fence line
// that opens a new block instead of closing the one before it.
func blockStarts(doc gast.Node) []int {
	blocks := blockNodes(doc)
	starts := make([]int, 0, len(blocks))
	for _, n := range blocks {
		if p := n.Pos(); p >= 0 {
			starts = append(starts, p)
		}
	}
	slices.Sort(starts)
	return starts
}

// listItemSpan resolves one list item to its whole-line extent and its tight
// content extent.
//
// goldmark sets a list item's Pos to the first byte of its marker and gives
// the content to the item's child blocks, so the content extent is those
// children's lines and only the two ends of the item need measuring.
func listItemSpan(li *gast.ListItem, src []byte, depth int, starts []int) (ListItem, bool) {
	pos := li.Pos()
	if pos < 0 || pos >= len(src) {
		return ListItem{}, false
	}
	out := ListItem{Depth: depth}
	if l, ok := li.Parent().(*gast.List); ok {
		out.Ordered = l.IsOrdered()
	}
	stop := lineEndAfter(src, pos)
	if start, end, ok := nodeSpan(li); ok {
		out.Text = Span{Start: start, Stop: end}
		if end > stop {
			stop = wholeLineEnd(src, end)
		}
	} else {
		// An item with no content carries no line to measure, so the content
		// goes right after the marker and the padding that follows it. `-`
		// alone is the whole item, and Text sits at the end of that line.
		at := itemTextStart(src, pos)
		out.Text = Span{Start: at, Stop: at}
	}
	// goldmark consumes a fence's CLOSING line without recording it, so an
	// item whose last block is a fenced code block ends, as measured, one
	// line short. Both spans have to reach past it: a Span stopping before
	// the closer would leave "```" at the head of whatever follows the item,
	// and a Text stopping there would slice an unterminated fence.
	if end := itemFenceEnd(li, src, starts); end > stop {
		stop = end
		out.Text.Stop = trimLineBreak(src, end)
	}
	out.Span = Span{Start: lineStartBefore(src, pos), Stop: stop}
	return out, true
}

// itemFenceEnd returns the furthest offset any fenced code block inside the
// item reaches, its closing fence line included, or 0 when it holds none.
//
// Every fenced block is measured, not only the last one, because "last" is a
// guess about the item's shape and max is not: a fence nested in a sub-item
// cannot reach past the item that contains it, so a block that is not the
// tail never widens anything.
func itemFenceEnd(li gast.Node, src []byte, starts []int) int {
	end := 0
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Type() == gast.TypeInline {
				continue
			}
			if _, ok := c.(*gast.FencedCodeBlock); ok {
				if sp, ok := codeBlockSpan(c, src, true, starts); ok {
					end = max(end, sp.Stop)
				}
			}
			walk(c)
		}
	}
	walk(li)
	return end
}

// itemTextStart returns the offset where an empty list item's content would
// begin: past the marker and the padding after it, and never past the end of
// the line. It is the list-item twin of atxTextStart.
func itemTextStart(src []byte, pos int) int {
	at := pos
	for at < len(src) && src[at] >= '0' && src[at] <= '9' {
		at++
	}
	if at < len(src) {
		switch src[at] {
		case '-', '+', '*', '.', ')':
			at++
		}
	}
	for at < len(src) && (src[at] == ' ' || src[at] == '\t') {
		at++
	}
	return at
}

// trimLineBreak backs an offset off the newline that precedes it, so a span
// widened to a whole line can be reported tight.
func trimLineBreak(src []byte, end int) int {
	if end > 0 && end <= len(src) && src[end-1] == '\n' {
		return end - 1
	}
	return end
}
