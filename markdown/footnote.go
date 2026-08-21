package markdown

import (
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/pmarschik/adfast/ast"
)

// GFM footnotes: "[^label]" references and "[^label]: content"
// definitions, parsed the way micromark's gfm-footnote extension does.
//
// goldmark ships extension.Footnote, and adfast deliberately does not use
// it: it is a footnote-to-HTML pipeline, not a footnote parser. Its AST
// transformer moves every definition into a list at the end of the
// document, sorts that list by first-reference order, appends backlink
// nodes inside each definition, and DELETES a definition nothing
// references. Four rewrites remark does not do, one of them silent data
// loss. The parsers below keep each definition where the source put it,
// which is what makes the md → md formatter footnote-preserving.
//
// Without them, "[^1]: note" reads as a link reference definition and
// "a[^1]" as the reference to it, so the pair silently renders as
// "a[^1](note)" — the bug these parsers close.

// kindFootnoteDef and kindFootnoteRef are the goldmark node kinds the two
// parsers below produce; the lift (goldmark_to_ast.go) turns them into
// ast.FootnoteDef and ast.FootnoteRef.
var (
	kindFootnoteDef = gast.NewNodeKind("AdfastFootnoteDefinition")
	kindFootnoteRef = gast.NewNodeKind("AdfastFootnoteReference")
)

// footnoteDefBlock is a parsed "[^label]:" definition, holding the
// definition's blocks as its children.
type footnoteDefBlock struct {
	Label string
	gast.BaseBlock
}

// Kind implements goldmark's ast.Node.
func (*footnoteDefBlock) Kind() gast.NodeKind { return kindFootnoteDef }

// Dump implements goldmark's ast.Node.
func (n *footnoteDefBlock) Dump(src []byte, level int) {
	gast.DumpHelper(n, src, level, map[string]string{"Label": n.Label}, nil)
}

// footnoteRefInline is a parsed "[^label]" reference.
type footnoteRefInline struct {
	Label string
	gast.BaseInline
}

// Kind implements goldmark's ast.Node.
func (*footnoteRefInline) Kind() gast.NodeKind { return kindFootnoteRef }

// Dump implements goldmark's ast.Node.
func (n *footnoteRefInline) Dump(src []byte, level int) {
	gast.DumpHelper(n, src, level, map[string]string{"Label": n.Label}, nil)
}

// footnoteLabelsKey holds the set of normalized definition labels seen in
// the document, as a map[string]struct{} in the parse context. goldmark
// parses every block before any inline, so the set is complete by the
// time the reference parser consults it — which is what lets a reference
// stay literal text when nothing defines it, like GFM.
var footnoteLabelsKey = parser.NewContextKey()

// registerFootnoteLabel records a definition's label for the reference
// parser.
func registerFootnoteLabel(pc parser.Context, label string) {
	set, ok := pc.Get(footnoteLabelsKey).(map[string]struct{})
	if !ok {
		set = map[string]struct{}{}
		pc.Set(footnoteLabelsKey, set)
	}
	set[ast.NormalizeFootnoteLabel(label)] = struct{}{}
}

// footnoteLabelDefined reports whether the document defines label.
func footnoteLabelDefined(pc parser.Context, label string) bool {
	set, isSet := pc.Get(footnoteLabelsKey).(map[string]struct{})
	if !isSet {
		return false
	}
	_, ok := set[ast.NormalizeFootnoteLabel(label)]
	return ok
}

// footnoteLabelMax is micromark's link-reference label cap, which the
// footnote label shares: 999 raw characters between "[^" and "]" are a
// label, 1000 are not (measured).
const footnoteLabelMax = 999

// scanFootnoteLabel reads a "[^label]" opener at the start of line and
// returns the raw label and the offset just past the "]". The label is
// kept exactly as written, backslash escapes included: the render writes
// it back verbatim, and NormalizeFootnoteLabel pairs the two ends on the
// same raw text.
//
// The label rules are micromark's, all measured against remark-gfm. It
// may hold NO whitespace, not even escaped ("[^a b]: x" and "[^a\ b]: x"
// are link reference definitions, not footnotes, and "a[^a b]" is a link
// reference); an unescaped bracket inside ends it ("[^a[b]]: x" and
// "[^a]b]: x" are paragraphs); it is at most footnoteLabelMax
// characters; and an empty or unterminated label is not a label at all.
func scanFootnoteLabel(line []byte) (label string, after int, ok bool) {
	if len(line) < 2 || line[0] != '[' || line[1] != '^' {
		return "", 0, false
	}
	for i := 2; i < len(line) && i-2 <= footnoteLabelMax; i++ {
		switch line[i] {
		case ']':
			raw := line[2:i]
			if len(raw) == 0 {
				return "", 0, false
			}
			return string(raw), i + 1, true
		case '[':
			return "", 0, false
		case '\\':
			// Skip the escaped character, so "\]" does not close — but an
			// escaped space is still whitespace to the label rule.
			if i+1 >= len(line) || isFootnoteLabelBreak(line[i+1]) {
				return "", 0, false
			}
			i++
		default:
			if isFootnoteLabelBreak(line[i]) {
				return "", 0, false
			}
		}
	}
	return "", 0, false
}

// isFootnoteLabelBreak reports whether a byte ends a footnote label
// without closing it: the whitespace micromark forbids inside one.
func isFootnoteLabelBreak(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// footnoteDefParser parses a "[^label]: content" definition block. It is
// modeled on goldmark's own footnote block parser (the indented
// continuation rules are CommonMark's, at four spaces), minus the
// relocation into a document-end list.
type footnoteDefParser struct{}

// Trigger implements parser.BlockParser.
func (*footnoteDefParser) Trigger() []byte { return []byte{'['} }

// Open implements parser.BlockParser.
func (*footnoteDefParser) Open(
	_ gast.Node, reader text.Reader, pc parser.Context,
) (gast.Node, parser.State) {
	line, segment := reader.PeekLine()
	offset := pc.BlockOffset()
	if offset < 0 {
		return nil, parser.NoChildren
	}
	label, after, ok := scanFootnoteLabel(line[offset:])
	if !ok {
		return nil, parser.NoChildren
	}
	pos := offset + after
	if pos >= len(line) || line[pos] != ':' {
		return nil, parser.NoChildren
	}
	pos++
	// Eat the whitespace run after the colon (micromark does), so
	// "[^1]:     x" opens a paragraph and not an indented code block.
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	registerFootnoteLabel(pc, label)
	node := &footnoteDefBlock{Label: label}
	// line is the padded line; reader positions are padding-adjusted.
	padding := segment.Padding
	if pos >= len(line) {
		reader.Advance(pos - padding)
		return node, parser.NoChildren
	}
	reader.AdvanceAndSetPadding(pos-padding, padding)
	return node, parser.HasChildren
}

// Continue implements parser.BlockParser: a blank line or a line indented
// by four spaces stays in the definition, anything else closes it.
func (*footnoteDefParser) Continue(_ gast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Continue | parser.HasChildren
	}
	childpos, padding := util.IndentPosition(line, reader.LineOffset(), 4)
	if childpos < 0 {
		return parser.Close
	}
	reader.AdvanceAndSetPadding(childpos, padding)
	return parser.Continue | parser.HasChildren
}

// Close implements parser.BlockParser.
func (*footnoteDefParser) Close(_ gast.Node, _ text.Reader, _ parser.Context) {}

// CanInterruptParagraph implements parser.BlockParser: a definition may
// follow a paragraph line, and another definition line, without a blank
// line between them.
func (*footnoteDefParser) CanInterruptParagraph() bool { return true }

// CanAcceptIndentedLine implements parser.BlockParser: four spaces make
// the line indented code, not a definition.
func (*footnoteDefParser) CanAcceptIndentedLine() bool { return false }

// footnoteRefParser parses a "[^label]" reference, and only when the
// document defines that label — an unmatched reference is literal text
// (measured: remark renders "a[^1]" with no definition as "a\[^1]").
// The '!' trigger keeps "![^1]" a '!' plus a reference instead of letting
// the image parser claim the bracket, like remark.
type footnoteRefParser struct{}

// Trigger implements parser.InlineParser.
func (*footnoteRefParser) Trigger() []byte { return []byte{'!', '['} }

// Parse implements parser.InlineParser.
func (*footnoteRefParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, segment := block.PeekLine()
	start := 0
	if len(line) > 0 && line[0] == '!' {
		start = 1
	}
	label, after, ok := scanFootnoteLabel(line[start:])
	if !ok || !footnoteLabelDefined(pc, label) {
		return nil
	}
	block.Advance(start + after)
	if start == 1 {
		parent.AppendChild(parent, gast.NewTextSegment(segment.WithStop(segment.Start+1)))
	}
	return &footnoteRefInline{Label: label}
}
