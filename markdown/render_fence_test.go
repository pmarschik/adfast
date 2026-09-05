package markdown

import (
	"strings"
	"testing"
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
