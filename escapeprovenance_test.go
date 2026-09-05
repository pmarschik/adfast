package adfast

import (
	"strings"
	"testing"
)

// This file pins the escape provenance the format leg carries: which
// authored backslashes come back, and — the half that is easy to lose while
// fixing the first — which authored backslashes stay LITERAL backslashes.
//
// The two are one mechanism, not two. The parse records the source's
// backslash run verbatim (markdown.PreservedEscapes, which holds '\' itself
// for exactly this reason) and the render reads the run's PARITY: an odd run
// ends in a backslash bound to the next byte (an authored escape), an even
// one leaves that byte bare (an authored literal backslash). Fix either
// direction without the parity and the other breaks — a widened preserved
// set with no provenance re-emits every backslash-before-punctuation as an
// escape and deletes the author's literal backslash instead.
//
// escapableASCIIPunct is CommonMark's escapable set: the 32 ASCII
// punctuation bytes a backslash can escape.
const escapableASCIIPunct = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

// escapesTheFormatLegKeeps are the bytes whose AUTHORED escape the format
// leg writes back byte for byte. It is 30 of the 32 above.
//
// '_' is dropped, and so is prettier 3.8.1's: the reference recomputes the
// underscore escape from its own word-boundary rule, so an intraword
// "a_b\_c d" comes back "a_b_c d" from the reference too. Dropping it here
// is parity, not loss.
//
// '#' is dropped for a reason that is ours alone, and it is the one byte
// where this leg is behind the reference's 31 of 32: an ATX heading's
// trailing '#' is rewritten into "\#" after the escaper has run, and that
// rewrite reads no parity, so an escape already standing there comes back
// doubled. See markdown.PreservedEscapes.
//
// Neither drop costs a character: the escape is defensive in both positions
// and the decoded value is identical, which is what the ADF assertions below
// check on every row.
const escapesTheFormatLegKeeps = "!\"$%&'()*+,-./:;<=>?@[\\]^`{|}~"

// TestFormatKeepsAnAuthoredPunctuationEscape walks all 32 escapable
// punctuation bytes twice: once written as an authored ESCAPE ("a b\Xc d")
// and once as an authored LITERAL BACKSLASH before the same byte
// ("a b\\Xc d").
//
// The two columns hold each other honest. Widening the preserved-escape set
// without recording provenance passes the escape column and DELETES a
// character from the literal column, which is the trade every earlier
// attempt at this bug made; keeping the old four-byte set passes the literal
// column and drops 25 of the 32 escapes.
func TestFormatKeepsAnAuthoredPunctuationEscape(t *testing.T) {
	t.Parallel()
	for i := range len(escapableASCIIPunct) {
		c := escapableASCIIPunct[i : i+1]
		t.Run("authored escape of "+c, func(t *testing.T) {
			t.Parallel()
			src := "a b\\" + c + "c d\n"
			// A kept escape comes back as written; a dropped one comes
			// back as the character it stood for, never as anything else.
			want := "a b" + c + "c d\n"
			if strings.Contains(escapesTheFormatLegKeeps, c) {
				want = src
			}
			assertFormatKeepsMeaning(t, src, want)
		})
		t.Run("authored literal backslash before "+c, func(t *testing.T) {
			t.Parallel()
			// The GOOD case: the value here is a backslash followed by the
			// punctuation byte, and the format leg must not read that
			// backslash as an escape and drop it.
			src := "a b\\\\" + c + "c d\n"
			assertFormatKeepsMeaning(t, src, "")
		})
	}
}

// TestFormatKeepsALiteralBackslashTheAuthorWrote is the value-loss half of
// this bug, on the four bytes the old preserved set held ("~:-+"). Those
// four were kept by BYTE with no provenance, so a backslash standing in
// front of one was re-emitted bare and the next parse read it as the escape
// it now looked like: the author's backslash was gone from the prose.
//
// The assertions are on the VALUE (PlainTextOf), not on the bytes, because
// that is what was lost: the old leg wrote "a b\~c d" for an input whose
// text is backslash-tilde, and that output's text is a bare tilde.
//
// The last row is the good case in the same table: an authored ESCAPE of the
// same byte, which must still come back as an escape. A fix that re-escapes
// every backslash indiscriminately passes the first four rows and breaks
// this one.
func TestFormatKeepsALiteralBackslashTheAuthorWrote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string // the document's text, before and after formatting
	}{
		{name: "before a tilde", md: "a b\\\\~c d\n", want: "a b\\~c d"},
		{name: "before a colon", md: "a b\\\\:c d\n", want: "a b\\:c d"},
		{name: "before a dash", md: "a b\\\\-c d\n", want: "a b\\-c d"},
		{name: "before a plus", md: "a b\\\\+c d\n", want: "a b\\+c d"},
		{
			name: "and an authored escape of the same byte still escapes",
			md:   "a b\\~c d\n",
			want: "a b~c d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PlainTextOf(tt.md); got != tt.want {
				t.Fatalf("source text = %q, want %q", got, tt.want)
			}
			out := fmtMD(tt.md)
			if got := PlainTextOf(out); got != tt.want {
				t.Errorf("formatted %q -> %q, whose text is %q, want %q", tt.md, out, got, tt.want)
			}
			assertFormatKeepsMeaning(t, tt.md, tt.md)
		})
	}
}

// TestFormatReadsTheBackslashRunParity is the mechanism itself: a run of N
// backslashes before a tilde, for N of 1 through 5. CommonMark pairs a run
// left to right, so an odd run ends in an escape of the tilde and an even
// one leaves the tilde bare — which is how an authored escape and an
// authored literal backslash tell apart at all.
//
// Rows 2 and 3 are the pair that no byte-set rule can separate: their
// documents have the SAME text (one backslash, then a tilde) and different
// spellings, and both spellings must come back as the author wrote them.
func TestFormatReadsTheBackslashRunParity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		text string
	}{
		{name: "one backslash escapes the tilde", md: "a b\\~c d\n", text: "a b~c d"},
		{name: "two leave it bare", md: "a b\\\\~c d\n", text: "a b\\~c d"},
		{name: "three escape it again", md: "a b\\\\\\~c d\n", text: "a b\\~c d"},
		{name: "four leave it bare", md: "a b\\\\\\\\~c d\n", text: "a b\\\\~c d"},
		{name: "five escape it again", md: "a b\\\\\\\\\\~c d\n", text: "a b\\\\~c d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PlainTextOf(tt.md); got != tt.text {
				t.Fatalf("source text = %q, want %q", got, tt.text)
			}
			assertFormatKeepsMeaning(t, tt.md, tt.md)
		})
	}
}

// TestFormatCountsAMarkerEscapeAgainstTheWrapColumn is the boundary case the
// widened preserved set opened, and it is a joint failure of two rules rather
// than of either one.
//
// A line-leading ordered marker is escaped twice over: once per character by
// escapeOrderedMarker, and once per rendered LINE by
// escapeLeadingOrderedMarker, which runs AFTER the wrapper. While '.' and ')'
// were decoded away by the next parse, only the second rule ever fired in
// prettier mode and nobody noticed that its backslash is a byte the wrapper
// never measured. Now that the parse hands those escapes back verbatim, the
// byte is present on pass two and absent on pass one, so a paragraph sitting
// exactly on the wrap column formats to two different documents depending on
// how many times you format it.
//
// The rows below are that paragraph: 80 columns, the last of which is the
// marker's separator. The GOOD rows in the same table are short paragraphs
// with nothing to wrap, which must keep the escape they always had — a "fix"
// that simply stopped escaping the marker would pass the wrap rows and turn
// those two paragraphs into lists. Their sources carry the escape because
// without it they are not paragraphs at all.
func TestFormatCountsAMarkerEscapeAgainstTheWrapColumn(t *testing.T) {
	t.Parallel()
	// 77 digits, a marker byte, a space and one more digit: 80 columns
	// before the escape, 81 after it.
	digits := strings.Repeat("0", 77)
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			name: "a paren marker that lands on the wrap column",
			md:   digits + ") 0\n",
			want: digits + "\\)\n0\n",
		},
		{
			name: "a dot marker that lands on the wrap column",
			md:   digits + ". 0\n",
			want: digits + "\\.\n0\n",
		},
		{name: "and a short paren marker keeps its escape", md: "1\\) x\n", want: "1\\) x\n"},
		{name: "and a short dot marker keeps its escape", md: "1\\. x\n", want: "1\\. x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertFormatKeepsMeaning(t, tt.md, tt.want)
		})
	}
}

// assertFormatKeepsMeaning runs the three checks every row above wants: the
// formatted bytes (when want is non-empty), idempotence, and — the one that
// catches a deleted or invented backslash — the canonical ADF of the
// formatted document against the canonical ADF of the source.
func assertFormatKeepsMeaning(t *testing.T, md, want string) {
	t.Helper()
	got := fmtMD(md)
	if want != "" && got != want {
		t.Errorf("format %q = %q, want %q", md, got, want)
	}
	if twice := fmtMD(got); twice != got {
		t.Errorf("not idempotent:\n once:  %q\n twice: %q", got, twice)
	}
	if adfGot, adfWant := marshalADF(t, got), marshalADF(t, md); adfGot != adfWant {
		t.Errorf("format %q = %q changed meaning:\n adf(fmt): %s\n adf(src): %s", md, got, adfGot, adfWant)
	}
}
