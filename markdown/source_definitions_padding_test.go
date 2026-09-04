package markdown

import (
	"testing"

	"github.com/yuin/goldmark/text"
)

// TestMatchesAsRead pins the PREDICATE itself, not the discount it applies.
//
// matchesAsRead is the verification step of the definition span view:
// definitionLabel and definitionTitle both refuse to locate a definition when
// it says no, so a predicate that always answered yes would let the view hand
// out a span the parser did not read that value from — and a caller
// rewriting through such a span writes the wrong bytes into the document,
// which is the one outcome this view exists to prevent.
//
// TestTrimLineLeadingSpaces below covers the helper the loose reading uses,
// and the padded tables in source_unlocated_test.go cover the loose reading
// being loose ENOUGH. Neither notices a predicate that stops discriminating:
// with the body replaced by `return true` both still pass. The rows here that
// want FALSE are what notices it.
//
// MEASURED, and the reason this is a unit table rather than a document: on
// this build no source makes the rejection observable through Definitions().
// The label and the title spans are built with goldmark's own closure grammar
// from the position goldmark recorded and bounded by goldmark's own line
// extent, so a scan that resolves a part at all resolves it at the offsets
// goldmark used; only the BYTES can disagree, through a container prefix or
// through padding, and every source where they do is dropped anyway by the
// `end != stop` extent check or by a recorded title with none written. A
// search comparing Definitions() against the always-yes predicate over four
// million structured samples, plus a coverage-guided run of the same
// comparison, found no source where the two answers differ. So the predicate
// is defense in depth today, and this table is what keeps it a predicate.
func TestMatchesAsRead(t *testing.T) {
	// segments builds a block's content segments from [start, stop) pairs —
	// one per line, each beginning past that line's container prefix, which
	// is the shape bytesAsRead reads them in.
	segments := func(spans ...[2]int) *text.Segments {
		segs := text.NewSegments()
		for _, sp := range spans {
			segs.Append(text.NewSegment(sp[0], sp[1]))
		}
		return segs
	}
	// The field order is the one govet's fieldalignment wants, not the
	// reading order — as on Definition itself.
	for _, tc := range []struct {
		lines *text.Segments
		name  string
		src   string
		want  string
		sp    Span
		ok    bool
	}{
		{
			name:  "the span is the value as read",
			src:   "[a]: x.md",
			lines: segments([2]int{0, 9}),
			sp:    Span{Start: 1, Stop: 2},
			want:  "a",
			ok:    true,
		},
		{
			name:  "padding stands in front of the recorded value",
			src:   "[a]: x.md",
			lines: segments([2]int{0, 9}),
			sp:    Span{Start: 1, Stop: 2},
			want:  "  a",
			ok:    true,
		},
		{
			name:  "the span crosses a prefix the segments strip",
			src:   "> a\n> b",
			lines: segments([2]int{2, 4}, [2]int{6, 7}),
			sp:    Span{Start: 2, Stop: 7},
			want:  "a\nb",
			ok:    true,
		},
		{
			name:  "the span crosses a prefix no segment strips",
			src:   "> a\n> b",
			lines: segments([2]int{2, 4}),
			sp:    Span{Start: 2, Stop: 7},
			want:  "a\nb",
			ok:    false,
		},
		{
			name:  "a different value entirely",
			src:   "[a]: x.md",
			lines: segments([2]int{0, 9}),
			sp:    Span{Start: 1, Stop: 2},
			want:  "b",
			ok:    false,
		},
		{
			name:  "the recorded value is a prefix of the span",
			src:   "[ab]: x.md",
			lines: segments([2]int{0, 10}),
			sp:    Span{Start: 1, Stop: 3},
			want:  "a",
			ok:    false,
		},
		{
			name:  "a tab at a line start is content, not padding",
			src:   "> a\n> b",
			lines: segments([2]int{2, 4}, [2]int{6, 7}),
			sp:    Span{Start: 2, Stop: 7},
			want:  "a\n\tb",
			ok:    false,
		},
		{
			name:  "a space before the newline is content too",
			src:   "> a\n> b",
			lines: segments([2]int{2, 4}, [2]int{6, 7}),
			sp:    Span{Start: 2, Stop: 7},
			want:  "a \nb",
			ok:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesAsRead([]byte(tc.src), tc.lines, tc.sp, []byte(tc.want))
			if got != tc.ok {
				t.Errorf("matchesAsRead(%q, %v, want %q) = %v, want %v",
					tc.src, tc.sp, tc.want, got, tc.ok)
			}
		})
	}
}

// TestDefinitionUnlocated_APrefixTheExtentLeavesOutside PINS PRESERVED
// BEHAVIOR on the one measured source where matchesAsRead does answer no.
// It is not a fix proof: the same source is dropped with the predicate
// stubbed to accept everything, because the extent check catches it too (see
// TestMatchesAsRead for what that measurement covered).
//
// The source is a definition whose LABEL is folded across a blockquote's
// second line while the title on a later line carries trailing content. The
// trailing content ends the definition at the destination, so the node keeps
// one line — the label's second half, and the `>` between the halves, are
// outside the extent it records, and no segment is left to strip that `>`
// out of the comparison. Whatever the reason, the answer a rewriter needs is
// the same one: this definition has no trustworthy offsets, so it is counted
// rather than reported.
func TestDefinitionUnlocated_APrefixTheExtentLeavesOutside(t *testing.T) {
	const src = ">[\n>a]: x.md\n\"t\" tail\n"
	s := NewSource([]byte(src))
	if got := s.Definitions(); len(got) != 0 {
		t.Errorf("Definitions() = %v, want none", got)
	}
	if got := s.UnlocatedDefinitions(); got != 1 {
		t.Errorf("UnlocatedDefinitions() = %d, want 1", got)
	}
}

// TestTrimLineLeadingSpaces pins exactly how much the definition resolver's
// comparison discounts (see matchesAsRead). The padded rows are what the
// discount is FOR; every other row is a byte it must keep, because each one
// is a byte an author wrote and a span has to be able to address it.
//
// This is the guard on the loose half of that comparison. The table over
// padded definitions in source_unlocated_test.go fails when the discount is
// too small; nothing there fails when it is too large, because the label and
// the title spans are anchored on delimiters the resolver matched separately
// and a wider discount cannot move them. What a wider discount does instead
// is stop the comparison being a check at all, and that is what this table
// notices.
func TestTrimLineLeadingSpaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "the padding a partly consumed tab leaves at the start",
			in:   "  t",
			want: "t",
		},
		{
			name: "the padding counted twice, as a folded label gets it",
			in:   "a\n    b",
			want: "a\nb",
		},
		{
			name: "nothing to discount",
			in:   "a\nb",
			want: "a\nb",
		},
		{
			name: "a space inside a line stays",
			in:   "a b\nc d",
			want: "a b\nc d",
		},
		{
			name: "a space before the newline stays",
			in:   "a \nb",
			want: "a \nb",
		},
		{
			name: "a tab at the start of a line stays, the padding before it goes",
			in:   "a\n  \tb",
			want: "a\n\tb",
		},
		{
			name: "a tab is not padding even alone",
			in:   "\ta",
			want: "\ta",
		},
		{
			name: "an empty value",
			in:   "",
			want: "",
		},
		{
			name: "a line that is only padding",
			in:   "a\n  \nb",
			want: "a\n\nb",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(trimLineLeadingSpaces([]byte(tc.in))); got != tc.want {
				t.Errorf("trimLineLeadingSpaces(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
