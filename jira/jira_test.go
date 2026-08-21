package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

func TestTransforms(t *testing.T) {
	t.Run("turns Jira issue links into inline cards", func(t *testing.T) {
		adfDoc := adfast.ToADF(adfast.FromMarkdown("See [INFRA-891](https://ixolit.atlassian.net/browse/INFRA-891)."), MarkdownOptions("", "explicit")...)
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if !strings.Contains(string(js), `"type":"inlineCard"`) {
			t.Errorf("expected inlineCard in %s", js)
		}
	})

	t.Run("does not expand bare issue keys with explicit mode (default)", func(t *testing.T) {
		adfDoc := adfast.ToADF(adfast.FromMarkdown("See INFRA-123 for details."), MarkdownOptions("", "explicit")...)
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if strings.Contains(string(js), `"type":"inlineCard"`) {
			t.Errorf("should not have inlineCard in %s", js)
		}
		if !strings.Contains(string(js), "INFRA-123") {
			t.Errorf("expected bare text preserved in %s", js)
		}
	})

	t.Run("expands bare issue keys with all mode", func(t *testing.T) {
		adfDoc := adfast.ToADF(adfast.FromMarkdown("See INFRA-123 for details."), MarkdownOptions("https://ixolit.atlassian.net", "all")...)
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if !strings.Contains(string(js), `"type":"inlineCard"`) {
			t.Errorf("expected inlineCard in %s", js)
		}
		if !strings.Contains(string(js), "https://ixolit.atlassian.net/browse/INFRA-123") {
			t.Errorf("expected Jira URL in %s", js)
		}
	})

	t.Run("preserves punctuation after expanded issue keys", func(t *testing.T) {
		adfDoc := adfast.ToADF(adfast.FromMarkdown("Fixed in INFRA-123, INFRA-456."), MarkdownOptions("https://ixolit.atlassian.net", "all")...)
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if !strings.Contains(string(js), `"type":"inlineCard"`) {
			t.Errorf("expected inlineCard in %s", js)
		}
		md := adfast.ToMarkdown(adfast.FromADF(adfDoc, RenderOptions()...), RenderOptions()...)
		if !strings.Contains(md, ",") {
			t.Errorf("expected comma in %q", md)
		}
	})

	t.Run("does not expand issue keys in inline code", func(t *testing.T) {
		adfDoc := adfast.ToADF(adfast.FromMarkdown("Code: `INFRA-123` should not expand."), MarkdownOptions("https://ixolit.atlassian.net", "all")...)
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if !strings.Contains(string(js), `"type":"code"`) {
			t.Errorf("expected code mark in %s", js)
		}
		count := strings.Count(string(js), `"type":"inlineCard"`)
		if count != 0 {
			t.Errorf("expected no inlineCard in code, got %d: %s", count, js)
		}
	})
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
