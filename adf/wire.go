package adf

// Wire safety: the markdown conversion can leave a small number of
// synthetic, never-wire constructs on a document — the ColwidthsHint
// placeholder (::colwidths before its table resolves; it never appears
// in convert output but can be built by hand), the "tight"
// list-tightness attribute written by WithPreserveListTightness, the
// heading "anchor" and the table "align" attribute. Such
// documents must not be submitted to the host product as-is: IsWireSafe
// is the guard consumers can run before submission, and StripSynthetic
// the corresponding cleanup.
//
// Historically the style-preserving formatter also parked frontmatter,
// raw HTML, inline images, and md* style attributes on the ADF tree;
// The prettier md→ast→md format path carries no ADF leg, so those synthetic
// kinds no longer exist and formatter output never touches ADF.

// IsWireSafe reports whether the document is free of the synthetic,
// never-wire constructs the markdown conversion can produce:
//
//   - the ColwidthsHint placeholder kind,
//   - the "tight" list flag written by WithPreserveListTightness,
//   - the "anchor" heading attribute (ast.Heading.ID; ADF has no neutral
//     anchor construct, so a product addon must lower or drop it —
//     confluence.MarkdownOptions lowers it to an anchor-macro
//     inlineExtension, jira.MarkdownOptions drops it with a diagnostic),
//     and
//   - the "align" table attribute (ast.Table.Align, a GFM table's column
//     alignment; ADF tables have no alignment attribute, so it must be
//     lowered onto the alignment block mark of the blocks in each column
//     — see LowerTableAlign, which both products' MarkdownOptions
//     install — or dropped by StripSynthetic).
//
// Documents produced by the canonical conversion of markdown without
// heading anchors or table alignment, and without tightness preservation,
// are always wire-safe. A false result means one of the above is present
// and the document must not be submitted as-is; see StripSynthetic.
func IsWireSafe(doc Doc) bool {
	for _, root := range doc.Content {
		for n := range Walk(root) {
			if !nodeWireSafe(n) {
				return false
			}
		}
	}
	return true
}

// syntheticCarrier is implemented by the four kinds that can carry a
// synthetic, never-wire ATTRIBUTE (ColwidthsHint is a synthetic kind, so
// it needs no attribute answer). The interface is optional rather than
// part of Node because the other kinds have nothing to say — and because
// it gives IsWireSafe and StripSynthetic one source of truth instead of
// two parallel type switches that drift apart when a fifth synthetic
// attribute arrives.
type syntheticCarrier interface {
	Node
	// hasSynthetic reports whether the synthetic attribute is set.
	hasSynthetic() bool
	// clearedCopy is a shallow copy with the synthetic attribute cleared.
	clearedCopy() Node
}

func (n *Heading) hasSynthetic() bool { return n.Anchor != "" }
func (n *Heading) clearedCopy() Node  { c := *n; c.Anchor = ""; return &c }

func (n *Table) hasSynthetic() bool { return n.Align != nil }
func (n *Table) clearedCopy() Node  { c := *n; c.Align = nil; return &c }

func (n *BulletList) hasSynthetic() bool { return n.Tight != nil }
func (n *BulletList) clearedCopy() Node  { c := *n; c.Tight = nil; return &c }

func (n *OrderedList) hasSynthetic() bool { return n.Tight != nil }
func (n *OrderedList) clearedCopy() Node  { c := *n; c.Tight = nil; return &c }

// nodeWireSafe checks one node for synthetic kinds and style markers.
func nodeWireSafe(n Node) bool {
	if _, isHint := n.(*ColwidthsHint); isHint {
		return false
	}
	c, ok := n.(syntheticCarrier)
	return !ok || !c.hasSynthetic()
}

// StripSynthetic returns a copy of the document with every synthetic,
// never-wire construct removed: ColwidthsHint nodes are dropped (they
// have no wire form) and the "tight", heading "anchor" and table "align"
// flags are cleared. Clearing an anchor or a table alignment loses it
// silently, so lower them first: confluence.MarkdownOptions lowers
// anchors to the anchor macro, and both products' MarkdownOptions lower
// table alignment to the alignment block mark (see LowerTableAlign). The
// result
// satisfies IsWireSafe. The rewrite is copy-on-write; the input
// document is never mutated.
func StripSynthetic(doc Doc) Doc {
	content, _ := stripNodes(doc.Content)
	return Doc{Type: doc.Type, Version: doc.Version, Content: content}
}

// stripNodes rewrites one content slice copy-on-write.
func stripNodes(nodes []Node) ([]Node, bool) {
	if len(nodes) == 0 {
		return nodes, false
	}
	changed := false
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if _, isHint := n.(*ColwidthsHint); isHint {
			changed = true
			continue
		}
		if stripped, nodeChanged := stripNode(n); nodeChanged {
			n = stripped
			changed = true
		}
		out = append(out, n)
	}
	if !changed {
		return nodes, false
	}
	return out, true
}

// stripNode clears a single node's synthetic attribute and recurses into
// its content, copying the node only when something changes.
func stripNode(n Node) (Node, bool) {
	copied := false
	if c, ok := n.(syntheticCarrier); ok && c.hasSynthetic() {
		n = c.clearedCopy()
		copied = true
	}
	newContent, contentChanged := stripNodes(NodeContent(n))
	if !contentChanged {
		return n, copied
	}
	if !copied {
		n = n.shallowCopy()
		copied = true
	}
	if s := n.slots(); s.content != nil {
		*s.content = newContent
	}
	return n, copied
}
