package markdown

import (
	"bytes"
	"testing"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// The subject of this file is parseGuarded's recovery ladder and the Source
// contract that rests on it. Both were unasserted: the ladder exists because
// goldmark ≤1.8.4 panicked on a tab-indented fence trigger inside a list
// item, and no source is known to make v1.8.5 panic, so every test written
// against a real crasher shape went quiet when the dependency moved. The
// panic is INJECTED here instead, which makes the coverage independent of
// which goldmark version is pinned — and the rungs are what a user-authored
// file falls back through when a future version panics again.
//
// Both tests carry the no-panic case alongside the recovered one, so a guard
// that normalized eagerly — the obvious over-correction — fails here too.

// panicParser is a goldmark parser that panics on the sources panicOn
// accepts and delegates the rest to the real parser. attempts records what
// parseGuarded handed it, in order, which is the rung-by-rung account the
// return value alone cannot give.
type panicParser struct {
	panicOn  func([]byte) bool
	attempts []string
}

func (*panicParser) AddOptions(...parser.Option) {}

func (p *panicParser) Parse(reader text.Reader, opts ...parser.ParseOption) gast.Node {
	src := reader.Source()
	p.attempts = append(p.attempts, string(src))
	if p.panicOn(src) {
		panic("injected goldmark parse panic")
	}
	return NewParser().Parse(text.NewReader(src), opts...)
}

// TestParseGuarded_RecoveryLadder walks every rung. The first row is the
// good case — nothing panics, and the source must come back untouched, so a
// guard that normalized eagerly fails here — and each row after it needs one
// more rung than the row above.
func TestParseGuarded_RecoveryLadder(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		// panicOn decides which sources the injected parser rejects.
		panicOn func([]byte) bool
		// want is the source parseGuarded must report as the one it parsed.
		want string
		// wantAttempts is every source it must have tried, in order.
		wantAttempts []string
	}{
		{
			name:         "no panic parses the source itself",
			src:          "*\n  \t`",
			panicOn:      func([]byte) bool { return false },
			want:         "*\n  \t`",
			wantAttempts: []string{"*\n  \t`"},
		},
		{
			name:         "a panic on a tab retries with the tabs expanded",
			src:          "*\n  \t`",
			panicOn:      func(b []byte) bool { return bytes.Contains(b, []byte("\t")) },
			want:         "*\n      `",
			wantAttempts: []string{"*\n  \t`", "*\n      `"},
		},
		{
			name: "a panic on the expansion too retries with the backticks escaped",
			src:  "*\n  \t`",
			// Only the escaped rung introduces a backslash, so this is
			// "panic until the backticks are escaped" without naming the
			// rung the ladder is expected to reach.
			panicOn:      func(b []byte) bool { return !bytes.Contains(b, []byte(`\`)) },
			want:         "*\n  \t\\`",
			wantAttempts: []string{"*\n  \t`", "*\n      `", "*\n  \t\\`"},
		},
		{
			name:         "a panic on every rewrite falls back to an empty document",
			src:          "*\n  \t`",
			panicOn:      func(b []byte) bool { return len(b) > 0 },
			want:         "",
			wantAttempts: []string{"*\n  \t`", "*\n      `", "*\n  \t\\`", ""},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &panicParser{panicOn: tc.panicOn}
			doc, out := parseGuarded(p, []byte(tc.src))
			if doc == nil {
				t.Fatal("parseGuarded returned no document")
			}
			if string(out) != tc.want {
				t.Errorf("parsed source = %q, want %q", out, tc.want)
			}
			if len(p.attempts) != len(tc.wantAttempts) {
				t.Fatalf("attempts = %q, want %q", p.attempts, tc.wantAttempts)
			}
			for i, got := range p.attempts {
				if got != tc.wantAttempts[i] {
					t.Errorf("attempt %d = %q, want %q", i, got, tc.wantAttempts[i])
				}
			}
		})
	}
}

// TestSource_VerbatimIsFalseAfterRecovery pins the contract that matters to a
// byte-preserving caller: when the guarded parse had to normalize the source,
// Bytes is the normalized copy, the spans address THAT, and Verbatim says so.
//
// It replaces a test of the same name that read this way and asserted
// nothing, because it reached the recovered state through a source shape
// goldmark used to panic on and returned early once the pinned version stopped
// panicking. The panic is injected here, so the assertion runs whatever
// goldmark does; the verbatim half moved to
// TestSource_VerbatimIsTrueWithoutRecovery in source_test.go, which drives it
// through the exported constructor.
//
// The tab sits BEFORE the code span, so expanding it shifts that span: the
// two rows below want different offsets for the same written document, and a
// view left addressing the caller's bytes reads past their end on the second.
func TestSource_VerbatimIsFalseAfterRecovery(t *testing.T) {
	const src = "a\tb `x`\n"
	for _, tc := range []struct {
		name         string
		panicOn      func([]byte) bool
		wantBytes    string
		wantSpan     Span
		wantVerbatim bool
	}{
		{
			name:         "no recovery keeps the source and its offsets",
			panicOn:      func([]byte) bool { return false },
			wantVerbatim: true,
			wantBytes:    src,
			wantSpan:     Span{Start: 4, Stop: 7},
		},
		{
			name:         "recovery reports the copy and its own offsets",
			panicOn:      func(b []byte) bool { return bytes.Contains(b, []byte("\t")) },
			wantVerbatim: false,
			wantBytes:    "a    b `x`\n",
			wantSpan:     Span{Start: 7, Stop: 10},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSourceWithParser(&panicParser{panicOn: tc.panicOn}, []byte(src))
			if got := s.Verbatim(); got != tc.wantVerbatim {
				t.Errorf("Verbatim = %v, want %v", got, tc.wantVerbatim)
			}
			if got := string(s.Bytes()); got != tc.wantBytes {
				t.Errorf("Bytes = %q, want %q", got, tc.wantBytes)
			}
			spans := s.InlineCodeSpans()
			if len(spans) != 1 {
				t.Fatalf("InlineCodeSpans() = %v, want one span", spans)
			}
			if spans[0] != tc.wantSpan {
				t.Errorf("InlineCodeSpans()[0] = %v, want %v", spans[0], tc.wantSpan)
			}
			if got := string(s.Text(spans[0])); got != "`x`" {
				t.Errorf("the span selects %q, want %q", got, "`x`")
			}
		})
	}
}
