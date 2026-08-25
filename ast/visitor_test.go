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

// footnoteNamer adds the optional footnote half, so the two footnote
// kinds route to it instead of VisitExtension.
type footnoteNamer struct{ kindNamer }

func (footnoteNamer) VisitFootnoteDef(*FootnoteDef) string { return "FootnoteDef" }
func (footnoteNamer) VisitFootnoteRef(*FootnoteRef) string { return "FootnoteRef" }

// TestVisitDispatchIsExhaustive pins one node per known kind against the
// method it must reach. Visit splits the kind list across the
// visit*Kind helpers, and a kind dropped from all of them still
// compiles — it silently becomes an extension. This table is what
// notices.
func TestVisitDispatchIsExhaustive(t *testing.T) {
	cases := []struct {
		node Node
		want string
	}{
		{&Root{}, "Root"},
		{&Paragraph{}, "Paragraph"},
		{&Heading{}, "Heading"},
		{&ThematicBreak{}, "ThematicBreak"},
		{&Blockquote{}, "Blockquote"},
		{&List{}, "List"},
		{&ListItem{}, "ListItem"},
		{&Code{}, "Code"},
		{&HTML{}, "HTML"},
		{&Frontmatter{}, "Frontmatter"},
		{&Table{}, "Table"},
		{&TableRow{}, "TableRow"},
		{&TableCell{}, "TableCell"},
		{&ContainerDirective{}, "ContainerDirective"},
		{&LeafDirective{}, "LeafDirective"},
		{&Text{}, "Text"},
		{&Emphasis{}, "Emphasis"},
		{&Strong{}, "Strong"},
		{&Delete{}, "Delete"},
		{&InlineCode{}, "InlineCode"},
		{&Break{}, "Break"},
		{&Link{}, "Link"},
		{&Image{}, "Image"},
		{&TextDirective{}, "TextDirective"},
		{&FootnoteDef{}, "FootnoteDef"},
		{&FootnoteRef{}, "FootnoteRef"},
	}
	var fv footnoteNamer
	var _ FootnoteVisitor[string] = fv
	for _, tc := range cases {
		if got := Visit[string](tc.node, fv); got != tc.want {
			t.Errorf("%s dispatched to %q, want %q", tc.node.Kind(), got, tc.want)
		}
	}
}

// TestVisitFootnoteFallback pins the other half of the optional
// interface: a visitor without the footnote methods keeps treating the
// two kinds as extensions.
func TestVisitFootnoteFallback(t *testing.T) {
	v := kindNamer{}
	if got := Visit[string](&FootnoteDef{}, v); got != "extension:footnoteDefinition" {
		t.Errorf("footnote definition: %q", got)
	}
	if got := Visit[string](&FootnoteRef{}, v); got != "extension:footnoteReference" {
		t.Errorf("footnote reference: %q", got)
	}
}
