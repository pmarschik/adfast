package markdown_test

import (
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// The subject of this file is the one thing the three source views could not
// say before: that a construct exists in the document which the view could
// not locate, and therefore left out. A consumer that rewrites destinations
// iterates a view, so a construct missing from one is bytes the pass leaves
// stale — and until UnlocatedDefinitions/UnlocatedLinks/UnlocatedImages the
// pass had no way to notice, or to decline to call the rewrite complete.
//
// The shape that used to provoke it is a container prefix ending in a TAB:
// goldmark consumes one column of the tab and records the rest as padding on
// the line's segment, spaces that stand for no byte of the source, which then
// turn up in the label and the title the definition parser recorded. That is
// what matchesAsRead discounts, and the first table below is the proof. Every
// case there comes in two, a tab and a SPACE prefix that is otherwise
// identical, because a discount that also changes the space case is reading
// something other than the padding.
//
// A count of zero is not the same claim as a count that cannot fire, so the
// second table keeps a shape that still drops.

// unlocatedCase is one document and what the definitions view must say about
// it, including the bytes each part of a located definition must select — a
// definition reported with a span shifted by the padding it was supposed to
// discount is worse than one left out.
type unlocatedCase struct {
	name string
	src  string
	// wantLabel and wantTitle are the source bytes those spans must select,
	// per located definition, in the view's order. An empty string is the
	// zero Span, which is how a definition with no title is reported.
	wantLabel []string
	wantTitle []string
	// wantDefsUnlocated is how many definitions the view must admit it
	// could not locate.
	wantDefsUnlocated int
}

// TestUnlocated_APaddedTabHidesNothing is a DEFECT PROOF for the padding
// discount: every tab row below was reported as an unlocated definition
// before matchesAsRead, so the view returned nothing for it and a rewriter
// had to decline the whole document.
//
// The space rows are the good cases — the same definitions with a space
// where the tab is — and so is the destination row, which is a tab case that
// resolved all along because the parser skips leading whitespace before it
// reads a destination. A discount keyed on the tab rather than on the
// recorded value gets those wrong.
func TestUnlocated_APaddedTabHidesNothing(t *testing.T) {
	cases := []unlocatedCase{
		{
			name:      "title on the next line, blockquote, tab",
			src:       "> [a]: x.md\n>\t\"t\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t"},
		},
		{
			name:      "title on the next line, blockquote, space",
			src:       "> [a]: x.md\n>  \"t\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t"},
		},
		{
			name:      "title on the next line, list item, tab",
			src:       "- [a]: x.md\n\t\"t\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t"},
		},
		{
			name:      "title on the next line, list item, space",
			src:       "- [a]: x.md\n  \"t\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t"},
		},
		{
			// The padding reaches the title TWICE over here: once for the
			// line it opens on, and once more because the parser reads it
			// as two segments and the first one picks up the padding of the
			// line the second one is on.
			name:      "title folded across a line, blockquote, tab",
			src:       "> [a]: x.md\n>\t\"t\n> u\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t\n> u"},
		},
		{
			name:      "label folded across a line, blockquote, tab",
			src:       "> [a\n>\tb]: x.md\n",
			wantLabel: []string{"a\n>\tb"},
			wantTitle: []string{""},
		},
		{
			name:      "label folded across a line, blockquote, space",
			src:       "> [a\n>  b]: x.md\n",
			wantLabel: []string{"a\n>  b"},
			wantTitle: []string{""},
		},
		{
			name:      "label folded across two lines, blockquote, tab",
			src:       "> [a\n>\tb\n>\tc]: x.md\n",
			wantLabel: []string{"a\n>\tb\n>\tc"},
			wantTitle: []string{""},
		},
		{
			// The definition's OWN first line: the parser skips the padding
			// before it reads the label, so the segment the definition node
			// keeps records none — yet the label it recorded has the padding
			// in front of it anyway, because the reader that valued it was
			// built over the paragraph's segments, which do.
			name:      "whole definition on a padded line, blockquote, tab",
			src:       ">\t[a]: x.md \"t\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t"},
		},
		{
			// The padding is leading whitespace the parser skips before it
			// reads a destination, so this one resolved before the discount
			// too. It is a good case with a tab in it, which is the row a
			// fix that keys off the tab rather than the padding gets wrong.
			name:      "destination on the next line, blockquote, tab",
			src:       "> [a]:\n>\tx.md\n",
			wantLabel: []string{"a"},
			wantTitle: []string{""},
		},
		{
			name:      "destination on the next line, blockquote, space",
			src:       "> [a]:\n>  x.md\n",
			wantLabel: []string{"a"},
			wantTitle: []string{""},
		},
		{
			name:      "two definitions, one behind a tab",
			src:       "> [a]: x.md\n>\t\"t\"\n\n[b]: y.md\n",
			wantLabel: []string{"a", "b"},
			wantTitle: []string{"t", ""},
		},
		{
			name: "no definition at all",
			src:  "plain text\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			checkDefinitionParts(t, tc, s.Definitions())
			if got := s.UnlocatedDefinitions(); got != tc.wantDefsUnlocated {
				t.Errorf("UnlocatedDefinitions() = %d, want %d",
					got, tc.wantDefsUnlocated)
			}
		})
	}
}

// checkDefinitionParts fails when the view did not report the definitions the
// case names, or when a reported label or title selects other bytes.
func checkDefinitionParts(t *testing.T, tc unlocatedCase, got []markdown.Definition) {
	t.Helper()
	if len(got) != len(tc.wantLabel) {
		t.Fatalf("len(Definitions()) = %d, want %d", len(got), len(tc.wantLabel))
	}
	for i, d := range got {
		for _, part := range []struct {
			name string
			want string
			span markdown.Span
		}{
			{"Label", tc.wantLabel[i], d.Label},
			{"Title", tc.wantTitle[i], d.Title},
		} {
			if part.want == "" {
				if part.span != (markdown.Span{}) {
					t.Errorf("definition %d %s = %v, want the zero Span",
						i, part.name, part.span)
				}
				continue
			}
			if sel := tc.src[part.span.Start:part.span.Stop]; sel != part.want {
				t.Errorf("definition %d %s selects %q, want %q",
					i, part.name, sel, part.want)
			}
		}
	}
}

// TestUnlocated_CountTheDefinitionAnUnwritableTitleHides is the DEFECT PROOF
// for the count itself, on the shape that still drops: goldmark records a
// title it read from a line it then leaves OUT of the definition's extent,
// so the definition it describes and the definition it delimits are not the
// same one and no span can be trusted. Before UnlocatedDefinitions a caller
// saw an empty view and nothing else, which is what a document with no
// definition at all looks like.
//
// (The parse is wrong about the title as well — CommonMark ends the
// definition at the destination when the title's line has trailing content,
// and the reference parser reports no title — but a view over spans is not
// where that is fixed. Dropping is the safe direction, and the count is what
// makes the drop visible.)
//
// The good case is the same document with the trailing content removed: the
// title is then part of the definition, the definition locates, and the
// count stays zero — so a counter that fires on any title written on its own
// line fails here.
func TestUnlocated_CountTheDefinitionAnUnwritableTitleHides(t *testing.T) {
	for _, tc := range []unlocatedCase{
		{
			name:              "title line with trailing content",
			src:               "[a]: x.md\n\"t\" tail\n",
			wantDefsUnlocated: 1,
		},
		{
			name:      "title line without it",
			src:       "[a]: x.md\n\"t\"\n",
			wantLabel: []string{"a"},
			wantTitle: []string{"t"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			checkDefinitionParts(t, tc, s.Definitions())
			if got := s.UnlocatedDefinitions(); got != tc.wantDefsUnlocated {
				t.Errorf("UnlocatedDefinitions() = %d, want %d",
					got, tc.wantDefsUnlocated)
			}
		})
	}
}

// TestUnlocated_LocatedDefinitionsStillCarryTheirParts is the other half of
// the good case: a located definition's FOUR spans must all select the bytes
// that were written, so a change that reports one more definition by widening
// or shifting a span fails here rather than passing quietly.
//
// The space row is a PRESERVED-BEHAVIOR PIN — it passes on the implementation
// before the padding discount too, which is the point, since nothing about an
// unpadded definition changed. The tab row is the one the discount added, and
// it is where a span that swallowed the padding it discounted would show up:
// the extent still reaches over the `>\t` prefix, because that is bytes the
// author wrote, while the label and the title do not.
func TestUnlocated_LocatedDefinitionsStillCarryTheirParts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		span  string
		label string
		dest  string
		title string
	}{
		{
			name:  "space prefix",
			src:   "> [a]: x.md\n>  \"t\"\n",
			span:  "[a]: x.md\n>  \"t\"",
			label: "a",
			dest:  "x.md",
			title: "t",
		},
		{
			name:  "tab prefix",
			src:   "> [a]: x.md\n>\t\"t\"\n",
			span:  "[a]: x.md\n>\t\"t\"",
			label: "a",
			dest:  "x.md",
			title: "t",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			defs := s.Definitions()
			if len(defs) != 1 {
				t.Fatalf("len(Definitions()) = %d, want 1", len(defs))
			}
			got := defs[0]
			for _, part := range []struct {
				name string
				want string
				span markdown.Span
			}{
				{"Span", tc.span, got.Span},
				{"Label", tc.label, got.Label},
				{"Dest", tc.dest, got.Dest},
				{"Title", tc.title, got.Title},
			} {
				if slice := tc.src[part.span.Start:part.span.Stop]; slice != part.want {
					t.Errorf("%s selects %q, want %q", part.name, slice, part.want)
				}
			}
		})
	}
}

// TestUnlocated_AccessorForcesItsOwnView is a DEFECT PROOF for the accessor
// contract: the count must be right when it is the FIRST thing a caller
// asks for. A consumer that only wants to know whether a rewrite can be
// complete has no reason to read the view first, and an accessor returning
// the zero value of an unpopulated field would tell it everything is fine.
func TestUnlocated_AccessorForcesItsOwnView(t *testing.T) {
	for _, tc := range []struct {
		got  func(*markdown.Source) int
		name string
		want int
	}{
		{
			name: "definitions",
			got:  (*markdown.Source).UnlocatedDefinitions,
			want: 1,
		},
		{
			name: "links",
			got:  (*markdown.Source).UnlocatedLinks,
			want: 0,
		},
		{
			name: "images",
			got:  (*markdown.Source).UnlocatedImages,
			want: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte("[a]: x.md\n\"t\" tail\n"))
			if got := tc.got(s); got != tc.want {
				t.Errorf("accessor before its view = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestUnlocated_LinksAndImagesAreImmuneToATabPrefix records the measured
// reason the limitation belongs to Definitions alone: the blocks Links and
// Images resolve against are paragraphs, and a paragraph's parser trims each
// line's leading whitespace — the tab's leftover padding included — when it
// closes. So every shape here is located, tab or space.
//
// The view assertions are a PRESERVED-BEHAVIOR PIN: they pass on the
// implementation before the count too, and they exist to keep a future
// attempt at "handling" the tab from breaking what already works. The
// Unlocated assertions are new, and zero is what they must be.
func TestUnlocated_LinksAndImagesAreImmuneToATabPrefix(t *testing.T) {
	for _, tc := range []struct {
		name      string
		src       string
		wantLinks int
		wantImgs  int
	}{
		{"link destination continued, blockquote, tab", "> [t](\n>\tx.md)\n", 1, 0},
		{"link destination continued, blockquote, space", "> [t](\n>  x.md)\n", 1, 0},
		{"link destination continued, list item, tab", "- [t](\n\tx.md)\n", 1, 0},
		{"link destination continued, list item, space", "- [t](\n  x.md)\n", 1, 0},
		{"link title continued, blockquote, tab", "> [t](x.md\n>\t\"y\")\n", 1, 0},
		{"link title continued, blockquote, space", "> [t](x.md\n>  \"y\")\n", 1, 0},
		{"link text folded, blockquote, tab", "> [a\n>\tb](x.md)\n", 1, 0},
		{"link text folded, blockquote, space", "> [a\n>  b](x.md)\n", 1, 0},
		{
			"reference label folded, blockquote, tab",
			"> [t][i\n>\td]\n\n[i d]: x.md\n", 1, 0,
		},
		{
			"reference label folded, blockquote, space",
			"> [t][i\n>  d]\n\n[i d]: x.md\n", 1, 0,
		},
		{"image destination continued, blockquote, tab", "> ![a](\n>\tx.png)\n", 0, 1},
		{"image destination continued, blockquote, space", "> ![a](\n>  x.png)\n", 0, 1},
		{"image destination continued, list item, tab", "- ![a](\n\tx.png)\n", 0, 1},
		{"image title continued, blockquote, tab", "> ![a](x.png\n>\t\"y\")\n", 0, 1},
		{
			"image reference label folded, blockquote, tab",
			"> ![a][i\n>\td]\n\n[i d]: x.png\n", 0, 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			if got := len(s.Links()); got != tc.wantLinks {
				t.Errorf("len(Links()) = %d, want %d", got, tc.wantLinks)
			}
			if got := len(s.Images()); got != tc.wantImgs {
				t.Errorf("len(Images()) = %d, want %d", got, tc.wantImgs)
			}
			if got := s.UnlocatedLinks(); got != 0 {
				t.Errorf("UnlocatedLinks() = %d, want 0", got)
			}
			if got := s.UnlocatedImages(); got != 0 {
				t.Errorf("UnlocatedImages() = %d, want 0", got)
			}
		})
	}
}
