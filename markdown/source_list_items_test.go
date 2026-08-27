package markdown_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// listItemCase is one source plus the exact shape every item must report.
// "depth|ordered|block|text" per item keeps a failure readable — the depth,
// the block extent, and the content extent are the three things that can
// independently go wrong.
type listItemCase struct {
	name string
	src  string
	want []string
}

func listItemTexts(src string, items []markdown.ListItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, strings.Join([]string{
			strconv.Itoa(it.Depth),
			strconv.FormatBool(it.Ordered),
			src[it.Span.Start:it.Span.Stop],
			src[it.Text.Start:it.Text.Stop],
		}, "|"))
	}
	return out
}

var listItemCases = []listItemCase{{
	name: "a bullet list",
	src:  "- a\n- b\n",
	want: []string{"0|false|- a\n|a", "0|false|- b\n|b"},
}, {
	name: "a star list",
	src:  "* one\n* two\n",
	want: []string{"0|false|* one\n|one", "0|false|* two\n|two"},
}, {
	name: "a plus list",
	src:  "+ plus\n",
	want: []string{"0|false|+ plus\n|plus"},
}, {
	// A line scanner keyed on "- " and "* " sees nothing here at all.
	name: "an ordered list",
	src:  "1. one\n2. two\n",
	want: []string{"0|true|1. one\n|one", "0|true|2. two\n|two"},
}, {
	name: "an ordered list with a paren marker and a wide number",
	src:  "10) ten\n",
	want: []string{"0|true|10) ten\n|ten"},
}, {
	// The whole point of the view: goldmark decides, so a bullet quoted
	// inside a fence is not an item. A field read from these lines would
	// write a value the document never stated.
	name: "a bullet inside a fence is not an item",
	src:  "```\n- not an item\n```\n",
	want: nil,
}, {
	name: "a bullet inside an indented block is not an item",
	src:  "text\n\n    - not an item\n",
	want: nil,
}, {
	// Four spaces past the item's own content column is code, not a
	// sub-item, however much it looks like one.
	name: "a bullet inside a code block nested in an item",
	src:  "- outer\n\n      - not an item\n",
	want: []string{"0|false|- outer\n\n      - not an item\n|outer\n\n      - not an item\n"},
}, {
	name: "a bullet inside a fence nested in an item",
	src:  "- outer\n\n  ```\n  - not an item\n  ```\n",
	want: []string{"0|false|- outer\n\n  ```\n  - not an item\n  ```\n|outer\n\n  ```\n  - not an item\n  ```"},
}, {
	// CommonMark reads this as a thematic break, not as an item holding
	// "- -".
	name: "a spaced dash run is a thematic break",
	src:  "- - -\n",
	want: nil,
}, {
	name: "a marker with no space opens nothing",
	src:  "-item\n",
	want: nil,
}, {
	name: "nested lists count their depth",
	src:  "- a\n  - b\n    - c\n",
	want: []string{
		"0|false|- a\n  - b\n    - c\n|a\n  - b\n    - c",
		"1|false|  - b\n    - c\n|b\n    - c",
		"2|false|    - c\n|c",
	},
}, {
	// A container between the two lists does not reset the depth: the inner
	// item is still a sub-item of the outer one.
	name: "a list quoted inside an item is still nested",
	src:  "- a\n  > - b\n",
	want: []string{
		"0|false|- a\n  > - b\n|a\n  > - b",
		"1|false|  > - b\n|b",
	},
}, {
	name: "a list inside a blockquote is depth zero",
	src:  "> - quoted\n",
	want: []string{"0|false|> - quoted\n|quoted"},
}, {
	// The continuation indent is written between the two lines, so it is
	// inside Text — the same as-written rule a code span crossing a line
	// follows.
	name: "a continuation line is part of the item",
	src:  "- first line\n  second line\n",
	want: []string{"0|false|- first line\n  second line\n|first line\n  second line"},
}, {
	name: "a second paragraph is part of the item",
	src:  "- a\n\n  para two\n- b\n",
	want: []string{
		"0|false|- a\n\n  para two\n|a\n\n  para two",
		"0|false|- b\n|b",
	},
}, {
	// goldmark consumes the closing fence without recording it, so both
	// spans have to reach past it deliberately.
	name: "an item ending in a fence covers the closing fence",
	src:  "- a\n\n  ```\n  x\n  ```\n\nafter\n",
	want: []string{"0|false|- a\n\n  ```\n  x\n  ```\n|a\n\n  ```\n  x\n  ```"},
}, {
	name: "an item whose content starts on the next line",
	src:  "-\n  content\n",
	want: []string{"0|false|-\n  content\n|content"},
}, {
	name: "an empty item has an empty text at the insertion point",
	src:  "-\n- b\n",
	want: []string{"0|false|-\n|", "0|false|- b\n|b"},
}, {
	name: "an empty item with padding",
	src:  "- \n",
	want: []string{"0|false|- \n|"},
}, {
	name: "a task checkbox is content, not marker",
	src:  "- [x] task\n",
	want: []string{"0|false|- [x] task\n|[x] task"},
}, {
	name: "inline markup stays in the text verbatim",
	src:  "- *em* and `code`\n",
	want: []string{"0|false|- *em* and `code`\n|*em* and `code`"},
}, {
	name: "a wide marker gap is outside the text",
	src:  "-   wide gap\n",
	want: []string{"0|false|-   wide gap\n|wide gap"},
}, {
	name: "an item without a trailing newline",
	src:  "- last",
	want: []string{"0|false|- last|last"},
}, {
	// A list interrupting a paragraph takes the line below as a lazy
	// continuation, so "more" belongs to the first item.
	name: "a lazy continuation belongs to the item above it",
	src:  "some text\n- item\nmore\n- other\n",
	want: []string{
		"0|false|- item\nmore\n|item\nmore",
		"0|false|- other\n|other",
	},
}, {
	name: "a document with no list",
	src:  "text\n\nmore\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestListItems_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range listItemCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.ListItems([]byte(c.src))
			if !eq(listItemTexts(c.src, got), c.want) {
				t.Errorf("ListItems(%q)\n got %q\nwant %q", c.src, listItemTexts(c.src, got), c.want)
			}
		})
	}
}

// TestListItems_ContractHolds pins document order, the containment of Text
// in Span, and the containment of a nested item in the item above it, over
// one document holding every awkward shape at once.
func TestListItems_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "- one\n  - sub\n\n1. ordered\n\n> - quoted\n\n```\n- fenced\n```\n\n- last\n"
	items := markdown.ListItems([]byte(src))
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5: %q", len(items), listItemTexts(src, items))
	}
	prev := -1
	for i, it := range items {
		if it.Span.Start < prev {
			t.Errorf("item %d = %v is out of document order", i, it.Span)
		}
		prev = it.Span.Start
		if it.Span.Start < 0 || it.Span.Stop > len(src) || it.Span.Len() <= 0 {
			t.Errorf("item %d span %v is not a range of a %d byte source", i, it.Span, len(src))
		}
		if it.Text.Start < it.Span.Start || it.Text.Stop > it.Span.Stop {
			t.Errorf("item %d text %v is not inside its block %v", i, it.Text, it.Span)
		}
		if it.Depth < 0 {
			t.Errorf("item %d depth = %d", i, it.Depth)
		}
	}
	// The only overlap this view produces is containment: item 1 is the
	// sub-item of item 0.
	if items[1].Depth != 1 || items[1].Span.Start < items[0].Span.Start || items[1].Span.Stop > items[0].Span.Stop {
		t.Errorf("sub-item %v is not contained in its parent %v", items[1], items[0])
	}
}

// TestListItems_DoNotOverlapCodeSpans is the composition guarantee that
// matters: the two views come from ONE tree, so a fenced or an indented
// block's contents can never also be a list item. Two independent scanners
// would disagree here, which is what this surface exists to prevent.
func TestListItems_DoNotOverlapCodeSpans(t *testing.T) {
	t.Parallel()
	src := []byte("- real\n\n```\n- fake\n```\n\ntext\n\n    - also fake\n\n- real too\n")
	s := markdown.NewSource(src)
	code := s.CodeSpans()
	items := s.ListItems()
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %q", len(items), listItemTexts(string(src), items))
	}
	for _, it := range items {
		if code.Overlaps(it.Text) {
			t.Errorf("item %q overlaps a code span", src[it.Text.Start:it.Text.Stop])
		}
	}
}

// TestListItems_EmptyItemTextIsTheInsertionPoint pins WHERE an empty item's
// Text sits, which the coverage table cannot see: an empty span slices to ""
// wherever it is. The offset is the whole value of reporting an empty item at
// all, so it is pinned through Apply, the only sanctioned consumer of a span.
func TestListItems_EmptyItemTextIsTheInsertionPoint(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"- \n":     "- x\n",
		"-\n- b\n": "-x\n- b\n",
		"1.\n":     "1.x\n",
	}
	for src, want := range cases {
		t.Run(strconv.Quote(src), func(t *testing.T) {
			t.Parallel()
			s := markdown.NewSource([]byte(src))
			items := s.ListItems()
			if len(items) == 0 {
				t.Fatalf("ListItems(%q) found nothing", src)
			}
			if items[0].Text.Len() != 0 {
				t.Fatalf("item text = %q, want empty", src[items[0].Text.Start:items[0].Text.Stop])
			}
			got, err := s.Apply(markdown.Edit{Span: items[0].Text, Text: "x"})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(got) != want {
				t.Errorf("Apply = %q, want %q", got, want)
			}
		})
	}
}

// TestListItems_Memoize pins that a second call is the same slice, i.e.
// that the view is computed once per Source rather than per call.
func TestListItems_Memoize(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("- a\n- b\n"))
	first, second := s.ListItems(), s.ListItems()
	if len(first) != 2 || len(second) != len(first) {
		t.Fatalf("ListItems = %v then %v", first, second)
	}
	if &first[0] != &second[0] {
		t.Error("ListItems recomputed on the second call")
	}
}

// TestListItems_SpliceComposesInOneParse is the shape a rewriting consumer
// takes: read each item's text verbatim, decide policy over it, hand an Edit
// back. The caller never indexes a byte and never re-derives an offset.
func TestListItems_SpliceComposesInOneParse(t *testing.T) {
	t.Parallel()
	const src = "- keep\n- *drop me*\n\n```\n- drop me\n```\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, it := range s.ListItems() {
		if !strings.Contains(string(s.Text(it.Text)), "drop me") {
			continue
		}
		edits = append(edits, markdown.Edit{Span: it.Text, Text: "replaced"})
	}
	if len(edits) != 1 {
		t.Fatalf("found %d items to rewrite, want 1", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "- keep\n- replaced\n\n```\n- drop me\n```\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}
