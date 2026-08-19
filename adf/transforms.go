package adf

import (
	"reflect"
	"regexp"
	"strings"
)

var (
	newlineRunRe = regexp.MustCompile(`[ \t]*\n+[ \t]*`)
	spaceRunRe   = regexp.MustCompile(` {2,}`)
)

// NormalizeTextNewlines collapses newline runs (with their surrounding
// spaces and tabs) inside non-code text nodes to single spaces — the
// spec-level normalization the canonical markdown→ADF conversion always
// applies. Code block content is left untouched.
//
// A space run that spans the junction between two adjacent text nodes
// with the same marks collapses too. Such a junction is what a dropped
// inline node leaves behind (":u" with no label in "[0 :u ]" converts to
// nothing, leaving "0 " next to " ]"), and markdown cannot write it: the
// two nodes render contiguously, and re-parsing the result merges them
// into one text node that this same collapse then shortens. Normalizing
// here is what keeps the md → adf → md round trip idempotent.
func NormalizeTextNewlines(doc Doc) Doc {
	doc = Transform(doc, func(n Node) ([]Node, bool) {
		switch t := n.(type) {
		case *CodeBlock:
			// Code content stays verbatim: keep the node and prune the
			// rewrite from its subtree.
			return []Node{t}, true
		case *Text:
			if t.Text == "" {
				return nil, false
			}
			normalized := newlineRunRe.ReplaceAllString(t.Text, " ")
			normalized = spaceRunRe.ReplaceAllString(normalized, " ")
			if normalized == t.Text {
				return nil, false
			}
			nt := *t
			nt.Text = normalized
			return []Node{&nt}, true
		}
		return nil, false
	})
	content, _ := collapseTextJunctions(doc.Content)
	return Doc{Type: doc.Type, Version: doc.Version, Content: content}
}

// collapseTextJunctions drops the leading spaces a text node contributes
// to a space run continuing from the text node before it, for every
// content slice in the tree. Nodes that would be left empty are dropped,
// so a run of pure-space nodes collapses to the single space its first
// node already carries. Code block content is skipped, like the per-node
// pass. Unchanged slices are returned as-is for copy-on-write.
func collapseTextJunctions(nodes []Node) ([]Node, bool) {
	if len(nodes) == 0 {
		return nodes, false
	}
	changed := false
	out := make([]Node, 0, len(nodes))
	// prev is the last text node written to out; the run it may continue
	// is broken by any other kind, and does not cross a content slice.
	var prev *Text
	for _, n := range nodes {
		if _, isCode := n.(*CodeBlock); !isCode {
			if content := NodeContent(n); len(content) > 0 {
				if newContent, contentChanged := collapseTextJunctions(content); contentChanged {
					n = WithContent(n, newContent)
					changed = true
				}
			}
		}
		t, isText := n.(*Text)
		if !isText {
			prev = nil
			out = append(out, n)
			continue
		}
		if prev != nil && strings.HasSuffix(prev.Text, " ") && marksEqual(prev.Marks, t.Marks) {
			if trimmed := strings.TrimLeft(t.Text, " "); trimmed != t.Text {
				nt := *t
				nt.Text = trimmed
				t = &nt
				changed = true
				if trimmed == "" {
					continue
				}
			}
		}
		prev = t
		out = append(out, t)
	}
	if !changed {
		return nodes, false
	}
	return out, true
}

// marksEqual reports whether two mark lists are the same — the condition
// under which re-parsing merges the text nodes carrying them.
func marksEqual(a, b []Mark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}
