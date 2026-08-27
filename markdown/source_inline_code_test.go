package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// inlineCodeCases pin the written extent — delimiters included — over the
// shapes a backtick-parity count over a line gets wrong.
var inlineCodeCases = []codeSpanCase{{
	name: "one span in prose",
	src:  "text `code` more\n",
	want: []string{"`code`"},
}, {
	name: "two spans on one line",
	src:  "`a` and `b`\n",
	want: []string{"`a`", "`b`"},
}, {
	// A parity count reads the third backtick as an opener and calls the
	// rest of the line code.
	name: "a double-backtick run closes only a double run",
	src:  "``a `b` c`` d\n",
	want: []string{"``a `b` c``"},
}, {
	// The parser trims one space at each end, so the content it keeps is
	// "`", and the span still has to reach the delimiters around it.
	name: "a span whose content is a backtick",
	src:  "`` ` ``\n",
	want: []string{"`` ` ``"},
}, {
	name: "a leading space is content when the trailing one is missing",
	src:  "` a` b\n",
	want: []string{"` a`"},
}, {
	name: "a blank span is not trimmed",
	src:  "a `  ` b\n",
	want: []string{"`  `"},
}, {
	// The count never leaves the line, so it calls the second backtick an
	// unclosed opener and every following line code.
	name: "a span across a line break",
	src:  "text `a\nb` more\n",
	want: []string{"`a\nb`"},
}, {
	name: "an escaped backtick opens nothing",
	src:  "a \\`b\\` c `d` e\n",
	want: []string{"`d`"},
}, {
	// An unclosed run is literal text, which a parity count reads as code
	// running to the end of the document.
	name: "an unclosed run is not a span",
	src:  "text ` http://a.example more\n",
	want: nil,
}, {
	name: "inline code inside a fenced block is not inline code",
	src:  "```\n`a`\n```\n",
	want: nil,
}, {
	name: "inside a blockquote",
	src:  "> a `b` c\n",
	want: []string{"`b`"},
}, {
	name: "inside a heading",
	src:  "# a `b`\n",
	want: []string{"`b`"},
}, {
	name: "inside a link label",
	src:  "[a `b`](x.md)\n",
	want: []string{"`b`"},
}, {
	name: "a document with no inline code",
	src:  "# t\n\ntext http://a.example\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestInlineCodeSpans_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range inlineCodeCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.InlineCodeSpans([]byte(c.src))
			if !eq(texts(c.src, got), c.want) {
				t.Errorf("InlineCodeSpans(%q)\n got %q\nwant %q", c.src, texts(c.src, got), c.want)
			}
		})
	}
}

// TestInlineCodeSpans_ContractHolds checks the ordering and non-overlap the
// Spans binary searches depend on.
func TestInlineCodeSpans_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "`a` b `c`\n\n> `d`\n\n- `e` and ``f `g` h``\n\n# `i`\n"
	spans := markdown.InlineCodeSpans([]byte(src))
	if len(spans) == 0 {
		t.Fatal("no spans")
	}
	for i, s := range spans {
		if s.Start < 0 || s.Stop > len(src) || s.Start >= s.Stop {
			t.Errorf("span %d = %v is not a range of a %d byte source", i, s, len(src))
		}
		if i > 0 && s.Start < spans[i-1].Stop {
			t.Errorf("span %d = %v overlaps %v", i, s, spans[i-1])
		}
	}
}

// TestInlineCodeSpans_GuardsAURLRewriter is the call site's question: which
// URLs of a document may a rewriter touch. The two views answer it together,
// and neither answers it alone.
func TestInlineCodeSpans_GuardsAURLRewriter(t *testing.T) {
	t.Parallel()
	src := "See http://prose.example and `http://inline.example`.\n\n    http://indented.example\n"
	code := markdown.CodeSpans([]byte(src))
	inline := markdown.InlineCodeSpans([]byte(src))
	guarded := func(needle string) bool {
		at := strings.Index(src, needle)
		if at < 0 {
			t.Fatalf("fixture lost %q", needle)
		}
		return code.Contains(at) || inline.Contains(at)
	}
	if guarded("http://prose.example") {
		t.Error("the prose URL is guarded, so a rewriter would leave it alone")
	}
	for _, needle := range []string{"http://inline.example", "http://indented.example"} {
		if !guarded(needle) {
			t.Errorf("%s is code and no span covers it", needle)
		}
	}
}
