package adfast

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func doc(content ...adf.Node) adf.Doc {
	return adf.Doc{Type: "doc", Version: 1, Content: content}
}

func p(content ...adf.Node) adf.Node {
	return &adf.Paragraph{Content: content}
}

func txt(value string, marks ...adf.Mark) adf.Node {
	if len(marks) == 0 {
		return &adf.Text{Text: value}
	}
	return &adf.Text{Text: value, Marks: marks}
}

func li(content ...adf.Node) adf.Node {
	return &adf.ListItem{Content: content}
}

func link(href string) *adf.Link {
	return &adf.Link{Href: &href}
}

// wantContains fails when got is missing any of the fragments.
func wantContains(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, s := range want {
		if !strings.Contains(got, s) {
			t.Errorf("output %q does not contain %q", got, s)
		}
	}
}

// ---------------------------------------------------------------------------
// ADF → MD: Block types
// ---------------------------------------------------------------------------

func TestAdfToMarkdown_BlockTypes(t *testing.T) {
	for _, tt := range blockTypeCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := adfToMD(tt.input, WithSmartLinks(jiraTestSmartLinks))
			if tt.exact != "" && got != tt.exact {
				t.Errorf("got %q, want %q", got, tt.exact)
			}
			wantContains(t, got, tt.contains...)
		})
	}
}

// blockTypeCase is one ADF block kind's markdown expectation: exact pins
// the whole render, contains pins fragments of it.
type blockTypeCase struct {
	name     string
	exact    string
	contains []string
	input    adf.Doc
}

// blockTypeCases is the table TestAdfToMarkdown_BlockTypes runs. It lives
// outside the test body so the body stays the runner alone.
func blockTypeCases() []blockTypeCase {
	return []blockTypeCase{
		{
			name:  "renders a paragraph",
			input: doc(p(txt("Hello world"))),
			exact: "Hello world\n",
		},
		{
			name:  "renders a horizontal rule",
			input: doc(&adf.Rule{}),
			exact: "---\n",
		},
		{
			name:  "renders a blockquote",
			input: doc(&adf.Blockquote{Content: []adf.Node{p(txt("quoted"))}}),
			exact: "> quoted\n",
		},
		{
			name:  "renders a code block without language",
			input: doc(&adf.CodeBlock{Content: []adf.Node{txt("const x = 1;")}}),
			exact: "```\nconst x = 1;\n```\n",
		},
		{
			name: "renders a code block with language",
			input: doc(&adf.CodeBlock{
				Language: "typescript",
				Content:  []adf.Node{txt("const x = 1;")},
			}),
			exact: "```typescript\nconst x = 1;\n```\n",
		},
		{
			name: "renders a bullet list",
			input: doc(&adf.BulletList{
				Content: []adf.Node{
					li(p(txt("one"))),
					li(p(txt("two"))),
				},
			}),
			exact: "- one\n- two\n",
		},
		{
			name: "renders a nested bullet list",
			input: doc(&adf.BulletList{
				Content: []adf.Node{
					li(
						p(txt("parent")),
						&adf.BulletList{Content: []adf.Node{
							li(p(txt("child"))),
						}},
					),
				},
			}),
			contains: []string{"- parent", "  - child"},
		},
		{
			name: "renders an ordered list",
			input: doc(&adf.OrderedList{
				Content: []adf.Node{
					li(p(txt("first"))),
					li(p(txt("second"))),
				},
			}),
			contains: []string{"1. first", "1. second"},
		},
		{
			name: "renders a task list",
			input: doc(&adf.TaskList{
				Content: []adf.Node{
					&adf.TaskItem{State: "TODO", Content: []adf.Node{txt("pending")}},
					&adf.TaskItem{State: "DONE", Content: []adf.Node{txt("done")}},
				},
			}),
			contains: []string{"- [ ] pending", "- [x] done"},
		},
		{
			name: "renders a decision list as ::decisions plus a bullet list",
			input: doc(&adf.DecisionList{
				Content: []adf.Node{&adf.DecisionItem{Content: []adf.Node{txt("decided")}}},
			}),
			contains: []string{"::decisions\n\n- decided"},
		},
		{
			name: "renders mediaSingle as a ::media directive",
			input: doc(&adf.MediaSingle{
				Layout: new("align-start"), Width: new(float64(686)), WidthType: new("pixel"),
				Content: []adf.Node{&adf.Media{
					Type: "file", ID: "abc", Alt: "shot.png",
					Collection: new(""), Width: new(float64(2308)), Height: new(float64(551)),
				}},
			}),
			contains: []string{`::media[shot.png]{#abc height="551" layoutWidth="686" width="2308" widthType="pixel"}`},
		},
	}
}

func TestAdfToMarkdown_Headings(t *testing.T) {
	for level := 1; level <= 6; level++ {
		md := adfToMD(doc(&adf.Heading{
			Level:   level,
			Content: []adf.Node{txt("H" + string(rune('0'+level)))},
		}))
		prefix := strings.Repeat("#", level)
		expected := prefix + " H" + string(rune('0'+level)) + "\n"
		if md != expected {
			t.Errorf("level %d: got %q, want %q", level, md, expected)
		}
	}
}

func TestAdfToMarkdown_PanelTypes(t *testing.T) {
	for _, panelType := range []string{"info", "note", "warning", "error", "success"} {
		md := adfToMD(doc(&adf.Panel{
			PanelType: panelType,
			Content:   []adf.Node{p(txt("content"))},
		}))
		expected := ":::" + panelType + "\ncontent\n:::\n"
		if md != expected {
			t.Errorf("panelType %s: got %q, want %q", panelType, md, expected)
		}
	}
}

func TestAdfToMarkdown_EmptyPanel(t *testing.T) {
	md := adfToMD(doc(&adf.Panel{
		PanelType: "info",
		Content:   []adf.Node{},
	}))
	trimmed := strings.TrimSpace(md)
	if trimmed != ":::info\n:::" {
		t.Errorf("got %q, want %q", trimmed, ":::info\n:::")
	}
}

func TestAdfToMarkdown_Table(t *testing.T) {
	md := adfToMD(doc(&adf.Table{
		Content: []adf.Node{
			&adf.TableRow{Content: []adf.Node{
				&adf.TableHeader{Content: []adf.Node{p(txt("A"))}},
				&adf.TableHeader{Content: []adf.Node{p(txt("B"))}},
			}},
			&adf.TableRow{Content: []adf.Node{
				&adf.TableCell{Content: []adf.Node{p(txt("1"))}},
				&adf.TableCell{Content: []adf.Node{p(txt("2"))}},
			}},
		},
	}))
	if !strings.Contains(md, "| A | B |") {
		t.Errorf("missing header: %q", md)
	}
	if !strings.Contains(md, "| 1 | 2 |") {
		t.Errorf("missing data row: %q", md)
	}
	if !strings.Contains(md, "| - | - |") {
		t.Errorf("missing separator: %q", md)
	}
}

func TestAdfToMarkdown_TableWithCode(t *testing.T) {
	md := adfToMD(doc(&adf.Table{
		Content: []adf.Node{
			&adf.TableRow{Content: []adf.Node{
				&adf.TableHeader{Content: []adf.Node{p(txt("Method"))}},
				&adf.TableHeader{Content: []adf.Node{p(txt("Return"))}},
			}},
			&adf.TableRow{Content: []adf.Node{
				&adf.TableCell{Content: []adf.Node{p(txt("foo()", &adf.Code{}))}},
				&adf.TableCell{Content: []adf.Node{p(txt("string"))}},
			}},
		},
	}))
	if !strings.Contains(md, "`foo()`") {
		t.Errorf("missing code in table: %q", md)
	}
}

// ---------------------------------------------------------------------------
// ADF → MD: Inline types
// ---------------------------------------------------------------------------

// jiraTestSmartLinks replicates the jira addon's URL scheme for core
// tests: the fixture corpus was measured (against the remark reference
// implementation) with Jira link conventions configured.
var jiraTestSmartLinks = convert.SmartLinks{
	KeyFromURL: func(url string) (string, bool) {
		if m := jiraTestBrowseRe.FindStringSubmatch(url); m != nil {
			return m[1], true
		}
		return "", false
	},
}

var jiraTestBrowseRe = regexp.MustCompile(`/browse/([A-Z][A-Z0-9]+-\d+)\b`)

func TestAdfToMarkdown_InlineTypes(t *testing.T) {
	tests := []struct {
		name     string
		exact    string
		contains []string
		input    adf.Doc
	}{
		{
			name:  "plain text",
			input: doc(p(txt("hello"))),
			exact: "hello\n",
		},
		{
			name:  "strong text",
			input: doc(p(txt("bold", &adf.Strong{}))),
			exact: "**bold**\n",
		},
		{
			name:  "emphasized text",
			input: doc(p(txt("italic", &adf.Em{}))),
			exact: "_italic_\n",
		},
		{
			name:  "strikethrough text",
			input: doc(p(txt("struck", &adf.Strike{}))),
			exact: "~~struck~~\n",
		},
		{
			name:  "inline code",
			input: doc(p(txt("code", &adf.Code{}))),
			exact: "`code`\n",
		},
		{
			name:  "link",
			input: doc(p(txt("click", link("https://example.com")))),
			exact: "[click](https://example.com)\n",
		},
		{
			name:     "hardBreak",
			input:    doc(p(txt("before"), &adf.HardBreak{}, txt("after"))),
			contains: []string{"before", "after"},
		},
		{
			name:     "emoji",
			input:    doc(p(&adf.Emoji{Text: new("👍"), ShortName: ":thumbsup:"})),
			contains: []string{"👍"},
		},
		{
			name:     "mention",
			input:    doc(p(&adf.Mention{Text: new("@alice"), ID: "123"})),
			contains: []string{":mention[alice]{#123}"},
		},
		{
			name:     "inlineCard with Jira URL",
			input:    doc(p(&adf.InlineCard{URL: new("https://ixolit.atlassian.net/browse/PROJ-42")})),
			contains: []string{"[PROJ-42](https://ixolit.atlassian.net/browse/PROJ-42)"},
		},
		{
			name:     "inlineCard with non-Jira URL",
			input:    doc(p(&adf.InlineCard{URL: new("https://example.com/page")})),
			contains: []string{"https://example.com/page"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adfToMD(tt.input, WithSmartLinks(jiraTestSmartLinks))
			if tt.exact != "" && got != tt.exact {
				t.Errorf("got %q, want %q", got, tt.exact)
			}
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("output %q does not contain %q", got, s)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ADF → MD: adf.Mark combinations
// ---------------------------------------------------------------------------

func TestAdfToMarkdown_MarkCombinations(t *testing.T) {
	t.Run("strong + em", func(t *testing.T) {
		md := adfToMD(doc(p(txt("bolditalic", &adf.Strong{}, &adf.Em{}))))
		if !strings.Contains(md, "**_bolditalic_**") {
			t.Errorf("got %q, want containing **_bolditalic_**", md)
		}
	})

	t.Run("strong + code (bold wrapping code inference)", func(t *testing.T) {
		md := adfToMD(doc(p(
			txt("AC3: ", &adf.Strong{}),
			txt("GET /healthz", &adf.Code{}),
			txt(" Endpoint"),
		)))
		if !strings.Contains(md, "**AC3: `GET /healthz` Endpoint**") {
			t.Errorf("got %q, want containing **AC3: `GET /healthz` Endpoint**", md)
		}
	})

	t.Run("strong + link", func(t *testing.T) {
		md := adfToMD(doc(p(txt("click",
			&adf.Strong{},
			link("https://example.com"),
		))))
		if !strings.Contains(md, "**[click](https://example.com)**") {
			t.Errorf("got %q, want containing **[click](https://example.com)**", md)
		}
	})

	t.Run("em + code", func(t *testing.T) {
		md := adfToMD(doc(p(
			txt("pre ", &adf.Em{}),
			txt("code", &adf.Code{}),
			txt(" post"),
		)))
		if !strings.Contains(md, "_pre `code` post_") {
			t.Errorf("got %q, want containing _pre `code` post_", md)
		}
	})

	t.Run("escapes backticks in plain text", func(t *testing.T) {
		md := adfToMD(doc(p(txt("literal `something` here"))))
		if !strings.Contains(md, "literal \\`something\\` here") {
			t.Errorf("got %q, want containing escaped backticks", md)
		}
	})

	t.Run("escapes underscores in plain text", func(t *testing.T) {
		md := adfToMD(doc(p(txt("bindb_simplified"))))
		if !strings.Contains(md, `bindb\_simplified`) {
			t.Errorf("got %q, want containing bindb\\_simplified", md)
		}
	})

	t.Run("escapes asterisks in plain text", func(t *testing.T) {
		md := adfToMD(doc(p(txt("a*b*c"))))
		if !strings.Contains(md, `a\*b\*c`) {
			t.Errorf("got %q, want containing a\\*b\\*c", md)
		}
	})

	t.Run("code span containing backticks", func(t *testing.T) {
		md := adfToMD(doc(p(txt("a`b", &adf.Code{}))))
		if !strings.Contains(md, "``a`b``") {
			t.Errorf("got %q, want containing ``a`b``", md)
		}
	})
}

// ---------------------------------------------------------------------------
// MD → ADF: Round-trip tests
// ---------------------------------------------------------------------------

func TestRoundTrip(t *testing.T) {
	for _, tt := range roundTripCases() {
		t.Run(tt.name, func(t *testing.T) {
			roundTrip := adfToMD(mdToADF(tt.input))
			for _, s := range tt.contains {
				if !strings.Contains(roundTrip, s) {
					t.Errorf("round-trip of %q → %q does not contain %q", tt.input, roundTrip, s)
				}
			}
			for _, s := range tt.notContain {
				if strings.Contains(roundTrip, s) {
					t.Errorf("round-trip of %q → %q should not contain %q", tt.input, roundTrip, s)
				}
			}
		})
	}
}

// roundTripCase pins what one markdown input must (and must not) still
// contain after a trip through ADF and back.
type roundTripCase struct {
	name       string
	input      string
	contains   []string
	notContain []string
}

// roundTripCases is the table TestRoundTrip runs; see blockTypeCases for
// why it sits outside the test body.
func roundTripCases() []roundTripCase {
	return []roundTripCase{
		{
			name:       "strips leading YAML frontmatter",
			input:      "---\nstatus: Ready\n---\n\n# Title",
			contains:   []string{"# Title"},
			notContain: []string{"status: Ready"},
		},
		{
			name:     "collapses soft line breaks",
			input:    "Hello\nworld",
			contains: []string{"Hello world"},
		},
		{
			name:     "preserves newlines in code blocks",
			input:    "```php\nline1\nline2\n```",
			contains: []string{"```php", "line1\nline2"},
		},
		{
			name:     "round-trips headings",
			input:    "# Title\n\n## Subtitle",
			contains: []string{"# Title", "## Subtitle"},
		},
		{
			name:     "round-trips horizontal rule",
			input:    "before\n\n---\n\nafter",
			contains: []string{"---"},
		},
		{
			name:     "round-trips blockquote",
			input:    "> quoted text",
			contains: []string{"> quoted text"},
		},
		{
			name:     "round-trips bullet list",
			input:    "- one\n- two\n- three",
			contains: []string{"- one", "- two", "- three"},
		},
		{
			name:     "round-trips ordered list",
			input:    "1. first\n2. second",
			contains: []string{"1. first", "1. second"},
		},
		{
			name:     "round-trips bold text",
			input:    "some **bold** text",
			contains: []string{"**bold**"},
		},
		{
			name:     "round-trips italic text",
			input:    "some *italic* text",
			contains: []string{"_italic_"},
		},
		{
			name:     "round-trips inline code",
			input:    "use `foo()` here",
			contains: []string{"`foo()`"},
		},
		{
			name:     "round-trips links",
			input:    "click [here](https://example.com) please",
			contains: []string{"[here](https://example.com)"},
		},
		{
			name:     "round-trips bold wrapping code",
			input:    "- **AC3: `GET /healthz` Endpoint is Defined**",
			contains: []string{"**AC3: `GET /healthz` Endpoint is Defined**"},
		},
		{
			name:       "keeps bold separate from code",
			input:      "**bold** `code` normal",
			contains:   []string{"**bold**", "`code`"},
			notContain: []string{"**bold** **"},
		},
		{
			name:     "round-trips directive panels",
			input:    ":::note\nBe careful\n:::",
			contains: []string{":::note", "Be careful", ":::"},
		},
		{
			name:     "round-trips table",
			input:    "| A | B |\n| --- | --- |\n| 1 | 2 |",
			contains: []string{"| A | B |", "| 1 | 2 |"},
		},
	}
}

func TestRoundTrip_NoNewlinesInAdfTextNodes(t *testing.T) {
	adfDoc := mdToADF("The context includes transaction information when available (enough to\nsupport cutoff-date safeguards without additional call-site changes).")
	js, marshalErr := json.Marshal(adfDoc)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(js), "\\n") {
		t.Errorf("ADF text nodes should not contain literal newlines: %s", js)
	}
}

// ---------------------------------------------------------------------------

func TestNormalizeTextNewlines(t *testing.T) {
	adfDoc := mdToADF("The context includes transaction information when available (enough to\nsupport cutoff-date safeguards without additional call-site changes).")
	walkTexts(adfDoc.Content, func(text string) {
		if strings.Contains(text, "\n") {
			t.Errorf("text node should not contain newline: %q", text)
		}
	})
}

// A space run that spans two adjacent same-mark text nodes collapses
// too: markdown writes those nodes contiguously, so the run only exists
// in the ADF and re-parsing the render would shorten it. The junctions
// come from inline nodes that convert to nothing — an empty link, or a
// registered text directive with no content.
func TestNormalizeTextNewlines_AcrossTextNodes(t *testing.T) {
	tests := []struct {
		name, md, want string
	}{
		{"dropped empty link", "x []() y", "x |y"},
		{"dropped empty directive", "*0aaa[0 :u ]*", "0aaa[0 |]"},
		{"run of dropped constructs", "x []() []() y", "x |y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got strings.Builder
			walkTexts(mdToADF(tt.md).Content, func(text string) {
				if got.Len() > 0 {
					got.WriteString("|")
				}
				got.WriteString(text)
			})
			if got.String() != tt.want {
				t.Errorf("text nodes = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

// Marks part the run: two text nodes with different marks render with a
// mark delimiter between them, so both spaces survive the re-parse.
func TestNormalizeTextNewlines_KeepsRunAcrossMarks(t *testing.T) {
	var got strings.Builder
	walkTexts(mdToADF("[x ](https://example.com) y").Content, func(text string) {
		got.WriteString("|")
		got.WriteString(text)
	})
	if want := "|x | y"; got.String() != want {
		t.Errorf("text nodes = %q, want %q", got.String(), want)
	}
}

func walkTexts(nodes []adf.Node, fn func(string)) {
	for _, n := range nodes {
		if text, ok := n.(*adf.Text); ok && text.Text != "" {
			fn(text.Text)
		}
		if content := adf.NodeContent(n); len(content) > 0 {
			walkTexts(content, fn)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestMarkdownToAdf_PanelLabel(t *testing.T) {
	t.Run("label becomes a plain first paragraph (remark behavior)", func(t *testing.T) {
		adfDoc := mdToADF(":::info[Important Note]\nContent here\n:::")
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		s := string(js)
		if !strings.Contains(s, `"type":"panel"`) {
			t.Fatalf("expected panel node in %s", s)
		}
		if !strings.Contains(s, `"text":"Important Note"`) {
			t.Errorf("expected label text in %s", s)
		}
		// remark-directive represents the label as a plain paragraph child,
		// not a bold one (a bold-label branch would be dead code).
		if strings.Contains(s, `"type":"strong"`) {
			t.Errorf("label must not be bold: %s", s)
		}
		if len(adfDoc.Content) != 1 || len(adf.NodeContent(adfDoc.Content[0])) != 2 {
			t.Errorf("expected panel with label + content paragraphs: %s", s)
		}
	})

	t.Run("no label paragraph when directive has no label", func(t *testing.T) {
		adfDoc := mdToADF(":::info\nContent here\n:::")
		js, marshalErr := json.Marshal(adfDoc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		s := string(js)
		if !strings.Contains(s, `"type":"panel"`) {
			t.Fatalf("expected panel node in %s", s)
		}
		if strings.Contains(s, `"type":"strong"`) {
			t.Errorf("should not have strong mark without label: %s", s)
		}
	})
}

// The edge cases split by concern: each group is a top-level test so no
// single body has to carry all of them.

func TestEdgeCases_EmptyInput(t *testing.T) {
	t.Run("handles empty doc", func(t *testing.T) {
		md := adfToMD(doc())
		if md != "\n" {
			t.Errorf("got %q, want %q", md, "\n")
		}
	})

	t.Run("handles nil input", func(t *testing.T) {
		md := adfToMD(nil)
		if md != "" {
			t.Errorf("got %q, want empty string", md)
		}
	})

	t.Run("handles empty markdownToAdf input", func(t *testing.T) {
		wantEmptyParagraphDoc(t, mdToADF(""))
	})

	t.Run("handles whitespace-only markdownToAdf input", func(t *testing.T) {
		wantEmptyParagraphDoc(t, mdToADF("   \n  \n  "))
	})
}

// wantEmptyParagraphDoc pins the shape markdown with no content converts
// to: a version 1 doc holding one empty paragraph.
func wantEmptyParagraphDoc(t *testing.T, adfDoc adf.Doc) {
	t.Helper()
	if adfDoc.Type != "doc" || adfDoc.Version != 1 {
		t.Errorf("unexpected doc: %+v", adfDoc)
	}
	if len(adfDoc.Content) != 1 || adfDoc.Content[0].Kind() != "paragraph" {
		t.Errorf("expected single empty paragraph: %+v", adfDoc.Content)
	}
}

func TestEdgeCases_NestedBlocks(t *testing.T) {
	t.Run("handles deeply nested lists", func(t *testing.T) {
		md := adfToMD(doc(&adf.BulletList{
			Content: []adf.Node{li(
				p(txt("L1")),
				&adf.BulletList{Content: []adf.Node{li(
					p(txt("L2")),
					&adf.BulletList{Content: []adf.Node{
						li(p(txt("L3"))),
					}},
				)}},
			)},
		}))
		wantContains(t, md, "- L1", "  - L2", "    - L3")
	})

	t.Run("handles table with empty cells", func(t *testing.T) {
		md := adfToMD(doc(&adf.Table{
			Content: []adf.Node{
				&adf.TableRow{Content: []adf.Node{
					&adf.TableHeader{Content: []adf.Node{p(txt("H"))}},
					&adf.TableHeader{Content: []adf.Node{p(txt(""))}},
				}},
				&adf.TableRow{Content: []adf.Node{
					&adf.TableCell{Content: []adf.Node{p(txt(""))}},
					&adf.TableCell{Content: []adf.Node{p(txt("data"))}},
				}},
			},
		}))
		wantContains(t, md, "| H |", "data")
	})

	t.Run("handles multiple paragraphs in blockquote", func(t *testing.T) {
		md := adfToMD(doc(&adf.Blockquote{
			Content: []adf.Node{p(txt("first")), p(txt("second"))},
		}))
		wantContains(t, md, "> first", "> second")
		// blank separator line should have no trailing space (matches remark-stringify)
		if strings.Contains(md, "> \n") {
			t.Errorf("blockquote blank line should be '>\\n' not '> \\n': %q", md)
		}
	})
}

func TestEdgeCases_ListsAndPanels(t *testing.T) {
	t.Run("ordered list uses lazy numbering", func(t *testing.T) {
		md := adfToMD(doc(&adf.OrderedList{
			Content: []adf.Node{
				li(p(txt("a"))),
				li(p(txt("b"))),
				li(p(txt("c"))),
			},
		}))
		wantContains(t, md, "1. a", "1. b", "1. c")
	})

	t.Run("renders unknown panel types as info", func(t *testing.T) {
		md := adfToMD(doc(&adf.Panel{
			PanelType: "custom",
			Content:   []adf.Node{p(txt("content"))},
		}))
		wantContains(t, md, ":::info")
	})
}
