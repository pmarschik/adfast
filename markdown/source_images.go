package markdown

import (
	"bytes"
	"slices"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
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
	// An alt may cross a line, and the span then covers the break and any
	// container prefix after it, exactly as Span does.
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
// offset is not. UnlocatedImages counts what was dropped, so a caller that
// rewrites destinations can tell an empty view from an incomplete one.
//
// An image whose destination or title continues on the NEXT LINE is
// reported like any other, inside a blockquote or a list item as well as at
// the top level. Its Span then covers the written form as written, which
// includes the container prefix that sits between the two halves: the span
// of the image in `> ![a](x.png\n> "t")` selects `![a](x.png\n> "t")`, the
// `> ` included, exactly as a list item's span keeps its continuation
// indent. Replacing that span is still correct: a replacement drops the
// prefix together with the line break that precedes it, and the line the
// image began on keeps the prefix it already had.
//
// A REFERENCE image whose LABEL crosses a line is reported too, and its span
// covers the break and the container prefix after it by the same rule:
// `![a][i\nd]`, `> ![a][i\n> d]` and `- ![a][i\n  d]` all pair with
// `[i d]: …`, because CommonMark folds whitespace runs when it matches a
// label against a definition. The span selects the label as WRITTEN, so an
// edit replacing it replaces the fold as well.
//
// A TAB in the container prefix of such a continued line is not a limitation
// here, for the reason Source.Links states in full: the block this view
// resolves against is a paragraph, whose parser trims each line's leading
// whitespace — the padding a partly consumed tab leaves included — before
// this view sees a segment. Source.Definitions is the one view that keeps
// the padding, and the one with the limitation.
func (s *Source) Images() []Image {
	if s.imagesDone {
		return s.images
	}
	s.images, s.imagesUnlocated = collectImages(s.doc, s.src)
	s.imagesDone = true
	return s.images
}

// UnlocatedImages returns how many images of the source Images could NOT
// resolve to a written extent, and therefore left out of its result. See
// UnlocatedDefinitions for why the count is reported at all; no shape of
// input is currently known to make this one nonzero.
func (s *Source) UnlocatedImages() int {
	s.Images()
	return s.imagesUnlocated
}

// collectImages walks doc for images and resolves each to its written
// extent. It reports the images it located and how many it did not.
//
// Unlike the block views this walk descends into inline subtrees, because
// an image IS one. That is safe for the reason blockNodes documents in
// reverse: a text directive's label is parsed against its own detached
// source and its root is NOT attached to this tree, so an image inside one
// is unreachable from here and can never contribute a foreign offset.
func collectImages(doc gast.Node, src []byte) (out []Image, unlocated int) {
	nested := labelExtents{}
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if img, ok := c.(*gast.Image); ok {
				if got, ok := imageSpan(img, src, nested); ok {
					out = append(out, got)
				} else {
					unlocated++
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Image) int { return a.Span.Start - b.Span.Start })
	return out, unlocated
}

// imageSpan resolves one image to its written extent. An image is a `[label]`
// behind a `!`, so the resolver is the shared one — see bracketedSpan.
func imageSpan(img *gast.Image, src []byte, nested labelExtents) (Image, bool) {
	whole, alt, dest, ok := bracketedSpan(bracketed{
		node: img, dest: img.Destination, ref: img.Reference, lead: 1,
	}, src, nested)
	if !ok {
		return Image{}, false
	}
	return Image{Span: whole, Alt: alt, Dest: dest}, true
}

// bracketed is what an image and a link have in common, which is everything
// this file measures.
//
// goldmark gives both kinds the same three fields off one embedded baseLink —
// Destination, Title and Reference — and sets Pos to the first byte of the
// WRITTEN form for both (parser/link.go: SetPos(last.Segment.Start), where the
// segment starts one byte earlier for an image). So the four written forms,
// the label-closer search and the tail grammar are one problem, not two, and
// the only thing that differs between them is how many bytes stand before the
// `[`. That is lead.
//
// The fields are copied out rather than reached through an interface because
// they ARE fields: Go has no way to name a struct field two concrete types
// share, and an interface here would mean a method set goldmark does not have.
//
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type bracketed struct {
	// ref is the reference the parser matched, or nil for an inline form.
	ref *gast.ReferenceLink
	// node is the image or the link, used for its Pos and for the walk over
	// its label's children.
	node gast.Node
	// dest is the destination the parser recorded, which is what proves a
	// candidate closer.
	dest []byte
	// lead is the number of bytes between Pos and the `[`: 1 for an image's
	// `!`, 0 for a link.
	lead int
}

// bracketedSpan resolves one image or link to its written extent, and reports
// the whole span, the label's span, and the destination's span.
//
// goldmark records Pos — the `!` of an image, the `[` of a link — and the
// parsed destination, and nothing else about where the written form ends: the
// inline parser consumes the label, the destination, the title and the closers
// off a reader that keeps no positions. So the tail has to be re-read from the
// source, which this does with goldmark's own grammar (see scanLinkDestination
// and scanClosure) rather than a lookalike.
//
// The label's closing `]` is the one ambiguity, because a `]` can appear
// inside the label — escaped, inside inline code, inside a nested bracket
// pair, inside raw HTML, inside the destination of a written form nested in
// the label. Two facts resolve it together: the label's parsed children give a
// FLOOR below which the closer cannot be, and a candidate closer is only
// accepted when the tail after it parses back to the destination the parser
// recorded. A wrong candidate therefore fails rather than producing a
// plausible-looking wrong span.
//
// The floor is what carries a nested written form, and the destination check
// cannot stand in for it. CommonMark lets a bracket pair stand in link text —
// an IMAGE written inside link text is one — so a label can hold a whole
// written form, tail included, before its own closer; and when the nested
// destination is BYTE-EQUAL to the outer one, as a thumbnail linking to its
// own file writes it, the check passes on the nested closer and reports the
// outer form ending inside itself. A floor above the nested form is what
// leaves the label's own closer as the first candidate.
func bracketedSpan(
	b bracketed, src []byte, nested labelExtents,
) (whole, label, dest Span, ok bool) {
	pos := b.node.Pos()
	open := pos + b.lead
	if pos < 0 || open >= len(src) || src[open] != '[' {
		return Span{}, Span{}, Span{}, false
	}
	if b.lead == 1 && src[pos] != '!' {
		return Span{}, Span{}, Span{}, false
	}
	labelStart, lines := open+1, enclosingLines(b.node)
	for k := labelFloor(b.node, src, labelStart, nested); k < len(src); k++ {
		if src[k] != ']' {
			continue
		}
		dest, end, ok := bracketedTail(b, src, labelStart, k, lines)
		if !ok {
			continue
		}
		return Span{Start: pos, Stop: end}, Span{Start: labelStart, Stop: k}, dest, true
	}
	return Span{}, Span{}, Span{}, false
}

// labelFloor returns the lowest offset the label's closing `]` can have:
// one past the last source byte any of the label's parsed children claims,
// or the label's own start when it has none.
//
// The children are goldmark's reading of the label, so anything they cover
// is label content by definition and no `]` inside them is the closer. A
// node kind that carries no segments (an autolink) leaves the floor lower
// than it could be, which costs a few rejected candidates and no accuracy —
// bracketedTail is what decides.
//
// A child that is itself an image or a link claims more than its own label:
// it claims its whole written form, which this resolver measures for it. That
// is the one kind of child whose extent is not on the node, and taking it from
// the same resolver keeps the two answers consistent — the floor is one past
// the very span the view goes on to report for the nested form. It is also the
// only way past a nested DESTINATION or TITLE, where CommonMark does not
// require a bracket to balance (`[![](a[.png)](x.md)`); no reading of the
// label as brackets can know those bytes are a path. A nested form this
// resolver DROPS claims nothing, since its extent is unknown, and the floor is
// then what it was before.
func labelFloor(n gast.Node, src []byte, labelStart int, nested labelExtents) int {
	floor := labelStart
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch t := c.(type) {
			case *gast.Text:
				floor = max(floor, t.Segment.Stop)
			case *gast.RawHTML:
				if t.Segments != nil && t.Segments.Len() > 0 {
					floor = max(floor, t.Segments.At(t.Segments.Len()-1).Stop)
				}
			case *gast.Image, *gast.Link:
				if stop, ok := nested.stop(c, src); ok {
					floor = max(floor, stop)
					continue
				}
			}
			walk(c)
		}
	}
	walk(n)
	return floor
}

// labelExtents remembers where a written form nested in a label ends, so that
// a chain of forms nested one inside the next is measured ONCE for the whole
// walk instead of once for every ancestor that needs its extent. The value is
// the offset one past the written form, and zero when the resolver dropped it:
// no written form can end at offset zero, so the two answers stay distinct.
//
// Without it a document of forms nested n deep would cost n measurements per
// form and n² for the walk, which is a cost an author of a document does not
// control — the nesting is the input.
type labelExtents map[gast.Node]int

// stop measures the nested written form at n and reports the offset one past
// it, or false when the resolver drops it. Both answers are remembered.
func (e labelExtents) stop(n gast.Node, src []byte) (int, bool) {
	if stop, seen := e[n]; seen {
		return stop, stop != 0
	}
	stop := 0
	switch t := n.(type) {
	case *gast.Image:
		if got, ok := imageSpan(t, src, e); ok {
			stop = got.Span.Stop
		}
	case *gast.Link:
		if got, ok := linkSpan(t, src, e); ok {
			stop = got.Span.Stop
		}
	}
	e[n] = stop
	return stop, stop != 0
}

// bracketedTail reads what follows a candidate closing `]` at k and reports
// the destination's span and the offset one past the written form, or false
// when the candidate is not the closer.
//
// Which of the four written forms to expect is not guessed: goldmark
// records it on the node. An inline image or link has no Reference; a
// reference one has one, and its Value is the label bytes the parser READ,
// which is what makes a reference form checkable at all.
//
// Read, not written, is the load-bearing word, and the label is where the two
// diverge: a reference label may legally contain a newline, because CommonMark
// folds whitespace runs when it matches a label against a definition, and the
// bytes the parser read across that break are the line's content with its
// container prefix — a blockquote's `>`, a list item's indent, a paragraph
// continuation's leading spaces — already stripped. So every comparison here
// goes through bytesAsRead rather than slicing the source directly.
//
// Comparing the written bytes instead does not merely under-report. It rejects
// the label's own closer, and the caller's scan then walks on to the next `]`
// in the source, where a LATER bracket pair whose written bytes happen to
// equal the read label satisfies the check — and the form is reported ending
// inside a construct that is not it. `[][i\n d][i\nd]` against `[I d]: 0` is
// that document: both labels read as `i\nd`, so the empty-label link would be
// reported as covering the second link as well.
func bracketedTail(
	b bracketed, src []byte, labelStart, k int, lines *text.Segments,
) (Span, int, bool) {
	if b.ref == nil {
		return inlineTail(b, src, k, lines)
	}
	// A shortcut (`![alt]`) and a collapsed (`![alt][]`) reference both key
	// off the label itself, so the label is what proves the candidate.
	if b.ref.Type != gast.ReferenceLinkFull &&
		!bytes.Equal(bytesAsRead(src, lines, Span{Start: labelStart, Stop: k}), b.ref.Value) {
		return Span{}, 0, false
	}
	if b.ref.Type == gast.ReferenceLinkShortcut {
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
	if b.ref.Type == gast.ReferenceLinkFull &&
		!bytes.Equal(bytesAsRead(src, lines, Span{Start: k + 2, Stop: end - 1}), b.ref.Value) {
		return Span{}, 0, false
	}
	return Span{}, end, true
}

// inlineTail reads a `(destination "title")` tail after a candidate closing
// `]` at k. It mirrors goldmark's parseLink step for step, so that what it
// accepts is what the parser accepted.
func inlineTail(b bracketed, src []byte, k int, lines *text.Segments) (Span, int, bool) {
	if k+1 >= len(src) || src[k+1] != '(' {
		return Span{}, 0, false
	}
	i := skipMarkdownSpaces(src, k+2, lines)
	if i < len(src) && src[i] == ')' {
		// `![alt]()`, which the parser reads as an empty destination.
		if len(b.dest) != 0 {
			return Span{}, 0, false
		}
		return Span{Start: i, Stop: i}, i + 1, true
	}
	dest, next, ok := scanLinkDestination(src, i)
	if !ok || !bytes.Equal(src[dest.Start:dest.Stop], b.dest) {
		return Span{}, 0, false
	}
	i = skipMarkdownSpaces(src, next, lines)
	if i < len(src) && src[i] == ')' {
		return dest, i + 1, true
	}
	if i, ok = scanLinkTitle(src, i); !ok {
		return Span{}, 0, false
	}
	i = skipMarkdownSpaces(src, i, lines)
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

// bytesAsRead returns the bytes sp covers as goldmark READ them: the span's
// source bytes with the container prefix between two of the block's lines
// removed. lines is the block's content segments, one per line, each starting
// where the block parser stopped stripping that line's prefix.
//
// This is the bridge every span check in these views crosses. A written form
// may cross a line — a reference LABEL may (CommonMark folds whitespace runs
// when it matches a label), and so may a destination or a title continued
// after its opener — and the bytes between the two halves are prefix the block
// parser stripped before the inline or definition parser ever saw the content.
// A span reports what was WRITTEN, prefix included, because that is what an
// edit has to replace; the comparison that VALIDATES the span has to be
// against what was READ, because that is all the parser recorded. Comparing
// written bytes to a read value rejects a correct span, and a resolver that
// keeps searching after a false rejection can accept a lookalike belonging to
// a different construct — a wrong offset, which is the one outcome these
// views must never produce.
//
// This function reads a segment's Start and Stop and IGNORES its Padding,
// which is the deliberate limit of the bridge and the one place a construct
// is lost. Padding is how goldmark records a container prefix ending in a
// TAB it only partly consumed: `>\t` at the start of a line takes ONE column
// of the tab's expansion to the next tab stop and records the columns left
// over as a count of spaces on the segment, spaces that stand for no byte of
// the source. A value the parser read through such a segment therefore
// contains bytes no span can address, and adding them here would be worse
// than dropping the construct — it would make a span whose bytes differ from
// the source it indexes compare EQUAL.
//
// Which view pays for that is measurable rather than a matter of taste, and
// it is one: a paragraph's parser trims every line's leading whitespace,
// padding included, when it closes, so the segments Links and Images resolve
// against never carry any. A LinkReferenceDefinition keeps the paragraph's
// lines as they stood BEFORE that trim, so its segments do — see
// Source.Definitions, which names the two parts of a definition the padding
// reaches and the one it does not.
func bytesAsRead(src []byte, lines *text.Segments, sp Span) []byte {
	if lines == nil || lines.Len() == 1 {
		return src[sp.Start:sp.Stop]
	}
	out := make([]byte, 0, sp.Len())
	for i := range lines.Len() {
		s := lines.At(i)
		lo, hi := max(sp.Start, s.Start), min(sp.Stop, s.Stop)
		if lo < hi {
			out = append(out, src[lo:hi]...)
		}
	}
	return out
}

// skipMarkdownSpaces returns the first offset at or after i that is not
// whitespace and is not container prefix. Like goldmark's SkipSpaces it
// crosses line boundaries, which is what lets a destination sit on the line
// after its `(`; unlike it, it crosses them over the SOURCE rather than over
// the block's stripped content, so a line's prefix has to be stepped over
// too. lines is what says where that prefix ends — see resumeAfterNewline.
func skipMarkdownSpaces(src []byte, i int, lines *text.Segments) int {
	for i < len(src) && util.IsSpace(src[i]) {
		if src[i] == '\n' {
			if next, ok := resumeAfterNewline(lines, i); ok {
				i = next
				continue
			}
		}
		i++
	}
	return i
}

// enclosingLines returns the content segments of the block that holds n:
// one segment per line, each beginning where the block parser stopped
// stripping the container prefix off that line. Nil when the nearest block
// records none, in which case a scan over the source is the same scan
// goldmark's inline parser ran, because there was no prefix to strip.
func enclosingLines(n gast.Node) *text.Segments {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type() == gast.TypeBlock {
			return p.Lines()
		}
	}
	return nil
}

// resumeAfterNewline returns the offset at which the block's content
// resumes on the line after the newline at nl: past a blockquote's `>` and
// past a list item's indent.
//
// This is the whole reason an image whose destination or title continues on
// the next line resolves inside a blockquote. The inline parser never saw
// the `>`, so the written form it recorded — the destination this view
// checks itself against — is the text on the far side of it. Taking the
// resume offset from the parser's own segments rather than re-deriving the
// prefix keeps a `>` that is destination rather than prefix intact: no
// segment starts there, so nothing is skipped.
func resumeAfterNewline(lines *text.Segments, nl int) (int, bool) {
	if lines == nil {
		return 0, false
	}
	for i := range lines.Len() {
		if s := lines.At(i); s.Start > nl {
			return s.Start, true
		}
	}
	return 0, false
}
