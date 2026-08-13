package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// An empty paragraph is something only ADF can hold: Markdown writes it as a
// blank line, and a blank line between blocks re-parses as separation rather
// than content, so rendering one would put the tree a step further from itself
// on every pass. Render drops them, wherever the run of blocks sits.
func TestRender_BlankParagraphsAreDropped(t *testing.T) {
	para := func(text string) ast.Node {
		if text == "" {
			return &ast.Paragraph{}
		}
		return &ast.Paragraph{Children: []ast.Node{&ast.Text{Value: text}}}
	}

	cases := []struct {
		name string
		root ast.Node
		want string
	}{
		{
			name: "trailing",
			root: &ast.Root{Children: []ast.Node{para("hi"), para("")}},
			want: "hi\n",
		},
		{
			name: "between two blocks",
			root: &ast.Root{Children: []ast.Node{para("hi"), para(""), para("bye")}},
			want: "hi\n\nbye\n",
		},
		{
			name: "a run of them",
			root: &ast.Root{Children: []ast.Node{para(""), para(""), para("hi")}},
			want: "hi\n",
		},
		{
			name: "a paragraph holding only an empty text node",
			root: &ast.Root{Children: []ast.Node{
				&ast.Paragraph{Children: []ast.Node{&ast.Text{Value: ""}}}, para("hi"),
			}},
			want: "hi\n",
		},
		{
			name: "inside a container directive",
			root: &ast.Root{Children: []ast.Node{&ast.ContainerDirective{
				Name: "excerpt", Children: []ast.Node{para("hi"), para("")},
			}}},
			want: ":::excerpt\nhi\n:::\n",
		},
		{
			name: "inside a blockquote",
			root: &ast.Root{Children: []ast.Node{&ast.Blockquote{
				Children: []ast.Node{para("hi"), para(""), para("bye")},
			}}},
			want: "> hi\n>\n> bye\n",
		},
		{
			name: "nothing but blanks",
			root: &ast.Root{Children: []ast.Node{para(""), para("")}},
			want: "\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.root)
			if got != tc.want {
				t.Errorf("Render = %q, want %q", got, tc.want)
			}
			// The point of dropping them: what comes out parses back to itself.
			if again := Render(Parse([]byte(got))); again != got {
				t.Errorf("render not stable: %q then %q", got, again)
			}
		})
	}
}

// A container directive's label is a paragraph too, and an empty one still has
// to render — dropping it would move the body up onto the fence line.
func TestRender_BlankDirectiveLabelSurvives(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{&ast.ContainerDirective{
		Name: "expand",
		Children: []ast.Node{
			&ast.Paragraph{DirectiveLabel: true},
			&ast.Paragraph{Children: []ast.Node{&ast.Text{Value: "hi"}}},
		},
	}}}
	want := ":::expand[]\nhi\n:::\n"
	if got := Render(root); got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}
