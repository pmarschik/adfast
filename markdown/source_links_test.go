package markdown_test

import (
	"slices"
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
	// An OPENING bracket the label never closes. Each of the three below is
	// content of a parsed child, so the floor is already past it.
	name: "an opening bracket inside inline code in the link text",
	src:  "[a`[`b](x.md)\n",
	want: []string{"[a`[`b](x.md)|a`[`b|x.md"},
}, {
	name: "an escaped opening bracket in the link text",
	src:  "[a\\[b](x.md)\n",
	want: []string{"[a\\[b](x.md)|a\\[b|x.md"},
}, {
	name: "an opening bracket inside raw html in the link text",
	src:  "[a<i data-x=\"[\">b</i>](x.md)\n",
	want: []string{"[a<i data-x=\"[\">b</i>](x.md)|a<i data-x=\"[\">b</i>|x.md"},
}, {
	// An autolink carries its URL off the node instead of in a child, so it
	// contributes nothing to the floor and its brackets are read as source.
	// The label's closer is still the first `]` whose tail checks out.
	name: "an opening bracket inside an autolink in the link text",
	src:  "[<https://x/a[b>](y.md)\n",
	want: []string{"[<https://x/a[b>](y.md)|<https://x/a[b>|y.md"},
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
	// A thumbnail that links to its own file, which writes the SAME
	// destination twice. The destination check cannot tell the nested closer
	// from the label's own here, because the two candidate tails parse back
	// to byte-equal destinations; only a floor above the whole nested image
	// can.
	name: "an image in the link text sharing the link's destination",
	src:  "[![](./img/shot.png)](./img/shot.png)\n",
	want: []string{
		"[![](./img/shot.png)](./img/shot.png)|![](./img/shot.png)|./img/shot.png",
	},
}, {
	// The same collision with alt text, which is where the nested label's
	// own content raises the floor and the nested TAIL is all that is left
	// to get past.
	name: "an image with alt text sharing the link's destination",
	src:  "[![alt](x.png)](x.png)\n",
	want: []string{"[![alt](x.png)](x.png)|![alt](x.png)|x.png"},
}, {
	name: "an image in an image in the link text all sharing one destination",
	src:  "[![![](a.png)](a.png)](a.png)\n",
	want: []string{"[![![](a.png)](a.png)](a.png)|![![](a.png)](a.png)|a.png"},
}, {
	// A nested DESTINATION may hold a bracket nothing balances, which is
	// legal because a destination is a path and not inline content. Taking
	// the floor from the nested image's own resolved span is what gets past
	// it; reading the label as brackets never could.
	name: "a nested image destination holding an unbalanced bracket",
	src:  "[![](a[.png)](x.md)\n",
	want: []string{"[![](a[.png)](x.md)|![](a[.png)|x.md"},
}, {
	name: "a nested image title holding an unbalanced bracket",
	src:  "[![](a.png \"t[\")](b.md)\n",
	want: []string{"[![](a.png \"t[\")](b.md)|![](a.png \"t[\")|b.md"},
}, {
	// Both at once: the destination that is shared with the outer form, so
	// the destination check cannot decide, holds the bracket that no reading
	// of the label could balance. One floor above the nested form's whole
	// written extent answers both.
	name: "a shared destination holding an unbalanced bracket",
	src:  "[![](a[.png)](a[.png)\n",
	want: []string{"[![](a[.png)](a[.png)|![](a[.png)|a[.png"},
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
	// A reference label may cross a line, because CommonMark folds
	// whitespace runs when it matches a label. The span covers the fold and
	// the quoted line's prefix, the way a split tail does.
	name: "a reference label crossing a blockquote line",
	src:  "> [a][i\n> d]\n\n[i d]: x.md\n",
	want: []string{"[a][i\n> d]|a|-"},
}, {
	name: "a reference label crossing a list item line",
	src:  "- [a][i\n  d]\n\n[i d]: x.md\n",
	want: []string{"[a][i\n  d]|a|-"},
}, {
	name: "a shortcut reference label crossing a line",
	src:  "[i\nd]\n\n[i d]: x.md\n",
	want: []string{"[i\nd]|i\nd|-"},
}, {
	name: "a collapsed reference label crossing a line",
	src:  "[i\nd][]\n\n[i d]: x.md\n",
	want: []string{"[i\nd][]|i\nd|-"},
}, {
	// The regression this file exists to hold. Both labels READ as `i\nd`:
	// the parser sees the paragraph with its continuation indent stripped,
	// so the space written before the first `d` is not in the bytes it
	// recorded. A resolver comparing WRITTEN bytes to that value rejects the
	// empty-label link's own closer at offset 1, walks on to the next `]`,
	// and accepts the SECOND link's bracket pair — reporting the first link
	// as `[][i\n d][i\nd]`, a span whose tail belongs to another link and
	// whose Text starts at a `]`. Each link must cover its own extent only.
	name: "a folded label does not swallow the next bracket pair",
	src:  "[][i\n d][i\nd]\n\n[a][I d]\n\n[I d]:0",
	want: []string{"[][i\n d]||-", "[i\nd]|i\nd|-", "[a][I d]|a|-"},
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

// foldedLabelSrc is the document that made the folded-label resolution
// measurable. Two reference labels are written differently — `i\n d` and
// `i\nd` — and READ identically, because a paragraph's continuation line
// arrives with its leading whitespace already stripped; a third reference on
// one line is the control that must not move. `[I d]` pairs with all three,
// since matching a label folds its whitespace and ignores its case.
const foldedLabelSrc = "[][i\n d][i\nd]\n\n[a][I d]\n\n[I d]:0"

// TestLinks_FoldedLabelsStopAtTheirOwnCloser pins each link of
// foldedLabelSrc to its own written extent, and pins the property that makes
// the extent matter: the spans of two links never overlap.
//
// Overlap is the failure this test exists for, not a stylistic wish. A
// resolution that compares a label's WRITTEN bytes against the value the
// parser READ rejects the first link's own closer and then accepts the second
// link's bracket pair, reporting the first link as covering both — so a
// caller rewriting the first link's span would overwrite the second link. A
// wrong offset is the one answer this view must never give; dropping the link
// would have been acceptable, reporting someone else's bytes is not.
func TestLinks_FoldedLabelsStopAtTheirOwnCloser(t *testing.T) {
	t.Parallel()
	want := []struct{ span, text string }{
		{span: "[][i\n d]", text: ""},
		{span: "[i\nd]", text: "i\nd"},
		{span: "[a][I d]", text: "a"},
	}
	got := markdown.Links([]byte(foldedLabelSrc))
	if len(got) != len(want) {
		t.Fatalf("Links(%q) = %d links, want %d: %q",
			foldedLabelSrc, len(got), len(want), linkTexts(foldedLabelSrc, got))
	}
	for i, w := range want {
		l := got[i]
		if s := foldedLabelSrc[l.Span.Start:l.Span.Stop]; s != w.span {
			t.Errorf("link %d span %v slices to %q, want %q", i, l.Span, s, w.span)
		}
		if s := foldedLabelSrc[l.Text.Start:l.Text.Stop]; s != w.text {
			t.Errorf("link %d text %v slices to %q, want %q", i, l.Text, s, w.text)
		}
		if i > 0 && l.Span.Start < got[i-1].Span.Stop {
			t.Errorf("link %d span %v overlaps link %d span %v — an edit to one "+
				"would rewrite the other", i, l.Span, i-1, got[i-1].Span)
		}
	}
}

// TestLinks_FoldedLabelPinsAreOffByOneSensitive proves the pins above can
// fail, the way TestDefinitions_PinsAreOffByOneSensitive does: every non-empty
// span is nudged one byte in each direction and must then slice to something
// OTHER than the expectation. Without this, a span assertion could pass
// against an implementation that is uniformly off by one.
//
// The empty Text of the first link is excluded, because an empty span slices
// to "" at every offset. Its position is pinned by its neighbors instead: it
// must sit between the `[` and the `]` that are the whole of that label.
func TestLinks_FoldedLabelPinsAreOffByOneSensitive(t *testing.T) {
	t.Parallel()
	got := markdown.Links([]byte(foldedLabelSrc))
	if len(got) != 3 {
		t.Fatalf("Links = %d links, want 3", len(got))
	}
	checked := 0
	for i, l := range got {
		for _, p := range []struct {
			name string
			span markdown.Span
		}{{"Span", l.Span}, {"Text", l.Text}} {
			if p.span.Len() == 0 {
				continue
			}
			want := foldedLabelSrc[p.span.Start:p.span.Stop]
			for _, by := range []int{-1, 1} {
				start, stop := p.span.Start+by, p.span.Stop+by
				if start < 0 || stop > len(foldedLabelSrc) {
					continue
				}
				if s := foldedLabelSrc[start:stop]; s == want {
					t.Errorf("link %d %s shifted by %+d still slices to %q — "+
						"the pin is blind to an off-by-one", i, p.name, by, s)
				}
				checked++
			}
		}
	}
	// A guard on the guard: a loop that checked nothing would pass. Five
	// non-empty spans give two nudges each, less the one that would run off
	// the front of the document — the first link starts at offset 0.
	if checked != 9 {
		t.Fatalf("checked %d shifted spans, want 9", checked)
	}
	if empty := got[0].Text; foldedLabelSrc[empty.Start-1] != '[' ||
		foldedLabelSrc[empty.Stop] != ']' {
		t.Errorf("the empty text %v does not sit between its brackets", empty)
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

// TestLinks_SelfLinkingThumbnailSpans pins the exact offsets of the shape
// that used to collide: a thumbnail linking to its own file writes one
// destination twice, so the link and the image each resolved to the SAME
// bytes and the link's span stopped inside itself. The two good cases in the
// same document — a plain link and a link whose text holds an ordinary nested
// bracket pair — are there to prove the raised floor moved nothing else.
func TestLinks_SelfLinkingThumbnailSpans(t *testing.T) {
	t.Parallel()
	const src = "[![](./img/shot.png)](./img/shot.png) [ok](y.md) [a[b]c](z.md)\n"
	s := markdown.NewSource([]byte(src))

	wantLinks := []markdown.Link{
		{
			Span: markdown.Span{Start: 0, Stop: 37}, Text: markdown.Span{Start: 1, Stop: 20},
			Dest: markdown.Span{Start: 22, Stop: 36},
		},
		{
			Span: markdown.Span{Start: 38, Stop: 48}, Text: markdown.Span{Start: 39, Stop: 41},
			Dest: markdown.Span{Start: 43, Stop: 47},
		},
		{
			Span: markdown.Span{Start: 49, Stop: 62}, Text: markdown.Span{Start: 50, Stop: 55},
			Dest: markdown.Span{Start: 57, Stop: 61},
		},
	}
	if got := s.Links(); !slices.Equal(got, wantLinks) {
		t.Errorf("Links =\n %v\nwant\n %v", got, wantLinks)
	}
	wantImages := []markdown.Image{
		{
			Span: markdown.Span{Start: 1, Stop: 20}, Alt: markdown.Span{Start: 3, Stop: 3},
			Dest: markdown.Span{Start: 5, Stop: 19},
		},
	}
	if got := s.Images(); !slices.Equal(got, wantImages) {
		t.Errorf("Images =\n %v\nwant\n %v", got, wantImages)
	}
}

// TestLinks_SelfLinkingThumbnailEditsAreDisjoint is what the fix buys a
// consumer that rewrites destinations when a file moves. The link and the
// image write two separate destinations, so the two edits claim disjoint
// bytes and Apply rewrites both in one pass — where before the two views
// reported one destination twice and Apply had to refuse the pair.
func TestLinks_SelfLinkingThumbnailEditsAreDisjoint(t *testing.T) {
	t.Parallel()
	const src = "See [![](./img/shot.png)](./img/shot.png) here.\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, l := range s.Links() {
		edits = append(edits, markdown.Edit{Span: l.Dest, Text: "./pics/shot.png"})
	}
	for _, im := range s.Images() {
		edits = append(edits, markdown.Edit{Span: im.Dest, Text: "./pics/shot.png"})
	}
	if len(edits) != 2 {
		t.Fatalf("built %d edits, want 2", len(edits))
	}
	if edits[0].Span.Overlaps(edits[1].Span) {
		t.Fatalf("the link dest %v still overlaps the image dest %v", edits[0].Span, edits[1].Span)
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "See [![](./pics/shot.png)](./pics/shot.png) here.\n"
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

// linkLabelCases cover a DIRECTIVE'S LABEL, which is content rather than
// decoration: the conversion path encodes a link written there as a link, so
// a rewriter of destinations that could not see one would leave it pointing
// at the old target after it reported success.
//
// Every case carries the forms that ALREADY reported beside the one that did
// not. A `::leaf[…]` and a `:::container[…]` label are parsed in place, so a
// link in them has always been located here; only the TEXT form's label is
// parsed against a DETACHED COPY of its bytes, whose offsets address the
// copy. Reporting those unshifted would name bytes near the start of the
// document, and skipping the root — which is what this view did — loses the
// link and leaves UnlocatedLinks at 0, so the document reads as "no links"
// instead of "a link I could not place".
//
// Every want is the label's absolute extent in the source, so a shift that
// is off by the bracket, or omitted, fails these rather than passing with
// plausible-looking spans.
var linkLabelCases = []linkCase{{
	name: "all three directive forms in one document",
	src:  "Text :sup[[t](https://ex.com/t)] end\n\n::leaf[[l](https://ex.com/l)]\n\n:::box[[c](https://ex.com/c)]\nbody\n:::\n",
	want: []string{
		"[t](https://ex.com/t)|t|https://ex.com/t",
		"[l](https://ex.com/l)|l|https://ex.com/l",
		"[c](https://ex.com/c)|c|https://ex.com/c",
	},
}, {
	// A label parsed inside a label: the recursion runs in each copy's own
	// coordinates and the two shifts compose.
	name: "a label inside a label composes the shifts",
	src:  "Text :sup[a :sub[[t](https://ex.com/t)] b] end\n",
	want: []string{"[t](https://ex.com/t)|t|https://ex.com/t"},
}, {
	// The ordering claim: a shifted span has to sort with the unshifted
	// ones, not after them.
	name: "a label link and a prose link in document order",
	src:  "Text :sup[[a](https://ex.com/a)] and [b](https://ex.com/b)\n",
	want: []string{
		"[a](https://ex.com/a)|a|https://ex.com/a",
		"[b](https://ex.com/b)|b|https://ex.com/b",
	},
}, {
	name: "a label link inside a blockquote",
	src:  "> q :sup[[t](https://ex.com/t)]\n",
	want: []string{"[t](https://ex.com/t)|t|https://ex.com/t"},
}, {
	// A tab in the container prefix is what hides a definition from its own
	// view. It does not reach here: the label's offset is recovered from the
	// directive's extent in THIS source and verified against it.
	name: "a label link on a tab-prefixed continuation line",
	src:  "> q\n>\t:sup[[t](https://ex.com/t)]\n",
	want: []string{"[t](https://ex.com/t)|t|https://ex.com/t"},
}, {
	name: "a label link in a table cell",
	src:  "| :sup[[t](https://ex.com/t)] | b |\n| --- | --- |\n| c | d |\n",
	want: []string{"[t](https://ex.com/t)|t|https://ex.com/t"},
}, {
	// The one span from this view that overlaps one from Images does so
	// inside a label too, and both are shifted by the same offset.
	name: "a thumbnail link in a label reports in both views",
	src:  "Text :sup[[![a](i.png)](https://ex.com/t)] end\n",
	want: []string{"[![a](i.png)](https://ex.com/t)|![a](i.png)|https://ex.com/t"},
}, {
	// The carve-outs hold inside a label too, because the label is parsed
	// with the same parser.
	name: "a link in inline code inside a label is not a link",
	src:  "Text :sup[`[t](https://ex.com/t)`] end\n",
	want: nil,
}, {
	name: "an empty label reports nothing and does not hide prose",
	src:  "Text :sup[] and [b](x.md)\n",
	want: []string{"[b](x.md)|b|x.md"},
}}

// TestLinks_DirectiveLabels drives linkLabelCases. See that table for what
// each shape proves, and TestImages_DirectiveLabels for the same fix on the
// other destination view — one root cause, one descent, two views.
//
// Measured with the label descent removed from collectLinks, which is what
// this view did before:
//
//	--- FAIL: TestLinks_DirectiveLabels (0.00s)
//	    --- FAIL: TestLinks_DirectiveLabels/all_three_directive_forms_in_one_document (0.00s)
//	        Links("Text :sup[[t](https://ex.com/t)] end\n\n::leaf[[l](https://ex.com/l)]\n\n:::box[[c](https://ex.com/c)]\nbody\n:::\n") =
//	             ["[l](https://ex.com/l)|l|https://ex.com/l" "[c](https://ex.com/c)|c|https://ex.com/c"]
//	            want
//	             ["[t](https://ex.com/t)|t|https://ex.com/t" "[l](https://ex.com/l)|l|https://ex.com/l" "[c](https://ex.com/c)|c|https://ex.com/c"]
//	    --- FAIL: TestLinks_DirectiveLabels/a_label_link_on_a_tab-prefixed_continuation_line (0.00s)
//	        Links("> q\n>\t:sup[[t](https://ex.com/t)]\n") =
//	             []
//	            want
//	             ["[t](https://ex.com/t)|t|https://ex.com/t"]
//	    --- FAIL: TestLinks_DirectiveLabels/a_label_link_in_a_table_cell (0.00s)
//	        Links("| :sup[[t](https://ex.com/t)] | b |\n| --- | --- |\n| c | d |\n") =
//	             []
//	            want
//	             ["[t](https://ex.com/t)|t|https://ex.com/t"]
//	    --- FAIL: TestLinks_DirectiveLabels/a_thumbnail_link_in_a_label_reports_in_both_views (0.00s)
//	        Links("Text :sup[[![a](i.png)](https://ex.com/t)] end\n") =
//	             []
//	            want
//	             ["[![a](i.png)](https://ex.com/t)|![a](i.png)|https://ex.com/t"]
//	    --- FAIL: TestLinks_DirectiveLabels/a_label_link_and_a_prose_link_in_document_order (0.00s)
//	        Links("Text :sup[[a](https://ex.com/a)] and [b](https://ex.com/b)\n") =
//	             ["[b](https://ex.com/b)|b|https://ex.com/b"]
//	            want
//	             ["[a](https://ex.com/a)|a|https://ex.com/a" "[b](https://ex.com/b)|b|https://ex.com/b"]
//	    --- FAIL: TestLinks_DirectiveLabels/a_label_inside_a_label_composes_the_shifts (0.00s)
//	        Links("Text :sup[a :sub[[t](https://ex.com/t)] b] end\n") =
//	             []
//	            want
//	             ["[t](https://ex.com/t)|t|https://ex.com/t"]
//	    --- FAIL: TestLinks_DirectiveLabels/a_label_link_inside_a_blockquote (0.00s)
//	        Links("> q :sup[[t](https://ex.com/t)]\n") =
//	             []
//	            want
//	             ["[t](https://ex.com/t)|t|https://ex.com/t"]
//
// The rows that do NOT appear there passed on both versions and are the good
// cases: the inline-code carve-out, the empty label, and — inside the
// three-form document — the leaf and the container links, whose spans a
// wrongly composed shift would have moved. No OTHER test in this package
// failed under the mutation, which is the measurement that says this view's
// hole was untested rather than merely unfixed.
func TestLinks_DirectiveLabels(t *testing.T) {
	t.Parallel()
	for _, c := range linkLabelCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := markdown.NewSource([]byte(c.src))
			if got := linkTexts(c.src, s.Links()); !eq(got, c.want) {
				t.Errorf("Links(%q) =\n %q\nwant\n %q", c.src, got, c.want)
			}
			if u := s.UnlocatedLinks(); u != 0 {
				t.Errorf("UnlocatedLinks() = %d, want 0", u)
			}
		})
	}
}
