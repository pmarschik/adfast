package markdown

import (
	gast "github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// This is goldmark's GFM strikethrough parser with one behavioral change:
// the closing delimiter run must have the SAME length as the opening run,
// matching micromark-extension-gfm-strikethrough (the remark parity
// reference). Goldmark's stock processor pairs any '~' runs, so "~~0~"
// parsed as text "~" + strike "0" while remark keeps it literal — see the
// tilde probes in the directive fixtures.

type matchedStrikethroughDelimiterProcessor struct{}

func (*matchedStrikethroughDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '~'
}

func (*matchedStrikethroughDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char && opener.Length == closer.Length
}

func (*matchedStrikethroughDelimiterProcessor) OnMatch(int) gast.Node {
	return east.NewStrikethrough()
}

var matchedStrikethroughProcessor = &matchedStrikethroughDelimiterProcessor{}

type matchedStrikethroughParser struct{}

// newStrikethroughParser returns the matched-run strikethrough inline parser.
func newStrikethroughParser() parser.InlineParser {
	return &matchedStrikethroughParser{}
}

func (*matchedStrikethroughParser) Trigger() []byte { return []byte{'~'} }

func (*matchedStrikethroughParser) Parse(_ gast.Node, block text.Reader, pc parser.Context) gast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 1, matchedStrikethroughProcessor)
	if node == nil || node.OriginalLength > 2 || before == '~' {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (*matchedStrikethroughParser) CloseBlock(gast.Node, parser.Context) {}
