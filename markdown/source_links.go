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
	// in which a span from this view overlaps one from that view.
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
//   - A link reference DEFINITION, `[id]: x.md`. goldmark consumes it into
//     its reference map rather than the tree, so no node carries its
//     position. A caller rewriting paths has to find those for itself; this
//     view reports the reference links that USE one, with a zero Dest.
//   - An IMAGE. Images is that view, and the two are kept apart because a
//     caller usually treats them differently — an image destination is an
//     asset, a link destination is a document.
//
// A link is reported only when its written form can be resolved back to the
// exact bytes goldmark read: the destination this view reports is checked
// against the one the parser produced, and a mismatch DROPS the link rather
// than reporting a span that would splice into the wrong place.
// Under-reporting is the safe direction for a rewriter; a wrong offset is
// not.
//
// A link whose destination or title continues on the NEXT LINE is reported
// like any other, inside a blockquote or a list item as well as at the top
// level, with the same container-prefix rule Images documents. The one shape
// still known to be dropped is the same one too: a REFERENCE link whose label
// crosses a line, which the parser matched on a normalized label, so the
// written bytes do not compare equal.
func (s *Source) Links() []Link {
	if s.linksDone {
		return s.links
	}
	s.links, s.linksDone = collectLinks(s.doc, s.src), true
	return s.links
}

// collectLinks walks doc for links and resolves each to its written extent.
//
// Like collectImages this walk descends into inline subtrees, because a link
// IS one, and it is safe for the same reason: a text directive's label is
// parsed against its own detached source and its root is not attached here.
func collectLinks(doc gast.Node, src []byte) []Link {
	var out []Link
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if link, ok := c.(*gast.Link); ok {
				if got, ok := linkSpan(link, src); ok {
					out = append(out, got)
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Link) int { return a.Span.Start - b.Span.Start })
	return out
}

// linkSpan resolves one link to its written extent. A link is a `[label]`
// with nothing in front of it, so the resolver is the shared one — see
// bracketedSpan.
func linkSpan(link *gast.Link, src []byte) (Link, bool) {
	whole, text, dest, ok := bracketedSpan(bracketed{
		node: link, dest: link.Destination, ref: link.Reference, lead: 0,
	}, src)
	if !ok {
		return Link{}, false
	}
	return Link{Span: whole, Text: text, Dest: dest}, true
}
