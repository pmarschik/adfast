package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// headingOf parses src and returns its single heading.
func headingOf(t *testing.T, src string) *ast.Heading {
	t.Helper()
	root := Parse([]byte(src))
	kids := ast.Children(root)
	if len(kids) != 1 {
		t.Fatalf("parse %q: want 1 block, got %d", src, len(kids))
	}
	h, ok := kids[0].(*ast.Heading)
	if !ok {
		t.Fatalf("parse %q: want *ast.Heading, got %T", src, kids[0])
	}
	return h
}

func TestHeadingAnchor_Parse(t *testing.T) {
	tests := []struct {
		name, src, wantID, wantText string
	}{
		{"simple", "## Title {#my-anchor}\n", "my-anchor", "Title"},
		{"deep level", "###### T {#a}\n", "a", "T"},
		{"anchor only", "## {#solo}\n", "solo", ""},
		{"after emphasis", "## Title _em_ {#i}\n", "i", "Title em"},
		{"tab separated", "## Title\t{#t}\n", "t", "Title"},
		{"setext", "Title {#s}\n---\n", "s", "Title"},
		{"dots dashes underscores", "## T {#a.b-c_d9}\n", "a.b-c_d9", "T"},

		// Shapes the strip must leave as literal text.
		{"escaped brace", `## Title \{#lit}` + "\n", "", "Title {#lit}"},
		{"no separating space", "## Title{#x}\n", "", "Title{#x}"},
		{"empty id", "## Title {#}\n", "", "Title {#}"},
		{"id with space", "## Title {#a b}\n", "", "Title {#a b}"},
		{"class attribute", "## Title {.foo}\n", "", "Title {.foo}"},
		{"key value", "## Title {#a b=c}\n", "", "Title {#a b=c}"},
		{"bare braces", "## Title {bare}\n", "", "Title {bare}"},
		{"not at end", "## {#a} Title\n", "", "{#a} Title"},
		{"backslash in id", `## Title {#a\}b}` + "\n", "", "Title {#a}b}"},
		// A ':' opens a text directive, so ":b" is not plain text at all —
		// which is exactly why the id charset excludes it: there is no one
		// text node to strip and no way to render the id back verbatim.
		{"colon in id", "## T {#a:b}\n", "", "T {#a}"},
		{"leading underscore", "## T {#_a}\n", "", "T {#_a}"},
		{"inside code span", "## Title `{#c}`\n", "", "Title {#c}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := headingOf(t, tc.src)
			if h.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", h.ID, tc.wantID)
			}
			if got := ast.PlainText(h.Children); got != tc.wantText {
				t.Errorf("text = %q, want %q", got, tc.wantText)
			}
		})
	}
}

// TestHeadingAnchor_RoundTrip pins md → ast → md as byte-identical: both
// for real anchors and for the literal {#…} shapes that must not become
// one. The escaped form is the acceptance criterion for a literal anchor
// surviving the trip.
func TestHeadingAnchor_RoundTrip(t *testing.T) {
	srcs := []string{
		"## Title {#my-anchor}\n",
		"# Top {#top}\n",
		"###### Deep {#d}\n",
		"## {#solo}\n",
		"## Title _em_ {#i}\n",
		`## Title \{#lit}` + "\n",
		"## Title{#x}\n",
		"## Title {#}\n",
		"## Title {#a b}\n",
		"## Title {.foo}\n",
		"## Title {bare}\n",
		"## Title `{#c}`\n",
		"## Title# {#a}\n",
		"## T {#a:b}\n",
		"## T {#a.b-c_d9}\n",
		// Rejected id shapes stay literal text, so the inline renderer's own
		// escaping applies to them ("_" could open emphasis).
		`## T {#\_a}` + "\n",
	}
	for _, src := range srcs {
		t.Run(src, func(t *testing.T) {
			got := Render(Parse([]byte(src)))
			if got != src {
				t.Fatalf("round trip:\n got %q\nwant %q", got, src)
			}
			// Idempotent on a second pass too.
			if again := Render(Parse([]byte(got))); again != got {
				t.Fatalf("not idempotent:\n got %q\nwant %q", again, got)
			}
		})
	}
}

// TestHeadingAnchor_EscapeOnRender covers the direction the parser cannot
// reach: an AST built by hand (or decoded from ADF) whose heading text
// ends in the anchor shape must render with the brace escaped so it does
// not re-parse as an anchor.
func TestHeadingAnchor_EscapeOnRender(t *testing.T) {
	tests := []struct{ name, text, id, want string }{
		{"literal tail escapes", "Title {#lit}", "", `## Title \{#lit}` + "\n"},
		{"anchor wins over tail", "Title", "lit", "## Title {#lit}\n"},
		{"no whitespace, no escape", "Title{#lit}", "", "## Title{#lit}\n"},
		{"whole text is the shape", "{#lit}", "", `## \{#lit}` + "\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &ast.Heading{Depth: 2, ID: tc.id, Children: []ast.Node{&ast.Text{Value: tc.text}}}
			root := &ast.Root{Children: []ast.Node{h}}
			got := Render(root)
			if got != tc.want {
				t.Fatalf("render = %q, want %q", got, tc.want)
			}
			// And it must survive a re-parse unchanged.
			if again := Render(Parse([]byte(got))); again != got {
				t.Fatalf("re-parse changed it:\n got %q\nwant %q", again, got)
			}
		})
	}
}
