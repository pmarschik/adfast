package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

func TestPipeline_ExtensionsRegisterBothDirections(t *testing.T) {
	// One WithExtensions registration must cover parse (md→ADF) AND decode
	// (ADF→md): a custom mark-backed kind round-trips only when both
	// halves are registered.
	reg := testInlineReg("hl2", func(mark adf.Mark, inner []ast.Node) (ast.Node, bool) {
		raw, ok := mark.(*adf.RawMark)
		if !ok || raw.Type != "highlight" {
			return nil, false
		}
		return &testInline{name: "hl2", children: inner}, true
	}, nil)
	p := NewPipeline(WithPipelineOptions(WithExtensions(reg)))

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{
			&adf.Text{Text: "lit", Marks: []adf.Mark{&adf.RawMark{Type: "highlight"}}},
		}},
	}}
	out := p.ADFToMarkdown(doc)
	if !strings.Contains(out, ":hl2[lit]") {
		t.Fatalf("decode direction not registered: %q", out)
	}
	// And the parse direction promotes the same name.
	doc2 := p.MarkdownToADF(out)
	if len(doc2.Content) == 0 {
		t.Fatal("parse direction produced empty document")
	}
}

func TestPipeline_SmartLinksBothDirections(t *testing.T) {
	sl := convert.SmartLinks{
		KeyFromURL: func(url string) (string, bool) {
			if key, ok := strings.CutPrefix(url, "https://issues.example/browse/"); ok {
				return key, true
			}
			return "", false
		},
		URLForKey: func(key string) (string, bool) {
			return "https://issues.example/browse/" + key, true
		},
	}
	p := NewPipeline(WithPipelineOptions(WithSmartLinks(sl)))

	// Encode: a link whose text equals the derived key becomes an
	// inlineCard.
	doc := p.MarkdownToADF("[AB-1](https://issues.example/browse/AB-1)\n")
	foundCard := false
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if _, ok := n.(*adf.InlineCard); ok {
				foundCard = true
			}
		}
	}
	if !foundCard {
		t.Errorf("smart links not applied on the parse side")
	}
	// Decode: the card labels with the short key.
	if out := p.ADFToMarkdown(doc); !strings.Contains(out, "[AB-1]") {
		t.Errorf("smart-link labels not applied on the render side: %q", out)
	}
}

func TestPipeline_DiagnosticsBothDirections(t *testing.T) {
	var diags []convert.Diagnostic
	p := NewPipeline(WithPipelineOptions(WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) })))

	// Encode-side diagnostic: an orphaned ::colwidths.
	p.MarkdownToADF("::colwidths[80]\n")
	if !hasCode(diags, convert.CodeColwidthsOrphan) {
		t.Errorf("encode-side diagnostics not wired: %v", diags)
	}
	// Decode-side diagnostic: an unknown ADF node.
	diags = nil
	p.ADFToMarkdown(adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.RawNode{Type: "mysteryBlock"},
	}})
	if !hasCode(diags, convert.CodeRawNode) {
		t.Errorf("decode-side diagnostics not wired: %v", diags)
	}
}

func TestPipeline_DirectionalOptions(t *testing.T) {
	p := NewPipeline(WithPipelineOptions(
		WithDocTransforms(func(d adf.Doc) adf.Doc {
			d.Content = append(d.Content, &adf.Rule{})
			return d
		}),
		WithNoWrap(),
	))
	doc := p.MarkdownToADF("hello\n")
	if _, ok := doc.Content[len(doc.Content)-1].(*adf.Rule); !ok {
		t.Error("encode-side option not applied")
	}
	long := strings.Repeat("word ", 40)
	doc2 := p.MarkdownToADF(long + "\n")
	// The unwrapped paragraph stays on one (long) first line; default
	// rendering would wrap it at 80 columns.
	if out := p.ADFToMarkdown(doc2); len(strings.SplitN(out, "\n", 2)[0]) < 100 {
		t.Errorf("render-side WithNoWrap not applied: %q", out)
	}
}

func TestPipeline_ZeroValueMatchesFreeFunctions(t *testing.T) {
	var p Pipeline
	md := "# Title\n\nsome **bold** text\n"
	if got, want := p.ADFToMarkdown(p.MarkdownToADF(md)), ToMarkdown(FromADF(ToADF(FromMarkdown(md)))); got != want {
		t.Errorf("zero-value pipeline diverges: %q vs %q", got, want)
	}
	if got, want := p.Format("a  b\n"), fmtMD("a  b\n"); got != want {
		t.Errorf("zero-value Format diverges: %q vs %q", got, want)
	}
}

func TestPipeline_MarkdownToADFAll(t *testing.T) {
	p := NewPipeline()
	docs, err := p.MarkdownToADFAll([]string{"one\n", "two\n"})
	if err != nil || len(docs) != 2 {
		t.Fatalf("MarkdownToADFAll = %d docs, err %v", len(docs), err)
	}
}

func TestPipeline_FormatWidth(t *testing.T) {
	p := NewPipeline()
	long := strings.Repeat("word ", 10)
	wrapped := p.Format(long+"\n", WithPrintWidth(20))
	if strings.Count(strings.TrimSpace(wrapped), "\n") == 0 {
		t.Errorf("format width not honored: %q", wrapped)
	}
}
