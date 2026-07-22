package debug_test

import (
	"bytes"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/debug"
)

func sampleTree() ast.Node {
	checked := true
	return &ast.Root{Children: []ast.Node{
		&ast.Heading{Depth: 2, Children: []ast.Node{&ast.Text{Value: "Title"}}},
		&ast.Paragraph{Children: []ast.Node{
			&ast.Text{Value: "see "},
			&ast.Link{URL: "https://x.test", Bare: true, Children: []ast.Node{&ast.Text{Value: "https://x.test"}}},
		}},
		&ast.List{Children: []ast.Node{
			&ast.ListItem{Checked: &checked, Children: []ast.Node{
				&ast.Paragraph{Children: []ast.Node{&ast.TextDirective{
					Name:  "status",
					Attrs: map[string]string{"color": "green", "style": "bold"},
				}}},
			}},
		}},
	}}
}

func TestDump(t *testing.T) {
	var buf bytes.Buffer
	debug.Dump(&buf, sampleTree())
	want := `root
  heading depth=2
    text value="Title"
  paragraph
    text value="see "
    link url="https://x.test" bare=true
      text value="https://x.test"
  list
    listItem checked=true
      paragraph
        textDirective name="status" attrs={color="green" style="bold"}
`
	if got := buf.String(); got != want {
		t.Errorf("Dump mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpNil(t *testing.T) {
	var buf bytes.Buffer
	debug.Dump(&buf, nil)
	if got := buf.String(); got != "nil\n" {
		t.Errorf("Dump(nil) = %q, want %q", got, "nil\n")
	}
}

func TestMarshalJSON(t *testing.T) {
	n := &ast.Paragraph{Children: []ast.Node{
		&ast.Text{Value: "hi "},
		&ast.Strong{Children: []ast.Node{&ast.Text{Value: "x"}}},
	}}
	got, err := debug.MarshalJSON(n)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	want := `{"kind":"paragraph","children":[{"kind":"text","value":"hi "},{"kind":"strong","children":[{"kind":"text","value":"x"}]}]}`
	if string(got) != want {
		t.Errorf("MarshalJSON:\n got  %s\n want %s", got, want)
	}
}

func TestDumpADF(t *testing.T) {
	href := "https://x.test"
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{
			&adf.Text{Text: "hi", Marks: []adf.Mark{
				&adf.Strong{},
				&adf.Link{Href: &href},
			}},
		}},
		&adf.Heading{Level: 2, Content: []adf.Node{
			&adf.Text{Text: "Title"},
		}},
	}}
	var buf bytes.Buffer
	debug.DumpADF(&buf, doc)
	want := `doc version=1
  paragraph
    text text="hi" marks=[strong link{href=https://x.test}]
  heading attrs={level=2}
    text text="Title"
`
	if got := buf.String(); got != want {
		t.Errorf("DumpADF mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}
