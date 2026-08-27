package markdown_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// headingCase is one source plus the exact text every heading must select.
// "level|block|text" per heading keeps a failure readable — the block and
// the text extents are the two things that can independently go wrong.
type headingCase struct {
	name string
	src  string
	want []string
}

func headingTexts(src string, hs []markdown.Heading) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, strings.Join([]string{
			strconv.Itoa(h.Level),
			src[h.Span.Start:h.Span.Stop],
			src[h.Text.Start:h.Text.Stop],
		}, "|"))
	}
	return out
}

var headingCases = []headingCase{{
	name: "an ATX heading",
	src:  "# Title\n",
	want: []string{"1|# Title\n|Title"},
}, {
	name: "a closing run is not text",
	src:  "## Two ##\n",
	want: []string{"2|## Two ##\n|Two"},
}, {
	// The line scanners this replaces never saw setext headings at all.
	name: "a setext heading covers its underline",
	src:  "Setext\n======\n",
	want: []string{"1|Setext\n======\n|Setext"},
}, {
	name: "a setext level two heading",
	src:  "Sub\n---\n",
	want: []string{"2|Sub\n---\n|Sub"},
}, {
	// A '#' run with no space after it opens nothing, so this is a setext
	// heading whose text starts with a '#'. Reading the run as an ATX
	// marker would leave the underline outside the span.
	name: "a setext heading whose text starts with a hash",
	src:  "#foo\n---\n",
	want: []string{"2|#foo\n---\n|#foo"},
}, {
	// The mirror image: a real ATX heading followed by a thematic break.
	// The break is NOT part of the heading.
	name: "an ATX heading followed by a thematic break",
	src:  "# a\n---\n",
	want: []string{"1|# a\n|a"},
}, {
	name: "a multi-line setext heading",
	src:  "One\ntwo\n---\n",
	want: []string{"2|One\ntwo\n---\n|One\ntwo"},
}, {
	name: "indentation and padding are outside the text",
	src:  "   ###   spaced   \n",
	want: []string{"3|   ###   spaced   \n|spaced"},
}, {
	name: "a heading in a blockquote keeps the prefix in the block span",
	src:  "> # quoted\n",
	want: []string{"1|> # quoted\n|quoted"},
}, {
	name: "a setext heading in a blockquote",
	src:  "> Quoted\n> ---\n",
	want: []string{"2|> Quoted\n> ---\n|Quoted"},
}, {
	name: "a heading in a list item keeps the marker in the block span",
	src:  "- # in list\n",
	want: []string{"1|- # in list\n|in list"},
}, {
	// The whole point of the view: goldmark decides, so a heading-looking
	// line inside a fence is not a heading.
	name: "a hash line inside a fence is not a heading",
	src:  "```\n# not a heading\n```\n",
	want: nil,
}, {
	name: "a hash line inside an indented block is not a heading",
	src:  "text\n\n    # not a heading\n",
	want: nil,
}, {
	name: "a hash with no space is not a heading",
	src:  "#no-space\n",
	want: nil,
}, {
	name: "an empty ATX heading has an empty text at the insertion point",
	src:  "#\n",
	want: []string{"1|#\n|"},
}, {
	name: "an empty ATX heading with padding",
	src:  "## \n",
	want: []string{"2|## \n|"},
}, {
	name: "inline markup stays in the text verbatim",
	src:  "# *em* and `code`\n",
	want: []string{"1|# *em* and `code`\n|*em* and `code`"},
}, {
	name: "a setext heading with inline markup",
	src:  "Setext with *em*\n---\n",
	want: []string{"2|Setext with *em*\n---\n|Setext with *em*"},
}, {
	name: "a heading without a trailing newline",
	src:  "# last",
	want: []string{"1|# last|last"},
}, {
	name: "a setext heading without a trailing newline",
	src:  "last\n===",
	want: []string{"1|last\n===|last"},
}, {
	name: "every level",
	src:  "# a\n\n## b\n\n### c\n\n#### d\n\n##### e\n\n###### f\n",
	want: []string{
		"1|# a\n|a", "2|## b\n|b", "3|### c\n|c",
		"4|#### d\n|d", "5|##### e\n|e", "6|###### f\n|f",
	},
}, {
	name: "seven hashes is not a heading",
	src:  "####### g\n",
	want: nil,
}, {
	name: "a document with no headings",
	src:  "text\n\nmore\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestHeadings_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range headingCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Headings([]byte(c.src))
			if !eq(headingTexts(c.src, got), c.want) {
				t.Errorf("Headings(%q)\n got %q\nwant %q", c.src, headingTexts(c.src, got), c.want)
			}
		})
	}
}

// TestHeadings_ContractHolds pins document order, non-overlap, and the
// containment of Text in Span over one document holding every awkward
// shape at once.
func TestHeadings_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "# one\n\nSetext\n===\n\n> ## quoted\n\n- ### item\n\n```\n# fenced\n```\n\nlast\n---\n"
	hs := markdown.Headings([]byte(src))
	if len(hs) != 5 {
		t.Fatalf("got %d headings, want 5: %q", len(hs), headingTexts(src, hs))
	}
	prev := 0
	for i, h := range hs {
		if h.Span.Start < prev {
			t.Errorf("heading %d = %v is out of document order", i, h.Span)
		}
		prev = h.Span.Stop
		if h.Span.Start < 0 || h.Span.Stop > len(src) || h.Span.Len() <= 0 {
			t.Errorf("heading %d span %v is not a range of a %d byte source", i, h.Span, len(src))
		}
		if h.Text.Start < h.Span.Start || h.Text.Stop > h.Span.Stop {
			t.Errorf("heading %d text %v is not inside its block %v", i, h.Text, h.Span)
		}
		if h.Level < 1 || h.Level > 6 {
			t.Errorf("heading %d level = %d", i, h.Level)
		}
	}
}

// TestHeadings_DoNotOverlapCodeSpans is the composition guarantee that
// matters: the two views come from ONE tree, so a fenced block's contents
// can never also be a heading. Two independent scanners would disagree
// here, which is what this surface exists to prevent.
func TestHeadings_DoNotOverlapCodeSpans(t *testing.T) {
	t.Parallel()
	src := []byte("# real\n\n```\n# fake\n```\n\n    # also fake\n\n## real too\n")
	s := markdown.NewSource(src)
	code := s.CodeSpans()
	hs := s.Headings()
	if len(hs) != 2 {
		t.Fatalf("got %d headings, want 2: %q", len(hs), headingTexts(string(src), hs))
	}
	for _, h := range hs {
		if code.Overlaps(h.Span) {
			t.Errorf("heading %q overlaps a code span", src[h.Span.Start:h.Span.Stop])
		}
	}
}

// TestHeadings_Memoize pins that a second call is the same slice, i.e.
// that the view is computed once per Source rather than per call.
func TestHeadings_Memoize(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("# a\n\n## b\n"))
	first, second := s.Headings(), s.Headings()
	if len(first) != 2 || len(second) != len(first) {
		t.Fatalf("Headings = %v then %v", first, second)
	}
	if &first[0] != &second[0] {
		t.Error("Headings recomputed on the second call")
	}
}

// TestHeadings_SpliceComposesInOneParse is the shape 17uu takes: read the
// heading text verbatim, decide policy over it, hand an Edit back. The
// caller never indexes a byte and never re-derives an offset.
func TestHeadings_SpliceComposesInOneParse(t *testing.T) {
	t.Parallel()
	const src = "Old *title*\n===========\n\nbody\n\n```\n# not a heading\n```\n\n## Section\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, h := range s.Headings() {
		if h.Level != 1 {
			continue
		}
		if got := string(s.Text(h.Text)); got != "Old *title*" {
			t.Fatalf("H1 text = %q, want the raw written text", got)
		}
		edits = append(edits, markdown.Edit{Span: h.Text, Text: "New *title*"})
	}
	if len(edits) != 1 {
		t.Fatalf("found %d H1s, want 1", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "New *title*\n===========\n\nbody\n\n```\n# not a heading\n```\n\n## Section\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}
