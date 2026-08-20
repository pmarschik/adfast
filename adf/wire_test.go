package adf

import "testing"

func wireDoc(content ...Node) Doc {
	return Doc{Type: "doc", Version: 1, Content: content}
}

func TestIsWireSafe(t *testing.T) {
	tests := []struct {
		name string
		doc  Doc
		want bool
	}{
		{
			name: "canonical doc",
			doc: wireDoc(&Paragraph{Content: []Node{
				&Text{Text: "plain", Marks: []Mark{&Strong{}, &Link{Href: new("https://x")}}},
			}}),
			want: true,
		},
		{
			name: "colwidths hint",
			doc:  wireDoc(&ColwidthsHint{Widths: []float64{80}}),
			want: false,
		},
		{
			name: "tight bullet list flag",
			doc:  wireDoc(&BulletList{Tight: new(true), Content: []Node{&ListItem{}}}),
			want: false,
		},
		{
			name: "tight ordered list flag nested",
			doc: wireDoc(&Blockquote{Content: []Node{
				&OrderedList{Tight: new(false), Content: []Node{&ListItem{}}},
			}}),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWireSafe(tt.doc); got != tt.want {
				t.Errorf("IsWireSafe = %v, want %v", got, tt.want)
			}
			// StripSynthetic always yields a wire-safe document.
			if !IsWireSafe(StripSynthetic(tt.doc)) {
				t.Error("StripSynthetic result is not wire-safe")
			}
		})
	}
}

// stripFixture builds the shared StripSynthetic test document.
func stripFixture() Doc {
	return wireDoc(
		&ColwidthsHint{Widths: []float64{80, 120}},
		&Paragraph{Content: []Node{
			&Text{Text: "keep", Marks: []Mark{&Strong{}, &Link{Href: new("https://x")}}},
		}},
		&BulletList{Tight: new(true), Content: []Node{
			&ListItem{Content: []Node{&Paragraph{}}},
		}},
	)
}

func TestStripSynthetic(t *testing.T) {
	doc := stripFixture()
	out := StripSynthetic(doc)

	if len(out.Content) != 2 {
		t.Fatalf("colwidths hint not dropped: %d roots", len(out.Content))
	}
	para, ok := out.Content[0].(*Paragraph)
	if !ok || len(para.Content) != 1 {
		t.Fatalf("paragraph content lost: %+v", out.Content[0])
	}
	text, textOK := para.Content[0].(*Text)
	if !textOK || text.Text != "keep" || len(text.Marks) != 2 {
		t.Fatalf("content or marks lost: %+v", para.Content[0])
	}
	list, listOK := out.Content[1].(*BulletList)
	if !listOK || list.Tight != nil {
		t.Fatalf("tight flag not cleared: %+v", out.Content[1])
	}
	if item, itemOK := list.Content[0].(*ListItem); !itemOK || len(item.Content) != 1 {
		t.Errorf("item content lost: %+v", list.Content[0])
	}
}

func TestStripSynthetic_InputUntouched(t *testing.T) {
	doc := stripFixture()
	_ = StripSynthetic(doc)

	list, ok := doc.Content[2].(*BulletList)
	if !ok || list.Tight == nil || !*list.Tight {
		t.Error("StripSynthetic mutated the input")
	}
	if len(doc.Content) != 3 {
		t.Error("StripSynthetic mutated the input content list")
	}
}
