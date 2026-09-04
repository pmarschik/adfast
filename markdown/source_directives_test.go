package markdown_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// directiveCase is one source plus the exact text every directive must
// select. The rendering is "level:name|whole|attrs", which keeps a failure
// readable: the extent and the attribute block go wrong independently.
type directiveCase struct {
	name string
	src  string
	want []string
}

func spanText(src string, s markdown.Span) string {
	if s == (markdown.Span{}) {
		return "-"
	}
	return src[s.Start:s.Stop]
}

func directiveTexts(src string, ds []markdown.Directive) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, fmt.Sprintf("%d:%s|%s|%s",
			d.Level, d.Name, src[d.Span.Start:d.Span.Stop], spanText(src, d.AttrsSpan)))
	}
	return out
}

// attrTexts renders one directive's written attributes as
// "key=value|whole|keySpan|valueSpan", every field read back out of the
// source so a wrong offset shows up as wrong text rather than a wrong
// number.
func attrTexts(src string, d markdown.Directive) []string {
	out := make([]string, 0, len(d.AttrSpans))
	for _, a := range d.AttrSpans {
		out = append(out, strings.Join([]string{
			a.Key + "=" + a.Value,
			spanText(src, a.Span),
			spanText(src, a.KeySpan),
			spanText(src, a.ValueSpan),
		}, "|"))
	}
	return out
}

var directiveCases = []directiveCase{{
	name: "a leaf directive",
	src:  "::media[shot.png]{#abc width=\"2308\"}\n",
	want: []string{"2:media|::media[shot.png]{#abc width=\"2308\"}|{#abc width=\"2308\"}"},
}, {
	// The extent stops before the newline, unlike CodeSpans and Headings,
	// so replacing a directive does not join it to the block after it.
	name: "a leaf extent excludes its line terminator",
	src:  "::x\nafter\n",
	want: []string{"2:x|::x|-"},
}, {
	name: "a container runs to the end of its closing fence",
	src:  ":::info\nbody\n:::\n",
	want: []string{"3:info|:::info\nbody\n:::|-"},
}, {
	// The buffer mid-keystroke: no CloseFence exists, so the extent falls
	// back to the end of the source.
	name: "an unclosed container ends at the source",
	src:  ":::info\nbody\n",
	want: []string{"3:info|:::info\nbody\n|-"},
}, {
	// A nested container is emitted after the one it sits in, and the
	// closing fence that ends the outer block ends the inner one too — the
	// parser hands the same CloseFence to both, and the extent follows it.
	name: "a nested container",
	src:  "::::outer\n:::inner\nbody\n::::\ntail\n",
	want: []string{
		"3:outer|::::outer\n:::inner\nbody\n::::|-",
		"3:inner|:::inner\nbody\n::::|-",
	},
}, {
	name: "a text directive",
	src:  "a :status[Go]{color=green} b\n",
	want: []string{"1:status|:status[Go]{color=green}|{color=green}"},
}, {
	// A container is emitted before what is nested inside it, which is the
	// order a CodeMirror RangeSetBuilder wants.
	name: "a container comes before the directives inside it",
	src:  ":::warn\ntext :abbr[HTML]{title=\"HyperText\"} more\n:::\n",
	want: []string{
		"3:warn|:::warn\ntext :abbr[HTML]{title=\"HyperText\"} more\n:::|-",
		"1:abbr|:abbr[HTML]{title=\"HyperText\"}|{title=\"HyperText\"}",
	},
}, {
	name: "a leaf inside a blockquote",
	src:  "> ::media{#i}\n",
	want: []string{"2:media|::media{#i}|{#i}"},
}, {
	name: "a container inside a blockquote",
	src:  "> :::note{k='v'}\n> hi\n> :::\n",
	want: []string{"3:note|:::note{k='v'}\n> hi\n> :::|{k='v'}"},
}, {
	name: "a leaf inside a list item",
	src:  "- ::media{#i src=a.png}\n",
	want: []string{"2:media|::media{#i src=a.png}|{#i src=a.png}"},
}, {
	// The extent is the parser's line span, which starts at the first byte
	// of the line — an allowed indent is inside it, the way a container
	// prefix is inside a CodeSpans span.
	name: "an indented leaf keeps its indent inside the extent",
	src:  "  ::x{#a}\n",
	want: []string{"2:x|  ::x{#a}|{#a}"},
}, {
	name: "an empty attribute block is still a block",
	src:  "::x{}\n",
	want: []string{"2:x|::x{}|{}"},
}, {
	name: "a directive with no attribute block",
	src:  "::x[label]\n",
	want: []string{"2:x|::x[label]|-"},
}, {
	// The label rows below all locate the SAME attribute block. The block is
	// found by re-walking the prefix the directive parser walked, so a label
	// the walk misreads shifts the block off the end of the directive and
	// the spans are dropped altogether. This row is the plain form the other
	// rows are measured against: no nesting, no escape.
	name: "a plain label with an attribute block",
	src:  "::note[plain]{c=red}\n",
	want: []string{"2:note|::note[plain]{c=red}|{c=red}"},
}, {
	// A whole written form may stand inside a label, so the label ends at
	// the `]` that MATCHES its `[` and not at the first one on the line.
	name: "a label holding a nested written form",
	src:  "::note[[lab](x.md)]{c=red}\n",
	want: []string{"2:note|::note[[lab](x.md)]{c=red}|{c=red}"},
}, {
	name: "a label holding a nested bracket pair",
	src:  "::note[a[b]c]{c=red}\n",
	want: []string{"2:note|::note[a[b]c]{c=red}|{c=red}"},
}, {
	// A backslash escapes a bracket, so the escaped `]` closes nothing.
	name: "a label holding an escaped bracket",
	src:  "::note[a\\]b]{c=red}\n",
	want: []string{"2:note|::note[a\\]b]{c=red}|{c=red}"},
}, {
	// A backslash escapes a backslash too, so the `]` after one is the
	// label's own closer rather than an escaped bracket.
	name: "a label holding an escaped backslash",
	src:  "::note[a\\\\]{c=red}\n",
	want: []string{"2:note|::note[a\\\\]{c=red}|{c=red}"},
}, {
	// The parser caps a label at 32 nested pairs and this is the deepest
	// label it still accepts, which is the deepest one to locate a block
	// past. A 33rd pair invalidates the label, and with it the directive.
	name: "a label nested as deep as the parser allows",
	src: "::note[" + strings.Repeat("[", 32) + strings.Repeat("]", 32) +
		"]{c=red}\n",
	want: []string{"2:note|::note[" + strings.Repeat("[", 32) +
		strings.Repeat("]", 32) + "]{c=red}|{c=red}"},
}, {
	// An unclosed `[` ends the label attempt at the end of the LINE, never
	// at a `]` written on a later one — the same bound the parser's own scan
	// has, which is why the parser reports no directive here at all.
	//
	// This row and the next one hold whether or not the scan keeps that
	// bound: the parser's verdict already rules the whole line out, and a
	// block that ended past the directive's own extent would be refused
	// anyway. They pin the agreement, not the bound.
	name: "a label with an unclosed bracket is not a label",
	src:  "::note[unbal[\nx]{c=red}\n",
	want: nil,
}, {
	// One pair closes inside the label and the outer `[` is left unclosed,
	// so there is no `]` at balance zero and the whole line is prose.
	name: "a label whose own bracket stays unclosed is not a label",
	src:  "::note[a[b]{c=red}\n",
	want: nil,
}, {
	// A malformed block is not an attribute block at all: the parser leaves
	// the text as prose, so there is no directive to report.
	name: "an unterminated attribute block is not a directive",
	src:  "::x{#a\n",
	want: nil,
}, {
	name: "a directive inside a code fence is not one",
	src:  "```\n::x{#a}\n```\n",
	want: nil,
}, {
	// The label is parsed against its own detached buffer, so anything
	// found under it addresses the wrong source and must never be reported.
	name: "a directive nested in a text directive label is not reported",
	src:  "a :outer[:inner{#a}]{#b} z\n",
	want: []string{"1:outer|:outer[:inner{#a}]{#b}|{#b}"},
}, {
	name: "a document with no directives",
	src:  "# Title\n\nplain text\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestDirectives_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range directiveCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Directives([]byte(c.src))
			if !eq(directiveTexts(c.src, got), c.want) {
				t.Errorf("Directives(%q)\n got %q\nwant %q",
					c.src, directiveTexts(c.src, got), c.want)
			}
		})
	}
}

// attrCase is one attribute block plus the written occurrences it must
// yield, and the map the parser must still agree on.
type attrCase struct {
	name  string
	attrs string
	want  []string
}

var attrCases = []attrCase{{
	name:  "a bare key has no value written",
	attrs: "{collection}",
	want:  []string{"collection=|collection|collection|-"},
}, {
	name:  "an unquoted value",
	attrs: "{color=green}",
	want:  []string{"color=green|color=green|color|green"},
}, {
	// The quotes stay OUTSIDE ValueSpan, so splicing a path with a space in
	// it keeps the quoting the old value needed.
	name:  "a double quoted value excludes its quotes",
	attrs: "{src=\"a b.png\"}",
	want:  []string{"src=a b.png|src=\"a b.png\"|src|a b.png"},
}, {
	name:  "a single quoted value excludes its quotes",
	attrs: "{src='a b.png'}",
	want:  []string{"src=a b.png|src='a b.png'|src|a b.png"},
}, {
	// An empty value IS written, so its span is an empty range where a new
	// value goes rather than the missing-value sentinel.
	name:  "an explicitly empty value has an empty span, not the sentinel",
	attrs: "{src=\"\"}",
	want:  []string{"src=|src=\"\"|src|"},
}, {
	// The key of a shorthand is spelled nowhere, which the zero KeySpan
	// says. The value after the marker is a real range.
	name:  "an id shorthand has no key span",
	attrs: "{#abc}",
	want:  []string{"id=abc|#abc|-|abc"},
}, {
	name:  "a class shorthand has no key span",
	attrs: "{.warn}",
	want:  []string{"class=warn|.warn|-|warn"},
}, {
	// Two occurrences, one map entry: the map says class="a b" and cannot
	// say where either half was written. This is what AttrSpans is for.
	name:  "repeated classes stay separate occurrences",
	attrs: "{.a .b}",
	want:  []string{"class=a|.a|-|a", "class=b|.b|-|b"},
}, {
	name:  "a class shorthand joins onto a written class key",
	attrs: "{class=x .y}",
	want:  []string{"class=x|class=x|class|x", "class=y|.y|-|y"},
}, {
	// Both occurrences are reported although the map keeps only the last.
	name:  "a duplicate key keeps both occurrences",
	attrs: "{a=1 a=2}",
	want:  []string{"a=1|a=1|a|1", "a=2|a=2|a|2"},
}, {
	name:  "padding around the attributes stays outside their spans",
	attrs: "{  #a   b  }",
	want:  []string{"id=a|#a|-|a", "b=|b|b|-"},
}, {
	name:  "a mixed block",
	attrs: "{#abc collection height=\"551\" widthType=pixel}",
	want: []string{
		"id=abc|#abc|-|abc",
		"collection=|collection|collection|-",
		"height=551|height=\"551\"|height|551",
		"widthType=pixel|widthType=pixel|widthType|pixel",
	},
}, {
	name:  "an empty block has no occurrences",
	attrs: "{}",
	want:  nil,
}}

// TestDirectives_AttrSpans drives every attribute shape through all three
// directive forms, because the block is located from a different prefix in
// each and a mistake there shifts every span inside it.
func TestDirectives_AttrSpans(t *testing.T) {
	t.Parallel()
	forms := []struct {
		name string
		fmt  string
	}{
		{"text", "x :n%s y\n"},
		{"leaf", "::n%s\n"},
		{"container", ":::n%s\nbody\n:::\n"},
		{"leaf with a label", "::n[lbl]%s\n"},
		{"leaf in a blockquote", "> ::n%s\n"},
		{"leaf in a list item", "- ::n%s\n"},
		// A label that holds a bracket pair or an escaped bracket ends past
		// the first `]` on the line, so every span inside the block shifts
		// when the label is measured to the wrong closer.
		{"leaf labeled with a nested written form", "::n[[lab](x.md)]%s\n"},
		{"leaf labeled with a nested bracket pair", "::n[a[b]c]%s\n"},
		{"leaf labeled with an escaped bracket", "::n[a\\]b]%s\n"},
	}
	for _, c := range attrCases {
		for _, f := range forms {
			t.Run(c.name+" as a "+f.name, func(t *testing.T) {
				t.Parallel()
				src := fmt.Sprintf(f.fmt, c.attrs)
				ds := markdown.Directives([]byte(src))
				if len(ds) == 0 {
					t.Fatalf("Directives(%q) found none", src)
				}
				if got := attrTexts(src, ds[0]); !eq(got, c.want) {
					t.Errorf("AttrSpans of %q\n got %q\nwant %q", src, got, c.want)
				}
				if ds[0].AttrsSpan == (markdown.Span{}) {
					t.Errorf("AttrsSpan of %q is the zero span", src)
				}
			})
		}
	}
}

// TestDirectives_AttrsStayTheParsersVerdict pins that AttrSpans is only ever
// a second view of Attrs, never a second opinion: folding the occurrences
// back with the parser's rules has to reproduce its map exactly, or the
// spans would be pointing somewhere the caller was not told about.
func TestDirectives_AttrsStayTheParsersVerdict(t *testing.T) {
	t.Parallel()
	src := "::n{#a .b .c k=\"v w\" bare dup=1 dup=2}\n"
	ds := markdown.Directives([]byte(src))
	if len(ds) != 1 {
		t.Fatalf("got %d directives, want 1", len(ds))
	}
	want := map[string]string{"id": "a", "class": "b c", "k": "v w", "bare": "", "dup": "2"}
	for k, v := range want {
		if got := ds[0].Attrs[k]; got != v {
			t.Errorf("Attrs[%q] = %q, want %q", k, got, v)
		}
	}
	if len(ds[0].Attrs) != len(want) {
		t.Errorf("Attrs = %v, want %v", ds[0].Attrs, want)
	}
	// Seven occurrences fold into five map entries: two classes join and a
	// duplicate key is overwritten. The map cannot say where any of the
	// seven were written, which is exactly the gap AttrSpans fills.
	if len(ds[0].AttrSpans) != 7 {
		t.Errorf("got %d written attributes, want 7: %q", len(ds[0].AttrSpans), attrTexts(src, ds[0]))
	}
}

// TestDirectives_ContractHolds pins document order and the containment of
// every attribute span in the block, and of the block in the directive.
//
// The last directive of the source is the shape that spends a trap this
// check used to carry: a TEXT directive whose label cannot be parsed reports
// a zero AttrsSpan, and the containment comparison was written unguarded, so
// `0 < Span.Start` false-fired on a directive that reports nothing to
// locate. Every case in the table before it had a written block, which is
// the only reason the trap had not gone off. The antecedent is Attrs, not
// the span — see the guarded form below and FuzzSourceDirectiveSpans, which
// guards the same check the same way.
//
// With the unguarded comparison restored and this source in place:
//
//	--- FAIL: TestDirectives_ContractHolds (0.00s)
//	    directive 3 attrs {0 0} are not inside its span {54 59}
func TestDirectives_ContractHolds(t *testing.T) {
	t.Parallel()
	src := ":::info{#a}\n::media{#b}\n\ntext :status{#c} tail\n:::\n\nx :note[unbal[ y]{c=red} z\n"
	ds := markdown.Directives([]byte(src))
	if len(ds) != 4 {
		t.Fatalf("got %d directives, want 4: %q", len(ds), directiveTexts(src, ds))
	}
	prev := 0
	for i, d := range ds {
		if d.Span.Start < prev {
			t.Errorf("directive %d = %v is out of document order", i, d.Span)
		}
		prev = d.Span.Start
		if d.Span.Start < 0 || d.Span.Stop > len(src) || d.Span.Len() <= 0 {
			t.Errorf("directive %d span %v is not a range of a %d byte source", i, d.Span, len(src))
		}
		if src[d.Span.Start] != ':' {
			t.Errorf("directive %d starts at %q, want ':'", i, src[d.Span.Start])
		}
		// A directive with no recorded attributes locates no block, and its
		// zero AttrsSpan is not a position to compare against the span.
		if d.Attrs != nil && (d.AttrsSpan.Start < d.Span.Start || d.AttrsSpan.Stop > d.Span.Stop) {
			t.Errorf("directive %d attrs %v are not inside its span %v", i, d.AttrsSpan, d.Span)
		}
		for _, a := range d.AttrSpans {
			if a.Span.Start <= d.AttrsSpan.Start || a.Span.Stop >= d.AttrsSpan.Stop {
				t.Errorf("directive %d attr %v is not inside the braces %v", i, a.Span, d.AttrsSpan)
			}
		}
	}
}

// TestDirectives_TextDirectiveFallbackSpansTheNameOnly pins the case
// Directive.Span's godoc now documents, so the documented claim cannot drift
// from the parser.
//
// A leaf and a container are invalidated outright by a label they cannot
// parse; a TEXT directive instead falls back to a bare `:name` and leaves the
// rest as prose. The trap for a consumer is that the span covers less than a
// reader would call the directive, and that the `{c=red}` still on the page
// is prose rather than attributes — so nil Attrs does not mean no braces are
// written.
//
// The two invalidated forms are in the same table, because "less than the
// reader sees" and "nothing at all" are the two answers this grammar gives
// and a change that merged them would pass a test carrying only one.
func TestDirectives_TextDirectiveFallbackSpansTheNameOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		src  string
		want int
	}{
		{"text form falls back to the bare name", "x :note[unbal[ y]{c=red} z\n", 1},
		{"leaf form is invalidated outright", "::leaf[unbal[ y]{c=red}\n", 0},
		{"container form is invalidated outright", ":::cont[unbal[ y]{c=red}\nbody\n:::\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ds := markdown.Directives([]byte(tc.src))
			if len(ds) != tc.want {
				t.Fatalf("got %d directives, want %d: %q", len(ds), tc.want, directiveTexts(tc.src, ds))
			}
			if tc.want == 0 {
				return
			}
			d := ds[0]
			if d.Name != "note" || d.Level != markdown.DirectiveText {
				t.Errorf("got name %q level %v, want \"note\" and the text level", d.Name, d.Level)
			}
			if got := tc.src[d.Span.Start:d.Span.Stop]; got != ":note" {
				t.Errorf("span covers %q, want the bare %q", got, ":note")
			}
			if d.Attrs != nil {
				t.Errorf("Attrs = %v, want nil: the braces on the page are prose", d.Attrs)
			}
			if d.AttrsSpan != (markdown.Span{}) {
				t.Errorf("AttrsSpan = %v, want the zero span", d.AttrsSpan)
			}
			// The label bytes and the brace run are still in the document,
			// which is the half of the contract a replace-by-span consumer
			// has to plan for.
			if rest := tc.src[d.Span.Stop:]; rest != "[unbal[ y]{c=red} z\n" {
				t.Errorf("the bytes after the span are %q, want the label and braces left standing", rest)
			}
		})
	}
}

// TestDirectives_Memoize pins that a second call is the same slice, i.e.
// that the view is computed once per Source rather than per call.
func TestDirectives_Memoize(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("::a{#1}\n::b{#2}\n"))
	first, second := s.Directives(), s.Directives()
	if len(first) != 2 || len(second) != len(first) {
		t.Fatalf("Directives = %v then %v", first, second)
	}
	if &first[0] != &second[0] {
		t.Error("Directives recomputed on the second call")
	}
}

// TestDirectives_DoNotOverlapCodeSpans is the composition guarantee that
// matters: the two views come from ONE tree, so a directive written inside a
// code block is not a directive at all. Two independent scanners would
// disagree here, which is what this surface exists to prevent.
func TestDirectives_DoNotOverlapCodeSpans(t *testing.T) {
	t.Parallel()
	src := []byte("::real{#a}\n\n```\n::fake{#b}\n```\n\n    ::also-fake{#c}\n\n::real-too{#d}\n")
	s := markdown.NewSource(src)
	code := s.CodeSpans()
	ds := s.Directives()
	if len(ds) != 2 {
		t.Fatalf("got %d directives, want 2: %q", len(ds), directiveTexts(string(src), ds))
	}
	for _, d := range ds {
		if code.Overlaps(d.Span) {
			t.Errorf("directive %q overlaps a code span", src[d.Span.Start:d.Span.Stop])
		}
	}
}

// TestDirectives_RewriteAttributeValuesInOneParse is the shape the callers
// take, and the reason per-attribute spans exist at all: a rewriter that has
// only the parser's map has to re-find the value in the source by hand, and
// the quoting, the shorthands, and a duplicate key are all places that goes
// wrong. Nothing here indexes a byte or re-derives an offset.
func TestDirectives_RewriteAttributeValuesInOneParse(t *testing.T) {
	t.Parallel()
	src := []byte("::include{src=\"docs/old.md\"}\n\n```\n::include{src=\"skip.md\"}\n```\n\n" +
		"text :ref{src=old.md} tail\n")
	s := markdown.NewSource(src)
	var edits []markdown.Edit
	for _, d := range s.Directives() {
		for _, a := range d.AttrSpans {
			if a.Key != "src" {
				continue
			}
			edits = append(edits, markdown.Edit{
				Span: a.ValueSpan,
				Text: strings.ReplaceAll(a.Value, "old", "new"),
			})
		}
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "::include{src=\"docs/new.md\"}\n\n```\n::include{src=\"skip.md\"}\n```\n\n" +
		"text :ref{src=new.md} tail\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}

// TestDirectives_ComposeWithTheOtherViewsInOneParse is the whole point of
// the surface: four views over one buffer, measured from one parse, spliced
// together in one Apply. Independent scanners would each need their own
// parse and could not be handed to Apply at all.
func TestDirectives_ComposeWithTheOtherViewsInOneParse(t *testing.T) {
	t.Parallel()
	src := []byte("# Title\n\n::media{src=a.png}\n\n![alt](b.png)\n\n```\n::x{src=c.png}\n```\n")
	s := markdown.NewSource(src)

	heads := s.Headings()
	imgs := s.Images()
	ds := s.Directives()
	code := s.CodeSpans()
	if len(heads) != 1 || len(imgs) != 1 || len(ds) != 1 || len(code) != 1 {
		t.Fatalf("views = %d headings, %d images, %d directives, %d code blocks",
			len(heads), len(imgs), len(ds), len(code))
	}

	edits := []markdown.Edit{
		{Span: heads[0].Text, Text: "Renamed"},
		{Span: imgs[0].Dest, Text: "B.png"},
		{Span: ds[0].AttrSpans[0].ValueSpan, Text: "A.png"},
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "# Renamed\n\n::media{src=A.png}\n\n![alt](B.png)\n\n```\n::x{src=c.png}\n```\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
	for _, d := range ds {
		if code.Overlaps(d.Span) {
			t.Errorf("directive %q overlaps a code span", src[d.Span.Start:d.Span.Stop])
		}
	}
}
