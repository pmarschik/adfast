package ast

import "strings"

// Link reference definitions and the reference-style links that use them:
// the three nodes mdast calls definition, linkReference and
// imageReference.
//
// A reference is a link written in two places — "[spec]" here and
// "[spec]: ./spec.md" somewhere else — and the two halves pair on the
// NORMALIZED label (NormalizeLabel), which is why both kinds carry the
// SOURCE label and neither carries a copy of the other's data. That is
// the footnote pair's shape (footnote.go), for the same reason: the label
// is an identifier, so nothing may rewrite it, and mdast keeps `label`
// next to `identifier` for exactly this.
//
// Keeping the definition as a node is the whole point. goldmark's parser
// consumes every definition into its reference map AND leaves the
// LinkReferenceDefinition node in the tree; the lift used to drop that
// node and resolve each use to an inline link, so a parse/render round
// trip rewrote "[spec]" into "[spec](./spec.md)" and DELETED a definition
// nothing used. A link written as a reference stays a reference here, and
// an unused definition is written back.
//
// ADF has one link mark with a URL on it and no notion of a reference, so
// these kinds only ever enter the tree from the markdown parse; the ADF
// leg resolves each reference to an inline link and drops the definitions
// (see the convert package). They are therefore also kinds ast.Visitor
// cannot see: they arrived after the interface was published, so Visit
// offers them through the optional ReferenceVisitor and falls back to
// VisitExtension (see visitor.go).

// ReferenceType is the written form of a reference-style link: which of
// the three CommonMark shapes the source used. The values are mdast's
// own strings, and the zero value is deliberately not one of them — a
// reference node built without one is incomplete, and the renderer treats
// anything it does not recognize as the shortcut form.
type ReferenceType string

const (
	// ReferenceShortcut is "[spec]" (and "![logo]"): the label is the
	// whole reference, and it is also the link text.
	ReferenceShortcut ReferenceType = "shortcut"
	// ReferenceCollapsed is "[spec][]" (and "![logo][]"): the label is
	// the link text, followed by an empty second bracket pair.
	ReferenceCollapsed ReferenceType = "collapsed"
	// ReferenceFull is "[the spec][spec]" (and "![the logo][logo]"): the
	// first bracket holds the link text, the second the label.
	ReferenceFull ReferenceType = "full"
)

// Definition is a link reference definition block:
// "[label]: url \"title\"". It is mdast's definition, and it stays where
// the source put it — a definition is a block like any other, and may sit
// inside a blockquote or a list item — but it is visible to the whole
// document, so a reference anywhere pairs with it wherever it sits.
//
// A definition renders back even when nothing references it: an unused
// definition is content the author wrote, and dropping it is the silent
// data loss this kind exists to prevent.
type Definition struct {
	// Label is the source label between "[" and "]", kept verbatim for
	// the render; references pair with it on NormalizeLabel.
	Label string
	// URL is the destination, with its CommonMark escapes decoded.
	URL string
	// Title is the optional title. The delimiter the source used ('"',
	// '\'' or '(') is NOT recorded: remark-stringify writes every title
	// in double quotes, and so does this renderer for an inline link's
	// title already.
	Title string
	BlockSpacing
}

// Kind implements Node.
func (*Definition) Kind() string { return "definition" }

// Identifier returns the normalized label the definition pairs on. It is
// mdast's `identifier` field, computed rather than stored so it cannot
// drift from Label.
func (n *Definition) Identifier() string { return NormalizeLabel(n.Label) }

// LinkRef is a reference-style hyperlink: "[spec]", "[spec][]" or
// "[the spec][spec]". It is mdast's linkReference.
//
// Like GFM footnotes — and unlike an inline link — it only exists when a
// Definition with a matching normalized label exists in the same
// document; CommonMark reads an unmatched "[spec]" as literal text.
//
// The destination is not here. It is on the Definition, and the ADF leg
// resolves it through the definition index; a consumer that needs the URL
// of every link has to index the definitions too (a reference is a link
// whose destination is written elsewhere, and copying it here would give
// two places to update).
type LinkRef struct {
	// Label is the source label the definition pairs on, kept verbatim:
	// the second bracket's content for ReferenceFull, the first
	// bracket's for the other two forms.
	Label string
	// ReferenceType is the written form (see ReferenceType).
	ReferenceType ReferenceType
	// Children holds the link text as inline content. For the shortcut
	// and collapsed forms that text IS the label, so the renderer writes
	// Label back verbatim there and only ReferenceFull renders these
	// nodes — rewriting the label through the escaping rules would break
	// the pairing. They are still the label a reader sees, which is what
	// ast.PlainText and the ADF leg read.
	Children []Node
}

// Kind implements Node.
func (*LinkRef) Kind() string { return "linkReference" }

// Identifier returns the normalized label the reference pairs on
// (mdast's `identifier`; see Definition.Identifier).
func (n *LinkRef) Identifier() string { return NormalizeLabel(n.Label) }

// ImageRef is a reference-style image: "![logo]", "![logo][]" or
// "![the logo][logo]". It is mdast's imageReference, and it is LinkRef's
// twin in every respect except the leading '!'.
type ImageRef struct {
	// Label is the source label the definition pairs on, kept verbatim
	// (see LinkRef.Label).
	Label string
	// ReferenceType is the written form (see ReferenceType).
	ReferenceType ReferenceType
	// Children holds the alt text as inline content, the way Image does
	// (mdast keeps a plain `alt` string; this package keeps the inline
	// slice its Image kind already uses). See LinkRef.Children for which
	// forms render from it.
	Children []Node
}

// Kind implements Node.
func (*ImageRef) Kind() string { return "imageReference" }

// Identifier returns the normalized label the reference pairs on
// (mdast's `identifier`; see Definition.Identifier).
func (n *ImageRef) Identifier() string { return NormalizeLabel(n.Label) }

// NormalizeLabel returns the identifier a bracketed label pairs on:
// whitespace runs collapse to one space, the ends are trimmed, and the
// result case-folds. It is micromark's normalizeIdentifier, the rule
// CommonMark gives for link reference definitions ("[A]" matches
// "[ a ]") and the one GFM footnotes reuse (see NormalizeFootnoteLabel).
//
// goldmark's own parser pairs on util.ToLinkReference, which is the same
// three steps, so a reference this package resolves is a reference
// goldmark resolved.
func NormalizeLabel(label string) string {
	var b strings.Builder
	b.Grow(len(label))
	space := false
	for _, r := range label {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteRune(' ')
		}
		space = false
		b.WriteRune(r)
	}
	// The double fold is micromark's: .toLowerCase().toUpperCase() folds
	// the characters whose lower case is not a round trip (ẛ, İ).
	return strings.ToUpper(strings.ToLower(b.String()))
}
