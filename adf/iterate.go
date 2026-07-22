package adf

import "iter"

// Walk returns a preorder iterator over n and every node in its content
// subtree (each node before its children, siblings in document order).
// It yields NODES only — marks are attributes of their node, not tree
// members; read them via NodeMarks on the yielded node. The iteration
// is read-only: mutating content slices mid-walk is undefined.
func Walk(n Node) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		walkNode(n, yield)
	}
}

// walkNode yields n and recurses; false stops the walk.
func walkNode(n Node, yield func(Node) bool) bool {
	if !yield(n) {
		return false
	}
	for _, child := range NodeContent(n) {
		if !walkNode(child, yield) {
			return false
		}
	}
	return true
}

// Transform rewrites a document copy-on-write: f visits every node in
// preorder and returns (replacement, handled). When handled is true the
// node is replaced by the returned nodes verbatim — Transform does NOT
// recurse into replacements, so returning the node itself (with handled
// true) prunes its subtree from the rewrite (e.g. "leave code blocks
// alone"), and returning an empty slice deletes the node. When handled
// is false the node is kept and Transform recurses into its content.
// Untouched subtrees are shared with the input, not copied; the input
// document is never mutated.
func Transform(doc Doc, f func(Node) ([]Node, bool)) Doc {
	content, _ := transformNodes(doc.Content, f)
	return Doc{Type: doc.Type, Version: doc.Version, Content: content}
}

// transformNodes rewrites one content slice, reporting whether anything
// changed (unchanged slices are returned as-is for copy-on-write).
func transformNodes(nodes []Node, f func(Node) ([]Node, bool)) ([]Node, bool) {
	if len(nodes) == 0 {
		return nodes, false
	}
	changed := false
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if repl, handled := f(n); handled {
			if len(repl) != 1 || repl[0] != n {
				changed = true
			}
			out = append(out, repl...)
			continue
		}
		if content := NodeContent(n); len(content) > 0 {
			if newContent, contentChanged := transformNodes(content, f); contentChanged {
				n = WithContent(n, newContent)
				changed = true
			}
		}
		out = append(out, n)
	}
	if !changed {
		return nodes, false
	}
	return out, true
}
