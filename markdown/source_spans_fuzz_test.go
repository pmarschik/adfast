package markdown_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// FuzzSourceSpans drives Links and Images over arbitrary Markdown and checks
// the properties a consumer that rewrites destinations depends on. Every span
// this surface reports is offset arithmetic over bytes nobody wrote on
// purpose, so a table of shapes can only ever cover the shapes somebody
// thought of.
//
// Four properties, in the order they catch things:
//
//   - Each written form is SHAPED like one: the span starts at the right lead
//     byte, ends at a closer, the label closes with a `]` where the view says
//     the label stops, and the destination sits inside the span and after the
//     label.
//   - No two reported destinations SHARE a byte. Each destination is written
//     once, so two views claiming one run of bytes means one of them
//     misresolved — which is exactly what a nested form whose destination is
//     byte-equal to the outer one did before the floor rose past it.
//   - Apply accepts every destination edit TOGETHER, which is the operation a
//     rewriter performs and the one that fails on a collision.
//   - Two forms NEST or they are disjoint, and a nested one lies inside the
//     outer one's LABEL. A destination and a title are not inline content, so
//     no form can be reported inside another's tail; a form that appears to be
//     is one whose label closer was misread.
//
// Two properties that look inviting are deliberately absent, because neither
// follows from the contract:
//
//   - A whole-document ROUND TRIP. Rewriting a destination can change the
//     block parse around it: "```[](`)" is a paragraph, because a backtick
//     info string does not open a fence, and a backtick-free destination turns
//     the line into a code block whose content is no longer a link. It is one
//     of the seeds below.
//   - RE-PARSING a span on its own. Which bracket pair of a nested shape
//     goldmark calls the link depends on what precedes it — CommonMark caps a
//     link label at 999 characters, and an outer label over the cap is
//     abandoned, which activates the pair inside it. "[[]()]()" is the inner
//     pair on its own and the outer one after a long enough label, so the same
//     bytes are not the same form in isolation.
func FuzzSourceSpans(f *testing.F) {
	for _, c := range linkCases {
		f.Add(c.src)
	}
	for _, c := range imageCases {
		f.Add(c.src)
	}
	// The collision shapes, which is where the seeds and the tables meet.
	for _, seed := range []string{
		"[![](./img/shot.png)](./img/shot.png)\n",
		"![![](a.png)](a.png)\n",
		"[![![](a.png)](a.png)](a.png)\n",
		"[![](a[.png)](a[.png)\n",
		"[a`](x.md)`](x.md)\n",
		"> [![](a.png)](a.png)\n",
		"- [![](a.png\n  \"t\")](a.png)\n",
		"```[](`)",
		// A `]` that closes NOTHING, because the `[` before it was
		// deactivated: CommonMark turns off every unclosed link opener to the
		// left of a link it just matched, so the `]` at offset 7 is a literal
		// and the image's label runs on to the `]` at offset 9. Any reading of
		// a label as balanced brackets gets this one wrong.
		"![[[]()](]()[)]()",
		// A `[` inside an AUTOLINK, which carries its URL off the node rather
		// than in a child, so nothing in the tree says those bytes are not
		// structure.
		"[<A0:[>]()[<A0:]0>]()",
		// An EMPTY shortcut label, which a definition whose own label is
		// nothing but whitespace defines. It is the shortest written form
		// there is: two bytes.
		"[]\n\n[\v]:0",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		s := markdown.NewSource([]byte(in))
		src := string(s.Bytes())

		var dests []markdown.Span
		var forms []writtenForm
		for i, l := range s.Links() {
			what := fmt.Sprintf("link %d", i)
			checkWrittenForm(t, src, what, '[', l.Span, l.Text, l.Dest)
			forms = append(forms, writtenForm{what: what, span: l.Span, label: l.Text})
			if l.Dest != (markdown.Span{}) {
				dests = append(dests, l.Dest)
			}
		}
		for i, im := range s.Images() {
			what := fmt.Sprintf("image %d", i)
			checkWrittenForm(t, src, what, '!', im.Span, im.Alt, im.Dest)
			forms = append(forms, writtenForm{what: what, span: im.Span, label: im.Alt})
			if im.Dest != (markdown.Span{}) {
				dests = append(dests, im.Dest)
			}
		}
		if t.Failed() {
			return
		}
		checkNesting(t, src, forms)

		slices.SortFunc(dests, func(a, b markdown.Span) int { return a.Start - b.Start })
		edits := make([]markdown.Edit, 0, len(dests))
		for i, d := range dests {
			if i > 0 && dests[i-1].Stop > d.Start {
				t.Fatalf("%q: destinations %v and %v share a byte", src, dests[i-1], d)
			}
			edits = append(edits, markdown.Edit{Span: d, Text: "z" + strconv.Itoa(i)})
		}
		if _, err := s.Apply(edits...); err != nil {
			t.Fatalf("%q: Apply(%v): %v", src, edits, err)
		}
	})
}

// writtenForm is one reported link or image, reduced to the two spans the
// nesting rule is about.
type writtenForm struct {
	what  string
	span  markdown.Span
	label markdown.Span
}

// checkNesting asserts that no two reported forms partially overlap and that
// a form inside another lies inside that one's LABEL.
//
// A pair whose outer LABEL crosses a line is not checked, and the reason is a
// separate defect of its own: goldmark records a reference's label with each
// line's leading whitespace already stripped, so the bytes a crossing label
// is written with are not the bytes the parser matched. That is the drop
// Source.Links documents — and when a bracket pair somewhere AFTER the label
// happens to be byte-equal to what the parser recorded, the form resolves to
// a span reaching that far instead of being dropped
// ("[][i\n d][i\nd]\n\n[I d]:0"). It is not this target's business, and
// pinning it here would pin a bug.
func checkNesting(t *testing.T, src string, forms []writtenForm) {
	t.Helper()
	for i, a := range forms {
		for _, b := range forms[i+1:] {
			outer, inner := a, b
			if b.span.Start < a.span.Start || b.span.Stop > a.span.Stop {
				outer, inner = b, a
			}
			if strings.ContainsAny(src[outer.label.Start:outer.label.Stop], "\n\r") {
				continue
			}
			if !covers(outer.span, inner.span) {
				if outer.span.Overlaps(inner.span) {
					t.Errorf("%q: %s %v and %s %v overlap without nesting",
						src, a.what, a.span, b.what, b.span)
				}
				continue
			}
			if !covers(outer.label, inner.span) {
				t.Errorf("%q: %s %v is nested in %s %v but not in its label %v",
					src, inner.what, inner.span, outer.what, outer.span, outer.label)
			}
		}
	}
}

// covers reports whether outer contains every byte of inner.
func covers(outer, inner markdown.Span) bool {
	return inner.Start >= outer.Start && inner.Stop <= outer.Stop
}

// checkWrittenForm asserts that one reported form is shaped like the syntax it
// claims to cover. lead is the byte the span must begin at: `[` for a link,
// `!` for an image.
func checkWrittenForm(
	t *testing.T, src, what string, lead byte, span, label, dest markdown.Span,
) {
	t.Helper()
	// The shortest written form is a shortcut reference with an empty label:
	// `[]` for a link, `![]` for an image.
	shortest := 2
	if lead == '!' {
		shortest = 3
	}
	if span.Start < 0 || span.Stop > len(src) || span.Len() < shortest {
		t.Errorf("%q: %s span %v is not a written form of a %d byte source",
			src, what, span, len(src))
		return
	}
	if src[span.Start] != lead {
		t.Errorf("%q: %s starts at %q, want %q", src, what, src[span.Start], lead)
	}
	if last := src[span.Stop-1]; last != ')' && last != ']' {
		t.Errorf("%q: %s ends at %q, want ')' or ']'", src, what, last)
	}
	if label.Start < span.Start || label.Stop >= span.Stop {
		t.Errorf("%q: %s label %v is not inside its span %v", src, what, label, span)
		return
	}
	if src[label.Stop] != ']' {
		t.Errorf("%q: %s label %v stops at %q, want the closing ']'",
			src, what, label, src[label.Stop])
	}
	if dest == (markdown.Span{}) {
		return
	}
	if dest.Start <= label.Stop || dest.Stop > span.Stop {
		t.Errorf("%q: %s dest %v is not in the tail of its span %v after label %v",
			src, what, dest, span, label)
	}
}
