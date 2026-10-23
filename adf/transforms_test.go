package adf_test

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// TestNormalizeTextNewlines_CollapsesJunctionAcrossDroppedBlockBoundary
// unit-tests collapseTextJunctions's own remaining job directly against a
// hand-built Doc, independent of the Markdown→ADF pipeline.
//
// convert.mergeAdjacentADFText (convert/ast_to_adf.go) now closes the
// same-mark-text junction a dropped *inline* node leaves behind — but
// only within the one list of sibling inline nodes a single
// flattenChildren call sees, i.e. the children of one block. A junction
// that instead spans two originally separate blocks whose boundary
// disappears — two paragraphs whose content some block-dissolving step
// splices into one paragraph, outside any single flattenChildren call's
// scope — never passes through that merge at all. collapseTextJunctions
// is the pass that still has to notice the leftover space run there and
// trim it; nothing above it in the pipeline re-checks that boundary.
//
// This Doc is built by hand rather than through mdToADF because the
// point is to pin collapseTextJunctions's own documented contract — trim
// the duplicate space, but leave the two nodes distinct rather than
// fusing them like mergeAdjacentADFText does — directly, so a regression
// here is caught regardless of which markdown construct (if any, today)
// reaches this shape through the full pipeline.
func TestNormalizeTextNewlines_CollapsesJunctionAcrossDroppedBlockBoundary(t *testing.T) {
	doc := adf.Doc{
		Type:    "doc",
		Version: 1,
		Content: []adf.Node{
			&adf.Paragraph{Content: []adf.Node{
				&adf.Text{Text: "first block tail "},
				&adf.Text{Text: " second block head"},
			}},
		},
	}

	got := adf.NormalizeTextNewlines(doc)

	p, ok := got.Content[0].(*adf.Paragraph)
	if !ok || len(p.Content) != 2 {
		t.Fatalf("content = %#v, want one paragraph with 2 text nodes", got.Content)
	}
	first, ok1 := p.Content[0].(*adf.Text)
	second, ok2 := p.Content[1].(*adf.Text)
	if !ok1 || !ok2 {
		t.Fatalf("paragraph content kinds = %#v, want two *adf.Text nodes", p.Content)
	}
	if first.Text != "first block tail " {
		t.Errorf("first node text = %q, want %q (unchanged)", first.Text, "first block tail ")
	}
	if second.Text != "second block head" {
		t.Errorf("second node text = %q, want %q (leading space trimmed, not merged)", second.Text, "second block head")
	}
}
