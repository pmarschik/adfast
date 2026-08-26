package ast

import "testing"

// PlainText used to fall through to its default branch for a
// *TextDirective — recursing into Children(n), which is empty for a bare,
// unregistered directive — and contribute the empty string. That silently
// dropped the ":name" text a caller relying on PlainText as "what the
// reader sees" needed (see markdown/render_inline.go's image alt and
// internal/pages/alttext.go in the storysmith-md consumer). The literal
// form pinned here matches the other two renderers that already build it:
// convert/ast_to_adf.go's flattenTextDirective and convert/normalize.go's
// md formatter.
func TestPlainText_TextDirective(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		nodes []Node
	}{
		{
			name:  "bare directive, no label",
			nodes: []Node{&TextDirective{Name: "view"}},
			want:  ":view",
		},
		{
			name:  "directive with a label",
			nodes: []Node{&TextDirective{Name: "view", Children: []Node{&Text{Value: "label"}}}},
			want:  ":viewlabel",
		},
		{
			name: "intraword: text, then directive",
			nodes: []Node{
				&Text{Value: "Over"},
				&TextDirective{Name: "view"},
			},
			want: "Over:view",
		},
		{
			name:  "nested inside a styled wrapper",
			nodes: []Node{&Strong{Children: []Node{&TextDirective{Name: "view"}}}},
			want:  ":view",
		},
		{
			name:  "attributes drop, matching the other two renderers",
			nodes: []Node{&TextDirective{Name: "view", Attrs: map[string]string{"k": "v"}}},
			want:  ":view",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlainText(tt.nodes); got != tt.want {
				t.Errorf("PlainText = %q, want %q", got, tt.want)
			}
		})
	}
}
