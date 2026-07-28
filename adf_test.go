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

func ptrOf[T any](v T) *T { return &v }

// ---------------------------------------------------------------------------
// ADF → MD: Block types
// ---------------------------------------------------------------------------

func TestAdfToMarkdown_BlockTypes(t *testing.T) {
	tests := []struct {
		name     string
		exact    string
		contains []string
		input    adf.Doc
	}{
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
				Layout: ptrOf("align-start"), Width: ptrOf(float64(686)), WidthType: ptrOf("pixel"),
				Content: []adf.Node{&adf.Media{
					Type: "file", ID: "abc", Alt: "shot.png",
					Collection: ptrOf(""), Width: ptrOf(float64(2308)), Height: ptrOf(float64(551)),
				}},
			}),
			contains: []string{`::media[shot.png]{#abc collection height="551" layoutWidth="686" width="2308" widthType="pixel"}`},
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
			input:    doc(p(&adf.Emoji{Text: ptrOf("👍"), ShortName: ":thumbsup:"})),
			contains: []string{"👍"},
		},
		{
			name:     "mention",
			input:    doc(p(&adf.Mention{Text: ptrOf("@alice"), ID: "123"})),
			contains: []string{":mention[alice]{#123}"},
		},
		{
			name:     "inlineCard with Jira URL",
			input:    doc(p(&adf.InlineCard{URL: ptrOf("https://ixolit.atlassian.net/browse/PROJ-42")})),
			contains: []string{"[PROJ-42](https://ixolit.atlassian.net/browse/PROJ-42)"},
		},
		{
			name:     "inlineCard with non-Jira URL",
			input:    doc(p(&adf.InlineCard{URL: ptrOf("https://example.com/page")})),
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
	tests := []struct {
		name       string
		input      string
		contains   []string
		notContain []string
	}{
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adfDoc := mdToADF(tt.input)
			roundTrip := adfToMD(adfDoc)
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

func TestEdgeCases(t *testing.T) {
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
		if !strings.Contains(md, "- L1") {
			t.Errorf("missing L1: %q", md)
		}
		if !strings.Contains(md, "  - L2") {
			t.Errorf("missing L2: %q", md)
		}
		if !strings.Contains(md, "    - L3") {
			t.Errorf("missing L3: %q", md)
		}
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
		if !strings.Contains(md, "| H |") {
			t.Errorf("missing header: %q", md)
		}
		if !strings.Contains(md, "data") {
			t.Errorf("missing data: %q", md)
		}
	})

	t.Run("handles multiple paragraphs in blockquote", func(t *testing.T) {
		md := adfToMD(doc(&adf.Blockquote{
			Content: []adf.Node{p(txt("first")), p(txt("second"))},
		}))
		if !strings.Contains(md, "> first") {
			t.Errorf("missing first: %q", md)
		}
		if !strings.Contains(md, "> second") {
			t.Errorf("missing second: %q", md)
		}
		// blank separator line should have no trailing space (matches remark-stringify)
		if strings.Contains(md, "> \n") {
			t.Errorf("blockquote blank line should be '>\\n' not '> \\n': %q", md)
		}
	})

	t.Run("ordered list uses lazy numbering", func(t *testing.T) {
		md := adfToMD(doc(&adf.OrderedList{
			Content: []adf.Node{
				li(p(txt("a"))),
				li(p(txt("b"))),
				li(p(txt("c"))),
			},
		}))
		if !strings.Contains(md, "1. a") {
			t.Errorf("missing '1. a': %q", md)
		}
		if !strings.Contains(md, "1. b") {
			t.Errorf("missing '1. b': %q", md)
		}
		if !strings.Contains(md, "1. c") {
			t.Errorf("missing '1. c': %q", md)
		}
	})

	t.Run("renders unknown panel types as info", func(t *testing.T) {
		md := adfToMD(doc(&adf.Panel{
			PanelType: "custom",
			Content:   []adf.Node{p(txt("content"))},
		}))
		if !strings.Contains(md, ":::info") {
			t.Errorf("expected :::info: %q", md)
		}
	})

	t.Run("handles empty markdownToAdf input", func(t *testing.T) {
		adfDoc := mdToADF("")
		if adfDoc.Type != "doc" || adfDoc.Version != 1 {
			t.Errorf("unexpected doc: %+v", adfDoc)
		}
		if len(adfDoc.Content) != 1 || adfDoc.Content[0].Kind() != "paragraph" {
			t.Errorf("expected single empty paragraph: %+v", adfDoc.Content)
		}
	})

	t.Run("handles whitespace-only markdownToAdf input", func(t *testing.T) {
		adfDoc := mdToADF("   \n  \n  ")
		if adfDoc.Type != "doc" || adfDoc.Version != 1 {
			t.Errorf("unexpected doc: %+v", adfDoc)
		}
		if len(adfDoc.Content) != 1 || adfDoc.Content[0].Kind() != "paragraph" {
			t.Errorf("expected single empty paragraph: %+v", adfDoc.Content)
		}
	})
}

// Regression: empty list items must render their marker ("-", "1.") so the
// list survives re-parsing; a first block that renders empty must not leave
// a trailing-space marker. Both were fuzz findings (storysmith-md-7a7s).
func TestRoundTripIdempotent_EmptyListItems(t *testing.T) {
	inputs := []string{
		"0)\nA0",   // empty ordered item followed by lazy text
		"1.\ntext", // empty ordered item
		"-\n- b",   // empty bullet item before non-empty one
		"* [X]",    // bullet item whose paragraph renders empty
		"- [ ]",    // task marker without content
	}
	for _, md := range inputs {
		first := adfToMD(mdToADF(md))
		second := adfToMD(mdToADF(first))
		if first != second {
			t.Errorf("round-trip not idempotent for %q:\nfirst:  %q\nsecond: %q", md, first, second)
		}
	}
}

// Regression: paragraph text that looks like an ATX heading must be escaped
// or it becomes a real heading on re-parse (fuzz finding; remark escapes it too).
func TestRoundTripIdempotent_LeadingHash(t *testing.T) {
	for _, md := range []string{"\\#", "\\# literal hash", "\\## two"} {
		first := adfToMD(mdToADF(md))
		second := adfToMD(mdToADF(first))
		if first != second {
			t.Errorf("round-trip not idempotent for %q:\nfirst:  %q\nsecond: %q", md, first, second)
		}
		if !strings.Contains(first, `\#`) {
			t.Errorf("leading hash not escaped for %q: %q", md, first)
		}
	}
}

// The mention directive itself is the @: :mention[Name] (no leading @)
// encodes an ADF mention whose text carries the conventional "@" prefix,
// renders back without it, and the legacy :mention[@Name] form still
// parses (leading @ stripped).
func TestMention_LabelWithoutAt(t *testing.T) {
	want := doc(p(&adf.Mention{Text: ptrOf("@Jane Doe"), ID: "712020:aa"}))
	for _, md := range []string{
		":mention[Jane Doe]{#712020:aa}",
		":mention[@Jane Doe]{#712020:aa}", // legacy form keeps working
	} {
		got := mdToADF(md)
		if gotJSON, wantJSON := string(mustJSON(got)), string(mustJSON(want)); gotJSON != wantJSON {
			t.Errorf("mdToADF(%q) = %s, want %s", md, gotJSON, wantJSON)
		}
	}
	out := adfToMD(want)
	if out != ":mention[Jane Doe]{#712020:aa}\n" {
		t.Errorf("render = %q, want no leading @", out)
	}
	// Round-trip idempotence for the new form.
	first := adfToMD(mdToADF(":mention[Jane Doe]{#712020:aa}"))
	second := adfToMD(mdToADF(first))
	if first != second {
		t.Errorf("round-trip not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

// A ::decisions leaf directive marks the immediately following plain
// bullet list as an ADF decisionList (exactly like ::colwidths marks the
// following table), and the pair round-trips stably.
func TestDecisions_DirectiveMarksFollowingList(t *testing.T) {
	md := "::decisions\n\n- use jj for vcs\n- ship it\n"
	d := mdToADF(md)
	if len(d.Content) != 1 {
		t.Fatalf("content: %d nodes", len(d.Content))
	}
	dl, ok := d.Content[0].(*adf.DecisionList)
	if !ok {
		t.Fatalf("expected decisionList, got %T", d.Content[0])
	}
	if len(dl.Content) != 2 {
		t.Fatalf("items: %d", len(dl.Content))
	}
	for _, itemNode := range dl.Content {
		item, itemOK := itemNode.(*adf.DecisionItem)
		if !itemOK || item.State != "DECIDED" {
			t.Errorf("item: %+v", itemNode)
		}
	}
	first := adfToMD(d)
	if first != md {
		t.Errorf("render = %q, want %q", first, md)
	}
	if second := adfToMD(mdToADF(first)); second != first {
		t.Errorf("round-trip not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

// "- [~] x" is an ordinary bullet item with literal text: goldmark's GFM
// task-list extension only recognizes [ ]/[x]/[X] checkboxes, so the
// unrecognized [~] marker stays cell text (and renders escaped).
func TestDecisions_TildeCheckboxStaysLiteral(t *testing.T) {
	d := mdToADF("- [~] not a decision\n")
	bl, ok := d.Content[0].(*adf.BulletList)
	if !ok {
		t.Fatalf("expected bulletList, got %T", d.Content[0])
	}
	item := adf.NodeContent(bl)[0]
	para := adf.NodeContent(item)[0]
	if got := adf.NodeText(adf.NodeContent(para)[0]); got != "[~] not a decision" {
		t.Errorf("literal text: %q", got)
	}
	first := adfToMD(d)
	if !strings.Contains(first, `\[\~] not a decision`) {
		t.Errorf("render must escape the literal marker: %q", first)
	}
	if second := adfToMD(mdToADF(first)); second != first {
		t.Errorf("round-trip not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

// Regression: consecutive ordered lists must alternate their delimiter
// ("1." / "1)") or they merge into one list on re-parse (fuzz finding;
// remark-stringify alternates the same way).
func TestRoundTripIdempotent_AdjacentOrderedLists(t *testing.T) {
	first := adfToMD(mdToADF("1) a\n1. b"))
	if first != "1. a\n\n1) b\n" {
		t.Errorf("expected alternated delimiters, got %q", first)
	}
	second := adfToMD(mdToADF(first))
	if first != second {
		t.Errorf("round-trip not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

// The terse ::media directive (default type/layout omitted) must re-inflate to
// the original ADF, so a Markdown-authored push reproduces the media node.
func TestMediaDirective_DefaultsRoundTrip(t *testing.T) {
	in := doc(&adf.MediaSingle{
		Layout: ptrOf("align-start"), Width: ptrOf(float64(686)), WidthType: ptrOf("pixel"),
		Content: []adf.Node{&adf.Media{
			Type: "file", ID: "abc", Alt: "shot.png",
			Collection: ptrOf(""), Width: ptrOf(float64(2308)), Height: ptrOf(float64(551)),
		}},
	})
	md := adfToMD(in)
	if strings.Contains(md, `type="file"`) || strings.Contains(md, `layout="align-start"`) {
		t.Fatalf("expected default type/layout omitted, got %q", md)
	}

	back := mdToADF(md)
	if len(back.Content) != 1 {
		t.Fatalf("expected one block, got %d", len(back.Content))
	}
	single, ok := back.Content[0].(*adf.MediaSingle)
	if !ok {
		t.Fatalf("expected *adf.MediaSingle, got %T", back.Content[0])
	}
	if single.Layout == nil || *single.Layout != "align-start" {
		t.Errorf("layout not re-inferred: %v", single.Layout)
	}
	media, ok := single.Content[0].(*adf.Media)
	if !ok {
		t.Fatalf("expected *adf.Media, got %T", single.Content[0])
	}
	if media.Type != "file" {
		t.Errorf("type not re-inferred: %q", media.Type)
	}
}
