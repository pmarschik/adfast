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

// Visit dispatches n to the matching Visitor method. The dispatch is one
// case per node kind, held in five switches grouped by ADF content
// category; the categories are probed in descending order of how often
// each kind occurs in a document, so the common inline text lands on the
// first switch.
//
// The interface above is what makes a visitor exhaustive: a new kind adds
// a method there and every implementation stops compiling until it
// handles it. The switches below are what makes the DISPATCH exhaustive,
// and nothing in the compiler checks them — a kind with no case here
// would degrade to VisitRaw in silence. TestVisitDispatchADF walks the
// full kind list and asserts each one reaches its own method, so the
// missing case fails a test instead.
func Visit[T any](n Node, v Visitor[T]) T {
	if r, ok := visitInlineKind(n, v); ok {
		return r
	}
	if r, ok := visitBlockKind(n, v); ok {
		return r
	}
	if r, ok := visitChildKind(n, v); ok {
		return r
	}
	if r, ok := visitMediaKind(n, v); ok {
		return r
	}
	if r, ok := visitExtensionKind(n, v); ok {
		return r
	}
	// Unreachable for decoder-produced trees (the set is sealed);
	// hand-built foreign implementations land here.
	return v.VisitRaw(&RawNode{Type: n.Kind()})
}

// visitInlineKind dispatches the kinds that live inside a paragraph.
func visitInlineKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Text:
		return v.VisitText(n), true
	case *HardBreak:
		return v.VisitHardBreak(n), true
	case *Emoji:
		return v.VisitEmoji(n), true
	case *Mention:
		return v.VisitMention(n), true
	case *Status:
		return v.VisitStatus(n), true
	case *Date:
		return v.VisitDate(n), true
	case *Placeholder:
		return v.VisitPlaceholder(n), true
	case *InlineCard:
		return v.VisitInlineCard(n), true
	case *MediaInline:
		return v.VisitMediaInline(n), true
	case *ColwidthsHint:
		return v.VisitColwidthsHint(n), true
	}
	var zero T
	return zero, false
}

// visitBlockKind dispatches the kinds that stand on their own at the top
// level of a document or inside another block's content.
func visitBlockKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Paragraph:
		return v.VisitParagraph(n), true
	case *Heading:
		return v.VisitHeading(n), true
	case *Blockquote:
		return v.VisitBlockquote(n), true
	case *Rule:
		return v.VisitRule(n), true
	case *CodeBlock:
		return v.VisitCodeBlock(n), true
	case *Panel:
		return v.VisitPanel(n), true
	case *Expand:
		return v.VisitExpand(n), true
	case *NestedExpand:
		return v.VisitNestedExpand(n), true
	case *LayoutSection:
		return v.VisitLayoutSection(n), true
	case *LayoutColumn:
		return v.VisitLayoutColumn(n), true
	case *Caption:
		return v.VisitCaption(n), true
	}
	var zero T
	return zero, false
}

// visitChildKind dispatches the kinds that only ever appear as a child of
// a specific parent: the list families and the table grid.
func visitChildKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *BulletList:
		return v.VisitBulletList(n), true
	case *OrderedList:
		return v.VisitOrderedList(n), true
	case *ListItem:
		return v.VisitListItem(n), true
	case *TaskList:
		return v.VisitTaskList(n), true
	case *TaskItem:
		return v.VisitTaskItem(n), true
	case *BlockTaskItem:
		return v.VisitBlockTaskItem(n), true
	case *DecisionList:
		return v.VisitDecisionList(n), true
	case *DecisionItem:
		return v.VisitDecisionItem(n), true
	case *Table:
		return v.VisitTable(n), true
	case *TableRow:
		return v.VisitTableRow(n), true
	case *TableCell:
		return v.VisitTableCell(n), true
	case *TableHeader:
		return v.VisitTableHeader(n), true
	}
	var zero T
	return zero, false
}

// visitMediaKind dispatches the media wrappers and the block-level cards.
func visitMediaKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *MediaSingle:
		return v.VisitMediaSingle(n), true
	case *MediaGroup:
		return v.VisitMediaGroup(n), true
	case *Media:
		return v.VisitMedia(n), true
	case *BlockCard:
		return v.VisitBlockCard(n), true
	case *EmbedCard:
		return v.VisitEmbedCard(n), true
	}
	var zero T
	return zero, false
}

// visitExtensionKind dispatches the extension points and the RawNode
// escape for kinds the decoder did not recognize.
func visitExtensionKind[T any](n Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Extension:
		return v.VisitExtensionNode(n), true
	case *InlineExtension:
		return v.VisitInlineExtension(n), true
	case *BodiedExtension:
		return v.VisitBodiedExtension(n), true
	case *MultiBodiedExtension:
		return v.VisitMultiBodiedExtension(n), true
	case *ExtensionFrame:
		return v.VisitExtensionFrame(n), true
	case *SyncBlock:
		return v.VisitSyncBlock(n), true
	case *BodiedSyncBlock:
		return v.VisitBodiedSyncBlock(n), true
	case *RawNode:
		return v.VisitRaw(n), true
	}
	var zero T
	return zero, false
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
//
// The two switches split the same way Visit's do, and
// TestVisitDispatchADF guards them the same way.
func VisitMark[T any](m Mark, v MarkVisitor[T]) T {
	if r, ok := visitInlineMark(m, v); ok {
		return r
	}
	if r, ok := visitBlockMark(m, v); ok {
		return r
	}
	return v.VisitRawMark(&RawMark{Type: m.Kind()})
}

// visitInlineMark dispatches the marks that style a run of text.
func visitInlineMark[T any](m Mark, v MarkVisitor[T]) (T, bool) {
	switch m := m.(type) {
	case *Strong:
		return v.VisitStrong(m), true
	case *Em:
		return v.VisitEm(m), true
	case *Strike:
		return v.VisitStrike(m), true
	case *Code:
		return v.VisitCode(m), true
	case *Underline:
		return v.VisitUnderline(m), true
	case *Link:
		return v.VisitLink(m), true
	case *TextColor:
		return v.VisitTextColor(m), true
	case *BackgroundColor:
		return v.VisitBackgroundColor(m), true
	case *SubSup:
		return v.VisitSubSup(m), true
	case *Annotation:
		return v.VisitAnnotation(m), true
	case *FontSize:
		return v.VisitFontSize(m), true
	}
	var zero T
	return zero, false
}

// visitBlockMark dispatches the marks that decorate a whole block, plus
// the RawMark escape for marks the decoder did not recognize.
func visitBlockMark[T any](m Mark, v MarkVisitor[T]) (T, bool) {
	switch m := m.(type) {
	case *Alignment:
		return v.VisitAlignment(m), true
	case *Indentation:
		return v.VisitIndentation(m), true
	case *Breakout:
		return v.VisitBreakout(m), true
	case *Border:
		return v.VisitBorder(m), true
	case *DataConsumer:
		return v.VisitDataConsumer(m), true
	case *Fragment:
		return v.VisitFragment(m), true
	case *RawMark:
		return v.VisitRawMark(m), true
	}
	var zero T
	return zero, false
}
