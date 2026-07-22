package convert

import (
	"slices"
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/extension"
)

// ptr returns a pointer to v (presence-sensitive ADF attributes are
// pointer fields).
func ptr[T any](v T) *T { return &v }

// ToADF converts an AST root node to an ADF document. This is the
// AST→AST half of FromMarkdown, mirroring remark's mdast-to-ADF shape:
// the AST nests strong/emphasis/delete wrappers, while ADF stores flat
// mark arrays on text nodes, so inline conversion flattens the tree with
// an inherited mark context.
//
// WithPreserveListTightness stores the source list tightness on ADF list
// nodes (as the synthetic Tight field) so that a later FromADF can
// reproduce tight lists without blank lines.
func ToADF(root ast.Node, opts ...Option) adf.Doc {
	cfg := config{}
	for _, o := range opts {
		o(&cfg)
	}
	c := astConverter{
		preserveTight:    cfg.preserveListTightness,
		smartLinks:       cfg.smartLinks,
		resolveImageDims: cfg.resolveImageDims,
		resolveAssetID:   cfg.resolveAssetID,
		diagnostics:      cfg.diagnostics,
		codeLanguages:    cfg.codeLanguages,
		unsupportedKind:  cfg.unsupportedProduct,
		unsupportedKinds: cfg.unsupportedKinds,
	}
	content := c.convertBlocks(ast.Children(root))
	if len(content) == 0 {
		content = []adf.Node{&adf.Paragraph{Content: []adf.Node{}}}
	}
	doc := adf.Doc{Type: "doc", Version: 1, Content: content}
	c.checkUnsupportedKinds(doc)
	return doc
}

type astConverter struct {
	smartLinks       SmartLinks
	resolveImageDims ImageDimsResolver
	resolveAssetID   AssetIDResolver
	diagnostics      func(Diagnostic)
	codeLanguages    map[string]bool
	unsupportedKinds map[string]bool
	unsupportedKind  string
	preserveTight    bool
}

// checkUnsupportedKinds walks the produced document over both nodes and
// marks and emits one unsupported-in-product diagnostic per DISTINCT
// kind present in the configured set (see WithUnsupportedKinds). It is
// diagnostic-only: the document is not mutated. Without a set (or sink)
// it does nothing.
func (c *astConverter) checkUnsupportedKinds(doc adf.Doc) {
	if c.diagnostics == nil || len(c.unsupportedKinds) == 0 {
		return
	}
	seen := make(map[string]bool)
	report := func(kind string) {
		if !c.unsupportedKinds[kind] || seen[kind] {
			return
		}
		seen[kind] = true
		c.diagnostics(Diagnostic{
			Code:    CodeUnsupportedInProduct,
			Message: kind + " is not available in " + c.unsupportedKind,
		})
	}
	for _, top := range doc.Content {
		for n := range adf.Walk(top) {
			report(n.Kind())
			for _, m := range adf.NodeMarks(n) {
				report(m.Kind())
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Mark context: inherited marks accumulated during inline traversal
// ---------------------------------------------------------------------------

type markCtx struct {
	link        string
	textColor   string
	bgColor     string
	subsup      string // "sub" | "sup" | ""
	fontSize    string
	annotations []extension.Annotation // outermost first (ADF mark order)
	hasLink     bool                   // set even when link is "" ([x]() keeps its empty-href mark, like remark)
	strong      bool
	em          bool
	strike      bool
	underline   bool
}

func (*astConverter) buildMarks(ctx markCtx) []adf.Mark {
	var marks []adf.Mark
	if ctx.strong {
		marks = append(marks, &adf.Strong{})
	}
	if ctx.em {
		marks = append(marks, &adf.Em{})
	}
	if ctx.strike {
		marks = append(marks, &adf.Strike{})
	}
	if ctx.textColor != "" {
		marks = append(marks, &adf.TextColor{Color: ctx.textColor})
	}
	if ctx.bgColor != "" {
		marks = append(marks, &adf.BackgroundColor{Color: ctx.bgColor})
	}
	if ctx.underline {
		marks = append(marks, &adf.Underline{})
	}
	if ctx.subsup != "" {
		marks = append(marks, &adf.SubSup{Type: ctx.subsup})
	}
	if ctx.fontSize != "" {
		marks = append(marks, &adf.FontSize{Size: ctx.fontSize})
	}
	for _, a := range ctx.annotations {
		marks = append(marks, &adf.Annotation{ID: a.ID, AnnotationType: a.AnnotationType})
	}
	if ctx.hasLink {
		marks = append(marks, &adf.Link{Href: ptr(ctx.link)})
	}
	if len(marks) == 0 {
		return nil
	}
	return marks
}

// ---------------------------------------------------------------------------
// Block conversion: AST block nodes → ADF block nodes
// ---------------------------------------------------------------------------

func (c *astConverter) convertBlocks(nodes []ast.Node) []adf.Node {
	v := &astBlockVisitor{c: c}
	var out []adf.Node
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		// ::decisions marks the FOLLOWING plain bullet list as an ADF
		// decisionList. Like ::colwidths, the application is cross-sibling
		// and therefore structural — it stays here rather than inside the
		// Decisions kind; unlike colwidths it consumes the list BEFORE
		// encoding (decisionItems carry inline content, not the encoded
		// listItem blocks). A directive without a following plain bullet
		// list drops with a diagnostic.
		if _, isDecisions := node.(*dialect.Decisions); isDecisions {
			if list := decisionTargetList(nodes, i); list != nil {
				out = c.appendBlock(out, c.convertDecisionItems(listItemsOf(list)))
				i++ // list consumed
				continue
			}
			if c.diagnostics != nil {
				c.diagnostics(Diagnostic{
					Code:    CodeDecisionsOrphan,
					Message: "::decisions directive with no bullet list on the following line was dropped",
				})
			}
			continue
		}
		// Most kinds convert to one node, extension kinds to any number;
		// the style-preserving gap attribute lands on the first encoded
		// node.
		encoded := ast.Visit(node, v)
		for _, n := range encoded {
			out = c.appendBlock(out, n)
		}
	}
	return c.applyColwidths(out)
}

// decisionTargetList returns the list a ::decisions directive at nodes[i]
// marks: the immediately following sibling when it is a plain (unordered,
// non-task) bullet list; nil otherwise. Shared by the ADF encode
// (convertBlocks) and the Normalize pass (encodeBlocks) — the ::decisions
// marking is a cross-sibling pattern with no dedicated pivot-AST node, so
// both directions match it the same way.
func decisionTargetList(nodes []ast.Node, i int) *ast.List {
	if i+1 >= len(nodes) {
		return nil
	}
	list, ok := nodes[i+1].(*ast.List)
	if !ok || list.Ordered {
		return nil
	}
	for j := range list.Children {
		if item, isItem := list.Children[j].(*ast.ListItem); isItem && item.Checked != nil {
			return nil
		}
	}
	return list
}

// listItemsOf collects a list's ListItem children.
func listItemsOf(list *ast.List) []*ast.ListItem {
	items := make([]*ast.ListItem, 0, len(list.Children))
	for i := range list.Children {
		if item, ok := list.Children[i].(*ast.ListItem); ok {
			items = append(items, item)
		}
	}
	return items
}

// appendBlock appends a converted block node, reassembling adjacent
// ::media{group} items into one mediaGroup (only while neither side
// carries block marks — mark-wrapped groups stay separate).
func (*astConverter) appendBlock(out []adf.Node, n adf.Node) []adf.Node {
	// Adjacent ::media{group} items reassemble one mediaGroup.
	if group, ok := n.(*adf.MediaGroup); ok && len(out) > 0 && len(group.Marks) == 0 {
		if prev, ok := out[len(out)-1].(*adf.MediaGroup); ok && len(prev.Marks) == 0 {
			prev.Content = append(prev.Content, group.Content...)
			return out
		}
	}
	return append(out, n)
}

// resolveColwidthTargets is the shared cross-sibling ::colwidths matcher
// used by both the ADF encode (applyColwidths) and the Normalize pass
// (resolveColwidths): a colwidths directive canonicalizes onto the
// immediately following table sibling, and an orphan (no widths, or no
// table after it) drops with a diagnostic. Only the item type and the
// per-side payload work differ, so those come in as callbacks:
// directiveWidths reports a colwidths item's widths (ok=false for a
// non-directive item that passes through), isTarget recognizes the table
// sibling, and attach performs the side's payload (returning the item to
// keep). The pivot AST has no dedicated colwidths-on-table node, so this
// pattern stays structural in both directions.
func resolveColwidthTargets[T any](
	items []T,
	directiveWidths func(T) ([]float64, bool),
	isTarget func(T) bool,
	attach func(target T, widths []float64) T,
	orphan func(),
) []T {
	out := make([]T, 0, len(items))
	for i := 0; i < len(items); i++ {
		widths, isDir := directiveWidths(items[i])
		if !isDir {
			out = append(out, items[i])
			continue
		}
		if len(widths) > 0 && i+1 < len(items) && isTarget(items[i+1]) {
			out = append(out, attach(items[i+1], widths))
			i++ // target consumed
			continue
		}
		orphan()
	}
	return out
}

// applyColwidths attaches a ::colwidths placeholder (see
// adf.ColwidthsHint and dialect.ColwidthsPlaceholder) to the table that
// follows it: each cell gets the widths of the visual columns it covers
// (a colspan-N cell gets N entries; Jira repeats the widths on all
// rows). Placeholders without a following table drop.
func (c *astConverter) applyColwidths(nodes []adf.Node) []adf.Node {
	return resolveColwidthTargets(nodes,
		func(n adf.Node) ([]float64, bool) {
			if hint, ok := n.(*adf.ColwidthsHint); ok {
				return hint.Widths, true
			}
			return nil, false
		},
		func(n adf.Node) bool { _, ok := n.(*adf.Table); return ok },
		func(table adf.Node, widths []float64) adf.Node {
			if t, ok := table.(*adf.Table); ok {
				applyTableColwidths(t, widths)
			}
			return table
		},
		func() {
			if c.diagnostics != nil {
				c.diagnostics(Diagnostic{
					Code:    CodeColwidthsOrphan,
					Message: "::colwidths directive with no table on the following line was dropped",
				})
			}
		},
	)
}

// applyTableColwidths assigns per-cell colwidth arrays, accounting for
// merged cells: colspans consume several widths, rowspans cover columns in
// the following rows (shifting those rows' cells to later visual columns).
func applyTableColwidths(table *adf.Table, widths []float64) {
	covered := map[int]map[int]bool{}
	rowIdx := 0
	for _, rowNode := range table.Content {
		row, ok := rowNode.(*adf.TableRow)
		if !ok {
			continue
		}
		col := 0
		for _, cell := range row.Content {
			for covered[rowIdx][col] {
				col++
			}
			csRaw, rsRaw := cellSpans(cell)
			cs := max(csRaw, 1)
			rs := max(rsRaw, 1)
			if col < len(widths) {
				setCellColwidth(cell, slices.Clone(widths[col:min(col+cs, len(widths))]))
			}
			for r := 1; r < rs; r++ {
				if covered[rowIdx+r] == nil {
					covered[rowIdx+r] = map[int]bool{}
				}
				for c := col; c < col+cs; c++ {
					covered[rowIdx+r][c] = true
				}
			}
			col += cs
		}
		rowIdx++
	}
}

// setCellColwidth stores a colwidth array on a table cell.
func setCellColwidth(cell adf.Node, widths []float64) {
	switch c := cell.(type) {
	case *adf.TableCell:
		c.Colwidth = widths
	case *adf.TableHeader:
		c.Colwidth = widths
	case *adf.RawNode:
		vals := make([]any, len(widths))
		for i, w := range widths {
			vals[i] = w
		}
		if c.Attrs == nil {
			c.Attrs = map[string]any{}
		}
		c.Attrs["colwidth"] = vals
	}
}

// astBlockVisitor converts one AST node in block position to its ADF
// block node(s) — one node for the core kinds (nil drops it), any number
// for extension kinds. Implementing ast.Visitor[[]adf.Node] keeps the
// conversion exhaustive: a new AST kind fails compilation here until it
// gets an explicit conversion or the fallback.
type astBlockVisitor struct {
	c *astConverter
}

// singleBlock wraps one converted block node for the visitor result
// (nil stays nil so dropped blocks append nothing).
func singleBlock(n adf.Node) []adf.Node {
	if n == nil {
		return nil
	}
	return []adf.Node{n}
}

// VisitParagraph implements ast.Visitor.
func (v *astBlockVisitor) VisitParagraph(n *ast.Paragraph) []adf.Node {
	return singleBlock(v.c.convertParagraph(n))
}

// VisitHeading implements ast.Visitor.
func (v *astBlockVisitor) VisitHeading(n *ast.Heading) []adf.Node {
	level := min(max(n.Depth, 1), 6)
	return singleBlock(&adf.Heading{
		Level:   level,
		Content: v.c.convertInlines(n.Children),
	})
}

// VisitThematicBreak implements ast.Visitor.
func (*astBlockVisitor) VisitThematicBreak(*ast.ThematicBreak) []adf.Node {
	return singleBlock(&adf.Rule{})
}

// VisitBlockquote implements ast.Visitor.
func (v *astBlockVisitor) VisitBlockquote(n *ast.Blockquote) []adf.Node {
	return singleBlock(&adf.Blockquote{Content: v.c.convertBlocks(n.Children)})
}

// VisitCode implements ast.Visitor. With WithCodeLanguages configured,
// a language tag outside the set reports a diagnostic (the language
// still encodes verbatim).
func (v *astBlockVisitor) VisitCode(n *ast.Code) []adf.Node {
	v.c.checkCodeLanguage(n.Lang)
	var content []adf.Node
	if n.Value != "" {
		content = []adf.Node{&adf.Text{Text: n.Value}}
	}
	return singleBlock(&adf.CodeBlock{Language: n.Lang, Content: content})
}

// checkCodeLanguage emits the unsupported-code-language diagnostic for a
// non-empty language tag outside the configured WithCodeLanguages set
// (case-insensitive). Without a configured set (or sink) it does nothing.
func (c *astConverter) checkCodeLanguage(lang string) {
	if lang == "" || c.codeLanguages == nil || c.diagnostics == nil {
		return
	}
	if c.codeLanguages[strings.ToLower(lang)] {
		return
	}
	c.diagnostics(Diagnostic{
		Code:    CodeUnsupportedCodeLanguage,
		Message: "code block language " + strconv.Quote(lang) + " is not in the configured supported set",
	})
}

// VisitList implements ast.Visitor.
func (v *astBlockVisitor) VisitList(n *ast.List) []adf.Node {
	return singleBlock(v.c.convertList(n))
}

// VisitContainerDirective implements ast.Visitor.
func (v *astBlockVisitor) VisitContainerDirective(n *ast.ContainerDirective) []adf.Node {
	return singleBlock(v.c.convertContainerDirective(n))
}

// VisitLeafDirective implements ast.Visitor.
func (*astBlockVisitor) VisitLeafDirective(*ast.LeafDirective) []adf.Node {
	// Generic (unknown) leaf directives have no ADF mapping and are
	// dropped, matching the remark reference pipeline; the known
	// dialect leaves are typed extension nodes and never reach here.
	return nil
}

// VisitTable implements ast.Visitor.
func (v *astBlockVisitor) VisitTable(n *ast.Table) []adf.Node {
	return singleBlock(v.c.convertTable(n))
}

// VisitHTML implements ast.Visitor.
func (*astBlockVisitor) VisitHTML(*ast.HTML) []adf.Node {
	// Block-level HTML is dropped (mirrors the remark reference pipeline
	// — including the legacy "<!-- media omitted -->" placeholder).
	return nil
}

// VisitFrontmatter implements ast.Visitor.
func (*astBlockVisitor) VisitFrontmatter(*ast.Frontmatter) []adf.Node {
	// Frontmatter never reaches conversion (parseDocument splits it off);
	// a stray node in the tree has no ADF form and drops.
	return nil
}

// VisitExtension implements ast.Visitor: the extension kinds (dialect +
// custom) encode themselves; other unknown kinds degrade through the
// fallback.
func (v *astBlockVisitor) VisitExtension(n ast.Node) []adf.Node {
	if ext, ok := n.(extension.Node); ok {
		// Extension kinds encode themselves.
		return ext.EncodeADF(&blockEncodeContext{c: v.c})
	}
	return v.blockFallback(n)
}

// blockFallback handles the kinds without a dedicated block conversion.
func (v *astBlockVisitor) blockFallback(node ast.Node) []adf.Node {
	// Unknown block — try to recurse into children
	if kids := ast.Children(node); len(kids) > 0 {
		content := v.c.convertBlocks(kids)
		if len(content) > 0 {
			return content[:1]
		}
	}
	return nil
}

// The remaining kinds have no dedicated block conversion: structural
// children (list items, table rows/cells) and inline kinds in block
// position degrade through the recurse-into-children fallback.

// VisitRoot implements ast.Visitor.
func (v *astBlockVisitor) VisitRoot(n *ast.Root) []adf.Node { return v.blockFallback(n) }

// VisitListItem implements ast.Visitor.
func (v *astBlockVisitor) VisitListItem(n *ast.ListItem) []adf.Node { return v.blockFallback(n) }

// VisitTableRow implements ast.Visitor.
func (v *astBlockVisitor) VisitTableRow(n *ast.TableRow) []adf.Node { return v.blockFallback(n) }

// VisitTableCell implements ast.Visitor.
func (v *astBlockVisitor) VisitTableCell(n *ast.TableCell) []adf.Node { return v.blockFallback(n) }

// VisitText implements ast.Visitor.
func (v *astBlockVisitor) VisitText(n *ast.Text) []adf.Node { return v.blockFallback(n) }

// VisitEmphasis implements ast.Visitor.
func (v *astBlockVisitor) VisitEmphasis(n *ast.Emphasis) []adf.Node { return v.blockFallback(n) }

// VisitStrong implements ast.Visitor.
func (v *astBlockVisitor) VisitStrong(n *ast.Strong) []adf.Node { return v.blockFallback(n) }

// VisitDelete implements ast.Visitor.
func (v *astBlockVisitor) VisitDelete(n *ast.Delete) []adf.Node { return v.blockFallback(n) }

// VisitInlineCode implements ast.Visitor.
func (v *astBlockVisitor) VisitInlineCode(n *ast.InlineCode) []adf.Node { return v.blockFallback(n) }

// VisitBreak implements ast.Visitor.
func (v *astBlockVisitor) VisitBreak(n *ast.Break) []adf.Node { return v.blockFallback(n) }

// VisitLink implements ast.Visitor.
func (v *astBlockVisitor) VisitLink(n *ast.Link) []adf.Node { return v.blockFallback(n) }

// VisitImage implements ast.Visitor.
func (v *astBlockVisitor) VisitImage(n *ast.Image) []adf.Node { return v.blockFallback(n) }

// VisitTextDirective implements ast.Visitor.
func (v *astBlockVisitor) VisitTextDirective(n *ast.TextDirective) []adf.Node {
	return v.blockFallback(n)
}

// convertParagraph converts an AST paragraph. A paragraph holding exactly
// one absolute image becomes external media (the inverse of mediaAsImage);
// ADF has no inline image. The style-preserving formatter keeps images as
// extension nodes.
func (c *astConverter) convertParagraph(node *ast.Paragraph) adf.Node {
	if img, id, ok := c.singleAttachmentImage(node); ok {
		return withImageCaption(c.attachmentImageToMedia(img, id), img.Title)
	}
	if url, alt, ok := singleImageChild(node); ok {
		media := &adf.Media{Type: "external", URL: url, Alt: alt}
		title := ""
		if img, isImg := node.Children[0].(*ast.Image); isImg {
			title = img.Title
		}
		return withImageCaption(&adf.MediaSingle{
			Layout:  ptr("center"),
			Content: []adf.Node{media},
		}, title)
	}
	return &adf.Paragraph{Content: c.convertInlines(node.Children)}
}

// withImageCaption attaches an image title as the mediaSingle caption
// child (![alt](path "caption") ⇄ mediaSingle + caption).
func withImageCaption(n adf.Node, title string) adf.Node {
	single, ok := n.(*adf.MediaSingle)
	if !ok || title == "" {
		return n
	}
	single.Content = append(single.Content, &adf.Caption{
		Content: []adf.Node{&adf.Text{Text: title}},
	})
	return single
}

// convertContainerDirective converts a generic (unknown) container
// directive: its single converted child replaces it; anything else is
// dropped (observed remark behavior). The known dialect containers
// (panels, :::expand) are typed extension nodes and never reach here;
// for them the directive label, when present, is simply the first child
// paragraph (remark's representation). Directive attributes have no ADF
// equivalent.
func (c *astConverter) convertContainerDirective(node *ast.ContainerDirective) adf.Node {
	content := c.convertBlocks(node.Children)
	if len(content) == 1 {
		return content[0]
	}
	return nil
}

func (c *astConverter) convertList(node *ast.List) adf.Node {
	items := make([]*ast.ListItem, 0, len(node.Children))
	for i := range node.Children {
		if item, ok := node.Children[i].(*ast.ListItem); ok {
			items = append(items, item)
		}
	}

	// A list is a task list when any item carries a checkbox state.
	isTask := false
	for i := range items {
		if items[i].Checked != nil {
			isTask = true
			break
		}
	}

	if isTask {
		return c.convertTaskItems(items)
	}

	listItems := make([]adf.Node, 0, len(items))
	for i := range items {
		item := items[i]
		listItems = append(listItems, &adf.ListItem{Content: c.convertBlocks(item.Children)})
	}

	if node.Ordered {
		// Start 0 is a genuine "0)" list (remark keeps order 0); every
		// producer of ordered AST lists sets Start explicitly.
		l := &adf.OrderedList{Order: ptr(node.Start), Content: listItems}
		c.applyListTightness(&l.Tight, node)
		return l
	}
	l := &adf.BulletList{Content: listItems}
	c.applyListTightness(&l.Tight, node)
	return l
}

// applyListTightness stores the synthetic tightness attribute on an ADF
// list node (see WithPreserveListTightness).
func (c *astConverter) applyListTightness(tight **bool, node *ast.List) {
	if c.preserveTight {
		*tight = ptr(!node.Spread)
	}
}

// convertTaskItems converts task-list items to an ADF taskList: a
// single-paragraph item stays the historical inline taskItem; an item
// with additional (or non-paragraph) blocks becomes a blockTaskItem so
// the block content survives.
func (c *astConverter) convertTaskItems(items []*ast.ListItem) adf.Node {
	taskItems := make([]adf.Node, 0, len(items))
	for i := range items {
		item := items[i]
		state := "TODO"
		if item.Checked != nil && *item.Checked {
			state = "DONE"
		}
		if p, single := singleParagraphItem(item); single {
			taskItems = append(taskItems, &adf.TaskItem{
				LocalID: ptr(""),
				State:   state,
				Content: c.convertInlines(p.Children),
			})
			continue
		}
		if blockTaskItemLead(item.Children) {
			taskItems = append(taskItems, &adf.BlockTaskItem{
				LocalID: ptr(""),
				State:   state,
				Content: c.convertBlocks(item.Children),
			})
			continue
		}
		// Items the "- [ ] text" + blocks form cannot carry (no leading
		// paragraph text to anchor the checkbox) keep the historical
		// paragraph-flattening taskItem.
		var inlines []adf.Node
		for j := range item.Children {
			if p, ok := item.Children[j].(*ast.Paragraph); ok {
				inlines = append(inlines, c.convertInlines(p.Children)...)
			}
		}
		taskItems = append(taskItems, &adf.TaskItem{
			LocalID: ptr(""),
			State:   state,
			Content: inlines,
		})
	}
	return &adf.TaskList{LocalID: ptr(""), Content: taskItems}
}

// singleParagraphItem reports the item's sole paragraph child (the
// shape the inline taskItem models); empty items count as single so
// they keep encoding as empty taskItems.
func singleParagraphItem(item *ast.ListItem) (*ast.Paragraph, bool) {
	if len(item.Children) == 0 {
		return &ast.Paragraph{}, true
	}
	if len(item.Children) != 1 {
		return nil, false
	}
	p, ok := item.Children[0].(*ast.Paragraph)
	return p, ok
}

// blockTaskItemLead reports whether the item's first block is a
// non-empty paragraph — the only shape whose checkbox survives the
// markdown round trip (a bare "- [ ]" marker re-parses as literal
// text), and the schema's blockTaskItem lead anyway.
func blockTaskItemLead(children []ast.Node) bool {
	if len(children) == 0 {
		return false
	}
	p, ok := children[0].(*ast.Paragraph)
	return ok && len(p.Children) > 0
}

// convertDecisionItems converts the items of a ::decisions-marked bullet
// list to an ADF decisionList: each item's paragraph inlines flatten into
// one decisionItem (decisionItems carry inline content only; other
// blocks drop).
func (c *astConverter) convertDecisionItems(items []*ast.ListItem) adf.Node {
	decisionItems := make([]adf.Node, 0, len(items))
	for i := range items {
		var inlines []adf.Node
		for j := range items[i].Children {
			if p, ok := items[i].Children[j].(*ast.Paragraph); ok {
				inlines = append(inlines, c.convertInlines(p.Children)...)
			}
		}
		decisionItems = append(decisionItems, &adf.DecisionItem{
			LocalID: ptr(""),
			State:   "DECIDED",
			Content: inlines,
		})
	}
	return &adf.DecisionList{LocalID: ptr(""), Content: decisionItems}
}

// singleAttachmentImage reports the paragraph's sole child when it is an
// image whose path the asset store maps back to a media id (a downloaded
// attachment).
func (c *astConverter) singleAttachmentImage(node *ast.Paragraph) (*ast.Image, string, bool) {
	if c.resolveAssetID == nil || len(node.Children) != 1 {
		return nil, "", false
	}
	img, ok := node.Children[0].(*ast.Image)
	if !ok {
		return nil, "", false
	}
	if strings.HasPrefix(img.URL, "http://") || strings.HasPrefix(img.URL, "https://") {
		return nil, "", false
	}
	id, ok := c.resolveAssetID(img.URL)
	if !ok || id == "" {
		return nil, "", false
	}
	return img, strings.ToLower(id), true
}

// attachmentImageToMedia converts a downloaded-attachment image back to its
// ADF media form: the id comes from the filename, the intrinsic dimensions
// from the local file via the injected resolver.
func (c *astConverter) attachmentImageToMedia(img *ast.Image, id string) adf.Node {
	media := &adf.Media{Type: "file", ID: id}
	if alt := ast.PlainText(img.Children); alt != "" {
		media.Alt = alt
	}
	media.Collection = ptr("")
	if c.resolveImageDims != nil {
		if fw, fh, ok := c.resolveImageDims(img.URL); ok {
			media.Width = ptr(float64(fw))
			media.Height = ptr(float64(fh))
		}
	}
	return &adf.MediaSingle{
		Layout:  ptr("align-start"),
		Content: []adf.Node{media},
	}
}

// singleImageChild reports the paragraph's sole child when it is an image
// with an absolute http(s) URL (relative paths cannot be represented in
// Jira until asset upload exists).
func singleImageChild(node *ast.Paragraph) (url, alt string, ok bool) {
	if len(node.Children) != 1 {
		return "", "", false
	}
	img, ok := node.Children[0].(*ast.Image)
	if !ok {
		return "", "", false
	}
	if !strings.HasPrefix(img.URL, "http://") && !strings.HasPrefix(img.URL, "https://") {
		return "", "", false
	}
	return img.URL, ast.PlainText(img.Children), true
}

func (c *astConverter) convertTable(node *ast.Table) adf.Node {
	var rows []adf.Node
	rowIndex := 0
	for i := range node.Children {
		row, ok := node.Children[i].(*ast.TableRow)
		if !ok {
			continue
		}
		var cells []adf.Node
		for j := range row.Children {
			mdCell, ok := row.Children[j].(*ast.TableCell)
			if !ok {
				continue
			}
			content := []adf.Node{&adf.Paragraph{Content: c.convertInlines(mdCell.Children)}}
			var cell adf.Node
			if rowIndex == 0 {
				cell = &adf.TableHeader{Colspan: spanAttr(mdCell.ColSpan), Rowspan: spanAttr(mdCell.RowSpan), Content: content}
			} else {
				cell = &adf.TableCell{Colspan: spanAttr(mdCell.ColSpan), Rowspan: spanAttr(mdCell.RowSpan), Content: content}
			}
			cells = append(cells, cell)
		}
		rows = append(rows, &adf.TableRow{Content: cells})
		rowIndex++
	}

	if len(rows) == 0 {
		return nil
	}
	return &adf.Table{Content: rows}
}

// spanAttr keeps a colspan/rowspan only when it spans (>1), matching the
// historical attribute emission.
func spanAttr(v int) int {
	if v > 1 {
		return v
	}
	return 0
}

// ---------------------------------------------------------------------------
// Inline conversion: AST inline nodes → flat ADF inline nodes
// ---------------------------------------------------------------------------

func (c *astConverter) convertInlines(nodes []ast.Node) []adf.Node {
	return c.flattenChildren(nodes, markCtx{})
}

// inlineFlattener flattens one AST inline node to flat ADF inline nodes
// under the inherited mark context carried on the receiver (each nesting
// level flattens its children through a fresh flattener with the layered
// context, via flattenChildren). Implementing ast.Visitor[[]adf.Node]
// keeps the flattening exhaustive over the kind set.
type inlineFlattener struct {
	c   *astConverter
	ctx markCtx
}

// VisitText implements ast.Visitor.
func (v *inlineFlattener) VisitText(n *ast.Text) []adf.Node {
	if n.Value == "" {
		return nil
	}
	return []adf.Node{&adf.Text{Text: n.Value, Marks: v.c.buildMarks(v.ctx)}}
}

// VisitInlineCode implements ast.Visitor.
func (v *inlineFlattener) VisitInlineCode(n *ast.InlineCode) []adf.Node {
	// Code mark is exclusive in ADF — drop strong/em/strike
	marks := []adf.Mark{&adf.Code{}}
	if v.ctx.hasLink {
		marks = append(marks, &adf.Link{Href: ptr(v.ctx.link)})
	}
	return []adf.Node{&adf.Text{Text: n.Value, Marks: marks}}
}

// VisitStrong implements ast.Visitor.
func (v *inlineFlattener) VisitStrong(n *ast.Strong) []adf.Node {
	next := v.ctx
	next.strong = true
	return v.c.flattenChildren(n.Children, next)
}

// VisitEmphasis implements ast.Visitor.
func (v *inlineFlattener) VisitEmphasis(n *ast.Emphasis) []adf.Node {
	next := v.ctx
	next.em = true
	return v.c.flattenChildren(n.Children, next)
}

// VisitDelete implements ast.Visitor.
func (v *inlineFlattener) VisitDelete(n *ast.Delete) []adf.Node {
	next := v.ctx
	next.strike = true
	return v.c.flattenChildren(n.Children, next)
}

// VisitLink implements ast.Visitor.
func (v *inlineFlattener) VisitLink(n *ast.Link) []adf.Node {
	return v.c.flattenLink(n, v.ctx)
}

// VisitImage implements ast.Visitor.
func (v *inlineFlattener) VisitImage(n *ast.Image) []adf.Node {
	// An inline image that cannot become media is dropped
	// (mirrors remark, where mdast images carry alt as a string, not
	// children). A local-path image landing here usually means an asset
	// added to the markdown that is not in the store yet (no media id
	// before upload) — report it so an upload flow can pick it up.
	if v.c.diagnostics != nil && n.URL != "" &&
		!strings.HasPrefix(n.URL, "http://") && !strings.HasPrefix(n.URL, "https://") {
		v.c.diagnostics(Diagnostic{
			Code:    CodeUnresolvedAsset,
			Message: "image " + n.URL + " has no media id (not in the asset store); dropped from the ADF payload",
		})
	}
	return nil
}

// VisitBreak implements ast.Visitor.
func (*inlineFlattener) VisitBreak(*ast.Break) []adf.Node {
	return []adf.Node{&adf.HardBreak{}}
}

// VisitTextDirective implements ast.Visitor.
func (v *inlineFlattener) VisitTextDirective(n *ast.TextDirective) []adf.Node {
	return v.c.flattenTextDirective(n, v.ctx)
}

// VisitExtension implements ast.Visitor.
func (v *inlineFlattener) VisitExtension(n ast.Node) []adf.Node {
	if _, isFontSize := n.(*dialect.FontSize); isFontSize && v.c.diagnostics != nil {
		// fontSize is retired: EncodeADF unwraps it to plain text (no mark
		// produced). Report the drop here — EncodeADF has no sink.
		v.c.diagnostics(Diagnostic{Code: CodeFontSizeDropped, Message: fontSizeDroppedMessage})
	}
	if ext, ok := n.(extension.Node); ok {
		// Extension kinds encode themselves; the context carries the
		// inherited marks for styled children (see EncodeInlinesStyled).
		return ext.EncodeADF(&inlineEncodeContext{c: v.c, ctx: v.ctx})
	}
	return v.inlineFallback(n)
}

// VisitHTML implements ast.Visitor.
func (v *inlineFlattener) VisitHTML(n *ast.HTML) []adf.Node {
	// Inline HTML projects as plain text.
	if n.Value == "" {
		return nil
	}
	return []adf.Node{&adf.Text{Text: n.Value, Marks: v.c.buildMarks(v.ctx)}}
}

// inlineFallback handles the kinds without a dedicated inline
// conversion.
func (v *inlineFlattener) inlineFallback(node ast.Node) []adf.Node {
	// Unknown inline — try to recurse into children
	if kids := ast.Children(node); len(kids) > 0 {
		return v.c.flattenChildren(kids, v.ctx)
	}
	if val := nodeValue(node); val != "" {
		return []adf.Node{&adf.Text{Text: val, Marks: v.c.buildMarks(v.ctx)}}
	}
	return nil
}

// The remaining kinds have no dedicated inline conversion: block kinds
// in inline position degrade through the recurse-into-children (or raw
// text value) fallback.

// VisitRoot implements ast.Visitor.
func (v *inlineFlattener) VisitRoot(n *ast.Root) []adf.Node { return v.inlineFallback(n) }

// VisitParagraph implements ast.Visitor.
func (v *inlineFlattener) VisitParagraph(n *ast.Paragraph) []adf.Node { return v.inlineFallback(n) }

// VisitHeading implements ast.Visitor.
func (v *inlineFlattener) VisitHeading(n *ast.Heading) []adf.Node { return v.inlineFallback(n) }

// VisitThematicBreak implements ast.Visitor.
func (v *inlineFlattener) VisitThematicBreak(n *ast.ThematicBreak) []adf.Node {
	return v.inlineFallback(n)
}

// VisitBlockquote implements ast.Visitor.
func (v *inlineFlattener) VisitBlockquote(n *ast.Blockquote) []adf.Node { return v.inlineFallback(n) }

// VisitList implements ast.Visitor.
func (v *inlineFlattener) VisitList(n *ast.List) []adf.Node { return v.inlineFallback(n) }

// VisitListItem implements ast.Visitor.
func (v *inlineFlattener) VisitListItem(n *ast.ListItem) []adf.Node { return v.inlineFallback(n) }

// VisitCode implements ast.Visitor.
func (v *inlineFlattener) VisitCode(n *ast.Code) []adf.Node { return v.inlineFallback(n) }

// VisitFrontmatter implements ast.Visitor.
func (v *inlineFlattener) VisitFrontmatter(n *ast.Frontmatter) []adf.Node {
	return v.inlineFallback(n)
}

// VisitTable implements ast.Visitor.
func (v *inlineFlattener) VisitTable(n *ast.Table) []adf.Node { return v.inlineFallback(n) }

// VisitTableRow implements ast.Visitor.
func (v *inlineFlattener) VisitTableRow(n *ast.TableRow) []adf.Node { return v.inlineFallback(n) }

// VisitTableCell implements ast.Visitor.
func (v *inlineFlattener) VisitTableCell(n *ast.TableCell) []adf.Node { return v.inlineFallback(n) }

// VisitContainerDirective implements ast.Visitor.
func (v *inlineFlattener) VisitContainerDirective(n *ast.ContainerDirective) []adf.Node {
	return v.inlineFallback(n)
}

// VisitLeafDirective implements ast.Visitor.
func (v *inlineFlattener) VisitLeafDirective(n *ast.LeafDirective) []adf.Node {
	return v.inlineFallback(n)
}

// nodeValue returns the raw text value of the value-carrying kinds
// without a dedicated inline conversion, for the unknown-inline
// fallback.
func nodeValue(n ast.Node) string {
	switch v := n.(type) {
	case *ast.Code:
		return v.Value
	case *ast.Frontmatter:
		return v.Value
	}
	return ""
}

// flattenLink converts an AST link. Restores inline cards: either
// explicitly tagged (from adfToAst) or a Jira browse link whose label is
// exactly the issue key. The style-preserving mode is a formatter, not a
// normalizer, so links stay links there.
func (c *astConverter) flattenLink(node *ast.Link, ctx markCtx) []adf.Node {
	href := node.URL
	if node.InlineCard {
		return []adf.Node{&adf.InlineCard{URL: ptr(href)}}
	}
	// A link whose text equals the resolver-derived key encodes as an
	// inlineCard (smart link).
	if c.smartLinks.KeyFromURL != nil {
		if key, ok := c.smartLinks.KeyFromURL(href); ok && ast.PlainText(node.Children) == key {
			return []adf.Node{&adf.InlineCard{URL: ptr(href)}}
		}
	}
	next := ctx
	next.link = href
	next.hasLink = true
	return c.flattenChildren(node.Children, next)
}

// flattenTextDirective converts a generic (unknown) text directive: it
// emits the colon-prefixed name as plain text followed by the label
// children so the content round-trips without being lost; attributes are
// dropped (mirrors remark). The known dialect text directives (:media,
// :mention, :status, :color, :bg, :u, :sub/:sup) are typed extension
// nodes and never reach here.
func (c *astConverter) flattenTextDirective(node *ast.TextDirective, ctx markCtx) []adf.Node {
	out := []adf.Node{&adf.Text{Text: ":" + node.Name, Marks: c.buildMarks(ctx)}}
	return append(out, c.flattenChildren(node.Children, ctx)...)
}

func (c *astConverter) flattenChildren(children []ast.Node, ctx markCtx) []adf.Node {
	v := &inlineFlattener{c: c, ctx: ctx}
	var out []adf.Node
	for i := range children {
		out = append(out, ast.Visit(children[i], v)...)
	}
	return out
}

// smartLinkURL resolves a ::linkCard/::linkEmbed label to a URL: bare key
// labels expand via the configured SmartLinks resolver when it knows them
// (and stay as-is otherwise, which round-trips stably through
// smartLinkLabel), full URLs pass through.
func (c *astConverter) smartLinkURL(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return ""
	}
	if c.smartLinks.URLForKey != nil {
		if url, ok := c.smartLinks.URLForKey(trimmed); ok {
			return url
		}
	}
	return trimmed
}

// ---------------------------------------------------------------------------
// Extension encode contexts (extension.EncodeContext implementations)
// ---------------------------------------------------------------------------

// blockEncodeContext is the EncodeContext handed to extension nodes in
// block position (no inherited marks).
type blockEncodeContext struct {
	c *astConverter
}

// EncodeBlocks implements extension.EncodeContext.
func (e *blockEncodeContext) EncodeBlocks(children []ast.Node) []adf.Node {
	return e.c.convertBlocks(children)
}

// EncodeInlines implements extension.EncodeContext.
func (e *blockEncodeContext) EncodeInlines(children []ast.Node) []adf.Node {
	return e.c.convertInlines(children)
}

// EncodeInlinesStyled implements extension.EncodeContext.
func (e *blockEncodeContext) EncodeInlinesStyled(style extension.InlineStyle, children []ast.Node) []adf.Node {
	return e.c.flattenStyled(style, children, markCtx{})
}

// SmartLinkURL implements extension.EncodeContext.
func (e *blockEncodeContext) SmartLinkURL(label string) string {
	return e.c.smartLinkURL(label)
}

// inlineEncodeContext is the EncodeContext handed to extension nodes in
// inline position; it carries the marks inherited from enclosing
// constructs.
type inlineEncodeContext struct {
	c   *astConverter
	ctx markCtx
}

// EncodeBlocks implements extension.EncodeContext.
func (e *inlineEncodeContext) EncodeBlocks(children []ast.Node) []adf.Node {
	return e.c.convertBlocks(children)
}

// EncodeInlines implements extension.EncodeContext.
func (e *inlineEncodeContext) EncodeInlines(children []ast.Node) []adf.Node {
	return e.c.flattenChildren(children, e.ctx)
}

// EncodeInlinesStyled implements extension.EncodeContext.
func (e *inlineEncodeContext) EncodeInlinesStyled(style extension.InlineStyle, children []ast.Node) []adf.Node {
	return e.c.flattenStyled(style, children, e.ctx)
}

// SmartLinkURL implements extension.EncodeContext.
func (e *inlineEncodeContext) SmartLinkURL(label string) string {
	return e.c.smartLinkURL(label)
}

// flattenStyled layers an extension.InlineStyle onto the inherited mark
// context and flattens the children under it. Pointer fields overwrite
// the inherited value (even to empty, which clears the mark); the mark
// serialization order stays canonical (see buildMarks).
func (c *astConverter) flattenStyled(style extension.InlineStyle, children []ast.Node, ctx markCtx) []adf.Node {
	next := ctx
	if style.TextColor != nil {
		next.textColor = *style.TextColor
	}
	if style.BackgroundColor != nil {
		next.bgColor = *style.BackgroundColor
	}
	if style.SubSup != nil {
		next.subsup = *style.SubSup
	}
	if style.FontSize != nil {
		next.fontSize = *style.FontSize
	}
	if style.Annotation != nil {
		// Annotations accumulate outermost-first (fresh copy: the outer
		// wrapper's EncodeADF layers its style before nested ones flatten,
		// and siblings must not share the inherited backing array).
		annotations := make([]extension.Annotation, 0, len(ctx.annotations)+1)
		annotations = append(annotations, ctx.annotations...)
		annotations = append(annotations, *style.Annotation)
		next.annotations = annotations
	}
	if style.Underline {
		next.underline = true
	}
	return c.flattenChildren(children, next)
}
