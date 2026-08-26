package confluence

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// legacyExt builds the wrapper the way the wire does: the parameters
// blob goes through json.Unmarshal, so numbers are float64 and every
// nested value is a map[string]any, exactly as adf's decoder leaves it.
func legacyExt(t *testing.T, parametersJSON string) *adf.Extension {
	t.Helper()
	var params map[string]any
	if err := json.Unmarshal([]byte(parametersJSON), &params); err != nil {
		t.Fatal(err)
	}
	return &adf.Extension{
		ExtensionType: legacyContentType,
		ExtensionKey:  legacyContentKey,
		Parameters:    params,
	}
}

// TestExpandLegacyContentBlockquoteInListItem is the bead's regression
// fixture: a blockquote Confluence rewrote into a legacy-content wrapper
// because listItem's content model cannot hold one. The expansion must
// give back exactly the document a direct submission of the same
// Markdown would have produced — the acceptance criterion for the read
// to settle a comparison against a push.
func TestExpandLegacyContentBlockquoteInListItem(t *testing.T) {
	md := "- Item text\n\n  > Quoted text\n"
	want := adfToMD(t, mdToADF(t, md))

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.BulletList{Content: []adf.Node{
			&adf.ListItem{Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "Item text"}}},
				legacyExt(t, `{
					"cxhtml": "<the original storage format>",
					"nestedContent": {"type": "doc", "version": 1, "content": [
						{"type": "blockquote", "content": [
							{"type": "paragraph", "content": [
								{"type": "text", "text": "Quoted text"}
							]}
						]}
					]}
				}`),
			}},
		}},
	}}

	got := adfToMD(t, ExpandLegacyContent(doc))
	if got != want {
		t.Fatalf("expand = %q, want %q", got, want)
	}
}

// TestExpandLegacyContentWrapperAtTopLevel proves the expansion is
// placement-agnostic: nothing about it depends on the wrapper sitting
// inside a listItem. This placement was not itself observed on the
// probed page (there the wrapper sits inside the listItem, replacing
// only the blockquote — see TestExpandLegacyContentBlockquoteInListItem)
// but ExpandLegacyContent operates on adf.Transform, which visits every
// node alike regardless of position, so the contract holds here too.
func TestExpandLegacyContentWrapperAtTopLevel(t *testing.T) {
	md := "- Item text\n\n  > Quoted text\n"
	want := adfToMD(t, mdToADF(t, md))

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		legacyExt(t, `{
			"cxhtml": "<the original storage format>",
			"nestedContent": {"type": "doc", "version": 1, "content": [
				{"type": "bulletList", "content": [
					{"type": "listItem", "content": [
						{"type": "paragraph", "content": [
							{"type": "text", "text": "Item text"}
						]},
						{"type": "blockquote", "content": [
							{"type": "paragraph", "content": [
								{"type": "text", "text": "Quoted text"}
							]}
						]}
					]}
				]}
			]}
		}`),
	}}

	got := adfToMD(t, ExpandLegacyContent(doc))
	if got != want {
		t.Fatalf("expand = %q, want %q", got, want)
	}
}

// TestExpandLegacyContentTableInListItem is the second observed corpus
// case: a table Confluence rewrote into the wrapper for the same
// content-model reason as the blockquote case.
func TestExpandLegacyContentTableInListItem(t *testing.T) {
	md := "- Item text\n\n  | A | B |\n  | - | - |\n  | 1 | 2 |\n"
	want := adfToMD(t, mdToADF(t, md))

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.BulletList{Content: []adf.Node{
			&adf.ListItem{Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "Item text"}}},
				legacyExt(t, `{
					"cxhtml": "<the original storage format>",
					"nestedContent": {"type": "doc", "version": 1, "content": [
						{"type": "table", "content": [
							{"type": "tableRow", "content": [
								{"type": "tableHeader", "content": [
									{"type": "paragraph", "content": [{"type": "text", "text": "A"}]}
								]},
								{"type": "tableHeader", "content": [
									{"type": "paragraph", "content": [{"type": "text", "text": "B"}]}
								]}
							]},
							{"type": "tableRow", "content": [
								{"type": "tableCell", "content": [
									{"type": "paragraph", "content": [{"type": "text", "text": "1"}]}
								]},
								{"type": "tableCell", "content": [
									{"type": "paragraph", "content": [{"type": "text", "text": "2"}]}
								]}
							]}
						]}
					]}
				}`),
			}},
		}},
	}}

	got := adfToMD(t, ExpandLegacyContent(doc))
	if got != want {
		t.Fatalf("expand = %q, want %q", got, want)
	}
}

// TestExpandLegacyContentNestedString pins the defensive branch of
// nestedContentDoc: the probed page carried nestedContent as a JSON
// object, not as an escaped string, so this form is unmeasured on the
// wire. It is accepted anyway because cxhtml, the sibling parameter,
// does carry escaped JSON, so the same shape for nestedContent on some
// other page is plausible.
func TestExpandLegacyContentNestedString(t *testing.T) {
	md := "- Item text\n\n  > Quoted text\n"
	want := adfToMD(t, mdToADF(t, md))

	nested := `{"type":"doc","version":1,"content":[` +
		`{"type":"blockquote","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Quoted text"}]}` +
		`]}]}`
	encoded, err := json.Marshal(nested)
	if err != nil {
		t.Fatal(err)
	}

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.BulletList{Content: []adf.Node{
			&adf.ListItem{Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "Item text"}}},
				legacyExt(t, `{"cxhtml": "<the original storage format>", "nestedContent": `+string(encoded)+`}`),
			}},
		}},
	}}

	got := adfToMD(t, ExpandLegacyContent(doc))
	if got != want {
		t.Fatalf("expand = %q, want %q", got, want)
	}
}

// TestExpandLegacyContentDeclines covers every payload shape that
// leaves the extension exactly where it was: the fallback the pass
// promises for anything it cannot supply. Each case must leave the
// document structurally unchanged and still decode through the generic
// ::extension directive.
func TestExpandLegacyContentDeclines(t *testing.T) {
	cases := []struct {
		parameters any
		name       string
	}{
		{name: "parameters absent", parameters: nil},
		{name: "parameters not an object", parameters: "a string, not a map"},
		{name: "nestedContent absent, only cxhtml", parameters: mustParams(t, `{"cxhtml": "<html/>"}`)},
		{name: "nestedContent neither object nor string", parameters: mustParams(t, `{"nestedContent": 42}`)},
		{name: "nestedContent string not valid JSON", parameters: mustParams(t, `{"nestedContent": "not json"}`)},
		{name: "nestedContent type is paragraph", parameters: mustParams(t, `{"nestedContent": {"type": "paragraph", "version": 1, "content": []}}`)},
		{name: "nestedContent version 2", parameters: mustParams(t, `{"nestedContent": {"type": "doc", "version": 2, "content": [{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}}`)},
		{name: "nestedContent version absent", parameters: mustParams(t, `{"nestedContent": {"type": "doc", "content": [{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}}`)},
		{name: "nestedContent content empty", parameters: mustParams(t, `{"nestedContent": {"type": "doc", "version": 1, "content": []}}`)},
		{name: "nestedContent content absent", parameters: mustParams(t, `{"nestedContent": {"type": "doc", "version": 1}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := &adf.Extension{ExtensionType: legacyContentType, ExtensionKey: legacyContentKey}
			if tc.parameters != nil {
				ext.Parameters = tc.parameters
			}
			doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{ext}}

			before := docJSON(t, doc)
			got := ExpandLegacyContent(doc)
			after := docJSON(t, got)
			if before != after {
				t.Fatalf("document changed:\n before %s\n after  %s", before, after)
			}

			md := adfToMD(t, got)
			if !strings.Contains(md, "::extension") {
				t.Fatalf("expected the generic ::extension fallback, got %q", md)
			}
		})
	}
}

// mustParams unmarshals a parameters JSON literal the way the wire
// would deliver it.
func mustParams(t *testing.T, js string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestExpandLegacyContentLeavesOtherExtensions is the no-regression
// check: anything that is not the legacy-content wrapper must pass
// through byte-identically, including an extension that carries a
// nestedContent parameter but the wrong key or the wrong type.
func TestExpandLegacyContentLeavesOtherExtensions(t *testing.T) {
	cases := []struct {
		name string
		doc  adf.Doc
	}{
		{
			name: "sugared macro",
			doc: adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				mdToADFExtensionDoc(t, `::toc{maxLevel="3"}`+"\n"),
			}},
		},
		{
			name: "generic unsugared extension with arbitrary parameters",
			doc: adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				&adf.Extension{
					ExtensionType: "com.example.charts",
					ExtensionKey:  "chart",
					Parameters:    mustParams(t, `{"series": [1, 2, 3]}`),
				},
			}},
		},
		{
			name: "nestedContent parameter but a different extensionKey",
			doc: adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				&adf.Extension{
					ExtensionType: legacyContentType,
					ExtensionKey:  "not-legacy-content",
					Parameters: mustParams(t, `{"nestedContent": {"type": "doc", "version": 1,
						"content": [{"type": "paragraph", "content": [{"type": "text", "text": "x"}]}]}}`),
				},
			}},
		},
		{
			name: "legacy-content key but a different extensionType",
			doc: adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				&adf.Extension{
					ExtensionType: "com.example.other",
					ExtensionKey:  legacyContentKey,
					Parameters: mustParams(t, `{"nestedContent": {"type": "doc", "version": 1,
						"content": [{"type": "paragraph", "content": [{"type": "text", "text": "x"}]}]}}`),
				},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Compare the raw ADF JSON directly, NOT via adfToMD: that
			// helper renders through RenderOptions, which itself installs
			// ExpandLegacyContent, so a "before" taken from it would already
			// carry the pass's own effect and could no longer tell a
			// correct decline apart from a buggy expansion of one of these
			// near-matches — the same self-reference the recursion-cap test
			// below had to route around.
			before := docJSON(t, tc.doc)
			got := ExpandLegacyContent(tc.doc)
			after := docJSON(t, got)
			if before != after {
				t.Fatalf("document changed:\n before %s\n after  %s", before, after)
			}
		})
	}
}

// mdToADFExtensionDoc extracts the single top-level extension node a
// sugared macro directive encodes to, for building a case's document
// directly rather than through the full mdToADF/adfToMD round trip.
func mdToADFExtensionDoc(t *testing.T, md string) adf.Node {
	t.Helper()
	doc := mdToADF(t, md)
	if len(doc.Content) != 1 {
		t.Fatalf("expected exactly one top-level node, got %d", len(doc.Content))
	}
	return doc.Content[0]
}

// TestExpandLegacyContentRecursion covers a nestedContent payload that
// itself holds a second wrapper: both must expand.
func TestExpandLegacyContentRecursion(t *testing.T) {
	md := "> Outer quote\n\n> Inner quote\n"
	want := adfToMD(t, mdToADF(t, md))

	inner := legacyExtNode(t, `{"type": "doc", "version": 1, "content": [
		{"type": "blockquote", "content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "Inner quote"}]}
		]}
	]}`)
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}

	outerNested := `{"type": "doc", "version": 1, "content": [
		{"type": "blockquote", "content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "Outer quote"}]}
		]},
		` + string(innerJSON) + `
	]}`

	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		legacyExt(t, `{"nestedContent": `+outerNested+`}`),
	}}

	got := adfToMD(t, ExpandLegacyContent(doc))
	if got != want {
		t.Fatalf("expand = %q, want %q", got, want)
	}
}

// legacyExtNode builds a raw JSON legacy-content extension object (not
// a Go adf.Node) for embedding as a document's raw content, given a
// nestedContent document literal.
func legacyExtNode(t *testing.T, nestedContentJSON string) map[string]any {
	t.Helper()
	var nested map[string]any
	if err := json.Unmarshal([]byte(nestedContentJSON), &nested); err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"type": "extension",
		"attrs": map[string]any{
			"extensionType": legacyContentType,
			"extensionKey":  legacyContentKey,
			"parameters": map[string]any{
				"nestedContent": nested,
			},
		},
	}
}

// TestExpandLegacyContentRecursionCap proves recursion terminates on a
// self-referential payload: a chain deeper than maxLegacyNesting leaves
// the innermost wrapper in place — surviving as a directive, the same
// fallback every other decline takes — rather than looping or
// vanishing.
func TestExpandLegacyContentRecursionCap(t *testing.T) {
	depth := maxLegacyNesting + 2

	// Build from the innermost wrapper outward: each level's
	// nestedContent holds the next level's extension node.
	node := legacyExtNode(t, `{"type":"doc","version":1,"content":[
		{"type":"paragraph","content":[{"type":"text","text":"bottom"}]}
	]}`)
	for range depth {
		node = legacyExtNode(t, mustJSONString(t, map[string]any{
			"type":    "doc",
			"version": 1.0,
			"content": []any{node},
		}))
	}

	raw := map[string]any{
		"type":    "doc",
		"version": 1.0,
		"content": []any{node},
	}
	doc, ok := adf.DecodeDoc(raw)
	if !ok {
		t.Fatal("DecodeDoc reported invalid input for a map")
	}

	got := ExpandLegacyContent(doc)

	// The cap must have stopped the recursion: at least one
	// legacy-content extension must survive in the result.
	if !hasLegacyExtension(got.Content) {
		t.Fatal("recursion cap did not terminate: no surviving legacy-content extension")
	}

	// Render with the base package only, deliberately NOT through
	// confluence.RenderOptions(): that bundle installs ExpandLegacyContent
	// itself, and rendering through it here would re-run the pass with a
	// fresh depth budget and finish the job the cap just stopped,
	// defeating the check. The base ::extension directive needs no
	// confluence option to fire (adf_extended_test.go exercises it the
	// same way), so this isolates the cap's own behavior.
	md := adfast.ToMarkdown(adfast.FromADF(got))
	if !strings.Contains(md, "::extension") {
		t.Fatalf("expected a surviving ::extension fallback, got %q", md)
	}
}

// mustJSONString marshals v to a JSON string, for building nested raw
// literals level by level.
func mustJSONString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// hasLegacyExtension reports whether any node in content (recursively)
// is a legacy-content extension.
func hasLegacyExtension(content []adf.Node) bool {
	for _, n := range content {
		if ext, ok := n.(*adf.Extension); ok &&
			ext.ExtensionType == legacyContentType && ext.ExtensionKey == legacyContentKey {
			return true
		}
		if hasLegacyExtension(adf.NodeContent(n)) {
			return true
		}
	}
	return false
}

// TestExpandLegacyContentIsCopyOnWrite pins that a document with no
// wrapper is returned sharing its content slice with the input, not a
// copy of it — the adf.Transform contract ExpandLegacyContent relies on.
func TestExpandLegacyContentIsCopyOnWrite(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "no wrapper here"}}},
	}}
	got := ExpandLegacyContent(doc)
	if &got.Content[0] != &doc.Content[0] {
		t.Fatal("content was copied even though nothing changed")
	}
}

// TestExpandLegacyContentInstalledByRenderOptions is the wiring test:
// it fails if RenderOptions stops installing ExpandLegacyContent.
func TestExpandLegacyContentInstalledByRenderOptions(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.BulletList{Content: []adf.Node{
			&adf.ListItem{Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "Item text"}}},
				legacyExt(t, `{
					"cxhtml": "<the original storage format>",
					"nestedContent": {"type": "doc", "version": 1, "content": [
						{"type": "blockquote", "content": [
							{"type": "paragraph", "content": [
								{"type": "text", "text": "Quoted text"}
							]}
						]}
					]}
				}`),
			}},
		}},
	}}

	opts := RenderOptions()
	got := adfast.ToMarkdown(adfast.FromADF(doc, opts...), opts...)
	if !strings.Contains(got, "> Quoted text") {
		t.Fatalf("blockquote did not survive through RenderOptions(): %q", got)
	}
}
