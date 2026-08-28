package adfast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/markdown"
)

// bom is the UTF-8 byte order mark, written as an escape because a Go
// source file rejects the literal rune anywhere but its own first byte.
const bom = "\ufeff"

// topKinds names the root's children in order, which is what separates a
// first block that parsed from one the mark degraded: the rendered bytes
// of a BOM'd heading are the same either way (a paragraph holding "#
// Heading" prints back as "# Heading"), so only the tree tells the truth.
func topKinds(n ast.Node) string {
	root, ok := n.(*ast.Root)
	if !ok {
		return fmt.Sprintf("%T", n)
	}
	names := make([]string, 0, len(root.Children))
	for _, c := range root.Children {
		names = append(names, c.Kind())
	}
	return strings.Join(names, " ")
}

// A leading byte order mark is a decoding artifact. Left in the source it
// is ordinary text at the start of line 1, so the first block misparses:
// a heading degrades to a paragraph, a list's first item splits off as
// one, a fence is reflowed into escaped prose, and a frontmatter block
// becomes a setext heading. Every document here also holds a second block
// that never depended on the fix, so a row can only pass by parsing the
// FIRST block correctly.
func TestByteOrderMarkDoesNotCorruptTheFirstBlock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		in    string
		kinds string
		out   string
	}{
		{
			name:  "heading",
			in:    bom + "# Heading\n\nBody.\n",
			kinds: "heading paragraph",
			out:   bom + "# Heading\n\nBody.\n",
		},
		{
			name:  "list",
			in:    bom + "- a\n- b\n",
			kinds: "list",
			out:   bom + "- a\n- b\n",
		},
		{
			name:  "fenced code",
			in:    bom + "```go\nx := 1\n```\n\nAfter.\n",
			kinds: "code paragraph",
			out:   bom + "```go\nx := 1\n```\n\nAfter.\n",
		},
		{
			name:  "frontmatter",
			in:    bom + "---\ntitle: A\n---\n\n# H\n",
			kinds: "yaml heading",
			out:   bom + "---\ntitle: A\n---\n\n# H\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := FromMarkdown(tc.in, WithPrettierFormat())
			if got := topKinds(root); got != tc.kinds {
				t.Errorf("top-level kinds = %q, want %q", got, tc.kinds)
			}
			if got := fmtMD(tc.in); got != tc.out {
				t.Errorf("format = %q, want %q", got, tc.out)
			}
		})
	}
}

// The policy is strip and re-emit, prettier's: the mark is peeled before
// the parse and prepended after the print, so a document that opened with
// one still opens with one and a document that did not still does not.
// adfast never adds or drops an encoding preamble on its own.
func TestByteOrderMarkPolicyIsStripAndReEmit(t *testing.T) {
	t.Parallel()
	const body = "# H\n\nBody.\n"

	t.Run("carried on the root, not in the tree", func(t *testing.T) {
		t.Parallel()
		with, ok := FromMarkdown(bom + body).(*ast.Root)
		if !ok {
			t.Fatalf("FromMarkdown did not return *ast.Root")
		}
		if !with.ByteOrderMark {
			t.Errorf("Root.ByteOrderMark = false, want true")
		}
		if text := ast.PlainText(with.Children); strings.Contains(text, bom) {
			t.Errorf("the mark leaked into the tree: %q", text)
		}
		without, ok := FromMarkdown(body).(*ast.Root)
		if !ok {
			t.Fatalf("FromMarkdown did not return *ast.Root")
		}
		if without.ByteOrderMark {
			t.Errorf("Root.ByteOrderMark = true for a source with no mark")
		}
	})

	t.Run("re-emitted from the flag", func(t *testing.T) {
		t.Parallel()
		root := &ast.Root{Children: ast.Children(FromMarkdown(body)), ByteOrderMark: true}
		if got, want := ToMarkdown(root), bom+body; got != want {
			t.Errorf("render = %q, want %q", got, want)
		}
		root.ByteOrderMark = false
		if got, want := ToMarkdown(root), body; got != want {
			t.Errorf("render = %q, want %q", got, want)
		}
	})

	t.Run("an empty document keeps its mark", func(t *testing.T) {
		t.Parallel()
		// U+FEFF is not Unicode whitespace, so a mark left in the source
		// would make the document look non-empty and parse to a paragraph.
		// Assert the tree, not only the bytes: the render of a stray
		// mark-paragraph happens to spell the same thing.
		root, ok := FromMarkdown(bom, WithPrettierFormat()).(*ast.Root)
		if !ok {
			t.Fatalf("FromMarkdown did not return *ast.Root")
		}
		if len(root.Children) != 0 {
			t.Errorf("root has %d children (%q), want an empty document", len(root.Children), topKinds(root))
		}
		if got, want := fmtMD(bom), bom+"\n"; got != want {
			t.Errorf("format = %q, want %q", got, want)
		}
	})

	t.Run("ADF has nowhere to keep it", func(t *testing.T) {
		t.Parallel()
		b, err := json.Marshal(mdToADF(bom + body))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(b), bom) {
			t.Errorf("the mark reached ADF: %s", b)
		}
	})
}

// PlainTextOf projects the text a reader sees; a byte order mark is not
// text. It shares parseMarkdownSource with FromMarkdown, so the peel
// covers it too.
func TestByteOrderMarkStaysOutOfPlainText(t *testing.T) {
	t.Parallel()
	cases := []string{
		bom + "# Heading\n\nBody.\n",
		bom + "- a\n- b\n",
		bom + "```go\nx := 1\n```\n",
		bom + "---\ntitle: A\n---\n\n# H\n",
	}
	for i, in := range cases {
		got := PlainTextOf(in)
		if strings.Contains(got, bom) {
			t.Errorf("case %d: PlainTextOf(%q) = %q, still carries the mark", i, in, got)
		}
	}
}

// A FrontmatterProvider is handed source with the mark already peeled, so
// a caller's convention never needs its own tolerance for one. The
// provider here asserts that, and would not find its block if the mark
// still led the source.
func TestByteOrderMarkNeedsNoFrontmatterProviderTolerance(t *testing.T) {
	t.Parallel()
	var seen string
	provider := func(md string) (string, string, FrontmatterOutcome) {
		seen = md
		front, rest, ok := strings.Cut(md, "-->\n")
		if !ok || !strings.HasPrefix(md, "<!-- ") {
			return "", md, FrontmatterAbsent
		}
		return front + "-->\n", rest, FrontmatterFound
	}

	in := bom + "<!-- Space: DEV -->\n# H\n"
	root := FromMarkdown(in, WithFrontmatterProvider(provider))
	if strings.HasPrefix(seen, bom) {
		t.Errorf("the provider was handed a marked source: %q", seen)
	}
	if got, want := topKinds(root), "yaml heading"; got != want {
		t.Errorf("top-level kinds = %q, want %q", got, want)
	}
	// The mark is the only difference from the same document without one.
	unmarked := ToMarkdown(FromMarkdown(strings.TrimPrefix(in, bom), WithFrontmatterProvider(provider)))
	if got, want := ToMarkdown(root), bom+unmarked; got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
}

// The mark is a pure prefix: over the whole contract corpus, marking a
// source shifts nothing but byte 0 of the render, and changes the ADF not
// at all. This is the standing guard that the peel never reaches content.
func TestByteOrderMarkIsAPurePrefix_Corpus(t *testing.T) {
	t.Parallel()
	for name, md := range formatContractInputs(t) {
		plain, marked := fmtMD(md), fmtMD(bom+md)
		if want := bom + plain; marked != want {
			t.Errorf("%s: marked format = %q, want %q", name, marked, want)
		}
		if got, want := marshalADF(t, bom+md), marshalADF(t, md); got != want {
			t.Errorf("%s: the mark changed the ADF\n marked: %s\n plain:  %s", name, got, want)
		}
		if got, want := PlainTextOf(bom+md), PlainTextOf(md); got != want {
			t.Errorf("%s: the mark changed the plain text\n marked: %q\n plain:  %q", name, got, want)
		}
	}
}

// PIN (preserved behavior, not a regression test): markdown.ByteOrderMark
// is the one spelling of U+FEFF the package works from. Nothing today can
// break it, but a future hand-typed literal in either the parse or the
// render would drift silently, so the value is pinned to its bytes.
func TestByteOrderMarkConstant(t *testing.T) {
	t.Parallel()
	if got, want := markdown.ByteOrderMark, "\ufeff"; got != want {
		t.Errorf("markdown.ByteOrderMark = %q, want %q", got, want)
	}
	if got, want := []byte(markdown.ByteOrderMark), []byte{0xEF, 0xBB, 0xBF}; !bytes.Equal(got, want) {
		t.Errorf("markdown.ByteOrderMark bytes = % x, want % x", got, want)
	}
}
