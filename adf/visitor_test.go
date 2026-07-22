package adf

import (
	"testing"
)

type adfKindNamer struct{}

func (adfKindNamer) VisitParagraph(*Paragraph) string         { return "Paragraph" }
func (adfKindNamer) VisitHeading(*Heading) string             { return "Heading" }
func (adfKindNamer) VisitBlockquote(*Blockquote) string       { return "Blockquote" }
func (adfKindNamer) VisitRule(*Rule) string                   { return "Rule" }
func (adfKindNamer) VisitCodeBlock(*CodeBlock) string         { return "CodeBlock" }
func (adfKindNamer) VisitBulletList(*BulletList) string       { return "BulletList" }
func (adfKindNamer) VisitOrderedList(*OrderedList) string     { return "OrderedList" }
func (adfKindNamer) VisitListItem(*ListItem) string           { return "ListItem" }
func (adfKindNamer) VisitTaskList(*TaskList) string           { return "TaskList" }
func (adfKindNamer) VisitTaskItem(*TaskItem) string           { return "TaskItem" }
func (adfKindNamer) VisitDecisionList(*DecisionList) string   { return "DecisionList" }
func (adfKindNamer) VisitDecisionItem(*DecisionItem) string   { return "DecisionItem" }
func (adfKindNamer) VisitTable(*Table) string                 { return "Table" }
func (adfKindNamer) VisitTableRow(*TableRow) string           { return "TableRow" }
func (adfKindNamer) VisitTableCell(*TableCell) string         { return "TableCell" }
func (adfKindNamer) VisitTableHeader(*TableHeader) string     { return "TableHeader" }
func (adfKindNamer) VisitPanel(*Panel) string                 { return "Panel" }
func (adfKindNamer) VisitExpand(*Expand) string               { return "Expand" }
func (adfKindNamer) VisitNestedExpand(*NestedExpand) string   { return "NestedExpand" }
func (adfKindNamer) VisitMediaSingle(*MediaSingle) string     { return "MediaSingle" }
func (adfKindNamer) VisitMediaGroup(*MediaGroup) string       { return "MediaGroup" }
func (adfKindNamer) VisitMedia(*Media) string                 { return "Media" }
func (adfKindNamer) VisitBlockCard(*BlockCard) string         { return "BlockCard" }
func (adfKindNamer) VisitEmbedCard(*EmbedCard) string         { return "EmbedCard" }
func (adfKindNamer) VisitInlineCard(*InlineCard) string       { return "InlineCard" }
func (adfKindNamer) VisitText(*Text) string                   { return "Text" }
func (adfKindNamer) VisitHardBreak(*HardBreak) string         { return "HardBreak" }
func (adfKindNamer) VisitEmoji(*Emoji) string                 { return "Emoji" }
func (adfKindNamer) VisitMention(*Mention) string             { return "Mention" }
func (adfKindNamer) VisitStatus(*Status) string               { return "Status" }
func (adfKindNamer) VisitMediaInline(*MediaInline) string     { return "MediaInline" }
func (adfKindNamer) VisitColwidthsHint(*ColwidthsHint) string { return "ColwidthsHint" }
func (adfKindNamer) VisitDate(*Date) string                   { return "Date" }
func (adfKindNamer) VisitPlaceholder(*Placeholder) string     { return "Placeholder" }
func (adfKindNamer) VisitCaption(*Caption) string             { return "Caption" }
func (adfKindNamer) VisitBlockTaskItem(*BlockTaskItem) string { return "BlockTaskItem" }
func (adfKindNamer) VisitLayoutSection(*LayoutSection) string { return "LayoutSection" }
func (adfKindNamer) VisitLayoutColumn(*LayoutColumn) string   { return "LayoutColumn" }
func (adfKindNamer) VisitExtensionNode(*Extension) string     { return "Extension" }
func (adfKindNamer) VisitInlineExtension(*InlineExtension) string {
	return "InlineExtension"
}

func (adfKindNamer) VisitBodiedExtension(*BodiedExtension) string {
	return "BodiedExtension"
}

func (adfKindNamer) VisitMultiBodiedExtension(*MultiBodiedExtension) string {
	return "MultiBodiedExtension"
}

func (adfKindNamer) VisitExtensionFrame(*ExtensionFrame) string {
	return "ExtensionFrame"
}
func (adfKindNamer) VisitSyncBlock(*SyncBlock) string             { return "SyncBlock" }
func (adfKindNamer) VisitBodiedSyncBlock(*BodiedSyncBlock) string { return "BodiedSyncBlock" }

func (adfKindNamer) VisitRaw(n *RawNode) string { return "raw:" + n.Type }

type markCounter struct{}

func (markCounter) VisitStrong(*Strong) int                   { return 1 }
func (markCounter) VisitEm(*Em) int                           { return 1 }
func (markCounter) VisitStrike(*Strike) int                   { return 1 }
func (markCounter) VisitCode(*Code) int                       { return 1 }
func (markCounter) VisitUnderline(*Underline) int             { return 1 }
func (markCounter) VisitLink(*Link) int                       { return 2 }
func (markCounter) VisitTextColor(*TextColor) int             { return 1 }
func (markCounter) VisitBackgroundColor(*BackgroundColor) int { return 1 }
func (markCounter) VisitSubSup(*SubSup) int                   { return 1 }
func (markCounter) VisitAlignment(*Alignment) int             { return 1 }
func (markCounter) VisitIndentation(*Indentation) int         { return 1 }
func (markCounter) VisitBreakout(*Breakout) int               { return 1 }
func (markCounter) VisitBorder(*Border) int                   { return 1 }
func (markCounter) VisitAnnotation(*Annotation) int           { return 1 }
func (markCounter) VisitDataConsumer(*DataConsumer) int       { return 1 }
func (markCounter) VisitFragment(*Fragment) int               { return 1 }
func (markCounter) VisitFontSize(*FontSize) int               { return 1 }
func (markCounter) VisitRawMark(*RawMark) int                 { return 0 }

func TestVisitDispatchADF(t *testing.T) {
	var _ Visitor[string] = adfKindNamer{} // compile-time exhaustiveness
	var _ MarkVisitor[int] = markCounter{} // compile-time exhaustiveness

	v := adfKindNamer{}
	if got := Visit[string](&Panel{}, v); got != "Panel" {
		t.Errorf("panel: %q", got)
	}
	if got := Visit[string](&RawNode{Type: "futureNode"}, v); got != "raw:futureNode" {
		t.Errorf("raw routing: %q", got)
	}
	if got := VisitMark[int](&Link{}, markCounter{}); got != 2 {
		t.Errorf("mark dispatch: %d", got)
	}
	if got := VisitMark[int](&RawMark{Type: "sparkles"}, markCounter{}); got != 0 {
		t.Errorf("raw mark: %d", got)
	}
}
