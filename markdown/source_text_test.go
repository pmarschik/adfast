package markdown_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// keyRe is the shape the consuming call sites scan with: an issue key with a
// left-boundary character captured separately, because RE2 has no lookbehind
// and "-" and "~" join the segments of a longer key rather than ending one.
// Group 1 is the boundary character and group 2 is the key.
var keyRe = regexp.MustCompile(`(^|[^A-Za-z0-9_~-])((?:[A-Z][A-Z0-9]+-NEW-\d+|[A-Za-z0-9][A-Za-z0-9._-]*~NEW-\d+|NEW-\d+)\b)`)

// found returns the key of every match TextMatches reports, as text.
func found(t *testing.T, src string) []string {
	t.Helper()
	s := markdown.NewSource([]byte(src))
	var out []string
	for _, sp := range s.TextMatches(keyRe, 2) {
		out = append(out, string(s.Text(sp)))
	}
	return out
}

type textMatchCase struct {
	name string
	src  string
	want []string
}

// proseCases pin the places a key IS a reference to an issue. Every one of
// them is a place a reader sees the key as words.
var proseCases = []textMatchCase{{
	name: "in a paragraph",
	src:  "blocks PROJ-NEW-2 and more\n",
	want: []string{"PROJ-NEW-2"},
}, {
	// PIN. A sentence long enough for goldmark to split into several text
	// nodes still reports one key. The splits GFM's autolink scan makes land
	// on spaces, so this holds even without run coalescing; it is here so a
	// future split rule that lands elsewhere is caught.
	name: "in a sentence the parser splits into several text nodes",
	src:  "one two three PROJ-NEW-2 four five\n",
	want: []string{"PROJ-NEW-2"},
}, {
	// A single tilde is not a strikethrough delimiter, but it does end a
	// text node. Without run coalescing the GitHub spelling would be lost.
	name: "the tilde spelling, whose tilde splits the text node",
	src:  "see storysmith-md~NEW-1 here\n",
	want: []string{"storysmith-md~NEW-1"},
}, {
	name: "in a heading",
	src:  "# Head PROJ-NEW-1\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "in a block quote",
	src:  "> quote PROJ-NEW-1\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "in a list item",
	src:  "- item PROJ-NEW-1\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "in a table cell",
	src:  "| a | PROJ-NEW-1 |\n| - | - |\n| c | d |\n",
	want: []string{"PROJ-NEW-1"},
}, {
	// The label is what a reader reads; the destination is not.
	name: "in a link label but not its destination",
	src:  "[label PROJ-NEW-1](http://x/PROJ-NEW-2)\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "in an image alt but not its destination",
	src:  "![alt PROJ-NEW-1](img/PROJ-NEW-2.png)\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "inside emphasis",
	src:  "*PROJ-NEW-1* and **PROJ-NEW-2**\n",
	want: []string{"PROJ-NEW-1", "PROJ-NEW-2"},
}, {
	name: "in a leaf directive label",
	src:  "::colwidths[PROJ-NEW-1]\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "in a container directive label",
	src:  ":::note[PROJ-NEW-1]\nbody\n:::\n",
	want: []string{"PROJ-NEW-1"},
}, {
	name: "twice in one document",
	src:  "PROJ-NEW-1 then PROJ-NEW-2\n",
	want: []string{"PROJ-NEW-1", "PROJ-NEW-2"},
}, {
	// Unmatched emphasis delimiters are literal text, so the bytes around
	// them are one run and the shorter key inside is a hit. It is the hit a
	// scan over the raw text already gave, and preserving it is the point:
	// this view subtracts what was never prose and adds nothing.
	name: "beside emphasis delimiters that formed no emphasis",
	src:  "PROJ*-*NEW-1\n",
	want: []string{"NEW-1"},
}, {
	name: "at the very start of the document",
	src:  "PROJ-NEW-1 opens the file\n",
	want: []string{"PROJ-NEW-1"},
}}

func TestTextMatchesFindsAKeyInProse(t *testing.T) {
	for _, tc := range proseCases {
		t.Run(tc.name, func(t *testing.T) {
			got := found(t, tc.src)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("TextMatches(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

// notProseCases are the places a hand-written scan reached into and should
// not have. Each one is a document whose only key is NOT a reference: it is
// an example, a machine-readable target, or text the author took out of the
// document on purpose.
var notProseCases = []textMatchCase{{
	name: "a fenced code block",
	src:  "text\n\n```\nPROJ-NEW-1\n```\n",
}, {
	// A closing fence carrying an info string does not close its block, so a
	// line scanner that toggles on every fence line reads the rest as prose.
	name: "a fenced code block with an info string",
	src:  "text\n\n```yaml\nkey: PROJ-NEW-1\n```\n",
}, {
	name: "a tilde-fenced code block containing a backtick fence",
	src:  "text\n\n~~~\n```\nPROJ-NEW-1\n```\n~~~\n",
}, {
	name: "an indented code block",
	src:  "text\n\n    PROJ-NEW-1\n",
}, {
	name: "an indented code block inside a list item",
	src:  "- item\n\n      PROJ-NEW-1\n",
}, {
	name: "an inline code span",
	src:  "a `PROJ-NEW-1` b\n",
}, {
	name: "a double-backtick code span",
	src:  "a ``PROJ-NEW-1`` b\n",
}, {
	name: "a code span that runs over a line break",
	src:  "a `PROJ-\nNEW-1` b\n",
}, {
	name: "a link destination",
	src:  "[label](http://x/PROJ-NEW-1)\n",
}, {
	name: "a link title",
	src:  "[label](http://x \"about PROJ-NEW-1\")\n",
}, {
	name: "an image destination",
	src:  "![alt](img/PROJ-NEW-1.png)\n",
}, {
	name: "an autolink",
	src:  "<https://x/PROJ-NEW-1>\n",
}, {
	name: "a link reference definition",
	src:  "[x]: http://y/PROJ-NEW-1\n\ntext [x] text\n",
}, {
	name: "an HTML comment",
	src:  "<!-- PROJ-NEW-1 -->\n",
}, {
	name: "an HTML block",
	src:  "<div>\nPROJ-NEW-1\n</div>\n",
}, {
	name: "inline HTML",
	src:  "foo <span data-key=\"PROJ-NEW-1\"></span> bar\n",
}, {
	name: "a directive attribute block",
	src:  ":::info{note=\"PROJ-NEW-1\"}\nbody\n:::\n",
}, {
	name: "a leaf directive attribute block",
	src:  "::colwidths[label]{id=PROJ-NEW-1}\n",
}, {
	// The label of a TEXT directive is parsed against its own buffer, so its
	// offsets do not address this document at all.
	name: "a text directive label",
	src:  "a :sup[PROJ-NEW-1] b\n",
}, {
	// GFM reads the two tildes as strikethrough, so the key's middle is
	// markup. The document already renders struck through.
	name: "the two-tilde spelling GFM reads as strikethrough",
	src:  "see org~repo~NEW-1 here\n",
}, {
	name: "a key broken by emphasis",
	src:  "PROJ-*NEW*-1 text\n",
}, {
	name: "a key broken by an escape",
	src:  "text PROJ\\-NEW-1 text\n",
}}

func TestTextMatchesLeavesWhatIsNotProse(t *testing.T) {
	for _, tc := range notProseCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := found(t, tc.src); len(got) != 0 {
				t.Fatalf("TextMatches(%q) = %v, want none", tc.src, got)
			}
		})
	}
}

// TestTextMatchesKeepsProseNextToWhatItSkips is the other half of the two
// case tables: a document holding both spellings must lose exactly one. A
// view that returned nothing at all would pass every case above.
func TestTextMatchesKeepsProseNextToWhatItSkips(t *testing.T) {
	for _, tc := range notProseCases {
		t.Run(tc.name, func(t *testing.T) {
			src := "mentions PROJ-NEW-9 first\n\n" + tc.src
			got := found(t, src)
			if len(got) != 1 || got[0] != "PROJ-NEW-9" {
				t.Fatalf("TextMatches(%q) = %v, want [PROJ-NEW-9]", src, got)
			}
		})
	}
}

// TestTextMatchesSurvivesTheRoundTrip is the property the whole view is for.
// Replacing every match with the bytes it already holds must give the source
// back unchanged: a caller that finds nothing to change writes nothing, and a
// document that goes through this surface untouched is untouched to the byte.
func TestTextMatchesSurvivesTheRoundTrip(t *testing.T) {
	for _, tc := range append(append([]textMatchCase{}, proseCases...), notProseCases...) {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			var edits []markdown.Edit
			for _, sp := range s.TextMatches(keyRe, 2) {
				edits = append(edits, markdown.Edit{Span: sp, Text: string(s.Text(sp))})
			}
			out, err := s.Apply(edits...)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(out) != tc.src {
				t.Fatalf("round trip changed the document:\n got %q\nwant %q", out, tc.src)
			}
		})
	}
}

// TestTextMatchesEditsOnlyTheKey pins the other half of the round trip: a
// real substitution changes the matched bytes and nothing else. The two
// spellings in each document differ only in where they sit, so a splice that
// used the whole match instead of the group — or that mistook one hit for
// another — shows up as a changed byte outside the key.
func TestTextMatchesEditsOnlyTheKey(t *testing.T) {
	for _, tc := range proseCases {
		t.Run(tc.name, func(t *testing.T) {
			s := markdown.NewSource([]byte(tc.src))
			spans := s.TextMatches(keyRe, 2)
			if len(spans) != len(tc.want) {
				t.Fatalf("TextMatches gave %d spans, want %d", len(spans), len(tc.want))
			}
			var edits []markdown.Edit
			want := tc.src
			for i, sp := range spans {
				edits = append(edits, markdown.Edit{Span: sp, Text: "REAL-1"})
				want = strings.Replace(want, tc.want[i], "REAL-1", 1)
			}
			out, err := s.Apply(edits...)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if string(out) != want {
				t.Fatalf("substitution:\n got %q\nwant %q", out, want)
			}
		})
	}
}

// TestTextMatchesReportsTheWholeMatchAtGroupZero pins that the group index
// selects, so a caller whose pattern needs a boundary character can consume
// the boundary and still edit only the key.
func TestTextMatchesReportsTheWholeMatchAtGroupZero(t *testing.T) {
	const src = "blocks PROJ-NEW-2 and more\n"
	s := markdown.NewSource([]byte(src))
	whole := s.TextMatches(keyRe, 0)
	if len(whole) != 1 || string(s.Text(whole[0])) != " PROJ-NEW-2" {
		t.Fatalf("group 0 = %q, want %q", texts(src, whole), " PROJ-NEW-2")
	}
	key := s.TextMatches(keyRe, 2)
	if len(key) != 1 || string(s.Text(key[0])) != "PROJ-NEW-2" {
		t.Fatalf("group 2 = %q, want %q", texts(src, key), "PROJ-NEW-2")
	}
}

// TestTextMatchesRefusesACallItCannotAnswer pins that a caller's own mistake
// yields nothing rather than a panic or a span pointing at the wrong bytes.
// A view that panicked here would take down a lint run over a whole tree.
func TestTextMatchesRefusesACallItCannotAnswer(t *testing.T) {
	const src = "blocks PROJ-NEW-2\n"
	s := markdown.NewSource([]byte(src))
	for _, tc := range []struct {
		re    *regexp.Regexp
		name  string
		group int
	}{
		{name: "no pattern", re: nil, group: 0},
		{name: "a negative group", re: keyRe, group: -1},
		{name: "a group the pattern does not have", re: keyRe, group: 7},
		{name: "a group that did not participate", re: regexp.MustCompile(`(PROJ)|(NOPE)`), group: 2},
		{name: "a pattern that matches nothing", re: regexp.MustCompile(`(ZZZ-NEW-\d+)`), group: 1},
		{name: "a zero-width pattern", re: regexp.MustCompile(`()`), group: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.TextMatches(tc.re, tc.group); len(got) != 0 {
				t.Fatalf("TextMatches = %v, want none", texts(src, got))
			}
		})
	}
}

// TestTextMatchesHoldsTheSpansContract pins what every consumer relies on:
// spans come back ascending and non-overlapping, so the binary searches and
// Apply's ordering are correct. A run-coalescing bug that merged across
// markup would show up as an overlap.
func TestTextMatchesHoldsTheSpansContract(t *testing.T) {
	const src = "PROJ-NEW-1 a `PROJ-NEW-2` b PROJ-NEW-3\n\n" +
		"[l PROJ-NEW-4](u/PROJ-NEW-5) PROJ-NEW-6\n\n```\nPROJ-NEW-7\n```\n\nPROJ-NEW-8\n"
	s := markdown.NewSource([]byte(src))
	spans := s.TextMatches(keyRe, 2)
	want := []string{"PROJ-NEW-1", "PROJ-NEW-3", "PROJ-NEW-4", "PROJ-NEW-6", "PROJ-NEW-8"}
	if got := texts(src, spans); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("TextMatches = %v, want %v", got, want)
	}
	for i, sp := range spans {
		if sp.Len() <= 0 || sp.Stop > len(s.Bytes()) {
			t.Fatalf("span %d = %+v is not a range of the source", i, sp)
		}
		if i > 0 && sp.Start < spans[i-1].Stop {
			t.Fatalf("span %d = %+v overlaps %+v", i, sp, spans[i-1])
		}
	}
}

// TestTextMatchesOneShotMatchesTheView pins that the package-level form is
// the same view, so a read-only caller and an editing caller cannot disagree
// about what a document mentions.
func TestTextMatchesOneShotMatchesTheView(t *testing.T) {
	for _, tc := range append(append([]textMatchCase{}, proseCases...), notProseCases...) {
		t.Run(tc.name, func(t *testing.T) {
			one := markdown.TextMatches([]byte(tc.src), keyRe, 2)
			view := markdown.NewSource([]byte(tc.src)).TextMatches(keyRe, 2)
			if len(one) != len(view) {
				t.Fatalf("one-shot gave %d spans, view gave %d", len(one), len(view))
			}
			for i := range one {
				if one[i] != view[i] {
					t.Fatalf("span %d: one-shot %+v, view %+v", i, one[i], view[i])
				}
			}
		})
	}
}

// TestTextMatchesMemoizesTheRuns pins that two calls with different patterns
// see the same document. The runs are memoized and the matches are not, so a
// memo that leaked the pattern would answer the second call with the first
// one's hits.
func TestTextMatchesMemoizesTheRuns(t *testing.T) {
	const src = "one PROJ-NEW-1 and two NEW-2\n"
	s := markdown.NewSource([]byte(src))
	if got := texts(src, s.TextMatches(regexp.MustCompile(`(^|[^A-Za-z0-9_~-])(PROJ-NEW-\d+)`), 2)); len(got) != 1 || got[0] != "PROJ-NEW-1" {
		t.Fatalf("first call = %v, want [PROJ-NEW-1]", got)
	}
	if got := texts(src, s.TextMatches(regexp.MustCompile(`(^|[^A-Za-z0-9_~-])(NEW-\d+)\b`), 2)); len(got) != 1 || got[0] != "NEW-2" {
		t.Fatalf("second call = %v, want [NEW-2]", got)
	}
}
