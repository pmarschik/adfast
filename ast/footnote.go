package ast

import "strings"

// GFM footnotes: the reference/definition pair mdast calls
// footnoteReference and footnoteDefinition. The two nodes carry the
// SOURCE label, and pair up on the normalized identifier
// (NormalizeFootnoteLabel), exactly like mdast keeps `label` next to
// `identifier`.
//
// ADF has no footnote of any kind, so these kinds only ever enter the
// tree from the markdown parse; the ADF leg flattens them (see the
// convert package). They are therefore also the two kinds ast.Visitor
// cannot see: they arrived after the interface was published, so Visit
// offers them through the optional FootnoteVisitor and falls back to
// VisitExtension (see visitor.go).

// FootnoteDef is a footnote definition block: "[^label]: content", whose
// content is the indented block sequence under it. It is mdast's
// footnoteDefinition, and it stays where the source put it — a
// definition is a block like any other, and may sit inside a blockquote
// or a list item.
type FootnoteDef struct {
	// Label is the source label between "[^" and "]", kept verbatim for
	// the render; references pair with it on NormalizeFootnoteLabel.
	Label string
	// Children holds the definition's blocks.
	Children []Node
	BlockSpacing
}

// Kind implements Node.
func (*FootnoteDef) Kind() string { return "footnoteDefinition" }

// FootnoteRef is an inline footnote reference: "[^label]". It is mdast's
// footnoteReference, and — like GFM — it only exists when a FootnoteDef
// with a matching normalized label exists in the same document; an
// unmatched "[^label]" is literal text.
type FootnoteRef struct {
	// Label is the source label between "[^" and "]", kept verbatim for
	// the render; it pairs with a FootnoteDef on NormalizeFootnoteLabel.
	Label string
}

// Kind implements Node.
func (*FootnoteRef) Kind() string { return "footnoteReference" }

// NormalizeFootnoteLabel returns the identifier a footnote label pairs
// on: whitespace runs collapse to one space, the ends are trimmed, and
// the result case-folds. It is micromark's normalizeIdentifier, the same
// rule link reference definitions use, so "[^A]" matches "[^ a ]".
func NormalizeFootnoteLabel(label string) string {
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
