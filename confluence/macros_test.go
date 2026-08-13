package confluence

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// mdToADF and adfToMD run the two directions through the Confluence
// option bundles, the way a caller does.
func mdToADF(t *testing.T, md string) adf.Doc {
	t.Helper()
	opts := MarkdownOptions("https://x.atlassian.net")
	return adfast.ToADF(adfast.FromMarkdown(md, opts...), opts...)
}

func adfToMD(t *testing.T, doc adf.Doc) string {
	t.Helper()
	opts := RenderOptions()
	return adfast.ToMarkdown(adfast.FromADF(doc, opts...), opts...)
}

// docJSON is the document's ADF JSON, for shape assertions.
func docJSON(t *testing.T, doc adf.Doc) string {
	t.Helper()
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(js)
}

// TestMacroEncodeShape pins the ADF a sugared macro produces against the
// shape live Confluence pages carry: the core extensionType, the macro
// key, parameters split into macroParams and macroMetadata, and the
// per-key schemaVersion/title supplied from the spec table.
func TestMacroEncodeShape(t *testing.T) {
	cases := []struct {
		name string
		md   string
		want []string
	}{
		{
			name: "toc with a parameter",
			md:   "::toc{maxLevel=\"3\"}\n",
			want: []string{
				`"type":"extension"`,
				`"extensionType":"com.atlassian.confluence.macro.core"`,
				`"extensionKey":"toc"`,
				`"macroParams":{"maxLevel":{"value":"3"}}`,
				`"macroMetadata":{"schemaVersion":{"value":"1"},"title":"Table of Contents"}`,
			},
		},
		{
			name: "children carries its own schema version",
			md:   "::children\n",
			want: []string{
				`"extensionKey":"children"`,
				`"macroParams":{}`,
				`"macroMetadata":{"schemaVersion":{"value":"2"},"title":"Child pages"}`,
			},
		},
		{
			name: "the label is the unnamed parameter",
			md:   "::includePage[Hive Stand Design]\n",
			want: []string{
				`"extensionKey":"include"`,
				`"macroParams":{"":{"value":"Hive Stand Design"}}`,
				`"title":"Include Page"`,
			},
		},
		{
			name: "inline form",
			md:   "See :excerptInclude[Hive Notes] for more.\n",
			want: []string{
				`"type":"inlineExtension"`,
				`"extensionKey":"excerpt-include"`,
				`"macroParams":{"":{"value":"Hive Notes"}}`,
				`"title":"Insert excerpt"`,
			},
		},
		{
			name: "bodied form keeps its body",
			md:   ":::excerpt{name=\"summary\"}\nThe hive is healthy.\n:::\n",
			want: []string{
				`"type":"bodiedExtension"`,
				`"extensionKey":"excerpt"`,
				`"macroParams":{"name":{"value":"summary"}}`,
				`"content":[{"type":"paragraph"`,
				`"text":"The hive is healthy."`,
			},
		},
		{
			name: "a divergent schema version is written, not defaulted",
			md:   "::toc{schemaVersion=\"9\" title=\"Contents\"}\n",
			want: []string{
				`"macroMetadata":{"schemaVersion":{"value":"9"},"title":"Contents"}`,
				`"macroParams":{}`,
			},
		},
		{
			name: "layout and localId are node fields, not parameters",
			md:   "::toc{layout=\"wide\" localId=\"abc\"}\n",
			want: []string{
				`"macroParams":{}`,
				`"layout":"wide"`,
				`"localId":"abc"`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			js := docJSON(t, mdToADF(t, tc.md))
			for _, want := range tc.want {
				if !strings.Contains(js, want) {
					t.Errorf("missing %s in:\n%s", want, js)
				}
			}
			if strings.Contains(js, "macroId") {
				t.Errorf("macroId is server-generated and must not be written:\n%s", js)
			}
		})
	}
}

// TestMacroRoundTrip: every sugared form survives md → ADF → md
// unchanged, which is what keeps a pulled page stable across pushes.
func TestMacroRoundTrip(t *testing.T) {
	cases := []string{
		"::toc\n",
		"::toc{maxLevel=\"3\" minLevel=\"2\"}\n",
		"::children{all=\"true\" sort=\"title\"}\n",
		"::pagetree{root=\"Hive Notes\"}\n",
		"::includePage[Hive Stand Design]\n",
		"::excerptInclude[Hive Notes]{name=\"summary\"}\n",
		"::toc{layout=\"wide\" localId=\"abc-1\"}\n",
		"::toc{schemaVersion=\"9\"}\n",
		"::toc{title=\"Contents\"}\n",
		"Inline :pagetree{root=\"Notes\"} macro.\n",
		":::excerpt\nThe hive is healthy.\n:::\n",
		":::excerpt{name=\"summary\"}\nThe hive is healthy.\n:::\n",
	}
	for _, md := range cases {
		t.Run(strings.TrimSpace(md), func(t *testing.T) {
			if got := adfToMD(t, mdToADF(t, md)); got != md {
				t.Errorf("round trip = %q, want %q", got, md)
			}
		})
	}
}

// TestMacroDecodeDropsDerivedMetadata: a macro read back from Confluence
// carries a generated macroId and the constant schemaVersion/title. None
// of that is authored, so none of it reaches the markdown — a plain toc
// pulls as a bare ::toc.
func TestMacroDecodeDropsDerivedMetadata(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Extension{
		ExtensionType: MacroExtensionType,
		ExtensionKey:  "toc",
		Layout:        "default",
		LocalID:       "",
		Parameters: map[string]any{
			"macroParams": map[string]any{},
			"macroMetadata": map[string]any{
				"macroId":       map[string]any{"value": "b62cf278-5622-4306-aa6b-fe8bea688122"},
				"schemaVersion": map[string]any{"value": "1"},
				"title":         "Table of Contents",
			},
		},
	}}}

	if got, want := adfToMD(t, doc), "::toc\n"; got != want {
		t.Errorf("decoded = %q, want %q", got, want)
	}
}

// TestMacroDecodeKeepsDivergentMetadata: a schemaVersion or title that
// does NOT match the spec table is real information (a schema bump, a
// renamed macro), so it is written rather than silently rewritten.
func TestMacroDecodeKeepsDivergentMetadata(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Extension{
		ExtensionType: MacroExtensionType,
		ExtensionKey:  "toc",
		Parameters: map[string]any{
			"macroMetadata": map[string]any{
				"schemaVersion": map[string]any{"value": "4"},
				"title":         "Inhaltsverzeichnis",
			},
		},
	}}}

	want := "::toc{schemaVersion=\"4\" title=\"Inhaltsverzeichnis\"}\n"
	if got := adfToMD(t, doc); got != want {
		t.Errorf("decoded = %q, want %q", got, want)
	}
}

// TestMacroDegradesToGenericExtension: the sugar claims only what it can
// carry exactly. Everything else keeps the generic ::extension form with
// its parameters JSON intact — an unknown key, a nested parameter value,
// an unexpected metadata field, or a parameter named like one of the
// reserved attributes.
func TestMacroDegradesToGenericExtension(t *testing.T) {
	cases := []struct {
		name       string
		parameters any
		key        string
	}{
		{
			name: "unsugared macro key",
			key:  "jira",
			parameters: map[string]any{
				"macroParams": map[string]any{"key": map[string]any{"value": "ABC-1"}},
			},
		},
		{
			name: "non-string parameter value",
			key:  "toc",
			parameters: map[string]any{
				"macroParams": map[string]any{"nested": map[string]any{"value": map[string]any{"a": "b"}}},
			},
		},
		{
			name: "unexpected metadata field",
			key:  "toc",
			parameters: map[string]any{
				"macroMetadata": map[string]any{"placeholder": map[string]any{"value": "x"}},
			},
		},
		{
			name: "parameter named like a reserved attribute",
			key:  "toc",
			parameters: map[string]any{
				"macroParams": map[string]any{"title": map[string]any{"value": "x"}},
			},
		},
		{
			name: "parameter name outside the attribute grammar",
			key:  "toc",
			parameters: map[string]any{
				"macroParams": map[string]any{"a b": map[string]any{"value": "x"}},
			},
		},
		{
			name: "unknown parameters section",
			key:  "toc",
			parameters: map[string]any{
				"macroRenderedOutput": map[string]any{"value": "x"},
			},
		},
		{
			name: "empty unnamed parameter",
			key:  "include",
			parameters: map[string]any{
				"macroParams": map[string]any{"": map[string]any{"value": ""}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Extension{
				ExtensionType: MacroExtensionType,
				ExtensionKey:  tc.key,
				Parameters:    tc.parameters,
			}}}
			md := adfToMD(t, doc)
			if !strings.HasPrefix(md, "::extension{") {
				t.Fatalf("expected the generic form, got %q", md)
			}
			// The generic form is lossless: the parameters survive the
			// trip back to ADF.
			js := docJSON(t, mdToADF(t, md))
			if !strings.Contains(js, `"extensionKey":"`+tc.key+`"`) {
				t.Errorf("key lost: %s", js)
			}
		})
	}
}

// TestMacroInlineDegrades: the inline path declines the same shapes the
// block path does.
func TestMacroInlineDegrades(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Paragraph{
		Content: []adf.Node{&adf.InlineExtension{
			ExtensionType: MacroExtensionType,
			ExtensionKey:  "jira",
		}},
	}}}
	if md := adfToMD(t, doc); !strings.Contains(md, ":extension{") {
		t.Errorf("expected the generic inline form, got %q", md)
	}
}

// TestMacroForeignExtensionTypeUntouched: only core macros are sugared;
// a third-party extension with a colliding key keeps the generic form.
func TestMacroForeignExtensionTypeUntouched(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Extension{
		ExtensionType: "com.example.charts",
		ExtensionKey:  "toc",
	}}}
	md := adfToMD(t, doc)
	if !strings.Contains(md, `type="com.example.charts"`) {
		t.Errorf("foreign extension must keep the generic form, got %q", md)
	}
}

// TestMacroDecodeBodiedKeepsBodyAndLabel round-trips the bodied form
// from the ADF side, label included.
func TestMacroDecodeBodiedKeepsBodyAndLabel(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.BodiedExtension{
		ExtensionType: MacroExtensionType,
		ExtensionKey:  "excerpt",
		Parameters: map[string]any{
			"macroParams": map[string]any{
				"":     map[string]any{"value": "Summary"},
				"name": map[string]any{"value": "summary"},
			},
		},
		Content: []adf.Node{&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "The hive is healthy."}}}},
	}}}

	want := ":::excerpt[Summary]{name=\"summary\"}\nThe hive is healthy.\n:::\n"
	if got := adfToMD(t, doc); got != want {
		t.Errorf("decoded = %q, want %q", got, want)
	}
	// And back again, unchanged.
	if got := adfToMD(t, mdToADF(t, want)); got != want {
		t.Errorf("round trip = %q, want %q", got, want)
	}
}

// TestMacrosRegistrationIsValid guards the structural contract the
// registries check at install time.
func TestMacrosRegistrationIsValid(t *testing.T) {
	if err := Macros().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, spec := range macroSpecs {
		if macroNames[spec.key] != name {
			t.Errorf("reverse index for %q (key %q) = %q", name, spec.key, macroNames[spec.key])
		}
	}
}
