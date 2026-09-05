// This file is an external test package on purpose: the defect below is
// only visible through the facade, which imports convert, so an internal
// test could not reach it.
package convert_test

import (
	"encoding/json"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// TestFormatKeepsTheTrailingBlankLinesOfAFencedBody: the md → md format
// leg used to strip them, and NormalizeFormat's own doc comment is what
// makes that a defect rather than a style choice — it promises the leg is
// TOTAL ("every node that goes in comes back out") and that "encoding
// either result yields the same ADF". A format that deletes the last two
// lines of a code block breaks both halves of that promise, and it broke
// them silently: FuzzFormatSemanticsPreserved reaches it in seconds.
//
// The cause was a leftover. encodeCoreBlock trimmed *ast.Code's value with
// strings.TrimRight(v.Value, "\n"), which was a NO-OP for as long as the
// parser itself trimmed every trailing newline off the body. Once the
// parser was corrected to keep them (goldmark_to_ast.go's codeBlockValue,
// TrimRight -> TrimSuffix), the same line stopped being harmless and
// started deleting an author's blank lines on every format pass.
//
// Both references agree here, unlike the empty fence: prettier and
// mdast-util-to-markdown write the trailing blank lines back out, and the
// TS reference's markdownToAdf carries them in the ADF codeBlock text. So
// there is no per-mode split — one answer is right for both.
//
// The ADF column is asserted alongside the format column deliberately.
// The ADF leg was ALREADY correct when the format leg was wrong, so a test
// that only checked the format output could be satisfied by a change that
// broke the encode; pinning both is what holds them to the one answer the
// doc comment claims.
func TestFormatKeepsTheTrailingBlankLinesOfAFencedBody(t *testing.T) {
	for name, tc := range map[string]struct {
		src     string
		wantADF string
		wantFmt string
	}{
		// THE FIX: one trailing blank line after content.
		"a trailing blank line": {
			src:     "```json\na\n\n```\n",
			wantADF: "a\n",
			wantFmt: "```json\na\n\n```\n",
		},
		// THE FIX: blank lines on both sides of the content.
		"blank lines around the content": {
			src:     "```json\n\n\na\n\n\n```\n",
			wantADF: "\n\na\n\n",
			wantFmt: "```json\n\n\na\n\n\n```\n",
		},
		// GOOD: no trailing blank line — the shape that always worked, in
		// the same test, so the assertions above cannot be met by a
		// change that simply stops trimming everything everywhere.
		"no trailing blank line": {
			src:     "```json\na\n```\n",
			wantADF: "a",
			wantFmt: "```json\na\n```\n",
		},
		// GOOD: an interior blank line was never at risk, and must stay
		// interior rather than being folded into the tail.
		"an interior blank line": {
			src:     "```json\na\n\nb\n```\n",
			wantADF: "a\n\nb",
			wantFmt: "```json\na\n\nb\n```\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := codeBlockText(t, tc.src); got != tc.wantADF {
				t.Errorf("ADF codeBlock text = %q, want %q", got, tc.wantADF)
			}
			got := adfast.ToMarkdown(adfast.FromMarkdown(tc.src), adfast.WithPrettierFormat())
			if got != tc.wantFmt {
				t.Errorf("format = %q, want %q", got, tc.wantFmt)
			}
			// Totality is a fixpoint claim, so a single pass does not
			// establish it: a leg that trims on pass two is still lossy.
			if again := adfast.ToMarkdown(adfast.FromMarkdown(got), adfast.WithPrettierFormat()); again != got {
				t.Errorf("format is not stable: %q then %q", got, again)
			}
		})
	}
}

// codeBlockText digs the one codeBlock's text out of the encoded ADF, so
// the assertion above reads as the value it is about rather than a JSON
// blob.
func codeBlockText(t *testing.T, src string) string {
	t.Helper()
	raw, err := json.Marshal(adfast.ToADF(adfast.FromMarkdown(src)))
	if err != nil {
		t.Fatalf("marshal ADF: %v", err)
	}
	var doc struct {
		Content []struct {
			Type    string `json:"type"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal ADF: %v", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Type != "codeBlock" {
		t.Fatalf("want one codeBlock, got %s", raw)
	}
	if len(doc.Content[0].Content) == 0 {
		return ""
	}
	return doc.Content[0].Content[0].Text
}
