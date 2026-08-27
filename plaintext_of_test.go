package adfast_test

import (
	"strings"
	"testing"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/markdown"
	directive "github.com/pmarschik/goldmark-directive"
)

func TestPlainTextOf(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Inline markup drops, text stays.
		{"plain", "hello world", "hello world"},
		{"strong", "**bold text**", "bold text"},
		{"emphasis", "*italic text*", "italic text"},
		{"strikethrough", "~~deleted~~", "deleted"},
		{"link keeps the label", "[click here](https://example.com)", "click here"},
		{"empty link label", "[](https://example.com)", ""},
		{"image keeps the alt", "text with ![alt](img.png) image", "text with alt image"},
		{"nested", "**_nested_**", "nested"},
		{"mixed", "Fix **critical** bug in `parser` — see [issue](http://x)", "Fix critical bug in parser — see issue"},

		// Whitespace tidying.
		{"space runs collapse", "hello   world", "hello world"},
		{"ends trim", "  hello  ", "hello"},
		{"empty", "", ""},
		{"blank", "   \n\n  ", ""},

		// Code spans.
		{"code span", "`some code`", "some code"},
		{"code span keeps colons", "Redo `a:b:c` now", "Redo a:b:c now"},

		// Fenced blocks: code text, space-padded, line structure kept.
		{"fenced block", "```go\nfmt.Println(1)\nfmt.Println(2)\n```\n", "fmt.Println(1)\n fmt.Println(2)"},
		{"fenced block, no lang", "```\nplain\n```\n", "plain"},
		{"fenced block between prose", "before\n\n```\ncode\n```\n\nafter", "before code after"},
		{"indented block", "    indented code\n", "indented code"},

		// Text directives — the label follows the literal name.
		{"text directive, unknown name", "a :unknown[Label]{k=v} b", "a :unknownLabel b"},
		{"text directive, bare", "foo :bar", "foo :bar"},
		{"text directive, known name keeps the name", "a :status[Done]{color=green} b", "a :statusDone b"},
		{"text directive, known name, no label", "Update the deploy:status endpoint", "Update the deploy:status endpoint"},
		{"text directive, styled label", "a :unknown[Lab **el**] b", "a :unknownLab el b"},
		{"intraword colons", "Add a BE endpoint to handle auth0:user:created callback", "Add a BE endpoint to handle auth0:user:created callback"},

		// Leaf directives.
		{"leaf directive, unknown name", "::unknown[Label]{a=b}\n", ":unknownLabel"},
		{"leaf directive, known name", "::media[alt]{id=x}\n", ":mediaalt"},
		{"leaf directive, no label", "::decisions\n", ":decisions"},

		// Container directives — remark keeps the label as the first child
		// paragraph, so it simply falls out of the child walk.
		{"container directive, unknown name", ":::unknown\ncontent\n:::\n", ":unknowncontent"},
		{"container directive, known name", ":::panel{type=info}\nInside panel\n:::\n", ":panelInside panel"},
		{"container directive, label and body", ":::expand[Title]\nBody\n:::\n", ":expandTitleBody"},
		{"container directive name is not folded", ":::info\ntext\n:::\n", ":infotext"},
		{"a second name on the same typed kind", ":::warning\ntext\n:::\n", ":warningtext"},

		// Blocks concatenate without a separator, matching ast.PlainText.
		{"heading then paragraph", "# Heading\n\nBody text", "HeadingBody text"},
		{"list items", "- a\n- b\n", "ab"},
		{"table cells", "| a | b |\n| - | - |\n| c | d |\n", "abcd"},
		{"blockquote", "> quoted text\n", "quoted text"},

		// Frontmatter is metadata, not prose.
		{"frontmatter drops", "---\ntitle: x\n---\n\nBody\n", "Body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adfast.PlainTextOf(tt.in); got != tt.want {
				t.Errorf("PlainTextOf(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A directive name a WithExtensions registration owns still reads as its
// literal name: PlainTextOf parses generically on purpose, since a
// registration is exactly what would hide the name.
func TestPlainTextOfIgnoresExtensions(t *testing.T) {
	const md = "a :unknown[Label] b"
	want := adfast.PlainTextOf(md)
	got := adfast.PlainTextOf(md, adfast.WithExtensions(dialect.Registrations()...))
	if got != want {
		t.Errorf("registrations changed the projection: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Equivalence with the walk PlainTextOf replaces
// ---------------------------------------------------------------------------

// storysmith-md's internal/domain/format.go carried its own plaintext
// projection: it built markdown.NewParser() and walked the RAW GOLDMARK
// tree, reading LabelRoot and LabelSource off the goldmark-directive nodes.
// That reached past this library's public surface into its parser
// internals, which is what PlainTextOf exists to end.
//
// legacyWalk below is that walk, copied verbatim, so the replacement is
// provably equivalent rather than merely plausible. Every case where the
// two disagree is listed in knownDivergences with the reason; a case that
// starts agreeing, or a new disagreement, fails this test.
func TestPlainTextOfMatchesLegacyGoldmarkWalk(t *testing.T) {
	// knownDivergences maps an input to the legacy result PlainTextOf
	// deliberately does NOT reproduce. Every one is the legacy walk losing
	// text or leaking source bytes; PlainTextOf is the more faithful side
	// in each.
	knownDivergences := map[string]string{
		// goldmark's AutoLink node holds no *ast.Text child, so the legacy
		// walk silently deleted a bare URL. "Update https://…" became
		// "Update" — a summary quietly losing its subject.
		"Update https://ixolit.atlassian.net/wiki/spaces/DEVELOPMENT/pages/674103342/User": "Update",

		// A soft line break separates two words. The legacy walk emitted the
		// two goldmark text segments back to back and fused them into one
		// word ("line oneline two"); the pivot AST already carries the break
		// as the space it is.
		"line one\nline two": "line oneline two",
		"term\n: definition": "term: definition",

		// The legacy walk cased on *ast.FencedCodeBlock only, so an INDENTED
		// code block contributed nothing at all.
		"    indented code\n": "",

		// The legacy walk read goldmark's line segments, each of which keeps
		// the newline that ends it — including the last one, which is the
		// fence's line terminator rather than content. So a code block with
		// prose after it emitted a stray newline before that prose. The
		// pivot AST's Code.Value does not carry the terminator, and only
		// the ends of the whole projection are trimmed, so the difference
		// shows only when the block is not the last thing in the document.
		"before\n\n```\ncode\n```\n\nafter": "before code\n after",

		// The legacy walk ran the parser over the whole file, so a leading
		// frontmatter block leaked into the prose as its raw lines.
		"---\ntitle: x\n---\n\nBody\n": "title: xBody",

		// The legacy walk read raw source segments, so a backslash escape
		// stayed in the output as a literal backslash. Plain text is the
		// decoded text.
		"a\\:b": "a\\:b",
	}

	for _, md := range plainTextCorpus {
		t.Run(strings.ReplaceAll(md, "\n", "\\n"), func(t *testing.T) {
			legacy := legacyMarkdownToPlainText(md)
			got := adfast.PlainTextOf(md)
			if want, ok := knownDivergences[md]; ok {
				if legacy != want {
					t.Fatalf("the legacy walk no longer produces the divergence this case records:\n legacy = %q\n recorded = %q", legacy, want)
				}
				if got == legacy {
					t.Fatalf("PlainTextOf now agrees with the legacy walk on %q (= %q); drop the knownDivergences entry", md, got)
				}
				return
			}
			if got != legacy {
				t.Errorf("PlainTextOf(%q) = %q, legacy walk = %q\n(if this difference is intended, record it in knownDivergences with the reason)", md, got, legacy)
			}
		})
	}
}

// plainTextCorpus is the differential corpus: the cases storysmith-md's own
// test pinned, plus one input per construct the legacy walk cased on and per
// directive form.
var plainTextCorpus = []string{
	// storysmith-md's pinned cases.
	"hello world",
	"**bold text**",
	"__bold text__",
	"*italic text*",
	"_italic text_",
	"~~deleted~~",
	"`some code`",
	"[click here](https://example.com)",
	"[](https://example.com)",
	"**bold** and *italic* with `code`",
	"**_nested_**",
	"hello   world",
	"  hello  ",
	"Fix **critical** bug in `parser` — see [issue](http://x)",
	"",
	"Update https://ixolit.atlassian.net/wiki/spaces/DEVELOPMENT/pages/674103342/User",
	"Redo `ixopay:auth0:reset-oidc-last-sync-at` with queries instead of entity manager",
	"Add a BE endpoint to handle auth0:user:created callback",
	"Add auth0:event_streams:user_update resource to terraform",

	// Blocks.
	"para one\n\npara two",
	"# Heading\n\nBody text",
	"- a\n- b\n",
	"1. one\n2. two\n",
	"- [ ] task\n- [x] done\n",
	"> quoted text\n",
	"| a | b |\n| - | - |\n| c | d |\n",
	"***\n",
	"line one\nline two",
	"term\n: definition",
	"a\\\nb hard break",
	"a  \nb trailing space break",
	"<div>html block</div>\n",
	"text <b>inline html</b> here",
	"foo[^1]\n\n[^1]: note text\n",
	"[ref][a]\n\n[a]: http://x\n",
	"---\ntitle: x\n---\n\nBody\n",
	"a\\:b",

	// Code.
	"```go\nfmt.Println(1)\nfmt.Println(2)\n```\n",
	"```\nplain\n```\n",
	"```\nline\n\nblank inside\n```\n",
	"before\n\n```\ncode\n```\n\nafter",
	"    indented code\n",
	"text with ![alt](img.png) image",

	// Text directives, unknown and known.
	"foo :bar",
	":unknowntext[Label]{a=b}\n",
	"a :unknowntext[Lab **el**] b",
	"a :status[Done]{color=green} b",
	"a :mention[Bob]{id=1} b",
	"a :color[red text]{value=#ff0000} b",
	"a :u[under] b",
	"a :sub[x] b",
	"a :sup[2] b",
	"a :emoji[smile] b",
	"a :date[2024-01-01] b",
	"a :annotation[x]{id=1} b",
	"a :placeholder[hint] b",
	"a :fontSize[big]{value=20} b",
	"deploy:status and cache:date and x:u and y:media",
	"### h3 :sub[x] tail",

	// Leaf directives, unknown and known.
	"::unknownleaf[Label]{a=b}\n",
	"::media[alt]{id=x}\n",
	"::colwidths[100,200]\n",
	"::decisions\n",
	":jql[project = X]\n",

	// Container directives, unknown and known — including two names that
	// promote to one typed kind, which is why the generic parse is needed.
	":::unknowncontainer\ncontent\n:::\n",
	":::panel{type=info}\nInside panel\n:::\n",
	":::info\ntext\n:::\n",
	":::note\ntext\n:::\n",
	":::warning\ntext\n:::\n",
	":::center\ntext\n:::\n",
	":::end\ntext\n:::\n",
	":::expand[Title]\nBody\n:::\n",
}

// ---------------------------------------------------------------------------
// The replaced implementation, copied verbatim from storysmith-md's
// internal/domain/format.go (MarkdownToPlainText / walkMarkdownNodes).
// ---------------------------------------------------------------------------

func legacyMarkdownToPlainText(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}

	parser := markdown.NewParser()
	reader := text.NewReader([]byte(s))
	root := parser.Parse(reader)

	source := []byte(s)
	var parts []string
	legacyWalk(root, source, &parts)

	result := strings.Join(parts, "")
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
}

func legacyWalk(node gast.Node, source []byte, parts *[]string) {
	switch n := node.(type) {
	case *gast.Text:
		if val := n.Value(source); len(val) > 0 {
			*parts = append(*parts, string(val))
		}
		return
	case *gast.CodeSpan:
		var val []byte
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*gast.Text); ok {
				val = append(val, t.Segment.Value(source)...)
			}
		}
		if len(val) > 0 {
			*parts = append(*parts, string(val))
		}
		return
	case *gast.FencedCodeBlock:
		lines := n.Lines()
		for i := range lines.Len() {
			segment := lines.At(i)
			lineContent := segment.Value(source)
			if len(lineContent) > 0 {
				*parts = append(*parts, " ", string(lineContent), " ")
			}
		}
		return
	case *directive.ContainerDirective:
		*parts = append(*parts, ":"+n.Name)
	case *directive.LeafDirective:
		*parts = append(*parts, ":"+n.Name)
	case *directive.TextDirective:
		*parts = append(*parts, ":"+n.Name)
		if n.LabelRoot != nil {
			legacyWalk(n.LabelRoot, n.LabelSource, parts)
		}
		return
	}

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		legacyWalk(child, source, parts)
	}
}
