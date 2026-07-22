package jira

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
)

func TestInferRichTextFormat(t *testing.T) {
	tests := []struct {
		name     string
		existing any
		want     RichTextFormat
	}{
		{name: "adf document map", existing: map[string]any{"type": "doc", "version": float64(1)}, want: RichTextADF},
		{name: "map without doc type", existing: map[string]any{"type": "paragraph"}, want: RichTextText},
		{name: "empty map", existing: map[string]any{}, want: RichTextText},
		{name: "plain string", existing: "already text", want: RichTextText},
		{name: "nil", existing: nil, want: RichTextText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferRichTextFormat(tt.existing); got != tt.want {
				t.Errorf("InferRichTextFormat(%v) = %q, want %q", tt.existing, got, tt.want)
			}
		})
	}
}

func TestEncodeRichText(t *testing.T) {
	t.Run("adf format converts markdown", func(t *testing.T) {
		got := EncodeRichText("hello **world**", RichTextADF)
		doc, ok := got.(adf.Doc)
		if !ok {
			t.Fatalf("EncodeRichText(adf) = %T, want adf.Doc", got)
		}
		if doc.Type != "doc" || len(doc.Content) == 0 {
			t.Errorf("EncodeRichText(adf) produced %+v, want a doc with content", doc)
		}
	})
	t.Run("text format trims trailing whitespace", func(t *testing.T) {
		if got := EncodeRichText("plain text\n\t ", RichTextText); got != "plain text" {
			t.Errorf("EncodeRichText(text) = %q, want %q", got, "plain text")
		}
	})
	t.Run("unknown format degrades to text", func(t *testing.T) {
		if got := EncodeRichText("body \n", RichTextFormat("wikimarkup")); got != "body" {
			t.Errorf("EncodeRichText(unknown) = %q, want %q", got, "body")
		}
	})
}
