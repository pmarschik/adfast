package markdown_test

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/markdown"
)

// WithGenericDirectives exists because promotion loses the author's
// directive name: five container names share dialect.Panel and two share
// dialect.Align, so a name is only recoverable from the unpromoted tree.
// These cases pin both halves — the default parse promoting, and the
// generic parse keeping Name — for one name of each directive form.
func TestParseWithGenericDirectives(t *testing.T) {
	tests := []struct {
		name             string
		src              string
		wantName         string
		wantPromotedKind string
	}{
		{"container", ":::info\ntext\n:::\n", "info", "panel"},
		{"container, second name on the same kind", ":::warning\ntext\n:::\n", "warning", "panel"},
		{"leaf", "::media[alt]{id=x}\n", "media", "media"},
		{"text", "a :status[Done]{color=green} b", "status", "status"},
		{"text, no label", "a :status b", "status", "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promoted := firstDirectiveBearer(t, markdown.Parse([]byte(tt.src)))
			if got := promoted.Kind(); got != tt.wantPromotedKind {
				t.Fatalf("default parse: kind = %q, want %q", got, tt.wantPromotedKind)
			}
			if name, ok := directiveName(promoted); ok {
				t.Fatalf("default parse left a generic directive %q; the case no longer covers promotion", name)
			}

			generic := firstDirectiveBearer(t, markdown.Parse([]byte(tt.src), markdown.WithGenericDirectives()))
			name, ok := directiveName(generic)
			if !ok {
				t.Fatalf("generic parse: got a promoted %q, want a generic directive node", generic.Kind())
			}
			if name != tt.wantName {
				t.Errorf("generic parse: Name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

// A name a WithExtensions registration owns is not promoted either: the
// option skips the promotion step whole, so no registration can hide a
// name from a caller reading the generic tree.
func TestParseWithGenericDirectivesIgnoresExtensions(t *testing.T) {
	root := markdown.Parse([]byte(":::info\ntext\n:::\n"),
		markdown.WithGenericDirectives(),
		markdown.WithExtensions(dialect.Registrations()...),
	)
	name, ok := directiveName(firstDirectiveBearer(t, root))
	if !ok || name != "info" {
		t.Errorf("registrations promoted through WithGenericDirectives: name = %q, ok = %v", name, ok)
	}
}

// directiveName reports n's name when n is one of the three generic
// directive kinds.
func directiveName(n ast.Node) (string, bool) {
	switch d := n.(type) {
	case *ast.ContainerDirective:
		return d.Name, true
	case *ast.LeafDirective:
		return d.Name, true
	case *ast.TextDirective:
		return d.Name, true
	}
	return "", false
}

// firstDirectiveBearer returns the first node under root that is neither a
// root nor a paragraph — the directive node, whichever form it took.
func firstDirectiveBearer(t *testing.T, root ast.Node) ast.Node {
	t.Helper()
	var found ast.Node
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if found != nil {
			return
		}
		for _, c := range ast.Children(n) {
			switch c.Kind() {
			case "paragraph", "text":
				walk(c)
			default:
				found = c
				return
			}
			if found != nil {
				return
			}
		}
	}
	walk(root)
	if found == nil {
		t.Fatal("no directive node in the parsed tree")
	}
	return found
}
