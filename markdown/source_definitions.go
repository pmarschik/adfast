package markdown

import (
	"bytes"
	"slices"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Definition is one link reference definition of a Markdown source —
// `[label]: destination "title"` — with a SEPARATE span for each of its three
// written parts.
//
// Separate spans are the point of this view, not a convenience. The thing a
// caller does with a definition is rewrite ONE part of it: a path rewriter
// replaces the destination and must leave the label alone, because the label
// is the identifier every use of the definition pairs on (see
// ast.NormalizeLabel); and a bare-URL normalizer needs the destination span
// in order to EXCLUDE it, because a definition's destination is a URL that
// must not become an autolink. Neither is expressible with a span covering
// the definition as a whole.
//
// Every span here is TIGHT, for the reason Image documents in reverse: a
// definition is a block, but the parts of it a caller edits are not, and a
// span widened to whole lines would swallow the syntax that holds the parts
// apart.
//
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type Definition struct {
	// Span covers the definition exactly: from its `[` through one past the
	// last byte of the title, or of the destination when no title is
	// written. Neither the trailing whitespace CommonMark allows after the
	// last part nor the terminating newline is included.
	//
	// Inside a blockquote or a list item the span starts past the container
	// prefix, at the `[`, and — for a definition written across more than
	// one line — it covers the prefix that sits between the lines, exactly
	// as Images documents for a destination continued on the next line.
	Span Span
	// Label covers the label as written, between the `[` and the closing
	// `]`, escapes included. It is kept verbatim on purpose: the label is an
	// identifier, so a rewriter that touches it breaks the pairing with
	// every reference that uses it. A label may legally contain a newline
	// (CommonMark folds whitespace runs when it matches a label), and the
	// span then covers the break and any container prefix after it.
	//
	// Label is never empty and never the zero Span: CommonMark rejects a
	// blank label, and a definition's label cannot begin at offset 0
	// because its `[` precedes it.
	Label Span
	// Dest covers the destination as written, with any wrapping angle
	// brackets OUTSIDE it, so replacing it keeps a `<…>` wrapper intact and
	// leaves adding one to a caller that needs a space in a new path. A
	// title, and the whitespace before it, are outside it too. This is the
	// convention Link.Dest and Image.Dest use, so the three read alike.
	//
	// Dest is never the zero Span — a definition without a destination is
	// not a definition. It is EMPTY (Start == Stop) for `[a]: <>`, which
	// CommonMark reads as an empty destination, and it then sits between the
	// angle brackets so that an Edit there inserts one.
	Dest Span
	// Title covers the title's CONTENT as written, with the `"…"`, `'…'`, or
	// `(…)` delimiters OUTSIDE it — the same rule Dest applies to angle
	// brackets, so an edit keeps the quoting the author chose. Which of the
	// three delimiters was used is therefore not reported: the byte before
	// Title.Start is it.
	//
	// Title is the ZERO Span when no title is written. The zero value is
	// unambiguous: a title cannot begin at offset 0, because at least
	// `[a]:x ` precedes it. A title written EMPTY (`[a]: x.md ""`) reports
	// an empty Title between its delimiters, so an Edit there inserts one.
	Title Span
}

// Definitions returns every link reference definition in src, in document
// order. See Source.Definitions for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one parse
// and one buffer.
func Definitions(src []byte) []Definition { return NewSource(src).Definitions() }

// Definitions returns every link reference definition in the source, in
// document order, with the label, the destination, and the title of each
// separately located (see Definition).
//
// What counts as a definition is goldmark's verdict, so it is the same
// verdict the conversion path reaches: `[a]: x.md` inside a code block, or on
// a line that continues a paragraph, is not a definition and does not appear,
// and neither is one whose line carries anything after the title. A
// definition IS reported wherever CommonMark allows one to sit — inside a
// blockquote and inside a list item as well as at the top level — because a
// definition is a block like any other and is visible to the whole document
// regardless of where it sits.
//
// Every written definition is reported, including a DUPLICATE label. That is
// the difference between this view and the definition nodes of the pivot AST,
// and it is the reason this view exists rather than a walk over those:
//
//   - A pivot ast.Definition carries no offsets, and never will (see
//     Source), so nothing built from one can locate a part to rewrite.
//   - Its URL and Title are escape-DECODED strings, which are the values a
//     renderer wants and not the bytes on disk; a span has to address what
//     was written.
//   - goldmark's reference MAP, which is what resolves a use to a
//     destination, applies CommonMark's first-wins rule and keeps one entry
//     per normalized label. A rewriter needs one span per WRITTEN
//     definition, because every one of them is bytes an author put in the
//     file and a rewrite that visits only the winner leaves the others
//     stale.
//
// A definition is reported only when its written form can be resolved back to
// the exact bytes goldmark read: each of the three parts this view locates is
// checked against the value the parser recorded, and any mismatch — or an
// extent that disagrees with the parser's own line extent for the definition
// — DROPS the definition rather than reporting a span that would splice into
// the wrong place. Under-reporting is the safe direction for a rewriter; a
// wrong offset is not. UnlocatedDefinitions counts what was dropped, so a
// caller that rewrites definitions can tell an empty view from an incomplete
// one instead of claiming a rewrite it did not make.
//
// The one shape known to be dropped is a definition whose LABEL or TITLE sits
// on a continued line the block parser reached over a TAB — `> [a]: x.md`
// followed by `>\t"t"`, or a label folded across `>\t`. This view is the only
// one of the three with the problem, and PADDING is why: goldmark strips a
// container prefix by expanding a tab it only partly consumes into a padding
// count on the line's segment, and the definition node keeps the paragraph's
// lines as they were BEFORE the paragraph parser trimmed that padding away.
// The label and the title bytes the parser recorded therefore carry spaces
// that stand for no byte of the source, so the written form cannot be
// compared to them and the candidate fails. A DESTINATION on such a line is
// unaffected, because the padding is leading whitespace the parser skips
// before it reads one. Links and Images resolve against a paragraph's
// trimmed lines and never meet a padded segment at all.
func (s *Source) Definitions() []Definition {
	if s.definitionsDone {
		return s.definitions
	}
	s.definitions, s.definitionsUnlocated = collectDefinitions(s.doc, s.src)
	s.definitionsDone = true
	return s.definitions
}

// UnlocatedDefinitions returns how many link reference definitions of the
// source Definitions could NOT resolve to a written extent, and therefore
// left out of its result.
//
// The count exists because the drop is otherwise invisible. A consumer that
// rewrites a destination iterates the view, and a definition missing from it
// is one the pass silently leaves stale; with the count the pass can refuse
// to report a complete rewrite, or fall back to leaving the file alone. Zero
// is the answer for all but the shape Definitions documents.
//
// It is a COUNT and not the spans, because a definition that could not be
// located has no trustworthy offsets to hand out — that is what being
// unlocated means. Whoever needs to point at one has goldmark's own parse.
func (s *Source) UnlocatedDefinitions() int {
	s.Definitions()
	return s.definitionsUnlocated
}

// collectDefinitions walks doc for link reference definitions and resolves
// each to its written parts. It reports the definitions it located and how
// many it did not — see UnlocatedDefinitions for why the second number is
// part of the answer rather than a detail.
//
// The walk skips inline subtrees for the reason blockNodes documents — a text
// directive's label is parsed against its own detached source, so an offset
// found under one refers to no byte of this source — and a definition is a
// block, so nothing is lost by not descending.
func collectDefinitions(doc gast.Node, src []byte) (out []Definition, unlocated int) {
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Type() == gast.TypeInline {
				continue
			}
			if def, ok := c.(*gast.LinkReferenceDefinition); ok {
				if got, ok := definitionSpan(def, src); ok {
					out = append(out, got)
				} else {
					unlocated++
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Definition) int { return a.Span.Start - b.Span.Start })
	return out, unlocated
}

// definitionSpan resolves one definition to its written parts.
//
// goldmark's linkReferenceParagraphTransformer registers the definition in
// the parse context's reference map AND leaves this node in the tree, at the
// source position it was written, with the label, the destination, and the
// title it read. So Pos is the definition's `[` and the node's LINES are its
// source extent — the transformer trims the trailing whitespace off the last
// one — while the three parsed values are what proves a candidate span. Those
// four facts are everything this resolver needs; nothing here re-decides
// whether the bytes are a definition, because the node's existence already
// settled that.
//
// What has to be re-read from the source is where each part BEGINS and ENDS,
// because the parser read all three off a block reader that keeps no
// positions. The grammar used to re-read them is goldmark's own — the same
// scanLinkDestination, scanLinkTitle, and scanClosure the inline tail
// resolver uses — so what this accepts is what the parser accepted.
func definitionSpan(def *gast.LinkReferenceDefinition, src []byte) (Definition, bool) {
	lines := def.Lines()
	pos, stop, ok := definitionExtent(lines, len(src))
	if !ok || pos != def.Pos() || src[pos] != '[' {
		return Definition{}, false
	}
	label, next, ok := definitionLabel(def, src, lines, pos)
	if !ok {
		return Definition{}, false
	}
	dest, next, ok := definitionDest(def, src, lines, next)
	if !ok {
		return Definition{}, false
	}
	title, end, ok := definitionTitle(def, src, lines, next, stop)
	if !ok {
		return Definition{}, false
	}
	// The parser's own extent is the independent check on the whole walk: a
	// scan that stopped anywhere else read a different definition than the
	// one the node describes.
	if end != stop {
		return Definition{}, false
	}
	return Definition{
		Span: Span{Start: pos, Stop: stop}, Label: label, Dest: dest, Title: title,
	}, true
}

// definitionExtent reports the first and the last offset of the definition's
// source lines. The transformer appends one segment per line and trims the
// trailing whitespace off the last, so the pair IS the definition's written
// extent.
func definitionExtent(lines *text.Segments, size int) (start, stop int, ok bool) {
	if lines == nil || lines.Len() == 0 {
		return 0, 0, false
	}
	start, stop = lines.At(0).Start, lines.At(lines.Len()-1).Stop
	if start < 0 || stop > size || start >= stop {
		return 0, 0, false
	}
	return start, stop, true
}

// definitionLabel locates the label of the definition opening at pos and
// reports its span and the offset of the `:` that must follow its `]`.
//
// The closing `]` is found with goldmark's own closure scan, so an escaped
// `]` inside the label does not end it and a second `[` aborts rather than
// nesting — `[a[b]: x` is no definition, and the node would not exist.
func definitionLabel(
	def *gast.LinkReferenceDefinition, src []byte, lines *text.Segments, pos int,
) (Span, int, bool) {
	closer, ok := scanClosure(src, pos+1, '[', ']')
	if !ok {
		return Span{}, 0, false
	}
	label := Span{Start: pos + 1, Stop: closer - 1}
	if !bytes.Equal(bytesAsRead(src, lines, label), def.Label) {
		return Span{}, 0, false
	}
	// CommonMark allows no space between the label and its colon.
	if closer >= len(src) || src[closer] != ':' {
		return Span{}, 0, false
	}
	return label, closer + 1, true
}

// definitionDest locates the destination that follows the colon at from and
// reports its span and the offset just past it.
//
// The destination may sit on the line AFTER the colon, which is why the space
// skip is the container-aware one: `[a]:\n  x.md` inside a list item resumes
// past the continuation indent, and inside a blockquote past the `>`.
func definitionDest(
	def *gast.LinkReferenceDefinition, src []byte, lines *text.Segments, from int,
) (Span, int, bool) {
	i := skipMarkdownSpaces(src, from, lines)
	dest, next, ok := scanLinkDestination(src, i)
	if !ok || !bytes.Equal(bytesAsRead(src, lines, dest), def.Destination) {
		return Span{}, 0, false
	}
	return dest, next, true
}

// definitionTitle locates the optional title that follows the destination
// ending at from, and reports its span — the zero Span when none is written —
// together with the offset one past the definition.
//
// stop bounds the search, and it is load-bearing rather than defensive. A
// title is only a title when it is part of THIS definition, and the parser's
// line extent is what says where the definition ends; without the bound, a
// `""` in a later paragraph would satisfy the empty-title comparison and be
// reported as this definition's title.
func definitionTitle(
	def *gast.LinkReferenceDefinition, src []byte, lines *text.Segments, from, stop int,
) (Span, int, bool) {
	i := skipMarkdownSpaces(src, from, lines)
	// CommonMark requires whitespace between the destination and the title,
	// so an opener flush against the destination is part of the destination
	// and not a delimiter.
	if i >= stop || i == from {
		return Span{}, from, len(def.Title) == 0
	}
	end, ok := scanLinkTitle(src, i)
	if !ok || end > stop {
		return Span{}, from, len(def.Title) == 0
	}
	title := Span{Start: i + 1, Stop: end - 1}
	if !bytes.Equal(bytesAsRead(src, lines, title), def.Title) {
		return Span{}, from, len(def.Title) == 0
	}
	return title, end, true
}
