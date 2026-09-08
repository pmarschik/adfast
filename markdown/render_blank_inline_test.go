package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

// A typed dialect chip carries no plain text: a status, a media chip and an
// emoji all render as something while projecting to "". The blank-paragraph
// suppression used to measure a wrapper — emphasis, strong, strikethrough or a
// link label — by that same projection, so a chip alone inside one read as an
// empty paragraph and the whole paragraph was dropped. The author's line
// simply vanished, silently, on the faithful leg as well as the format leg, so
// no diff showed anything to review.
//
// Both halves of the rule live in ONE document here on purpose: the shapes
// that used to disappear and the shapes that always survived have to hold
// together, because the bug was never "chips vanish" but "a chip is judged one
// way at paragraph level and another way one level deeper".
func TestRender_InlineWrapperAroundATextlessChipKeepsItsParagraph(t *testing.T) {
	lines := []string{
		// The seven shapes that used to delete the paragraph whole.
		"*:status{color=red}*",
		"**:status{color=red}**",
		"~~:status{color=red}~~",
		"[:status{color=red}](x)",
		"*:media{id=abc}*",
		"*:emoji{id=1f600 text=:grinning:}*",
		"[:media{id=abc}](https://e.com)",
		// The boundary that scoped the bug — these were always fine and stay
		// fine: a chip at paragraph level, a chip with any text sibling, an
		// unknown directive name, and a chip fused to leading text.
		":status{color=red}",
		"a *:status{color=red}* b",
		"*:status{color=red}* b",
		"*:foo{a=b}*",
		"*x:status{color=red}*",
	}
	want := strings.Join([]string{
		`_:status{color="red"}_`,
		`**:status{color="red"}**`,
		`~~:status{color="red"}~~`,
		`[:status{color="red"}](x)`,
		`_:media{#abc}_`,
		`_:emoji{#1f600 text=":grinning:"}_`,
		`[:media{#abc}](https://e.com)`,
		`:status{color="red"}`,
		`a _:status{color="red"}_ b`,
		`_:status{color="red"}_ b`,
		`_:foo{a="b"}_`,
		`_x:status{color="red"}_`,
	}, "\n\n") + "\n"

	src := strings.Join(lines, "\n\n") + "\n"
	got := Render(Parse([]byte(src)))
	if got != want {
		t.Errorf("Render =\n%q\nwant\n%q", got, want)
	}
	// Not one paragraph fewer than went in: the count is the defect.
	if gotN, wantN := strings.Count(got, "\n\n")+1, len(lines); gotN != wantN {
		t.Errorf("rendered %d paragraphs, want %d — a line was dropped", gotN, wantN)
	}
	if again := Render(Parse([]byte(got))); again != got {
		t.Errorf("render not stable: %q then %q", got, again)
	}
}

// PIN — measured green against the pre-fix hasVisibleInline, so this records
// preserved behavior, not the fix. A wrapper with nothing visible inside it
// stays invisible: recursing into the children must not turn an empty
// emphasis, strong, strikethrough or link label into content. Markdown cannot
// write these (`**` parses as literal asterisks), so they are built by hand —
// which is also how ADF's own empty shapes arrive.
func TestRender_GenuinelyEmptyInlineWrapperIsStillBlank(t *testing.T) {
	doc := func(inline ast.Node) ast.Node {
		return &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{inline}}}}
	}
	cases := []struct {
		root ast.Node
		name string
	}{
		{name: "emphasis with no children", root: doc(&ast.Emphasis{})},
		{name: "strong with no children", root: doc(&ast.Strong{})},
		{name: "strikethrough with no children", root: doc(&ast.Delete{})},
		{name: "link with an empty label", root: doc(&ast.Link{URL: "x"})},
		{name: "emphasis over an empty text node", root: doc(&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: ""}}})},
		{name: "emphasis nested around nothing", root: doc(&ast.Emphasis{Children: []ast.Node{&ast.Emphasis{}}})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(tc.root); got != "\n" {
				t.Errorf("Render = %q, want %q", got, "\n")
			}
		})
	}
}

// PIN — measured green against the pre-fix hasVisibleInline. This is the case
// withoutBlankParagraphs exists to serve, and the fix must not trade one bug
// for the other: ADF carries empty paragraphs as spacing, Markdown has no way
// to write one, and rendering them would make the output re-parse to a
// different tree. They still render to nothing, alone or between blocks.
func TestRender_EmptyADFParagraphStillRendersToNothing(t *testing.T) {
	text := func(s string) adf.Node { return &adf.Text{Text: s} }
	cases := []struct {
		name string
		want string
		doc  adf.Doc
	}{
		{
			name: "a lone empty paragraph",
			doc:  adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Paragraph{}}},
			want: "\n",
		},
		{
			name: "spacing between two blocks",
			doc: adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{text("a")}},
				&adf.Paragraph{},
				&adf.Paragraph{Content: []adf.Node{text("b")}},
			}},
			want: "a\n\nb\n",
		},
		{
			name: "a paragraph holding only empty text",
			doc: adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{text("")}},
				&adf.Paragraph{Content: []adf.Node{text("a")}},
			}},
			want: "a\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(convert.FromADF(tc.doc)); got != tc.want {
				t.Errorf("Render = %q, want %q", got, tc.want)
			}
		})
	}
}

// The same depth asymmetry reached shapes with no dialect in them at all: an
// image carries no plain text either, so a link whose label is nothing but an
// image lost its paragraph — plain CommonMark, no extension involved. The
// alt-text row is the control: it survived before the fix only because the alt
// text projected to something.
func TestRender_ImageOnlyLinkLabelKeepsItsParagraph(t *testing.T) {
	cases := []struct{ src, want string }{
		{"[![](i.png)](x)", "[![](i.png)](x)\n"},
		{"*![](i.png)*", "_![](i.png)_\n"},
		{"[![alt](i.png)](x)", "[![alt](i.png)](x)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := Render(Parse([]byte(tc.src))); got != tc.want {
				t.Errorf("Render(%q) = %q, want %q", tc.src, got, tc.want)
			}
		})
	}
}
