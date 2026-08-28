package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// linkCase is one source plus the exact text every link must select.
// "whole|text|dest" per link keeps a failure readable — the three extents can
// go wrong independently, and the destination is the one a caller rewrites.
type linkCase struct {
	name string
	src  string
	want []string
}

func linkTexts(src string, links []markdown.Link) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		dest := "-"
		if l.Dest != (markdown.Span{}) {
			dest = src[l.Dest.Start:l.Dest.Stop]
		}
		out = append(out, strings.Join([]string{
			src[l.Span.Start:l.Span.Stop],
			src[l.Text.Start:l.Text.Stop],
			dest,
		}, "|"))
	}
	return out
}

var linkCases = []linkCase{{
	name: "a plain link",
	src:  "[a](x.md)\n",
	want: []string{"[a](x.md)|a|x.md"},
}, {
	name: "a link with no text",
	src:  "[](x.md)\n",
	want: []string{"[](x.md)||x.md"},
}, {
	// The angle brackets stay outside Dest so a rewrite lands between them.
	name: "an angle bracketed destination excludes its brackets",
	src:  "[a](<b c.md>)\n",
	want: []string{"[a](<b c.md>)|a|b c.md"},
}, {
	name: "a title is outside the destination",
	src:  "[a](x.md \"t\")\n",
	want: []string{"[a](x.md \"t\")|a|x.md"},
}, {
	name: "a parenthesized title",
	src:  "[a](x.md (t))\n",
	want: []string{"[a](x.md (t))|a|x.md"},
}, {
	name: "a single quoted title",
	src:  "[a](x.md 't')\n",
	want: []string{"[a](x.md 't')|a|x.md"},
}, {
	// A title may hold the very syntax the scanner looks for.
	name: "a title holding a closing paren",
	src:  "[a](x.md \"a ) b\")\n",
	want: []string{"[a](x.md \"a ) b\")|a|x.md"},
}, {
	name: "balanced parens inside a destination",
	src:  "[a](x(1).md)\n",
	want: []string{"[a](x(1).md)|a|x(1).md"},
}, {
	name: "an empty destination is an insertion point",
	src:  "[a]()\n",
	want: []string{"[a]()|a|"},
}, {
	name: "an empty angle bracketed destination",
	src:  "[a](<>)\n",
	want: []string{"[a](<>)|a|"},
}, {
	// The label's closing bracket is not the first one in the source.
	name: "a nested bracket pair in the link text",
	src:  "[a[b]c](x.md)\n",
	want: []string{"[a[b]c](x.md)|a[b]c|x.md"},
}, {
	name: "a closing bracket inside inline code in the link text",
	src:  "[a`]`b](x.md)\n",
	want: []string{"[a`]`b](x.md)|a`]`b|x.md"},
}, {
	name: "an escaped closing bracket in the link text",
	src:  "[a\\]b](x.md)\n",
	want: []string{"[a\\]b](x.md)|a\\]b|x.md"},
}, {
	// The whole tail lives inside inline code in the label, so only the
	// destination check can tell the two candidate closers apart.
	name: "a whole tail inside inline code in the link text",
	src:  "[`](x.md)`](y.md)\n",
	want: []string{"[`](x.md)`](y.md)|`](x.md)`|y.md"},
}, {
	name: "raw html in the link text",
	src:  "[a<b>c](x.md)\n",
	want: []string{"[a<b>c](x.md)|a<b>c|x.md"},
}, {
	// Link text is source, not text: the emphasis markers stay in it.
	name: "inline markup in the link text stays verbatim",
	src:  "[*em* x](y.md)\n",
	want: []string{"[*em* x](y.md)|*em* x|y.md"},
}, {
	name: "a multi byte link text",
	src:  "[мульти](x.md) tail\n",
	want: []string{"[мульти](x.md)|мульти|x.md"},
}, {
	name: "a full reference link has no destination at the link",
	src:  "[ref][id]\n\n[id]: x.md\n",
	want: []string{"[ref][id]|ref|-"},
}, {
	name: "a collapsed reference link",
	src:  "[id][]\n\n[id]: x.md\n",
	want: []string{"[id][]|id|-"},
}, {
	name: "a shortcut reference link",
	src:  "[id]\n\n[id]: x.md\n",
	want: []string{"[id]|id|-"},
}, {
	// No definition, so goldmark makes no link and neither does the view.
	name: "a reference link with no definition is not a link",
	src:  "[missing][id]\n",
	want: nil,
}, {
	// This is the gap the view closes: a regexp over the source reports this
	// destination, and rewriting it edits what the author wrote as an example.
	name: "a link inside inline code is not a link",
	src:  "`[no](x.md)`\n",
	want: nil,
}, {
	name: "a link inside a fenced block is not a link",
	src:  "```\n[no](x.md)\n```\n",
	want: nil,
}, {
	name: "a link inside an indented block is not a link",
	src:  "    [no](x.md)\n",
	want: nil,
}, {
	name: "a link inside an html comment is not a link",
	src:  "<!-- [no](x.md) -->\n",
	want: nil,
}, {
	// A definition's destination is not written at a link node — goldmark
	// consumes the definition into its reference map instead of the tree.
	name: "a link reference definition is not a link",
	src:  "[id]: x.md\n",
	want: nil,
}, {
	// An autolink is a different node, and its destination is its own text.
	name: "an angle bracket autolink is not a link",
	src:  "<https://x/a>\n",
	want: nil,
}, {
	name: "a linkified bare url is not a link",
	src:  "see https://x/a here\n",
	want: nil,
}, {
	name: "a link in a blockquote",
	src:  "> [q](x.md)\n",
	want: []string{"[q](x.md)|q|x.md"},
}, {
	name: "a link in a list item",
	src:  "- [l](x.md)\n",
	want: []string{"[l](x.md)|l|x.md"},
}, {
	name: "a link in a heading",
	src:  "# [h](x.md)\n",
	want: []string{"[h](x.md)|h|x.md"},
}, {
	name: "a link in a table cell",
	src:  "| a |\n| - |\n| [t](x.md) |\n",
	want: []string{"[t](x.md)|t|x.md"},
}, {
	name: "a link in a directive label",
	src:  "::note[[lab](x.md)]{c=red}\n",
	want: []string{"[lab](x.md)|lab|x.md"},
}, {
	name: "a link inside a container directive",
	src:  ":::note\n[in](x.md)\n:::\n",
	want: []string{"[in](x.md)|in|x.md"},
}, {
	name: "adjacent links",
	src:  "[a](x.md)[b](y.md)\n",
	want: []string{"[a](x.md)|a|x.md", "[b](y.md)|b|y.md"},
}, {
	name: "a link between prose",
	src:  "para [a](x.md) more\n",
	want: []string{"[a](x.md)|a|x.md"},
}, {
	// An image inside link text is allowed where a link inside one is not.
	// Only the link is reported here; Images is the other view.
	name: "an image in the link text",
	src:  "[![in](a.png)](b.md)\n",
	want: []string{"[![in](a.png)](b.md)|![in](a.png)|b.md"},
}, {
	name: "a destination on the line after its paren",
	src:  "[a](\nx.md)\n",
	want: []string{"[a](\nx.md)|a|x.md"},
}, {
	// A list's continuation prefix is whitespace, which the tail skips
	// anyway, so a split tail resolves normally here.
	name: "a title on the next line of a list item",
	src:  "- [a](x.md\n  \"t\")\n",
	want: []string{"[a](x.md\n  \"t\")|a|x.md"},
}, {
	// A blockquote's '>' prefix sits between the two halves of the tail.
	name: "a title on the next line of a blockquote",
	src:  "> [a](x.md\n> \"t\")\n",
	want: []string{"[a](x.md\n> \"t\")|a|x.md"},
}, {
	name: "a destination on the next line of a blockquote",
	src:  "> [a](\n> x.md)\n",
	want: []string{"[a](\n> x.md)|a|x.md"},
}, {
	name: "a destination on the next line of a nested blockquote",
	src:  ">> [a](\n>> x.md)\n",
	want: []string{"[a](\n>> x.md)|a|x.md"},
}, {
	// The prefix is skipped, not the padding a wrapped destination needs:
	// the angle brackets still bound Dest.
	name: "an angle bracketed destination on the next line of a blockquote",
	src:  "> [a](\n> <b c.md> \"t\")\n",
	want: []string{"[a](\n> <b c.md> \"t\")|a|b c.md"},
}, {
	// The link ends before the prose that follows it on the quoted line.
	name: "a split tail in a blockquote with trailing prose",
	src:  "> [a](x.md\n> \"t\") tail\n",
	want: []string{"[a](x.md\n> \"t\")|a|x.md"},
}, {
	// The same documented drop the image view has: the parser matched a
	// normalized label, so a reference label that crosses a line never
	// compares equal to its written bytes.
	name: "a reference label crossing a line is dropped",
	src:  "> [a][i\n> d]\n\n[i d]: x.md\n",
	want: nil,
}, {
	name: "a link with no trailing newline",
	src:  "[a](x.md)",
	want: []string{"[a](x.md)|a|x.md"},
}, {
	name: "a document with only an image has no links",
	src:  "text and an ![img](x.png)\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestLinks_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range linkCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Links([]byte(c.src))
			if !eq(linkTexts(c.src, got), c.want) {
				t.Errorf("Links(%q)\n got %q\nwant %q", c.src, linkTexts(c.src, got), c.want)
			}
		})
	}
}

// TestLinks_ContractHolds pins document order, the containment of Text and
// Dest in Span, and the tightness rule itself: a link's span starts at a '['
// and ends at a ')' or a ']', never at a line boundary it did not happen to
// land on.
func TestLinks_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "[a](x.md) and [b][id] and [c](<d e.md> \"t\")\n\n> [q](f.md)\n\n[id]: y.md\n"
	links := markdown.Links([]byte(src))
	if len(links) != 4 {
		t.Fatalf("got %d links, want 4: %q", len(links), linkTexts(src, links))
	}
	prev := 0
	for i, l := range links {
		if l.Span.Start < prev {
			t.Errorf("link %d = %v is out of document order", i, l.Span)
		}
		prev = l.Span.Start
		if l.Span.Start < 0 || l.Span.Stop > len(src) || l.Span.Len() < 4 {
			t.Errorf("link %d span %v is not a range of a %d byte source", i, l.Span, len(src))
		}
		if src[l.Span.Start] != '[' {
			t.Errorf("link %d starts at %q, want '['", i, src[l.Span.Start])
		}
		if last := src[l.Span.Stop-1]; last != ')' && last != ']' {
			t.Errorf("link %d ends at %q, want ')' or ']'", i, last)
		}
		if l.Text.Start < l.Span.Start || l.Text.Stop > l.Span.Stop {
			t.Errorf("link %d text %v is not inside its span %v", i, l.Text, l.Span)
		}
		if l.Dest != (markdown.Span{}) &&
			(l.Dest.Start < l.Span.Start || l.Dest.Stop > l.Span.Stop) {
			t.Errorf("link %d dest %v is not inside its span %v", i, l.Dest, l.Span)
		}
	}
}

// TestLinks_TightSpansStayOffTheProse is the reason the span is tight rather
// than widened to whole lines the way the block views are: replacing a link
// must leave the sentence around it alone.
func TestLinks_TightSpansStayOffTheProse(t *testing.T) {
	t.Parallel()
	const src = "Before [a](x.md) after.\n"
	s := markdown.NewSource([]byte(src))
	links := s.Links()
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	got, err := s.Apply(markdown.Edit{Span: links[0].Span, Text: "GONE"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if want := "Before GONE after.\n"; string(got) != want {
		t.Errorf("Apply = %q, want %q", got, want)
	}
}

// TestLinks_DoNotOverlapCodeSpans is the composition guarantee that matters:
// the two views come from ONE tree, so a link written inside a code block is
// not a link at all. Two independent scanners would disagree here, which is
// what this surface exists to prevent.
func TestLinks_DoNotOverlapCodeSpans(t *testing.T) {
	t.Parallel()
	src := []byte("[real](a.md)\n\n```\n[fake](b.md)\n```\n\n    [also fake](c.md)\n\n[real too](d.md)\n")
	s := markdown.NewSource(src)
	code := s.CodeSpans()
	links := s.Links()
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2: %q", len(links), linkTexts(string(src), links))
	}
	for _, l := range links {
		if code.Overlaps(l.Span) {
			t.Errorf("link %q overlaps a code span", src[l.Span.Start:l.Span.Stop])
		}
	}
}

// TestLinks_DoNotOverlapInlineCodeSpans is the half a line scanner cannot do
// at all. A code BLOCK is a range of whole lines, so a scanner that skips
// fenced ranges still reports the destination in `[a](x.md)` written inline
// as code — and rewriting it edits the example the author was quoting.
func TestLinks_DoNotOverlapInlineCodeSpans(t *testing.T) {
	t.Parallel()
	src := []byte("Write `[a](old.md)` to link, as in [b](old.md).\n")
	s := markdown.NewSource(src)
	links := s.Links()
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1: %q", len(links), linkTexts(string(src), links))
	}
	inline := s.InlineCodeSpans()
	if !inline.Overlaps(markdown.Span{Start: 7, Stop: 8}) {
		t.Fatalf("the fixture no longer holds an inline code span at 7")
	}
	if inline.Overlaps(links[0].Dest) {
		t.Errorf("link destination %q overlaps an inline code span", s.Text(links[0].Dest))
	}
	got, err := s.Apply(markdown.Edit{Span: links[0].Dest, Text: "new.md"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "Write `[a](old.md)` to link, as in [b](new.md).\n"
	if string(got) != want {
		t.Errorf("Apply = %q, want %q", got, want)
	}
}

// TestLinks_Memoize pins that a second call is the same slice, i.e. that the
// view is computed once per Source rather than per call.
func TestLinks_Memoize(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("[a](x.md) [b](y.md)\n"))
	first, second := s.Links(), s.Links()
	if len(first) != 2 || len(second) != len(first) {
		t.Fatalf("Links = %v then %v", first, second)
	}
	if &first[0] != &second[0] {
		t.Error("Links recomputed on the second call")
	}
}

// TestLinks_RewriteDestinationsInOneParse is the shape the callers take: read
// the destination verbatim, decide policy over it, hand Edits back. Nothing
// here indexes a byte or re-derives an offset, and the link inside the fence
// is not touched because the parser never called it one.
func TestLinks_RewriteDestinationsInOneParse(t *testing.T) {
	t.Parallel()
	const src = "[a](old/one.md)\n\n```\n[x](old/skip.md)\n```\n\n[b](<old/two a.md>) [c][id]\n\n[id]: old/three.md\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, l := range s.Links() {
		if l.Dest == (markdown.Span{}) {
			continue // a reference link is rewritten at its definition
		}
		dest := string(s.Text(l.Dest))
		if !strings.HasPrefix(dest, "old/") {
			t.Fatalf("destination = %q, want the raw written path", dest)
		}
		edits = append(edits, markdown.Edit{Span: l.Dest, Text: "new/" + strings.TrimPrefix(dest, "old/")})
	}
	if len(edits) != 2 {
		t.Fatalf("found %d rewritable links, want 2", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "[a](new/one.md)\n\n```\n[x](old/skip.md)\n```\n\n[b](<new/two a.md>) [c][id]\n\n[id]: old/three.md\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}

// TestLinks_ComposeWithImagesInOneParse pins the point of the surface: the
// two destination views come off ONE Source, their spans do not collide even
// where an image sits inside a link's text, and the edits they produce merge
// without a second parse.
func TestLinks_ComposeWithImagesInOneParse(t *testing.T) {
	t.Parallel()
	const src = "[![icon](old.png)](old.md) and ![lone](old2.png)\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, l := range s.Links() {
		edits = append(edits, markdown.Edit{Span: l.Dest, Text: "new.md"})
	}
	for _, im := range s.Images() {
		edits = append(edits, markdown.Edit{
			Span: im.Dest, Text: "new" + strings.TrimPrefix(string(s.Text(im.Dest)), "old"),
		})
	}
	if len(edits) != 3 {
		t.Fatalf("built %d edits, want 3", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "[![icon](new.png)](new.md) and ![lone](new2.png)\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}
