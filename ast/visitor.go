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
	// directives, or private kinds).
	VisitExtension(Node) T
}

// Visit dispatches n to the matching Visitor method. This switch is the
// single dispatch point, maintained in-package next to the kind list.
func Visit[T any](n Node, v Visitor[T]) T {
	switch n := n.(type) {
	case *Root:
		return v.VisitRoot(n)
	case *Paragraph:
		return v.VisitParagraph(n)
	case *Heading:
		return v.VisitHeading(n)
	case *ThematicBreak:
		return v.VisitThematicBreak(n)
	case *Blockquote:
		return v.VisitBlockquote(n)
	case *List:
		return v.VisitList(n)
	case *ListItem:
		return v.VisitListItem(n)
	case *Code:
		return v.VisitCode(n)
	case *HTML:
		return v.VisitHTML(n)
	case *Frontmatter:
		return v.VisitFrontmatter(n)
	case *Table:
		return v.VisitTable(n)
	case *TableRow:
		return v.VisitTableRow(n)
	case *TableCell:
		return v.VisitTableCell(n)
	case *ContainerDirective:
		return v.VisitContainerDirective(n)
	case *LeafDirective:
		return v.VisitLeafDirective(n)
	case *Text:
		return v.VisitText(n)
	case *Emphasis:
		return v.VisitEmphasis(n)
	case *Strong:
		return v.VisitStrong(n)
	case *Delete:
		return v.VisitDelete(n)
	case *InlineCode:
		return v.VisitInlineCode(n)
	case *Break:
		return v.VisitBreak(n)
	case *Link:
		return v.VisitLink(n)
	case *Image:
		return v.VisitImage(n)
	case *TextDirective:
		return v.VisitTextDirective(n)
	default:
		return v.VisitExtension(n)
	}
}
