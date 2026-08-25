package dialect

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

type dialectNamer struct{}

func (dialectNamer) VisitPanel(*Panel) string             { return "Panel" }
func (dialectNamer) VisitExpand(*Expand) string           { return "Expand" }
func (dialectNamer) VisitMedia(*Media) string             { return "Media" }
func (dialectNamer) VisitJQL(*JQL) string                 { return "JQL" }
func (dialectNamer) VisitLinkCard(*LinkCard) string       { return "LinkCard" }
func (dialectNamer) VisitLinkEmbed(*LinkEmbed) string     { return "LinkEmbed" }
func (dialectNamer) VisitColwidths(*Colwidths) string     { return "Colwidths" }
func (dialectNamer) VisitDecisions(*Decisions) string     { return "Decisions" }
func (dialectNamer) VisitMention(*Mention) string         { return "Mention" }
func (dialectNamer) VisitStatus(*Status) string           { return "Status" }
func (dialectNamer) VisitMediaInline(*MediaInline) string { return "MediaInline" }
func (dialectNamer) VisitColor(*Color) string             { return "Color" }
func (dialectNamer) VisitBg(*Bg) string                   { return "Bg" }
func (dialectNamer) VisitUnderline(*Underline) string     { return "Underline" }
func (dialectNamer) VisitSub(*Sub) string                 { return "Sub" }
func (dialectNamer) VisitSup(*Sup) string                 { return "Sup" }

func (dialectNamer) VisitDate(*Date) string                       { return "Date" }
func (dialectNamer) VisitPlaceholder(*Placeholder) string         { return "Placeholder" }
func (dialectNamer) VisitEmoji(*Emoji) string                     { return "Emoji" }
func (dialectNamer) VisitAnnotation(*Annotation) string           { return "Annotation" }
func (dialectNamer) VisitFontSize(*FontSize) string               { return "FontSize" }
func (dialectNamer) VisitInlineExtension(*InlineExtension) string { return "InlineExtension" }
func (dialectNamer) VisitExtension(*Extension) string             { return "Extension" }
func (dialectNamer) VisitBodiedExtension(*BodiedExtension) string { return "BodiedExtension" }
func (dialectNamer) VisitFrame(*Frame) string                     { return "Frame" }
func (dialectNamer) VisitSyncBlock(*SyncBlock) string             { return "SyncBlock" }
func (dialectNamer) VisitBodiedSyncBlock(*BodiedSyncBlock) string { return "BodiedSyncBlock" }
func (dialectNamer) VisitSection(*Section) string                 { return "Section" }
func (dialectNamer) VisitColumn(*Column) string                   { return "Column" }
func (dialectNamer) VisitMediaCaption(*MediaCaption) string       { return "MediaCaption" }
func (dialectNamer) VisitAlign(*Align) string                     { return "Align" }
func (dialectNamer) VisitIndent(*Indent) string                   { return "Indent" }
func (dialectNamer) VisitBreakout(*Breakout) string               { return "Breakout" }
func (dialectNamer) VisitDataConsumer(*DataConsumer) string       { return "DataConsumer" }
func (dialectNamer) VisitFragment(*Fragment) string               { return "Fragment" }

func (dialectNamer) VisitOther(n ast.Node) string { return "other:" + n.Kind() }

func TestVisitDispatch(t *testing.T) {
	var _ Visitor[string] = dialectNamer{} // compile-time exhaustiveness

	v := dialectNamer{}
	if got := Visit[string](&Panel{}, v); got != "Panel" {
		t.Errorf("panel: %q", got)
	}
	if got := Visit[string](&Status{}, v); got != "Status" {
		t.Errorf("status: %q", got)
	}
	if got := Visit[string](&ast.Paragraph{}, v); got != "other:paragraph" {
		t.Errorf("other routing: %q", got)
	}
}

// chainVisitor proves the ast → dialect exhaustiveness chain: ast.Visit
// routes dialect kinds through VisitExtension into dialect.Visit, so a
// consumer handles core AND dialect kinds with full compile enforcement.
type chainVisitor struct{}

func (chainVisitor) VisitRoot(*ast.Root) string                   { return "core:Root" }
func (chainVisitor) VisitParagraph(*ast.Paragraph) string         { return "core:Paragraph" }
func (chainVisitor) VisitHeading(*ast.Heading) string             { return "core:Heading" }
func (chainVisitor) VisitThematicBreak(*ast.ThematicBreak) string { return "core:ThematicBreak" }
func (chainVisitor) VisitBlockquote(*ast.Blockquote) string       { return "core:Blockquote" }
func (chainVisitor) VisitList(*ast.List) string                   { return "core:List" }
func (chainVisitor) VisitListItem(*ast.ListItem) string           { return "core:ListItem" }
func (chainVisitor) VisitCode(*ast.Code) string                   { return "core:Code" }
func (chainVisitor) VisitHTML(*ast.HTML) string                   { return "core:HTML" }
func (chainVisitor) VisitFrontmatter(*ast.Frontmatter) string     { return "core:Frontmatter" }
func (chainVisitor) VisitTable(*ast.Table) string                 { return "core:Table" }
func (chainVisitor) VisitTableRow(*ast.TableRow) string           { return "core:TableRow" }
func (chainVisitor) VisitTableCell(*ast.TableCell) string         { return "core:TableCell" }
func (chainVisitor) VisitContainerDirective(*ast.ContainerDirective) string {
	return "core:ContainerDirective"
}
func (chainVisitor) VisitLeafDirective(*ast.LeafDirective) string { return "core:LeafDirective" }
func (chainVisitor) VisitText(*ast.Text) string                   { return "core:Text" }
func (chainVisitor) VisitEmphasis(*ast.Emphasis) string           { return "core:Emphasis" }
func (chainVisitor) VisitStrong(*ast.Strong) string               { return "core:Strong" }
func (chainVisitor) VisitDelete(*ast.Delete) string               { return "core:Delete" }
func (chainVisitor) VisitInlineCode(*ast.InlineCode) string       { return "core:InlineCode" }
func (chainVisitor) VisitBreak(*ast.Break) string                 { return "core:Break" }
func (chainVisitor) VisitLink(*ast.Link) string                   { return "core:Link" }
func (chainVisitor) VisitImage(*ast.Image) string                 { return "core:Image" }
func (chainVisitor) VisitTextDirective(*ast.TextDirective) string { return "core:TextDirective" }

func (chainVisitor) VisitExtension(n ast.Node) string {
	return Visit[string](n, dialectNamer{})
}

func TestVisitorChain(t *testing.T) {
	var _ ast.Visitor[string] = chainVisitor{}

	if got := ast.Visit[string](&Panel{PanelType: "info"}, chainVisitor{}); got != "Panel" {
		t.Errorf("chained dialect dispatch: %q", got)
	}
	if got := ast.Visit[string](&ast.Heading{Depth: 2}, chainVisitor{}); got != "core:Heading" {
		t.Errorf("core dispatch: %q", got)
	}
}

// TestVisitDispatchIsExhaustive pins one node per dialect kind against
// the method it must reach. Visit splits the kind list across the
// visit*Kind helpers, and a kind dropped from all of them still compiles
// — it silently becomes someone else's node. This table is what notices.
func TestVisitDispatchIsExhaustive(t *testing.T) {
	cases := []struct {
		node ast.Node
		want string
	}{
		{&Panel{}, "Panel"},
		{&Expand{}, "Expand"},
		{&Media{}, "Media"},
		{&JQL{}, "JQL"},
		{&LinkCard{}, "LinkCard"},
		{&LinkEmbed{}, "LinkEmbed"},
		{&Colwidths{}, "Colwidths"},
		{&Decisions{}, "Decisions"},
		{&Mention{}, "Mention"},
		{&Status{}, "Status"},
		{&MediaInline{}, "MediaInline"},
		{&Color{}, "Color"},
		{&Bg{}, "Bg"},
		{&Underline{}, "Underline"},
		{&Sub{}, "Sub"},
		{&Sup{}, "Sup"},
		{&Date{}, "Date"},
		{&Placeholder{}, "Placeholder"},
		{&Emoji{}, "Emoji"},
		{&Annotation{}, "Annotation"},
		{&FontSize{}, "FontSize"},
		{&InlineExtension{}, "InlineExtension"},
		{&Extension{}, "Extension"},
		{&BodiedExtension{}, "BodiedExtension"},
		{&Frame{}, "Frame"},
		{&SyncBlock{}, "SyncBlock"},
		{&BodiedSyncBlock{}, "BodiedSyncBlock"},
		{&Section{}, "Section"},
		{&Column{}, "Column"},
		{&MediaCaption{}, "MediaCaption"},
		{&Align{}, "Align"},
		{&Indent{}, "Indent"},
		{&Breakout{}, "Breakout"},
		{&DataConsumer{}, "DataConsumer"},
		{&Fragment{}, "Fragment"},
	}
	v := dialectNamer{}
	for _, tc := range cases {
		if got := Visit[string](tc.node, v); got != tc.want {
			t.Errorf("%s dispatched to %q, want %q", tc.node.Kind(), got, tc.want)
		}
	}
}
