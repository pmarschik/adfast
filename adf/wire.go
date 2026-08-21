package adf

// Wire safety: the markdown conversion can leave a small number of
// synthetic, never-wire constructs on a document — the ColwidthsHint
// placeholder (::colwidths before its table resolves; it never appears
// in convert output but can be built by hand) and the "tight"
// list-tightness attribute written by WithPreserveListTightness. Such
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
//   - the "tight" list flag written by WithPreserveListTightness, and
//   - the "anchor" heading attribute (ast.Heading.ID; ADF has no neutral
//     anchor construct, so a product addon must lower or drop it —
//     confluence.MarkdownOptions lowers it to an anchor-macro
//     inlineExtension, jira.MarkdownOptions drops it with a diagnostic).
//
// Documents produced by the canonical conversion of markdown without
// heading anchors, and without tightness preservation, are always
// wire-safe. A false result means one of the above is present and the
// document must not be submitted as-is; see StripSynthetic.
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

// nodeWireSafe checks one node for synthetic kinds and style markers.
func nodeWireSafe(n Node) bool {
	switch t := n.(type) {
	case *ColwidthsHint:
		return false
	case *Heading:
		return t.Anchor == ""
	case *BulletList:
		return t.Tight == nil
	case *OrderedList:
		return t.Tight == nil
	}
	return true
}

// StripSynthetic returns a copy of the document with every synthetic,
// never-wire construct removed: ColwidthsHint nodes are dropped (they
// have no wire form) and the "tight" and heading "anchor" flags are
// cleared. Clearing an anchor loses it silently — a product addon that
// can express anchors should lower them first (see
// confluence.MarkdownOptions). The result
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

// stripNode clears a single node's tightness flag and recurses into its
// content, copying the node only when something changes.
func stripNode(n Node) (Node, bool) {
	copied := false
	ensure := func() {
		if !copied {
			n = copyNode(n)
			copied = true
		}
	}
	switch t := n.(type) {
	case *Heading:
		if t.Anchor != "" {
			ensure()
			if h, ok := n.(*Heading); ok {
				h.Anchor = ""
			}
		}
	case *BulletList:
		if t.Tight != nil {
			ensure()
			if bl, ok := n.(*BulletList); ok {
				bl.Tight = nil
			}
		}
	case *OrderedList:
		if t.Tight != nil {
			ensure()
			if ol, ok := n.(*OrderedList); ok {
				ol.Tight = nil
			}
		}
	}
	if content := NodeContent(n); len(content) > 0 {
		if newContent, contentChanged := stripNodes(content); contentChanged {
			ensure()
			if s := slotsOf(n); s.content != nil {
				*s.content = newContent
			}
		}
	}
	return n, copied
}
