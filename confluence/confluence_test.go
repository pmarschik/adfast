package confluence

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

func TestSmartLinks_KeyFromURL(t *testing.T) {
	sl := SmartLinks("https://x.atlassian.net")

	cases := []struct {
		name string
		url  string
		key  string
		ok   bool
	}{
		{"page URL with title slug", "https://x.atlassian.net/wiki/spaces/DOCS/pages/123456789/Stand+Design", "DOCS/123456789", true},
		{"page URL without title slug", "https://x.atlassian.net/wiki/spaces/DOCS/pages/123456789", "DOCS/123456789", true},
		{"personal space", "https://x.atlassian.net/wiki/spaces/~712020aa11/pages/42/Notes", "~712020aa11/42", true},
		{"query string after id", "https://x.atlassian.net/wiki/spaces/DOCS/pages/99?focusedCommentId=1", "DOCS/99", true},
		{"non-page wiki URL", "https://x.atlassian.net/wiki/spaces/DOCS/overview", "", false},
		{"jira browse URL", "https://x.atlassian.net/browse/ABC-12", "", false},
		{"unrelated URL", "https://example.com/wiki/page", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := sl.KeyFromURL(tc.url)
			if ok != tc.ok || key != tc.key {
				t.Errorf("KeyFromURL(%q) = %q, %v; want %q, %v", tc.url, key, ok, tc.key, tc.ok)
			}
		})
	}
}

func TestSmartLinks_URLForKey(t *testing.T) {
	sl := SmartLinks("https://x.atlassian.net/")

	cases := []struct {
		name string
		key  string
		url  string
		ok   bool
	}{
		{"space and page id", "DOCS/123456789", "https://x.atlassian.net/wiki/spaces/DOCS/pages/123456789", true},
		{"personal space key", "~712020aa11/42", "https://x.atlassian.net/wiki/spaces/~712020aa11/pages/42", true},
		{"personal space key with colon", "~712020:aa11/42", "https://x.atlassian.net/wiki/spaces/~712020:aa11/pages/42", true},
		{"jira issue key", "ABC-12", "", false},
		{"page id without space", "123456789", "", false},
		{"prose with a slash", "either/or 24", "", false},
		{"missing page id", "DOCS/", "", false},
		{"non-numeric page id", "DOCS/current", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, ok := sl.URLForKey(tc.key)
			if ok != tc.ok || url != tc.url {
				t.Errorf("URLForKey(%q) = %q, %v; want %q, %v", tc.key, url, ok, tc.url, tc.ok)
			}
		})
	}

	if SmartLinks("").URLForKey != nil {
		t.Error("empty baseURL must disable key expansion")
	}
}

func TestMarkdownOptions_PageLinksBecomeInlineCards(t *testing.T) {
	url := "https://x.atlassian.net/wiki/spaces/DOCS/pages/123456789/Stand+Design"
	md := "See [DOCS/123456789](" + url + ")."
	doc := adfast.ToADF(adfast.FromMarkdown(md), MarkdownOptions("https://x.atlassian.net")...)
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"type":"inlineCard"`) {
		t.Errorf("expected inlineCard in %s", js)
	}

	// A link whose text is NOT the derived key stays a plain link.
	doc = adfast.ToADF(adfast.FromMarkdown("See [the stand design]("+url+")."), MarkdownOptions("https://x.atlassian.net")...)
	js, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), `"type":"inlineCard"`) {
		t.Errorf("title-labeled link must stay a link: %s", js)
	}
}

func TestMarkdownOptions_LinkCardKeyExpansion(t *testing.T) {
	doc := adfast.ToADF(adfast.FromMarkdown("::linkCard[DOCS/123456789]\n"), MarkdownOptions("https://x.atlassian.net")...)
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "https://x.atlassian.net/wiki/spaces/DOCS/pages/123456789") {
		t.Errorf("expected expanded page URL in %s", js)
	}
	md := adfast.ToMarkdown(adfast.FromADF(doc, RenderOptions()...), RenderOptions()...)
	if !strings.Contains(md, "::linkCard[DOCS/123456789]") {
		t.Errorf("expected key label back, got %q", md)
	}
}

func TestRenderOptions_CardLabels(t *testing.T) {
	url := "https://x.atlassian.net/wiki/spaces/DOCS/pages/123456789/Stand+Design"
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Paragraph{
		Content: []adf.Node{&adf.InlineCard{URL: &url}},
	}}}
	md := adfast.ToMarkdown(adfast.FromADF(doc, RenderOptions()...), RenderOptions()...)
	if !strings.Contains(md, "[DOCS/123456789]("+url+")") {
		t.Errorf("card label: %q", md)
	}
}

func TestCodeLanguages_WiredIntoMarkdownOptions(t *testing.T) {
	var diags []convert.Diagnostic
	opts := append(MarkdownOptions(""),
		adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

	// Storage values, display aliases, and case-insensitive matches pass.
	adfast.ToADF(adfast.FromMarkdown("```JavaScript\nx\n```\n\n```erl\ny\n```\n\n```c++\nz\n```\n", opts...), opts...)
	if len(diags) != 0 {
		t.Fatalf("supported languages must not report: %+v", diags)
	}
	// Go is in Jira's editor set but NOT in the Confluence macro set.
	adfast.ToADF(adfast.FromMarkdown("```go\nx\n```\n", opts...), opts...)
	if len(diags) != 1 || diags[0].Code != convert.CodeUnsupportedCodeLanguage {
		t.Fatalf("expected one unsupported-code-language diagnostic: %+v", diags)
	}
}

func TestUnsupportedKinds_ConfluenceProfile(t *testing.T) {
	var diags []convert.Diagnostic
	opts := append(MarkdownOptions(""),
		adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))

	// extension is a Confluence-supported construct (confirmed by the
	// live round-trip probe), so it must NOT be flagged under the
	// Confluence profile even though Jira rejects it.
	adfast.ToADF(adfast.FromMarkdown("::extension{key=\"chart\" type=\"com.example.charts\"}\n", opts...), opts...)
	for _, d := range diags {
		if d.Code == convert.CodeUnsupportedInProduct {
			t.Fatalf("Confluence profile must not flag extension: %+v", diags)
		}
	}
	// The set holds the round-trip-confirmed kind Confluence does not
	// preserve: blockTaskItem (downgraded to taskItem). fontSize is also
	// stripped by Confluence, but adfast retires it (never produced), so
	// it is not flagged as unsupported-in-product.
	want := map[string]bool{"blockTaskItem": true}
	if len(UnsupportedKinds) != len(want) {
		t.Errorf("Confluence UnsupportedKinds = %v, want %d kinds", UnsupportedKinds, len(want))
	}
	for _, k := range UnsupportedKinds {
		if !want[k] {
			t.Errorf("unexpected kind %q in Confluence UnsupportedKinds = %v", k, UnsupportedKinds)
		}
	}
}

// TestUnsupportedKinds_FontSizeRetired proves fontSize is retired, not
// flagged as unsupported-in-product: the directive drops to plain text
// before any ADF is produced, so it emits a fontsize-dropped diagnostic
// (from the core) and never an unsupported-in-product one.
func TestUnsupportedKinds_FontSizeRetired(t *testing.T) {
	var codes []string
	opts := append(MarkdownOptions(""), adfast.WithDiagnostics(func(d convert.Diagnostic) {
		codes = append(codes, d.Code)
	}))
	adfast.ToADF(adfast.FromMarkdown(":fontSize[small type]{small}\n", opts...), opts...)
	dropped := false
	for _, c := range codes {
		if c == convert.CodeUnsupportedInProduct {
			t.Errorf("fontSize is retired; must not be flagged unsupported-in-product; got %v", codes)
		}
		if c == convert.CodeFontSizeDropped {
			dropped = true
		}
	}
	if !dropped {
		t.Errorf("expected %q for fontSize; got %v", convert.CodeFontSizeDropped, codes)
	}
}
