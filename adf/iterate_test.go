package adf

import (
	"testing"
)

func sampleDoc() Doc {
	return Doc{Type: "doc", Version: 1, Content: []Node{
		&Paragraph{Content: []Node{
			&Text{Text: "hello"},
			&Text{Text: "world", Marks: []Mark{&Strong{}}},
		}},
		&Blockquote{Content: []Node{
			&Paragraph{Content: []Node{&Text{Text: "quoted"}}},
		}},
		&CodeBlock{Language: "go", Content: []Node{&Text{Text: "code  text"}}},
	}}
}

func TestWalk_PreorderNodesOnly(t *testing.T) {
	doc := sampleDoc()
	var kinds []string
	for _, root := range doc.Content {
		for n := range Walk(root) {
			kinds = append(kinds, n.Kind())
		}
	}
	want := []string{
		"paragraph", "text", "text",
		"blockquote", "paragraph", "text",
		"codeBlock", "text",
	}
	if len(kinds) != len(want) {
		t.Fatalf("Walk yielded %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("Walk order %v, want %v", kinds, want)
		}
	}
}

func TestWalk_EarlyStop(t *testing.T) {
	doc := sampleDoc()
	count := 0
	for range Walk(doc.Content[0]) {
		count++
		break
	}
	if count != 1 {
		t.Errorf("early break iterated %d nodes, want 1", count)
	}
}

func TestTransform_ReplaceAndPrune(t *testing.T) {
	doc := sampleDoc()
	out := Transform(doc, func(n Node) ([]Node, bool) {
		switch t := n.(type) {
		case *CodeBlock:
			// Prune: keep verbatim, no recursion.
			return []Node{t}, true
		case *Text:
			if t.Text == "hello" {
				return []Node{&Text{Text: "hi"}, &Text{Text: "there"}}, true
			}
		}
		return nil, false
	})
	para, ok := out.Content[0].(*Paragraph)
	if !ok || len(para.Content) != 3 {
		t.Fatalf("replacement not spliced: %+v", out.Content[0])
	}
	first, ok1 := para.Content[0].(*Text)
	second, ok2 := para.Content[1].(*Text)
	if !ok1 || !ok2 || first.Text != "hi" || second.Text != "there" {
		t.Errorf("replacement nodes wrong: %+v", para.Content)
	}
	// Input untouched (copy-on-write).
	if NodeText(NodeContent(doc.Content[0])[0]) != "hello" {
		t.Error("Transform mutated the input document")
	}
	// Untouched subtree shared, pruned subtree identical.
	if out.Content[1] != doc.Content[1] {
		t.Error("unchanged subtree was copied, want shared")
	}
	if out.Content[2] != doc.Content[2] {
		t.Error("pruned code block was copied, want shared")
	}
}

func TestTransform_DeleteNode(t *testing.T) {
	doc := sampleDoc()
	out := Transform(doc, func(n Node) ([]Node, bool) {
		if _, ok := n.(*Blockquote); ok {
			return nil, true
		}
		return nil, false
	})
	if len(out.Content) != 2 {
		t.Fatalf("blockquote not deleted: %d roots", len(out.Content))
	}
}

func TestTransform_NoChangeSharesInput(t *testing.T) {
	doc := sampleDoc()
	out := Transform(doc, func(Node) ([]Node, bool) { return nil, false })
	if len(out.Content) != len(doc.Content) {
		t.Fatal("content length changed")
	}
	for i := range out.Content {
		if out.Content[i] != doc.Content[i] {
			t.Errorf("root %d copied without changes", i)
		}
	}
}
