package adf

// The listItem content model. ADF gives listItem, per the pinned schema
// oracle (docs/adf-coverage.md:122, atlassian-frontend-mirror commit
// f5ca0f120c6ea5d79873805d081a72c82917e1f8,
// editor/adf-schema/src/schema/nodes/list-item.ts, which delegates to
// listItemFactory):
//
//	(paragraph | bulletList | orderedList | taskList | mediaSingle |
//	 codeBlock | unsupportedBlock | extension)+
//
// a single flat, repeatable alternation. A blockquote, a table, a
// heading, a rule, a panel, an expand, a mediaGroup or a decisionList
// inside a list item is not representable, however sensible the
// markdown that produced it; extension and unsupportedBlock ARE
// representable there. A document that violates this model still
// encodes exactly as written — see ListItemViolations — but a product
// that enforces the schema on save (Confluence, measured 2026-08-26)
// rewrites the offending subtree into a wrapper of its own; see
// confluence.ExpandLegacyContent, which reads that wrapper back.
//
// The pinned model has no first-position restriction to enforce: it is
// one flat "+" alternation, not two positions. Older ADF schema
// revisions carried a first-position rule (only paragraph, mediaSingle,
// or codeBlock could open a listItem); this repo's pin postdates that
// rule. adfast does not enforce any ordering, and none is needed against
// the pinned schema. The one rewrite actually observed (see
// confluence.ExpandLegacyContent) was a set violation — a kind outside
// the alternation entirely — not an order violation, and no order
// violation has been observed to be rewritten, so this note does not
// claim more than that.

// listItemAllowed is the content model's alternation.
var listItemAllowed = map[string]bool{
	"paragraph": true, "bulletList": true, "orderedList": true,
	"taskList": true, "mediaSingle": true, "codeBlock": true,
	"unsupportedBlock": true, "extension": true,
}

// ListItemAllows reports whether a node kind may appear inside a
// listItem. An unknown kind (a RawNode's wire type, a custom extension)
// answers false: the content model is a whitelist and nothing outside it
// is known to be carried.
func ListItemAllows(kind string) bool { return listItemAllowed[kind] }

// ListItemViolation is one node found inside a listItem that the
// listItem content model does not permit.
type ListItemViolation struct {
	Item *ListItem
	Kind string
}

// ListItemViolations returns every direct child of every listItem in the
// document that the content model does not permit, in document order.
// A conforming document returns nil. The document is not modified and
// not copied.
func ListItemViolations(doc Doc) []ListItemViolation {
	var out []ListItemViolation
	for _, root := range doc.Content {
		for n := range Walk(root) {
			item, ok := n.(*ListItem)
			if !ok {
				continue
			}
			for _, child := range item.Content {
				if k := child.Kind(); !ListItemAllows(k) {
					out = append(out, ListItemViolation{Kind: k, Item: item})
				}
			}
		}
	}
	return out
}
