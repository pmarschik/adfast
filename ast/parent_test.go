package ast

import "testing"

// TestEveryContainerIsParent pins the contract parent.go carries: every
// kind that holds children must satisfy Parent, because Children and
// SetChildren dispatch through nothing else. A container that loses (or
// never gets) the two methods still compiles; it just quietly drops out
// of every walk. This is the signal.
func TestEveryContainerIsParent(t *testing.T) {
	containers := []Node{
		&Root{}, &Paragraph{}, &Heading{}, &Blockquote{}, &FootnoteDef{},
		&List{}, &ListItem{}, &Table{}, &TableRow{}, &TableCell{},
		&ContainerDirective{}, &LeafDirective{}, &Emphasis{}, &Strong{},
		&Delete{}, &Link{}, &Image{}, &TextDirective{},
	}
	kid := &Text{Value: "x"}
	for _, n := range containers {
		if _, ok := n.(Parent); !ok {
			t.Errorf("%s does not implement Parent", n.Kind())
			continue
		}
		SetChildren(n, []Node{kid})
		got := Children(n)
		if len(got) != 1 || got[0] != kid {
			t.Errorf("%s: round trip through SetChildren/Children gave %v", n.Kind(), got)
		}
	}
}

// TestLeafKindsHaveNoChildren pins the other side: a leaf answers with
// nil children and ignores SetChildren rather than panicking.
func TestLeafKindsHaveNoChildren(t *testing.T) {
	leaves := []Node{
		&Text{}, &InlineCode{}, &Break{}, &ThematicBreak{}, &Code{},
		&HTML{}, &Frontmatter{}, &FootnoteRef{}, &foreignNode{},
	}
	for _, n := range leaves {
		SetChildren(n, []Node{&Text{Value: "x"}})
		if got := Children(n); got != nil {
			t.Errorf("%s: Children = %v, want nil", n.Kind(), got)
		}
	}
}
