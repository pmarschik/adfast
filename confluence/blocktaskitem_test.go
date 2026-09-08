package confluence

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// TestBlockTaskItemJoinsParagraphs is the defect's own case: a task item
// whose body is two paragraphs. Confluence downgrades the blockTaskItem
// to a plain taskItem and concatenates the block bodies with NOTHING
// between them, so without the separator this document publishes as
// "FIRSTPARASECONDPARA" on a save that reports success.
//
// The good case rides in the same document: the healthy single-paragraph
// item above it must come out untouched, with no stray space.
func TestBlockTaskItemJoinsParagraphs(t *testing.T) {
	js := docJSON(t, mdToADF(t, "- [ ] GOODITEM\n- [ ] FIRSTPARA\n\n  SECONDPARA\n"))

	if !strings.Contains(js, `{"type":"text","text":"FIRSTPARA "}`) {
		t.Errorf("the first block does not end in the separator:\n%s", js)
	}
	if strings.Contains(js, `{"type":"text","text":"FIRSTPARA"}`) {
		t.Errorf("the first block still ends bare, so the two blocks concatenate as FIRSTPARASECONDPARA:\n%s", js)
	}
	// The last block closes the item: nothing follows it to run into.
	if !strings.Contains(js, `{"type":"text","text":"SECONDPARA"}`) {
		t.Errorf("the last block gained a trailing separator it does not need:\n%s", js)
	}
	// The good case: an item Confluence keeps as a plain taskItem.
	if !strings.Contains(js, `{"type":"taskItem","attrs":{"localId":"","state":"TODO"},"content":[{"type":"text","text":"GOODITEM"}]}`) {
		t.Errorf("the single-paragraph task item was not left alone:\n%s", js)
	}
	if !adf.IsWireSafe(mdToADF(t, "- [ ] FIRSTPARA\n\n  SECONDPARA\n")) {
		t.Error("the separated document is not wire-safe")
	}
}

// TestBlockTaskItemSeparatesEveryBoundary covers a body of three
// paragraphs: every boundary needs a separator, and only the last block
// must be left bare.
func TestBlockTaskItemSeparatesEveryBoundary(t *testing.T) {
	js := docJSON(t, mdToADF(t, "- [ ] ONE\n\n  TWO\n\n  THREE\n"))

	want := `"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"ONE "}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"TWO "}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"THREE"}]}]`
	if !strings.Contains(js, want) {
		t.Errorf("want the body\n%s\ngot\n%s", want, js)
	}
}

// TestBlockTaskItemSeparatesNonParagraphBlock covers the block kinds that
// have no trailing text to extend. A space appended inside a code block
// would be an edit to the code, so the separator is a paragraph of its
// own after the block. Confluence's downgrade of a code block inside a
// task item is not measured; if it contributes text, that text is
// separated from what follows, which is the point.
func TestBlockTaskItemSeparatesNonParagraphBlock(t *testing.T) {
	js := docJSON(t, mdToADF(t, "- [ ] LEAD\n\n  ```go\n  x := 1\n  ```\n\n  TAIL\n"))

	if !strings.Contains(js, `{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1"}]},{"type":"paragraph","content":[{"type":"text","text":" "}]}`) {
		t.Errorf("the code block did not gain a separator paragraph:\n%s", js)
	}
	// The good case: the code itself is untouched — no space smuggled in.
	if strings.Contains(js, `"text":"x := 1 "`) {
		t.Errorf("the separator was written into the code block's own content:\n%s", js)
	}
}

// TestBlockTaskItemSeparatesNestedItem covers a task list nested inside a
// task item: the inner blockTaskItem is downgraded by the same rule as
// the outer one, so it needs the same separator. The transform recurses
// before it rewrites for exactly this case.
func TestBlockTaskItemSeparatesNestedItem(t *testing.T) {
	js := docJSON(t, mdToADF(t, "- [ ] LEAD\n\n  - [ ] SUBLEAD\n\n    SUBTAIL\n\n  TAIL\n"))

	for _, want := range []string{
		`{"type":"text","text":"SUBLEAD "}`,
		`{"type":"text","text":"LEAD "}`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("missing %s in\n%s", want, js)
		}
	}
}

// TestBlockTaskItemGenericEncodeUnchanged is a PIN, not a regression
// test: it passes against the implementation before the separator too,
// and it is here to hold the separator inside the Confluence dialect.
//
// This is the Jira-leg control. Jira KEEPS blockTaskItem — the kind is
// absent from jira.UnsupportedKinds and the 2026-07-22 render probe found
// it rendered first-class — so nothing concatenates there and a separator
// applied on that leg would be corruption of its own. The Jira leg is the
// generic encode plus jira.MarkdownOptions' own transforms, none of which
// touches a task list, so pinning the generic encode pins it: the bytes
// below are the ones the Confluence leg itself produced before the
// separator existed. (The control lives here rather than in the jira
// module because confluence/ does not depend on jira/, and adding the
// dependency to assert a negative would be a worse trade.)
func TestBlockTaskItemGenericEncodeUnchanged(t *testing.T) {
	js := docJSON(t, adfast.ToADF(adfast.FromMarkdown("- [ ] FIRSTPARA\n\n  SECONDPARA\n")))

	want := `{"type":"doc","content":[{"type":"taskList","attrs":{"localId":""},"content":[` +
		`{"type":"blockTaskItem","attrs":{"localId":"","state":"TODO"},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"FIRSTPARA"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"SECONDPARA"}]}]}]}],"version":1}`
	if js != want {
		t.Errorf("the generic encode changed; the separator escaped the Confluence dialect\nwant %s\ngot  %s", want, js)
	}
}

// TestBlockTaskItemSingleBlockUntouched is a PIN: an item with one block
// has no boundary, and a task item Confluence keeps as-is must not grow a
// separator either. It passes against the implementation before the
// separator too.
func TestBlockTaskItemSingleBlockUntouched(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.TaskList{Content: []adf.Node{
			&adf.BlockTaskItem{State: "TODO", Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "ALONE"}}},
			}},
			&adf.TaskItem{State: "TODO", Content: []adf.Node{&adf.Text{Text: "INLINE"}}},
		}},
	}}
	if got := docJSON(t, SeparateBlockTaskItems(doc)); got != docJSON(t, doc) {
		t.Errorf("a single-block item was rewritten:\n%s", got)
	}
}

// TestBlockTaskItemSeparatorIdempotent is a PIN: applying the transform
// twice must not stack separators, so a block already ending in
// whitespace is left alone. It passes against the implementation before
// the separator too (a no-op is trivially idempotent), and it is here to
// keep the "already ends in whitespace" guard honest.
func TestBlockTaskItemSeparatorIdempotent(t *testing.T) {
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.TaskList{Content: []adf.Node{
			&adf.BlockTaskItem{State: "TODO", Content: []adf.Node{
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "FIRSTPARA"}}},
				&adf.CodeBlock{Content: []adf.Node{&adf.Text{Text: "x := 1"}}},
				&adf.Paragraph{Content: []adf.Node{&adf.Text{Text: "SECONDPARA"}}},
			}},
		}},
	}}
	once := SeparateBlockTaskItems(doc)
	if got, want := docJSON(t, SeparateBlockTaskItems(once)), docJSON(t, once); got != want {
		t.Errorf("the second pass stacked separators\nwant %s\ngot  %s", want, got)
	}
}
