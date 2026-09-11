package markdown

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The fence content is a line of its own between the two fence lines, and
// the two serializers disagree on an empty block. Prettier prints that line
// unconditionally, so an empty fence keeps one blank line; remark-stringify
// writes the line only when there is content. Both references were measured
// (prettier 3.9.6 through --parser markdown, mdast-util-to-markdown 2 through
// toMarkdown on a hand-built code node):
//
//	prettier   "```json\n```\n"     -> "```json\n\n```\n"
//	remark     {code json ""}       -> "```json\n```\n"
//
// The blank line is language-independent, so a bare fence behaves the same.
// Losing it makes every document holding an empty fence show a phantom
// one-line change in a canonical diff, on both sides, forever.
func TestRenderFence_PrettierKeepsTheBlankContentLine(t *testing.T) {
	t.Parallel()
	// One document, so the fix and the shapes that must NOT move are
	// measured on the same render: an empty fence, a bare empty fence, a
	// whitespace-only fence (whose line survives the trailing-space trim as
	// a blank one) and a fence with real content.
	const src = "```json\n```\n\n```\n```\n\n```json\n   \n```\n\n```json\na\n```\n"
	const want = "```json\n\n```\n\n```\n\n```\n\n```json\n\n```\n\n```json\na\n```\n"

	got := Render(Parse([]byte(src)), WithPrettierText())
	if got != want {
		t.Errorf("prettier render = %q, want %q", got, want)
	}
	// A blank content line has to re-parse as empty content, or the next
	// format pass would keep adding lines.
	if again := Render(Parse([]byte(got)), WithPrettierText()); again != got {
		t.Errorf("prettier render not stable: %q then %q", got, again)
	}
	// Neither fence may grow: the blank line is content, not a longer
	// fence, and it must not push the closing fence out of the block.
	if n := strings.Count(got, "```"); n != 8 {
		t.Errorf("fence count = %d, want 8, in %q", n, got)
	}
}

// The plain render follows remark-stringify, which omits the content line
// of an empty block entirely. A PIN: this render never wrote the blank
// line, and the prettier fix must not leak into it, because the adf→md leg
// (an ADF codeBlock with no content) goes through here.
func TestRenderFence_RemarkOmitsTheContentLineOfAnEmptyBlock(t *testing.T) {
	t.Parallel()
	const src = "```json\n```\n\n```\n```\n\n```json\n   \n```\n\n```json\na\n```\n"
	// Unchanged, whitespace and all: the remark render does not trim inside
	// a code block either.
	if got := Render(Parse([]byte(src))); got != src {
		t.Errorf("remark render = %q, want %q", got, src)
	}
}

// fenceValues is the sequence of code values a document parses to, so a
// single document can carry the shapes that must move next to the ones
// that must not.
func fenceValues(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, child := range ast.Children(Parse([]byte(src))) {
		if code, ok := child.(*ast.Code); ok {
			out = append(out, code.Value)
		}
	}
	return out
}

// fenceTrailingDoc holds, in order: a fence ending in a blank line, a fence
// padded with blank lines on BOTH sides, a fence with an interior blank
// line, and a plain one-line fence. The last two are the GOOD cases — the
// lift never lost a leading or an interior blank line, and only the
// trailing ones were at stake.
const fenceTrailingDoc = "```json\na\n\n```\n\n```json\n\n\na\n\n\n```\n\n```json\na\n\nb\n```\n\n```json\na\n```\n"

// A blank line at the END of a fence is content, and the lift dropped it.
// goldmark hands out the body one line at a time, each ending in "\n", so
// the buffer carries exactly one terminator more than the block has lines;
// only that one comes off. Trimming every trailing newline instead deleted
// the block's own last lines.
//
// The values are the reference's, measured with mdast-util-from-markdown
// 2.1.2 / micromark 4 (the parser remark-parse and the storysmith-md
// reference both use) on this exact document:
//
//	["a\n", "\n\na\n\n", "a\n\nb", "a"]
//
// It is not a rendering detail: this value is also the text of the ADF
// codeBlock, and the reference's own md→ADF leg carries the same bytes
// (measured: "```json\na\n\n```" → codeBlock text "a\n").
func TestParseFence_KeepsTheTrailingBlankLinesOfTheBody(t *testing.T) {
	t.Parallel()
	want := []string{"a\n", "\n\na\n\n", "a\n\nb", "a"}
	got := fenceValues(t, fenceTrailingDoc)
	if len(got) != len(want) {
		t.Fatalf("parsed %d code blocks, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("code[%d] value = %q, want %q", i, got[i], want[i])
		}
	}
}

// Both serializers write the trailing blank lines back out — they agree
// here, unlike on the empty fence above. Measured on this document:
//
//	prettier 3.8.1            → identical to the source
//	mdast-util-to-markdown 2  → identical to the source
//
// So the document is its own expected render in both modes, and a
// canonical diff over it is empty instead of deleting a line per fence.
func TestRenderFence_TrailingBlankLinesSurviveBothRenders(t *testing.T) {
	t.Parallel()
	for _, mode := range []struct {
		name string
		opts []RenderOption
	}{
		{"remark", nil},
		{"prettier", []RenderOption{WithPrettierText()}},
	} {
		got := Render(Parse([]byte(fenceTrailingDoc)), mode.opts...)
		if got != fenceTrailingDoc {
			t.Errorf("%s render = %q, want %q", mode.name, got, fenceTrailingDoc)
		}
		if again := Render(Parse([]byte(got)), mode.opts...); again != got {
			t.Errorf("%s render not stable: %q then %q", mode.name, got, again)
		}
	}
}

// A fence whose last line is whitespace only made the prettier render
// oscillate: the per-line trailing-space trim turned that line into a
// blank one, and the next parse then deleted it, so two format passes
// produced two different files forever. The line survives the second pass
// now because the parse keeps it.
//
// The document holds the oscillating shape, the same fence already written
// with a blank last line, and a plain fence that must not gain one.
// Measured with prettier 3.8.1, which converges on the first pass:
//
//	"```json\na\n   \n```\n\n```json\na\n\n```\n\n```json\na\n```\n"
//	→ "```json\na\n\n```\n\n```json\na\n\n```\n\n```json\na\n```\n"
func TestRenderFence_PrettierIsStableOnAWhitespaceOnlyLastLine(t *testing.T) {
	t.Parallel()
	const src = "```json\na\n   \n```\n\n```json\na\n\n```\n\n```json\na\n```\n"
	const want = "```json\na\n\n```\n\n```json\na\n\n```\n\n```json\na\n```\n"

	got := Render(Parse([]byte(src)), WithPrettierText())
	if got != want {
		t.Errorf("prettier render = %q, want %q", got, want)
	}
	if again := Render(Parse([]byte(got)), WithPrettierText()); again != got {
		t.Errorf("prettier render not stable: %q then %q", got, again)
	}
	// The remark render keeps the whitespace line verbatim — it trims
	// nothing inside a code block — and is stable as it stands.
	if r := Render(Parse([]byte(src))); r != src {
		t.Errorf("remark render = %q, want %q", r, src)
	}
}

// codeValues is fenceValues for a document whose code blocks are not all
// at the top level — an indented block can sit inside a list item.
func codeValues(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if code, ok := n.(*ast.Code); ok {
			out = append(out, code.Value)
		}
		for _, child := range ast.Children(n) {
			walk(child)
		}
	}
	walk(Parse([]byte(src)))
	return out
}

// indentedTrailingDoc holds, in order: an indented block whose trailing
// blank run ENDS in an indented blank line, one whose trailing lines are
// entirely empty, one with an interior blank line, and a plain one-line
// block. Only the first moves; the last three are the GOOD cases, and a
// paragraph separates the blocks so they stay four blocks.
const indentedTrailingDoc = "    a\n\n    \np\n\n    a\n\n\np\n\n    a\n\n    b\np\n\n    a\n"

// A blank line at the end of an INDENTED block is content when it is
// indented as far as the code, and it used to be lost. goldmark drops
// those lines in the parser — codeBlockParser.Close slices them off the
// segment list before the lift runs — so restoring them means reading them
// back off the source; see indentedCodeTrailingLines.
//
// The values are the reference's, measured with mdast-util-from-markdown
// 2.1.2 / micromark 4 on this exact document:
//
//	["a\n", "a", "a\n\nb", "a"]
//
// It is not a rendering detail: this value is also the text of the ADF
// codeBlock, so the loss reached the wire (measured on "    a\n\n    \n":
// codeBlock text "a" before, "a\n" after, and the reference's md→ADF leg
// emits "a\n").
func TestParseIndentedCode_KeepsTheTrailingBlankLinesInsideTheBlock(t *testing.T) {
	t.Parallel()
	want := []string{"a\n", "a", "a\n\nb", "a"}
	got := codeValues(t, indentedTrailingDoc)
	if len(got) != len(want) {
		t.Fatalf("parsed %d code blocks, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("code[%d] value = %q, want %q", i, got[i], want[i])
		}
	}
}

// The AST has no indented form, so both renders write these blocks as
// fences — and the recovered line has to survive that and re-parse to
// itself, or a format pass would keep deleting it. Measured with
// mdast-util-to-markdown 2 on indentedTrailingDoc, which is also stable
// there:
//
//	"```\na\n\n```\n\np\n\n```\na\n```\n\np\n\n```\na\n\nb\n```\n\np\n\n```\na\n```\n"
func TestRenderIndentedCode_TheRecoveredLineSurvivesBothRenders(t *testing.T) {
	t.Parallel()
	const want = "```\na\n\n```\n\np\n\n```\na\n```\n\np\n\n```\na\n\nb\n```\n\np\n\n```\na\n```\n"
	for _, mode := range []struct {
		name string
		opts []RenderOption
	}{
		{"remark", nil},
		{"prettier", []RenderOption{WithPrettierText()}},
	} {
		got := Render(Parse([]byte(indentedTrailingDoc)), mode.opts...)
		if got != want {
			t.Errorf("%s render = %q, want %q", mode.name, got, want)
		}
		if again := Render(Parse([]byte(got)), mode.opts...); again != got {
			t.Errorf("%s render not stable: %q then %q", mode.name, got, again)
		}
	}
}

// How far the trailing run reaches is the whole rule, and it is not "all
// blank lines" in either direction: the block ends at the LAST blank line
// indented as far as its content, and that line's own terminator is not
// part of the value. So one indented blank line adds nothing, two add one,
// and a run of entirely empty lines adds nothing however long it is.
//
// Every want is the reference's, measured with mdast-util-from-markdown
// 2.1.2 / micromark 4. The rows that must NOT move sit next to the ones
// that must.
func TestParseIndentedCode_TheRunEndsAtTheLastIndentedBlankLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, src, want string }{
		{"no trailing blank", "    a\n", "a"},
		{"one empty", "    a\n\n", "a"},
		{"one indented blank", "    a\n    \n", "a"},
		{"empty then indented", "    a\n\n    \n", "a\n"},
		{"two empty", "    a\n\n\n", "a"},
		{"two indented blank", "    a\n    \n    \n", "a\n"},
		{"three indented blank", "    a\n    \n    \n    \n", "a\n\n"},
		{"alternating", "    a\n\n    \n\n    \n", "a\n\n\n"},
		{"short then indented", "    a\n  \n    \n", "a\n"},
		{"indented then short", "    a\n    \n  \n", "a"},
		{"no final newline", "    a\n    ", "a"},
		{"then a paragraph", "    a\n\n    \np\n", "a\n"},
		{"interior blank", "    a\n\n    b\n", "a\n\nb"},
		// Past the code indent the rest of the blank line is content,
		// which is why the columns and not the blankness decide.
		{"over the indent", "    a\n     \n", "a\n "},
		{"over the indent, then flush", "    a\n     \n    \n", "a\n "},
		{"flush, then over the indent", "    a\n    \n     \n", "a\n\n "},
		// The code indent is 4 past the block's own left edge, so an
		// over-indented block and one inside a list item both keep the
		// columns the reference keeps.
		{"over-indented block", "        a\n        \n", "    a\n    "},
		{"over-indented, flush tail", "        a\n    \n", "    a"},
		{"inside a list item", "- x\n\n      a\n\n      \n", "a\n"},
	} {
		if got := codeValues(t, tc.src); len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s: indented code of %q = %q, want [%q]", tc.name, tc.src, got, tc.want)
		}
	}
}

// A PIN, and the two shapes the recovery deliberately skips, because the
// column arithmetic of a tab and the line ending of a CRLF file are each a
// divergence of their own. Both keep goldmark's answer, so both still
// differ from the reference — recorded here so the gap is a decision and
// not a surprise:
//
//	"    a\n\t \n"        micromark "a\n "  goldmark "a"
//	"    a\r\n    \r\n"   micromark "a"     goldmark "a\r"
//
// The CR one is the shared value helper's, not the recovery's: a fence in
// a CRLF file carries the same stray "\r" (measured: "```\na\r\n```\n"
// parses to "a\r"). The good case is the same body with LF, which the
// recovery does reach.
func TestParseIndentedCode_ATabOrACarriageReturnKeepsGoldmarksAnswer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, src, want string }{
		{"tab past the indent", "    a\n\t \n", "a"},
		{"tab at the indent", "    a\n\t\n", "a"},
		{"tab indents the code", "\ta\n\t\n", "a"},
		{"crlf", "    a\r\n    \r\n", "a\r"},
		{"same body with lf", "    a\n\n    \n", "a\n"},
	} {
		if got := codeValues(t, tc.src); len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s: indented code of %q = %q, want [%q]", tc.name, tc.src, got, tc.want)
		}
	}
}
