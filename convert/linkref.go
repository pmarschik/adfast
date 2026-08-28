package convert

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
)

// Link reference definitions lowered for ADF. ADF has one link mark with
// a URL on it and no notion of a destination written somewhere else, so
// the three kinds the markdown leg keeps (ast.Definition, ast.LinkRef and
// ast.ImageRef) cannot survive as themselves:
//
//   - a reference RESOLVES here. Its definition's destination becomes the
//     href of an ordinary inline link (or the URL of an ordinary image),
//     so the reader gets the same link they would have got from
//     "[text](url)" — only the two-places-in-the-source form is lost, and
//     nothing on the page changes;
//   - a definition DROPS. It is the source's bookkeeping, not content: it
//     renders as nothing on the page, and its destination has already
//     traveled to every use. A definition nothing uses drops with real
//     loss, though, so that one reports a diagnostic
//     (CodeUnusedDefinitionDropped).
//
// The flattening is one-way, like the footnote one (footnote.go): nothing
// in ADF decodes back to a reference, so FromADF has no case for these
// kinds and the md → ADF → md round trip returns inline links.

// definitionIndex indexes a document's link reference definitions for the
// resolution.
type definitionIndex struct {
	// byLabel maps a normalized label (ast.NormalizeLabel) to the FIRST
	// definition carrying it — CommonMark's rule when a label is defined
	// twice, and goldmark's when it paired the uses.
	byLabel map[string]*ast.Definition
	// used records the normalized labels some reference resolved against,
	// so the drop can tell an unused definition from a used one. It fills
	// during the conversion, which is why the diagnostics for it fire
	// after the body is converted (see definitionLosses).
	used map[string]bool
	// order holds every definition in document order, so the unused ones
	// are reported in the order the author wrote them rather than in map
	// order.
	order []*ast.Definition
}

// collectDefinitions indexes every link reference definition in the tree,
// in document order and wherever it sits: a definition is a block like any
// other and may be nested in a blockquote or a list item, while the label
// it defines is visible to the whole document.
func collectDefinitions(root ast.Node) definitionIndex {
	idx := definitionIndex{
		byLabel: make(map[string]*ast.Definition),
		used:    make(map[string]bool),
	}
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if def, ok := n.(*ast.Definition); ok {
			idx.order = append(idx.order, def)
			key := def.Identifier()
			if _, seen := idx.byLabel[key]; !seen {
				idx.byLabel[key] = def
			}
		}
		for _, kid := range ast.Children(n) {
			walk(kid)
		}
	}
	walk(root)
	return idx
}

// resolve returns the definition a label pairs with, and records the
// pairing so definitionLosses can stay quiet about it.
func (idx *definitionIndex) resolve(label string) (*ast.Definition, bool) {
	key := ast.NormalizeLabel(label)
	def, ok := idx.byLabel[key]
	if ok {
		idx.used[key] = true
	}
	return def, ok
}

// definitionLosses reports the definitions nothing referenced. It runs
// after the body is converted, because "nothing referenced it" is only
// known once every reference has been resolved.
func (c *astConverter) definitionLosses() {
	if c.diagnostics == nil {
		return
	}
	for _, def := range c.definitions.order {
		if c.definitions.used[def.Identifier()] {
			continue
		}
		c.diagnostics(Diagnostic{
			Code: CodeUnusedDefinitionDropped,
			Message: "link reference definition [" + def.Label + "]: " + def.URL +
				" dropped: nothing in the document references it and ADF has no definition construct",
		})
	}
}

// paragraphResolvingLoneImageRef substitutes the inline image an image
// reference stands for when it is a paragraph's only child, and answers
// the paragraph unchanged otherwise.
//
// It exists because that paragraph is not a paragraph in ADF: a lone image
// is promoted to block media (mediaSingle, or an attachment's media node —
// see convertParagraph). The promotion pattern-matches *ast.Image, so
// without the substitution "![logo]" would degrade to an inline image and
// then to a link, while "![logo](url)" became media. Resolving here also
// records the use, so the definition does not report as unused.
func (c *astConverter) paragraphResolvingLoneImageRef(node *ast.Paragraph) *ast.Paragraph {
	if len(node.Children) != 1 {
		return node
	}
	ref, ok := node.Children[0].(*ast.ImageRef)
	if !ok {
		return node
	}
	def, ok := c.definitions.resolve(ref.Label)
	if !ok {
		return node
	}
	out := *node
	out.Children = []ast.Node{&ast.Image{
		URL:      def.URL,
		Title:    def.Title,
		Children: ref.Children,
	}}
	return &out
}

// The link reference kinds joined the AST after ast.Visitor was published,
// so both converters implement its optional companion interface. The
// assertions are what keep the conversion exhaustive: without them a
// reference node would silently fall through to VisitExtension, which
// recurses into children — and a reference's children are its label, so
// every link would have quietly become plain text.
var (
	_ ast.ReferenceVisitor[[]adf.Node] = (*astBlockVisitor)(nil)
	_ ast.ReferenceVisitor[[]adf.Node] = (*inlineFlattener)(nil)
)

// VisitDefinition implements ast.ReferenceVisitor. A definition converts
// to nothing: its destination has already traveled to the uses, and ADF
// has nowhere to put the definition itself (see the file comment).
func (*astBlockVisitor) VisitDefinition(*ast.Definition) []adf.Node { return nil }

// VisitLinkRef implements ast.ReferenceVisitor. A reference in block
// position cannot come from the markdown parse (a reference is inline, so
// a paragraph always holds it); from a hand-built tree it still resolves,
// inside the paragraph ADF needs around inline content.
func (v *astBlockVisitor) VisitLinkRef(n *ast.LinkRef) []adf.Node {
	return singleBlock(&adf.Paragraph{Content: v.c.convertInlines([]ast.Node{n})})
}

// VisitImageRef implements ast.ReferenceVisitor (see VisitLinkRef).
func (v *astBlockVisitor) VisitImageRef(n *ast.ImageRef) []adf.Node {
	return singleBlock(&adf.Paragraph{Content: v.c.convertInlines([]ast.Node{n})})
}

// VisitDefinition implements ast.ReferenceVisitor. A definition in inline
// position (only a hand-built tree holds one) contributes nothing here
// either: it is a block, and it has no inline content to degrade to.
func (*inlineFlattener) VisitDefinition(*ast.Definition) []adf.Node { return nil }

// VisitLinkRef implements ast.ReferenceVisitor: the reference becomes the
// inline link its definition describes, so it takes every link path
// (smart links, the link resolver, file cards) an inline link takes.
//
// A label nothing defines cannot come from the markdown parse — an
// unmatched "[spec]" is literal text there — so the fallback below only
// answers a hand-built tree: the source brackets as literal text, with
// the children inside them, which is what the markdown renderer writes
// for the same node.
func (v *inlineFlattener) VisitLinkRef(n *ast.LinkRef) []adf.Node {
	def, ok := v.c.definitions.resolve(n.Label)
	if !ok {
		return v.unresolvedRef("", n.Label, n.ReferenceType, n.Children)
	}
	return v.c.flattenLink(&ast.Link{
		URL:      def.URL,
		Title:    def.Title,
		Explicit: true,
		Children: n.Children,
	}, v.ctx)
}

// VisitImageRef implements ast.ReferenceVisitor: the reference becomes the
// image its definition describes, and takes the same three fates an
// inline image takes (see inlineFlattener.VisitImage).
func (v *inlineFlattener) VisitImageRef(n *ast.ImageRef) []adf.Node {
	def, ok := v.c.definitions.resolve(n.Label)
	if !ok {
		return v.unresolvedRef("!", n.Label, n.ReferenceType, n.Children)
	}
	return v.VisitImage(&ast.Image{
		URL:      def.URL,
		Title:    def.Title,
		Children: n.Children,
	})
}

// unresolvedRef writes a reference no definition matched as the source
// text it was: the brackets as literal text under the inherited marks,
// with the children flattened between them for the full form so their own
// marks survive. lead is "" for a link and "!" for an image.
func (v *inlineFlattener) unresolvedRef(lead, label string, kind ast.ReferenceType, children []ast.Node) []adf.Node {
	marks := v.c.buildMarks(v.ctx)
	if kind == ast.ReferenceFull {
		out := []adf.Node{&adf.Text{Text: lead + "[", Marks: marks}}
		out = append(out, v.c.flattenChildren(children, v.ctx)...)
		return append(out, &adf.Text{Text: "][" + label + "]", Marks: v.c.buildMarks(v.ctx)})
	}
	tail := "]"
	if kind == ast.ReferenceCollapsed {
		tail = "][]"
	}
	return []adf.Node{&adf.Text{Text: lead + "[" + label + tail, Marks: marks}}
}
