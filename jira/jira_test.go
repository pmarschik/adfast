package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// transformCase is one markdown → ADF expectation for the Jira
// transforms: the encoded document must contain every fragment in
// contains and none of the ones in notContain.
type transformCase struct {
	name       string
	baseURL    string
	mode       ExpandMode
	md         string
	contains   []string
	notContain []string
}

func TestTransforms(t *testing.T) {
	const base = "https://ixolit.atlassian.net"
	cases := []transformCase{
		{
			name:     "turns Jira issue links into inline cards",
			mode:     ExpandExplicit,
			md:       "See [INFRA-891](" + base + "/browse/INFRA-891).",
			contains: []string{`"type":"inlineCard"`},
		},
		{
			name:       "does not expand bare issue keys with explicit mode (default)",
			mode:       ExpandExplicit,
			md:         "See INFRA-123 for details.",
			contains:   []string{"INFRA-123"},
			notContain: []string{`"type":"inlineCard"`},
		},
		{
			name:     "expands bare issue keys with all mode",
			baseURL:  base,
			mode:     ExpandAll,
			md:       "See INFRA-123 for details.",
			contains: []string{`"type":"inlineCard"`, base + "/browse/INFRA-123"},
		},
		{
			name:       "does not expand issue keys in inline code",
			baseURL:    base,
			mode:       ExpandAll,
			md:         "Code: `INFRA-123` should not expand.",
			contains:   []string{`"type":"code"`},
			notContain: []string{`"type":"inlineCard"`},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			js := adfJSON(t, tt.md, tt.baseURL, tt.mode)
			for _, want := range tt.contains {
				if !strings.Contains(js, want) {
					t.Errorf("expected %s in %s", want, js)
				}
			}
			for _, unwanted := range tt.notContain {
				if strings.Contains(js, unwanted) {
					t.Errorf("did not expect %s in %s", unwanted, js)
				}
			}
		})
	}
}

// TestTransforms_PunctuationSurvivesExpansion: an expanded key becomes a
// card, and the punctuation that followed it in the source is still
// there on the way back to markdown.
func TestTransforms_PunctuationSurvivesExpansion(t *testing.T) {
	const base = "https://ixolit.atlassian.net"
	md := "Fixed in INFRA-123, INFRA-456."
	adfDoc := adfast.ToADF(adfast.FromMarkdown(md), MarkdownOptions(base, ExpandAll)...)
	if js := mustJSON(t, adfDoc); !strings.Contains(js, `"type":"inlineCard"`) {
		t.Errorf("expected inlineCard in %s", js)
	}
	if out := adfast.ToMarkdown(adfast.FromADF(adfDoc, RenderOptions()...), RenderOptions()...); !strings.Contains(out, ",") {
		t.Errorf("expected comma in %q", out)
	}
}

// adfJSON converts markdown to ADF through the Jira markdown options and
// returns the encoded document — what the transform assertions match on.
func adfJSON(t *testing.T, md, baseURL string, mode ExpandMode) string {
	t.Helper()
	return mustJSON(t, adfast.ToADF(adfast.FromMarkdown(md), MarkdownOptions(baseURL, mode)...))
}

// mustJSON encodes a document, failing the test when it cannot.
func mustJSON(t *testing.T, doc adf.Doc) string {
	t.Helper()
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(js)
}

func TestSmartLinks(t *testing.T) {
	sl := SmartLinks("https://x.atlassian.net")
	if key, ok := sl.KeyFromURL("https://x.atlassian.net/browse/ABC-12"); !ok || key != "ABC-12" {
		t.Errorf("KeyFromURL: %q %v", key, ok)
	}
	if _, ok := sl.KeyFromURL("https://example.com/page"); ok {
		t.Error("non-issue URL must not resolve")
	}
	if url, ok := sl.URLForKey("ABC-12"); !ok || url != "https://x.atlassian.net/browse/ABC-12" {
		t.Errorf("URLForKey: %q %v", url, ok)
	}
	if _, ok := sl.URLForKey("not a key"); ok {
		t.Error("non-key label must not expand")
	}
	if SmartLinks("").URLForKey != nil {
		t.Error("empty baseURL must disable key expansion")
	}
}

func TestRenderOptions_CardLabels(t *testing.T) {
	url := "https://x.atlassian.net/browse/ABC-12"
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Paragraph{
		Content: []adf.Node{&adf.InlineCard{URL: &url}},
	}}}
	md := adfast.ToMarkdown(adfast.FromADF(doc, RenderOptions()...), RenderOptions()...)
	if !strings.Contains(md, "[ABC-12](https://x.atlassian.net/browse/ABC-12)") {
		t.Errorf("card label: %q", md)
	}
}

func TestCodeLanguages_WiredIntoMarkdownOptions(t *testing.T) {
	var diags []convert.Diagnostic
	opts := append(MarkdownOptions("", ExpandExplicit),
		adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

	// Aliases and case-insensitive matches are supported; unknown tags report.
	adfast.ToADF(adfast.FromMarkdown("```TypeScript\nx\n```\n\n```dockerfile\ny\n```\n\n```c++\nz\n```\n", opts...), opts...)
	if len(diags) != 0 {
		t.Fatalf("supported languages must not report: %+v", diags)
	}
	adfast.ToADF(adfast.FromMarkdown("```brainfuck\nx\n```\n", opts...), opts...)
	if len(diags) != 1 || diags[0].Code != convert.CodeUnsupportedCodeLanguage {
		t.Fatalf("expected one unsupported-code-language diagnostic: %+v", diags)
	}
}

func TestUnsupportedKinds_WiredIntoMarkdownOptions(t *testing.T) {
	var diags []convert.Diagnostic
	opts := append(MarkdownOptions("", ExpandExplicit),
		adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

	// A placeholder (render-confirmed dropped by Jira) targeted at Jira
	// reports once, naming the kind and the product.
	adfast.ToADF(adfast.FromMarkdown(":placeholder[type here…]\n", opts...), opts...)
	if len(diags) != 1 ||
		diags[0].Code != convert.CodeUnsupportedInProduct ||
		diags[0].Message != "placeholder is not available in jira" {
		t.Fatalf("expected one unsupported-in-product diagnostic naming jira: %+v", diags)
	}

	// A Jira-supported document reports nothing.
	diags = nil
	adfast.ToADF(adfast.FromMarkdown("# Heading\n\nplain paragraph\n", opts...), opts...)
	if len(diags) != 0 {
		t.Fatalf("supported-only document must not report: %+v", diags)
	}

	// The Jira set is render-confirmed non-support only (see
	// UnsupportedKinds): placeholder is dropped by the render, and
	// multiBodiedExtension/extensionFrame are rejected by the Jira REST
	// endpoint (INVALID_INPUT). fontSize is excluded because adfast
	// retires it (never produced). Docs-by-omission kinds Jira actually
	// renders (layoutSection, extension, alignment, …) are deliberately
	// excluded to avoid false positives.
	want := map[string]bool{"placeholder": true, "multiBodiedExtension": true, "extensionFrame": true}
	if len(UnsupportedKinds) != len(want) {
		t.Errorf("Jira UnsupportedKinds = %v, want %d kinds", UnsupportedKinds, len(want))
	}
	for _, k := range UnsupportedKinds {
		if !want[k] {
			t.Errorf("unexpected kind %q in Jira UnsupportedKinds = %v", k, UnsupportedKinds)
		}
	}
}

// TestHeadingAnchors_DroppedForJira pins the Jira half of the heading
// anchor pair: Jira has no anchor construct, so a "## Title {#id}" suffix
// cannot be lowered — the anchor drops with a diagnostic naming it, the
// heading text survives, and the document is wire-safe.
func TestHeadingAnchors_DroppedForJira(t *testing.T) {
	var diags []convert.Diagnostic
	opts := append(MarkdownOptions("", ExpandExplicit),
		adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

	doc := adfast.ToADF(adfast.FromMarkdown("## Title {#my-anchor}\n\n### Other {#second}\n", opts...), opts...)
	if len(diags) != 2 ||
		diags[0].Code != convert.CodeHeadingAnchorDropped ||
		diags[0].Message != "heading anchor {#my-anchor} is not available in jira" ||
		diags[1].Message != "heading anchor {#second} is not available in jira" {
		t.Fatalf("expected one heading-anchor-dropped diagnostic per anchor: %+v", diags)
	}
	if !adf.IsWireSafe(doc) {
		t.Error("document with dropped anchors is not wire-safe")
	}
	// The heading text is untouched; only the anchor is gone.
	md := adfast.ToMarkdown(adfast.FromADF(doc, RenderOptions()...), RenderOptions()...)
	if md != "## Title\n\n### Other\n" {
		t.Errorf("heading text after drop: %q", md)
	}

	// A document with no anchors reports nothing.
	diags = nil
	adfast.ToADF(adfast.FromMarkdown("## Plain\n", opts...), opts...)
	if len(diags) != 0 {
		t.Fatalf("anchor-free document must not report: %+v", diags)
	}
}
