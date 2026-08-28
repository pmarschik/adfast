package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"sort"

	gast "github.com/yuin/goldmark/ast"
)

// Span is a half-open byte range [Start, Stop) of a Markdown source.
//
// Offsets are BYTES, not runes and not UTF-16 code units: they index the
// source slice directly, which is what Go slicing and Source.Apply need. A
// consumer that speaks a different unit (a JavaScript editor indexes UTF-16)
// converts at its own boundary, where it holds both representations — see
// the adfast/wasm module.
type Span struct {
	// Start is the offset of the span's first byte.
	Start int
	// Stop is the offset one past the span's last byte.
	Stop int
}

// Len is the span's width in bytes.
func (s Span) Len() int { return s.Stop - s.Start }

// Contains reports whether off lies inside the span. The span's Stop is
// NOT contained: a span is half-open, so two adjacent spans never both
// claim the byte between them.
func (s Span) Contains(off int) bool { return off >= s.Start && off < s.Stop }

// Overlaps reports whether the two spans share at least one byte. An empty
// span (Start >= Stop) covers no byte, so it overlaps nothing — including
// itself.
func (s Span) Overlaps(o Span) bool {
	if s.Len() <= 0 || o.Len() <= 0 {
		return false
	}
	return s.Start < o.Stop && o.Start < s.Stop
}

// Spans is a set of spans in ascending order, no two of which overlap.
// Every view on Source returns spans in that form, which is what makes the
// binary searches below correct.
type Spans []Span

// Contains reports whether any span covers off.
func (ss Spans) Contains(off int) bool {
	return ss.Overlaps(Span{Start: off, Stop: off + 1})
}

// Overlaps reports whether any span shares a byte with s.
func (ss Spans) Overlaps(s Span) bool {
	if s.Len() <= 0 {
		return false
	}
	// Ascending and non-overlapping, so the only candidate is the first
	// span reaching past s.Start.
	i := sort.Search(len(ss), func(i int) bool { return ss[i].Stop > s.Start })
	return i < len(ss) && ss[i].Start < s.Stop
}

// Source is a Markdown source parsed once, with its byte positions kept.
//
// It is the one source-anchored surface in adfast: every view over a
// document's byte layout — CodeSpans, InlineCodeSpans, Headings, Images,
// Directives, ListItems, and TextMatches — is a method here, computed from a SINGLE parse
// of a SINGLE buffer. Views therefore cannot disagree with each other, and Apply — the
// only sanctioned way to turn spans back into bytes — splices into the very
// buffer the spans were measured in.
//
// The goldmark tree is deliberately NOT exposed. A caller that walked it
// would be re-deriving positions by hand, which is the thing this type
// exists to remove; a view that is missing belongs here, as a method.
//
// The pivot ast.Node tree carries no positions and never will: Parse
// documents it as source-independent, and a tree built from ADF has no
// source at all. Positions live at the goldmark layer, which is this one.
//
// A Source is immutable as a document but NOT safe for concurrent use:
// every view memoizes on its first call. Parse once per goroutine, or
// call the views before sharing.
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type Source struct {
	// doc is the goldmark parse of src, and stays unexported on purpose:
	// see above.
	doc gast.Node
	// src is the buffer every span addresses — the parse's input, which is
	// not always NewSource's (see Verbatim).
	src []byte
	// code memoizes CodeSpans.
	code Spans
	// inlineCode memoizes InlineCodeSpans.
	inlineCode Spans
	// headings memoizes Headings.
	headings []Heading
	// images memoizes Images.
	images []Image
	// directives memoizes Directives.
	directives []Directive
	// listItems memoizes ListItems.
	listItems []ListItem
	// prose memoizes the runs of literal text TextMatches filters against.
	prose Spans
	// same backs Verbatim.
	same bool
	// codeDone guards code, which is legitimately empty for most documents.
	codeDone bool
	// inlineCodeDone guards inlineCode, for the same reason.
	inlineCodeDone bool
	// headingsDone guards headings, for the same reason.
	headingsDone bool
	// imagesDone guards images, for the same reason.
	imagesDone bool
	// directivesDone guards directives, for the same reason.
	directivesDone bool
	// listItemsDone guards listItems, for the same reason.
	listItemsDone bool
	// proseDone guards prose, for the same reason.
	proseDone bool
}

// NewSource parses src and retains its byte positions. src is expected to
// be the Markdown body with \n line endings and without frontmatter — the
// same input Parse takes.
//
// The parse is the guarded one Parse uses, so a source that makes goldmark
// panic still yields a usable Source. Recovery re-parses a NORMALIZED COPY
// of src, and the spans then address that copy rather than the caller's
// bytes; Bytes returns what the spans address and Verbatim reports whether
// it is the input unchanged. A byte-preserving caller checks Verbatim
// before it trusts an offset against its own buffer.
func NewSource(src []byte) *Source {
	doc, parsed := parseGuarded(NewParser(), src)
	return &Source{src: parsed, doc: doc, same: bytes.Equal(parsed, src)}
}

// Bytes returns the source the spans address. Apply splices into it. The
// slice is the Source's own and must not be modified.
func (s *Source) Bytes() []byte { return s.src }

// Verbatim reports whether Bytes is byte-for-byte the source NewSource was
// given. It is false only when the guarded parse had to recover from a
// goldmark panic by normalizing the input.
func (s *Source) Verbatim() bool { return s.same }

// Text returns the bytes sp covers, or nil when sp is not a range of this
// source. The slice aliases the Source and must not be modified.
func (s *Source) Text(sp Span) []byte {
	if sp.Start < 0 || sp.Stop > len(s.src) || sp.Start > sp.Stop {
		return nil
	}
	return s.src[sp.Start:sp.Stop]
}

// Errors Apply returns. They are the mistakes a hand-rolled splice makes
// silently.
var (
	// ErrSpanRange reports an edit whose span is not a range of the source.
	ErrSpanRange = errors.New("edit span is out of range")
	// ErrSpanOverlap reports two edits claiming the same byte.
	ErrSpanOverlap = errors.New("edit spans overlap")
)

// Edit replaces the bytes Span covers with Text. An empty Span inserts at
// its offset; an empty Text deletes.
type Edit struct {
	// Text is what replaces the span's bytes.
	Text string
	// Span is the range of Source.Bytes to replace.
	Span Span
}

// Apply returns a new slice: Bytes with every edit applied.
//
// This is the splice half of the surface, and it is the only one. adfast
// owns both the positions and the edit that consumes them, so a change to
// the walk that produces a span changes the edit with it, and no consumer
// re-derives either.
//
// Every edit is validated against the source before anything is written:
// out-of-range and inverted spans are rejected (ErrSpanRange), and so is
// any pair of edits sharing a byte (ErrSpanOverlap). The edits are then
// applied in one ascending pass, so a caller never has to know the
// apply-back-to-front trick that hand-splicing needs to keep its offsets
// valid. The Source is untouched, so its spans stay usable afterwards.
//
// Apply enforces NO policy about where an edit may land — an edit inside a
// code span is allowed, because avoiding code blocks is one caller's rule
// and not another's. Composing a view with Apply is where policy lives.
func (s *Source) Apply(edits ...Edit) ([]byte, error) {
	if len(edits) == 0 {
		return slices.Clone(s.src), nil
	}
	ordered := slices.Clone(edits)
	for _, e := range ordered {
		if e.Span.Start < 0 || e.Span.Stop > len(s.src) || e.Span.Start > e.Span.Stop {
			return nil, fmt.Errorf(
				"%w: [%d,%d) in a source of %d bytes",
				ErrSpanRange, e.Span.Start, e.Span.Stop, len(s.src),
			)
		}
	}
	// Stable, so two insertions at the same offset keep their argument
	// order and the result is deterministic.
	slices.SortStableFunc(ordered, func(a, b Edit) int { return a.Span.Start - b.Span.Start })
	for i := 1; i < len(ordered); i++ {
		if ordered[i].Span.Start < ordered[i-1].Span.Stop {
			return nil, fmt.Errorf(
				"%w: [%d,%d) and [%d,%d)",
				ErrSpanOverlap,
				ordered[i-1].Span.Start, ordered[i-1].Span.Stop,
				ordered[i].Span.Start, ordered[i].Span.Stop,
			)
		}
	}
	size := len(s.src)
	for _, e := range ordered {
		size += len(e.Text) - e.Span.Len()
	}
	out := make([]byte, 0, max(size, 0))
	at := 0
	for _, e := range ordered {
		out = append(out, s.src[at:e.Span.Start]...)
		out = append(out, e.Text...)
		at = e.Span.Stop
	}
	return append(out, s.src[at:]...), nil
}

// CodeSpans returns the byte span of every code block in src — fenced and
// indented alike. See Source.CodeSpans for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
//
// Inline code is a separate view: see InlineCodeSpans.
func CodeSpans(src []byte) Spans { return NewSource(src).CodeSpans() }

// CodeSpans returns the byte span of every code block — fenced and indented
// alike — in document order, non-overlapping.
//
// NAME: in CommonMark, "code span" is INLINE code. These are code BLOCK
// extents; the name is the one the consuming call sites were filed against.
// Inline code is not reported here — Source.InlineCodeSpans is that view,
// and a caller guarding a rewriter against every literal byte wants both.
//
// A span covers WHOLE LINES: it starts at the first byte of the line the
// block opens on and ends just past the newline of the line it closes on.
// It therefore includes the fence delimiter lines, an indented block's
// indent, and — inside a blockquote or a list item — the container's prefix
// on those lines, up to and including a list marker when the fence opens on
// the marker's own line. That over-includes by a few bytes of syntax, which
// is the safe direction for the only thing these spans are for: a rewriter
// deciding what NOT to touch. No link, URL, or piece of prose can live in
// the bytes it over-includes.
//
// The spans come from goldmark's own block parse, so what they call a code
// block is what the conversion path calls one. The line scanners they
// replace disagree with CommonMark in both directions — a closing fence
// carrying an info string does not close its block, and four-space content
// inside a blockquoted list item IS code — and the disagreements that
// UNDER-report are the ones that corrupt a document.
func (s *Source) CodeSpans() Spans {
	if s.codeDone {
		return s.code
	}
	s.code, s.codeDone = collectCodeSpans(s.doc, s.src), true
	return s.code
}

// collectCodeSpans walks doc for code blocks and widens each to whole lines.
func collectCodeSpans(doc gast.Node, src []byte) Spans {
	blocks := blockNodes(doc)
	starts := make([]int, 0, len(blocks))
	for _, n := range blocks {
		if p := n.Pos(); p >= 0 {
			starts = append(starts, p)
		}
	}
	slices.Sort(starts)

	var out Spans
	for _, n := range blocks {
		_, fenced := n.(*gast.FencedCodeBlock)
		if _, indented := n.(*gast.CodeBlock); !fenced && !indented {
			continue
		}
		if sp, ok := codeBlockSpan(n, src, fenced, starts); ok {
			out = append(out, sp)
		}
	}
	slices.SortFunc(out, func(a, b Span) int { return a.Start - b.Start })
	return clipOverlaps(out)
}

// codeBlockSpan widens one code block to its whole-line source extent.
func codeBlockSpan(n gast.Node, src []byte, fenced bool, starts []int) (Span, bool) {
	// goldmark sets a block's Pos to the first byte of its opening content
	// (parser.go: blockPos.Start + BlockOffset), which is the ONLY anchor an
	// empty fence has — it carries neither content lines nor an info string.
	pos := n.Pos()
	if pos < 0 || pos >= len(src) {
		return Span{}, false
	}
	start := lineStartBefore(src, pos)
	stop := lineEndAfter(src, pos)
	if _, end, ok := nodeSpan(n); ok && end > stop {
		stop = wholeLineEnd(src, end)
	}
	if fenced {
		stop = withClosingFence(src, pos, stop, starts)
	}
	return Span{Start: start, Stop: stop}, true
}

// withClosingFence extends a fenced block's end over its closing fence line,
// when the line at stop is one. goldmark consumes the closer without
// recording it, so it has to be recognized in the source — from the fence
// character and run length of the OPENING fence, which pos points at.
//
// The candidate is rejected when another block starts on that line: an
// unclosed fence inside a blockquote can be followed by a new top-level
// fence ("> ```\n> x\n```\n"), which looks exactly like a closer and is not
// one. Accepting it would overlap the block that really starts there.
func withClosingFence(src []byte, pos, stop int, starts []int) int {
	if stop >= len(src) {
		return stop
	}
	marker := src[pos]
	if marker != '`' && marker != '~' {
		return stop
	}
	run := 0
	for i := pos; i < len(src) && src[i] == marker; i++ {
		run++
	}
	end := lineEndAfter(src, stop)
	if !isClosingFence(src[stop:end], marker, run) {
		return stop
	}
	if i, _ := slices.BinarySearch(starts, stop); i < len(starts) && starts[i] < end {
		return stop
	}
	return end
}

// isClosingFence reports whether line closes a fence of at least run marker
// characters. The leading run of spaces, tabs, and blockquote markers is the
// container prefix the fence sits behind; CommonMark forbids anything but
// whitespace after the closing run.
func isClosingFence(line []byte, marker byte, run int) bool {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '>') {
		i++
	}
	got := 0
	for i < len(line) && line[i] == marker {
		got++
		i++
	}
	if got < run {
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

// blockNodes returns doc's block descendants in document order.
//
// Inline subtrees are skipped whole, not merely filtered: a text
// directive's label is parsed against its own DETACHED source, so offsets
// found under one do not refer to this source at all and must never reach
// a span.
func blockNodes(doc gast.Node) []gast.Node {
	var out []gast.Node
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Type() == gast.TypeInline {
				continue
			}
			out = append(out, c)
			walk(c)
		}
	}
	walk(doc)
	return out
}

// lineEndAfter returns the offset one past the newline that terminates the
// line containing pos, or len(src) at an unterminated last line.
func lineEndAfter(src []byte, pos int) int {
	for pos < len(src) && src[pos] != '\n' {
		pos++
	}
	if pos < len(src) {
		pos++
	}
	return pos
}

// wholeLineEnd rounds end up to a line boundary. goldmark's code-block line
// segments already include their newline, so the scan only runs on a source
// whose last line is unterminated.
func wholeLineEnd(src []byte, end int) int {
	if end > 0 && end <= len(src) && src[end-1] == '\n' {
		return end
	}
	return lineEndAfter(src, end)
}

// clipOverlaps enforces Spans' non-overlapping contract on an ascending
// slice. Nothing measured produces an overlap; the clip is what lets every
// consumer rely on the contract rather than re-checking it.
func clipOverlaps(in Spans) Spans {
	for i := 1; i < len(in); i++ {
		if in[i].Start < in[i-1].Stop {
			in[i].Start = in[i-1].Stop
			in[i].Stop = max(in[i].Stop, in[i].Start)
		}
	}
	return in
}
