package convert

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/markdown"
)

// Adjacent same-mark text nodes at ADF encode time, pinned both as a
// direct unit test of mergeAdjacentADFText/marksSetEqual and as an
// end-to-end probe through the real parser.
//
// The pivot AST keeps a logical run of plain prose split across several
// sibling nodes whenever something in the middle of it parses as its own
// node and then dissolves back to text — a bare token that merely looks
// like a directive ("pre :name post") is the common case, and the AST
// has to keep that split: directives must stay separately addressable
// there for dynamic manipulation before the tree is finalized. But ADF
// has no node kind for "this text used to be more than one AST node",
// and the split is only invisible in rendered Markdown/HTML — it is very
// visible in the raw ADF payload. Jira's own ProseMirror editor merges
// adjacent same-mark text nodes on every save, so a document a human
// edited there would never produce the split form; an encoder that still
// emits it turns "nothing changed" into a diff for any push-preflight
// comparison that reads the raw ADF. mergeAdjacentADFText closes that
// gap at the one place it belongs: ToADF's inline flatten, not the
// parse/AST stage.

// TestMergeAdjacentADFText_BareDirectiveTokenMerges is the FIX positive
// case: "pre :name post" parses as three AST siblings under the same
// (empty) mark context — the bare-name rule keeps ":name" from being
// promoted to a real directive, but it is still parsed and re-flattened
// as its own node — and the ADF encode must fold them into one text node.
func TestMergeAdjacentADFText_BareDirectiveTokenMerges(t *testing.T) {
	t.Parallel()
	doc := ToADF(markdown.Parse([]byte("pre :name post")))
	texts := collectTexts(t, doc)
	if len(texts) != 1 {
		t.Fatalf("text nodes = %d %v, want exactly 1 merged node", len(texts), texts)
	}
	if texts[0].Text != "pre :name post" {
		t.Errorf("merged text = %q, want %q", texts[0].Text, "pre :name post")
	}
	if len(texts[0].Marks) != 0 {
		t.Errorf("merged text carries marks %v, want none", texts[0].Marks)
	}
}

// TestMergeAdjacentADFText_DifferentMarksDoNotMerge is the FIX negative
// case: adjacent text whose mark sets differ — here, plain text next to
// a bold run — must stay two nodes. Proving this alongside the positive
// case is what shows the merge is mark-aware rather than a blind
// concatenation of everything flattenChildren produces.
func TestMergeAdjacentADFText_DifferentMarksDoNotMerge(t *testing.T) {
	t.Parallel()
	doc := ToADF(markdown.Parse([]byte("plain **bold**")))
	texts := collectTexts(t, doc)
	if len(texts) != 2 {
		t.Fatalf("text nodes = %d %v, want exactly 2 (plain and bold stay separate)", len(texts), texts)
	}
	if texts[0].Text != "plain " || len(texts[0].Marks) != 0 {
		t.Errorf("first node = %q marks=%v, want \"plain \" with no marks", texts[0].Text, texts[0].Marks)
	}
	if texts[1].Text != "bold" || !adf.HasMark(texts[1].Marks, "strong") {
		t.Errorf("second node = %q marks=%v, want \"bold\" marked strong", texts[1].Text, texts[1].Marks)
	}
}

// TestMergeAdjacentADFText_Unit exercises mergeAdjacentADFText directly,
// independent of the parser, including the order-independent mark-set
// comparison: a node marked [strong, em] must merge with one marked
// [em, strong], the same normalization trap order-dependent mark
// comparison hits elsewhere (an unordered list of modifiers is one set
// regardless of which order a builder happened to append them in).
func TestMergeAdjacentADFText_Unit(t *testing.T) {
	t.Parallel()
	strongThenEm := []adf.Mark{&adf.Strong{}, &adf.Em{}}
	emThenStrong := []adf.Mark{&adf.Em{}, &adf.Strong{}}
	linkA := []adf.Mark{&adf.Link{Href: new("https://example.com")}}
	linkB := []adf.Mark{&adf.Link{Href: new("https://example.org")}}

	tests := map[string]struct {
		in   []adf.Node
		want []adf.Node
	}{
		"two adjacent bare text nodes merge": {
			in:   []adf.Node{&adf.Text{Text: "a"}, &adf.Text{Text: "b"}},
			want: []adf.Node{&adf.Text{Text: "ab"}},
		},
		"three adjacent bare text nodes merge into one": {
			in:   []adf.Node{&adf.Text{Text: "a"}, &adf.Text{Text: "b"}, &adf.Text{Text: "c"}},
			want: []adf.Node{&adf.Text{Text: "abc"}},
		},
		"equal mark sets in different order merge": {
			in: []adf.Node{
				&adf.Text{Text: "a", Marks: strongThenEm},
				&adf.Text{Text: "b", Marks: emThenStrong},
			},
			want: []adf.Node{&adf.Text{Text: "ab", Marks: strongThenEm}},
		},
		"plain and bold stay separate": {
			in: []adf.Node{
				&adf.Text{Text: "a"},
				&adf.Text{Text: "b", Marks: []adf.Mark{&adf.Strong{}}},
			},
			want: []adf.Node{
				&adf.Text{Text: "a"},
				&adf.Text{Text: "b", Marks: []adf.Mark{&adf.Strong{}}},
			},
		},
		"same mark kind, different attributes, stays separate": {
			in: []adf.Node{
				&adf.Text{Text: "a", Marks: linkA},
				&adf.Text{Text: "b", Marks: linkB},
			},
			want: []adf.Node{
				&adf.Text{Text: "a", Marks: linkA},
				&adf.Text{Text: "b", Marks: linkB},
			},
		},
		"a non-text node between two same-mark text nodes breaks the run": {
			in: []adf.Node{
				&adf.Text{Text: "a"},
				&adf.HardBreak{},
				&adf.Text{Text: "b"},
			},
			want: []adf.Node{
				&adf.Text{Text: "a"},
				&adf.HardBreak{},
				&adf.Text{Text: "b"},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := mergeAdjacentADFText(tc.in)
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal got: %v", err)
			}
			wantJSON, err := json.Marshal(tc.want)
			if err != nil {
				t.Fatalf("marshal want: %v", err)
			}
			if !bytes.Equal(gotJSON, wantJSON) {
				t.Errorf("mergeAdjacentADFText(%d nodes) = %s, want %s", len(tc.in), gotJSON, wantJSON)
			}
		})
	}
}

// collectTexts walks a document's paragraph content and returns the
// leaf *adf.Text nodes it finds, in order. The fixtures in this file are
// all single-paragraph documents, so this does not need to recurse
// through every node kind.
func collectTexts(t *testing.T, doc adf.Doc) []*adf.Text {
	t.Helper()
	var out []*adf.Text
	for _, block := range doc.Content {
		p, ok := block.(*adf.Paragraph)
		if !ok {
			continue
		}
		for _, n := range p.Content {
			if text, ok := n.(*adf.Text); ok {
				out = append(out, text)
			}
		}
	}
	return out
}
