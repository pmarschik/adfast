package adf_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// decodeCollecting decodes raw JSON through the typed codec, collecting
// diagnostics.
func decodeCollecting(t *testing.T, raw string) (map[string]any, adf.Doc, []adf.Diagnostic) {
	t.Helper()
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatal(err)
	}
	var diags []adf.Diagnostic
	doc, ok := adf.DecodeDocOpts(input, adf.DecodeOptions{
		Diagnostics: func(d adf.Diagnostic) { diags = append(diags, d) },
	})
	if !ok {
		t.Fatal("DecodeDocOpts failed")
	}
	return input, doc, diags
}

// assertLossless re-encodes doc and requires semantic JSON equality
// with the decoded input.
func assertLossless(t *testing.T, input map[string]any, doc adf.Doc) {
	t.Helper()
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Errorf("decode→encode not lossless:\n got: %s\nwant: %v", out, input)
	}
}

// assertRawUnknown checks the unknown node kind decoded to RawNode
// with typed children.
func assertRawUnknown(t *testing.T, n adf.Node) {
	t.Helper()
	rawNode, isRaw := n.(*adf.RawNode)
	if !isRaw || rawNode.Type != "futuristicBlock" {
		t.Fatalf("expected RawNode(futuristicBlock), got %T", n)
	}
	if len(rawNode.Content) != 1 || rawNode.Content[0].Kind() != "paragraph" {
		t.Errorf("RawNode children must decode typed: %#v", rawNode.Content)
	}
}

// assertRawShapeFallback checks a known kind used with slots its typed
// struct does not model (content on the leaf "extension" node without a
// body) falls back to RawNode losslessly, without a diagnostic.
func assertRawShapeFallback(t *testing.T, n adf.Node) {
	t.Helper()
	rawNode, isRaw := n.(*adf.RawNode)
	if !isRaw || rawNode.Type != "extension" {
		t.Fatalf("expected RawNode(extension), got %T", n)
	}
	if len(rawNode.Content) != 1 || rawNode.Content[0].Kind() != "paragraph" {
		t.Errorf("RawNode children must decode typed: %#v", rawNode.Content)
	}
}

// assertPanelExtras checks the known node kept its typed field, the
// unmodeled attrs in Extra, and the unknown mark as RawMark.
func assertPanelExtras(t *testing.T, n adf.Node) {
	t.Helper()
	panel, isPanel := n.(*adf.Panel)
	if !isPanel || panel.PanelType != "info" {
		t.Fatalf("expected typed Panel(info), got %#v", n)
	}
	if panel.Extra["panelIconId"] != "1f600" || panel.Extra["customAttr"] != float64(42) {
		t.Errorf("unmodeled attrs must land in Extra: %#v", panel.Extra)
	}
	text, isText := adf.NodeContent(adf.NodeContent(panel)[0])[0].(*adf.Text)
	if !isText {
		t.Fatal("expected text node inside panel")
	}
	if _, isRawMark := text.Marks[0].(*adf.RawMark); !isRawMark {
		t.Errorf("expected RawMark, got %T", text.Marks[0])
	}
	if _, isStrong := text.Marks[1].(*adf.Strong); !isStrong {
		t.Errorf("expected Strong, got %T", text.Marks[1])
	}
}

// assertDiagnosticCounts checks the diagnostic codes fired.
func assertDiagnosticCounts(t *testing.T, diags []adf.Diagnostic, want map[string]int) {
	t.Helper()
	counts := map[string]int{}
	for _, d := range diags {
		counts[d.Code]++
	}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("diagnostics: got %v want %v (%+v)", counts, want, diags)
	}
}

// TestDecodeEncode_Lossless pins the codec's losslessness contract:
// a document containing (a) an unknown node kind, (b) a known node with
// unmodeled attributes, and (c) an unknown mark kind decodes into the
// typed model (RawNode/RawMark/Extra) and re-encodes to semantically
// identical JSON, with the matching diagnostics fired.
func TestDecodeEncode_Lossless(t *testing.T) {
	raw := `{
		"type": "doc",
		"version": 1,
		"content": [
			{
				"type": "futuristicBlock",
				"attrs": {"futureAttr": "x"},
				"content": [{"type": "paragraph", "content": [{"type": "text", "text": "inside"}]}]
			},
			{
				"type": "extension",
				"attrs": {"extensionType": "com.atlassian.confluence.macro.core", "layout": "default"},
				"content": [{"type": "paragraph", "content": [{"type": "text", "text": "inside"}]}]
			},
			{
				"type": "panel",
				"attrs": {"panelType": "info", "panelIconId": "1f600", "customAttr": 42},
				"content": [{
					"type": "paragraph",
					"content": [{
						"type": "text",
						"text": "hi",
						"marks": [
							{"type": "glowMark", "attrs": {"glow": "high"}},
							{"type": "strong"}
						]
					}]
				}]
			}
		]
	}`
	input, doc, diags := decodeCollecting(t, raw)
	assertRawUnknown(t, doc.Content[0])
	assertRawShapeFallback(t, doc.Content[1])
	assertPanelExtras(t, doc.Content[2])
	assertLossless(t, input, doc)
	assertDiagnosticCounts(t, diags, map[string]int{
		"unknown-node": 1, // futuristicBlock
		"unknown-attr": 2, // panelIconId, customAttr
		"unknown-mark": 1, // glowMark
	})
}

// TestDecodeEncode_NoTypedKindStaysRaw is the regression for a drift
// between the two halves of the node-kind table: "image", "frontmatter",
// and "html" were classified with a generic-slot shape but had no typed
// constructor, so a document containing one decoded to a nil Node —
// silent tree corruption that crashed the first caller to touch it. Every
// kind the decoder does not build must take the RawNode escape.
func TestDecodeEncode_NoTypedKindStaysRaw(t *testing.T) {
	raw := `{"type":"doc","version":1,"content":[
		{"type":"image","attrs":{"src":"a.png"}},
		{"type":"html","text":"<b>x</b>"},
		{"type":"frontmatter","text":"title: x"}
	]}`
	input, doc, diags := decodeCollecting(t, raw)
	for i, n := range doc.Content {
		if n == nil {
			t.Fatalf("content[%d] decoded to a nil Node", i)
		}
		if _, isRaw := n.(*adf.RawNode); !isRaw {
			t.Errorf("content[%d]: expected RawNode, got %T", i, n)
		}
	}
	assertLossless(t, input, doc)
	assertDiagnosticCounts(t, diags, map[string]int{"unknown-node": 3})
}

// TestDecodeEncode_PresenceSensitiveAttrs pins the pointer fields and
// the Extra fallback for known attributes whose decoded value the
// plain typed field could not represent faithfully (zero values where
// encoding only emits non-zero).
func TestDecodeEncode_PresenceSensitiveAttrs(t *testing.T) {
	raw := `{"type":"doc","version":1,"content":[
		{"type":"heading","attrs":{"level":0},"content":[{"type":"text","text":"h"}]},
		{"type":"expand","attrs":{"title":""},"content":[{"type":"paragraph","content":[{"type":"text","text":"p"}]}]},
		{"type":"taskList","attrs":{"localId":""},"content":[
			{"type":"taskItem","attrs":{"localId":"","state":"TODO"},"content":[{"type":"text","text":"x"}]}
		]}
	]}`
	input, doc, _ := decodeCollecting(t, raw)
	assertLossless(t, input, doc)
	expand, ok := doc.Content[1].(*adf.Expand)
	if !ok {
		t.Fatalf("expected typed Expand, got %T", doc.Content[1])
	}
	if expand.Title == nil || *expand.Title != "" {
		t.Errorf("empty-but-present title must decode as pointer: %v", expand.Title)
	}
}
