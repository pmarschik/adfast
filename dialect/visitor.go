package dialect

import (
	"github.com/pmarschik/adfast/ast"
)

// Visitor is the exhaustive, type-safe visitor over the dialect's typed
// directive kinds — the extension-level counterpart of ast.Visitor. Call
// Visit from your ast.Visitor's VisitExtension to extend the compile-time
// exhaustiveness chain down into the dialect: implementing this interface
// requires one method per kind, so a new dialect directive breaks every
// implementation at compile time.
//
// Kinds that are not part of the dialect (third-party extension nodes, or
// private kinds) dispatch to VisitOther — chain further extension
// visitors there. This escape-and-chain shape is the recommended pattern
// for any extension package that ships its own node kinds.
type Visitor[T any] interface {
	VisitPanel(*Panel) T
	VisitExpand(*Expand) T
	VisitMedia(*Media) T
	VisitJQL(*JQL) T
	VisitLinkCard(*LinkCard) T
	VisitLinkEmbed(*LinkEmbed) T
	VisitColwidths(*Colwidths) T
	VisitDecisions(*Decisions) T
	VisitMention(*Mention) T
	VisitStatus(*Status) T
	VisitMediaInline(*MediaInline) T
	VisitColor(*Color) T
	VisitBg(*Bg) T
	VisitUnderline(*Underline) T
	VisitSub(*Sub) T
	VisitSup(*Sup) T
	VisitDate(*Date) T
	VisitPlaceholder(*Placeholder) T
	VisitEmoji(*Emoji) T
	VisitAnnotation(*Annotation) T
	VisitFontSize(*FontSize) T
	VisitInlineExtension(*InlineExtension) T
	VisitExtension(*Extension) T
	VisitBodiedExtension(*BodiedExtension) T
	VisitFrame(*Frame) T
	VisitSyncBlock(*SyncBlock) T
	VisitBodiedSyncBlock(*BodiedSyncBlock) T
	VisitSection(*Section) T
	VisitColumn(*Column) T
	VisitMediaCaption(*MediaCaption) T
	VisitAlign(*Align) T
	VisitIndent(*Indent) T
	VisitBreakout(*Breakout) T
	VisitDataConsumer(*DataConsumer) T
	VisitFragment(*Fragment) T
	// VisitOther receives every node kind the dialect does not define.
	VisitOther(ast.Node) T
}

// Visit dispatches n to the matching Visitor method. The kind list is
// split by category across the visit*Kind helpers below; each answers
// ok=false for a kind outside its category, and the fallthrough is the
// escape-and-chain contract: anything none of them claims is not a
// dialect kind.
func Visit[T any](n ast.Node, v Visitor[T]) T {
	if r, ok := visitBlockKind(n, v); ok {
		return r
	}
	if r, ok := visitInlineKind(n, v); ok {
		return r
	}
	if r, ok := visitExtensionKind(n, v); ok {
		return r
	}
	if r, ok := visitBlockMarkKind(n, v); ok {
		return r
	}
	return v.VisitOther(n)
}

// visitBlockKind dispatches the block-level directives.
func visitBlockKind[T any](n ast.Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Panel:
		return v.VisitPanel(n), true
	case *Expand:
		return v.VisitExpand(n), true
	case *Media:
		return v.VisitMedia(n), true
	case *JQL:
		return v.VisitJQL(n), true
	case *LinkCard:
		return v.VisitLinkCard(n), true
	case *LinkEmbed:
		return v.VisitLinkEmbed(n), true
	case *Colwidths:
		return v.VisitColwidths(n), true
	case *Decisions:
		return v.VisitDecisions(n), true
	}
	var zero T
	return zero, false
}

// visitInlineKind dispatches the inline directives: the atoms and the
// text-span decorations.
func visitInlineKind[T any](n ast.Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Mention:
		return v.VisitMention(n), true
	case *Status:
		return v.VisitStatus(n), true
	case *MediaInline:
		return v.VisitMediaInline(n), true
	case *Color:
		return v.VisitColor(n), true
	case *Bg:
		return v.VisitBg(n), true
	case *Underline:
		return v.VisitUnderline(n), true
	case *Sub:
		return v.VisitSub(n), true
	case *Sup:
		return v.VisitSup(n), true
	case *Date:
		return v.VisitDate(n), true
	case *Placeholder:
		return v.VisitPlaceholder(n), true
	case *Emoji:
		return v.VisitEmoji(n), true
	case *Annotation:
		return v.VisitAnnotation(n), true
	case *FontSize:
		return v.VisitFontSize(n), true
	}
	var zero T
	return zero, false
}

// visitExtensionKind dispatches the ADF extension points and the layout
// wrappers built on them.
func visitExtensionKind[T any](n ast.Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *InlineExtension:
		return v.VisitInlineExtension(n), true
	case *Extension:
		return v.VisitExtension(n), true
	case *BodiedExtension:
		return v.VisitBodiedExtension(n), true
	case *Frame:
		return v.VisitFrame(n), true
	case *SyncBlock:
		return v.VisitSyncBlock(n), true
	case *BodiedSyncBlock:
		return v.VisitBodiedSyncBlock(n), true
	case *Section:
		return v.VisitSection(n), true
	case *Column:
		return v.VisitColumn(n), true
	case *MediaCaption:
		return v.VisitMediaCaption(n), true
	}
	var zero T
	return zero, false
}

// visitBlockMarkKind dispatches the directives that decorate a whole
// block rather than standing on their own.
func visitBlockMarkKind[T any](n ast.Node, v Visitor[T]) (T, bool) {
	switch n := n.(type) {
	case *Align:
		return v.VisitAlign(n), true
	case *Indent:
		return v.VisitIndent(n), true
	case *Breakout:
		return v.VisitBreakout(n), true
	case *DataConsumer:
		return v.VisitDataConsumer(n), true
	case *Fragment:
		return v.VisitFragment(n), true
	}
	var zero T
	return zero, false
}
