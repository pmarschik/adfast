package adfast

import "testing"

// TestBlockTaskItemDegradeEscapesASplitDigitRun covers
// FuzzRoundTripIdempotent's seed '* [X] \n\n  0\n\n  )': a task item whose
// first block is an empty paragraph degrades through the historical
// paragraph-flattening projection (convertAdfTaskList in
// convert/adf_to_ast.go), which used to concatenate "0" and ")" into one
// line with no escape, because each source paragraph's inline content
// became its OWN text node and render_escape.go's escapeOrderedMarker only
// sees a line-start digit run within a single node's string. The unescaped
// "0)" then read back on the next parse as one text node, where the rule
// DOES see the digit run and escapes it — two different documents from one
// input.
func TestBlockTaskItemDegradeEscapesASplitDigitRun(t *testing.T) {
	const md = "* [X] \n\n  0\n\n  )"
	const want = "- [x] 0\\)\n"

	first := adfToMD(mdToADF(md))
	if first != want {
		t.Fatalf("first render = %q, want %q", first, want)
	}
	second := adfToMD(mdToADF(first))
	if second != first {
		t.Errorf("round trip of %q is not a fixpoint\nfirst:  %q\nsecond: %q", md, first, second)
	}
}

// TestBlockTaskItemDegradeFixpointControls pins neighboring shapes the fix
// must leave alone: a non-digit split (no escape was ever needed), a
// digit run entirely within one paragraph (already escaped correctly before
// this fix), and a split where a mark wrapper sits between the two
// paragraphs' content (the wrapper already breaks node adjacency, so the
// escape rule's single-node view was never the problem there).
func TestBlockTaskItemDegradeFixpointControls(t *testing.T) {
	tests := map[string]string{
		"a non-digit split needs no escape":        "* [X] \n\n  a\n\n  b",
		"a digit run inside one paragraph alone":   "* [X] \n\n  0)",
		"a mark wrapper separates the digit split": "* [X] \n\n  0\n\n  *x*\n\n  )",
	}
	for name, md := range tests {
		t.Run(name, func(t *testing.T) {
			first := adfToMD(mdToADF(md))
			second := adfToMD(mdToADF(first))
			if second != first {
				t.Errorf("round trip of %q is not a fixpoint\nfirst:  %q\nsecond: %q", md, first, second)
			}
		})
	}
}
