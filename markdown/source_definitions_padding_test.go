package markdown

import "testing"

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
