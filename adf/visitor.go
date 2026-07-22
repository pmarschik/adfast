package adf

// Visitor is the exhaustive, type-safe visitor over the ADF node kinds:
// implementing it requires one method per kind, so adding a kind breaks
// every visitor implementation at compile time. The type parameter is the
// visit result ("struct{}" for pure side effects).
//
// The node set is sealed; unknown wire content decodes to RawNode and
// dispatches to VisitRaw — a RawNode reaching a visitor is the "document
// contains something this schema doesn't model" signal (the decoder emits
// an unknown-node diagnostic when it creates one).
//
// Visitors receive single nodes and recurse themselves (via Visit over
// NodeContent(n)); for casual traversal use NodeContent directly.
type Visitor[T any] interface {
	VisitParagraph(*Paragraph) T
	VisitHeading(*Heading) T
	VisitBlockquote(*Blockquote) T
	VisitRule(*Rule) T
	VisitCodeBlock(*CodeBlock) T
	VisitBulletList(*BulletList) T
	VisitOrderedList(*OrderedList) T
	VisitListItem(*ListItem) T
	VisitTaskList(*TaskList) T
	VisitTaskItem(*TaskItem) T
	VisitDecisionList(*DecisionList) T
	VisitDecisionItem(*DecisionItem) T
	VisitTable(*Table) T
	VisitTableRow(*TableRow) T
	VisitTableCell(*TableCell) T
	VisitTableHeader(*TableHeader) T
	VisitPanel(*Panel) T
	VisitExpand(*Expand) T
	VisitNestedExpand(*NestedExpand) T
	VisitMediaSingle(*MediaSingle) T
	VisitMediaGroup(*MediaGroup) T
	VisitMedia(*Media) T
	VisitBlockCard(*BlockCard) T
	VisitEmbedCard(*EmbedCard) T
	VisitInlineCard(*InlineCard) T
	VisitText(*Text) T
	VisitHardBreak(*HardBreak) T
	VisitEmoji(*Emoji) T
	VisitMention(*Mention) T
	VisitStatus(*Status) T
	VisitMediaInline(*MediaInline) T
	VisitColwidthsHint(*ColwidthsHint) T
	VisitDate(*Date) T
	VisitPlaceholder(*Placeholder) T
	VisitCaption(*Caption) T
	VisitBlockTaskItem(*BlockTaskItem) T
	VisitLayoutSection(*LayoutSection) T
	VisitLayoutColumn(*LayoutColumn) T
	VisitExtensionNode(*Extension) T
	VisitInlineExtension(*InlineExtension) T
	VisitBodiedExtension(*BodiedExtension) T
	VisitMultiBodiedExtension(*MultiBodiedExtension) T
	VisitExtensionFrame(*ExtensionFrame) T
	VisitSyncBlock(*SyncBlock) T
	VisitBodiedSyncBlock(*BodiedSyncBlock) T
	// VisitRaw receives unknown node kinds preserved losslessly by the
	// decoder.
	VisitRaw(*RawNode) T
}

// Visit dispatches n to the matching Visitor method. This switch is the
// single dispatch point, maintained in-package next to the kind list.
func Visit[T any](n Node, v Visitor[T]) T {
	switch n := n.(type) {
	case *Paragraph:
		return v.VisitParagraph(n)
	case *Heading:
		return v.VisitHeading(n)
	case *Blockquote:
		return v.VisitBlockquote(n)
	case *Rule:
		return v.VisitRule(n)
	case *CodeBlock:
		return v.VisitCodeBlock(n)
	case *BulletList:
		return v.VisitBulletList(n)
	case *OrderedList:
		return v.VisitOrderedList(n)
	case *ListItem:
		return v.VisitListItem(n)
	case *TaskList:
		return v.VisitTaskList(n)
	case *TaskItem:
		return v.VisitTaskItem(n)
	case *DecisionList:
		return v.VisitDecisionList(n)
	case *DecisionItem:
		return v.VisitDecisionItem(n)
	case *Table:
		return v.VisitTable(n)
	case *TableRow:
		return v.VisitTableRow(n)
	case *TableCell:
		return v.VisitTableCell(n)
	case *TableHeader:
		return v.VisitTableHeader(n)
	case *Panel:
		return v.VisitPanel(n)
	case *Expand:
		return v.VisitExpand(n)
	case *NestedExpand:
		return v.VisitNestedExpand(n)
	case *MediaSingle:
		return v.VisitMediaSingle(n)
	case *MediaGroup:
		return v.VisitMediaGroup(n)
	case *Media:
		return v.VisitMedia(n)
	case *BlockCard:
		return v.VisitBlockCard(n)
	case *EmbedCard:
		return v.VisitEmbedCard(n)
	case *InlineCard:
		return v.VisitInlineCard(n)
	case *Text:
		return v.VisitText(n)
	case *HardBreak:
		return v.VisitHardBreak(n)
	case *Emoji:
		return v.VisitEmoji(n)
	case *Mention:
		return v.VisitMention(n)
	case *Status:
		return v.VisitStatus(n)
	case *MediaInline:
		return v.VisitMediaInline(n)
	case *ColwidthsHint:
		return v.VisitColwidthsHint(n)
	case *Date:
		return v.VisitDate(n)
	case *Placeholder:
		return v.VisitPlaceholder(n)
	case *Caption:
		return v.VisitCaption(n)
	case *BlockTaskItem:
		return v.VisitBlockTaskItem(n)
	case *LayoutSection:
		return v.VisitLayoutSection(n)
	case *LayoutColumn:
		return v.VisitLayoutColumn(n)
	case *Extension:
		return v.VisitExtensionNode(n)
	case *InlineExtension:
		return v.VisitInlineExtension(n)
	case *BodiedExtension:
		return v.VisitBodiedExtension(n)
	case *MultiBodiedExtension:
		return v.VisitMultiBodiedExtension(n)
	case *ExtensionFrame:
		return v.VisitExtensionFrame(n)
	case *SyncBlock:
		return v.VisitSyncBlock(n)
	case *BodiedSyncBlock:
		return v.VisitBodiedSyncBlock(n)
	case *RawNode:
		return v.VisitRaw(n)
	default:
		// Unreachable for decoder-produced trees (the set is sealed);
		// hand-built foreign implementations land here.
		return v.VisitRaw(&RawNode{Type: n.Kind()})
	}
}

// MarkVisitor is the exhaustive visitor over the mark kinds, with RawMark
// as the unknown-set escape (see Visitor).
type MarkVisitor[T any] interface {
	VisitStrong(*Strong) T
	VisitEm(*Em) T
	VisitStrike(*Strike) T
	VisitCode(*Code) T
	VisitUnderline(*Underline) T
	VisitLink(*Link) T
	VisitTextColor(*TextColor) T
	VisitBackgroundColor(*BackgroundColor) T
	VisitSubSup(*SubSup) T
	VisitAlignment(*Alignment) T
	VisitIndentation(*Indentation) T
	VisitBreakout(*Breakout) T
	VisitBorder(*Border) T
	VisitAnnotation(*Annotation) T
	VisitDataConsumer(*DataConsumer) T
	VisitFragment(*Fragment) T
	VisitFontSize(*FontSize) T
	// VisitRawMark receives unknown mark kinds preserved losslessly by
	// the decoder.
	VisitRawMark(*RawMark) T
}

// VisitMark dispatches m to the matching MarkVisitor method.
func VisitMark[T any](m Mark, v MarkVisitor[T]) T {
	switch m := m.(type) {
	case *Strong:
		return v.VisitStrong(m)
	case *Em:
		return v.VisitEm(m)
	case *Strike:
		return v.VisitStrike(m)
	case *Code:
		return v.VisitCode(m)
	case *Underline:
		return v.VisitUnderline(m)
	case *Link:
		return v.VisitLink(m)
	case *TextColor:
		return v.VisitTextColor(m)
	case *BackgroundColor:
		return v.VisitBackgroundColor(m)
	case *SubSup:
		return v.VisitSubSup(m)
	case *Alignment:
		return v.VisitAlignment(m)
	case *Indentation:
		return v.VisitIndentation(m)
	case *Breakout:
		return v.VisitBreakout(m)
	case *Border:
		return v.VisitBorder(m)
	case *Annotation:
		return v.VisitAnnotation(m)
	case *DataConsumer:
		return v.VisitDataConsumer(m)
	case *Fragment:
		return v.VisitFragment(m)
	case *FontSize:
		return v.VisitFontSize(m)
	case *RawMark:
		return v.VisitRawMark(m)
	default:
		return v.VisitRawMark(&RawMark{Type: m.Kind()})
	}
}
