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
// AN UNREFERENCED DEFINITION IS KEPT. A definition no reference resolves
// to still becomes an item of the list. That was measured against remark
// (2026-09-04) because it looks like a divergence, and the two remark
// legs disagree with each other on it. For
//
//	[^n]: Note one.
//
//	[^m]: Note two.
//
//	Body uses only one.[^n]
//
// remark's md → md render returns both definitions verbatim, which is
// what this package's own formatter leg returns too, byte for byte.
// remark-rehype's HTML render drops "[^m]" and renumbers by
// FIRST-REFERENCE order. Neither is the authority here, and the reason to
// follow the md leg rather than the HTML one is the difference in what
// the output IS: an HTML render is a view, and the markdown source behind
// it still holds the definition, so nothing is lost by leaving it out. An
// ADF encode is a push — the ADF becomes the stored document, with no
// source behind it — so dropping the definition there deletes the
// author's text for good, against the whole point of the flatten, which
// is that nothing is lost from the page. GFM's own reason for the drop
// does not carry over either: HTML omits an unreferenced definition
// because its back-reference anchor would point nowhere, and this flatten
// emits no anchors at all.
//
// A caller that wants GFM's drop can have it: the CodeFootnoteFlattened
// diagnostic says which definitions are unreferenced. A caller cannot
// recover a definition this package has already deleted, so keeping is
// the direction that leaves the choice open.
//
// The same measurement shows the numbering itself diverging from the HTML
// oracle whether or not an unreferenced definition is involved. For a
// body referencing "[^b]" before "[^a]", with the definitions in the
// order a then b, remark-rehype numbers b=1 and a=2 and reorders the list
// to match; this package numbers a=1 and b=2, in definition order, and
// the list order agrees with the superscripts either way. That is the
// deliberate choice above, not a consequence of the keep.
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
	// refs holds every normalized label some reference in the document
	// uses. It does not change what is emitted — every definition is
	// emitted either way (see the note above) — it only lets the
	// diagnostic tell an unreferenced definition apart from a referenced
	// one, which is the hook a caller needs to apply GFM's HTML drop for
	// itself.
	refs map[string]bool
	// defs holds every definition in document order; a definition's
	// number is its index + 1, which is also its position in the emitted
	// ordered list.
	defs []*ast.FootnoteDef
}

// reachable reports whether some reference resolves to the definition at
// index i. A reference resolves to the FIRST definition sharing its
// normalized label, so a duplicate definition of a referenced label is
// itself unreachable.
func (idx footnoteIndex) reachable(i int) bool {
	key := ast.NormalizeFootnoteLabel(idx.defs[i].Label)
	return idx.refs[key] && idx.nums[key] == i+1
}

// collectFootnotes indexes every footnote definition in the tree, in
// document order and wherever it sits: a definition is a block like any
// other, and may be nested in a blockquote or a list item (measured, and
// remark keeps it there too).
func collectFootnotes(root ast.Node) footnoteIndex {
	var idx footnoteIndex
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		switch v := n.(type) {
		case *ast.FootnoteDef:
			idx.defs = append(idx.defs, v)
			if idx.nums == nil {
				idx.nums = make(map[string]int)
			}
			key := ast.NormalizeFootnoteLabel(v.Label)
			if _, seen := idx.nums[key]; !seen {
				idx.nums[key] = len(idx.defs)
			}
		case *ast.FootnoteRef:
			if idx.refs == nil {
				idx.refs = make(map[string]bool)
			}
			idx.refs[ast.NormalizeFootnoteLabel(v.Label)] = true
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
				Code:    CodeFootnoteFlattened,
				Message: footnoteFlattenedMessage(def.Label, i+1, c.footnotes.reachable(i)),
			})
		}
	}
	return []adf.Node{
		&adf.Rule{},
		&adf.OrderedList{Order: new(1), Content: items},
	}
}

// footnoteFlattenedMessage words the flatten for one definition. A
// definition no reference resolves to still becomes an item — nothing is
// dropped — but it has no superscript anywhere in the document, so
// claiming it was "flattened to superscript N" would name a number the
// reader cannot find. Say what actually happened instead, and let the
// caller decide whether to drop it (GFM's HTML renderer does; remark's
// own md → md render does not, and neither does this one).
func footnoteFlattenedMessage(label string, num int, reachable bool) string {
	if !reachable {
		return "footnote [^" + label + "] is defined but never referenced; " +
			"it is kept as item " + strconv.Itoa(num) +
			" of the list at the end of the document, with no superscript pointing at it"
	}
	return "footnote [^" + label + "] flattened to superscript " + strconv.Itoa(num) +
		" with its definition in the list at the end of the document"
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
