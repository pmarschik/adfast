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

// referenceNamer adds the optional link reference half on top, so the
// three reference kinds route to it instead of VisitExtension.
type referenceNamer struct{ footnoteNamer }

func (referenceNamer) VisitDefinition(*Definition) string { return "Definition" }
func (referenceNamer) VisitLinkRef(*LinkRef) string       { return "LinkRef" }
func (referenceNamer) VisitImageRef(*ImageRef) string     { return "ImageRef" }

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
		{&Definition{}, "Definition"},
		{&LinkRef{}, "LinkRef"},
		{&ImageRef{}, "ImageRef"},
	}
	var fv referenceNamer
	var (
		_ FootnoteVisitor[string]  = fv
		_ ReferenceVisitor[string] = fv
	)
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

// TestVisitReferenceFallback is TestVisitFootnoteFallback for the link
// reference kinds: a consumer's Visitor written before they existed still
// compiles, and sees them as extensions rather than not at all.
func TestVisitReferenceFallback(t *testing.T) {
	v := kindNamer{}
	cases := map[Node]string{
		&Definition{}: "extension:definition",
		&LinkRef{}:    "extension:linkReference",
		&ImageRef{}:   "extension:imageReference",
	}
	for node, want := range cases {
		if got := Visit[string](node, v); got != want {
			t.Errorf("%s: %q, want %q", node.Kind(), got, want)
		}
	}
}

// TestNormalizeLabelIsTheParsersIdentifier pins the pairing rule the
// three kinds share with GFM footnotes: whitespace runs collapse, the
// ends are trimmed, and the case FULLY folds — goldmark's own
// util.ToLinkReference, because goldmark decides whether a reference node
// exists at all and a fold that disagrees with the parser drops the link
// the parser paired (see NormalizeLabel, and the ADF-leg measurement in
// the root package's linkref_test.go).
//
// The identifier string itself is pinned, not just the equivalence
// classes, because it is exported: full folding is lowercase-directed
// ("A" is "a"), where the fold this replaced ran lower-then-upper and
// returned "A".
//
// The last two cases are the accepted divergence from micromark, whose
// normalizeIdentifier maps BOTH "ı" and "I" to "I" and would pair them.
func TestNormalizeLabelIsTheParsersIdentifier(t *testing.T) {
	cases := []struct{ in, want string }{
		{"A", "a"},
		{" a ", "a"},
		{"the  spec", "the spec"},
		{"the\n\tspec", "the spec"},
		{"", ""},
		// Full folding maps these onto sequences a simple case mapping
		// leaves alone, which is what makes "[ss]" pair with "[ẞ]".
		{"ẞ", "ss"},
		{"ß", "ss"},
		{"ﬁ", "fi"},
		{"ẛ", "ṡ"},
		{"Σ", "σ"},
		{"ς", "σ"},
		{"ı", "ı"},
		{"I", "i"},
	}
	for _, tc := range cases {
		if got := NormalizeLabel(tc.in); got != tc.want {
			t.Errorf("NormalizeLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
		// The footnote-facing name is the same rule, by construction.
		if got := NormalizeFootnoteLabel(tc.in); got != tc.want {
			t.Errorf("NormalizeFootnoteLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
