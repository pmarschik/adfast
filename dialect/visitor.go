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

// Visit dispatches n to the matching Visitor method. This switch is the
// single dispatch point, maintained in-package next to the kind list.
func Visit[T any](n ast.Node, v Visitor[T]) T {
	switch n := n.(type) {
	case *Panel:
		return v.VisitPanel(n)
	case *Expand:
		return v.VisitExpand(n)
	case *Media:
		return v.VisitMedia(n)
	case *JQL:
		return v.VisitJQL(n)
	case *LinkCard:
		return v.VisitLinkCard(n)
	case *LinkEmbed:
		return v.VisitLinkEmbed(n)
	case *Colwidths:
		return v.VisitColwidths(n)
	case *Decisions:
		return v.VisitDecisions(n)
	case *Mention:
		return v.VisitMention(n)
	case *Status:
		return v.VisitStatus(n)
	case *MediaInline:
		return v.VisitMediaInline(n)
	case *Color:
		return v.VisitColor(n)
	case *Bg:
		return v.VisitBg(n)
	case *Underline:
		return v.VisitUnderline(n)
	case *Sub:
		return v.VisitSub(n)
	case *Sup:
		return v.VisitSup(n)
	case *Date:
		return v.VisitDate(n)
	case *Placeholder:
		return v.VisitPlaceholder(n)
	case *Emoji:
		return v.VisitEmoji(n)
	case *Annotation:
		return v.VisitAnnotation(n)
	case *FontSize:
		return v.VisitFontSize(n)
	case *InlineExtension:
		return v.VisitInlineExtension(n)
	case *Extension:
		return v.VisitExtension(n)
	case *BodiedExtension:
		return v.VisitBodiedExtension(n)
	case *Frame:
		return v.VisitFrame(n)
	case *SyncBlock:
		return v.VisitSyncBlock(n)
	case *BodiedSyncBlock:
		return v.VisitBodiedSyncBlock(n)
	case *Section:
		return v.VisitSection(n)
	case *Column:
		return v.VisitColumn(n)
	case *MediaCaption:
		return v.VisitMediaCaption(n)
	case *Align:
		return v.VisitAlign(n)
	case *Indent:
		return v.VisitIndent(n)
	case *Breakout:
		return v.VisitBreakout(n)
	case *DataConsumer:
		return v.VisitDataConsumer(n)
	case *Fragment:
		return v.VisitFragment(n)
	default:
		return v.VisitOther(n)
	}
}
