package adfast_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

type listRoundTrip struct {
	name string
	md   string
	want string
}

// A list that is a list item's FIRST block is written by
// renderItemFirstBlock, which supplies the item's own indent and marker
// prefix. blockRenderVisitor.VisitList used to add "  " per nesting depth
// on top of that, counting the nesting twice: at depth 2 the four extra
// columns pushed the inner marker line past the indented-code threshold,
// so a nested empty ordered list came back as a code block.
func TestNestedListAsFirstBlockIsNotIndentedCode(t *testing.T) {
	t.Parallel()
	runListRoundTrips(t, []listRoundTrip{
		{
			// The reported repro: the innermost item's only child is an
			// empty "0)" ordered list, three bullet levels down.
			name: "empty ordered list three levels down",
			md:   "*     0\n  * 0\n    * 0)",
			want: "- ```\n  0\n  ```\n  - 0\n    - 0.\n",
		},
		{
			name: "plain nested lists stay flush",
			md:   "- - - x",
			want: "- - - x\n",
		},
	})
}

// A block after a nested list inside a list item must be blank-separated:
// it lands on the OUTER item's content column, which is a lazy continuation
// line for the paragraph that ended the nested list. remark-stringify emits
// no blank there and loses the block into that paragraph, so this is a
// DELIBERATE divergence — see markdown.blockRunsToBlankLine and the
// re-pinned "after list" entry in testdata/directive_fixtures.json.
func TestBlockAfterNestedListSurvivesReparse(t *testing.T) {
	t.Parallel()
	runListRoundTrips(t, []listRoundTrip{
		{
			name: "paragraph after a nested list",
			md:   "- - x\n\n  y",
			want: "- - x\n\n  y\n",
		},
		{
			name: "the remark reference corpus shape",
			md:   "- before\n\n  ```go\n  x := 1\n  ```\n\n  after code\n\n  - sub\n\n  after list\n",
			want: "- before\n  ```go\n  x := 1\n  ```\n  after code\n  - sub\n\n  after list\n",
		},
		{
			// A list ending in an EMPTY item absorbs nothing, and forcing a
			// blank line there ejects the paragraph from the item entirely.
			name: "empty nested list keeps its tight follow block",
			md:   "- 0.\n  0\n",
			want: "- 0.\n  0\n",
		},
	})
}

// A table after a paragraph inside a list item must be blank-separated too:
// a GFM table cannot interrupt a paragraph, so attached they re-parse as one
// table whose header is the paragraph's last line — the table's own header
// slides down into the body. See markdown.blockRunsToBlankLine; only the
// pairing is new, the rule is the run-on one above.
func TestTableAfterParagraphInAListItemKeepsItsHeader(t *testing.T) {
	t.Parallel()
	runListRoundTrips(t, []listRoundTrip{
		{
			// The fuzz repro: the item's blocks are a code block, a
			// paragraph and a table, and nothing in the item's spread
			// asked for the blank the table needs.
			name: "table after a paragraph",
			md:   "*     0\n  0\n\n  --\n--\n0",
			want: "- ```\n  0\n  ```\n  0\n\n  | -- |\n  | -- |\n  | 0  |\n",
		},
		{
			// Blocks that open with their own marker still attach.
			name: "code fence after a paragraph stays tight",
			md:   "- 0\n  ```\n  x\n  ```\n",
			want: "- 0\n  ```\n  x\n  ```\n",
		},
		{
			name: "blockquote after a paragraph stays tight",
			md:   "- 0\n  > q\n",
			want: "- 0\n  > q\n",
		},
	})
}

// A nested task list shares the bullet-alternation chain with the plain
// lists around it. renderTaskList used to hard-code "-" inside a list item,
// so a task list next to a plain one re-parsed as a single list at the same
// column — and one checkbox anywhere turns the whole list into a task list,
// giving every plain item a spurious "[ ]".
func TestNestedTaskListAlternatesItsBullet(t *testing.T) {
	t.Parallel()
	runListRoundTrips(t, []listRoundTrip{
		{
			name: "task list after a plain nested list",
			md:   "* + 0\n  * [X] 0",
			want: "- - 0\n  * [x] 0\n",
		},
		{
			name: "plain nested list after a task list",
			md:   "* + [x] 0\n  * 0",
			want: "- - [x] 0\n  * 0\n",
		},
		{
			name: "three alternating nested lists",
			md:   "* + [x] 0\n  * 1\n  + [ ] 2",
			want: "- - [x] 0\n  * 1\n  - [ ] 2\n",
		},
	})
}

// runListRoundTrips asserts the rendered form, its idempotence, and that no
// paragraph was merged away by the re-parse.
func runListRoundTrips(t *testing.T, tests []listRoundTrip) {
	t.Helper()
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
			if before, after := paragraphTexts(tt.md), paragraphTexts(got); before != after {
				t.Errorf("paragraphs changed across the render:\n before: %q\n after:  %q", before, after)
			}
		})
	}
}

// paragraphTexts returns every paragraph's plain text, in document order.
func paragraphTexts(md string) string {
	var out strings.Builder
	for _, top := range adfast.ToADF(adfast.FromMarkdown(md)).Content {
		for n := range adf.Walk(top) {
			p, ok := n.(*adf.Paragraph)
			if !ok {
				continue
			}
			out.WriteString("|")
			for _, c := range p.Content {
				out.WriteString(adf.NodeText(c))
			}
		}
	}
	return out.String()
}
