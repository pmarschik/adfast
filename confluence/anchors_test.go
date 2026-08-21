package confluence

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// TestAnchorEncodeShape pins the ADF a "## Title {#id}" heading produces
// against the anchor macro measured on a live page: an inlineExtension
// inside the heading's content, and no synthetic anchor attribute left on
// the heading itself.
func TestAnchorEncodeShape(t *testing.T) {
	doc := mdToADF(t, "## Title {#my-anchor}\n")
	js := docJSON(t, doc)
	for _, want := range []string{
		`"type":"heading"`,
		`"type":"inlineExtension"`,
		`"extensionType":"com.atlassian.confluence.macro.core"`,
		`"extensionKey":"anchor"`,
		`"macroParams":{"":{"value":"my-anchor"}}`,
		`"macroMetadata":{"schemaVersion":{"value":"1"},"title":"Anchor"}`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("missing %s in\n%s", want, js)
		}
	}
	// The synthetic carrier must not reach the wire.
	if strings.Contains(js, `"anchor":"my-anchor"`) {
		t.Errorf("synthetic anchor attribute survived encoding:\n%s", js)
	}
	if !adf.IsWireSafe(doc) {
		t.Error("encoded document is not wire-safe")
	}
}

// TestAnchorRoundTrip is the acceptance criterion: the anchor survives
// md → ADF → md byte-identically through the Confluence bundles.
func TestAnchorRoundTrip(t *testing.T) {
	for _, md := range []string{
		"## Title {#my-anchor}\n",
		"# Top {#top}\n",
		"###### Deep {#d}\n",
		"## {#solo}\n",
		"## First {#a}\n\n## Second {#b}\n",
		// No anchor: nothing to lower, nothing to lift.
		"## Plain\n",
		// A literal {#…} in heading text must not become an anchor.
		`## Title \{#lit}` + "\n",
	} {
		t.Run(md, func(t *testing.T) {
			got := adfToMD(t, mdToADF(t, md))
			if got != md {
				t.Fatalf("round trip:\n got %q\nwant %q", got, md)
			}
		})
	}
}

// TestAnchorLiftDeclines covers the anchors that cannot become a "{#id}"
// suffix: they stay inline macros and decode to the :anchor[name]
// directive, which is the lossless fallback.
func TestAnchorLiftDeclines(t *testing.T) {
	anchor := func(name string) *adf.InlineExtension {
		return &adf.InlineExtension{
			ExtensionType: MacroExtensionType,
			ExtensionKey:  "anchor",
			Parameters:    macroParameters(anchorMacro, nil, name),
		}
	}
	cases := []struct {
		name    string
		want    string
		content []adf.Node
	}{
		{
			name:    "name outside the id charset",
			content: []adf.Node{&adf.Text{Text: "T "}, anchor("my anchor")},
			want:    "## T :anchor[my anchor]\n",
		},
		{
			name:    "two anchors on one heading",
			content: []adf.Node{&adf.Text{Text: "T "}, anchor("a"), anchor("b")},
			want:    "## T :anchor[a]:anchor[b]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				&adf.Heading{Level: 2, Content: tc.content},
			}}
			if got := adfToMD(t, doc); got != tc.want {
				t.Fatalf("render = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAnchorLiftDropsLegacyAnchorID pins the one parameter the lift drops
// rather than declining over: legacyAnchorId is derived from the page
// title, which the document does not carry and Confluence regenerates. It
// also pins the hand-authored shape — the separating space living inside
// the heading text, which the lift must absorb (see trimAnchorGap).
func TestAnchorLiftDropsLegacyAnchorID(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Heading{Level: 2, Content: []adf.Node{
			&adf.Text{Text: "Title "},
			&adf.InlineExtension{
				ExtensionType: MacroExtensionType,
				ExtensionKey:  "anchor",
				Parameters: map[string]any{
					"macroParams": map[string]any{
						"":               map[string]any{"value": "my-anchor"},
						"legacyAnchorId": map[string]any{"value": "Page Title-my-anchor"},
					},
					"macroMetadata": map[string]any{
						"macroId":       map[string]any{"value": "abc123"},
						"schemaVersion": map[string]any{"value": "1"},
						"title":         "Anchor",
					},
				},
			},
		}},
	}}
	const want = "## Title {#my-anchor}\n"
	if got := adfToMD(t, doc); got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

// TestStandaloneAnchorDirective covers an anchor with no heading to attach
// to: it round trips through the :anchor[name] macro directive.
func TestStandaloneAnchorDirective(t *testing.T) {
	const md = "See :anchor[here] below.\n"
	doc := mdToADF(t, md)
	js := docJSON(t, doc)
	if !strings.Contains(js, `"extensionKey":"anchor"`) {
		t.Errorf("no anchor macro in\n%s", js)
	}
	if got := adfToMD(t, doc); got != md {
		t.Fatalf("round trip:\n got %q\nwant %q", got, md)
	}
}
