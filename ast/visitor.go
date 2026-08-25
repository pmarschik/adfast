package ast

// Visitor is the exhaustive, type-safe visitor over the known node kinds:
// implementing it requires one method per kind, so adding a kind to the
// AST breaks every visitor implementation at compile time — the compiler
// enforcement Go's type switches cannot give. The type parameter is the
// visit result ("struct{}" for pure side effects).
//
// The node set is open (see the extension package): kinds this package
// does not define — dialect directives, custom extension nodes — dispatch
// to VisitExtension.
//
// Visitors receive single nodes and choose themselves whether to recurse
// (via Visit on Children(n)); for casual non-exhaustive traversal use
// Children directly.
type Visitor[T any] interface {
	VisitRoot(*Root) T
	VisitParagraph(*Paragraph) T
	VisitHeading(*Heading) T
	VisitThematicBreak(*ThematicBreak) T
	VisitBlockquote(*Blockquote) T
	VisitList(*List) T
	VisitListItem(*ListItem) T
	VisitCode(*Code) T
	VisitHTML(*HTML) T
	VisitFrontmatter(*Frontmatter) T
	VisitTable(*Table) T
	VisitTableRow(*TableRow) T
	VisitTableCell(*TableCell) T
	VisitContainerDirective(*ContainerDirective) T
	VisitLeafDirective(*LeafDirective) T
	VisitText(*Text) T
	VisitEmphasis(*Emphasis) T
	VisitStrong(*Strong) T
	VisitDelete(*Delete) T
	VisitInlineCode(*InlineCode) T
	VisitBreak(*Break) T
	VisitLink(*Link) T
	VisitImage(*Image) T
	VisitTextDirective(*TextDirective) T
	// VisitExtension receives every node kind not defined in this
	// package (extension.Node implementations such as the dialect
	// directives, or private kinds), and the kinds the optional visitor
	// interfaces below cover when the visitor does not implement them.
	VisitExtension(Node) T
}

// FootnoteVisitor is the optional companion of Visitor for the two GFM
// footnote kinds. They joined the AST after Visitor was published, and
// Visitor is an interface a consumer implements: a new method on it
// would break every implementation outside this module. So Visit
// dispatches a footnote node to this interface when the visitor
// implements it, and to VisitExtension when it does not — an
// implementation that knows nothing of footnotes keeps the treatment it
// already gives an unknown kind.
//
// In-module visitors implement it, and assert it
// ("var _ ast.FootnoteVisitor[T] = …") next to their Visitor assertion
// so the compiler still catches a missing case.
type FootnoteVisitor[T any] interface {
	VisitFootnoteDef(*FootnoteDef) T
	VisitFootnoteRef(*FootnoteRef) T
}

// Visit dispatches n to the matching Visitor method. The kind list is
// split by category across the visit*Kind helpers below; each answers
// ok=false for a kind outside its category, and the fallthrough is the
// open-node contract: anything none of them claims is an extension.
func Visit[T any](n Node, v Visitor[T]) T {
	if r, ok := visitBlockKind(n, v); ok {
		return r
	}
	if r, ok := visitContainerKind(n, v); ok {
		return r
	}
	if r, ok := visitInlineKind(n, v); ok {
		return r
	}
	if r, ok := visitFootnoteKind(n, v); ok {
		return r
	}
	return v.VisitExtension(n)
}

// visitBlockKind dispatches the leaf and flow blocks.
func visitBlockKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Root:
		return v.VisitRoot(n), true
	case *Paragraph:
		return v.VisitParagraph(n), true
	case *Heading:
		return v.VisitHeading(n), true
	case *ThematicBreak:
		return v.VisitThematicBreak(n), true
	case *Blockquote:
		return v.VisitBlockquote(n), true
	case *List:
		return v.VisitList(n), true
	case *ListItem:
		return v.VisitListItem(n), true
	case *Code:
		return v.VisitCode(n), true
	case *HTML:
		return v.VisitHTML(n), true
	case *Frontmatter:
		return v.VisitFrontmatter(n), true
	}
	var zero T
	return zero, false
}

// visitContainerKind dispatches the table parts and the block-level
// directives.
func visitContainerKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Table:
		return v.VisitTable(n), true
	case *TableRow:
		return v.VisitTableRow(n), true
	case *TableCell:
		return v.VisitTableCell(n), true
	case *ContainerDirective:
		return v.VisitContainerDirective(n), true
	case *LeafDirective:
		return v.VisitLeafDirective(n), true
	}
	var zero T
	return zero, false
}

// visitInlineKind dispatches the inline kinds.
func visitInlineKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Text:
		return v.VisitText(n), true
	case *Emphasis:
		return v.VisitEmphasis(n), true
	case *Strong:
		return v.VisitStrong(n), true
	case *Delete:
		return v.VisitDelete(n), true
	case *InlineCode:
		return v.VisitInlineCode(n), true
	case *Break:
		return v.VisitBreak(n), true
	case *Link:
		return v.VisitLink(n), true
	case *Image:
		return v.VisitImage(n), true
	case *TextDirective:
		return v.VisitTextDirective(n), true
	}
	var zero T
	return zero, false
}

// visitFootnoteKind dispatches the two GFM footnote kinds through the
// optional FootnoteVisitor, falling back to VisitExtension for a visitor
// that does not implement it.
func visitFootnoteKind[T any](n Node, v Visitor[T]) (T, bool) {
	fv, hasFootnotes := v.(FootnoteVisitor[T])
	switch n := n.(type) {
	case *FootnoteDef:
		if hasFootnotes {
			return fv.VisitFootnoteDef(n), true
		}
		return v.VisitExtension(n), true
	case *FootnoteRef:
		if hasFootnotes {
			return fv.VisitFootnoteRef(n), true
		}
		return v.VisitExtension(n), true
	}
	var zero T
	return zero, false
}
