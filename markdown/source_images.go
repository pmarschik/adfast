package markdown

import (
	"bytes"
	"slices"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// Image is one image of a Markdown source: where it is written, what its
// alt text is, and where its destination is written when it is written at
// the image at all.
//
// Every span here is TIGHT, which is the opposite of the whole-line rule
// Source.CodeSpans and Source.Headings use, and deliberately so. Those two
// name BLOCKS, and a consumer of a block span wants the lines it owns; an
// image is an INLINE, and the only thing a caller does with one is replace
// a piece of it in place. A span widened to its line would swallow the
// prose around the image, so an edit built from it would destroy the
// sentence it sits in.
type Image struct {
	// Span covers the image exactly: from its `!` through one past the
	// closing `)` of an inline image, or one past the closing `]` of a
	// reference image. Nothing before the `!` and nothing after the closer
	// is included.
	//
	// A nested image — an image inside another image's alt text, which
	// CommonMark allows — is reported in its own right, and its Span lies
	// inside the outer image's Alt. That is the one case in which two
	// spans from this view overlap.
	Span Span
	// Alt covers the alt text as written, between the `![` and the closing
	// `]`, and is empty (Start == Stop) for `![](x.png)`. Inline markup
	// inside the label is part of it verbatim: an alt is a run of inline
	// content, not a string, so slicing it yields source rather than text.
	Alt Span
	// Dest covers the destination as written, with any wrapping angle
	// brackets OUTSIDE it, so replacing it keeps a `<…>` wrapper intact and
	// leaves adding one to a caller that needs a space in a new path. A
	// title, and the whitespace before it, are outside it too.
	//
	// Dest is the ZERO Span for a reference image (`![alt][id]`,
	// `![alt][]`, `![alt]`), whose destination is written at the link
	// definition and not here — there is nothing at the image to rewrite.
	// The zero value is unambiguous: an inline image's destination cannot
	// begin at offset 0, because at least `![](` precedes it. An inline
	// image with an EMPTY destination (`![alt]()`) reports an empty Dest at
	// the offset where a destination would go, so an Edit there inserts
	// one.
	Dest Span
}

// Images returns every image in src, in document order. See Source.Images
// for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func Images(src []byte) []Image { return NewSource(src).Images() }

// Images returns every image in the source, in document order.
//
// What counts as an image is goldmark's verdict, so it is the same verdict
// the conversion path reaches: an `![…](…)` inside inline code, inside a
// code block, or inside an HTML comment is not an image and does not
// appear, and a reference image whose definition is missing is not an image
// either. That is the whole reason this view exists — a regexp over the
// source finds all four.
//
// An image is reported only when its written form can be resolved back to
// the exact bytes goldmark read: the destination this view reports is
// checked against the one the parser produced, and a mismatch DROPS the
// image rather than reporting a span that would splice into the wrong
// place. Under-reporting is the safe direction for a rewriter; a wrong
// offset is not.
//
// The one shape known to be dropped is an image whose destination or title
// continues on the next line INSIDE A BLOCKQUOTE, where the `>` prefix that
// goldmark strips sits between the two halves and the written form is
// therefore not contiguous. Inside a list item the same shape resolves
// normally, because a list's continuation prefix is whitespace, which a
// destination and a title both skip anyway.
func (s *Source) Images() []Image {
	if s.imagesDone {
		return s.images
	}
	s.images, s.imagesDone = collectImages(s.doc, s.src), true
	return s.images
}

// collectImages walks doc for images and resolves each to its written
// extent.
//
// Unlike the block views this walk descends into inline subtrees, because
// an image IS one. That is safe for the reason blockNodes documents in
// reverse: a text directive's label is parsed against its own detached
// source and its root is NOT attached to this tree, so an image inside one
// is unreachable from here and can never contribute a foreign offset.
func collectImages(doc gast.Node, src []byte) []Image {
	var out []Image
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if img, ok := c.(*gast.Image); ok {
				if got, ok := imageSpan(img, src); ok {
					out = append(out, got)
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Image) int { return a.Span.Start - b.Span.Start })
	return out
}

// imageSpan resolves one image to its written extent.
//
// goldmark records an image's Pos — the `!` — and its parsed destination,
// and nothing else about where it ends: the inline parser consumes the
// label, the destination, the title and the closers off a reader that
// keeps no positions. So the tail has to be re-read from the source, which
// this does with goldmark's own grammar (see scanLinkDestination and
// scanClosure) rather than a lookalike.
//
// The label's closing `]` is the one ambiguity, because a `]` can appear
// inside the label — escaped, inside inline code, inside a nested bracket
// pair, inside raw HTML. Two facts resolve it together: the label's parsed
// children give a FLOOR below which the closer cannot be, and a candidate
// closer is only accepted when the tail after it parses back to the
// destination the parser recorded. A wrong candidate therefore fails
// rather than producing a plausible-looking wrong span.
func imageSpan(img *gast.Image, src []byte) (Image, bool) {
	pos := img.Pos()
	if pos < 0 || pos+1 >= len(src) || src[pos] != '!' || src[pos+1] != '[' {
		return Image{}, false
	}
	altStart := pos + 2
	for k := labelFloor(img, altStart); k < len(src); k++ {
		if src[k] != ']' {
			continue
		}
		dest, end, ok := imageTail(img, src, altStart, k)
		if !ok {
			continue
		}
		return Image{
			Span: Span{Start: pos, Stop: end},
			Alt:  Span{Start: altStart, Stop: k},
			Dest: dest,
		}, true
	}
	return Image{}, false
}

// labelFloor returns the lowest offset the label's closing `]` can have:
// one past the last source byte any of the label's parsed children claims,
// or the label's own start when it has none.
//
// The children are goldmark's reading of the label, so anything they cover
// is label content by definition and no `]` inside them is the closer. A
// node kind that carries no segments (an autolink) leaves the floor lower
// than it could be, which costs a few rejected candidates and no accuracy —
// imageTail is what decides.
func labelFloor(img *gast.Image, altStart int) int {
	floor := altStart
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		switch t := n.(type) {
		case *gast.Text:
			floor = max(floor, t.Segment.Stop)
		case *gast.RawHTML:
			if t.Segments != nil && t.Segments.Len() > 0 {
				floor = max(floor, t.Segments.At(t.Segments.Len()-1).Stop)
			}
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(img)
	return floor
}

// imageTail reads what follows a candidate closing `]` at k and reports the
// destination's span and the offset one past the image, or false when the
// candidate is not the closer.
//
// Which of the four written forms to expect is not guessed: goldmark
// records it on the node. An inline image has no Reference; a reference
// image has one, and its Value is the raw label bytes the parser matched,
// which is what makes a reference form checkable at all.
func imageTail(img *gast.Image, src []byte, altStart, k int) (Span, int, bool) {
	if img.Reference == nil {
		return inlineTail(img, src, k)
	}
	// A shortcut (`![alt]`) and a collapsed (`![alt][]`) reference both key
	// off the label itself, so the label is what proves the candidate.
	if img.Reference.Type != gast.ReferenceLinkFull &&
		!bytes.Equal(src[altStart:k], img.Reference.Value) {
		return Span{}, 0, false
	}
	if img.Reference.Type == gast.ReferenceLinkShortcut {
		return Span{}, k + 1, true
	}
	if k+1 >= len(src) || src[k+1] != '[' {
		return Span{}, 0, false
	}
	end, ok := scanClosure(src, k+2, '[', ']')
	if !ok {
		return Span{}, 0, false
	}
	// A full reference (`![alt][id]`) keys off the second bracket pair.
	if img.Reference.Type == gast.ReferenceLinkFull &&
		!bytes.Equal(src[k+2:end-1], img.Reference.Value) {
		return Span{}, 0, false
	}
	return Span{}, end, true
}

// inlineTail reads a `(destination "title")` tail after a candidate closing
// `]` at k. It mirrors goldmark's parseLink step for step, so that what it
// accepts is what the parser accepted.
func inlineTail(img *gast.Image, src []byte, k int) (Span, int, bool) {
	if k+1 >= len(src) || src[k+1] != '(' {
		return Span{}, 0, false
	}
	i := skipMarkdownSpaces(src, k+2)
	if i < len(src) && src[i] == ')' {
		// `![alt]()`, which the parser reads as an empty destination.
		if len(img.Destination) != 0 {
			return Span{}, 0, false
		}
		return Span{Start: i, Stop: i}, i + 1, true
	}
	dest, next, ok := scanLinkDestination(src, i)
	if !ok || !bytes.Equal(src[dest.Start:dest.Stop], img.Destination) {
		return Span{}, 0, false
	}
	i = skipMarkdownSpaces(src, next)
	if i < len(src) && src[i] == ')' {
		return dest, i + 1, true
	}
	if i, ok = scanLinkTitle(src, i); !ok {
		return Span{}, 0, false
	}
	i = skipMarkdownSpaces(src, i)
	if i >= len(src) || src[i] != ')' {
		return Span{}, 0, false
	}
	return dest, i + 1, true
}

// scanLinkDestination reads a link destination at i and reports its span
// (angle brackets excluded), the offset just after it, and whether one is
// there. It is goldmark's parseLinkDestination over the source bytes: a
// destination never crosses a line, `\` escapes a punctuation character,
// and an unwrapped destination ends at whitespace or at the `)` that
// closes the tail, with balanced parentheses allowed inside it.
func scanLinkDestination(src []byte, i int) (Span, int, bool) {
	// goldmark reads a destination off ONE peeked line, newline included.
	end := lineEndAfter(src, i)
	if i >= end {
		return Span{}, 0, false
	}
	if src[i] == '<' {
		return scanWrappedDestination(src, i, end)
	}
	opened, j := 0, i
scan:
	for j < end {
		switch c := src[j]; {
		case c == '\\' && j+1 < end && util.IsPunct(src[j+1]):
			j += 2
			continue
		case c == '(':
			opened++
		case c == ')':
			opened--
			if opened < 0 {
				break scan
			}
		case util.IsSpace(c):
			break scan
		}
		j++
	}
	return Span{Start: i, Stop: j}, j, j > i
}

// scanWrappedDestination reads a `<…>` destination that opens at i on the
// line ending at end, and reports the span BETWEEN the brackets. An
// unclosed `<` is not a destination at all, which is what makes
// `![a](<b c.png)` no image.
func scanWrappedDestination(src []byte, i, end int) (Span, int, bool) {
	for j := i + 1; j < end; j++ {
		if src[j] == '\\' && j+1 < end && util.IsPunct(src[j+1]) {
			j++
			continue
		}
		if src[j] == '>' {
			return Span{Start: i + 1, Stop: j}, j + 1, true
		}
	}
	return Span{}, 0, false
}

// scanLinkTitle reads a `"…"`, `'…'`, or `(…)` title at i and reports the
// offset just after it. It is goldmark's parseLinkTitle: the opener picks
// the closer, and the closure is the one scanClosure finds.
func scanLinkTitle(src []byte, i int) (int, bool) {
	if i >= len(src) {
		return 0, false
	}
	opener, closer := src[i], src[i]
	switch opener {
	case '"', '\'':
	case '(':
		closer = ')'
	default:
		return 0, false
	}
	return scanClosure(src, i+1, opener, closer)
}

// scanClosure returns the offset one past the closer that ends a run opened
// before i, or false when the run does not close.
//
// It is goldmark's text.Reader.FindClosure with the options its link parser
// uses: escapes are honored, a newline does NOT end the search, and nesting
// is OFF — a second opener aborts rather than deepening, which is why
// `[a[b]` is not a reference and `(a(b)` is not a title.
func scanClosure(src []byte, i int, opener, closer byte) (int, bool) {
	for ; i < len(src); i++ {
		switch c := src[i]; {
		case c == '\\' && i+1 < len(src) && util.IsPunct(src[i+1]):
			i++
		case c == closer:
			return i + 1, true
		case c == opener:
			return 0, false
		}
	}
	return 0, false
}

// skipMarkdownSpaces returns the first offset at or after i that is not
// whitespace. Like goldmark's SkipSpaces it crosses line boundaries, which
// is what lets a destination sit on the line after its `(`.
func skipMarkdownSpaces(src []byte, i int) int {
	for i < len(src) && util.IsSpace(src[i]) {
		i++
	}
	return i
}
