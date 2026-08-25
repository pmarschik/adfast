package adfast_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
)

// A bare text directive ends in its name, which is word class for every
// registered kind. An emphasis construct opening right after it cannot
// flank, and remark's repair — hex-encoding one of the runes touching the
// marker — cannot reach a directive name without renaming the directive.
// The renderer therefore emits the inert empty attribute block so the rune
// before the marker is '}'. See markdown.needsPunctTrail.
func TestBareDirectiveBeforeEmphasisGetsPunctuationTail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			name: "punctuation lead, emphasis",
			md:   "*:media!*",
			want: ":media{}_!_\n",
		},
		{
			name: "punctuation lead, strong",
			md:   "**:media!**",
			want: ":media{}**!**\n",
		},
		{
			name: "punctuation lead, strikethrough",
			md:   "~~:media!~~",
			want: ":media{}~~!~~\n",
		},
		{
			// The space lead still needs the character reference; the two
			// repairs compose rather than replace each other.
			name: "whitespace lead keeps the hex reference",
			md:   "*:media x!*",
			want: ":media{}_&#x20;x!_\n",
		},
		{
			// A label already ends the form in punctuation, so the empty
			// attribute block must not be added on top.
			name: "labeled directive is left alone",
			md:   "*:media[l]!*",
			want: ":media[l]_!_\n",
		},
		{
			// A brace still reaches the labeled form: goldmark-directive
			// rejects the line ending inside {…}, so the attribute block is
			// literal text here — and without the terminator it would be read
			// back as the directive's attributes.
			name: "brace after a labeled directive",
			md:   ":media[0]{0=\"\n\"}",
			want: ":media[0]{}{0=\" \"}\n",
		},
		{
			name: "brace after a bare directive",
			md:   ":media{0=\"\n\"}",
			want: ":media{}{0=\" \"}\n",
		},
		{
			// An attribute block already terminates the form; a following
			// brace cannot open a second one.
			name: "directive with attributes is left alone",
			md:   ":status[done]{color=\"green\"}{0=\"\n\"}",
			want: ":status[done]{color=\"green\"}{0=\" \"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := roundTripMarkdown(tt.md)
			if got != tt.want {
				t.Fatalf("first render = %q, want %q", got, tt.want)
			}
			if second := roundTripMarkdown(got); second != got {
				t.Fatalf("not idempotent:\n first:  %q\n second: %q", got, second)
			}
		})
	}
}

// A literal backslash written directly before a text directive is a
// re-parse hazard the RENDERER used to repair: goldmark-directive decided
// whether ':' may open a directive from the single preceding source byte,
// so an escaped literal backslash (`\\:u`) suppressed it exactly like the
// escape marker (`\:u`) does, and the directive came back as text.
//
// goldmark-directive v0.3.1 counts the backslash run and suppresses only on
// odd parity (micromark's rule), so the escaped pair the renderer already
// writes is enough and the character-reference repair is gone. This test
// stays as the pin on that dependency: it asserts the escaped form, its
// idempotence, and that the directive survives the re-parse.
func TestBackslashBeforeDirectiveKeepsIt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro, minimized: an unregistered ":00" degrades to
			// text ending in a backslash, right before a real ":u".
			name: "degraded directive leaves a trailing backslash",
			md:   ":00[0\\]:u[0]",
			want: ":000\\\\:u[0]\n",
		},
		{
			name: "label content ending in a backslash",
			md:   ":zz[\\\\]:u[0]",
			want: "\\:zz\\\\:u[0]\n",
		},
		{
			// Nothing to repair when the directive does not touch the
			// backslash.
			name: "separated by a space",
			md:   "x\\\\ :u[0]",
			want: "x\\\\ :u[0]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := roundTripMarkdown(tt.md)
			if got != tt.want {
				t.Fatalf("first render = %q, want %q", got, tt.want)
			}
			if second := roundTripMarkdown(got); second != got {
				t.Fatalf("not idempotent:\n first:  %q\n second: %q", got, second)
			}
			// The point of the repair: the directive survives the re-parse
			// instead of degrading back to text.
			if !strings.Contains(adfJSON(t, got), `"underline"`) {
				t.Errorf("directive lost on re-parse: %s", adfJSON(t, got))
			}
		})
	}
}

// A backslash in a leaf or container directive's [label] is written
// verbatim by remark-stringify, which is lossy: it is consumed as an
// escape marker on re-parse. The renderer escapes the ones that could
// start an escape sequence. See markdown.escapeDirectiveLabel.
func TestDirectiveLabelEscapesBackslash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: "\!" in the alt text.
			name: "backslash before punctuation",
			md:   "::media[\\\\!0]",
			want: "::media[\\\\!0]\n",
		},
		{
			// The "]" written after the label would be the escaped byte,
			// leaving the label unterminated.
			name: "trailing backslash",
			md:   "::media[a\\\\]",
			want: "::media[a\\\\]\n",
		},
		{
			// A backslash before a letter is not an escape sequence, so it
			// stays verbatim — remark parity.
			name: "backslash before a letter is left alone",
			md:   "::media[a\\\\b]",
			want: "::media[a\\b]\n",
		},
		{
			name: "container label",
			md:   ":::expand[a\\\\!b]\nx\n:::",
			want: ":::expand[a\\\\!b]\nx\n:::\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := roundTripMarkdown(tt.md)
			if got != tt.want {
				t.Fatalf("first render = %q, want %q", got, tt.want)
			}
			if second := roundTripMarkdown(got); second != got {
				t.Fatalf("not idempotent:\n first:  %q\n second: %q", got, second)
			}
			if before, after := adfJSON(t, tt.md), adfJSON(t, got); before != after {
				t.Errorf("ADF changed across the render:\n before: %s\n after:  %s", before, after)
			}
		})
	}
}

// A ':' inside a directive [label] can open a NESTED text directive on
// re-parse, and the label is read back with ast.PlainText, which has no
// text for a directive node — so the label content vanishes. The
// renderer escapes those colons, including the digit-led names the prose
// escaper leaves alone for remark parity. See
// markdown.escapesColon and markdown.escapeDirectiveLabel.
func TestDirectiveLabelEscapesNestedDirectiveColon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: the label's leading ":0" is a directive on
			// re-parse (the trailing ":0" already degraded on the way in),
			// and the placeholder was left with no text at all.
			name: "digit-led name at the label start",
			md:   ":placeholder[:0:0]",
			want: `:placeholder[\:0]` + "\n",
		},
		{
			name: "digit-led name inside the label",
			md:   `:placeholder[x\:0]`,
			want: `:placeholder[x\:0]` + "\n",
		},
		{
			name: "letter-led name inside the label",
			md:   `:placeholder[a\:b]`,
			want: `:placeholder[a\:b]` + "\n",
		},
		{
			// A ':' after another ':' cannot open a text directive, so it
			// stays verbatim — the prose rule, unchanged.
			name: "second colon of a run is left alone",
			md:   `:placeholder[a\:\:b]`,
			want: ":placeholder[a::b]\n",
		},
		{
			// A ':' that cannot start a name is not a hazard.
			name: "trailing colon is left alone",
			md:   ":placeholder[a:]",
			want: ":placeholder[a:]\n",
		},
		{
			name: "leaf directive label",
			md:   `::media[\:0]`,
			want: `::media[\:0]` + "\n",
		},
		{
			name: "container directive label",
			md:   ":::expand[\\:0]\nx\n:::",
			want: ":::expand[\\:0]\nx\n:::\n",
		},
		{
			name: "inside emphasis in a label",
			md:   `:placeholder[*x\:0*]`,
			want: `:placeholder[x\:0]` + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := roundTripMarkdown(tt.md)
			if got != tt.want {
				t.Fatalf("first render = %q, want %q", got, tt.want)
			}
			if second := roundTripMarkdown(got); second != got {
				t.Fatalf("not idempotent:\n first:  %q\n second: %q", got, second)
			}
			if before, after := adfJSON(t, tt.md), adfJSON(t, got); before != after {
				t.Errorf("ADF changed across the render:\n before: %s\n after:  %s", before, after)
			}
		})
	}
}

// The prose wrapper masks the spaces inside a directive span so a wrap
// never lands in a label or attribute block — a newline there ends the
// directive on re-parse. It masked only spaces, but wrapText splits words
// on tabs too. See markdown.wrapTextProtected.
func TestTabInDirectiveLabelIsNotAWrapPoint(t *testing.T) {
	t.Parallel()
	// Long enough that the line exceeds the 80-column wrap budget, so the
	// tab is the only candidate break point.
	md := ":placeholder[\t" + strings.Repeat("0", 66) + "]"
	got := roundTripMarkdown(md)
	if strings.Contains(got, "[\n") || strings.Count(got, "\n") != 1 {
		t.Fatalf("wrapped inside the label: %q", got)
	}
	if second := roundTripMarkdown(got); second != got {
		t.Fatalf("not idempotent:\n first:  %q\n second: %q", got, second)
	}
}

// The empty attribute block has to be semantically inert, or the repair
// would change the document it is repairing.
func TestEmptyAttributeBlockIsInert(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"annotation", "bg", "color", "date", "emoji", "extension", "fontSize",
		"media", "mention", "placeholder", "status", "sub", "sup", "u",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bare := adfJSON(t, ":"+name)
			braced := adfJSON(t, ":"+name+"{}")
			if bare != braced {
				t.Fatalf(":%s{} decodes differently:\n bare:   %s\n braced: %s", name, bare, braced)
			}
		})
	}
}

func roundTripMarkdown(md string) string {
	return adfast.ToMarkdown(adfast.FromADF(adfast.ToADF(adfast.FromMarkdown(md))))
}

func adfJSON(t *testing.T, md string) string {
	t.Helper()
	b, err := json.Marshal(adfast.ToADF(adfast.FromMarkdown(md)))
	if err != nil {
		t.Fatalf("marshal %q: %v", md, err)
	}
	return string(b)
}
