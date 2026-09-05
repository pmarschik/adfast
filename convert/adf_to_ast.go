package convert

import (
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/extension"
)

// tableColwidths joins the first table row's cell colwidth attrs in column
// order ("79,320") — Jira repeats the same array on every row; empty when
// no cell carries widths.
func tableColwidths(table *adf.Table) string {
	for _, rowNode := range table.Content {
		row, ok := rowNode.(*adf.TableRow)
		if !ok {
			continue
		}
		var parts []string
		found := false
		for _, cell := range row.Content {
			for _, f := range cellColwidths(cell) {
				parts = append(parts, formatJSNumber(f))
				found = true
			}
		}
		if found {
			return strings.Join(parts, ",")
		}
		return ""
	}
	return ""
}

// cellColwidths returns a table cell's colwidth array (nil when absent).
func cellColwidths(cell adf.Node) []float64 {
	switch c := cell.(type) {
	case *adf.TableCell:
		return c.Colwidth
	case *adf.TableHeader:
		return c.Colwidth
	case *adf.RawNode:
		cw, ok := c.Attrs["colwidth"].([]any)
		if !ok {
			return nil
		}
		var out []float64
		for _, v := range cw {
			if f, ok := v.(float64); ok {
				out = append(out, f)
			}
		}
		return out
	}
	return nil
}

// cellSpans returns a table cell's colspan/rowspan (0 when absent; call
// sites clamp to 1 like the historical IntAttr defaults).
func cellSpans(cell adf.Node) (colspan, rowspan int) {
	switch c := cell.(type) {
	case *adf.TableCell:
		return c.Colspan, c.Rowspan
	case *adf.TableHeader:
		return c.Colspan, c.Rowspan
	case *adf.RawNode:
		return adf.IntAttr(c.Attrs, "colspan", 1), adf.IntAttr(c.Attrs, "rowspan", 1)
	}
	return 0, 0
}

// formatJSNumber renders a float the way JavaScript String(n) does for the
// JSON numbers ADF carries ("686", "20.5").
func formatJSNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// smartLinkLabel is the directive label for a smart-link card: the short
// key when the configured SmartLinks resolver knows the URL (mirrors
// inlineCard rendering), the full URL otherwise.
func smartLinkLabel(rc renderCtx, url string) string {
	if rc.smartLinks.KeyFromURL != nil {
		if key, ok := rc.smartLinks.KeyFromURL(url); ok {
			return key
		}
	}
	return url
}

// renderCtx carries per-conversion configuration through the ADF→AST walk.
type renderCtx struct {
	smartLinks          SmartLinks
	linkResolver        LinkResolver
	fileCards           FileCards
	assets              mediaAssets
	diagnostics         func(Diagnostic)
	blockHooks          []func(adf.Node, extension.DecodeContext) (ast.Node, bool)
	blockListHooks      []func(adf.Node, extension.DecodeContext) ([]ast.Node, bool)
	inlineHooks         []func(adf.Node, extension.DecodeContext) ([]ast.Node, bool)
	markHooks           []func(adf.Mark, []ast.Node) (ast.Node, bool)
	preserveLocalImages bool
	incrementLists      bool
}

// fileCardLink answers with the link a card reads back as: what the resolver
// says, then the link resolver's own way home, and a label that falls back to
// the card's alt text and then to the filename the href ends in.
func (rc renderCtx) fileCardLink(n *adf.MediaInline) (FileCardLink, bool) {
	if rc.fileCards.Link == nil || n.ID == "" {
		return FileCardLink{}, false
	}
	link, ok := rc.fileCards.Link(n.ID)
	if !ok || link.Href == "" {
		return FileCardLink{}, false
	}
	if rc.linkResolver.Decode != nil {
		if resolved, decoded := rc.linkResolver.Decode(link.Href); decoded {
			link.Href = resolved
		}
	}
	if link.Label == "" {
		link.Label = n.Alt
	}
	if link.Label == "" {
		link.Label = path.Base(link.Href)
	}
	return link, true
}

// mediaInlineAsImage answers with the inline image a mediaInline came
// from, or nil to leave it a :media directive. It is the inline mirror
// of the block fileMediaAsImage, and declines on the same principle:
// only a card that carries nothing beyond what ![alt](path) can say
// goes back to an image, so anything the directive alone can express
// (a collection, an occurrence key, a border, a non-file type) keeps the
// directive rather than losing that detail silently.
//
// A LINK mark is the exception, because markdown says it perfectly:
// [![alt](path)](href) is the linked image, and it is the exact form the
// encode leg turns back into a mediaInline carrying the same mark. Any
// other mark alongside it sends the card back to the directive, which
// carries both as attributes.
func (rc renderCtx) mediaInlineAsImage(n *adf.MediaInline) ast.Node {
	if n.Type != "file" || n.ID == "" {
		return nil
	}
	if n.Collection != nil && *n.Collection != "" {
		return nil
	}
	if adf.HasExtra(n, "occurrenceKey") {
		return nil
	}
	href, hasLink := linkMarkHref(n.Marks)
	if len(n.Marks) > 1 || (len(n.Marks) == 1 && !hasLink) {
		return nil
	}
	asset, ok := rc.assets.lookup(n.ID)
	if !ok || asset.Path == "" {
		return nil
	}
	img := &ast.Image{URL: asset.Path}
	if n.Alt != "" {
		img.Children = []ast.Node{&ast.Text{Value: n.Alt}}
	}
	if href == "" {
		return img
	}
	// Explicit: the resource form is what a markdown parse of this
	// output produces, so the projection and the parse agree on the node.
	return &ast.Link{URL: href, Explicit: true, Children: []ast.Node{img}}
}

// linkMarkHref answers the first link mark's destination, mirroring
// dialect's helper of the same name: an absent or empty href is no
// destination at all.
func linkMarkHref(marks []adf.Mark) (string, bool) {
	link, ok := adf.FindMark[*adf.Link](marks)
	if !ok || link.Href == nil || *link.Href == "" {
		return "", false
	}
	return *link.Href, true
}

// reportRawNode emits the raw-node diagnostic: the markdown projection
// met an unknown (RawNode) kind and either recursed into its content or
// dropped it.
func (rc renderCtx) reportRawNode(raw *adf.RawNode) {
	if rc.diagnostics == nil {
		return
	}
	action := "dropped"
	if len(raw.Content) > 0 {
		action = "projected through its first child"
	}
	rc.diagnostics(Diagnostic{
		Code:    CodeRawNode,
		Message: "unknown ADF node " + strconv.Quote(raw.Type) + " has no markdown form; " + action,
	})
}

// addExtensions validates the registrations and indexes their decode
// hooks for dispatch.
func (rc *renderCtx) addExtensions(regs []extension.Registration) {
	for _, reg := range regs {
		if err := reg.Validate(); err != nil {
			panic(err)
		}
		if reg.DecodeBlock != nil {
			rc.blockHooks = append(rc.blockHooks, reg.DecodeBlock)
		}
		if reg.DecodeBlockList != nil {
			rc.blockListHooks = append(rc.blockListHooks, reg.DecodeBlockList)
		}
		if reg.DecodeInline != nil {
			rc.inlineHooks = append(rc.inlineHooks, reg.DecodeInline)
		}
		if reg.DecodeTextMark != nil {
			rc.markHooks = append(rc.markHooks, reg.DecodeTextMark)
		}
	}
}

// decodeContext implements extension.DecodeContext over renderCtx.
type decodeContext struct {
	rc renderCtx
}

// DecodeBlocks implements extension.DecodeContext.
func (d *decodeContext) DecodeBlocks(nodes []adf.Node) []ast.Node {
	return convertAdfBlocks(nodes, d.rc)
}

// DecodeInlines implements extension.DecodeContext.
func (d *decodeContext) DecodeInlines(nodes []adf.Node) []ast.Node {
	return convertAdfInlines(nodes, d.rc)
}

// SmartLinkLabel implements extension.DecodeContext.
func (d *decodeContext) SmartLinkLabel(url string) string {
	return smartLinkLabel(d.rc, url)
}

// Asset implements extension.DecodeContext.
func (d *decodeContext) Asset(id string) (extension.MediaAsset, bool) {
	return d.rc.assets.lookup(id)
}

// PreserveLocalImages implements extension.DecodeContext.
func (d *decodeContext) PreserveLocalImages() bool {
	return d.rc.preserveLocalImages
}

// decodeBlockHook dispatches an ADF block node to the registered
// DecodeBlock hooks, in registration order.
func decodeBlockHook(node adf.Node, rc renderCtx) (ast.Node, bool) {
	ctx := &decodeContext{rc: rc}
	for _, hook := range rc.blockHooks {
		if n, ok := hook(node, ctx); ok {
			return n, true
		}
	}
	return nil, false
}

// decodeBlockListHook dispatches an ADF block node to the registered
// DecodeBlockList hooks (one-to-many decodes), in registration order.
func decodeBlockListHook(node adf.Node, rc renderCtx) ([]ast.Node, bool) {
	if len(rc.blockListHooks) == 0 {
		return nil, false
	}
	ctx := &decodeContext{rc: rc}
	for _, hook := range rc.blockListHooks {
		if nodes, ok := hook(node, ctx); ok {
			return nodes, true
		}
	}
	return nil, false
}

// decodeInlineHook dispatches an ADF inline node to the registered
// DecodeInline hooks, in registration order.
func decodeInlineHook(node adf.Node, rc renderCtx) ([]ast.Node, bool) {
	ctx := &decodeContext{rc: rc}
	for _, hook := range rc.inlineHooks {
		if nodes, ok := hook(node, ctx); ok {
			return nodes, true
		}
	}
	return nil, false
}

// applyTextMarkHook dispatches one projected text mark to the registered
// DecodeTextMark hooks, in registration order, wrapping node in the
// claimed result; an unclaimed mark leaves the node unwrapped (the mark
// drops, matching the historical behavior for unknown marks).
func applyTextMarkHook(mark adf.Mark, node ast.Node, rc renderCtx) ast.Node {
	inner := []ast.Node{node}
	for _, hook := range rc.markHooks {
		if wrapped, ok := hook(mark, inner); ok {
			return wrapped
		}
	}
	return node
}

// FromADF converts an ADF document to an AST root node. This is the
// AST→AST half of ToMarkdown; the renderer (markdown.Render) turns the
// AST tree into Markdown text. The registered extension kinds (the
// dialect set by default, plus WithExtensions) decode the ADF shapes
// they own into typed nodes; user hooks are tried before the dialect's,
// so a user registration can override a dialect decode (see the
// extension package's conflict policy). With WithDiagnostics, a RawNode
// reaching the projection reports a "raw-node" diagnostic.
func FromADF(doc adf.Doc, opts ...Option) ast.Node {
	cfg := config{}
	for _, o := range opts {
		o(&cfg)
	}
	if err := extension.ValidateSet(cfg.extensions); err != nil {
		panic(err)
	}
	rc := renderCtx{
		assets: newMediaAssets(cfg), smartLinks: cfg.smartLinks, linkResolver: cfg.linkResolver,
		fileCards:           cfg.fileCards,
		diagnostics:         cfg.diagnostics,
		preserveLocalImages: cfg.preserveLocalImages, incrementLists: cfg.incrementListMarkers,
	}
	// User registrations first: their decode hooks win over the dialect.
	regs := make([]extension.Registration, 0, len(cfg.extensions)+len(dialect.Registrations()))
	regs = append(regs, cfg.extensions...)
	regs = append(regs, dialect.Registrations()...)
	rc.addExtensions(regs)
	return &ast.Root{Children: convertAdfBlocks(doc.Content, rc)}
}

// ---------------------------------------------------------------------------
// Block conversion: ADF block nodes → AST block nodes
// ---------------------------------------------------------------------------

func convertAdfBlocks(nodes []adf.Node, rc renderCtx) []ast.Node {
	v := &adfBlockVisitor{rc: rc}
	var out []ast.Node
	for _, node := range nodes {
		var produced []ast.Node
		// Table column widths precede the table as a ::colwidths node
		// (GFM table syntax cannot carry them). The emission is
		// cross-sibling and therefore structural — it stays here rather
		// than inside the Colwidths kind (see the dialect package).
		switch typed := node.(type) {
		case *adf.Table:
			if widths := tableColwidths(typed); widths != "" {
				produced = append(produced, &dialect.Colwidths{
					Children: []ast.Node{&ast.Text{Value: widths}},
				})
			}
			if n := adf.Visit(node, v); n != nil {
				produced = append(produced, n)
			}
		case *adf.DecisionList:
			// A decisionList decodes as ::decisions followed by a plain
			// bullet list (the inverse of the encode-side marking); the
			// emission is cross-sibling and therefore structural, like the
			// table's ::colwidths companion above.
			produced = append(produced, &dialect.Decisions{})
			if n := adf.Visit(node, v); n != nil {
				produced = append(produced, n)
			}
		default:
			if decoded, ok := decodeBlockListHook(node, rc); ok {
				// One-to-many decodes (a media group flattening to one
				// ::media node per attachment) replace the ADF node
				// verbatim.
				produced = decoded
			} else if n := adf.Visit(node, v); n != nil {
				produced = []ast.Node{n}
			}
		}
		// Block marks (alignment, indentation, breakout, dataConsumer,
		// fragment) wrap everything the node produced — including a
		// table's ::colwidths companion, so the pair stays adjacent
		// inside the wrapper for the encode-side reattachment.
		produced = wrapBlockMarks(node, produced)
		out = append(out, produced...)
	}
	return out
}

// wrapBlockMarks wraps a block's decoded nodes in the mark-wrapper
// container directives for the block marks the ADF node carries. The
// canonical nesting order maps the ADF mark array inside-out: the FIRST
// mark becomes the innermost wrapper (its EncodeADF appends first on
// the way back), so ADF → md → ADF preserves the mark order exactly.
// Marks without a wrapper form (and anything on inline nodes) are
// dropped, as before.
func wrapBlockMarks(node adf.Node, kids []ast.Node) []ast.Node {
	if len(kids) == 0 {
		return kids
	}
	for _, mark := range adf.NodeMarks(node) {
		if wrapper := blockMarkWrapper(mark, kids); wrapper != nil {
			kids = []ast.Node{wrapper}
		}
	}
	return kids
}

// blockMarkWrapper builds the wrapper directive node for one block
// mark (nil for kinds without a wrapper form).
func blockMarkWrapper(mark adf.Mark, kids []ast.Node) ast.Node {
	switch m := mark.(type) {
	case *adf.Alignment:
		if m.Align != "center" && m.Align != "end" {
			return nil
		}
		return &dialect.Align{Align: m.Align, Children: kids}
	case *adf.Indentation:
		if m.Level < 1 {
			return nil
		}
		return &dialect.Indent{
			Attrs:    map[string]string{strconv.Itoa(m.Level): ""},
			Children: kids,
		}
	case *adf.Breakout:
		if m.Mode == "" {
			return nil
		}
		attrs := map[string]string{m.Mode: ""}
		if m.Width != nil {
			attrs["width"] = formatJSNumber(*m.Width)
		}
		return &dialect.Breakout{Attrs: attrs, Children: kids}
	case *adf.DataConsumer:
		if len(m.Sources) == 0 {
			return nil
		}
		return &dialect.DataConsumer{
			Attrs:    map[string]string{"sources": dialect.EncodeSources(m.Sources)},
			Children: kids,
		}
	case *adf.Fragment:
		if m.LocalID == "" {
			return nil
		}
		attrs := map[string]string{"localId": m.LocalID}
		if m.Name != "" {
			attrs["name"] = m.Name
		}
		return &dialect.Fragment{Attrs: attrs, Children: kids}
	}
	return nil
}

// adfBlockVisitor converts one ADF node in block position to its AST
// block node (nil drops it). Implementing adf.Visitor[ast.Node] keeps
// the projection exhaustive: a new ADF kind fails compilation here until
// it gets an explicit conversion or the extension-hook fallback.
type adfBlockVisitor struct {
	rc renderCtx
}

// VisitParagraph implements adf.Visitor.
func (v *adfBlockVisitor) VisitParagraph(n *adf.Paragraph) ast.Node {
	return &ast.Paragraph{Children: convertAdfInlines(n.Content, v.rc)}
}

// VisitHeading implements adf.Visitor.
func (v *adfBlockVisitor) VisitHeading(n *adf.Heading) ast.Node {
	level := min(max(n.Level, 1), 6)
	// Anchor is the synthetic never-wire attribute (see adf.Heading); a
	// product addon lifts its own anchor construct into it before decode.
	return &ast.Heading{Depth: level, ID: n.Anchor, Children: convertAdfInlines(n.Content, v.rc)}
}

// VisitRule implements adf.Visitor.
func (*adfBlockVisitor) VisitRule(*adf.Rule) ast.Node {
	return &ast.ThematicBreak{}
}

// VisitBlockquote implements adf.Visitor.
func (v *adfBlockVisitor) VisitBlockquote(n *adf.Blockquote) ast.Node {
	return &ast.Blockquote{Children: convertAdfBlocks(n.Content, v.rc)}
}

// VisitCodeBlock implements adf.Visitor.
func (*adfBlockVisitor) VisitCodeBlock(n *adf.CodeBlock) ast.Node {
	return convertAdfCodeBlock(n)
}

// VisitBulletList implements adf.Visitor.
func (v *adfBlockVisitor) VisitBulletList(n *adf.BulletList) ast.Node {
	return convertAdfListItems(adfListShape{tight: n.Tight, start: 1}, n.Content, v.rc)
}

// VisitOrderedList implements adf.Visitor.
func (v *adfBlockVisitor) VisitOrderedList(n *adf.OrderedList) ast.Node {
	start := 1
	if n.Order != nil {
		start = *n.Order
	}
	return convertAdfListItems(adfListShape{ordered: true, start: start, tight: n.Tight}, n.Content, v.rc)
}

// VisitTaskList implements adf.Visitor.
func (v *adfBlockVisitor) VisitTaskList(n *adf.TaskList) ast.Node {
	return convertAdfTaskList(n, v.rc)
}

// VisitDecisionList implements adf.Visitor.
func (v *adfBlockVisitor) VisitDecisionList(n *adf.DecisionList) ast.Node {
	return convertAdfDecisionList(n, v.rc)
}

// VisitTable implements adf.Visitor.
func (v *adfBlockVisitor) VisitTable(n *adf.Table) ast.Node {
	return convertAdfTable(n, v.rc)
}

// blockFallback handles every kind without a dedicated block conversion.
// Extension kinds (panel, expand, cards, media, …) decode via
// the registered hooks.
func (v *adfBlockVisitor) blockFallback(node adf.Node) ast.Node {
	if decoded, ok := decodeBlockHook(node, v.rc); ok {
		return decoded
	}
	// Unknown block: try to recurse into the first content child.
	if kids := adf.NodeContent(node); len(kids) > 0 {
		return adf.Visit(kids[0], v)
	}
	return nil
}

// VisitRaw implements adf.Visitor: a decode hook may still claim the
// unknown kind; otherwise the raw-node diagnostic fires before the
// recurse-into-first-child/drop fallback.
func (v *adfBlockVisitor) VisitRaw(n *adf.RawNode) ast.Node {
	if decoded, ok := decodeBlockHook(n, v.rc); ok {
		return decoded
	}
	v.rc.reportRawNode(n)
	// Unknown block: try to recurse into the first content child.
	if kids := adf.NodeContent(n); len(kids) > 0 {
		return adf.Visit(kids[0], v)
	}
	return nil
}

// The remaining kinds have no dedicated block conversion: the extension
// kinds decode via their registered hooks, structural children (list
// items, table rows/cells) and inline kinds in block position degrade
// through the recurse-into-first-child fallback.

// VisitListItem implements adf.Visitor.
func (v *adfBlockVisitor) VisitListItem(n *adf.ListItem) ast.Node { return v.blockFallback(n) }

// VisitTaskItem implements adf.Visitor.
func (v *adfBlockVisitor) VisitTaskItem(n *adf.TaskItem) ast.Node { return v.blockFallback(n) }

// VisitDecisionItem implements adf.Visitor.
func (v *adfBlockVisitor) VisitDecisionItem(n *adf.DecisionItem) ast.Node { return v.blockFallback(n) }

// VisitTableRow implements adf.Visitor.
func (v *adfBlockVisitor) VisitTableRow(n *adf.TableRow) ast.Node { return v.blockFallback(n) }

// VisitTableCell implements adf.Visitor.
func (v *adfBlockVisitor) VisitTableCell(n *adf.TableCell) ast.Node { return v.blockFallback(n) }

// VisitTableHeader implements adf.Visitor.
func (v *adfBlockVisitor) VisitTableHeader(n *adf.TableHeader) ast.Node { return v.blockFallback(n) }

// VisitPanel implements adf.Visitor.
func (v *adfBlockVisitor) VisitPanel(n *adf.Panel) ast.Node { return v.blockFallback(n) }

// VisitExpand implements adf.Visitor.
func (v *adfBlockVisitor) VisitExpand(n *adf.Expand) ast.Node { return v.blockFallback(n) }

// VisitNestedExpand implements adf.Visitor.
func (v *adfBlockVisitor) VisitNestedExpand(n *adf.NestedExpand) ast.Node { return v.blockFallback(n) }

// VisitMediaSingle implements adf.Visitor.
func (v *adfBlockVisitor) VisitMediaSingle(n *adf.MediaSingle) ast.Node { return v.blockFallback(n) }

// VisitMediaGroup implements adf.Visitor.
func (v *adfBlockVisitor) VisitMediaGroup(n *adf.MediaGroup) ast.Node { return v.blockFallback(n) }

// VisitMedia implements adf.Visitor.
func (v *adfBlockVisitor) VisitMedia(n *adf.Media) ast.Node { return v.blockFallback(n) }

// VisitBlockCard implements adf.Visitor.
func (v *adfBlockVisitor) VisitBlockCard(n *adf.BlockCard) ast.Node { return v.blockFallback(n) }

// VisitEmbedCard implements adf.Visitor.
func (v *adfBlockVisitor) VisitEmbedCard(n *adf.EmbedCard) ast.Node { return v.blockFallback(n) }

// VisitInlineCard implements adf.Visitor.
func (v *adfBlockVisitor) VisitInlineCard(n *adf.InlineCard) ast.Node { return v.blockFallback(n) }

// VisitText implements adf.Visitor.
func (v *adfBlockVisitor) VisitText(n *adf.Text) ast.Node { return v.blockFallback(n) }

// VisitHardBreak implements adf.Visitor.
func (v *adfBlockVisitor) VisitHardBreak(n *adf.HardBreak) ast.Node { return v.blockFallback(n) }

// VisitEmoji implements adf.Visitor.
func (v *adfBlockVisitor) VisitEmoji(n *adf.Emoji) ast.Node { return v.blockFallback(n) }

// VisitMention implements adf.Visitor.
func (v *adfBlockVisitor) VisitMention(n *adf.Mention) ast.Node { return v.blockFallback(n) }

// VisitStatus implements adf.Visitor.
func (v *adfBlockVisitor) VisitStatus(n *adf.Status) ast.Node { return v.blockFallback(n) }

// VisitMediaInline implements adf.Visitor.
func (v *adfBlockVisitor) VisitMediaInline(n *adf.MediaInline) ast.Node { return v.blockFallback(n) }

// VisitColwidthsHint implements adf.Visitor.
func (v *adfBlockVisitor) VisitColwidthsHint(n *adf.ColwidthsHint) ast.Node {
	return v.blockFallback(n)
}

// VisitDate implements adf.Visitor.
func (v *adfBlockVisitor) VisitDate(n *adf.Date) ast.Node { return v.blockFallback(n) }

// VisitPlaceholder implements adf.Visitor.
func (v *adfBlockVisitor) VisitPlaceholder(n *adf.Placeholder) ast.Node { return v.blockFallback(n) }

// VisitCaption implements adf.Visitor.
func (v *adfBlockVisitor) VisitCaption(n *adf.Caption) ast.Node { return v.blockFallback(n) }

// VisitBlockTaskItem implements adf.Visitor.
func (v *adfBlockVisitor) VisitBlockTaskItem(n *adf.BlockTaskItem) ast.Node {
	return v.blockFallback(n)
}

// VisitLayoutSection implements adf.Visitor.
func (v *adfBlockVisitor) VisitLayoutSection(n *adf.LayoutSection) ast.Node {
	return v.blockFallback(n)
}

// VisitLayoutColumn implements adf.Visitor.
func (v *adfBlockVisitor) VisitLayoutColumn(n *adf.LayoutColumn) ast.Node {
	return v.blockFallback(n)
}

// VisitExtensionNode implements adf.Visitor.
func (v *adfBlockVisitor) VisitExtensionNode(n *adf.Extension) ast.Node {
	return v.blockFallback(n)
}

// VisitInlineExtension implements adf.Visitor.
func (v *adfBlockVisitor) VisitInlineExtension(n *adf.InlineExtension) ast.Node {
	return v.blockFallback(n)
}

// VisitBodiedExtension implements adf.Visitor.
func (v *adfBlockVisitor) VisitBodiedExtension(n *adf.BodiedExtension) ast.Node {
	return v.blockFallback(n)
}

// VisitMultiBodiedExtension implements adf.Visitor.
func (v *adfBlockVisitor) VisitMultiBodiedExtension(n *adf.MultiBodiedExtension) ast.Node {
	return v.blockFallback(n)
}

// VisitExtensionFrame implements adf.Visitor.
func (v *adfBlockVisitor) VisitExtensionFrame(n *adf.ExtensionFrame) ast.Node {
	return v.blockFallback(n)
}

// VisitSyncBlock implements adf.Visitor.
func (v *adfBlockVisitor) VisitSyncBlock(n *adf.SyncBlock) ast.Node { return v.blockFallback(n) }

// VisitBodiedSyncBlock implements adf.Visitor.
func (v *adfBlockVisitor) VisitBodiedSyncBlock(n *adf.BodiedSyncBlock) ast.Node {
	return v.blockFallback(n)
}

// convertAdfCodeBlock converts an ADF codeBlock to an AST code node.
// Code block text nodes may carry marks for syntax highlighting —
// just concatenate raw text and strip trailing newlines.
func convertAdfCodeBlock(node *adf.CodeBlock) *ast.Code {
	var b strings.Builder
	for _, child := range node.Content {
		b.WriteString(adf.NodeText(child))
	}
	value := strings.TrimRight(b.String(), "\n")
	return &ast.Code{Lang: node.Language, Value: value}
}

// convertAdfTaskList converts an ADF taskList to an AST checklist.
// blockTaskItems keep their block children (markdown task items carry
// indented blocks naturally); plain taskItems wrap their inline content
// in a single paragraph as before.
func convertAdfTaskList(node *adf.TaskList, rc renderCtx) *ast.List {
	var items []ast.Node
	for _, itemNode := range node.Content {
		switch item := itemNode.(type) {
		case *adf.TaskItem:
			checked := strings.EqualFold(item.State, "DONE")
			items = append(items, &ast.ListItem{
				Checked:  &checked,
				Children: []ast.Node{&ast.Paragraph{Children: convertAdfInlines(item.Content, rc)}},
			})
		case *adf.BlockTaskItem:
			checked := strings.EqualFold(item.State, "DONE")
			blocks := convertAdfBlocks(item.Content, rc)
			if !blockTaskItemRenderable(blocks) {
				// Without a leading paragraph the "- [ ] text" form has
				// nothing to anchor the checkbox to (a bare marker
				// re-parses as literal text); degrade to the historical
				// paragraph-flattening projection.
				var inlines []ast.Node
				for _, c := range item.Content {
					if p, ok := c.(*adf.Paragraph); ok {
						inlines = append(inlines, convertAdfInlines(p.Content, rc)...)
					}
				}
				blocks = []ast.Node{&ast.Paragraph{Children: inlines}}
			}
			items = append(items, &ast.ListItem{
				Checked:  &checked,
				Children: blocks,
			})
		}
	}
	return &ast.List{Children: items}
}

// blockTaskItemRenderable reports whether decoded block-task-item
// content starts with a non-empty paragraph (see the degrade note in
// convertAdfTaskList).
func blockTaskItemRenderable(blocks []ast.Node) bool {
	if len(blocks) == 0 {
		return false
	}
	p, ok := blocks[0].(*ast.Paragraph)
	return ok && len(p.Children) > 0
}

// convertAdfDecisionList converts an ADF decisionList to a plain AST
// bullet list; convertAdfBlocks emits the ::decisions companion before
// it (the pair is the markdown form of a decision list).
func convertAdfDecisionList(node *adf.DecisionList, rc renderCtx) *ast.List {
	var items []ast.Node
	for _, itemNode := range node.Content {
		item, ok := itemNode.(*adf.DecisionItem)
		if !ok {
			continue
		}
		items = append(items, &ast.ListItem{
			Children: []ast.Node{&ast.Paragraph{Children: convertAdfInlines(item.Content, rc)}},
		})
	}
	return &ast.List{Children: items}
}

// adfListShape carries the shared BulletList/OrderedList fields into
// the list conversion.
type adfListShape struct {
	tight   *bool
	start   int
	ordered bool
}

func convertAdfListItems(shape adfListShape, content []adf.Node, rc renderCtx) *ast.List {
	// Loose/tight from the ADF "tight" attribute (set by astToAdf from
	// Goldmark's IsTight when WithPreserveListTightness is enabled).
	// Jira-sourced ADF renders tight, like remark-stringify with listItem
	// spread always false (see the ordered-list fixtures).
	loose := false
	if shape.tight != nil {
		loose = !*shape.tight
	}

	var items []ast.Node
	for _, itemNode := range content {
		item, ok := itemNode.(*adf.ListItem)
		if !ok {
			continue
		}
		items = append(items, &ast.ListItem{
			Children: convertAdfBlocks(item.Content, rc),
		})
	}

	return &ast.List{
		Ordered: shape.ordered,
		// ADF records no marker style, so the reference rendering repeats
		// the start number. WithIncrementListMarkers asks for the form
		// people write instead (see convert.WithIncrementListMarkers).
		Increment: shape.ordered && rc.incrementLists,
		Start:     shape.start,
		Spread:    loose,
		Children:  items,
	}
}

// adfRowConverter converts an ADF table's rows one at a time, carrying
// the rowspan state between them. Column count and padding are measured
// in VISUAL columns: a colspan-N cell covers N, and rowspans carry
// covered columns into the following rows.
type adfRowConverter struct {
	carryState
	rc       renderCtx
	colCount int
}

func convertAdfTable(node *adf.Table, rc renderCtx) ast.Node {
	rows := adfTableRows(node)
	if len(rows) == 0 {
		return nil
	}
	conv := &adfRowConverter{rc: rc, colCount: visualColCount(rows[0])}

	var mdRows []ast.Node
	if !rowIsHeader(rows[0]) {
		// No header row — prepend an empty header, then all rows as data.
		mdRows = append(mdRows, emptyTableRow(conv.colCount))
	}
	for _, row := range rows {
		mdRows = append(mdRows, conv.convertRow(row))
	}
	return &ast.Table{Children: mdRows, Align: liftTableAlign(node.Align)}
}

// adfTableRows picks the tableRow children out of a table, skipping
// anything else the document may carry there.
func adfTableRows(node *adf.Table) []*adf.TableRow {
	var rows []*adf.TableRow
	for _, child := range node.Content {
		if row, ok := child.(*adf.TableRow); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// rowIsHeader reports whether the row holds any tableHeader cell, the
// ADF shape markdown spells as the header row.
func rowIsHeader(row *adf.TableRow) bool {
	for _, cell := range row.Content {
		if _, ok := cell.(*adf.TableHeader); ok {
			return true
		}
	}
	return false
}

// visualColCount is the row's width in visual columns.
func visualColCount(row *adf.TableRow) int {
	count := 0
	for _, cell := range row.Content {
		cs, _ := cellSpans(cell)
		count += max(cs, 1)
	}
	return count
}

// convertRow converts one ADF row, padding it out to the table's visual
// column count and advancing the rowspan carries.
func (c *adfRowConverter) convertRow(row *adf.TableRow) ast.Node {
	var cells []ast.Node
	var fresh []rowspanCarry
	width := c.carriedWidth()
	for _, cell := range row.Content {
		mdCell, cs, rs := c.convertCell(cell)
		if rs > 1 {
			fresh = append(fresh, rowspanCarry{rowsLeft: rs - 1, width: cs})
		}
		width += cs
		cells = append(cells, mdCell)
	}
	for width < c.colCount {
		cells = append(cells, &ast.TableCell{})
		width++
	}
	c.advance(fresh)
	return &ast.TableRow{Children: cells}
}

// convertCell flattens one cell's block content to inlines and lifts its
// spans onto the markdown cell, returning the visual column span and the
// row span alongside it.
func (c *adfRowConverter) convertCell(cell adf.Node) (mdCell *ast.TableCell, colSpan, rowSpan int) {
	// Table cells contain block nodes; flatten to inlines.
	mdCell = &ast.TableCell{Children: cellInlines(adf.NodeContent(cell), c.rc)}
	csRaw, rs := cellSpans(cell)
	cs := max(csRaw, 1)
	if cs > 1 {
		mdCell.ColSpan = cs
	}
	if rs > 1 {
		mdCell.RowSpan = rs
	}
	return mdCell, cs, rs
}

// ---------------------------------------------------------------------------
// Cell projection: a cell's ADF blocks → the inlines one table row holds
// ---------------------------------------------------------------------------

// cellInlines converts a table cell's ADF content and projects the result
// onto the inline nodes a GFM table row can carry.
//
// The cell's children are full ADF blocks, so they run through the ordinary
// block conversion first — the only path that knows every kind, including
// media, cards, extensions and the caller's registered decode hooks — and
// the projection then flattens what came back.
//
// Converting the cell's GRANDCHILDREN as inlines instead, which is what
// this did before, reached exactly one level down. A paragraph's children
// are inlines, so a paragraph cell survived; a blockquote's, a list's, a
// panel's, a nested table's children are blocks, and the inline visitor's
// hook-or-drop fallback deleted every one of them. A cell holding anything
// but a paragraph or a heading therefore arrived empty — measured on the
// wire-ADF path, a blockquote cell, a bullet-list cell, a panel cell, a
// task-list cell, a nested-table cell and a media cell all rendered as
// "|   |". That is silent data loss, so the projection recovers the
// content and gives up only the block markers no row can spell.
func cellInlines(nodes []adf.Node, rc renderCtx) []ast.Node {
	p := &cellProjector{}
	p.children(convertAdfBlocks(nodes, rc))
	return p.out
}

// cellProjector accumulates a cell's inline projection together with the
// one space that stands in for a block boundary the row cannot spell.
type cellProjector struct {
	out []ast.Node
	sep bool
}

// children projects a run of siblings, marking a boundary wherever a block
// meets a neighbor. Without it the blocks' texts run together: a cell
// holding the paragraphs "one" and "two" flattened to "onetwo", and a
// two-item list to "ab". The reference joins sibling blocks with a blank
// line, which is an end of line the row cannot hold, and the end of line a
// cell cannot hold folds to a space (the same answer the reference's own
// serializer gives a break inside a cell).
func (p *cellProjector) children(kids []ast.Node) {
	for i, kid := range kids {
		if i > 0 && (cellBlockNode(kid) || cellBlockNode(kids[i-1])) {
			p.boundary()
		}
		p.push(kid)
	}
}

// boundary records that a block boundary was crossed. The space it stands
// for is written only once more content follows, so a block that projects
// to nothing — a rule, an empty paragraph — leaves no space behind, and a
// cell never picks up a leading or trailing one.
func (p *cellProjector) boundary() {
	if len(p.out) > 0 {
		p.sep = true
	}
}

// emit appends one projected inline, flushing a pending boundary first.
func (p *cellProjector) emit(node ast.Node) {
	if p.sep {
		p.sep = false
		p.writeBoundarySpace()
	}
	p.out = append(p.out, node)
}

// writeBoundarySpace writes the boundary space, folding it into the
// preceding text node when there is one so the cell keeps one run of text
// rather than gaining a bare space node between every pair of blocks.
func (p *cellProjector) writeBoundarySpace() {
	if last, ok := p.out[len(p.out)-1].(*ast.Text); ok {
		if !strings.HasSuffix(last.Value, " ") {
			last.Value += " "
		}
		return
	}
	p.out = append(p.out, &ast.Text{Value: " "})
}

// push projects one converted node into the cell.
//
// Every block flattens to its content: a block owns whole lines, and a row
// has one, so its markers cannot be written there. The four cases ahead of
// that rule are the blocks whose content is NOT in their children — a Value
// field, or nothing at all — plus the one inline that spells an end of
// line.
func (p *cellProjector) push(node ast.Node) {
	switch n := node.(type) {
	case *ast.Code:
		// A fence needs its own lines, so the block form cannot ride in a
		// row; an inline code span is the nearest form that can, and it
		// keeps both the text and the code-ness. The language is lost with
		// the fence. A code block is normally several lines, so the fold is
		// the common path here rather than the exception.
		p.emitValue(n.Value)
	case *ast.Frontmatter:
		// No ADF kind decodes to frontmatter today, but a decode hook may
		// return one and its text must not vanish; like a code block it is
		// multi-line by nature.
		p.emitValue(n.Value)
	case *ast.HTML:
		// Raw HTML stays raw — the renderer writes the value verbatim — so
		// it has to arrive already fit for a row: line ends folded, and its
		// own pipes escaped, which the inline writer does not do for a
		// value it does not interpret.
		if n.Value != "" {
			p.emit(&ast.HTML{Value: cellSafeRawHTML(n.Value)})
		}
	case *ast.ThematicBreak:
		// The one block with nothing to lose. Written out it would be
		// literal "***" text in the row, which carries no information.
	case *dialect.Colwidths:
		// The companion convertAdfBlocks emits ahead of a table: its label
		// is the machine-readable colwidth list of the table that FOLLOWS,
		// not prose. A nested table dissolves into this cell's own text, so
		// the widths describe nothing any more, and flattening the label
		// like any other would write "79,320" into the middle of the row.
		// It is the only dialect kind whose label is a payload rather than
		// content (the others hold an alt text, a URL, a query, a title).
	case *ast.Break:
		// A hard break inside a cell renders as a backslash and a NEWLINE,
		// which ends the row and cuts the table in half. It is a boundary
		// like any other, so it folds to the same space — which is what the
		// reference serializer writes for a break inside a cell.
		p.boundary()
	default:
		if form, ok := cellAttrOnlyLeaf(node); ok {
			p.emit(form)
			return
		}
		if cellBlockNode(node) {
			p.children(ast.Children(node))
			return
		}
		p.emit(node)
	}
}

// emitValue projects a block that carries its text in a Value field onto
// the inline code span that fits a row, dropping it when it is empty.
func (p *cellProjector) emitValue(value string) {
	if value == "" {
		return
	}
	p.emit(&ast.InlineCode{Value: foldCellLines(value)})
}

// cellAttrOnlyLeaf answers the one case where flattening a foreign node to
// its content deletes the node outright: a registered kind that spells the
// LEAF directive form (::name[label]{attrs}) and whose whole payload sits
// in the attributes, because it has no content to flatten to.
//
// Two separate mechanisms both end in that deletion, and this catches both.
// A kind that records block spacing is flattened here to children it does
// not have. A kind that does not is handed to the renderer instead, which
// writes NOTHING for it: extension.RenderContext.WriteLeafDirective is a
// no-op in inline position, so the leaf form has no spelling there at all.
// Measured 2026-09-05 on the wire-ADF path, a table cell holding an ADF
// extension, a syncBlock or a media node arrived as "|   |" — the table
// renders, one cell is merely blank, and nothing reports the loss.
//
// The repair is the inline directive form of the same name, which a row
// CAN spell and which carries the attributes the flatten dropped. For the
// two kinds the dialect gives a text form (media, extension) it reads back
// as the typed inline node; for the rest it reads back as a generic text
// directive, which is still the payload in the document rather than gone.
//
// The rule is deliberately confined to a leaf that projects to nothing, so
// no cell that renders content today changes: a leaf whose label survives
// (a link card's URL, a decision's text) keeps flattening to that label.
// The empty-attrs guard excludes the companion markers, whose payload is
// the SIBLING they annotate rather than themselves — ::decisions is
// emitted ahead of the plain list it marks, exactly like ::colwidths above
// it, and writing ":decisions" into the row would be noise, not recovery.
func cellAttrOnlyLeaf(node ast.Node) (ast.Node, bool) {
	ext, ok := node.(extension.Node)
	if !ok {
		return nil, false
	}
	var form cellLeafForm
	ext.RenderMarkdown(&form)
	if !form.written || len(form.attrs) == 0 {
		return nil, false
	}
	probe := &cellProjector{}
	probe.children(ast.Children(node))
	if len(probe.out) > 0 {
		return nil, false
	}
	return &ast.TextDirective{Name: form.name, Attrs: form.attrs}, true
}

// cellLeafForm is the extension.RenderContext that asks a node which
// directive form it spells without rendering anything: the projection
// needs the name and the attributes, which no other part of the extension
// contract exposes. Only the leaf form is recorded — a container form has
// block children to flatten, and a text form already rides in a row.
type cellLeafForm struct {
	attrs   map[string]string
	name    string
	written bool
}

// WriteLeafDirective implements extension.RenderContext.
func (f *cellLeafForm) WriteLeafDirective(name string, attrs map[string]string, _ []ast.Node) {
	f.name, f.attrs, f.written = name, attrs, true
}

// WriteContainerDirective implements extension.RenderContext.
func (*cellLeafForm) WriteContainerDirective(string, map[string]string, []ast.Node) {}

// WriteTextDirective implements extension.RenderContext.
func (*cellLeafForm) WriteTextDirective(string, map[string]string, []ast.Node) {}

// cellBlockNode reports whether a node owns whole lines in markdown. That
// is the projection's one classification: it decides both that the node's
// own markers cannot be written in a row (so only its content comes along)
// and that the gap to its sibling is a boundary rather than the ordinary
// adjacency of two inlines inside one paragraph.
//
// ast.Spaced answers it for every kind but four, core and foreign alike:
// the interface is the AST's own record of a block's blank-line structure,
// so a kind embedding ast.BlockSpacing has declared itself a block, and an
// extension kind joins the classification by the same embed rather than by
// being listed here. That matters because the renderer writes NOTHING for a
// foreign block in inline position — both extension.RenderContext block
// primitives are no-ops there — so an unflattened one is a silent deletion,
// measured on a decisionList (its ::decisions companion), a blockCard, an
// embedCard, an ADF extension and a mediaGroup.
//
// The four exceptions:
//   - Root, ListItem, TableRow and TableCell record no spacing (they are
//     never a block's sibling) but are containers all the same.
//   - HTML embeds the spacing yet is the one kind that is block OR inline:
//     a parse splits inline HTML into tag-shaped nodes inside a paragraph's
//     text run, and treating those as blocks would put a space in the
//     middle of a tag pair. push writes its value out either way.
func cellBlockNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.HTML:
		return false
	case *ast.Root, *ast.ListItem, *ast.TableRow, *ast.TableCell:
		return true
	}
	_, ok := node.(ast.Spaced)
	return ok
}

// foldCellLines folds every end of line in a recovered value to one space,
// so the value fits the single line a table row is. A CRLF is one end of
// line and folds to one space.
//
// This is the half of the one-line rule no kind can opt out of: a raw
// newline inside a cell ends the row wherever it came from, and the four
// output lines that follow are a worse loss than the empty cell this
// projection was written to repair.
func foldCellLines(s string) string {
	if !strings.ContainsAny(s, "\n\r") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			b.WriteByte(' ')
		case '\n':
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// cellSafeRawHTML makes a raw HTML value writable on one table row: every
// end of line becomes a space, and a bare pipe is escaped.
//
// The pipe half is raw HTML's alone. Every other inline the projection
// emits has its pipes escaped for it by the cell's inline writer; raw HTML
// is written verbatim, so an unescaped pipe in it splits the row into an
// extra column. A pipe already carrying an odd run of backslashes is
// escaped and stays as it is.
func cellSafeRawHTML(s string) string {
	s = foldCellLines(s)
	if !strings.Contains(s, "|") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	backslashes := 0
	for i := range len(s) {
		switch c := s[i]; c {
		case '|':
			if backslashes%2 == 0 {
				b.WriteByte('\\')
			}
			b.WriteByte(c)
			backslashes = 0
		case '\\':
			backslashes++
			b.WriteByte(c)
		default:
			backslashes = 0
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Inline conversion: ADF inline nodes → AST inline nodes
// ---------------------------------------------------------------------------

// flatInline is an intermediate inline item before mark-nesting is applied.
// ADF stores marks as flat arrays on text nodes; AST nests strong/emphasis/
// delete wrappers around children instead. The *Mark fields keep the
// original mark values for the DecodeTextMark hook dispatch (nil marks
// are synthesized from the extracted value at dispatch time).
type flatInline struct {
	directive ast.Node
	text      string
	href      string
	// linkTitle is the link mark's title attribute, the advisory text a
	// markdown link spells after its destination.
	linkTitle     string
	textColor     string
	bgColor       string
	subsup        string
	annotations   []*adf.Annotation
	textColorMark adf.Mark
	bgColorMark   adf.Mark
	subsupMark    adf.Mark
	underlineMark adf.Mark
	// otherMarks are marks the core does not consume itself (unknown or
	// unprojected kinds), offered to DecodeTextMark hooks in ADF
	// mark-array order.
	otherMarks   []adf.Mark
	isLink       bool
	isCode       bool
	isInlineCard bool
	isBreak      bool
	underline    bool
	// breakLead/breakTrail record that the folded text had a line break at
	// its very start or end (see collapseInlineNewlines): between two nodes
	// the space that break folded into is content, at the edge of a block
	// it is only the producer's wrap.
	breakLead  bool
	breakTrail bool
	strong     bool
	em         bool
	strike     bool
}

func convertAdfInlines(nodes []adf.Node, rc renderCtx) []ast.Node {
	items := collectInlines(nodes, rc)
	trimBreakEdges(items)
	ops := flatInlineSpanOps(rc)
	// lax=true: ADF's marks may already be lossy (an editor can drop a
	// mark around a code span), so the forward inference also recovers an
	// unmarked run right after the code span — see inferAfterCode.
	inferAcrossCode(items, ops, true)
	return groupSpans(items, ops, openMarks{})
}

// flatInlineSpanOps adapts flatInline to the shared flat→nested spanning
// algorithm (spanning.go); the leaf constructor closes over the render
// context the mark hooks need.
func flatInlineSpanOps(rc renderCtx) spanOps[flatInline] {
	return spanOps[flatInline]{
		strong:  func(i *flatInline) bool { return i.strong },
		em:      func(i *flatInline) bool { return i.em },
		strike:  func(i *flatInline) bool { return i.strike },
		isCode:  func(i *flatInline) bool { return i.isCode },
		isBreak: func(i *flatInline) bool { return i.isBreak },
		text:    func(i *flatInline) string { return i.text },
		set: func(i *flatInline, mark spanMark) {
			switch mark {
			case spanStrong:
				i.strong = true
			case spanEm:
				i.em = true
			default: // spanStrike
				i.strike = true
			}
		},
		leaf: func(i *flatInline) ast.Node { return inlineLeafNode(*i, rc) },
	}
}

// trimBreakEdges drops the space a folded line break left where nothing renders
// it: at the very start or end of a block's inline run, and against a hardBreak
// on either side.
//
// The hardBreak case is how Confluence stores one — the break node, then a text
// node holding the newline that followed the <br/> in storage format. Folding
// that newline to a space put it at the head of the next line, which markdown
// can only write as &#x20;, so every second and later line of the paragraph came
// back carrying a character the document never had.
//
// Whitespace beside an explicit break is the producer's wrap either way: the
// break already ended the line, and a renderer that honors it shows nothing for
// the space. A fold between two words is a different thing and stays a space.
func trimBreakEdges(items []flatInline) {
	unrendered := func(i int) bool { return i < 0 || i >= len(items) || items[i].isBreak }
	for i := range items {
		item := &items[i]
		if item.breakLead && unrendered(i-1) {
			item.text = strings.TrimPrefix(item.text, " ")
		}
		if item.breakTrail && unrendered(i+1) {
			item.text = strings.TrimSuffix(item.text, " ")
		}
	}
}

func collectInlines(nodes []adf.Node, rc renderCtx) []flatInline {
	v := &adfInlineVisitor{rc: rc}
	var items []flatInline
	for _, node := range nodes {
		items = append(items, adf.Visit(node, v)...)
	}
	return items
}

// adfInlineVisitor converts one ADF node in inline position to its flat
// inline items (nil drops it) — the inline-position counterpart of
// adfBlockVisitor, with the same compile-time exhaustiveness.
type adfInlineVisitor struct {
	rc renderCtx
}

// VisitText implements adf.Visitor.
func (v *adfInlineVisitor) VisitText(n *adf.Text) []flatInline {
	if v.rc.diagnostics != nil && adf.HasMark(n.Marks, "fontSize") {
		// fontSize is retired: the mark decodes to bare text (see
		// convertTextInline / coreConsumedMark). Report the drop.
		v.rc.diagnostics(Diagnostic{Code: CodeFontSizeDropped, Message: fontSizeDroppedMessage})
	}
	item := convertTextInline(n)
	if item.isLink && v.rc.linkResolver.Decode != nil {
		if resolved, ok := v.rc.linkResolver.Decode(item.href); ok {
			item.href = resolved
		}
	}
	return []flatInline{item}
}

// VisitHardBreak implements adf.Visitor.
func (*adfInlineVisitor) VisitHardBreak(*adf.HardBreak) []flatInline {
	return []flatInline{{isBreak: true}}
}

// VisitEmoji implements adf.Visitor. An emoji WITH rendered text keeps
// projecting as that text (deliberately lossy across markdown
// persistence — parity with the measured behavior); without text the
// shortname table restores the unicode rendering, and custom/site
// emojis fall back to the :emoji directive so they survive markdown.
func (*adfInlineVisitor) VisitEmoji(n *adf.Emoji) []flatInline {
	if n.Text != nil {
		return []flatInline{{text: *n.Text}}
	}
	if n.ShortName == "" {
		return nil
	}
	if unicode, ok := dialect.EmojiUnicode(n.ShortName); ok {
		return []flatInline{{text: unicode}}
	}
	attrs := map[string]string{"shortName": n.ShortName}
	if n.ID != "" {
		attrs["id"] = n.ID
	}
	return []flatInline{{directive: &dialect.Emoji{Attrs: attrs}}}
}

// VisitInlineCard implements adf.Visitor.
func (v *adfInlineVisitor) VisitInlineCard(n *adf.InlineCard) []flatInline {
	return convertInlineCard(n, v.rc)
}

// decodeInline dispatches to the registered inline decode hooks.
// Extension kinds (mention, status, mediaInline, …) decode via the
// registered hooks; the produced nodes ride through mark grouping as
// opaque directive items.
func (v *adfInlineVisitor) decodeInline(node adf.Node) ([]flatInline, bool) {
	decoded, ok := decodeInlineHook(node, v.rc)
	if !ok {
		return nil, false
	}
	items := make([]flatInline, 0, len(decoded))
	for _, n := range decoded {
		items = append(items, flatInline{directive: n})
	}
	return items, true
}

// inlineFallback handles every kind without a dedicated inline
// conversion: a registered hook may decode it; anything else drops.
func (v *adfInlineVisitor) inlineFallback(node adf.Node) []flatInline {
	items, _ := v.decodeInline(node)
	return items
}

// VisitRaw implements adf.Visitor: a decode hook may still claim the
// unknown kind; otherwise the raw-node diagnostic fires and the node
// drops.
func (v *adfInlineVisitor) VisitRaw(n *adf.RawNode) []flatInline {
	if items, ok := v.decodeInline(n); ok {
		return items
	}
	v.rc.reportRawNode(n)
	return nil
}

// The remaining kinds have no dedicated inline conversion and degrade
// through the hook-or-drop fallback.

// VisitParagraph implements adf.Visitor.
func (v *adfInlineVisitor) VisitParagraph(n *adf.Paragraph) []flatInline { return v.inlineFallback(n) }

// VisitHeading implements adf.Visitor.
func (v *adfInlineVisitor) VisitHeading(n *adf.Heading) []flatInline { return v.inlineFallback(n) }

// VisitBlockquote implements adf.Visitor.
func (v *adfInlineVisitor) VisitBlockquote(n *adf.Blockquote) []flatInline {
	return v.inlineFallback(n)
}

// VisitRule implements adf.Visitor.
func (v *adfInlineVisitor) VisitRule(n *adf.Rule) []flatInline { return v.inlineFallback(n) }

// VisitCodeBlock implements adf.Visitor.
func (v *adfInlineVisitor) VisitCodeBlock(n *adf.CodeBlock) []flatInline { return v.inlineFallback(n) }

// VisitBulletList implements adf.Visitor.
func (v *adfInlineVisitor) VisitBulletList(n *adf.BulletList) []flatInline {
	return v.inlineFallback(n)
}

// VisitOrderedList implements adf.Visitor.
func (v *adfInlineVisitor) VisitOrderedList(n *adf.OrderedList) []flatInline {
	return v.inlineFallback(n)
}

// VisitListItem implements adf.Visitor.
func (v *adfInlineVisitor) VisitListItem(n *adf.ListItem) []flatInline { return v.inlineFallback(n) }

// VisitTaskList implements adf.Visitor.
func (v *adfInlineVisitor) VisitTaskList(n *adf.TaskList) []flatInline { return v.inlineFallback(n) }

// VisitTaskItem implements adf.Visitor.
func (v *adfInlineVisitor) VisitTaskItem(n *adf.TaskItem) []flatInline { return v.inlineFallback(n) }

// VisitDecisionList implements adf.Visitor.
func (v *adfInlineVisitor) VisitDecisionList(n *adf.DecisionList) []flatInline {
	return v.inlineFallback(n)
}

// VisitDecisionItem implements adf.Visitor.
func (v *adfInlineVisitor) VisitDecisionItem(n *adf.DecisionItem) []flatInline {
	return v.inlineFallback(n)
}

// VisitTable implements adf.Visitor.
func (v *adfInlineVisitor) VisitTable(n *adf.Table) []flatInline { return v.inlineFallback(n) }

// VisitTableRow implements adf.Visitor.
func (v *adfInlineVisitor) VisitTableRow(n *adf.TableRow) []flatInline { return v.inlineFallback(n) }

// VisitTableCell implements adf.Visitor.
func (v *adfInlineVisitor) VisitTableCell(n *adf.TableCell) []flatInline { return v.inlineFallback(n) }

// VisitTableHeader implements adf.Visitor.
func (v *adfInlineVisitor) VisitTableHeader(n *adf.TableHeader) []flatInline {
	return v.inlineFallback(n)
}

// VisitPanel implements adf.Visitor.
func (v *adfInlineVisitor) VisitPanel(n *adf.Panel) []flatInline { return v.inlineFallback(n) }

// VisitExpand implements adf.Visitor.
func (v *adfInlineVisitor) VisitExpand(n *adf.Expand) []flatInline { return v.inlineFallback(n) }

// VisitNestedExpand implements adf.Visitor.
func (v *adfInlineVisitor) VisitNestedExpand(n *adf.NestedExpand) []flatInline {
	return v.inlineFallback(n)
}

// VisitMediaSingle implements adf.Visitor.
func (v *adfInlineVisitor) VisitMediaSingle(n *adf.MediaSingle) []flatInline {
	return v.inlineFallback(n)
}

// VisitMediaGroup implements adf.Visitor.
func (v *adfInlineVisitor) VisitMediaGroup(n *adf.MediaGroup) []flatInline {
	return v.inlineFallback(n)
}

// VisitMedia implements adf.Visitor.
func (v *adfInlineVisitor) VisitMedia(n *adf.Media) []flatInline { return v.inlineFallback(n) }

// VisitBlockCard implements adf.Visitor.
func (v *adfInlineVisitor) VisitBlockCard(n *adf.BlockCard) []flatInline { return v.inlineFallback(n) }

// VisitEmbedCard implements adf.Visitor.
func (v *adfInlineVisitor) VisitEmbedCard(n *adf.EmbedCard) []flatInline { return v.inlineFallback(n) }

// VisitMention implements adf.Visitor.
func (v *adfInlineVisitor) VisitMention(n *adf.Mention) []flatInline { return v.inlineFallback(n) }

// VisitStatus implements adf.Visitor.
func (v *adfInlineVisitor) VisitStatus(n *adf.Status) []flatInline { return v.inlineFallback(n) }

// VisitMediaInline implements adf.Visitor. A card the host product owns reads
// back as the link it stands for; an attachment the asset store has the file
// for reads back as the inline image it came from; every other one stays a
// :media directive.
//
// The chip that addresses NOTHING — a mediaInline with neither id nor
// collection, which the product does emit — is the one whose directive
// had nothing left to spell, so it rendered as the bare name ":media"
// and the next parse read that back as the word rather than the node.
// unbareSourcelessChip gives it the default type to spell instead; see
// there for why the same rule serves this leg and the format leg.
func (v *adfInlineVisitor) VisitMediaInline(n *adf.MediaInline) []flatInline {
	if link, ok := v.rc.fileCardLink(n); ok {
		return []flatInline{{text: link.Label, href: link.Href, isLink: true}}
	}
	if img := v.rc.mediaInlineAsImage(n); img != nil {
		return []flatInline{{directive: img}}
	}
	items := v.inlineFallback(n)
	for i := range items {
		unbareSourcelessChip(items[i].directive)
	}
	return items
}

// VisitColwidthsHint implements adf.Visitor.
func (v *adfInlineVisitor) VisitColwidthsHint(n *adf.ColwidthsHint) []flatInline {
	return v.inlineFallback(n)
}

// VisitDate implements adf.Visitor.
func (v *adfInlineVisitor) VisitDate(n *adf.Date) []flatInline { return v.inlineFallback(n) }

// VisitPlaceholder implements adf.Visitor.
func (v *adfInlineVisitor) VisitPlaceholder(n *adf.Placeholder) []flatInline {
	return v.inlineFallback(n)
}

// VisitCaption implements adf.Visitor.
func (v *adfInlineVisitor) VisitCaption(n *adf.Caption) []flatInline { return v.inlineFallback(n) }

// VisitBlockTaskItem implements adf.Visitor.
func (v *adfInlineVisitor) VisitBlockTaskItem(n *adf.BlockTaskItem) []flatInline {
	return v.inlineFallback(n)
}

// VisitLayoutSection implements adf.Visitor.
func (v *adfInlineVisitor) VisitLayoutSection(n *adf.LayoutSection) []flatInline {
	return v.inlineFallback(n)
}

// VisitLayoutColumn implements adf.Visitor.
func (v *adfInlineVisitor) VisitLayoutColumn(n *adf.LayoutColumn) []flatInline {
	return v.inlineFallback(n)
}

// VisitExtensionNode implements adf.Visitor.
func (v *adfInlineVisitor) VisitExtensionNode(n *adf.Extension) []flatInline {
	return v.inlineFallback(n)
}

// VisitInlineExtension implements adf.Visitor.
func (v *adfInlineVisitor) VisitInlineExtension(n *adf.InlineExtension) []flatInline {
	return v.inlineFallback(n)
}

// VisitBodiedExtension implements adf.Visitor.
func (v *adfInlineVisitor) VisitBodiedExtension(n *adf.BodiedExtension) []flatInline {
	return v.inlineFallback(n)
}

// VisitMultiBodiedExtension implements adf.Visitor.
func (v *adfInlineVisitor) VisitMultiBodiedExtension(n *adf.MultiBodiedExtension) []flatInline {
	return v.inlineFallback(n)
}

// VisitExtensionFrame implements adf.Visitor.
func (v *adfInlineVisitor) VisitExtensionFrame(n *adf.ExtensionFrame) []flatInline {
	return v.inlineFallback(n)
}

// VisitSyncBlock implements adf.Visitor.
func (v *adfInlineVisitor) VisitSyncBlock(n *adf.SyncBlock) []flatInline {
	return v.inlineFallback(n)
}

// VisitBodiedSyncBlock implements adf.Visitor.
func (v *adfInlineVisitor) VisitBodiedSyncBlock(n *adf.BodiedSyncBlock) []flatInline {
	return v.inlineFallback(n)
}

// convertTextInline converts an ADF text node with its flat mark array to a
// flatInline item.
func convertTextInline(node *adf.Text) flatInline {
	marks := node.Marks
	text, breakLead, breakTrail := collapseInlineNewlines(node.Text)
	item := flatInline{text: text, breakLead: breakLead, breakTrail: breakTrail}
	if adf.HasMark(marks, "code") {
		// Code mark is exclusive in ADF — strong/em/strike are stripped.
		item.isCode = true
	} else {
		item.strong = adf.HasMark(marks, "strong")
		item.em = adf.HasMark(marks, "em")
		item.strike = adf.HasMark(marks, "strike")
	}
	if linkMark, ok := adf.FindMark[*adf.Link](marks); ok && linkMark.Href != nil {
		item.isLink = true
		item.href = *linkMark.Href
		if linkMark.Title != nil {
			item.linkTitle = *linkMark.Title
		}
	}
	if m, ok := adf.FindMark[*adf.TextColor](marks); ok {
		item.textColor = m.Color
		item.textColorMark = m
	}
	if m, ok := adf.FindMark[*adf.BackgroundColor](marks); ok {
		item.bgColor = m.Color
		item.bgColorMark = m
	}
	item.underline = adf.HasMark(marks, "underline")
	if m, ok := adf.FindMark[*adf.Underline](marks); ok {
		item.underlineMark = m
	}
	if m, ok := adf.FindMark[*adf.SubSup](marks); ok {
		if m.Type == "sup" {
			item.subsup = "sup"
		} else {
			item.subsup = "sub"
		}
		item.subsupMark = m
	}
	// Only the first (outermost) annotation projects: directive labels
	// cannot nest brackets, so overlapping inline-comment anchors on one
	// text node degrade to the outermost across markdown persistence.
	if a, ok := adf.FindMark[*adf.Annotation](marks); ok && a.ID != "" {
		item.annotations = []*adf.Annotation{a}
	}
	// Marks outside the projected vocabulary (unknown kinds, RawMark) are
	// offered to the DecodeTextMark hooks in mark-array order.
	for _, m := range marks {
		if !coreConsumedMark(m.Kind()) {
			item.otherMarks = append(item.otherMarks, m)
		}
	}
	return item
}

// collapseInlineNewlines folds the line breaks a producer left inside a text
// node into single spaces, the way CommonMark folds a soft break, and reports
// whether a break sat at either edge of the node.
//
// ADF spells a line break as a hardBreak node; a newline inside a text node is
// whitespace, and that is how the products render it. The markdown side
// already agrees — FromMarkdown turns a soft break into a space and never
// produces a text node containing one — so keeping the byte here would make
// the same paragraph render differently depending on which side it came from,
// and the wrapper would then break lines at the producer's old width instead
// of ours.
//
// Only inline text passes through: codeBlock content is read straight off the
// node (see convertAdfCodeBlock) and keeps its newlines.
func collapseInlineNewlines(s string) (text string, lead, trail bool) {
	if !strings.ContainsAny(s, "\r\n") {
		return s, false, false
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' && s[i] != '\r' {
			out = append(out, s[i])
			continue
		}
		// Swallow the whole break — CRLF, blank lines, and the indentation
		// on either side of it, all of which CommonMark drops too.
		for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t') {
			out = out[:len(out)-1]
		}
		lead = lead || len(out) == 0
		for i+1 < len(s) && isInlineBreakSpace(s[i+1]) {
			i++
		}
		trail = i+1 >= len(s)
		out = append(out, ' ')
	}
	return string(out), lead, trail
}

// isInlineBreakSpace reports whether the byte is part of the whitespace run a
// soft break may span.
func isInlineBreakSpace(c byte) bool {
	return c == '\n' || c == '\r' || c == ' ' || c == '\t'
}

// coreConsumedMark reports whether the mark kind is consumed by the core
// text projection above (and therefore never offered to the hooks as an
// "other" mark). fontSize is core-consumed too: it is retired and
// dropped to bare text (see VisitText), never handed to a decode hook.
func coreConsumedMark(kind string) bool {
	switch kind {
	case "strong", "em", "strike", "code", "link",
		"textColor", "backgroundColor", "underline", "subsup",
		"fontSize", "annotation":
		return true
	}
	return false
}

// convertInlineCard converts an ADF inlineCard node to a smart-link item
// (the short key when the configured SmartLinks resolver knows the URL).
func convertInlineCard(node *adf.InlineCard, rc renderCtx) []flatInline {
	if node.URL != nil {
		url := *node.URL
		linkText := smartLinkLabel(rc, url)
		return []flatInline{{
			text:         linkText,
			isLink:       true,
			href:         url,
			isInlineCard: true,
		}}
	}
	return nil
}

// inlineLeafNode converts a single flat inline item to its AST leaf node
// (strong/em/delete wrappers are handled by groupSpans). ADF marks
// without native markdown syntax wrap the leaf in the typed mark kinds
// via the DecodeTextMark hook dispatch, in fixed canonical nesting order
// (outside → inside): :annotation (in ADF mark order), :color, :bg, :u,
// :sub/:sup. (A legacy fontSize mark is retired — dropped to bare text in
// VisitText — so it never wraps here.) The core owns WHICH marks project
// and this order; the registered hooks (the dialect set by default) own the node
// construction — ADF stores these as text marks, not nodes the other
// decode hooks could own (see the dialect package). Marks outside that
// vocabulary wrap outermost in mark-array order (first innermost) when
// a hook claims them, and drop otherwise.
func inlineLeafNode(item flatInline, rc renderCtx) ast.Node {
	if item.isBreak {
		return &ast.Break{}
	}
	if item.directive != nil {
		return item.directive
	}

	var node ast.Node
	if item.isCode {
		node = &ast.InlineCode{Value: item.text}
	} else {
		node = &ast.Text{Value: item.text}
	}
	if item.isLink {
		node = &ast.Link{
			URL:        item.href,
			Title:      item.linkTitle,
			InlineCard: item.isInlineCard,
			Children:   []ast.Node{node},
		}
	}
	if item.subsup != "" {
		node = applyTextMarkHook(markOr(item.subsupMark, func() adf.Mark { return &adf.SubSup{Type: item.subsup} }), node, rc)
	}
	if item.underline {
		node = applyTextMarkHook(markOr(item.underlineMark, func() adf.Mark { return &adf.Underline{} }), node, rc)
	}
	if item.bgColor != "" {
		node = applyTextMarkHook(markOr(item.bgColorMark, func() adf.Mark { return &adf.BackgroundColor{Color: item.bgColor} }), node, rc)
	}
	if item.textColor != "" {
		node = applyTextMarkHook(markOr(item.textColorMark, func() adf.Mark { return &adf.TextColor{Color: item.textColor} }), node, rc)
	}
	// Annotations wrap outermost (in ADF mark order, first mark
	// outermost) so the comment anchor encloses the styled text.
	for _, a := range slices.Backward(item.annotations) {
		node = applyTextMarkHook(a, node, rc)
	}
	// Unconsumed marks wrap outside the canonical stack, first mark
	// innermost (the block-mark canon); unclaimed ones drop.
	for _, m := range item.otherMarks {
		node = applyTextMarkHook(m, node, rc)
	}
	return node
}

// markOr returns the captured original mark, or synthesizes one from the
// extracted value for items built without mark capture.
func markOr(m adf.Mark, synth func() adf.Mark) adf.Mark {
	if m != nil {
		return m
	}
	return synth()
}
