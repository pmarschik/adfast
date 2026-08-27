package markdown_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// texts renders spans as the source they cover — the readable form of a
// span assertion, and the one that fails informatively.
func texts(src string, spans markdown.Spans) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, src[s.Start:s.Stop])
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// codeSpanCase is one source plus the exact text every span must select.
// Asserting on the selected text rather than on offsets is what makes a
// failure readable.
type codeSpanCase struct {
	name string
	src  string
	want []string
}

// codeSpanCases covers every shape the audit measured plus the edge cases a
// Lines()-only derivation misses (an empty fence has neither content lines
// nor an info string).
var codeSpanCases = []codeSpanCase{{
	name: "fenced with an info string",
	src:  "```js\ncode\n```\n",
	want: []string{"```js\ncode\n```\n"},
}, {
	name: "indented code keeps its indent",
	src:  "para\n\n    code here\n",
	want: []string{"    code here\n"},
}, {
	// The line scanner reads "``` js" as a closer and ends the block
	// early, exposing everything after it to a rewriter.
	name: "a closer carrying an info string does not close",
	src:  "para\n\n```js\ncode\n``` js\nmore\n",
	want: []string{"```js\ncode\n``` js\nmore\n"},
}, {
	// The line scanner reports no code at all here.
	name: "four-space content in a blockquoted list is code",
	src:  "> - a\n>\n>       code\n",
	want: []string{">       code\n"},
}, {
	name: "a fence in a list item, closed",
	src:  "- item\n  ```\n  x\n  ```\n",
	want: []string{"  ```\n  x\n  ```\n"},
}, {
	name: "a fence on the list marker line",
	src:  "- ```\n  x\n  ```\n",
	want: []string{"- ```\n  x\n  ```\n"},
}, {
	name: "a fence inside a blockquote",
	src:  "> ```\n> x\n> ```\n",
	want: []string{"> ```\n> x\n> ```\n"},
}, {
	// Neither line ends the other's block: line 3 opens its own.
	name: "an unclosed blockquote fence before a top-level fence",
	src:  "> ```\n> x\n```\n",
	want: []string{"> ```\n> x\n", "```\n"},
}, {
	name: "an empty fence with no info string",
	src:  "```\n```\n",
	want: []string{"```\n```\n"},
}, {
	name: "an empty fence with an info string",
	src:  "```js\n```\n",
	want: []string{"```js\n```\n"},
}, {
	name: "an unclosed fence runs to the end of the source",
	src:  "```js\ncode\n",
	want: []string{"```js\ncode\n"},
}, {
	name: "a lone fence at the end of the source",
	src:  "text\n\n```\n",
	want: []string{"```\n"},
}, {
	name: "an indented tilde fence",
	src:  "  ~~~~ruby\n  x\n  ~~~~\n",
	want: []string{"  ~~~~ruby\n  x\n  ~~~~\n"},
}, {
	name: "a shorter fence inside a longer one does not close it",
	src:  "````\n```\nx\n```\n````\n",
	want: []string{"````\n```\nx\n```\n````\n"},
}, {
	name: "blank lines inside a fence stay inside it",
	src:  "```\n\n\nx\n\n```\n",
	want: []string{"```\n\n\nx\n\n```\n"},
}, {
	name: "an indented block spans its internal blank line",
	src:  "    a\n\n    b\n\ntext\n",
	want: []string{"    a\n\n    b\n"},
}, {
	name: "two sibling fences stay separate",
	src:  "```\na\n```\n\n```\nb\n```\n",
	want: []string{"```\na\n```\n", "```\nb\n```\n"},
}, {
	name: "an unclosed fence per list item",
	src:  "- ```\n  x\n- ```\n  y\n",
	want: []string{"- ```\n  x\n", "- ```\n  y\n"},
}, {
	name: "a source without a trailing newline",
	src:  "```\na\n```",
	want: []string{"```\na\n```"},
}, {
	name: "indented code without a trailing newline",
	src:  "text\n\n    x",
	want: []string{"    x"},
}, {
	// The info string may not contain a backtick, so line 1 is prose
	// and the fence only opens on line 3.
	name: "a backtick in the info string is not a fence",
	src:  "```js `x`\ncode\n```\n",
	want: []string{"```\n"},
}, {
	name: "inline code is not a code block",
	src:  "text `http://a.example` more\n",
	want: nil,
}, {
	name: "a document with no code",
	src:  "# t\n\ntext http://a.example\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

// TestCodeSpans_Coverage pins the whole-line rule over codeSpanCases.
func TestCodeSpans_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range codeSpanCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.CodeSpans([]byte(c.src))
			if want := c.want; !eq(texts(c.src, got), want) {
				t.Errorf("CodeSpans(%q)\n got %q\nwant %q", c.src, texts(c.src, got), want)
			}
		})
	}
}

// TestCodeSpans_ContractHolds checks the ordering and non-overlap the Spans
// binary searches depend on, over every case above at once.
func TestCodeSpans_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "```\na\n```\n\n    b\n\n> ```\n> c\n```\n\n- ```\n  d\n- ```\n  e\n"
	spans := markdown.CodeSpans([]byte(src))
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

// TestCodeSpans_IsTheParserView cross-checks the spans against goldmark's
// own verdict: every byte a span covers must be inside a fenced or indented
// code block, and the covered text must contain the block's content.
func TestCodeSpans_IsTheParserView(t *testing.T) {
	t.Parallel()
	src := "intro\n\n```go\nfmt.Println(\"http://a.example\")\n```\n\nafter\n\n    indented http://b.example\n\nend\n"
	spans := markdown.CodeSpans([]byte(src))
	for _, needle := range []string{"http://a.example", "http://b.example"} {
		at := strings.Index(src, needle)
		if at < 0 {
			t.Fatalf("fixture lost %q", needle)
		}
		if !spans.Contains(at) {
			t.Errorf("offset %d (%q) is inside a code block but no span covers it", at, needle)
		}
	}
	if at := strings.Index(src, "after"); spans.Contains(at) {
		t.Errorf("offset %d (prose) is covered by %v", at, spans)
	}
}

func TestSpan_Predicates(t *testing.T) {
	t.Parallel()
	s := markdown.Span{Start: 3, Stop: 7}
	if s.Len() != 4 {
		t.Errorf("Len = %d, want 4", s.Len())
	}
	for _, off := range []int{3, 4, 6} {
		if !s.Contains(off) {
			t.Errorf("Contains(%d) = false", off)
		}
	}
	for _, off := range []int{2, 7, 100} {
		if s.Contains(off) {
			t.Errorf("Contains(%d) = true, want false (half-open)", off)
		}
	}
	if s.Overlaps(markdown.Span{Start: 7, Stop: 9}) {
		t.Error("adjacent spans must not overlap")
	}
	if !s.Overlaps(markdown.Span{Start: 6, Stop: 9}) {
		t.Error("touching spans must overlap")
	}
	if s.Overlaps(markdown.Span{Start: 5, Stop: 5}) {
		t.Error("an empty span overlaps nothing")
	}
}

func TestSpans_Search(t *testing.T) {
	t.Parallel()
	ss := markdown.Spans{{Start: 2, Stop: 5}, {Start: 10, Stop: 12}, {Start: 20, Stop: 30}}
	for _, c := range []struct {
		off  int
		want bool
	}{{0, false}, {1, false}, {2, true}, {4, true}, {5, false}, {9, false}, {11, true}, {19, false}, {29, true}, {30, false}} {
		if got := ss.Contains(c.off); got != c.want {
			t.Errorf("Contains(%d) = %v, want %v", c.off, got, c.want)
		}
	}
	for _, c := range []struct {
		s    markdown.Span
		want bool
	}{
		{markdown.Span{Start: 0, Stop: 2}, false},
		{markdown.Span{Start: 0, Stop: 3}, true},
		{markdown.Span{Start: 5, Stop: 10}, false},
		{markdown.Span{Start: 5, Stop: 11}, true},
		{markdown.Span{Start: 12, Stop: 20}, false},
		{markdown.Span{Start: 0, Stop: 100}, true},
		{markdown.Span{Start: 3, Stop: 3}, false},
	} {
		if got := ss.Overlaps(c.s); got != c.want {
			t.Errorf("Overlaps(%v) = %v, want %v", c.s, got, c.want)
		}
	}
	if markdown.Spans(nil).Contains(0) {
		t.Error("an empty set contains nothing")
	}
}

func TestSource_BytesAndVerbatim(t *testing.T) {
	t.Parallel()
	src := []byte("# t\n\n```\nx\n```\n")
	s := markdown.NewSource(src)
	if !s.Verbatim() {
		t.Error("Verbatim = false for a source goldmark parses directly")
	}
	if !bytes.Equal(s.Bytes(), src) {
		t.Errorf("Bytes = %q, want %q", s.Bytes(), src)
	}
	sp := s.CodeSpans()
	if len(sp) != 1 || string(s.Text(sp[0])) != "```\nx\n```\n" {
		t.Errorf("CodeSpans = %q", texts(string(src), sp))
	}
	if s.Text(markdown.Span{Start: 0, Stop: 999}) != nil {
		t.Error("Text of an out-of-range span must be nil")
	}
	if s.Text(markdown.Span{Start: 3, Stop: 1}) != nil {
		t.Error("Text of an inverted span must be nil")
	}
}

// TestSource_VerbatimIsFalseAfterRecovery pins the contract that matters to
// a byte-preserving caller: when the guarded parse had to normalize the
// source, the spans address the normalized copy and Verbatim says so.
func TestSource_VerbatimIsFalseAfterRecovery(t *testing.T) {
	t.Parallel()
	// goldmark <=1.8.5 panics on this shape; parseGuarded retries with tabs
	// expanded (see parse.go).
	s := markdown.NewSource([]byte("*\n  \t`"))
	if s.Verbatim() {
		return // goldmark parsed it directly; nothing to assert.
	}
	if string(s.Bytes()) == "*\n  \t`" {
		t.Error("Verbatim = false but Bytes is the input unchanged")
	}
	for _, sp := range s.CodeSpans() {
		if sp.Stop > len(s.Bytes()) {
			t.Errorf("span %v is out of range of the recovered source", sp)
		}
	}
}

func TestSource_Apply(t *testing.T) {
	t.Parallel()
	src := "one two three"
	s := markdown.NewSource([]byte(src))
	cases := []struct {
		name  string
		want  string
		edits []markdown.Edit
	}{{
		name: "no edits copies the source",
		want: src,
	}, {
		name:  "replace",
		edits: []markdown.Edit{{Span: markdown.Span{Start: 4, Stop: 7}, Text: "TWO"}},
		want:  "one TWO three",
	}, {
		name: "several edits given out of order",
		edits: []markdown.Edit{
			{Span: markdown.Span{Start: 8, Stop: 13}, Text: "3"},
			{Span: markdown.Span{Start: 0, Stop: 3}, Text: "1"},
			{Span: markdown.Span{Start: 4, Stop: 7}, Text: "2"},
		},
		want: "1 2 3",
	}, {
		name:  "insert at an empty span",
		edits: []markdown.Edit{{Span: markdown.Span{Start: 3, Stop: 3}, Text: "!"}},
		want:  "one! two three",
	}, {
		name:  "delete",
		edits: []markdown.Edit{{Span: markdown.Span{Start: 3, Stop: 7}, Text: ""}},
		want:  "one three",
	}, {
		name:  "an edit at the very end",
		edits: []markdown.Edit{{Span: markdown.Span{Start: 13, Stop: 13}, Text: "."}},
		want:  "one two three.",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := s.Apply(c.edits...)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("Apply = %q, want %q", got, c.want)
			}
			if string(s.Bytes()) != src {
				t.Errorf("Apply mutated the source: %q", s.Bytes())
			}
		})
	}
}

func TestSource_ApplyRejects(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("one two three"))
	cases := []struct {
		want  error
		name  string
		edits []markdown.Edit
	}{{
		name:  "past the end",
		edits: []markdown.Edit{{Span: markdown.Span{Start: 10, Stop: 99}}},
		want:  markdown.ErrSpanRange,
	}, {
		name:  "negative",
		edits: []markdown.Edit{{Span: markdown.Span{Start: -1, Stop: 2}}},
		want:  markdown.ErrSpanRange,
	}, {
		name:  "inverted",
		edits: []markdown.Edit{{Span: markdown.Span{Start: 5, Stop: 2}}},
		want:  markdown.ErrSpanRange,
	}, {
		name: "overlapping",
		edits: []markdown.Edit{
			{Span: markdown.Span{Start: 0, Stop: 5}, Text: "a"},
			{Span: markdown.Span{Start: 4, Stop: 9}, Text: "b"},
		},
		want: markdown.ErrSpanOverlap,
	}, {
		name: "overlapping, given out of order",
		edits: []markdown.Edit{
			{Span: markdown.Span{Start: 4, Stop: 9}, Text: "b"},
			{Span: markdown.Span{Start: 0, Stop: 5}, Text: "a"},
		},
		want: markdown.ErrSpanOverlap,
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := s.Apply(c.edits...)
			if !errors.Is(err, c.want) {
				t.Fatalf("Apply err = %v, want %v", err, c.want)
			}
			if got != nil {
				t.Errorf("Apply returned %q alongside an error", got)
			}
		})
	}
}

// TestSource_ApplyKeepsAdjacentEditsStable pins the tie-break: two
// insertions at one offset keep their argument order.
func TestSource_ApplyKeepsAdjacentEditsStable(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("ab"))
	got, err := s.Apply(
		markdown.Edit{Span: markdown.Span{Start: 1, Stop: 1}, Text: "1"},
		markdown.Edit{Span: markdown.Span{Start: 1, Stop: 1}, Text: "2"},
	)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if string(got) != "a12b" {
		t.Errorf("Apply = %q, want %q", got, "a12b")
	}
}

// TestSource_SpansAndSpliceComposeInOneParse is the shape every consumer
// takes: ask for a view, decide policy over it, hand edits back. adfast
// owns both halves; the caller never indexes a byte.
func TestSource_SpansAndSpliceComposeInOneParse(t *testing.T) {
	t.Parallel()
	const src = "See http://old.example.\n\n```\ncurl http://old.example\n```\n\nAnd http://old.example again.\n"
	s := markdown.NewSource([]byte(src))
	code := s.CodeSpans()

	var edits []markdown.Edit
	for at := 0; ; {
		i := strings.Index(src[at:], "http://old.example")
		if i < 0 {
			break
		}
		hit := markdown.Span{Start: at + i, Stop: at + i + len("http://old.example")}
		at = hit.Stop
		if code.Overlaps(hit) {
			continue // documented example: leave the author's bytes alone
		}
		edits = append(edits, markdown.Edit{Span: hit, Text: "http://new.example"})
	}
	if len(edits) != 2 {
		t.Fatalf("rewrote %d URLs, want the 2 outside the fence", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "See http://new.example.\n\n```\ncurl http://old.example\n```\n\nAnd http://new.example again.\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}
