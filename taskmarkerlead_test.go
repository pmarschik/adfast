package adfast_test

import (
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// TestTaskMarker_BareLeadKeepsCheckbox covers a checkbox marker that ends
// its own line and hands its content to the blocks indented under it:
//
//   - [ ]
//
//     ORPHAN
//
// The marker has no lead text, so goldmark's checkbox used to be thrown
// away as literal bracket text — the item lost its TODO state, left the
// task list entirely, and published a stray "[ ]" where the checkbox
// belongs. The document also carries a conventional "- [ ] LEAD" item so
// the fix is visibly confined to the bare-marker shape.
func TestTaskMarker_BareLeadKeepsCheckbox(t *testing.T) {
	const md = "- [ ] LEAD\n\n  BODY\n\n- [x]\n\n  ORPHAN\n"

	const want = `{"type":"doc","content":[` +
		`{"type":"taskList","attrs":{"localId":""},"content":[` +
		`{"type":"blockTaskItem","attrs":{"localId":"","state":"TODO"},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"LEAD"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"BODY"}]}]},` +
		`{"type":"blockTaskItem","attrs":{"localId":"","state":"DONE"},"content":[` +
		`{"type":"paragraph"},` +
		`{"type":"paragraph","content":[{"type":"text","text":"ORPHAN"}]}]}]}],"version":1}`

	if got := adfJSON(t, md); got != want {
		t.Errorf("bare marker must stay a task item with an empty lead\n got: %s\nwant: %s", got, want)
	}
}

// TestTaskMarker_BareLeadCarriesNonParagraphBlocks pins the block kinds a
// flattening projection would silently delete: only paragraph inlines
// survive flattening, so a code block or a nested list under a bare marker
// vanished from the encoded document entirely.
func TestTaskMarker_BareLeadCarriesNonParagraphBlocks(t *testing.T) {
	tests := map[string]struct {
		md   string
		want string
	}{
		"code block body": {
			md: "- [ ]\n\n  ```go\n  x := 1\n  ```\n",
			want: `{"type":"doc","content":[{"type":"taskList","attrs":{"localId":""},"content":[` +
				`{"type":"blockTaskItem","attrs":{"localId":"","state":"TODO"},"content":[` +
				`{"type":"paragraph"},` +
				`{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1"}]}]}]}],"version":1}`,
		},
		"nested list body": {
			md: "- [ ]\n  - sub\n",
			want: `{"type":"doc","content":[{"type":"taskList","attrs":{"localId":""},"content":[` +
				`{"type":"blockTaskItem","attrs":{"localId":"","state":"TODO"},"content":[` +
				`{"type":"paragraph"},` +
				`{"type":"bulletList","content":[{"type":"listItem","content":[` +
				`{"type":"paragraph","content":[{"type":"text","text":"sub"}]}]}]}]}]}],"version":1}`,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := adfJSON(t, tc.md); got != tc.want {
				t.Errorf("body block must survive under a bare marker\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// TestTaskMarker_BareLeadFormatsStably checks the render leg agrees with
// the parse leg: an empty-lead task item writes the marker on its own line
// with its blocks indented under it, which is exactly the shape the parse
// leg reads back as the same item. Without the agreement the format pass
// would drop the body it cannot hang off a lead.
func TestTaskMarker_BareLeadFormatsStably(t *testing.T) {
	tests := map[string]string{
		// A task list always renders in its canonical tight form, so the
		// multi-item source loses only the blank line between the items.
		"- [ ] LEAD\n\n  BODY\n\n- [x]\n\n  ORPHAN\n": "- [ ] LEAD\n\n  BODY\n- [x]\n\n  ORPHAN\n",
		"- [ ]\n\n  ORPHAN\n":                         "- [ ]\n\n  ORPHAN\n",
		"- [ ]\n  - sub\n":                            "- [ ]\n  - sub\n",
	}
	for md, want := range tests {
		t.Run(md, func(t *testing.T) {
			once := markdown.Render(markdown.Parse([]byte(md)))
			if once != want {
				t.Errorf("format dropped the marker or its body\n got: %q\nwant: %q", once, want)
			}
			if twice := markdown.Render(markdown.Parse([]byte(once))); twice != once {
				t.Errorf("format is not a fixpoint\nfirst:  %q\nsecond: %q", once, twice)
			}
		})
	}
}

// TestTaskMarker_LoneMarkerStaysLiteral is a preserved-behavior PIN, not a
// regression test: a marker with nothing at all under it heads no content,
// so remark-gfm's rule stands and it keeps parsing as literal bracket text
// in a plain bullet. Rendering it as "- [ ] " would round-trip back to this
// same literal, so the checkbox has nowhere to live.
func TestTaskMarker_LoneMarkerStaysLiteral(t *testing.T) {
	const md = "- [ ]\n"

	const want = `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"[ ]"}]}]}]}],"version":1}`

	if got := adfJSON(t, md); got != want {
		t.Errorf("lone marker must stay literal\n got: %s\nwant: %s", got, want)
	}
}
