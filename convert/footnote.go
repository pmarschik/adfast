package convert

import (
	"strconv"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
)

// GFM footnotes lowered for ADF. ADF has no footnote construct of any
// kind, so the pair the markdown leg keeps (ast.FootnoteRef and
// ast.FootnoteDef) flattens to the shape a reader of the rendered page
// can still follow: every reference becomes its number as superscript
// text, and the definitions collect at the end of the document behind a
// rule, as one ordered list whose item numbers are those numbers.
//
// The numbering is DEFINITION order, not first-reference order: the list
// at the end of the document is what a reader sees, and its own order is
// the only one the superscripts can agree with. Every definition gets a
// number of its own, duplicates included (remark keeps both definition
// nodes when "[^1]:" appears twice), so nothing is dropped; a reference
// resolves to the FIRST definition sharing its normalized label, like
// GFM.
//
// No reference carries a link mark to its definition: ADF has no anchor
// construct, so a "#fn-1" href would be a fabricated dead link. The
// superscript is the whole of the reference.
//
// The flattening is one-way. Nothing in ADF decodes back to a footnote,
// so FromADF has no footnote case; the md → ADF → md round trip returns
// the flattened form, which is why every flattened footnote reports a
// diagnostic (CodeFootnoteFlattened).

// footnoteIndex indexes a document's footnote definitions for the
// flattening.
type footnoteIndex struct {
	// nums maps a normalized label (ast.NormalizeFootnoteLabel) to the
	// number of the first definition carrying it.
	nums map[string]int
	// defs holds every definition in document order; a definition's
	// number is its index + 1, which is also its position in the emitted
	// ordered list.
	defs []*ast.FootnoteDef
}

// collectFootnotes indexes every footnote definition in the tree, in
// document order and wherever it sits: a definition is a block like any
// other, and may be nested in a blockquote or a list item (measured, and
// remark keeps it there too).
func collectFootnotes(root ast.Node) footnoteIndex {
	var idx footnoteIndex
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if def, ok := n.(*ast.FootnoteDef); ok {
			idx.defs = append(idx.defs, def)
			if idx.nums == nil {
				idx.nums = make(map[string]int)
			}
			key := ast.NormalizeFootnoteLabel(def.Label)
			if _, seen := idx.nums[key]; !seen {
				idx.nums[key] = len(idx.defs)
			}
		}
		for _, kid := range ast.Children(n) {
			walk(kid)
		}
	}
	walk(root)
	return idx
}

// footnoteTail converts the collected definitions to the document tail
// that holds them: the rule that separates them from the body, and the
// ordered list of their content. It is empty for a document without a
// footnote, and reports one diagnostic per definition.
func (c *astConverter) footnoteTail() []adf.Node {
	if len(c.footnotes.defs) == 0 {
		return nil
	}
	items := make([]adf.Node, 0, len(c.footnotes.defs))
	for i, def := range c.footnotes.defs {
		content := c.convertBlocks(def.Children)
		if len(content) == 0 {
			// An empty definition ("[^1]:" with nothing under it) still
			// takes an item, so the item numbers keep matching the
			// superscripts.
			content = []adf.Node{&adf.Paragraph{Content: []adf.Node{}}}
		}
		items = append(items, &adf.ListItem{Content: content})
		if c.diagnostics != nil {
			c.diagnostics(Diagnostic{
				Code: CodeFootnoteFlattened,
				Message: "footnote [^" + def.Label + "] flattened to superscript " +
					strconv.Itoa(i+1) + " with its definition in the list at the end of the document",
			})
		}
	}
	return []adf.Node{
		&adf.Rule{},
		&adf.OrderedList{Order: new(1), Content: items},
	}
}

// The footnote kinds joined the AST after ast.Visitor was published, so
// both converters implement its optional companion interface. The
// assertions are what keep the conversion exhaustive: without them a
// footnote node would silently fall through to VisitExtension.
var (
	_ ast.FootnoteVisitor[[]adf.Node] = (*astBlockVisitor)(nil)
	_ ast.FootnoteVisitor[[]adf.Node] = (*inlineFlattener)(nil)
)

// VisitFootnoteDef implements ast.FootnoteVisitor. A definition converts
// to nothing where it stands: footnoteTail emits every definition of the
// document, wherever it sat.
func (*astBlockVisitor) VisitFootnoteDef(*ast.FootnoteDef) []adf.Node { return nil }

// VisitFootnoteRef implements ast.FootnoteVisitor. A reference in block
// position cannot come from the markdown parse (a reference is inline,
// so a paragraph always holds it); from a hand-built tree it still
// flattens to its superscript, inside the paragraph ADF needs around
// inline content.
func (v *astBlockVisitor) VisitFootnoteRef(n *ast.FootnoteRef) []adf.Node {
	return singleBlock(&adf.Paragraph{Content: v.c.convertInlines([]ast.Node{n})})
}

// VisitFootnoteDef implements ast.FootnoteVisitor. A definition in inline
// position (only a hand-built tree holds one) contributes nothing here:
// footnoteTail emits it, and recursing into its blocks would duplicate
// that content inline.
func (*inlineFlattener) VisitFootnoteDef(*ast.FootnoteDef) []adf.Node { return nil }

// VisitFootnoteRef implements ast.FootnoteVisitor: the number of the
// definition the reference resolves to, as superscript text under the
// inherited marks. A label nothing defines cannot come from the markdown
// parse either (an unmatched "[^x]" stays literal text there); from a
// hand-built tree it keeps its source form as plain text, which is what
// remark renders for it.
func (v *inlineFlattener) VisitFootnoteRef(n *ast.FootnoteRef) []adf.Node {
	num, ok := v.c.footnotes.nums[ast.NormalizeFootnoteLabel(n.Label)]
	if !ok {
		return []adf.Node{&adf.Text{Text: "[^" + n.Label + "]", Marks: v.c.buildMarks(v.ctx)}}
	}
	ctx := v.ctx
	ctx.subsup = "sup"
	return []adf.Node{&adf.Text{Text: strconv.Itoa(num), Marks: v.c.buildMarks(ctx)}}
}
