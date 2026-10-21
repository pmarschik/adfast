package convert

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// TestMergeAdjacentText covers mergeAdjacentText directly: the
// BlockTaskItem paragraph-flattening degrade (convertAdfTaskList) builds one
// paragraph's children by concatenating each source paragraph's inlines
// with no separator, so a digit run can end one paragraph's text node and a
// '.'/')' begin the next one's. render_escape.go's escapeOrderedMarker only
// looks for a line-start digit run within its OWN node's string
// (digitRunFromLineStart), so a run split across two adjacent text nodes is
// invisible to the node holding the trailing punctuation, and the escape
// that would prevent "0)" from re-parsing as an ordered-list marker never
// fires. Merging adjacent bare text nodes before they reach render restores
// the single-node locality the escape rules assume.
func TestMergeAdjacentText(t *testing.T) {
	tests := map[string]struct {
		in   []ast.Node
		want []ast.Node
	}{
		"two adjacent bare text nodes merge": {
			in: []ast.Node{
				&ast.Text{Value: "0"},
				&ast.Text{Value: ")"},
			},
			want: []ast.Node{&ast.Text{Value: "0)"}},
		},
		"three adjacent bare text nodes merge into one": {
			in: []ast.Node{
				&ast.Text{Value: "a"},
				&ast.Text{Value: "b"},
				&ast.Text{Value: "c"},
			},
			want: []ast.Node{&ast.Text{Value: "abc"}},
		},
		"a mark-wrapper between two text nodes breaks the merge": {
			in: []ast.Node{
				&ast.Text{Value: "0"},
				&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "x"}}},
				&ast.Text{Value: ")"},
			},
			want: []ast.Node{
				&ast.Text{Value: "0"},
				&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "x"}}},
				&ast.Text{Value: ")"},
			},
		},
		"a single text node is unchanged": {
			in:   []ast.Node{&ast.Text{Value: "solo"}},
			want: []ast.Node{&ast.Text{Value: "solo"}},
		},
		"no nodes stays empty": {
			in:   nil,
			want: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := mergeAdjacentText(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("merged to %d nodes, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range got {
				gt, gIsText := got[i].(*ast.Text)
				wt, wIsText := tc.want[i].(*ast.Text)
				if gIsText != wIsText {
					t.Fatalf("node %d kind mismatch: got %T, want %T", i, got[i], tc.want[i])
				}
				if gIsText && gt.Value != wt.Value {
					t.Errorf("node %d text = %q, want %q", i, gt.Value, wt.Value)
				}
			}
		})
	}
}
