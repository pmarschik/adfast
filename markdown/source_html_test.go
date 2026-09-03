package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// htmlBlockCases pin the whole-line rule over the shapes a pattern hunting
// for `<!--` and the next `-->` gets wrong.
var htmlBlockCases = []codeSpanCase{{
	name: "a comment on its own line",
	src:  "<!-- https://a.example -->\n",
	want: []string{"<!-- https://a.example -->\n"},
}, {
	// CommonMark puts the rest of the closing line inside the block, so the
	// URL after the closer is not prose either.
	name: "text after the closer is part of the block",
	src:  "<!-- x --> https://a.example\n",
	want: []string{"<!-- x --> https://a.example\n"},
}, {
	name: "a comment across lines is one block",
	src:  "<!--\nhttps://a.example\n-->\n",
	want: []string{"<!--\nhttps://a.example\n-->\n"},
}, {
	// Two blocks, not one: a pattern reading to the next `-->` would call
	// the pair a single span and be right by accident here, and wrong the
	// moment prose sits between them.
	name: "two comment lines are two blocks",
	src:  "<!-- a -->\n<!-- b -->\n",
	want: []string{"<!-- a -->\n", "<!-- b -->\n"},
}, {
	name: "a tag block",
	src:  "<div>\nhttps://a.example\n</div>\n",
	want: []string{"<div>\nhttps://a.example\n</div>\n"},
}, {
	// A blank line ends the block, so the paragraph between the tags IS
	// prose and no span may cover it.
	name: "a blank line ends the block",
	src:  "<div>\n\nhttps://a.example\n\n</div>\n",
	want: []string{"<div>\n", "</div>\n"},
}, {
	name: "inside a blockquote the span reaches the line start",
	src:  "> <!-- https://a.example -->\n",
	want: []string{"> <!-- https://a.example -->\n"},
}, {
	name: "inside a list item the span reaches the line start",
	src:  "- <!-- https://a.example -->\n",
	want: []string{"- <!-- https://a.example -->\n"},
}, {
	name: "an unterminated last line",
	src:  "<!-- https://a.example -->",
	want: []string{"<!-- https://a.example -->"},
}, {
	// A comment written inside a sentence is inline HTML, and this view
	// says nothing about it. InlineHTMLSpans is that view.
	name: "a comment inside prose is not a block",
	src:  "Text <!-- c --> more\n",
	want: nil,
}, {
	name: "a comment inside a fenced block is code, not HTML",
	src:  "```\n<!-- https://a.example -->\n```\n",
	want: nil,
}, {
	name: "a document with no HTML",
	src:  "# t\n\ntext https://a.example\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestHTMLSpans_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range htmlBlockCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.HTMLSpans([]byte(c.src))
			if !eq(texts(c.src, got), c.want) {
				t.Errorf("HTMLSpans(%q)\n got %q\nwant %q", c.src, texts(c.src, got), c.want)
			}
		})
	}
}

// inlineHTMLCases pin the tight extent: the tag or the comment, and never
// the words around or between them.
var inlineHTMLCases = []codeSpanCase{{
	name: "a comment inside prose",
	src:  "Text <!-- https://a.example --> more\n",
	want: []string{"<!-- https://a.example -->"},
}, {
	// The content between the tags is prose and is deliberately NOT
	// covered: a rewriter may touch the words and not the tags.
	name: "a tag pair reports the tags and not their content",
	src:  "Text <span>https://a.example</span> more\n",
	want: []string{"<span>", "</span>"},
}, {
	name: "a tag across a line break is one span",
	src:  "Text <span\nclass=\"a\">b</span>\n",
	want: []string{"<span\nclass=\"a\">", "</span>"},
}, {
	name: "inside a table cell",
	src:  "| <!-- c --> | b |\n| --- | --- |\n| c | d |\n",
	want: []string{"<!-- c -->"},
}, {
	name: "inside a link label",
	src:  "[a <b> c](x.md)\n",
	want: []string{"<b>"},
}, {
	name: "a tag inside inline code is code, not HTML",
	src:  "`<span>` and <em>\n",
	want: []string{"<em>"},
}, {
	name: "a lone angle bracket opens nothing",
	src:  "a < b and 3 < 4\n",
	want: nil,
}, {
	// An autolink is its own node, not raw HTML, even though it is written
	// in angle brackets.
	name: "an autolink is not inline HTML",
	src:  "See <https://a.example>\n",
	want: nil,
}, {
	name: "a comment on its own line is a block, not inline",
	src:  "<!-- c -->\n",
	want: nil,
}, {
	name: "a document with no HTML",
	src:  "# t\n\ntext https://a.example\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestInlineHTMLSpans_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range inlineHTMLCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.InlineHTMLSpans([]byte(c.src))
			if !eq(texts(c.src, got), c.want) {
				t.Errorf("InlineHTMLSpans(%q)\n got %q\nwant %q", c.src, texts(c.src, got), c.want)
			}
		})
	}
}

// TestHTMLSpans_ContractHolds checks the ordering and non-overlap the Spans
// binary searches depend on, for both halves of the surface.
func TestHTMLSpans_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "<!-- a -->\n<!-- b -->\n\n> <div>\n> x\n> </div>\n\n" +
		"p <span>t</span> and <!-- c --> q\n\n- <br>\n\n<div>\n\ny\n\n</div>\n"
	s := markdown.NewSource([]byte(src))
	for name, spans := range map[string]markdown.Spans{
		"HTMLSpans":       s.HTMLSpans(),
		"InlineHTMLSpans": s.InlineHTMLSpans(),
	} {
		if len(spans) == 0 {
			t.Errorf("%s: no spans", name)
		}
		for i, sp := range spans {
			if sp.Start < 0 || sp.Stop > len(src) || sp.Start >= sp.Stop {
				t.Errorf("%s: span %d = %v is not a range of a %d byte source", name, i, sp, len(src))
			}
			if i > 0 && sp.Start < spans[i-1].Stop {
				t.Errorf("%s: span %d = %v overlaps %v", name, i, sp, spans[i-1])
			}
		}
	}
}

// TestHTMLSpans_GuardsAURLRewriter is the call site's question: which URLs of
// a document may a rewriter touch. The HTML views answer the half the code
// views cannot, and a URL an author commented out is the case that made them
// necessary — the edit there is invisible in the rendered output.
func TestHTMLSpans_GuardsAURLRewriter(t *testing.T) {
	t.Parallel()
	src := "See http://prose.example and <!-- http://inline.example --> too.\n\n" +
		"<!-- http://block.example -->\n"
	s := markdown.NewSource([]byte(src))
	block, inline := s.HTMLSpans(), s.InlineHTMLSpans()
	guarded := func(needle string) bool {
		at := strings.Index(src, needle)
		if at < 0 {
			t.Fatalf("fixture lost %q", needle)
		}
		return block.Contains(at) || inline.Contains(at)
	}
	if guarded("http://prose.example") {
		t.Error("the prose URL is guarded, so a rewriter would leave it alone")
	}
	for _, needle := range []string{"http://inline.example", "http://block.example"} {
		if !guarded(needle) {
			t.Errorf("%s is HTML and no span covers it", needle)
		}
	}
}
