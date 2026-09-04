package adfast

import (
	"slices"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// Raw HTML on the two legs, pinned side by side.
//
// A downstream consumer reported the ADF round trip "escaping inline HTML
// into literal text", filed against the remark reference toolchain
// (mdast-util-from-markdown + mdast-util-to-markdown) which leaves it
// alone. The comparison was between two different pipelines: the
// reference figure was md → mdast → md, and the Go figure was
// md → ADF → md. Measured on the same pipeline the two agree byte for
// byte, so there is nothing to fix — these tests are the pin that keeps
// the next reader from re-filing it.
//
// The two legs, both measured 2026-09-04:
//
//	md → md (the formatter)   "a b <br> c"  =>  "a b <br> c\n"
//	                          mdast-util-to-markdown 2.1.0: identical.
//	md → ADF → md             "a b <br> c"  =>  "a b \\<br> c\n"
//	                          the reference serializer, handed the same
//	                          text-folded input the ADF leg forces
//	                          (a text node holding the bytes "<br>", or
//	                          the same three-way node split adfast
//	                          produces): "a b \\<br> c\n" — identical.
//
// The escape is the honest render of a conversion that already happened
// one step earlier: the ADF schema has no node kind for raw HTML (45
// kinds, none of them html — see docs/adf-availability.json), so the
// html node has to fold into text, and a text node holding '<' has to
// be escaped or the next parse would read it back as a tag.

const htmlInlineSrc = "a b <br> c"

// TestRawHTML_FormatterKeepsInlineHTML is the GOOD case for the pair
// below: with no ADF leg in between, the tag survives untouched, which
// is what the reference toolchain does with the same input.
func TestRawHTML_FormatterKeepsInlineHTML(t *testing.T) {
	got := ToMarkdown(
		FromMarkdown(htmlInlineSrc, WithPrettierFormat()),
		WithPrettierFormat(), WithPrintWidth(80),
	)
	if want := "a b <br> c\n"; got != want {
		t.Fatalf("md→md inline HTML: got %q, want %q", got, want)
	}

	// Block HTML too: the formatter may reshape an author's syntax but
	// never delete it.
	block := "<div>\nblock html\n</div>"
	got = ToMarkdown(
		FromMarkdown(block, WithPrettierFormat()),
		WithPrettierFormat(), WithPrintWidth(80),
	)
	if want := block + "\n"; got != want {
		t.Fatalf("md→md block HTML: got %q, want %q", got, want)
	}
}

// TestRawHTML_ADFLegFoldsInlineHTMLToText pins the fold itself, so the
// escape assertion that follows it is not vacuous: the ADF really does
// carry the tag as text bytes and really does carry no html kind.
func TestRawHTML_ADFLegFoldsInlineHTMLToText(t *testing.T) {
	doc := ToADF(FromMarkdown(htmlInlineSrc))

	var texts []string
	var walked int
	var rawKinds []string
	for n := range walkDoc(doc) {
		walked++
		switch v := n.(type) {
		case *adf.Text:
			texts = append(texts, v.Text)
		case *adf.RawNode:
			rawKinds = append(rawKinds, v.Type)
		}
	}
	// The walk itself has to have seen something, or every assertion
	// below is about an empty document.
	if walked == 0 {
		t.Fatal("walked no ADF nodes; the assertions below would be vacuous")
	}

	joined := strings.Join(texts, "")
	if joined != htmlInlineSrc {
		t.Errorf("ADF text bytes: got %q, want %q", joined, htmlInlineSrc)
	}
	// The precondition the escape depends on: the tag lives in a text
	// node, verbatim and unescaped.
	if !slices.Contains(texts, "<br>") {
		t.Errorf("ADF text nodes %q: none holds the verbatim tag %q", texts, "<br>")
	}
	// And it lives there rather than in some html-carrying kind smuggled
	// through as an unknown node.
	if len(rawKinds) != 0 {
		t.Errorf("ADF carries unknown kinds %q for inline HTML; the fold-to-text premise no longer holds", rawKinds)
	}
}

// TestRawHTML_ADFRoundTripEscapesFoldedText is a preserved-behavior PIN,
// not a fix proof: it passes before and after, and it exists because the
// output looks wrong until you see that the reference serializer prints
// the same bytes from the same input.
func TestRawHTML_ADFRoundTripEscapesFoldedText(t *testing.T) {
	const want = "a b \\<br> c\n"

	first := ToMarkdown(FromADF(ToADF(FromMarkdown(htmlInlineSrc))), WithPrintWidth(80))
	if first != want {
		t.Fatalf("first md→ADF→md trip: got %q, want %q", first, want)
	}

	// Idempotence is the half that would catch a partial change: the
	// backslash must not accumulate on the way round again. The escaped
	// '<' parses back as literal text, encodes as the same text, and
	// re-escapes to the same one backslash.
	second := ToMarkdown(FromADF(ToADF(FromMarkdown(first))), WithPrintWidth(80))
	if second != want {
		t.Fatalf("second md→ADF→md trip: got %q, want %q (the escape is accumulating)", second, want)
	}

	// Block HTML has no ADF form at all and drops on this leg, where the
	// formatter above keeps it.
	blockOut := ToMarkdown(FromADF(ToADF(FromMarkdown("<div>\nblock html\n</div>"))), WithPrintWidth(80))
	if strings.Contains(blockOut, "div") {
		t.Fatalf("block HTML md→ADF→md: got %q, want the tag dropped", blockOut)
	}
}

// walkDoc walks every node in a document: adf.Walk takes a Node and a
// Doc is not one, so iterate its top-level blocks.
func walkDoc(doc adf.Doc) func(func(adf.Node) bool) {
	return func(yield func(adf.Node) bool) {
		for _, block := range doc.Content {
			for n := range adf.Walk(block) {
				if !yield(n) {
					return
				}
			}
		}
	}
}
