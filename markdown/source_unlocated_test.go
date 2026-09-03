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
// The shape that provokes it is a container prefix ending in a TAB on a
// CONTINUED line: goldmark consumes one column of the tab and records the
// rest as padding on the line's segment, spaces that stand for no byte of
// the source. Every case below therefore comes in two, a tab and a SPACE
// prefix that is otherwise identical, because a count that reacts to the
// space case is over-reporting and no better than the silence it replaced.

// unlocatedCase is one document and what the three views must say about it.
type unlocatedCase struct {
	name string
	src  string
	// wantDefs is how many definitions the view must report.
	wantDefs int
	// wantDefsUnlocated is how many it must admit it could not locate.
	wantDefsUnlocated int
}

// TestUnlocated_CountTheDefinitionAPaddedTabHides is a DEFECT PROOF for the
// count: before UnlocatedDefinitions existed, the tab rows below reported an
// empty view and nothing else, which a caller cannot distinguish from a
// document that has no definition at all.
//
// The space rows are the good cases. They are the same definitions written
// with a space where the tab is, they must be located, and their count must
// stay zero — so a counter that fires on any continued line, rather than on
// the padding, fails here.
func TestUnlocated_CountTheDefinitionAPaddedTabHides(t *testing.T) {
	cases := []unlocatedCase{
		{
			name:              "title on the next line, blockquote, tab",
			src:               "> [a]: x.md\n>\t\"t\"\n",
			wantDefs:          0,
			wantDefsUnlocated: 1,
		},
		{
			name:              "title on the next line, blockquote, space",
			src:               "> [a]: x.md\n>  \"t\"\n",
			wantDefs:          1,
			wantDefsUnlocated: 0,
		},
		{
			name:              "title on the next line, list item, tab",
			src:               "- [a]: x.md\n\t\"t\"\n",
			wantDefs:          0,
			wantDefsUnlocated: 1,
		},
		{
			name:              "title on the next line, list item, space",
			src:               "- [a]: x.md\n  \"t\"\n",
			wantDefs:          1,
			wantDefsUnlocated: 0,
		},
		{
			name:              "label folded across a line, blockquote, tab",
			src:               "> [a\n>\tb]: x.md\n",
			wantDefs:          0,
			wantDefsUnlocated: 1,
		},
		{
			name:              "label folded across a line, blockquote, space",
			src:               "> [a\n>  b]: x.md\n",
			wantDefs:          1,
			wantDefsUnlocated: 0,
		},
		{
			// The padding is leading whitespace the parser skips before it
			// reads a destination, so this one resolves. It is a good case
			// with a tab in it, which is the row a fix that keys off the
			// tab rather than the padding gets wrong.
			name:              "destination on the next line, blockquote, tab",
			src:               "> [a]:\n>\tx.md\n",
			wantDefs:          1,
			wantDefsUnlocated: 0,
		},
		{
			name:              "destination on the next line, blockquote, space",
			src:               "> [a]:\n>  x.md\n",
			wantDefs:          1,
			wantDefsUnlocated: 0,
		},
		{
			name:              "two definitions, one hidden by a tab",
			src:               "> [a]: x.md\n>\t\"t\"\n\n[b]: y.md\n",
			wantDefs:          1,
			wantDefsUnlocated: 1,
		},
		{
			name:              "no definition at all",
			src:               "plain text\n",
			wantDefs:          0,
			wantDefsUnlocated: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			if got := len(s.Definitions()); got != tc.wantDefs {
				t.Errorf("len(Definitions()) = %d, want %d", got, tc.wantDefs)
			}
			if got := s.UnlocatedDefinitions(); got != tc.wantDefsUnlocated {
				t.Errorf("UnlocatedDefinitions() = %d, want %d",
					got, tc.wantDefsUnlocated)
			}
		})
	}
}

// TestUnlocated_LocatedDefinitionsStillCarryTheirParts is the other half of
// the good case: a located definition's spans must be untouched by the
// counting, so a change that reports a count by widening or dropping a span
// fails here rather than passing quietly.
//
// It is a PRESERVED-BEHAVIOR PIN. It passes on the implementation before the
// count as well, which is the point — nothing about the spans changed.
func TestUnlocated_LocatedDefinitionsStillCarryTheirParts(t *testing.T) {
	const src = "> [a]: x.md\n>  \"t\"\n"
	s := markdown.NewSource([]byte(src))
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
		{"Span", "[a]: x.md\n>  \"t\"", got.Span},
		{"Label", "a", got.Label},
		{"Dest", "x.md", got.Dest},
		{"Title", "t", got.Title},
	} {
		if slice := src[part.span.Start:part.span.Stop]; slice != part.want {
			t.Errorf("%s selects %q, want %q", part.name, slice, part.want)
		}
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
			s := markdown.NewSource([]byte("> [a]: x.md\n>\t\"t\"\n"))
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
