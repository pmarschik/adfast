package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// autolinkCase is one source plus the exact text and flags every autolink
// must carry. "whole|text|target|bare|email" per autolink keeps a failure
// readable: the written extent, the address inside it, where it points, and
// the two verdicts a caller's policy turns on can each go wrong on their
// own.
type autolinkCase struct {
	name string
	src  string
	want []string
}

func autolinkTexts(src string, links []markdown.Autolink) []string {
	out := make([]string, 0, len(links))
	for _, a := range links {
		out = append(out, strings.Join([]string{
			src[a.Span.Start:a.Span.Stop],
			src[a.Text.Start:a.Text.Stop],
			a.Target,
			map[bool]string{true: "bare", false: "angle"}[a.Bare],
			map[bool]string{true: "email", false: "url"}[a.Email],
		}, "|"))
	}
	return out
}

// autolinkFormCases pin the two written forms and the extents that tell
// them apart. The parser records ONE offset for both — the byte before the
// address — so a view that trusted it without reading the source back would
// report the angle form's `<` as part of an address and the bare form's
// preceding space as part of one.
var autolinkFormCases = []autolinkCase{{
	name: "an angle autolink excludes its brackets from Text",
	src:  "See <https://a.example> here\n",
	want: []string{"<https://a.example>|https://a.example|https://a.example|angle|url"},
}, {
	name: "a bare autolink spans exactly the address",
	src:  "See https://a.example here\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	// The fragment-head case: at offset 0 there is no byte before the
	// address, so the recorded offset IS the address and a view assuming
	// "one before" would start inside it.
	name: "a bare autolink at offset zero",
	src:  "https://a.example\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "an angle email autolink",
	src:  "Mail <a@b.example> now\n",
	want: []string{"<a@b.example>|a@b.example|a@b.example|angle|email"},
}, {
	name: "a bare email autolink",
	src:  "Mail a@b.example now\n",
	want: []string{"a@b.example|a@b.example|a@b.example|bare|email"},
}, {
	// The one shape where Target is not the written text. A caller that
	// wrapped Text in angle brackets here would write "<www.a.example>",
	// a link to a relative path — which is why Target is reported.
	name: "a www autolink targets a completed scheme",
	src:  "See www.a.example here\n",
	want: []string{"www.a.example|www.a.example|http://www.a.example|bare|url"},
}, {
	// goldmark linkifies "ftp://" and GFM does not. The view reports the
	// parser's verdict and leaves the policy to the caller, which needs
	// the written scheme in Text to apply one.
	name: "an ftp autolink is reported with its written scheme",
	src:  "See ftp://a.example/f here\n",
	want: []string{"ftp://a.example/f|ftp://a.example/f|ftp://a.example/f|bare|url"},
}, {
	name: "both forms in one paragraph, in document order",
	src:  "See <https://a.example>, and https://b.example.\n",
	want: []string{
		"<https://a.example>|https://a.example|https://a.example|angle|url",
		"https://b.example|https://b.example|https://b.example|bare|url",
	},
}, {
	name: "two angle autolinks side by side",
	src:  "See <https://a.example> <https://b.example>\n",
	want: []string{
		"<https://a.example>|https://a.example|https://a.example|angle|url",
		"<https://b.example>|https://b.example|https://b.example|angle|url",
	},
}, {
	name: "an email and a URL autolink in one paragraph",
	src:  "See <a@b.example> and <https://a.example>\n",
	want: []string{
		"<a@b.example>|a@b.example|a@b.example|angle|email",
		"<https://a.example>|https://a.example|https://a.example|angle|url",
	},
}, {
	name: "an autolink in a heading",
	src:  "# See https://a.example\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "an autolink in a list item",
	src:  "- item https://a.example\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "an autolink in a blockquote, both forms",
	src:  "> quoted <https://a.example> and https://b.example\n",
	want: []string{
		"<https://a.example>|https://a.example|https://a.example|angle|url",
		"https://b.example|https://b.example|https://b.example|bare|url",
	},
}, {
	name: "an autolink in a table cell",
	src:  "| <https://a.example> | b |\n| --- | --- |\n| c | d |\n",
	want: []string{"<https://a.example>|https://a.example|https://a.example|angle|url"},
}, {
	name: "a document with no autolink",
	src:  "# t\n\nno urls here\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestAutolinks_Forms(t *testing.T) {
	t.Parallel()
	for _, c := range autolinkFormCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Autolinks([]byte(c.src))
			if !eq(autolinkTexts(c.src, got), c.want) {
				t.Errorf("Autolinks(%q) =\n %q\nwant\n %q",
					c.src, autolinkTexts(c.src, got), c.want)
			}
			if u := markdown.NewSource([]byte(c.src)).UnlocatedAutolinks(); u != 0 {
				t.Errorf("UnlocatedAutolinks() = %d, want 0", u)
			}
		})
	}
}

// autolinkBoundaryCases are the shapes a pattern over the raw source gets
// wrong in one direction or the other. They are the reason this view exists
// at all: a consumer steering a character class cannot reach the parser's
// answer here, however the class is written.
var autolinkBoundaryCases = []autolinkCase{{
	// A trailing comma is the sentence's, not the URL's. A class that
	// accepts "," reports an address the author never wrote.
	name: "a trailing comma is not part of the address",
	src:  "Read https://a.example/x, not y\n",
	want: []string{"https://a.example/x|https://a.example/x|https://a.example/x|bare|url"},
}, {
	name: "a sentence-ending period is not part of the address",
	src:  "Ends https://a.example/x.\n",
	want: []string{"https://a.example/x|https://a.example/x|https://a.example/x|bare|url"},
}, {
	// Trimming is not blanket, which is the other half of the same point:
	// a BALANCED paren pair inside a URL belongs to it. A fix that only
	// trimmed harder would cut this address short.
	name: "a balanced paren inside a URL is part of the address",
	src:  "paren https://a.example/a(b) end\n",
	want: []string{
		"https://a.example/a(b)|https://a.example/a(b)|https://a.example/a(b)|bare|url",
	},
}, {
	// No whitespace in front of the address. A pattern requiring a space
	// or a line start misses this one entirely.
	name: "a URL flush against an inline tag is an autolink",
	src:  "Text <span>https://a.example</span> more\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "a URL flush against an emphasis marker is an autolink",
	src:  "emph *https://a.example* end\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	// goldmark's linkifier requires a dotted domain, so a dotless host is
	// not an autolink at all. A caller mirroring an implementation that
	// linkifies it has to know that, and a silent empty view is how it
	// finds out — hence the count assertion in the runner.
	name: "a dotless host is not an autolink",
	src:  "See https://localhost:8080/x end\n",
	want: nil,
}, {
	name: "a URL in a link label is not an autolink",
	src:  "[see https://a.example](https://b.example)\n",
	want: nil,
}, {
	name: "a URL in image alt text is not an autolink",
	src:  "![alt https://a.example](i.png)\n",
	want: nil,
}, {
	name: "a URL in inline code is not an autolink",
	src:  "See `https://a.example` here\n",
	want: nil,
}, {
	name: "a URL in a fenced block is not an autolink",
	src:  "```\nhttps://a.example\n```\n",
	want: nil,
}, {
	name: "a URL in an HTML comment is not an autolink",
	src:  "<!-- https://a.example -->\n",
	want: nil,
}, {
	name: "a URL in an HTML block is not an autolink",
	src:  "<div>\nhttps://a.example\n</div>\n",
	want: nil,
}, {
	name: "a definition destination is not an autolink",
	src:  "See [s].\n\n[s]: https://a.example\n",
	want: nil,
}}

func TestAutolinks_Boundaries(t *testing.T) {
	t.Parallel()
	for _, c := range autolinkBoundaryCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := markdown.NewSource([]byte(c.src))
			if got := autolinkTexts(c.src, s.Autolinks()); !eq(got, c.want) {
				t.Errorf("Autolinks(%q) =\n %q\nwant\n %q", c.src, got, c.want)
			}
			// An empty view here must mean "no autolink", not "one I could
			// not place" — the distinction the count exists for.
			if u := s.UnlocatedAutolinks(); u != 0 {
				t.Errorf("UnlocatedAutolinks() = %d, want 0", u)
			}
		})
	}
}

// autolinkLabelCases cover the subtree every OTHER view in this package
// skips: a text directive's label, which the directive parser parses
// against a detached copy of its bytes. Under that root the parser's
// offsets address the copy, so reporting them unchanged would name bytes
// near the start of the document — and skipping the root, as the other
// views do, would lose the only autolink of a document like the first case
// here.
//
// Every want below is the label's absolute extent in the source, so a shift
// that is off by the bracket, or omitted, fails these rather than passing
// with plausible-looking spans.
var autolinkLabelCases = []autolinkCase{{
	name: "a bare autolink in a text directive label",
	src:  "Text :sup[see https://a.example] end\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "an angle autolink in a text directive label",
	src:  "Text :sup[<https://a.example>] end\n",
	want: []string{"<https://a.example>|https://a.example|https://a.example|angle|url"},
}, {
	// A label parsed inside a label: the recursion runs in each copy's own
	// coordinates and the shifts compose.
	name: "an autolink in a nested text directive label",
	src:  "Text :sup[a :sub[https://b.example] c] end\n",
	want: []string{"https://b.example|https://b.example|https://b.example|bare|url"},
}, {
	// The ordering claim: a shifted span has to sort with the unshifted
	// ones, not after them.
	name: "a label autolink and a prose autolink in document order",
	src:  "Text :sup[see https://a.example] and https://b.example\n",
	want: []string{
		"https://a.example|https://a.example|https://a.example|bare|url",
		"https://b.example|https://b.example|https://b.example|bare|url",
	},
}, {
	name: "a label autolink inside a blockquote",
	src:  "> q :sup[see https://a.example]\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	// A tab in the container prefix is what hides a definition from its
	// own view. It does not reach here: the label's offset is recovered
	// from the directive's extent in THIS source and verified against it.
	name: "a label autolink on a tab-prefixed continuation line",
	src:  "> q\n>\t:sup[see https://a.example]\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "a label autolink in a list item continuation",
	src:  "- i\n\t:sup[see https://a.example]\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	name: "a label autolink in a table cell",
	src:  "| :sup[https://a.example] | b |\n| --- | --- |\n| c | d |\n",
	want: []string{"https://a.example|https://a.example|https://a.example|bare|url"},
}, {
	// The carve-outs hold inside a label too, because the label is parsed
	// with the same parser.
	name: "a URL in inline code inside a label is not an autolink",
	src:  "Text :sup[`https://a.example`] end\n",
	want: nil,
}, {
	name: "a URL in a link label inside a label is not an autolink",
	src:  "Text :sup[[l](https://a.example)] end\n",
	want: nil,
}, {
	name: "an empty label reports nothing and does not hide prose",
	src:  "Text :sup[] end https://b.example\n",
	want: []string{"https://b.example|https://b.example|https://b.example|bare|url"},
}}

func TestAutolinks_TextDirectiveLabels(t *testing.T) {
	t.Parallel()
	for _, c := range autolinkLabelCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := markdown.NewSource([]byte(c.src))
			if got := autolinkTexts(c.src, s.Autolinks()); !eq(got, c.want) {
				t.Errorf("Autolinks(%q) =\n %q\nwant\n %q", c.src, got, c.want)
			}
			if u := s.UnlocatedAutolinks(); u != 0 {
				t.Errorf("UnlocatedAutolinks() = %d, want 0", u)
			}
		})
	}
}

// TestAutolinks_ContractHolds checks what the callers rely on across a
// document holding every shape at once: ascending order, no overlap, spans
// inside the source, Text inside Span, and a nonempty address in both
// forms.
func TestAutolinks_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "https://a.example and <https://b.example>.\n\n" +
		"> q <c@d.example> and www.e.example\n\n" +
		"- item https://f.example/x, then :sup[see https://g.example]\n\n" +
		"| <https://h.example> | b |\n| --- | --- |\n| c | d |\n\n" +
		"Text <span>https://i.example</span> end\n"
	got := markdown.Autolinks([]byte(src))
	if len(got) != 8 {
		t.Fatalf("Autolinks reported %d autolinks, want 8:\n%q", len(got), autolinkTexts(src, got))
	}
	for i, a := range got {
		if a.Span.Start < 0 || a.Span.Stop > len(src) || a.Span.Start >= a.Span.Stop {
			t.Errorf("autolink %d Span = %v is not a range of a %d byte source",
				i, a.Span, len(src))
		}
		if a.Text.Start < a.Span.Start || a.Text.Stop > a.Span.Stop || a.Text.Start >= a.Text.Stop {
			t.Errorf("autolink %d Text = %v is not a nonempty range inside Span = %v",
				i, a.Text, a.Span)
		}
		if i > 0 && a.Span.Start < got[i-1].Span.Stop {
			t.Errorf("autolink %d Span = %v overlaps %v", i, a.Span, got[i-1].Span)
		}
		if a.Bare != (a.Span == a.Text) {
			t.Errorf("autolink %d says Bare=%v with Span = %v and Text = %v",
				i, a.Bare, a.Span, a.Text)
		}
	}
}

// TestAutolinks_NormalizeBareURLsInOnePass is the call site's question, and
// the reason the view reports Bare and Target rather than a span alone: a
// consumer rewriting bare URLs into the bracketed form needs to skip the
// ones already bracketed (or it doubles the brackets), and to skip a `www.`
// address (or it writes a relative link). Both decisions are made from the
// view, and the edits it yields go through Apply in one pass.
func TestAutolinks_NormalizeBareURLsInOnePass(t *testing.T) {
	t.Parallel()
	const src = "Read https://a.example/x, see <https://b.example> and www.c.example.\n\n" +
		"`https://d.example` stays, :sup[and https://e.example] does not.\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, a := range s.Autolinks() {
		written := string(s.Text(a.Text))
		if !a.Bare || a.Email || written != a.Target {
			continue
		}
		edits = append(edits, markdown.Edit{Span: a.Text, Text: "<" + written + ">"})
	}
	if len(edits) != 2 {
		t.Fatalf("built %d edits, want 2: %v", len(edits), edits)
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "Read <https://a.example/x>, see <https://b.example> and www.c.example.\n\n" +
		"`https://d.example` stays, :sup[and <https://e.example>] does not.\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}

// TestAutolinks_MemoizesOneParse pins the Source contract the one-shot form
// gives up: the view is computed once, and the count accessor forces it
// rather than reading an unpopulated field.
func TestAutolinks_MemoizesOneParse(t *testing.T) {
	t.Parallel()
	const src = "See https://a.example and <https://b.example>\n"
	s := markdown.NewSource([]byte(src))
	if got := s.UnlocatedAutolinks(); got != 0 {
		t.Errorf("UnlocatedAutolinks() before its view = %d, want 0", got)
	}
	first := s.Autolinks()
	second := s.Autolinks()
	if len(first) != 2 {
		t.Fatalf("len(Autolinks()) = %d, want 2", len(first))
	}
	if &first[0] != &second[0] {
		t.Error("Autolinks recomputed its view instead of returning the memo")
	}
}

// FuzzAutolinks is the invariant behind every case above, over inputs no
// table would think of: a reported span must select the autolink AS
// WRITTEN. That is what makes the spans safe to splice, and a wrong offset
// is the failure mode that costs a rewriter the document rather than a
// normalization.
func FuzzAutolinks(f *testing.F) {
	for _, c := range autolinkFormCases {
		f.Add(c.src)
	}
	for _, c := range autolinkBoundaryCases {
		f.Add(c.src)
	}
	for _, c := range autolinkLabelCases {
		f.Add(c.src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		s := markdown.NewSource([]byte(src))
		prev := -1
		for _, a := range s.Autolinks() {
			if a.Span.Start < prev {
				t.Fatalf("autolink %v is out of order in %q", a.Span, src)
			}
			prev = a.Span.Stop
			whole, text := string(s.Text(a.Span)), string(s.Text(a.Text))
			if a.Bare {
				if whole != text {
					t.Fatalf("bare autolink %v spans %q but its address is %q in %q",
						a.Span, whole, text, src)
				}
				continue
			}
			if whole != "<"+text+">" {
				t.Fatalf("angle autolink %v spans %q, want %q in %q",
					a.Span, whole, "<"+text+">", src)
			}
		}
	})
}
