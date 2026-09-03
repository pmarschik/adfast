package markdown

import (
	"slices"

	gast "github.com/yuin/goldmark/ast"
)

// Link is one link of a Markdown source: where it is written, what its text
// is, and where its destination is written when it is written at the link at
// all.
//
// Every span here is TIGHT, for the reason Image documents: a link is an
// INLINE, and a span widened to its line would swallow the sentence around
// it.
type Link struct {
	// Span covers the link exactly: from its `[` through one past the
	// closing `)` of an inline link, or one past the closing `]` of a
	// reference link. Nothing after the closer is included.
	Span Span
	// Text covers the link text as written, between the `[` and the closing
	// `]`, and is empty (Start == Stop) for `[](x.md)`. Inline markup inside
	// the label is part of it verbatim: link text is a run of inline content,
	// not a string, so slicing it yields source rather than text. An IMAGE
	// written inside link text is part of it too, and is reported separately
	// by Images — its Span then lies inside this Text, which is the one case
	// in which a span from this view overlaps one from that view. Link text
	// may cross a line, and the span then covers the break and any container
	// prefix after it, exactly as Span does.
	Text Span
	// Dest covers the destination as written, with any wrapping angle
	// brackets OUTSIDE it, so replacing it keeps a `<…>` wrapper intact and
	// leaves adding one to a caller that needs a space in a new path. A
	// title, and the whitespace before it, are outside it too.
	//
	// Dest is the ZERO Span for a reference link (`[text][id]`, `[text][]`,
	// `[text]`), whose destination is written at the link definition and not
	// here — there is nothing at the link to rewrite. The zero value is
	// unambiguous: an inline link's destination cannot begin at offset 0,
	// because at least `[](` precedes it. An inline link with an EMPTY
	// destination (`[text]()`) reports an empty Dest at the offset where a
	// destination would go, so an Edit there inserts one.
	Dest Span
}

// Links returns every link in src, in document order. See Source.Links for
// the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one parse
// and one buffer.
func Links(src []byte) []Link { return NewSource(src).Links() }

// Links returns every link in the source, in document order.
//
// What counts as a link is goldmark's verdict, so it is the same verdict the
// conversion path reaches: a `[…](…)` inside inline code, inside a code
// block, or inside an HTML comment is not a link and does not appear, and a
// reference link whose definition is missing is not a link either. That is
// the whole reason this view exists — a regexp over the source finds all
// four, and a line scanner that skips only code BLOCKS still finds the one
// inside inline code, because inline code is not a range of lines.
//
// Three things a reader may call a link are NOT reported here, each because
// it has no destination written at a link node:
//
//   - An AUTOLINK, whether written `<https://x>` or linkified from a bare
//     URL. goldmark makes a different node for it, and its destination IS its
//     text, so there is nothing to rewrite separately.
//   - A link reference DEFINITION, `[id]: x.md`. It is not a link: it is the
//     block that declares one, and the destination written in it belongs to
//     every reference link that pairs with its label rather than to any one
//     of them. This view reports those USES, each with a zero Dest, and says
//     nothing about where the destination they share is written.
//   - An IMAGE. Images is that view, and the two are kept apart because a
//     caller usually treats them differently — an image destination is an
//     asset, a link destination is a document.
//
// A definition's POSITION is not the reason it is absent from this view, and
// the record here once said it was. goldmark's paragraph transformer does
// both: it registers a definition in the parse context's reference map AND
// leaves a LinkReferenceDefinition node in the tree at the source position it
// was written, inside a blockquote and inside a list item as well as at the
// top level. Source.Definitions is the view over those nodes, and a caller
// rewriting paths wants it and this one together — Definitions locates the
// destination of each definition, Links locates the destination of each
// inline link.
//
// A link is reported only when its written form can be resolved back to the
// exact bytes goldmark read: the destination this view reports is checked
// against the one the parser produced, and a mismatch DROPS the link rather
// than reporting a span that would splice into the wrong place.
// Under-reporting is the safe direction for a rewriter; a wrong offset is
// not. UnlocatedLinks counts what was dropped, so a caller that rewrites
// destinations can tell an empty view from an incomplete one.
//
// A link whose destination, title or reference LABEL continues on the NEXT
// LINE is reported like any other, inside a blockquote or a list item as well
// as at the top level, with the same container-prefix rule Images documents.
// A label that crosses a line is legal — CommonMark folds whitespace runs
// when it matches a label against a definition — and the span then ends at
// that link's own closing `]`, never at a later bracket pair's. See
// bracketedTail for why the second half of that sentence needs saying.
//
// A TAB in the container prefix of such a continued line is not a limitation
// here, which is worth saying because it IS one for Definitions: the block
// whose lines this view resolves against is a paragraph (or the text block a
// tight list item turns one into), and its parser trims each line's leading
// whitespace — the padding a partly consumed tab leaves included — before
// this view ever sees a segment. See bytesAsRead for the mechanism and
// Source.Definitions for the view that keeps the padding.
func (s *Source) Links() []Link {
	if s.linksDone {
		return s.links
	}
	s.links, s.linksUnlocated = collectLinks(s.doc, s.src)
	s.linksDone = true
	return s.links
}

// UnlocatedLinks returns how many links of the source Links could NOT
// resolve to a written extent, and therefore left out of its result. See
// UnlocatedDefinitions for why the count is reported at all; no shape of
// input is currently known to make this one nonzero.
func (s *Source) UnlocatedLinks() int {
	s.Links()
	return s.linksUnlocated
}

// collectLinks walks doc for links and resolves each to its written extent.
// It reports the links it located and how many it did not.
//
// Like collectImages this walk descends into inline subtrees, because a link
// IS one, and it is safe for the same reason: a text directive's label is
// parsed against its own detached source and its root is not attached here.
func collectLinks(doc gast.Node, src []byte) (out []Link, unlocated int) {
	nested := labelExtents{}
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if link, ok := c.(*gast.Link); ok {
				if got, ok := linkSpan(link, src, nested); ok {
					out = append(out, got)
				} else {
					unlocated++
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Link) int { return a.Span.Start - b.Span.Start })
	return out, unlocated
}

// linkSpan resolves one link to its written extent. A link is a `[label]`
// with nothing in front of it, so the resolver is the shared one — see
// bracketedSpan.
func linkSpan(link *gast.Link, src []byte, nested labelExtents) (Link, bool) {
	whole, text, dest, ok := bracketedSpan(bracketed{
		node: link, dest: link.Destination, ref: link.Reference, lead: 0,
	}, src, nested)
	if !ok {
		return Link{}, false
	}
	return Link{Span: whole, Text: text, Dest: dest}, true
}
