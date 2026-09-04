package adfast

import (
	"strings"
	"testing"
)

// The wrapping path is Pipeline.Format with WithPrintWidth, i.e.
// ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat(),
// WithPrintWidth(w)). It replaced a raw-text prose rewrapper that scanned
// the rendered bytes line by line and guessed which lines were syntax. That
// guess had no way to be right: a line ending can BE content (a hard break)
// and a block opener can be a construct the scanner had never heard of (a
// container directive), and the rewrapper flattened both into well-formed
// Markdown that meant something else. Wrapping from the AST cannot make that
// mistake, because the tree already says what every line is.
//
// The cases below are the specification the retired function carried, run
// against the path that replaced it, plus the two constructs it destroyed.

func formatAtWidth(t *testing.T, md string, width int) string {
	t.Helper()
	return NewPipeline().Format(md, WithPrintWidth(width))
}

// TestPrintWidth_KeepsAHardBreak pins the first construct the raw-text
// rewrapper destroyed: it joined the two lines on a space, leaving a stray
// "break\ line two" whose backslash no longer means anything.
func TestPrintWidth_KeepsAHardBreak(t *testing.T) {
	const input = "break\\\nline two\n"
	if got := formatAtWidth(t, input, 40); got != input {
		t.Errorf("hard break not preserved:\ngot  %q\nwant %q", got, input)
	}
}

// TestPrintWidth_KeepsAContainerDirective pins the second construct: the
// rewrapper saw three plain-looking lines and produced ":::info Inner prose.
// :::", a single paragraph with no container in it at all.
func TestPrintWidth_KeepsAContainerDirective(t *testing.T) {
	const input = ":::info\nInner prose.\n:::\n"
	if got := formatAtWidth(t, input, 40); got != input {
		t.Errorf("container directive not preserved:\ngot  %q\nwant %q", got, input)
	}
}

// TestPrintWidth_ReflowsProseAroundThePreservedConstructs is the acceptance
// case: preserving the constructs must not be achieved by declining to wrap.
// The hard break and the container survive byte-identically in the same
// document whose ordinary paragraph is reflowed at the requested width.
func TestPrintWidth_ReflowsProseAroundThePreservedConstructs(t *testing.T) {
	const input = "---\nstatus: Open\n---\n\n:::info\nInner prose.\n:::\n\nbreak\\\nline two\n\n" +
		"This is an ordinary paragraph long enough that it must be reflowed by the wrapping path at the requested width of forty columns.\n"
	const want = "---\nstatus: Open\n---\n\n:::info\nInner prose.\n:::\n\nbreak\\\nline two\n\n" +
		"This is an ordinary paragraph long\nenough that it must be reflowed by the\nwrapping path at the requested width of\nforty columns.\n"

	got := formatAtWidth(t, input, 40)
	if got != want {
		t.Errorf("wrapped document mismatch:\ngot  %q\nwant %q", got, want)
	}
	// Fixpoint: wrapping an already-wrapped document must change nothing.
	if again := formatAtWidth(t, got, 40); again != got {
		t.Errorf("wrapping is not idempotent:\nfirst  %q\nsecond %q", got, again)
	}
}

// TestPrintWidth_KeepsFrontmatterVerbatim is the case that decided the shape
// of this replacement. The frontmatter split lives on the facade, not in the
// markdown package, so a thin in-package wrapper over Parse+Render could not
// have kept it: bare markdown.Render(markdown.Parse(src)) reads the fences as
// a thematic break plus a setext heading and rewrites the block as prose.
// Going through FromMarkdown keeps it byte-for-byte, long lines and all.
func TestPrintWidth_KeepsFrontmatterVerbatim(t *testing.T) {
	const input = "---\nstatus: Open\nsummary: A very long summary that should not be wrapped ever\n---\n\nSome text.\n"
	if got := formatAtWidth(t, input, 40); got != input {
		t.Errorf("frontmatter was modified:\ngot  %q\nwant %q", got, input)
	}
}

func TestPrintWidth_KeepsAFencedCodeBlockVerbatim(t *testing.T) {
	const input = "```\nthis is a very long line inside a code block that should never be wrapped at all ever\n```\n"
	if got := formatAtWidth(t, input, 40); got != input {
		t.Errorf("code block was modified:\ngot  %q\nwant %q", got, input)
	}
}

func TestPrintWidth_DoesNotWrapAHeading(t *testing.T) {
	const input = "# This is a very long heading that should not be wrapped at all by the prose wrapper\n"
	if got := formatAtWidth(t, input, 40); got != input {
		t.Errorf("heading was wrapped:\ngot  %q\nwant %q", got, input)
	}
}

// TestPrintWidth_WrapsAListItemUnderItsMarker records a DELIBERATE change
// from the retired function, which refused to touch any line opening a list
// and so left long items overlong. Wrapping from the AST knows the item's
// marker width, so it reflows the item text and indents the continuation to
// line up under it — prettier's behavior, and re-parsing the result gives
// back the same list.
func TestPrintWidth_WrapsAListItemUnderItsMarker(t *testing.T) {
	const input = "- This is a list item that is quite long and should not be wrapped by the wrapper\n- Another item\n"
	const want = "- This is a list item that is quite long\n  and should not be wrapped by the\n  wrapper\n- Another item\n"
	got := formatAtWidth(t, input, 40)
	if got != want {
		t.Errorf("list item wrap mismatch:\ngot  %q\nwant %q", got, want)
	}
	if again := formatAtWidth(t, got, 40); again != got {
		t.Errorf("wrapped list is not a fixpoint:\nfirst  %q\nsecond %q", got, again)
	}
}

// TestPrintWidth_KeepsATableWiderThanTheWidth pins that a table is never
// reflowed to fit — cells cannot move to the next line. The delimiter row is
// padded to the column widths, which is the formatter canonicalizing the
// table it parsed, not the wrapper cutting it up; the retired function left
// the row as authored because it never looked inside.
func TestPrintWidth_KeepsATableWiderThanTheWidth(t *testing.T) {
	const input = "| Column A | Column B | Very Long Column C That Exceeds Width |\n| --- | --- | --- |\n"
	const want = "| Column A | Column B | Very Long Column C That Exceeds Width |\n" +
		"| -------- | -------- | ------------------------------------- |\n"
	if got := formatAtWidth(t, input, 40); got != want {
		t.Errorf("table wrap mismatch:\ngot  %q\nwant %q", got, want)
	}
}

// TestPrintWidth_KeepsAnHTMLCommentVerbatim covers both the single-line and
// the multi-line form. The comment text is untouched; it becomes its own
// block, separated from the following paragraph by a blank line, because a
// comment IS a block and the retired function's output ran it into the prose.
func TestPrintWidth_KeepsAnHTMLCommentVerbatim(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{
			name:  "single line",
			input: "<!-- adfast:lint-ignore-next-line some-rule -->\nSome text.\n",
			want:  "<!-- adfast:lint-ignore-next-line some-rule -->\n\nSome text.\n",
		},
		{
			name:  "multi line",
			input: "<!--\nThis is a long multi-line comment that should be preserved as-is\n-->\nSome text.\n",
			want:  "<!--\nThis is a long multi-line comment that should be preserved as-is\n-->\n\nSome text.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAtWidth(t, tc.input, 40)
			if got != tc.want {
				t.Errorf("comment mismatch:\ngot  %q\nwant %q", got, tc.want)
			}
			if again := formatAtWidth(t, got, 40); again != got {
				t.Errorf("comment block is not a fixpoint:\nfirst  %q\nsecond %q", got, again)
			}
		})
	}
}

// TestPrintWidth_FullDocument is the retired function's end-to-end case: a
// document mixing frontmatter, headings, prose, a list and a code block. It
// asserted only that nothing was lost; the whole output is pinned here, since
// wrapping from the AST is deterministic enough to state exactly.
func TestPrintWidth_FullDocument(t *testing.T) {
	const input = `---
status: Open
---

# PROJ-1 Hello World

This is a paragraph with enough words that it should definitely be wrapped at the default width of eighty characters.

## Description

Another paragraph here that is also very long and needs to be wrapped properly at the specified width.

- List item one
- List item two

` + "```go\nfunc main() { fmt.Println(\"this is a long line in code\") }\n```\n"

	const want = `---
status: Open
---

# PROJ-1 Hello World

This is a paragraph with enough words that it should definitely be wrapped at
the default width of eighty characters.

## Description

Another paragraph here that is also very long and needs to be wrapped properly
at the specified width.

- List item one
- List item two

` + "```go\nfunc main() { fmt.Println(\"this is a long line in code\") }\n```\n"

	got := formatAtWidth(t, input, 80)
	if got != want {
		t.Errorf("full document mismatch:\ngot  %q\nwant %q", got, want)
	}
	for line := range strings.SplitSeq(got, "\n") {
		if strings.HasPrefix(line, "This is a paragraph") && len(line) > 80 {
			t.Errorf("prose line exceeds the width (%d): %q", len(line), line)
		}
	}
	if again := formatAtWidth(t, got, 80); again != got {
		t.Errorf("full document is not a fixpoint:\nfirst  %q\nsecond %q", got, again)
	}
}

// TestPrintWidth_ZeroWidthLeavesProseUnwrapped keeps the retired function's
// zero-width case with the opposite meaning, which is the honest one: a width
// of zero used to be silently replaced by 80, so asking for no wrapping got
// you the default. Zero now means what it says.
func TestPrintWidth_ZeroWidthLeavesProseUnwrapped(t *testing.T) {
	const input = "This is a long paragraph that should be wrapped at eighty characters because that is the default width.\n"
	if got := formatAtWidth(t, input, 0); got != input {
		t.Errorf("width 0 wrapped the prose:\ngot  %q\nwant %q", got, input)
	}
}

// TestNoWrapIsTheAliasForWidthZero is a PRESERVED-BEHAVIOR PIN for what
// WithNoWrap's doc comment now states: it is deliberately the readable
// alias for WithPrintWidth(0), so the two produce identical bytes, and
// given both, no-wrap wins in either order because the facade applies
// printWidth first and no-wrap after it. Nothing changed in the option
// plumbing; the comment previously said neither thing, leaving the pair
// looking like an accident one of the two names should be removed to fix.
func TestNoWrapIsTheAliasForWidthZero(t *testing.T) {
	const input = "This is a long paragraph that should be wrapped at eighty characters because that is the default width.\n"

	// Precondition: wrapping must be live in this build, or every
	// comparison below would pass on a formatter that never wraps.
	if wrapped := NewPipeline().Format(input, WithPrintWidth(40)); wrapped == input {
		t.Fatalf("width 40 did not wrap; the assertions below would be vacuous: %q", wrapped)
	}

	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{"WithPrintWidth(0)", []Option{WithPrintWidth(0)}},
		{"WithNoWrap()", []Option{WithNoWrap()}},
		{"width then no-wrap", []Option{WithPrintWidth(40), WithNoWrap()}},
		{"no-wrap then width", []Option{WithNoWrap(), WithPrintWidth(40)}},
	} {
		if got := NewPipeline().Format(input, tc.opts...); got != input {
			t.Errorf("%s wrapped the prose:\ngot  %q\nwant %q", tc.name, got, input)
		}
	}
}

// TestPrintWidth_ShortLinesThatOpenNoBlock carries forward the inputs of a
// regression test whose defect no longer has a subject: the retired function
// sliced the first four bytes of a digit-led line to sniff an ordered-list
// marker and panicked on a three-character one. Nothing slices raw lines any
// more, so the panic cannot recur by construction — but the inputs still
// belong to the wrapping path, and the parse it now goes through is
// infallible, so they are pinned as output rather than as a no-panic probe.
func TestPrintWidth_ShortLinesThatOpenNoBlock(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"1. ", "1.\n"},
		{"3 m", "3 m\n"},
		{"9.x", "9.x\n"},
		{"12.", "12.\n"},
	} {
		if got := formatAtWidth(t, tc.input, 40); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}
