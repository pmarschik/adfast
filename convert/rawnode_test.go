// This file is an external test package on purpose: the behavior pinned
// here is visible only through the facade, which composes convert's ADF
// decode, the markdown render, the markdown parse, and convert's ADF
// encode into one round trip.
package convert_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// decodeADF decodes raw ADF JSON through the typed codec.
func decodeADF(t *testing.T, raw string) (map[string]any, adf.Doc) {
	t.Helper()
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatal(err)
	}
	doc, ok := adf.DecodeDoc(input)
	if !ok {
		t.Fatal("DecodeDoc failed")
	}
	return input, doc
}

// assertADFLossless re-encodes doc (after the ADF→markdown→ADF round
// trip under test rebuilt it) and requires semantic JSON equality with the
// original decoded input — the same check adf/lossless_test.go uses for
// the ADF⇄ADF leg, extended across the markdown hop.
func assertADFLossless(t *testing.T, input map[string]any, doc adf.Doc) {
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
		t.Errorf("ADF -> markdown -> ADF not lossless:\n got: %s\nwant: %v", out, input)
	}
}

// TestRawNode_RoundTrip pins the lossless ADF -> markdown -> ADF round
// trip this directive closes for adf.RawNode, one case per content shape
// (see dialect.RawBlock): a block-position node decodes to the container
// form when it carries block content and to the leaf form when it does
// not; an inline-position node always decodes to the text form, with or
// without inline children.
func TestRawNode_RoundTrip(t *testing.T) {
	for name, tc := range map[string]struct {
		adfJSON  string
		wantForm string // a substring the rendered markdown must contain
	}{
		"block with content": {
			adfJSON:  `{"version":1,"type":"doc","content":[{"type":"futuristicBlock","attrs":{"mode":"beta"},"content":[{"type":"paragraph","content":[{"type":"text","text":"inside"}]}]}]}`,
			wantForm: ":::rawNode{",
		},
		"block, content-less": {
			adfJSON:  `{"version":1,"type":"doc","content":[{"type":"futuristicBlock","attrs":{"mode":"beta"}}]}`,
			wantForm: "::rawNode{",
		},
		"inline with content": {
			adfJSON:  `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"before "},{"type":"futuristicInline","content":[{"type":"text","text":"inside"}]},{"type":"text","text":" after"}]}]}`,
			wantForm: ":rawNode[inside]{",
		},
		"inline, content-less": {
			adfJSON:  `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"before "},{"type":"futuristicInline","attrs":{"mode":"beta"}},{"type":"text","text":" after"}]}]}`,
			wantForm: ":rawNode{",
		},
	} {
		t.Run(name, func(t *testing.T) {
			input, doc := decodeADF(t, tc.adfJSON)
			md := adfast.ToMarkdown(adfast.FromADF(doc))
			if !strings.Contains(md, tc.wantForm) {
				t.Fatalf("markdown = %q, want it to contain %q", md, tc.wantForm)
			}
			back := adfast.ToADF(adfast.FromMarkdown(md))
			assertADFLossless(t, input, back)
		})
	}
}

// TestRawNode_Diagnostic pins CodeRawNode firing on the decode side, worded
// for the now-common case where the dialect's own rawNode directive
// preserves the node rather than dropping it.
func TestRawNode_Diagnostic(t *testing.T) {
	var diags []convert.Diagnostic
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.RawNode{Type: "mysteryBlock"}}}
	adfast.ToMarkdown(adfast.FromADF(doc, adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) })))
	if !hasDiagCode(diags, convert.CodeRawNode) {
		t.Errorf("CodeRawNode did not fire: %v", diags)
	}
	for _, d := range diags {
		if d.Code == convert.CodeRawNode && !strings.Contains(d.Message, "rawNode directive") {
			t.Errorf("CodeRawNode message should describe preservation as a rawNode directive, got %q", d.Message)
		}
	}
}

// TestRawNode_Corrupted pins the "fail loudly" half of the design: a
// rawNode directive a human hand-edited into something unparsable
// reports CodeRawNodeCorrupted on encode, and degrades the same way any
// other malformed directive does (drops, or for the container form
// dissolves into its decoded children) rather than guessing a shape for
// it.
func TestRawNode_Corrupted(t *testing.T) {
	for name, tc := range map[string]struct {
		md       string
		wantKept string // non-empty: the corrupted container dissolved into this text
		wantNil  bool   // the corrupted leaf dropped outright
	}{
		"missing type, leaf": {
			md:      `::rawNode{data='{"attrs":{"a":1}}'}` + "\n",
			wantNil: true,
		},
		"unparsable data, leaf": {
			md:      `::rawNode{type="x" data="not json"}` + "\n",
			wantNil: true,
		},
		"unparsable data, container dissolves into children": {
			md:       ":::rawNode{type=\"x\" data=\"not json\"}\nkept\n:::\n",
			wantKept: "kept",
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertRawNodeCorrupted(t, tc.md, tc.wantNil, tc.wantKept)
		})
	}
}

// assertRawNodeCorrupted encodes md and requires CodeRawNodeCorrupted to
// have fired, then checks the one degraded shape the case expects:
// wantNil for a leaf's outright drop, wantKept for a container's dissolve
// into its decoded children.
func assertRawNodeCorrupted(t *testing.T, md string, wantNil bool, wantKept string) {
	t.Helper()
	var diags []convert.Diagnostic
	doc := adfast.ToADF(adfast.FromMarkdown(md), adfast.WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))
	if !hasDiagCode(diags, convert.CodeRawNodeCorrupted) {
		t.Errorf("CodeRawNodeCorrupted did not fire: %v", diags)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if wantNil && strings.Contains(string(out), `"type":"x"`) {
		t.Errorf("corrupted leaf should drop, got %s", out)
	}
	if wantKept != "" && !strings.Contains(string(out), wantKept) {
		t.Errorf("corrupted container should dissolve into its children, got %s", out)
	}
}

// hasDiagCode reports whether diags contains code.
func hasDiagCode(diags []convert.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}
