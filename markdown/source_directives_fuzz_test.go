package markdown_test

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// FuzzSourceDirectiveSpans drives Directives over arbitrary Markdown and
// checks the properties a consumer that rewrites a directive or one of its
// attribute values depends on.
//
// This view re-derives more offsets from bytes than any other one on Source:
// three scanners here MIRROR the directive parser's own unexported grammar —
// the name, the `[label]`, and the `{…}` block — and each has to agree with
// it byte for byte. A table can only cover the shapes somebody thought of,
// and the failure mode of a mirror that drifts is not an error but a span
// that quietly points at the wrong bytes, or no span at all where one was
// written.
//
// Five properties, in the order they catch things:
//
//   - The prefix is SHAPED like the level says: at most three spaces of
//     indent, then a colon run of one for a text directive, two for a leaf,
//     three or more for a container, then a name byte.
//   - AGREEMENT with the parser: a directive whose Attrs the parser recorded
//     has an AttrsSpan, and a non-empty Attrs has written occurrences. This
//     is the load-bearing one. Directives documents a fallback — on any
//     disagreement between the re-read and the parser's map the spans are
//     DROPPED — which is a safety net for grammar drift, not an outcome any
//     input should reach: a `[label]` measured to the wrong closer leaves the
//     parse right and the byte view empty, so a caller working from spans
//     cannot locate attributes that are plainly written. Nothing here reaches
//     the net.
//   - CONTAINMENT: AttrsSpan lies inside Span with its braces at its ends,
//     each attribute lies strictly inside the braces in ascending order, and
//     each key and value span reads back as the Key and Value reported.
//   - Directives NEST or they are disjoint, in document order, and never
//     partially overlap — the contract that makes the result a []Directive
//     rather than a Spans.
//   - Apply accepts every attribute value edit TOGETHER, which is the
//     operation a rewriter performs and the one that fails on a collision.
//
// And the property that is the actual contract, RE-PARSING the bytes Span
// covers: see checkDirectiveReparse for the one bound it is sound under.
func FuzzSourceDirectiveSpans(f *testing.F) {
	for _, c := range directiveCases {
		f.Add(c.src)
	}
	// Every attribute shape through three prefixes, because the `{` is
	// located by re-walking the prefix and each level walks a different one.
	// The labels are the shapes whose closer is not the first `]` on the
	// line, which is where a mirrored label scan drifts.
	for _, c := range attrCases {
		f.Add("::n[lbl]" + c.attrs + "\n")
		f.Add("x :n[a[b]c]" + c.attrs + " y\n")
		f.Add(":::n[a\\]b]" + c.attrs + "\nbody\n:::\n")
	}
	for _, seed := range []string{
		// A TEXT directive whose label does not parse: the parser does not
		// invalidate it, so the reported span is the bare `:note` and the
		// `{c=red}` after it is prose. See checkDirectiveShape for why the
		// agreement property is not violated by it.
		"x :note[unbal[ y]{c=red} z\n",
		// One pair closes inside the label and the outer `[` never does, so
		// a LEAF is not a directive at all: the marker line requires only
		// whitespace after the block.
		"::note[a[b]{c=red}\n",
		// A label one pair deeper than the parser's nesting cap, which
		// invalidates the label there and so must invalidate it here.
		"::n[" + strings.Repeat("[", 33) + strings.Repeat("]", 33) + "]{c=red}\n",
		// A backslash escapes a backslash, so the `]` after the pair is the
		// label's own closer and not an escaped bracket.
		"::n[a\\\\]{c=red}\n",
		// The same label shapes WITHOUT a block, so inserting one is a
		// single mutation away. Mutation does not reach the crossing on its
		// own: with a seed set of `::n{#a}`, `x :n[lbl]{#a} y` and a
		// container, 94 million executions against the label scan that
		// ended at the first `]` never produced a nested label carrying a
		// block. The seeds are what make that class reachable, so the
		// bridging shapes belong in them.
		"::n[a[b]c]\n",
		"x :n[a\\]b] y\n",
		// A tab as the list marker's padding: the parser reads a
		// tab-expanded LINE while the walk here reads raw bytes, so the two
		// disagree about where the name starts if the padding leaks.
		"-\t::x{#a}\n",
		"-\t:x{#a} y\n",
		// Container prefixes: the bytes of a continuation line are not the
		// bytes the parser matched, which is the bound the re-parse property
		// is scoped by.
		"> :::n{#a}\n> b\n> :::\n",
		"- :::n{#a}\n  b\n  :::\n",
		// Nested containers closed by one fence, with a text directive in
		// between: the nesting and document-order properties at once.
		"::::a{#1}\n:::b{#2}\nx :c{#3} y\n::::\n",
		// An unclosed container, which is what the buffer looks like while
		// the block is still being typed.
		":::n{#a}",
		// A directive in a table cell and in a link label, both of which are
		// inline-parsed out of a segment rather than a whole line.
		"| :n{#a} |\n| --- |\n",
		"[:n{#a}](y)\n",
		// The emoji-shortcode guard and the backslash-escape guard, whose
		// parity decides whether the colon is free.
		":a:b{#c}\n",
		"\\:n{#a}\n",
		"\\\\:n{#a}\n",
		// Every occurrence shape in one block: two classes that fold, a
		// duplicate key that overwrites, a bare key, and a quoted value.
		":n{k=\"v w\" .a .b #i bare dup=1 dup=2}\n",
		// Trailing whitespace is inside a marker line's extent.
		"::n[a]{#b}   \n",
		// A CRLF terminator, which lineSpan trims and the mirrored scans
		// stop at.
		"::n{#a}\r\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		s := markdown.NewSource([]byte(in))
		src := string(s.Bytes())
		ds := s.Directives()

		prevStart := -1
		var edits []markdown.Edit
		for i, d := range ds {
			what := fmt.Sprintf("directive %d", i)
			if !checkDirectiveShape(t, src, what, d) {
				continue
			}
			if d.Span.Start < prevStart {
				t.Errorf("%q: %s span %v is out of document order", src, what, d.Span)
			}
			prevStart = d.Span.Start
			for j, a := range d.AttrSpans {
				if a.ValueSpan == (markdown.Span{}) {
					continue
				}
				edits = append(edits, markdown.Edit{
					Span: a.ValueSpan,
					Text: fmt.Sprintf("z%d.%d", i, j),
				})
			}
		}
		if t.Failed() {
			return
		}
		checkDirectiveNesting(t, src, ds)
		checkDirectiveReparse(t, src, ds)
		if t.Failed() {
			return
		}
		if _, err := s.Apply(edits...); err != nil {
			t.Fatalf("%q: Apply(%v): %v", src, edits, err)
		}
	})
}

// checkDirectiveShape asserts that one reported directive is shaped like the
// level it claims, that its attribute spans agree with the parser's map, and
// that every span it reports is contained where the contract says. It reports
// whether the directive is intact enough for the properties that read across
// directives to mean anything.
func checkDirectiveShape(t *testing.T, src, what string, d markdown.Directive) bool {
	t.Helper()
	// The shortest written form is a text directive with a one-byte name.
	if d.Span.Start < 0 || d.Span.Stop > len(src) || d.Span.Len() < 2 {
		t.Errorf("%q: %s span %v is not a written form of a %d byte source",
			src, what, d.Span, len(src))
		return false
	}
	if !checkDirectivePrefix(t, src, what, d) {
		return false
	}
	return checkDirectiveAttrsBlock(t, src, what, d)
}

// checkDirectiveAttrsBlock asserts the agreement between the parser's
// attribute map and the byte view of it, and the containment of the block in
// the directive.
//
// AGREEMENT is the load-bearing property. Attrs is non-nil exactly when a
// `{…}` block was read, and an empty block yields a non-nil EMPTY map — so
// the parser having a map at all means the block is written in the source
// and the walk here must find it. The one input that looks like a
// counterexample is a TEXT directive whose label does not parse
// ("x :note[unbal[ y]{c=red} z"): the parser falls back to a bare `:note`
// and leaves the label bytes and the `{c=red}` after them as PROSE, so it
// records no Attrs either. The implication is vacuous there rather than
// loosened, which is the point — no reported map may go unlocated.
func checkDirectiveAttrsBlock(t *testing.T, src, what string, d markdown.Directive) bool {
	t.Helper()
	if d.Attrs == nil {
		if d.AttrsSpan != (markdown.Span{}) || d.AttrSpans != nil {
			t.Errorf("%q: %s has no attributes but reports %v and %d spans",
				src, what, d.AttrsSpan, len(d.AttrSpans))
		}
		return true
	}
	if d.AttrsSpan == (markdown.Span{}) {
		t.Errorf("%q: %s recorded attributes %v but no attribute block span",
			src, what, d.Attrs)
		return false
	}
	if len(d.Attrs) > 0 && len(d.AttrSpans) == 0 {
		t.Errorf("%q: %s recorded attributes %v but no written occurrences",
			src, what, d.Attrs)
	}

	// CONTAINMENT of the block in the directive, braces included.
	if d.AttrsSpan.Start < d.Span.Start || d.AttrsSpan.Stop > d.Span.Stop ||
		d.AttrsSpan.Len() < 2 {
		t.Errorf("%q: %s attribute block %v is not inside its span %v",
			src, what, d.AttrsSpan, d.Span)
		return false
	}
	if src[d.AttrsSpan.Start] != '{' || src[d.AttrsSpan.Stop-1] != '}' {
		t.Errorf("%q: %s attribute block %v is %q, want it braced",
			src, what, d.AttrsSpan, src[d.AttrsSpan.Start:d.AttrsSpan.Stop])
	}
	checkAttrSpans(t, src, what, d)
	return true
}

// checkDirectivePrefix asserts that the bytes the span opens with are the
// prefix the reported level is written with: at most three spaces of indent,
// then a colon run whose length the level fixes, then a name byte. The colon
// count is the one thing about a directive nothing else in the view repeats,
// so a span that started one byte off is otherwise invisible.
func checkDirectivePrefix(t *testing.T, src, what string, d markdown.Directive) bool {
	t.Helper()
	i := d.Span.Start
	for i < d.Span.Stop && i-d.Span.Start < 3 && src[i] == ' ' {
		i++
	}
	colons := 0
	for i < d.Span.Stop && src[i] == ':' {
		colons++
		i++
	}
	if want := wrongColonRun(d.Level, colons); want != "" {
		t.Errorf("%q: %s is level %d with %d colons, want %s",
			src, what, d.Level, colons, want)
		return false
	}
	// The name is the parser's, so it must be the bytes right after the run.
	if i+len(d.Name) > d.Span.Stop || src[i:i+len(d.Name)] != d.Name {
		t.Errorf("%q: %s is named %q but the bytes after its colons are %q",
			src, what, d.Name, src[i:min(i+len(d.Name), d.Span.Stop)])
		return false
	}
	return true
}

// wrongColonRun names the colon run the level is written with, or "" when
// the run of that length is the right one for it.
func wrongColonRun(level markdown.DirectiveLevel, colons int) string {
	switch level {
	case markdown.DirectiveText:
		if colons != 1 {
			return "exactly one colon"
		}
	case markdown.DirectiveLeaf:
		if colons != 2 {
			return "exactly two colons"
		}
	case markdown.DirectiveContainer:
		if colons < 3 {
			return "three or more colons"
		}
	default:
		return "a known level"
	}
	return ""
}

// checkAttrSpans asserts that every written attribute lies strictly inside
// the braces, in ascending non-overlapping order, and that the key and value
// spans read back as the Key and Value reported.
//
// Reading the spans back is what a rewriter does, and neither scan unescapes
// anything, so the bytes and the strings must be equal. A bare key has no
// value written at all and reports the zero span; an explicitly empty value
// reports an empty span at the offset a new value goes.
func checkAttrSpans(t *testing.T, src, what string, d markdown.Directive) {
	t.Helper()
	prevStop := d.AttrsSpan.Start
	for j, a := range d.AttrSpans {
		which := fmt.Sprintf("%s attribute %d", what, j)
		if a.Span.Start <= d.AttrsSpan.Start || a.Span.Stop >= d.AttrsSpan.Stop ||
			a.Span.Len() < 1 {
			t.Errorf("%q: %s %v is not strictly inside the braces %v",
				src, which, a.Span, d.AttrsSpan)
			continue
		}
		if a.Span.Start < prevStop {
			t.Errorf("%q: %s %v is out of order after [,%d)", src, which, a.Span, prevStop)
		}
		prevStop = a.Span.Stop
		if a.KeySpan == (markdown.Span{}) {
			// A `#id` or `.class` shorthand spells its key nowhere.
			if a.Key != "id" && a.Key != "class" {
				t.Errorf("%q: %s is keyed %q with no key span", src, which, a.Key)
			}
		} else if !checkInnerSpan(t, src, which+" key", a.Span, a.KeySpan, a.Key) {
			continue
		}
		if a.ValueSpan == (markdown.Span{}) {
			if a.Value != "" {
				t.Errorf("%q: %s has value %q with no value span", src, which, a.Value)
			}
			continue
		}
		checkInnerSpan(t, src, which+" value", a.Span, a.ValueSpan, a.Value)
	}
}

// checkInnerSpan asserts that inner lies within outer and reads back as want.
func checkInnerSpan(t *testing.T, src, which string, outer, inner markdown.Span, want string) bool {
	t.Helper()
	if inner.Start < outer.Start || inner.Stop > outer.Stop || inner.Start > inner.Stop {
		t.Errorf("%q: %s %v is not inside %v", src, which, inner, outer)
		return false
	}
	if got := src[inner.Start:inner.Stop]; got != want {
		t.Errorf("%q: %s %v reads %q, want %q", src, which, inner, got, want)
		return false
	}
	return true
}

// checkDirectiveNesting asserts that two directives nest or are disjoint,
// which is the contract that makes Directives a slice rather than a Spans:
// a container's span covers the ones written inside it, and nothing else may
// share a byte. A PARTIAL overlap is a span whose end was misread — there is
// no written shape it could come from.
func checkDirectiveNesting(t *testing.T, src string, ds []markdown.Directive) {
	t.Helper()
	for i, a := range ds {
		for _, b := range ds[i+1:] {
			outer, inner := a, b
			if b.Span.Start < a.Span.Start || b.Span.Stop > a.Span.Stop {
				outer, inner = b, a
			}
			if outer.Span.Start <= inner.Span.Start && inner.Span.Stop <= outer.Span.Stop {
				continue
			}
			if a.Span.Overlaps(b.Span) {
				t.Errorf("%q: %s %v and %s %v overlap without nesting",
					src, a.Name, a.Span, b.Name, b.Span)
			}
		}
	}
}

// checkDirectiveReparse asserts the property the three mirrored scanners
// actually owe the parser: the bytes Span covers, parsed on their own, are
// the same directive. Agreement with the parser is the whole contract of a
// mirror, and it is what a label measured to the wrong closer breaks.
//
// It is sound only for a SINGLE-LINE span, and the bound is not a
// convenience. A container's extent runs from its opening fence to its
// closing one, and every line in between reaches the parser with the
// enclosing container's prefix already stripped — the `> ` of a blockquote,
// the indent of a list item. Those bytes are inside the span and are NOT the
// bytes the parser matched, so a re-parse of the raw range is a parse of a
// different document. A single-line span has no such line: a leaf's and a
// container fence's extent begins after the prefix at its own line's start,
// and a text directive's is inside one line by construction, since both the
// label and the attribute scan stop at a line ending.
//
// Only the FIRST directive of the re-parse is compared. A leaf or container
// label is inline-parsed in the outer document, so a directive written in
// one is reported too and comes back in the re-parse as well.
func checkDirectiveReparse(t *testing.T, src string, ds []markdown.Directive) {
	t.Helper()
	for i, d := range ds {
		sub := src[d.Span.Start:d.Span.Stop]
		if strings.ContainsAny(sub, "\n\r") {
			continue
		}
		got := markdown.Directives([]byte(sub))
		if len(got) == 0 {
			t.Errorf("%q: directive %d %q is no directive on its own", src, i, sub)
			continue
		}
		g := got[0]
		if g.Span != (markdown.Span{Start: 0, Stop: len(sub)}) {
			t.Errorf("%q: directive %d %q re-parses to span %v, want the whole range",
				src, i, sub, g.Span)
		}
		if g.Level != d.Level || g.Name != d.Name {
			t.Errorf("%q: directive %d %q re-parses to level %d name %q, want %d and %q",
				src, i, sub, g.Level, g.Name, d.Level, d.Name)
		}
		if !maps.Equal(g.Attrs, d.Attrs) {
			t.Errorf("%q: directive %d %q re-parses to attributes %v, want %v",
				src, i, sub, g.Attrs, d.Attrs)
		}
	}
}
