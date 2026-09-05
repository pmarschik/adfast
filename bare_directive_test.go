package adfast_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/pmarschik/adfast"
)

// A dialect directive NAME written alone in prose is not a directive.
//
// ":media", ":status" and ":date" are directive names, but ":name" is
// also just a colon in the middle of a sentence, and goldmark-directive
// reads one as the other. Promoting it was damage in two shapes, both
// silent:
//
//   - "see :status in the log" formatted to "see  in the log". The kind
//     carries its payload in a label or an attribute, so a payload-less
//     one had nothing to encode and the node was dropped — on the ADF
//     leg and, because the formatter canonicalizes through the same
//     re-derivation, on the md→md leg too. A formatter deleted a word.
//
//   - "see :media in the log" round-tripped byte for byte, so no diff
//     showed anything, while the pushed ADF carried a mediaInline with
//     no id and no collection: a media reference addressing nothing.
//
// The rule that fixes both is a property, not a list of names: a text
// directive that carries neither a label nor an attribute block is the
// text ":name". Writing ANY payload makes the name a directive again,
// which is what the GOOD rows below hold in place — the rule cannot be
// satisfied by breaking the real directives.
func TestBareDialectNameIsProse(t *testing.T) {
	t.Parallel()
	for _, tt := range bareNameCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := adfast.ToMarkdown(adfast.FromMarkdown(tt.md), adfast.WithPrettierFormat()); got != tt.wantFmt {
				t.Errorf("format = %q, want %q", got, tt.wantFmt)
			}
			if got := inlineShape(t, tt.md); got != tt.wantADF {
				t.Errorf("adf = %q, want %q", got, tt.wantADF)
			}
		})
	}
}

// bareNameCases is the table TestBareDialectNameIsProse runs. The FIX
// rows are the defect; the GOOD rows are what the rule must not touch,
// and they sit here rather than in a test of their own so a change that
// buys one at the cost of the other cannot pass.
var bareNameCases = []struct {
	name    string
	md      string
	wantFmt string
	wantADF string
}{
	// ----- FIX: a bare name is the word the author wrote -----
	{
		name:    "fix: media in prose",
		md:      "see :media in the log",
		wantFmt: "see :media in the log\n",
		wantADF: "text(see )|text(:media)|text( in the log)",
	},
	{
		name:    "fix: status in prose",
		md:      "see :status in the log",
		wantFmt: "see :status in the log\n",
		wantADF: "text(see )|text(:status)|text( in the log)",
	},
	{
		name:    "fix: date in prose",
		md:      "see :date in the log",
		wantFmt: "see :date in the log\n",
		wantADF: "text(see )|text(:date)|text( in the log)",
	},
	{
		name:    "fix: mention in prose",
		md:      "see :mention in the log",
		wantFmt: "see :mention in the log\n",
		wantADF: "text(see )|text(:mention)|text( in the log)",
	},
	{
		name:    "fix: emoji in prose",
		md:      "see :emoji in the log",
		wantFmt: "see :emoji in the log\n",
		wantADF: "text(see )|text(:emoji)|text( in the log)",
	},
	{
		// The intraword colon PlainTextOf's doc comment names as the
		// hazard: no space in front of the name at all.
		name:    "fix: intraword colon",
		md:      "deploy:status is set",
		wantFmt: "deploy:status is set\n",
		wantADF: "text(deploy)|text(:status)|text( is set)",
	},
	{
		// An image alt is read with ast.PlainText, which had no text
		// for the promoted node, so the alt truncated to "Over".
		name:    "fix: name inside an image alt",
		md:      "![Over:media](x.png)",
		wantFmt: "![Over:media](x.png)\n",
		wantADF: "[link]text(Over:media)",
	},
	{
		name:    "fix: name in a heading",
		md:      "# :status heading",
		wantFmt: "# :status heading\n",
		wantADF: "text(:status)|text( heading)",
	},
	{
		name:    "fix: name in a table cell",
		md:      "| :media |\n| --- |\n| x |",
		wantFmt: "| :media |\n| ------ |\n| x      |\n",
		wantADF: "text(:media)|text(x)",
	},

	// ----- GOOD: unchanged by the rule -----
	{
		// A name the dialect does not know already degraded to text.
		// The fix makes a known name behave exactly like this one —
		// the two ADF projections are identical.
		name:    "good: unknown name",
		md:      "see :foo in the log",
		wantFmt: "see :foo in the log\n",
		wantADF: "text(see )|text(:foo)|text( in the log)",
	},
	{
		name:    "good: escaped colon",
		md:      "see \\:media in the log",
		wantFmt: "see \\:media in the log\n",
		wantADF: "text(see :media in the log)",
	},
	{
		// A trailing colon ends the name, so this was never a
		// directive; the formatter adds the escape that keeps it
		// from becoming one.
		name:    "good: trailing colon",
		md:      "see :media: in the log",
		wantFmt: "see \\:media: in the log\n",
		wantADF: "text(see :media: in the log)",
	},
	{
		name:    "good: code span",
		md:      "see `:media` in the log",
		wantFmt: "see `:media` in the log\n",
		wantADF: "text(see )|[code]text(:media)|text( in the log)",
	},
	{
		// The attributed forms are the ones the rule must NOT reach:
		// a payload is exactly what tells a directive from a word.
		name:    "good: media with a source",
		md:      "see :media{id=1 collection=c} in the log",
		wantFmt: "see :media{#1 collection=\"c\"} in the log\n",
		wantADF: "text(see )|mediaInline|text( in the log)",
	},
	{
		name:    "good: status with a label and a color",
		md:      "see :status[Done]{color=green} in the log",
		wantFmt: "see :status[Done]{color=\"green\"} in the log\n",
		wantADF: "text(see )|status|text( in the log)",
	},
	{
		// A label alone is payload too.
		name:    "good: status with a label only",
		md:      "see :status[Done] in the log",
		wantFmt: "see :status[Done]{color=\"neutral\"} in the log\n",
		wantADF: "text(see )|status|text( in the log)",
	},
	{
		// And an attribute block alone is payload too, so the rule
		// is not "must have a label".
		name:    "good: date with a timestamp only",
		md:      "see :date{timestamp=1752537600000} in the log",
		wantFmt: "see :date[2025-07-15]{timestamp=\"1752537600000\"} in the log\n",
		wantADF: "text(see )|date|text( in the log)",
	},
}

// The word is not lost anywhere else either: the plaintext projection and
// the block directive forms read the same before and after the rule.
func TestBareDialectNameOnlyReachesTheTextForm(t *testing.T) {
	t.Parallel()
	// A leaf form needs its own line and a doubled colon, so an author
	// writes one on purpose — it is not the prose collision, and it is
	// left alone whether or not it carries attributes.
	if got := inlineShape(t, "::media[alt]{id=x}"); got != "media" {
		t.Errorf("leaf media = %q", got)
	}
	if got := inlineShape(t, ":::info\nx\n:::"); got != "text(x)" {
		t.Errorf("container panel = %q", got)
	}
	// The label of a real directive is prose too, and a name inside it
	// is the word, not a nested directive.
	if got := adfast.ToMarkdown(adfast.FromMarkdown(":status[a :media b]"), adfast.WithPrettierFormat()); got != ":status[a \\:media b]{color=\"neutral\"}\n" {
		t.Errorf("label = %q", got)
	}
}

// inlineShape projects a document's leaf nodes to a compact string: the
// node type, the text it carries, and the marks on it. It is enough to
// tell "the word survived as text" from "a chip was built out of it",
// which is the whole subject here.
func inlineShape(t *testing.T, md string) string {
	t.Helper()
	raw, err := json.Marshal(adfast.ToADF(adfast.FromMarkdown(md)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Content []shapeNode `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return strings.Join(shapeOf(nil, doc.Content), "|")
}

// shapeNode is the slice of the ADF wire shape inlineShape reads.
type shapeNode struct {
	Type    string      `json:"type"`
	Text    string      `json:"text"`
	Marks   []shapeMark `json:"marks"`
	Content []shapeNode `json:"content"`
}

type shapeMark struct {
	Type string `json:"type"`
}

// shapeOf appends one part per LEAF node, descending through anything
// that has content of its own.
func shapeOf(parts []string, nodes []shapeNode) []string {
	for _, n := range nodes {
		if len(n.Content) > 0 {
			parts = shapeOf(parts, n.Content)
			continue
		}
		part := n.Type
		if n.Text != "" {
			part += "(" + n.Text + ")"
		}
		if len(n.Marks) > 0 {
			marks := make([]string, 0, len(n.Marks))
			for _, m := range n.Marks {
				marks = append(marks, m.Type)
			}
			sort.Strings(marks)
			part = "[" + strings.Join(marks, ",") + "]" + part
		}
		parts = append(parts, part)
	}
	return parts
}
