package adfast

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// A significant space at an inline boundary is normally written as the
// "&#x20;" character reference, because Markdown strips a bare one there and
// the space would not survive a re-parse. A caller that renders two documents
// only to compare them, and throws both renderings away, has the opposite
// problem: a working copy cannot hold a trailing space at all (an editor takes
// it away), so the escaped side and the file side never line up and the
// comparison reports a change that nobody made. This option is that caller's
// answer — the space itself, on every site that would have escaped it.
func TestWithoutSignificantSpaceEscapesWritesTheSpaceItself(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    string
		wantOff string
		in      adf.Doc
	}{
		{
			name:    "trailing boundary space",
			in:      doc(p(txt("a", &adf.Strong{}), txt(" b "))),
			wantOff: "**a** b&#x20;\n",
			want:    "**a** b \n",
		},
		{
			name:    "leading boundary space",
			in:      doc(p(txt(" b"))),
			wantOff: "&#x20;b\n",
			want:    " b\n",
		},
		{
			name:    "non-breaking space alone in a paragraph",
			in:      doc(p(txt(" "))),
			wantOff: "&#xA0;\n",
			want:    " \n",
		},
		{
			// writeEncodedLead's hard-break rule: the space opens the line
			// after the backslash break, where a re-parse would eat it.
			name:    "space at a line start after a hard break",
			in:      doc(&adf.Paragraph{Content: []adf.Node{txt("a"), &adf.HardBreak{}, txt(" b")}}),
			wantOff: "a\\\n&#x20;b\n",
			want:    "a\\\n b\n",
		},
		{
			// encodedTrail's whitespace half: the space is the last rune
			// INSIDE the emphasis, not at the paragraph boundary. The
			// alphanumeric "&#x62;" beside the closing marker is a different
			// rule and stays either way — see the flanking pin below.
			name:    "space closing an emphasis run",
			in:      doc(p(txt("a ", &adf.Em{}), txt("b"))),
			wantOff: "_a&#x20;_&#x62;\n",
			want:    "_a _&#x62;\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := adfToMD(tt.in); got != tt.wantOff {
				t.Errorf("default render = %q, want %q", got, tt.wantOff)
			}
			if got := adfToMD(tt.in, WithoutSignificantSpaceEscapes()); got != tt.want {
				t.Errorf("render without space escapes = %q, want %q", got, tt.want)
			}
		})
	}
}

// The word-class references are not whitespace and are not negotiable: they
// are what keeps an emphasis marker flankable, so dropping one would change
// what the text says rather than how a space survives. Nothing about the
// option may touch them.
func TestWithoutSignificantSpaceEscapesKeepsTheFlankingEncodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want string
		in   adf.Doc
	}{
		{name: "emphasis closing before a word", in: doc(p(txt("a", &adf.Em{}), txt("b"))), want: "_&#x61;_&#x62;\n"},
		{name: "emphasis opening after a word", in: doc(p(txt("b"), txt("a", &adf.Em{}))), want: "&#x62;_&#x61;_\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := adfToMD(tt.in); got != tt.want {
				t.Errorf("default render = %q, want %q", got, tt.want)
			}
			if got := adfToMD(tt.in, WithoutSignificantSpaceEscapes()); got != tt.want {
				t.Errorf("render without space escapes = %q, want the same %q", got, tt.want)
			}
		})
	}
}

// The option suppresses an escape the RENDERER would have written. Six
// characters an author wrote — inside a code span, inside a fence, behind a
// backslash — are content, not an escape, and the round trip carries them
// through untouched in either mode. This is the shape a text substitution over
// the finished output destroys, which is why the suppression belongs here.
func TestWithoutSignificantSpaceEscapesLeavesTheAuthorsOwnReferenceAlone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, md string
	}{
		{"code span", "a `&#x20;` b\n"},
		{"fenced block", "```\n&#x20;\n```\n"},
		{"backslash escape", "a \\&#x20; b\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithoutSignificantSpaceEscapes()
			if got := adfToMD(mdToADF(tt.md, opt), opt); got != tt.md {
				t.Errorf("round trip = %q, want the document unchanged %q", got, tt.md)
			}
		})
	}
}
