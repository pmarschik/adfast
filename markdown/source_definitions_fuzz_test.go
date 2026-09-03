package markdown_test

import (
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// FuzzSourceDefinitions drives Definitions over arbitrary Markdown and checks
// the properties a rewriter of definition destinations depends on. The table
// in source_definitions_test.go can only ever cover the shapes somebody
// thought of, and every span this view reports is offset arithmetic over
// bytes nobody wrote on purpose.
//
// Four properties, in the order they catch things:
//
//   - Each definition is SHAPED like one: its span opens on a `[`, its label
//     is delimited by that `[` and by a `]` the colon follows, and an
//     angle-bracketed destination and a quoted title have their delimiters
//     just OUTSIDE the span the view reports. That is the convention the doc
//     comments promise, stated as an assertion over the bytes.
//   - Every part lies INSIDE its definition, and in the written order:
//     label, then destination, then title. A part outside is an offset from
//     a different definition.
//   - Definitions are blocks, so no two of them share a byte. Unlike a link
//     inside link text there is no nesting to allow.
//   - Apply accepts every destination edit TOGETHER, which is the operation
//     a rewriter performs and the one that fails on a collision or an
//     out-of-range offset.
//
// A whole-document ROUND TRIP is deliberately absent, for the reason
// FuzzSourceSpans documents: rewriting a destination can change the block
// parse around it, so the definition count after a rewrite is not an
// invariant.
func FuzzSourceDefinitions(f *testing.F) {
	for _, c := range definitionCases {
		f.Add(c.src)
	}
	for _, seed := range []string{
		"[a]: x.md\n[a]: y.md\n\n[a]\n",
		"> [a]: <a b>\n> \"t\"\n\n> [a]\n",
		"- [a\n  b]: (c)\n\n- [a b]\n",
		"[a]:\n<>\n\n[a]\n",
		"[a]: x.md '\\''\n\n[a]\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		src := []byte(in)
		s := markdown.NewSource(src)
		got := s.Definitions()
		// The parse may have recovered by normalizing the input, in which
		// case the spans address that copy and not in.
		src = s.Bytes()

		var edits []markdown.Edit
		prev := markdown.Span{}
		for i, d := range got {
			checkDefinitionBounds(t, in, i, src, d)
			checkDefinitionShape(t, in, i, src, d)
			checkDefinitionOrder(t, in, i, d)
			if i > 0 && d.Span.Start < prev.Stop {
				t.Fatalf("%q: definition %d at %v overlaps %v", in, i, d.Span, prev)
			}
			prev = d.Span
			edits = append(edits, markdown.Edit{Span: d.Dest, Text: "x"})
		}
		if _, err := s.Apply(edits...); err != nil {
			t.Fatalf("%q: the destination rewrite Apply rejected: %v", in, err)
		}
	})
}

// checkDefinitionBounds fails when any of a definition's spans is not a range
// of the source the view measured.
func checkDefinitionBounds(t *testing.T, in string, i int, src []byte, d markdown.Definition) {
	t.Helper()
	for _, part := range []struct {
		name string
		span markdown.Span
	}{{"Span", d.Span}, {"Label", d.Label}, {"Dest", d.Dest}, {"Title", d.Title}} {
		if part.span == (markdown.Span{}) {
			continue
		}
		if part.span.Start < 0 || part.span.Stop > len(src) || part.span.Start > part.span.Stop {
			t.Fatalf("%q: definition %d %s %v is not a range of a %d-byte source",
				in, i, part.name, part.span, len(src))
		}
	}
}

// checkDefinitionShape fails when the bytes around a definition's spans are
// not the delimiters the doc comments say sit outside them.
func checkDefinitionShape(t *testing.T, in string, i int, src []byte, d markdown.Definition) {
	t.Helper()
	if src[d.Span.Start] != '[' {
		t.Fatalf("%q: definition %d opens on %q, want %q", in, i, src[d.Span.Start], byte('['))
	}
	if src[d.Label.Start-1] != '[' || src[d.Label.Stop] != ']' {
		t.Fatalf("%q: definition %d label %v is not bracketed", in, i, d.Label)
	}
	if src[d.Label.Stop+1] != ':' {
		t.Fatalf("%q: definition %d label is not followed by a colon", in, i)
	}
	if d.Dest.Start > 0 && src[d.Dest.Start-1] == '<' && src[d.Dest.Stop] != '>' {
		t.Fatalf("%q: definition %d wrapped destination %v is not closed", in, i, d.Dest)
	}
	if d.Title != (markdown.Span{}) {
		checkTitleDelimiters(t, in, i, src, d.Title)
	}
}

// checkDefinitionOrder fails when a part is outside its definition or out of
// the written order: label, then destination, then title.
func checkDefinitionOrder(t *testing.T, in string, i int, d markdown.Definition) {
	t.Helper()
	if d.Label.Start < d.Span.Start || d.Label.Stop > d.Dest.Start ||
		d.Dest.Stop > d.Span.Stop {
		t.Fatalf("%q: definition %d parts out of order: %+v", in, i, d)
	}
	if d.Title != (markdown.Span{}) &&
		(d.Title.Start < d.Dest.Stop || d.Title.Stop > d.Span.Stop) {
		t.Fatalf("%q: definition %d title %v is not after the destination in %v",
			in, i, d.Title, d.Span)
	}
}

// checkTitleDelimiters fails when the byte before the title is not one of
// CommonMark's three openers, or the byte after is not its closer. The view
// reports the content and not the delimiters, so this is the assertion that
// the delimiters were excluded rather than merely trimmed.
func checkTitleDelimiters(t *testing.T, in string, i int, src []byte, title markdown.Span) {
	t.Helper()
	if title.Start == 0 || title.Stop >= len(src) {
		t.Fatalf("%q: definition %d title %v leaves no room for its delimiters", in, i, title)
	}
	opener, closer := src[title.Start-1], src[title.Stop]
	want := map[byte]byte{'"': '"', '\'': '\'', '(': ')'}[opener]
	if want == 0 || closer != want {
		t.Fatalf("%q: definition %d title %v is delimited by %q…%q, want a CommonMark pair",
			in, i, title, opener, closer)
	}
}
