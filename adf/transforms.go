package adf

import (
	"regexp"
)

var (
	newlineRunRe = regexp.MustCompile(`[ \t]*\n+[ \t]*`)
	spaceRunRe   = regexp.MustCompile(` {2,}`)
)

// NormalizeTextNewlines collapses newline runs (with their surrounding
// spaces and tabs) inside non-code text nodes to single spaces — the
// spec-level normalization the canonical markdown→ADF conversion always
// applies. Code block content is left untouched.
func NormalizeTextNewlines(doc Doc) Doc {
	return Transform(doc, func(n Node) ([]Node, bool) {
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
}
