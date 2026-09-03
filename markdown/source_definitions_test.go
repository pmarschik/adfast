package markdown_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/markdown"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Source.Definitions is a NEW view, so there is no "before" that fails: no
// call site could regress, because none existed. What stands in for a
// regression test is the pin below, and it is written to be OFF-BY-ONE
// SENSITIVE by construction — every expectation is the bytes the span
// SLICES out of the source, never the offset integers, and
// TestDefinitions_PinsAreOffByOneSensitive proves the slicing catches a
// shift that an offset comparison would sail past.

// definitionCase is one document and what every part of every definition in
// it slices to.
type definitionCase struct {
	name string
	src  string
	// want holds one row per definition, in document order. An absent
	// title is the sentinel noTitle, which is not a string a title can
	// hold, so the two answers cannot be confused.
	want []definitionParts
}

// definitionParts is what one definition's four spans slice to.
type definitionParts struct {
	whole, label, dest, title string
}

// noTitle is what a definition with no title written reports, as text: the
// zero Span slices the empty prefix of the source, and this sentinel is what
// distinguishes it from a title written EMPTY.
const noTitle = "\x00no title\x00"

var definitionCases = []definitionCase{
	{
		name: "top level, no title",
		src:  "See [spec].\n\n[spec]: https://ex.com/spec\n",
		want: []definitionParts{{
			whole: "[spec]: https://ex.com/spec",
			label: "spec", dest: "https://ex.com/spec", title: noTitle,
		}},
	},
	{
		name: "no destination separator",
		src:  "[a]:x.md\n\n[a]\n",
		want: []definitionParts{{whole: "[a]:x.md", label: "a", dest: "x.md", title: noTitle}},
	},
	{
		name: "unterminated last line",
		src:  "[a]: x.md",
		want: []definitionParts{{whole: "[a]: x.md", label: "a", dest: "x.md", title: noTitle}},
	},
	{
		name: "three-space indent, which CommonMark allows",
		src:  "   [a]: x.md\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md", label: "a", dest: "x.md", title: noTitle}},
	},
	{
		name: "trailing whitespace is outside every span",
		src:  "[a]: x.md   \n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md", label: "a", dest: "x.md", title: noTitle}},
	},

	// --- the three title quotings CommonMark allows ---
	{
		name: "double-quoted title",
		src:  "[a]: x.md \"dq\"\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md \"dq\"", label: "a", dest: "x.md", title: "dq"}},
	},
	{
		name: "single-quoted title",
		src:  "[a]: x.md 'sq'\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md 'sq'", label: "a", dest: "x.md", title: "sq"}},
	},
	{
		name: "parenthesized title",
		src:  "[a]: x.md (par)\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md (par)", label: "a", dest: "x.md", title: "par"}},
	},
	{
		name: "empty title, between its delimiters",
		src:  "[a]: x.md \"\"\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md \"\"", label: "a", dest: "x.md", title: ""}},
	},
	{
		name: "escaped delimiter inside the title",
		src:  "[a]: x.md \"a \\\" b\"\n\n[a]\n",
		want: []definitionParts{{
			whole: "[a]: x.md \"a \\\" b\"", label: "a", dest: "x.md", title: "a \\\" b",
		}},
	},

	// --- destinations ---
	{
		name: "angle-bracketed destination keeps its wrapper outside the span",
		src:  "[a]: <a b.md> \"t\"\n\n[a]\n",
		want: []definitionParts{{
			whole: "[a]: <a b.md> \"t\"", label: "a", dest: "a b.md", title: "t",
		}},
	},
	{
		name: "empty angle-bracketed destination",
		src:  "[a]: <>\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: <>", label: "a", dest: "", title: noTitle}},
	},
	{
		name: "destination on the line after the colon",
		src:  "[a]:\n  x.md\n  \"t\"\n\n[a]\n",
		want: []definitionParts{{
			whole: "[a]:\n  x.md\n  \"t\"", label: "a", dest: "x.md", title: "t",
		}},
	},

	// --- containers ---
	{
		name: "inside a blockquote",
		src:  "> See [spec].\n>\n> [spec]: https://ex.com/spec \"T\"\n",
		want: []definitionParts{{
			whole: "[spec]: https://ex.com/spec \"T\"",
			label: "spec", dest: "https://ex.com/spec", title: "T",
		}},
	},
	{
		name: "inside a list item",
		src:  "- See [spec].\n\n  [spec]: https://ex.com/spec\n",
		want: []definitionParts{{
			whole: "[spec]: https://ex.com/spec",
			label: "spec", dest: "https://ex.com/spec", title: noTitle,
		}},
	},
	{
		name: "blockquote prefix inside a title that crosses a line",
		src:  "> [a]: x.md \"one\n> two\"\n\n> [a]\n",
		want: []definitionParts{{
			whole: "[a]: x.md \"one\n> two\"", label: "a", dest: "x.md", title: "one\n> two",
		}},
	},
	{
		name: "one definition per container",
		src:  "[a]: /a\n> [a]: /b\n\n[a]\n",
		want: []definitionParts{
			{whole: "[a]: /a", label: "a", dest: "/a", title: noTitle},
			{whole: "[a]: /b", label: "a", dest: "/b", title: noTitle},
		},
	},

	// --- a label that crosses a line, which CommonMark allows ---
	{
		name: "folded label at the top level",
		src:  "[one\ntwo]: x.md\n\n[one two]\n",
		want: []definitionParts{{
			whole: "[one\ntwo]: x.md", label: "one\ntwo", dest: "x.md", title: noTitle,
		}},
	},
	{
		name: "folded label in a blockquote keeps the prefix in the span",
		src:  "> [one\n> two]: x.md\n\n> [one two]\n",
		want: []definitionParts{{
			whole: "[one\n> two]: x.md", label: "one\n> two", dest: "x.md", title: noTitle,
		}},
	},
	{
		name: "folded label in a list item keeps the indent in the span",
		src:  "- [one\n  two]: x.md\n\n- [one two]\n",
		want: []definitionParts{{
			whole: "[one\n  two]: x.md", label: "one\n  two", dest: "x.md", title: noTitle,
		}},
	},
	{
		name: "escaped bracket inside the label",
		src:  "[a\\]b]: x.md\n\n[a\\]b]\n",
		want: []definitionParts{{
			whole: "[a\\]b]: x.md", label: "a\\]b", dest: "x.md", title: noTitle,
		}},
	},
	{
		name: "emphasis in a label is label bytes, not markup",
		src:  "[*a*]: x.md\n\n[*a*]\n",
		want: []definitionParts{{whole: "[*a*]: x.md", label: "*a*", dest: "x.md", title: noTitle}},
	},

	// --- duplicate labels, which is the whole reason for one span each ---
	{
		name: "duplicate label reports every written definition",
		src:  "[a]: one.md\n[a]: two.md\n[a]: three.md\n\n[a]\n",
		want: []definitionParts{
			{whole: "[a]: one.md", label: "a", dest: "one.md", title: noTitle},
			{whole: "[a]: two.md", label: "a", dest: "two.md", title: noTitle},
			{whole: "[a]: three.md", label: "a", dest: "three.md", title: noTitle},
		},
	},
	{
		name: "labels that normalize to one identifier are still two definitions",
		src:  "[Foo bar]: one.md\n[foo   BAR]: two.md\n\n[FOO Bar]\n",
		want: []definitionParts{
			{whole: "[Foo bar]: one.md", label: "Foo bar", dest: "one.md", title: noTitle},
			{whole: "[foo   BAR]: two.md", label: "foo   BAR", dest: "two.md", title: noTitle},
		},
	},

	// --- what is not a definition ---
	{name: "inside a fenced code block", src: "```\n[a]: x.md\n```\n"},
	{name: "inside an indented code block", src: "    [a]: x.md\n"},
	{name: "continuing a paragraph", src: "para\n[a]: x.md\n\n[a]\n"},
	{name: "junk after the title", src: "[a]: x.md \"t\" trailing\n\n[a]\n"},
	{name: "unclosed title", src: "[a]: x.md \"unclosed\nmore\n\n[a]\n"},
	{name: "blank label", src: "[ ]: x.md\n\n[ ]\n"},
	{name: "nested bracket in the label", src: "[a[b]: x.md\n\n[a[b]\n"},
	{name: "inline code that looks like one", src: "`[a]: x.md`\n"},
	{
		name: "a quoted string in a later paragraph is not this title",
		src:  "[a]: x.md\n\n\"\"\n\n[a]\n",
		want: []definitionParts{{whole: "[a]: x.md", label: "a", dest: "x.md", title: noTitle}},
	},
}

// text returns what sp slices out of src, with an absent title reported as
// the noTitle sentinel rather than as the empty string the zero Span yields.
func partText(src string, sp markdown.Span, isTitle bool) string {
	if isTitle && sp == (markdown.Span{}) {
		return noTitle
	}
	return src[sp.Start:sp.Stop]
}

// TestDefinitions_SliceToTheirWrittenParts is the pin. Each expectation is
// the bytes the returned span slices out of the source, so an implementation
// whose offsets are shifted — uniformly or in one part — fails here even
// though its arithmetic is self-consistent.
func TestDefinitions_SliceToTheirWrittenParts(t *testing.T) {
	for _, tc := range definitionCases {
		t.Run(tc.name, func(t *testing.T) {
			got := markdown.Definitions([]byte(tc.src))
			if len(got) != len(tc.want) {
				t.Fatalf("Definitions(%q) = %d definitions, want %d: %v",
					tc.src, len(got), len(tc.want), sliced(tc.src, got))
			}
			for i, w := range tc.want {
				d := got[i]
				parts := definitionParts{
					whole: partText(tc.src, d.Span, false),
					label: partText(tc.src, d.Label, false),
					dest:  partText(tc.src, d.Dest, false),
					title: partText(tc.src, d.Title, true),
				}
				if parts != w {
					t.Errorf("definition %d of %q sliced to %+v, want %+v", i, tc.src, parts, w)
				}
			}
		})
	}
}

// sliced renders what a definition list slices to, for a failure message.
func sliced(src string, defs []markdown.Definition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, fmt.Sprintf("%q", src[d.Span.Start:d.Span.Stop]))
	}
	return out
}

// TestDefinitions_PinsAreOffByOneSensitive proves the pin above can fail.
// Every span of every case is nudged by one byte in each direction, and the
// nudged span must slice to something OTHER than the expectation — otherwise
// the pin would pass for an implementation off by one, which is the failure
// mode an offset-only assertion cannot see.
//
// The nudge is applied to the span, not to the implementation, because the
// implementation is what is being trusted: shifting the assertion's own input
// is the only way to measure the assertion's sensitivity without a second
// implementation to disagree with.
//
// An EMPTY span is the one shape text cannot pin, because it slices to "" at
// every offset: its position is its whole content. Those are excluded here
// and pinned by their neighbor bytes instead — see
// TestDefinitions_EmptyPartsSitBetweenTheirDelimiters.
func TestDefinitions_PinsAreOffByOneSensitive(t *testing.T) {
	checked := 0
	for _, tc := range definitionCases {
		for i, d := range markdown.Definitions([]byte(tc.src)) {
			for _, part := range namedParts(d, tc.want[i]) {
				checked += shiftedSpanDiffers(t, tc.name, tc.src, part)
			}
		}
	}
	// A guard on the guard: a loop that checked nothing would pass.
	if checked < 100 {
		t.Fatalf("only %d shifted spans checked, expected the whole table", checked)
	}
}

// namedPart pairs one of a definition's spans with the text it is pinned to.
//
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type namedPart struct {
	name string
	want string
	span markdown.Span
}

// namedParts pairs each of a definition's four spans with its expectation.
func namedParts(d markdown.Definition, w definitionParts) []namedPart {
	return []namedPart{
		{name: "Span", span: d.Span, want: w.whole},
		{name: "Label", span: d.Label, want: w.label},
		{name: "Dest", span: d.Dest, want: w.dest},
		{name: "Title", span: d.Title, want: w.title},
	}
}

// shiftedSpanDiffers nudges one span by a byte in each direction and reports
// how many nudges it could check. A nudge that still slices to the
// expectation means the expectation cannot see an off-by-one.
func shiftedSpanDiffers(t *testing.T, name, src string, part namedPart) int {
	t.Helper()
	if part.want == noTitle || part.span.Len() == 0 {
		return 0
	}
	checked := 0
	for _, by := range []int{-1, 1} {
		start, stop := part.span.Start+by, part.span.Stop+by
		if start < 0 || stop > len(src) {
			continue
		}
		if got := src[start:stop]; got == part.want {
			t.Errorf("%s: %s shifted by %+d still slices to %q — the pin is blind to an off-by-one",
				name, part.name, by, got)
		}
		checked++
	}
	return checked
}

// TestDefinitions_DelimitersSitOutsideTheSpans pins the convention the doc
// comments promise, from the OTHER side: the byte just outside a span is the
// delimiter the span excludes. A span one byte too wide would report the
// delimiter as content, and one byte too narrow would leave a content byte
// outside — and the neighbor check catches the shift even when the sliced
// text happens to look plausible.
func TestDefinitions_DelimitersSitOutsideTheSpans(t *testing.T) {
	const src = "[lbl]: <a b.md> 'ttl'\n\n[lbl]\n"
	got := markdown.Definitions([]byte(src))
	if len(got) != 1 {
		t.Fatalf("Definitions = %d, want 1", len(got))
	}
	d := got[0]
	for _, c := range []struct {
		name        string
		span        markdown.Span
		before, aft byte
	}{
		{"Label", d.Label, '[', ']'},
		{"Dest", d.Dest, '<', '>'},
		{"Title", d.Title, '\'', '\''},
	} {
		if b := src[c.span.Start-1]; b != c.before {
			t.Errorf("%s: byte before the span is %q, want %q", c.name, b, c.before)
		}
		if b := src[c.span.Stop]; b != c.aft {
			t.Errorf("%s: byte after the span is %q, want %q", c.name, b, c.aft)
		}
	}
	// The whole span starts AT the `[` and ends just past the title's
	// closing delimiter, so neither neighbor is part of the definition.
	if b := src[d.Span.Start]; b != '[' {
		t.Errorf("Span starts at %q, want %q", b, byte('['))
	}
	if b := src[d.Span.Stop]; b != '\n' {
		t.Errorf("Span ends before %q, want a newline", b)
	}
}

// TestDefinitions_EmptyPartsSitBetweenTheirDelimiters pins the two shapes an
// empty span reports, which the text pin cannot see: `<>` is an empty
// destination and `""` is an empty title, and each has to sit BETWEEN its
// delimiters so that an Edit there inserts content rather than replacing the
// wrapper. An offset one byte out puts the insertion outside the delimiter,
// which the neighbor bytes are what detect.
func TestDefinitions_EmptyPartsSitBetweenTheirDelimiters(t *testing.T) {
	for _, tc := range []struct {
		part        func(markdown.Definition) markdown.Span
		name, src   string
		insert      string
		want        string
		before, aft byte
	}{
		{
			name: "empty destination", src: "[a]: <> \"t\"\n\n[a]\n",
			part:   func(d markdown.Definition) markdown.Span { return d.Dest },
			before: '<', aft: '>',
			insert: "x.md", want: "[a]: <x.md> \"t\"\n\n[a]\n",
		},
		{
			name: "empty title", src: "[a]: x.md \"\"\n\n[a]\n",
			part:   func(d markdown.Definition) markdown.Span { return d.Title },
			before: '"', aft: '"',
			insert: "t", want: "[a]: x.md \"t\"\n\n[a]\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			got := s.Definitions()
			if len(got) != 1 {
				t.Fatalf("Definitions = %d, want 1", len(got))
			}
			sp := tc.part(got[0])
			if sp.Len() != 0 {
				t.Fatalf("span %v is not empty", sp)
			}
			if b := tc.src[sp.Start-1]; b != tc.before {
				t.Errorf("byte before the span is %q, want %q", b, tc.before)
			}
			if b := tc.src[sp.Stop]; b != tc.aft {
				t.Errorf("byte after the span is %q, want %q", b, tc.aft)
			}
			// The operation the offset exists for: an insertion at an empty
			// span, which lands one byte off if the span does.
			out, err := s.Apply(markdown.Edit{Span: sp, Text: tc.insert})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("inserting %q at the empty span gave %q, want %q",
					tc.insert, out, tc.want)
			}
		})
	}
}

// TestDefinitions_RestOnANodeGoldmarkLeavesInTheTree pins the fact the whole
// view is built on, because the repo once recorded the opposite: that
// goldmark "consumes a definition into its reference map rather than the
// tree, so no node carries its position". It does BOTH. The transformer
// registers the definition in the parse context AND inserts a
// LinkReferenceDefinition node before the paragraph it came out of, at the
// source position it was written — inside a blockquote and inside a list item
// as well as at the top level.
//
// The measurement is over goldmark's own tree rather than over this package's
// view, so it fails if the upstream behavior ever changes rather than only
// when this package's walk does. That is the point: the belief that no node
// existed is what made the destination of a definition look unlocatable.
func TestDefinitions_RestOnANodeGoldmarkLeavesInTheTree(t *testing.T) {
	for _, tc := range []struct {
		name, src, parent string
		at                int
	}{
		{name: "top level", src: "See [s].\n\n[s]: x.md\n", parent: "Document", at: 10},
		{name: "blockquote", src: "> See [s].\n>\n> [s]: x.md\n", parent: "Blockquote", at: 15},
		{name: "list item", src: "- See [s].\n\n  [s]: x.md\n", parent: "ListItem", at: 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(tc.src)
			found, parents := goldmarkDefinitionNodes(
				markdown.NewParser().Parse(text.NewReader(src)))
			if len(found) != 1 {
				t.Fatalf("goldmark left %d definition nodes in the tree, want 1", len(found))
			}
			if parents[0] != tc.parent {
				t.Errorf("definition node sits under %s, want %s", parents[0], tc.parent)
			}
			if got := found[0].Pos(); got != tc.at {
				t.Errorf("definition node Pos() = %d, want %d (the `[`)", got, tc.at)
			}
			if b := src[found[0].Pos()]; b != '[' {
				t.Errorf("definition node Pos() addresses %q, want %q", b, byte('['))
			}
		})
	}
}

// goldmarkDefinitionNodes returns every LinkReferenceDefinition node in the
// goldmark tree, in document order, with the kind of the parent each one sits
// under. Inline subtrees are skipped, because a definition is a block.
func goldmarkDefinitionNodes(
	doc gast.Node,
) (found []*gast.LinkReferenceDefinition, parents []string) {
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if def, ok := c.(*gast.LinkReferenceDefinition); ok {
				found = append(found, def)
				parents = append(parents, c.Parent().Kind().String())
			}
			if c.Type() != gast.TypeInline {
				walk(c)
			}
		}
	}
	walk(doc)
	return found, parents
}

// TestDefinitions_ReportEveryWrittenDefinitionUnlikeTheReferenceMap pins the
// premise this view rests on. goldmark's reference map — the thing that
// resolves a use to a destination — applies CommonMark's first-wins rule and
// keeps ONE entry per normalized label. A rewriter needs one span per WRITTEN
// definition, and this is the measurement that says the map cannot supply
// them.
func TestDefinitions_ReportEveryWrittenDefinitionUnlikeTheReferenceMap(t *testing.T) {
	const src = "[a]: one.md\n[A]: two.md\n[ a ]: three.md\n\n[a]\n"
	pc := parser.NewContext()
	markdown.NewParser().Parse(text.NewReader([]byte(src)), parser.WithContext(pc))
	if n := len(pc.References()); n != 1 {
		t.Fatalf("goldmark reference map holds %d entries, want the 1 that makes this view necessary", n)
	}
	got := markdown.Definitions([]byte(src))
	if len(got) != 3 {
		t.Fatalf("Definitions = %d, want 3 (one per written definition): %v", len(got), sliced(src, got))
	}
	for i, want := range []string{"one.md", "two.md", "three.md"} {
		if s := src[got[i].Dest.Start:got[i].Dest.Stop]; s != want {
			t.Errorf("definition %d destination = %q, want %q", i, s, want)
		}
	}
}

// TestDefinitions_PivotASTNodesCarryNoOffsets pins the other half of the
// premise: the pivot AST's definition nodes hold the same three values, and
// hold them DECODED, but hold no position — so a walk over them cannot locate
// anything to rewrite. The escape below is what makes "decoded" visible: the
// node reports the destination a renderer wants, and the span reports the
// bytes on disk.
func TestDefinitions_PivotASTNodesCarryNoOffsets(t *testing.T) {
	const src = "[a]: x\\_y.md \"t\"\n\n[a]\n"
	var nodes []*ast.Definition
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if d, ok := n.(*ast.Definition); ok {
			nodes = append(nodes, d)
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	walk(markdown.Parse([]byte(src)))
	if len(nodes) != 1 {
		t.Fatalf("pivot definitions = %d, want 1", len(nodes))
	}
	if nodes[0].URL != "x_y.md" {
		t.Errorf("pivot URL = %q, want the decoded %q", nodes[0].URL, "x_y.md")
	}
	got := markdown.Definitions([]byte(src))
	if len(got) != 1 {
		t.Fatalf("Definitions = %d, want 1", len(got))
	}
	if s := src[got[0].Dest.Start:got[0].Dest.Stop]; s != "x\\_y.md" {
		t.Errorf("Dest slices to %q, want the written %q", s, "x\\_y.md")
	}
}

// TestDefinitions_ComposeWithApply is the operation the view exists for: a
// rewriter that replaces every definition's destination and nothing else, in
// one splice. It is also the test that fails on an overlap or a stale offset,
// because Apply validates the edit set before it writes anything.
func TestDefinitions_ComposeWithApply(t *testing.T) {
	const src = "See [a] and [b].\n\n[a]: <old one.md> \"t\"\n> [b]: old-two.md\n"
	s := markdown.NewSource([]byte(src))
	var edits []markdown.Edit
	for _, d := range s.Definitions() {
		edits = append(edits, markdown.Edit{Span: d.Dest, Text: "new.md"})
	}
	if len(edits) != 2 {
		t.Fatalf("definitions = %d, want 2", len(edits))
	}
	out, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "See [a] and [b].\n\n[a]: <new.md> \"t\"\n> [b]: new.md\n"
	if string(out) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", out, want)
	}
	// The rewrite has to survive a re-parse as the same shape: an edit that
	// widened past the angle brackets or over the title would change what
	// the destinations are.
	for i, d := range markdown.Definitions(out) {
		if s := string(out[d.Dest.Start:d.Dest.Stop]); s != "new.md" {
			t.Errorf("after Apply, definition %d destination = %q, want %q", i, s, "new.md")
		}
	}
}

// TestDefinitions_ExcludeADefinitionDestinationFromABareURLRewrite is the
// consuming shape the view was filed for. A normalizer that turns a bare URL
// into an autolink must not touch the URL written as a definition's
// destination, because a `<…>` there is a wrapper and not an autolink — and
// the destination SPAN is the only thing that says which bytes those are.
func TestDefinitions_ExcludeADefinitionDestinationFromABareURLRewrite(t *testing.T) {
	const src = "Visit https://ex.com/one now.\n\n[a]: https://ex.com/two\n\n[a]\n"
	s := markdown.NewSource([]byte(src))
	var protected markdown.Spans
	for _, d := range s.Definitions() {
		protected = append(protected, d.Dest)
	}
	if len(protected) != 1 {
		t.Fatalf("definitions = %d, want 1", len(protected))
	}
	// Both URLs are in the source; only the one outside a definition may be
	// rewritten.
	for _, at := range []struct {
		url       string
		protected bool
	}{
		{"https://ex.com/one", false},
		{"https://ex.com/two", true},
	} {
		i := strings.Index(src, at.url)
		if i < 0 {
			t.Fatalf("%q not in the source", at.url)
		}
		sp := markdown.Span{Start: i, Stop: i + len(at.url)}
		if got := protected.Overlaps(sp); got != at.protected {
			t.Errorf("%q protected = %v, want %v", at.url, got, at.protected)
		}
	}
}

// TestDefinitions_MemoizeOnOneParse pins the contract Source carries for
// every view: the answer is computed once, from the one parse, and a second
// call returns the same slice rather than a fresh walk.
func TestDefinitions_MemoizeOnOneParse(t *testing.T) {
	s := markdown.NewSource([]byte("[a]: x.md\n\n[a]\n"))
	first, second := s.Definitions(), s.Definitions()
	if len(first) != 1 {
		t.Fatalf("Definitions = %d, want 1", len(first))
	}
	if &first[0] != &second[0] {
		t.Error("Definitions re-walked the tree instead of memoizing")
	}
}

// TestDefinitions_SpansAreOrderedAndDisjoint pins the form every view on
// Source promises. Definitions are blocks, so unlike Links and Images no two
// of them can nest, and a report in which two overlap is a misresolved
// extent.
func TestDefinitions_SpansAreOrderedAndDisjoint(t *testing.T) {
	for _, tc := range definitionCases {
		got := markdown.Definitions([]byte(tc.src))
		for i := 1; i < len(got); i++ {
			if got[i].Span.Start < got[i-1].Span.Stop {
				t.Errorf("%s: definition %d at %v overlaps %d at %v",
					tc.name, i, got[i].Span, i-1, got[i-1].Span)
			}
		}
		for i, d := range got {
			for _, part := range []struct {
				name string
				span markdown.Span
			}{{"Label", d.Label}, {"Dest", d.Dest}, {"Title", d.Title}} {
				if part.span == (markdown.Span{}) {
					continue
				}
				if part.span.Start < d.Span.Start || part.span.Stop > d.Span.Stop {
					t.Errorf("%s: definition %d %s at %v is not inside %v",
						tc.name, i, part.name, part.span, d.Span)
				}
			}
			if d.Label.Stop > d.Dest.Start {
				t.Errorf("%s: definition %d label at %v is not before the destination at %v",
					tc.name, i, d.Label, d.Dest)
			}
		}
	}
}
