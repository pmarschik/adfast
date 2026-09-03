package adf_test

import (
	"encoding/json"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// The codec half of the link mark's title attribute. Link used to model
// the href alone, so "title" was an unmodeled attribute: it decoded into
// Extra, raised an unknown-attr diagnostic, and no typed field held it —
// which is why the conversion above this package had nowhere to put the
// title a markdown link spelled.
//
// The attribute is spelled "title" after the published JSON schema of
// the shared ADF schema (@atlaskit/adf-schema@57.3.0, link_mark:
// attrs.title, type string, with additionalProperties false so the name
// has to be exact) and the Jira Cloud ADF reference page for the mark.
//
// Each document below carries a TITLELESS link mark beside the titled
// one, so a change that wrote a title onto every mark fails here too.
const linkMarkDoc = `{
  "version": 1,
  "type": "doc",
  "content": [
    {
      "type": "paragraph",
      "content": [
        {
          "type": "text",
          "text": "spec",
          "marks": [{"type": "link", "attrs": {"href": "https://ex/s", "title": "The Spec"}}]
        },
        {
          "type": "text",
          "text": "guide",
          "marks": [{"type": "link", "attrs": {"href": "https://ex/g"}}]
        }
      ]
    }
  ]
}`

// The title decodes into the typed field, an absent one stays absent, and
// neither reports as an unmodeled attribute.
func TestLinkMarkDecodesItsTitleAttribute(t *testing.T) {
	_, doc, diags := decodeCollecting(t, linkMarkDoc)

	titled, bare := linkMarksOf(t, doc)
	if titled.Title == nil || *titled.Title != "The Spec" {
		t.Errorf("want Title %q, got %v", "The Spec", titled.Title)
	}
	if bare.Title != nil {
		t.Errorf("a mark with no title attribute must decode to a nil Title, got %q", *bare.Title)
	}
	for _, d := range diags {
		if d.Code == adf.CodeUnknownAttr {
			t.Errorf("a modeled attribute must not report as unknown: %s", d.Message)
		}
	}
}

// Decode → encode is lossless, and the titleless mark writes no title
// key: only an ABSENT attribute keeps a titleless link mark byte-identical
// to what every earlier version of this package produced.
func TestLinkMarkTitleReEncodesAsItCame(t *testing.T) {
	input, doc, _ := decodeCollecting(t, linkMarkDoc)
	assertLossless(t, input, doc)
}

// An empty-but-present title is a wire shape markdown cannot spell, so
// the conversion never produces one — but ADF that arrives with it has to
// come back out with it, which is why Title is a pointer rather than a
// string.
func TestLinkMarkKeepsAnEmptyButPresentTitle(t *testing.T) {
	const raw = `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"https://ex/s","title":""}}]}]}]}`

	input, doc, _ := decodeCollecting(t, raw)

	link := firstLinkMark(t, doc)
	if link.Title == nil {
		t.Fatal("an empty-but-present title must decode to a non-nil Title")
	}
	if *link.Title != "" {
		t.Errorf("want an empty Title, got %q", *link.Title)
	}
	assertLossless(t, input, doc)
}

// The encoder writes the title it is given and nothing when given none.
func TestLinkMarkEncodesItsTitleAttribute(t *testing.T) {
	for _, tt := range []struct {
		name string
		mark *adf.Link
		want string
	}{
		{
			"a title",
			&adf.Link{Href: new("https://ex/s"), Title: new("The Spec")},
			`{"attrs":{"href":"https://ex/s","title":"The Spec"},"type":"link"}`,
		},
		{
			"no title",
			&adf.Link{Href: new("https://ex/g")},
			`{"attrs":{"href":"https://ex/g"},"type":"link"}`,
		},
	} {
		out, err := json.Marshal(tt.mark)
		if err != nil {
			t.Fatalf("%s: marshal: %v", tt.name, err)
		}
		if string(out) != tt.want {
			t.Errorf("%s: encoded to %s, want %s", tt.name, out, tt.want)
		}
	}
}

// linkMarksOf answers the link marks of the two text nodes in
// linkMarkDoc, in order.
func linkMarksOf(t *testing.T, doc adf.Doc) (titled, bare *adf.Link) {
	t.Helper()
	para, ok := doc.Content[0].(*adf.Paragraph)
	if !ok || len(para.Content) != 2 {
		t.Fatalf("want a paragraph of two text nodes, got %#v", doc.Content[0])
	}
	return linkMarkOfText(t, para.Content[0]), linkMarkOfText(t, para.Content[1])
}

// firstLinkMark answers the link mark of the first text node of the first
// paragraph.
func firstLinkMark(t *testing.T, doc adf.Doc) *adf.Link {
	t.Helper()
	para, ok := doc.Content[0].(*adf.Paragraph)
	if !ok || len(para.Content) == 0 {
		t.Fatalf("want a non-empty paragraph, got %#v", doc.Content[0])
	}
	return linkMarkOfText(t, para.Content[0])
}

func linkMarkOfText(t *testing.T, n adf.Node) *adf.Link {
	t.Helper()
	text, ok := n.(*adf.Text)
	if !ok {
		t.Fatalf("want a text node, got %#v", n)
	}
	link, ok := adf.FindMark[*adf.Link](text.Marks)
	if !ok {
		t.Fatalf("want a link mark on %q, got %#v", text.Text, text.Marks)
	}
	return link
}
