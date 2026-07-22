package ast

import (
	"testing"
)

// kindNamer proves the compile-time contract: it must implement every
// Visit method to satisfy Visitor[string].
type kindNamer struct{}

func (kindNamer) VisitRoot(*Root) string                             { return "Root" }
func (kindNamer) VisitParagraph(*Paragraph) string                   { return "Paragraph" }
func (kindNamer) VisitHeading(*Heading) string                       { return "Heading" }
func (kindNamer) VisitThematicBreak(*ThematicBreak) string           { return "ThematicBreak" }
func (kindNamer) VisitBlockquote(*Blockquote) string                 { return "Blockquote" }
func (kindNamer) VisitList(*List) string                             { return "List" }
func (kindNamer) VisitListItem(*ListItem) string                     { return "ListItem" }
func (kindNamer) VisitCode(*Code) string                             { return "Code" }
func (kindNamer) VisitHTML(*HTML) string                             { return "HTML" }
func (kindNamer) VisitFrontmatter(*Frontmatter) string               { return "Frontmatter" }
func (kindNamer) VisitTable(*Table) string                           { return "Table" }
func (kindNamer) VisitTableRow(*TableRow) string                     { return "TableRow" }
func (kindNamer) VisitTableCell(*TableCell) string                   { return "TableCell" }
func (kindNamer) VisitContainerDirective(*ContainerDirective) string { return "ContainerDirective" }
func (kindNamer) VisitLeafDirective(*LeafDirective) string           { return "LeafDirective" }
func (kindNamer) VisitText(*Text) string                             { return "Text" }
func (kindNamer) VisitEmphasis(*Emphasis) string                     { return "Emphasis" }
func (kindNamer) VisitStrong(*Strong) string                         { return "Strong" }
func (kindNamer) VisitDelete(*Delete) string                         { return "Delete" }
func (kindNamer) VisitInlineCode(*InlineCode) string                 { return "InlineCode" }
func (kindNamer) VisitBreak(*Break) string                           { return "Break" }
func (kindNamer) VisitLink(*Link) string                             { return "Link" }
func (kindNamer) VisitImage(*Image) string                           { return "Image" }
func (kindNamer) VisitTextDirective(*TextDirective) string           { return "TextDirective" }

func (kindNamer) VisitExtension(n Node) string { return "extension:" + n.Kind() }

// foreignNode simulates an extension kind unknown to this package.
type foreignNode struct{}

func (*foreignNode) Kind() string { return "custom" }

func TestVisitDispatch(t *testing.T) {
	var _ Visitor[string] = kindNamer{} // compile-time exhaustiveness

	v := kindNamer{}
	if got := Visit[string](&Paragraph{}, v); got != "Paragraph" {
		t.Errorf("paragraph: %q", got)
	}
	if got := Visit[string](&TextDirective{}, v); got != "TextDirective" {
		t.Errorf("text directive: %q", got)
	}
	if got := Visit[string](&foreignNode{}, v); got != "extension:custom" {
		t.Errorf("extension routing: %q", got)
	}
}
