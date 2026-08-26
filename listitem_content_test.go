package adfast

import (
	"bytes"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// TestDiagnostics_ListItemContent covers every forbidden construct: a
// block inside a list item that ADF's listItem content model does not
// allow. Each subtest expects exactly one convert.CodeListItemContent
// diagnostic naming the offending kind.
func TestDiagnostics_ListItemContent(t *testing.T) {
	tests := []struct {
		name string
		md   string
		kind string
	}{
		{"blockquote", "- item\n\n  > quote\n", "blockquote"},
		{"table", "- item\n\n  | a | b |\n  | - | - |\n", "table"},
		{"heading", "- item\n\n  ## sub\n", "heading"},
		{"rule", "- item\n\n  ---\n", "rule"},
		{"panel", "- item\n\n  :::info\n  note\n  :::\n", "panel"},
		{"expand", "- item\n\n  :::expand[more]\n  body\n  :::\n", "expand"},
		{"mediaGroup", "- item\n\n  ::media[a]{group=\"true\" type=\"external\" url=\"https://example.com/i.png\"}\n", "mediaGroup"},
		{"decisionList", "- item\n\n  ::decisions\n\n  - decided\n", "decisionList"},
		{"ordered list too", "1. item\n\n   > quote\n", "blockquote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags []convert.Diagnostic
			sink := func(d convert.Diagnostic) { diags = append(diags, d) }
			mdToADF(tt.md, WithDiagnostics(sink))
			if len(diags) != 1 {
				t.Fatalf("mdToADF(%q) diagnostics: %+v", tt.md, diags)
			}
			if diags[0].Code != convert.CodeListItemContent {
				t.Errorf("Code = %q, want %q", diags[0].Code, convert.CodeListItemContent)
			}
			if !bytes.Contains([]byte(diags[0].Message), []byte(tt.kind)) {
				t.Errorf("Message %q does not mention kind %q", diags[0].Message, tt.kind)
			}
		})
	}
}

// TestDiagnostics_ListItemContent_ByteGate is the most important test: it
// pins that the encoded structure is unchanged (the blockquote stays
// nested inside the listItem, not lifted out or re-nested) AND that
// wiring a diagnostics sink does not perturb the encode at all.
func TestDiagnostics_ListItemContent_ByteGate(t *testing.T) {
	t.Run("pinned structure", func(t *testing.T) {
		md := "- item\n\n  > quote\n"
		want := doc(&adf.BulletList{Content: []adf.Node{
			li(p(txt("item")), &adf.Blockquote{Content: []adf.Node{p(txt("quote"))}}),
		}})
		assertSameADF(t, want, mdToADF(md))
	})

	t.Run("sink does not perturb the encode", func(t *testing.T) {
		mds := []string{
			"- item\n\n  > quote\n",
			"- item\n\n  | a | b |\n  | - | - |\n",
			"- item\n\n  ## sub\n",
			"- item\n\n  ---\n",
			"- item\n\n  :::info\n  note\n  :::\n",
			"- item\n\n  :::expand[more]\n  body\n  :::\n",
			"- item\n\n  ::media[a]{group=\"true\" type=\"external\" url=\"https://example.com/i.png\"}\n",
			"- item\n\n  ::decisions\n\n  - decided\n",
			"- item\n\n  ::extension{key=\"chart\" type=\"com.example.charts\"}\n",
			"1. item\n\n   > quote\n",
		}
		for _, md := range mds {
			without := mustJSON(mdToADF(md))
			with := mustJSON(mdToADF(md, WithDiagnostics(func(convert.Diagnostic) {})))
			if !bytes.Equal(without, with) {
				t.Errorf("mdToADF(%q): sink perturbed the encode\nwithout: %s\nwith:    %s", md, without, with)
			}
		}
	})
}

// TestDiagnostics_ListItemContent_LeadingNestedListIsSilent pins that a
// nested list opening a list item ("- - x") is silent: the pinned
// listItem content model is a single flat "+" alternation with no
// first-position restriction to enforce (older ADF schema revisions did
// restrict the first position; this repo's pin postdates that rule).
// adfast checks only the allowed-kind set, and against the pinned model
// that is the whole check. This is the mutation-check guard for that.
func TestDiagnostics_ListItemContent_LeadingNestedListIsSilent(t *testing.T) {
	for _, md := range []string{"- - x", "- - - x", "* - item\n"} {
		var diags []convert.Diagnostic
		sink := func(d convert.Diagnostic) { diags = append(diags, d) }
		mdToADF(md, WithDiagnostics(sink))
		if hasCode(diags, convert.CodeListItemContent) {
			t.Errorf("mdToADF(%q): unexpected list-item-content diagnostic: %+v", md, diags)
		}
	}
}

// TestDiagnostics_ListItemContent_ConformingDocumentIsSilent exercises
// all six allowed kinds in list items and expects no list-item-content
// diagnostic (an unresolved-asset diagnostic may legitimately co-fire
// for the image, since no asset store is configured).
func TestDiagnostics_ListItemContent_ConformingDocumentIsSilent(t *testing.T) {
	md := "- a paragraph\n\n" +
		"  ```\n" +
		"  code\n" +
		"  ```\n\n" +
		"  - nested bullet\n\n" +
		"  1. nested ordered\n\n" +
		"  - [ ] a task\n\n" +
		"  ![alt](image.png)\n"
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	mdToADF(md, WithDiagnostics(sink))
	if hasCode(diags, convert.CodeListItemContent) {
		t.Errorf("mdToADF(%q): unexpected list-item-content diagnostic: %+v", md, diags)
	}
}

// TestDiagnostics_ListItemContent_ExtensionIsSilent pins that extension
// is in the listItem content model's alternation (the pinned oracle:
// docs/adf-coverage.md:122, atlassian-frontend-mirror commit
// f5ca0f120c6ea5d79873805d081a72c82917e1f8, list-item.ts's
// listItemFactory), so an extension inside a list item is representable
// and must not raise a list-item-content diagnostic — unlike blockquote
// and table, which remain genuine violations and stay covered by
// TestDiagnostics_ListItemContent above.
func TestDiagnostics_ListItemContent_ExtensionIsSilent(t *testing.T) {
	md := "- item\n\n  ::extension{key=\"chart\" type=\"com.example.charts\"}\n"
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	mdToADF(md, WithDiagnostics(sink))
	if hasCode(diags, convert.CodeListItemContent) {
		t.Errorf("mdToADF(%q): unexpected list-item-content diagnostic: %+v", md, diags)
	}
}

// TestDiagnostics_ListItemContent_NestedListsReachTheDeepestItem proves
// the check is not shallow: a violation buried three list levels deep is
// still found, and violations at multiple levels are all counted.
func TestDiagnostics_ListItemContent_NestedListsReachTheDeepestItem(t *testing.T) {
	md := "- a\n\n  - b\n\n    - c\n\n      > quote\n"
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	mdToADF(md, WithDiagnostics(sink))
	if n := len(codes(diags)); n != 1 {
		t.Fatalf("diagnostics: %+v, want exactly 1", diags)
	}

	md2 := "- a\n\n  | x | y |\n  | - | - |\n\n  - b\n\n    - c\n\n      > quote\n"
	diags = nil
	mdToADF(md2, WithDiagnostics(sink))
	if n := len(codes(diags)); n != 2 {
		t.Fatalf("diagnostics: %+v, want exactly 2", diags)
	}
}

// TestDiagnostics_ListItemContent_DistinctKindsFireOnce pins the dedup:
// three blockquote-in-item violations and two table-in-item violations
// across one document report exactly two diagnostics, one per kind.
func TestDiagnostics_ListItemContent_DistinctKindsFireOnce(t *testing.T) {
	md := "- a\n\n  > q1\n\n- b\n\n  > q2\n\n- c\n\n  > q3\n\n" +
		"- d\n\n  | x | y |\n  | - | - |\n\n" +
		"- e\n\n  | x | y |\n  | - | - |\n"
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	mdToADF(md, WithDiagnostics(sink))
	if len(diags) != 2 {
		t.Fatalf("diagnostics: %+v, want exactly 2 (one per distinct kind)", diags)
	}
	for _, d := range diags {
		if d.Code != convert.CodeListItemContent {
			t.Errorf("Code = %q, want %q", d.Code, convert.CodeListItemContent)
		}
	}
}

// TestDiagnostics_ListItemContent_FootnoteDefinition is the test that a
// 641-only implementation (checking only convertList) fails: a footnote
// definition holding a blockquote lands in the tail's ordered list
// (convert/footnote.go:89), which also builds a listItem. If this test
// does not fail against a 641-only variant, the footnote path does not
// carry the blockquote and the post-pass reasoning needs revisiting.
func TestDiagnostics_ListItemContent_FootnoteDefinition(t *testing.T) {
	md := "ref[^a]\n\n[^a]: def\n\n    > quote\n"
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	mdToADF(md, WithDiagnostics(sink))
	if !hasCode(diags, convert.CodeListItemContent) {
		t.Fatalf("mdToADF(%q): expected a list-item-content diagnostic for the footnote tail, got %+v", md, diags)
	}
}

// TestDiagnostics_ListItemContent_NoSinkIsSilent is a cheap regression on
// the c.diagnostics == nil early return: no sink must not panic and must
// produce the same output as the sink case.
func TestDiagnostics_ListItemContent_NoSinkIsSilent(t *testing.T) {
	md := "- item\n\n  > quote\n"
	got := mustJSON(mdToADF(md))
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	want := mustJSON(mdToADF(md, WithDiagnostics(sink)))
	if !bytes.Equal(got, want) {
		t.Errorf("mdToADF without a sink differs from mdToADF with one\nwithout: %s\nwith:    %s", got, want)
	}
}
