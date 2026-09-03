package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/markdown"
)

// These are PRESERVED-BEHAVIOR PINS, not regression tests: the behavior
// they capture is unchanged and deliberate. They exist because Parse's and
// Render's doc comments now state it in concrete terms — that Parse is not
// frontmatter aware, that a YAML fence therefore lifts to a thematicBreak
// plus a setext heading, and that Render nevertheless emits an
// ast.Frontmatter child verbatim. A silent change to any of those would
// leave the doc comments lying, which the pins prevent.

// TestParseIsNotFrontmatterAware pins the shape Parse produces for a
// document opening a YAML frontmatter fence, and the prose Render then
// writes back. The metadata split is the root facade's job (its
// FrontmatterProvider), and the dependency runs facade → markdown, so this
// package structurally cannot reach it.
func TestParseIsNotFrontmatterAware(t *testing.T) {
	const in = "---\nstatus: Open\n---\nBody.\n"

	root := markdown.Parse([]byte(in))

	var kinds []string
	for _, child := range ast.Children(root) {
		kinds = append(kinds, child.Kind())
	}
	if got, want := strings.Join(kinds, ","), "thematicBreak,heading,paragraph"; got != want {
		t.Errorf("Parse(%q) child kinds = %q, want %q", in, got, want)
	}

	// The closing fence acted as a setext underline, so "status: Open"
	// became a level-2 heading rather than metadata.
	if got, want := markdown.Render(root), "---\n\n## status: Open\n\nBody.\n"; got != want {
		t.Errorf("Render(Parse(%q)) = %q, want %q", in, got, want)
	}
}

// TestRenderWritesFrontmatterVerbatim pins the other half of the
// asymmetry: Render can write a metadata block that Parse can never build.
// A caller that splits frontmatter itself — or the facade, which does —
// prepends the node and gets the block back byte for byte.
func TestRenderWritesFrontmatterVerbatim(t *testing.T) {
	const front = "---\nstatus: Open\n---\n"

	root := markdown.Parse([]byte("Body.\n"))
	ast.SetChildren(root, append(
		[]ast.Node{&ast.Frontmatter{Value: front}},
		ast.Children(root)...,
	))

	if got, want := markdown.Render(root), front+"\nBody.\n"; got != want {
		t.Errorf("Render(frontmatter + body) = %q, want %q", got, want)
	}
}
