package markdown

import (
	"slices"
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/ast"
)

// List rendering: plain, ordered, and task lists — marker
// alternation, per-item spreading, and item block composition. Split
// from render.go.

func (r *mdRenderer) renderList(b *strings.Builder, node *ast.List, indent, bullet, ordDelim string) {
	loose := node.Spread
	if node.PerItemSpread {
		// Goldmark-sourced lists carry blank-line structure per item.
		loose = false
	}
	bullet = breakSafeBullet(bullet, node)

	// Start 0 is a genuine "0." list (remark renders it); the ADF decoder
	// defaults absent order attributes to 1.
	start := node.Start
	if start < 0 {
		start = 1
	}

	first := true
	itemIdx := 0
	var prevItem *ast.ListItem
	for idx := range node.Children {
		item, ok := node.Children[idx].(*ast.ListItem)
		if !ok {
			continue
		}
		// Per-item looseness like prettier: a blank separates two items when
		// the source had one there (GapAfter) or the previous item contains
		// blank-separated blocks (Spread). List-level loose covers the
		// ADF-side "tight" attribute flow.
		if !first && itemNeedsBlankBefore(loose, prevItem) {
			b.WriteByte('\n')
		}
		first = false
		prevItem = item
		prefix := bullet + " "
		if node.Ordered {
			prefix = orderedItemPrefix(node, start, itemIdx, ordDelim)
		}
		itemIdx++
		// childIndent aligns nested content with the parent item's text column.
		// CommonMark requires indentation ≥ len(indent+prefix) for nested lists.
		childIndent := indent + strings.Repeat(" ", len(prefix))
		depth := len(indent) / 2
		// An empty list item still needs its marker (remark renders a bare
		// "-"/"1."); dropping it entirely would make the list vanish on
		// re-parse, breaking round-trip idempotency. Marker-only break
		// chains are handled at composition (see writeFirstBlockLines).
		if len(item.Children) == 0 {
			b.WriteString(indent)
			b.WriteString(strings.TrimRight(prefix, " "))
			b.WriteString("\n")
			continue
		}
		// Render child blocks. Consecutive nested sibling lists alternate
		// their markers like top-level ones (remark does this at every
		// depth) so "0." + "1)" empty lists don't merge on re-parse.
		var alt nestedListAlternation
		for i := range item.Children {
			child := item.Children[i]
			if i == 0 {
				r.renderItemFirstBlock(b, child, indent, prefix, childIndent, depth)
				if l, isList := child.(*ast.List); isList {
					// Task lists join the chain too (see
					// renderItemFollowBlock); they are never ordered.
					alt.prime(l.Ordered && !isTaskList(l))
				}
				continue
			}
			r.renderItemFollowBlock(b, item, i, childIndent, depth, loose, node.PerItemSpread, &alt)
		}
	}
}

// nestedListAlternation tracks marker alternation for consecutive nested
// sibling lists inside one item (remark alternates at every depth so
// adjacent lists don't merge on re-parse).
type nestedListAlternation struct {
	bullet  bool
	ordered bool
}

// prime records the first nested list's kind.
func (a *nestedListAlternation) prime(ordered bool) {
	a.ordered = ordered
	a.bullet = !ordered
}

// next returns the marker pair for the upcoming nested list and advances
// the alternation.
func (a *nestedListAlternation) next(ordered bool) (bullet, delim string) {
	bullet, delim = "-", "."
	if ordered {
		if a.ordered {
			delim = ")"
		}
		a.ordered = !a.ordered
		a.bullet = false
		return bullet, delim
	}
	if a.bullet {
		bullet = "*"
	}
	a.bullet = !a.bullet
	a.ordered = false
	return bullet, delim
}

// reset clears the alternation (a non-list block breaks the chain).
func (a *nestedListAlternation) reset() {
	a.bullet = false
	a.ordered = false
}

// renderItemFollowBlock renders item.Children[i] (a non-first block of a
// list item). A nested list always attaches with a single newline (prettier
// removes even a source blank line there); other blocks blank-separate in
// loose items, and tight items only separate paragraph pairs
// (mdast-util-to-markdown would otherwise merge them on re-parse — measured
// in the ordered-list fixtures).
func (r *mdRenderer) renderItemFollowBlock(b *strings.Builder, item *ast.ListItem, i int, childIndent string, depth int, loose, perItemSpread bool, alt *nestedListAlternation) {
	child := item.Children[i]
	nested, isNested := nestedPlainList(child)
	if !isNested && followBlockNeedsGap(item, i, loose, perItemSpread) {
		b.WriteByte('\n')
	}
	if isNested {
		bullet, delim := alt.next(nested.Ordered)
		r.renderList(b, nested, childIndent, bullet, delim)
		return
	}
	var inner strings.Builder
	saved := r.prefixWidth
	r.prefixWidth += len(childIndent)
	// A nested TASK list renders through the indented path (its checkboxes
	// are paragraph content, so it carries no indent of its own), but it
	// still shares the bullet-alternation chain with the plain lists around
	// it — same marker at the same column merges into one list on re-parse,
	// and one checkbox anywhere makes the whole list a task list.
	if l, ok := child.(*ast.List); ok && isTaskList(l) {
		bullet, _ := alt.next(false)
		r.renderTaskList(&inner, l, bullet)
	} else {
		alt.reset()
		r.renderBlock(&inner, child, depth)
	}
	r.prefixWidth = saved
	rendered := strings.TrimRight(inner.String(), "\n")
	for line := range strings.SplitSeq(rendered, "\n") {
		if line != "" {
			b.WriteString(childIndent)
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
}

// itemNeedsBlankBefore reports whether a blank line separates this list item
// from the previous one: list-level loose, a source blank after the previous
// item (GapAfter), or blank-separated blocks inside it (Spread).
func itemNeedsBlankBefore(loose bool, prevItem *ast.ListItem) bool {
	return loose || (prevItem != nil && (prevItem.Spread || prevItem.GapAfter))
}

// orderedItemPrefix builds an ordered item's marker prefix. remark-stringify
// with incrementListMarker off (the canonical reference form) repeats the
// list's start number on every item; Increment (goldmark-sourced incrementing
// lists) renumbers sequentially like prettier. Content aligns to the column
// set by the first marker plus its source gap (OrderedGap).
func orderedItemPrefix(node *ast.List, start, itemIdx int, ordDelim string) string {
	num := start
	if node.Increment {
		num = start + itemIdx
	}
	marker := strconv.Itoa(num) + ordDelim
	gap := max(node.OrderedGap, 1)
	contentCol := len(strconv.Itoa(start)) + len(ordDelim) + gap
	return marker + strings.Repeat(" ", max(contentCol-len(marker), 1))
}

// renderItemFirstBlock renders a list item's first block onto the marker
// line: the marker prefix (trimmed for empty content — "- " re-parses as an
// empty item, "-" is stable) followed by the block with continuation lines
// indented to the item's content column, matching prettier.
func (r *mdRenderer) renderItemFirstBlock(b *strings.Builder, child ast.Node, indent, prefix, childIndent string, depth int) {
	var inner strings.Builder
	saved := r.prefixWidth
	r.prefixWidth += len(childIndent)
	r.renderBlock(&inner, child, depth)
	r.prefixWidth = saved
	s := strings.TrimRight(inner.String(), "\n")
	b.WriteString(indent)
	if s == "" {
		b.WriteString(strings.TrimRight(prefix, " "))
	} else {
		b.WriteString(prefix)
	}
	writeFirstBlockLines(b, s, childIndent)
	b.WriteString("\n")
}

// writeFirstBlockLines writes the rendered first block of a list item,
// indenting continuation lines to the item's content column. Marker-only
// chains from nested empty lists ("- " + "- -") would compose a line that
// re-parses as a thematic break; the innermost marker flips to the other
// bullet, like remark ("* + *" renders "- - *").
func writeFirstBlockLines(b *strings.Builder, s, childIndent string) {
	firstLine := true
	for line := range strings.SplitSeq(s, "\n") {
		switch {
		case firstLine:
			b.WriteString(fixMarkerBreakSuffix(currentLine(b), line))
			firstLine = false
		case line == "":
			b.WriteByte('\n')
		default:
			b.WriteByte('\n')
			b.WriteString(childIndent)
			b.WriteString(line)
		}
	}
}

// nestedPlainList returns the item child as a list when it renders as a
// nested plain list (attached without a blank line, single-newline
// separated).
func nestedPlainList(child ast.Node) (*ast.List, bool) {
	l, ok := child.(*ast.List)
	if !ok || isTaskList(l) {
		return nil, false
	}
	return l, true
}

// followBlockNeedsGap reports whether a blank line precedes item.Children[i]
// (a non-first block of a list item): loose or spread items blank-separate
// blocks, tight items only separate paragraph pairs; goldmark-sourced items
// (PerItemSpread) separate blocks exactly where the source had blank lines.
func followBlockNeedsGap(item *ast.ListItem, i int, loose, perItemSpread bool) bool {
	if i == 0 {
		// Degenerate block-task shape: the block follows the marker line
		// directly (see renderTaskItemFollowBlocks).
		return false
	}
	child := item.Children[i]
	_, childIsPara := child.(*ast.Paragraph)
	_, prevIsPara := item.Children[i-1].(*ast.Paragraph)
	paraPair := childIsPara && prevIsPara
	gap := loose || item.Spread || paraPair
	if perItemSpread {
		gap = ast.GapBefore(child)
	}
	if blockRunsToBlankLine(item.Children[i-1]) && blockIsAbsorbable(child) {
		gap = true
	}
	return gap
}

// blockIsAbsorbable reports whether a block's opening line can be swallowed
// by a run-on predecessor. Only a paragraph and a table can: a paragraph
// line is read as another table row or as a lazy continuation, and a GFM
// table cannot interrupt a paragraph, so its header row is absorbed the
// same way. Every other block opens with a marker that breaks the run.
func blockIsAbsorbable(child ast.Node) bool {
	switch child.(type) {
	case *ast.Paragraph, *ast.Table:
		return true
	}
	return false
}

// blockRunsToBlankLine reports whether a list item's block keeps absorbing
// the lines that follow it, so the next block must be blank-separated
// whatever the item's spread says.
//
//   - A paragraph runs to the first blank line too, and a GFM table cannot
//     interrupt it: written adjacently, the paragraph's last line becomes
//     the table's header row and the table's own header becomes a body row
//     ("0\n| -- |\n| -- |\n| 0 |"). A tight item would otherwise attach
//     them, since neither the item's spread nor the paragraph-pair rule
//     covers this pairing.
//   - A GFM table runs to the first blank line: "| 0 |\n| - |\n0"
//     re-parses as a two-row table.
//   - A nested list whose own last block runs on hands the hazard up: the
//     next block lands on the OUTER item's content column, which is a lazy
//     continuation line for a paragraph deeper in. "- - x\n  y" re-parses
//     as one paragraph "x y", dropping the sibling paragraph outright. A
//     list ending in an empty item ("- 0.") absorbs nothing, and forcing a
//     blank there would eject the next block from the item.
func blockRunsToBlankLine(prev ast.Node) bool {
	switch n := prev.(type) {
	case *ast.Paragraph, *ast.Table:
		return true
	case *ast.List:
		return listTailAbsorbsNextLine(n)
	}
	return false
}

// listTailAbsorbsNextLine reports whether the last block of a list's last
// item continues onto the line after it.
func listTailAbsorbsNextLine(list *ast.List) bool {
	for _, v := range slices.Backward(list.Children) {
		item, ok := v.(*ast.ListItem)
		if !ok {
			continue
		}
		if len(item.Children) == 0 {
			return false
		}
		return blockRunsToBlankLine(item.Children[len(item.Children)-1])
	}
	return false
}

func (r *mdRenderer) renderTaskList(b *strings.Builder, node *ast.List, bullet string) {
	for idx := range node.Children {
		item, ok := node.Children[idx].(*ast.ListItem)
		if !ok {
			continue
		}
		inlines := firstParagraphInlines(item)
		if len(inlines) == 0 {
			// remark renders an empty task item as a bare marker — "- [ ] "
			// with no content would re-parse as a literal "[ ]". Follow
			// blocks without a leading paragraph are unreachable from the
			// conversions (they degrade before rendering) and are dropped
			// here like the historical renderer did.
			b.WriteString(bullet)
			b.WriteString("\n")
			continue
		}
		b.WriteString(bullet)
		if item.Checked != nil && *item.Checked {
			b.WriteString(" [x] ")
		} else {
			b.WriteString(" [ ] ")
		}
		// Wrap like any list item: the "- [ ] " prefix consumes 6 columns
		// and continuation lines align under the content.
		saved := r.prefixWidth
		r.prefixWidth += 6
		wrapped := wrapTextProtected(r.renderInlineString(inlines), r.availWidth())
		r.prefixWidth = saved
		firstLine := true
		for line := range strings.SplitSeq(wrapped, "\n") {
			if !firstLine {
				b.WriteString("\n      ")
			}
			firstLine = false
			b.WriteString(line)
		}
		b.WriteString("\n")
		r.renderTaskItemFollowBlocks(b, item)
	}
}

// renderTaskItemFollowBlocks renders a block task item's blocks after
// the marker-line paragraph, indented to the item's content column
// (2 spaces — the checkbox is paragraph content, not marker, so deeper
// indents would re-parse as indented code after a blank line). Items
// whose first child is not a paragraph never reach this point (the
// conversions degrade them first).
func (r *mdRenderer) renderTaskItemFollowBlocks(b *strings.Builder, item *ast.ListItem) {
	if len(item.Children) == 0 {
		return
	}
	if _, ok := item.Children[0].(*ast.Paragraph); !ok {
		return
	}
	var alt nestedListAlternation
	for i := 1; i < len(item.Children); i++ {
		r.renderItemFollowBlock(b, item, i, "  ", 0, false, false, &alt)
	}
}

// firstParagraphInlines returns the inline children of a list item's first
// paragraph. Task items always wrap their inline content in a single
// paragraph (see adfToAst).
func firstParagraphInlines(item *ast.ListItem) []ast.Node {
	for i := range item.Children {
		if p, ok := item.Children[i].(*ast.Paragraph); ok {
			return p.Children
		}
	}
	return nil
}
