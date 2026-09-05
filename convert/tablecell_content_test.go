// This file is an external test package on purpose: the defect below is
// visible only through the facade, which composes convert's ADF decode
// with the markdown render, and an internal test could not reach that.
package convert_test

import (
	"encoding/json"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// cellDoc builds a two-column, two-row ADF table whose one interesting
// cell holds cellJSON — the content array as it arrives on the wire — and
// whose neighbor is always the plain-paragraph cell that has worked all
// along. Every assertion below therefore carries its own control: a change
// that empties or mangles cells wholesale cannot satisfy it.
func cellDoc(t *testing.T, cellJSON string) adf.Doc {
	t.Helper()
	src := `{"version":1,"type":"doc","content":[{"type":"table","content":[` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"H"}]}]},` +
		`{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"H2"}]}]}]},` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"ok"}]}]},` +
		`{"type":"tableCell","content":` + cellJSON + `}]}` +
		`]}]}`
	var v any
	if err := json.Unmarshal([]byte(src), &v); err != nil {
		t.Fatalf("test ADF is not valid JSON: %v", err)
	}
	doc, ok := adf.DecodeDoc(v)
	if !ok {
		t.Fatalf("test ADF did not decode: %s", src)
	}
	return doc
}

// cellMarkdown renders the document cellDoc built.
func cellMarkdown(t *testing.T, cellJSON string, opts ...adfast.Option) string {
	t.Helper()
	return adfast.ToMarkdown(adfast.FromADF(cellDoc(t, cellJSON), opts...), opts...)
}

// bodyRow returns the table's third line — the body row carrying the cell
// under test — and fails when the table is not exactly three lines, which
// is the shape a two-row GFM table has and the shape a cell that leaked a
// newline destroys.
func bodyRow(t *testing.T, md string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(md, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("a two-row table must render as 3 lines, got %d: %q", len(lines), md)
	}
	return lines[2]
}

// cellText returns the trimmed text of the body row's second cell.
func cellText(t *testing.T, md string) string {
	t.Helper()
	row := bodyRow(t, md)
	cells := splitRowCells(row)
	if len(cells) != 2 {
		t.Fatalf("the body row must hold exactly 2 cells, got %d in %q", len(cells), row)
	}
	return strings.TrimSpace(cells[1])
}

// splitRowCells splits a rendered row on its UNESCAPED pipes — the only
// pipes a GFM parser reads as column separators. A pipe carrying an odd
// run of backslashes is escaped and belongs to the cell's text; counting
// pipes naively would read "a\|b" as two columns and pass a test that
// should fail.
func splitRowCells(row string) []string {
	var cells []string
	var cur strings.Builder
	backslashes := 0
	for i := range len(row) {
		switch c := row[i]; c {
		case '|':
			if backslashes%2 == 0 {
				cells = append(cells, cur.String())
				cur.Reset()
			} else {
				cur.WriteByte(c)
			}
			backslashes = 0
		case '\\':
			backslashes++
			cur.WriteByte(c)
		default:
			backslashes = 0
			cur.WriteByte(c)
		}
	}
	cells = append(cells, cur.String())
	// A row is "|a|b|", so the split yields an empty lead and an empty
	// trail around the cells.
	if len(cells) < 2 {
		return nil
	}
	return cells[1 : len(cells)-1]
}

// cellShapes are the ADF cell contents the tests below share: every kind
// the tableCell content model admits, plus the nesting and the neighbor
// combinations that the one-line rule has to survive.
var cellShapes = map[string]string{
	// GOOD, everywhere: the shape that has always worked.
	"a paragraph": `[{"type":"paragraph","content":[{"type":"text","text":"plain"}]}]`,

	"two paragraphs":     `[{"type":"paragraph","content":[{"type":"text","text":"one"}]},{"type":"paragraph","content":[{"type":"text","text":"two"}]}]`,
	"a blockquote":       `[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]}]`,
	"a bullet list":      `[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]`,
	"an ordered list":    `[{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]`,
	"a code block":       `[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1\ny := 2"}]}]`,
	"a heading":          `[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]}]`,
	"a panel":            `[{"type":"panel","attrs":{"panelType":"info"},"content":[{"type":"paragraph","content":[{"type":"text","text":"noted"}]}]}]`,
	"a nested expand":    `[{"type":"nestedExpand","attrs":{"title":"T"},"content":[{"type":"paragraph","content":[{"type":"text","text":"hidden"}]}]}]`,
	"a task list":        `[{"type":"taskList","attrs":{"localId":"l"},"content":[{"type":"taskItem","attrs":{"localId":"a","state":"TODO"},"content":[{"type":"text","text":"todo"}]}]}]`,
	"a decision list":    `[{"type":"decisionList","attrs":{"localId":"d"},"content":[{"type":"decisionItem","attrs":{"localId":"i","state":"DECIDED"},"content":[{"type":"text","text":"decided"}]}]}]`,
	"a block card":       `[{"type":"blockCard","attrs":{"url":"https://example.com/x"}}]`,
	"an embed card":      `[{"type":"embedCard","attrs":{"url":"https://example.com/e","layout":"center"}}]`,
	"a media single":     `[{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"type":"external","url":"https://example.com/i.png","alt":"pic"}}]}]`,
	"a media group":      `[{"type":"mediaGroup","content":[{"type":"media","attrs":{"type":"external","url":"https://example.com/i.png","alt":"pic"}}]}]`,
	"a nested table":     `[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","attrs":{"colwidth":[79,320]},"content":[{"type":"paragraph","content":[{"type":"text","text":"inner"}]}]}]}]}]`,
	"a rule":             `[{"type":"rule"}]`,
	"an extension":       `[{"type":"extension","attrs":{"extensionType":"com.x","extensionKey":"k"}}]`,
	"an unsupportedomit": `[{"type":"unsupportedBlock","attrs":{"originalValue":{"type":"weird"}}}]`,

	"a blockquote holding a pipe":      `[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"a|b"}]}]}]`,
	"a code block holding a pipe":      `[{"type":"codeBlock","content":[{"type":"text","text":"a|b\nc"}]}]`,
	"a blockquote around a list":       `[{"type":"blockquote","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"deep"}]}]}]}]}]`,
	"a list around a blockquote":       `[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"deep"}]}]}]}]}]`,
	"a blockquote around a code block": `[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"q"}]},{"type":"codeBlock","content":[{"type":"text","text":"x\ny"}]}]}]`,
	"a marked run in a blockquote":     `[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"bold","marks":[{"type":"strong"}]}]}]}]`,
	"a mention in a blockquote":        `[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"a1","text":"@Ann"}}]}]}]`,
	"a hard break in a paragraph":      `[{"type":"paragraph","content":[{"type":"text","text":"a"},{"type":"hardBreak"},{"type":"text","text":"b"}]}]`,
	"a paragraph then a list":          `[{"type":"paragraph","content":[{"type":"text","text":"lead"}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}]}]`,
	"a rule between two paragraphs":    `[{"type":"paragraph","content":[{"type":"text","text":"one"}]},{"type":"rule"},{"type":"paragraph","content":[{"type":"text","text":"two"}]}]`,
	"an empty paragraph then one":      `[{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"two"}]}]`,
	"a CRLF in a code block":           `[{"type":"codeBlock","content":[{"type":"text","text":"a\r\nb"}]}]`,
}

// TestTableCellKeepsTheContentOfEveryBlockItHolds is the fix.
//
// A table cell holds BLOCKS in ADF, and the decode used to convert its
// grandchildren as inlines — one level down. A paragraph's children are
// inlines, so a paragraph cell survived; a blockquote's, a list's, a
// panel's, a nested table's children are blocks, and the inline visitor's
// hook-or-drop fallback deleted them without a diagnostic. Every cell
// below therefore rendered as "|   |" before the fix: not a formatting
// nit, the text was gone off the wire-ADF path.
//
// The frozen TS reference does not decide the spelling here — its
// adfToMarkdown has no table handling at all and flattens the whole table
// to a block sequence — but it does decide the principle: it renders every
// one of these blocks and drops none of their text. So recovering the
// content is the reference-shaped answer, and only the block MARKERS, which
// no single row can spell, are given up.
func TestTableCellKeepsTheContentOfEveryBlockItHolds(t *testing.T) {
	for name, want := range map[string]string{
		// GOOD, and a PIN: the plain paragraph cell that always worked,
		// asserted in the same test so the fixes below cannot be satisfied
		// by a change that simply writes every cell's plain text. Measured
		// to pass against the old implementation too.
		"a paragraph": "plain",

		// PINS, measured to pass against the old implementation as well —
		// they are here for the neighborhood, not for the repair. A
		// heading's children are already inlines, so the one-level flatten
		// happened to reach them; an empty paragraph contributes nothing,
		// so no separator question arose. Both would regress silently if
		// the projection started treating an inline-childed block or an
		// empty one specially.
		"a heading":                   "Title",
		"an empty paragraph then one": "two",

		// THE FIX: each of these came back empty.
		"a blockquote":                     "quoted",
		"a bullet list":                    "a b",
		"an ordered list":                  "a b",
		"a panel":                          "noted",
		"a nested expand":                  "T hidden",
		"a task list":                      "todo",
		"a decision list":                  "decided",
		"a media group":                    "pic",
		"a nested table":                   "inner",
		"a blockquote around a list":       "deep",
		"a list around a blockquote":       "deep",
		"a marked run in a blockquote":     "**bold**",
		"a mention in a blockquote":        ":mention[Ann]{#a1}",
		"a blockquote around a code block": "q `x y`",
		"a block card":                     `https\://example.com/x`,
		"an embed card":                    `https\://example.com/e`,
		"a media single":                   "![pic](https://example.com/i.png)",

		// THE FIX: a code block's text reached the cell as plain prose,
		// losing the code-ness a row can still spell. An inline span keeps
		// it, and the fence's own lines fold away.
		"a code block": "`x := 1 y := 2`",

		// THE FIX: blocks used to be concatenated with nothing between
		// them, so two paragraphs came out as one run — "onetwo".
		"two paragraphs":                "one two",
		"a paragraph then a list":       "lead a",
		"a rule between two paragraphs": "one two",

		// THE FIX: a hard break rendered as a backslash and a REAL newline
		// inside the row, cutting the three-line table into four lines.
		"a hard break in a paragraph": "a b",

		// THE FIX: the pipes survive escaped, in the cell they belong to.
		"a blockquote holding a pipe": `a\|b`,
		"a code block holding a pipe": "`a\\|b c`",

		// THE FIX: a CRLF is one end of line and folds to one space.
		"a CRLF in a code block": "`a b`",
	} {
		shape, ok := cellShapes[name]
		if !ok {
			t.Fatalf("no cell shape named %q", name)
		}
		t.Run(name, func(t *testing.T) {
			md := cellMarkdown(t, shape)
			if got := cellText(t, md); got != want {
				t.Errorf("cell content = %q, want %q\nfull table:\n%s", got, want, md)
			}
			// The control cell, in the same document: its own content must
			// be untouched by whatever the neighbor recovered.
			if got := strings.TrimSpace(splitRowCells(bodyRow(t, md))[0]); got != "ok" {
				t.Errorf("the plain paragraph neighbor = %q, want %q", got, "ok")
			}
		})
	}
}

// TestTableCellDropsWhatItCannotCarry is a PIN, not a fix test: every
// shape here rendered as an empty cell before the change and still does.
// It is here because the projection now reaches these kinds and could
// start writing them, and each one would break the row if it did.
func TestTableCellDropsWhatItCannotCarry(t *testing.T) {
	for name, why := range map[string]string{
		// A rule has no content at all. Written out it would be the literal
		// text "***", which re-parses as text and carries nothing.
		"a rule": "a rule holds no content",
		// An ADF extension block decodes to ::extension{…}, a LEAF
		// directive: extension.RenderContext's leaf primitive is a no-op in
		// inline position, so there is no one-line form to write and the
		// node carries no text of its own to fall back on.
		"an extension": "a block leaf directive has no inline form",
		// unsupportedBlock drops on decode, in a cell and at the top level
		// alike — measured; it is not a cell defect.
		"an unsupportedomit": "unsupportedBlock drops everywhere, not only in a cell",
	} {
		t.Run(name, func(t *testing.T) {
			md := cellMarkdown(t, cellShapes[name])
			if got := cellText(t, md); got != "" {
				t.Errorf("cell content = %q, want empty (%s)\nfull table:\n%s", got, why, md)
			}
			if got := strings.TrimSpace(splitRowCells(bodyRow(t, md))[0]); got != "ok" {
				t.Errorf("the plain paragraph neighbor = %q, want %q", got, "ok")
			}
		})
	}
}

// TestTableCellDropsANestedTablesColwidthCompanion is a fix test for the
// one label in the dialect that is a payload rather than prose.
//
// convertAdfBlocks emits ::colwidths ahead of a table, carrying the
// following table's column widths in its directive LABEL. Flattening that
// label the way every other block's content is flattened put the machine
// string "79,320" into the middle of the row — and the table it describes
// has just been dissolved into that same row, so the widths describe
// nothing any more.
func TestTableCellDropsANestedTablesColwidthCompanion(t *testing.T) {
	md := cellMarkdown(t, cellShapes["a nested table"])
	if got := cellText(t, md); strings.Contains(got, "79,320") {
		t.Errorf("the colwidth payload leaked into the cell: %q\nfull table:\n%s", got, md)
	}
	// GOOD: the nested table's own text is still recovered, so the drop is
	// scoped to the companion rather than to the whole nested table.
	if got := cellText(t, md); got != "inner" {
		t.Errorf("cell content = %q, want %q\nfull table:\n%s", got, "inner", md)
	}
}

// TestTableCellStaysOneRow is the invariant every recovered kind owes the
// table: a GFM row is ONE line, and it ends at the first unescaped pipe
// boundary. A cell that leaks a newline turns the three-line table into
// four and the tail re-parses as a paragraph; a cell that leaks a bare
// pipe grows the row a column. Both are worse than the empty cell this
// change was written to repair, so the rule is checked for EVERY shape
// rather than for the two kinds that motivated it.
//
// Measured against the old implementation, one shape fails here — a hard
// break, which used to write a backslash and a real newline into the row.
// The rest are PINS: they were empty cells, which are one line by
// accident. They stay because the repair is what puts content at risk.
func TestTableCellStaysOneRow(t *testing.T) {
	for name, shape := range cellShapes {
		t.Run(name, func(t *testing.T) {
			md := cellMarkdown(t, shape)
			// bodyRow fails the test when the table is not 3 lines.
			row := bodyRow(t, md)
			if cells := splitRowCells(row); len(cells) != 2 {
				t.Errorf("row = %q split into %d cells, want 2 (an unescaped pipe leaked)", row, len(cells))
			}
			if strings.ContainsAny(md, "\r") {
				t.Errorf("a carriage return survived into the table: %q", md)
			}
		})
	}
}

// htmlBlockKind is the ADF node type the decode hook below claims.
const htmlBlockKind = "x.rawHTML"

// hookHTML is the extension kind that makes the raw-HTML branch of the
// cell projection reachable: no ADF kind decodes to ast.HTML on its own,
// but a consumer's DecodeBlock hook may return one, and raw HTML is the
// one recovered value the renderer writes VERBATIM — so it has to arrive
// already fit for a row.
type hookHTML struct{ children []ast.Node }

func (*hookHTML) Kind() string                           { return htmlBlockKind }
func (n *hookHTML) ChildNodes() []ast.Node               { return n.children }
func (n *hookHTML) SetChildNodes(kids []ast.Node)        { n.children = kids }
func (*hookHTML) MarkdownLead() byte                     { return ':' }
func (*hookHTML) RenderMarkdown(extension.RenderContext) {}

func (*hookHTML) EncodeADF(extension.EncodeContext) []adf.Node { return nil }

// rawHTMLRegistration decodes the x.rawHTML ADF node into an ast.HTML
// carrying the node's "value" attribute.
func rawHTMLRegistration() extension.Registration {
	return extension.Registration{
		Kind: "rawhtml",
		Texts: map[string]func(*ast.TextDirective) extension.Node{
			"rawhtml": func(d *ast.TextDirective) extension.Node {
				return &hookHTML{children: d.Children}
			},
		},
		DecodeBlock: func(n adf.Node, _ extension.DecodeContext) (ast.Node, bool) {
			raw, ok := n.(*adf.RawNode)
			if !ok || raw.Type != htmlBlockKind {
				return nil, false
			}
			value, isString := raw.Attrs["value"].(string)
			if !isString {
				return nil, false
			}
			return &ast.HTML{Value: value}, true
		},
	}
}

// TestTableCellMakesHookSuppliedRawHTMLFitTheRow is a fix test on the one
// value the renderer does not interpret.
//
// Before the change the hook never fired for a cell at all — the decode
// looked at the cell's grandchildren, so a block hook's node was dropped
// with everything else. Now that the value reaches the row, it has to
// arrive safe: an end of line in it ends the row, and a bare pipe in it
// opens a column, neither of which the inline writer fixes for a value it
// writes verbatim.
func TestTableCellMakesHookSuppliedRawHTMLFitTheRow(t *testing.T) {
	for name, tc := range map[string]struct{ value, want string }{
		// The control: a value that already fits must ride through
		// untouched, so the folding and the escaping below are targeted
		// rather than a blanket rewrite. All four cases fail against the
		// old implementation, which never called the hook for a cell.
		"a value that already fits": {value: "<b>x</b>", want: "<b>x</b>"},

		// THE FIX: the newlines fold to spaces so the row stays one line.
		"a value spanning lines": {value: "<div>\nA\n</div>", want: "<div> A </div>"},
		// THE FIX: the bare pipe is escaped so the row keeps its columns.
		"a value holding a pipe": {value: "<b>a|b</b>", want: `<b>a\|b</b>`},
		// GOOD: a pipe that already carries its backslash is left alone
		// rather than doubled.
		"a value holding an escaped pipe": {value: `<b>a\|b</b>`, want: `<b>a\|b</b>`},
	} {
		t.Run(name, func(t *testing.T) {
			shape := `[{"type":"` + htmlBlockKind + `","attrs":{"value":` + mustJSON(t, tc.value) + `}}]`
			md := cellMarkdown(t, shape, adfast.WithExtensions(rawHTMLRegistration()))
			if got := cellText(t, md); got != tc.want {
				t.Errorf("cell content = %q, want %q\nfull table:\n%s", got, tc.want, md)
			}
		})
	}
}

// mustJSON quotes a string as a JSON literal.
func mustJSON(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal %q: %v", s, err)
	}
	return string(b)
}
