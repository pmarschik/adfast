package markdown

import (
	"bytes"
	"regexp"

	directive "github.com/pmarschik/goldmark-directive"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// colonURLParser autolinks URLs immediately preceded by ':' (e.g.
// "link:https://..."). GFM's remark-gfm linkifier handles this case, but
// Goldmark's built-in linkify only triggers on whitespace and a few punctuation
// delimiters (space, *, _, ~, (). This parser fills the gap so that
// normalisation round-trips produce the same output as remark-gfm.
//
// Trigger fires on ':'. When ':' is immediately followed by https://, http://,
// or ftp://, the ':' is emitted as text and the URL is returned as an AutoLink.
type colonURLParser struct{}

// colonURLRegexp matches http/https/ftp URLs — same pattern as Goldmark's
// internal linkify urlRegexp in extension/linkify.go (path group made optional).
var colonURLRegexp = regexp.MustCompile(
	`^(?:https?|ftp)://[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+(?::\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`)

func (*colonURLParser) Trigger() []byte { return []byte{':'} }

func (*colonURLParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	if pc.IsInLinkLabel() {
		return nil
	}
	line, segment := block.PeekLine()
	// line[0] == ':'; check if a URL immediately follows.
	if len(line) < 2 {
		return nil
	}
	rest := line[1:]
	if !bytes.HasPrefix(rest, []byte("https://")) &&
		!bytes.HasPrefix(rest, []byte("http://")) &&
		!bytes.HasPrefix(rest, []byte("ftp://")) {
		return nil
	}
	m := colonURLRegexp.FindIndex(rest)
	if len(m) == 0 || m[0] != 0 {
		return nil
	}
	urlLen := m[1]
	// Emit the ':' as a text segment, then return the URL as an autolink.
	colonSeg := segment.WithStop(segment.Start + 1)
	gast.MergeOrAppendTextSegment(parent, colonSeg)
	// Advance past ':' + URL.
	block.Advance(1 + urlLen)
	urlStart := segment.Start + 1
	textNode := gast.NewTextSegment(text.NewSegment(urlStart, urlStart+urlLen))
	return gast.NewAutoLink(gast.AutoLinkURL, textNode)
}

// angleAutoLinkParser wraps goldmark's core autolink parser ('<url>' /
// '<mail@example.com>') and tags the produced node with the
// "angleAutoLink" attribute so consumers can distinguish angle-bracket
// autolinks from linkified bare URLs (prettier preserves the source form).
// Acceptance is identical to the core parser by construction (it delegates),
// and it registers just ahead of it so tagged nodes win.
type angleAutoLinkParser struct{ inner parser.InlineParser }

// newAngleAutoLinkParser returns the tagging autolink parser.
func newAngleAutoLinkParser() parser.InlineParser {
	return &angleAutoLinkParser{inner: parser.NewAutoLinkParser()}
}

func (*angleAutoLinkParser) Trigger() []byte { return []byte{'<'} }

func (p *angleAutoLinkParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	n := p.inner.Parse(parent, block, pc)
	if n != nil {
		n.SetAttributeString("angleAutoLink", true)
	}
	return n
}

// gfmEmailRe is the GFM autolink-literal email shape (micromark's
// restricted local part), anchored for goldmark's linkify extension. The
// domain is matched greedily (absorbing trailing digits so partial matches
// can't split "a@b.co1"); micromark's remaining constraints — a dot in the
// domain and a final LETTER (measured: "a@b.co1", "ab@0.0" stay text while
// "a@b.c_d" links) — are validated in adf's autolink conversion, which
// reverts invalid matches to plain text.
var gfmEmailRe = regexp.MustCompile(`^[a-zA-Z0-9.+_-]+@[a-zA-Z0-9_-]+(?:\.[a-zA-Z0-9_-]+)*`)

// NewParser returns a configured goldmark parser with GFM extensions and
// remark-directive-compatible directive support (container, leaf, and text
// directives via github.com/pmarschik/goldmark-directive).
func NewParser() parser.Parser {
	// GFM minus its stock strikethrough: we register a matched-run variant
	// (see strikethrough.go) at goldmark's own priority 500 instead.
	md := goldmark.New(
		goldmark.WithExtensions(
			// GFM (micromark) restricts email-autolink local parts to
			// [A-Za-z0-9._+-]; goldmark's default FindEmailIndex accepts the
			// larger RFC 5322 set (backtick, %, …), linkifying text remark
			// leaves alone. goldmark still enforces the ≥1-dot domain and
			// trailing-character rules on top of this pattern.
			extension.NewLinkify(extension.WithLinkifyEmailRegexp(gfmEmailRe)),
			extension.Table,
			// TaskList is replaced by strictTaskCheckBoxParser below —
			// goldmark accepts "[ ]" without following whitespace, where
			// GFM/micromark requires it ("[ ]()" is a LINK in remark).
		),
		goldmark.WithParserOptions(
			parser.WithBlockParsers(
				util.Prioritized(directive.NewDirectiveParser(), 50),
				util.Prioritized(directive.NewCloseFenceParser(), 55),
				util.Prioritized(directive.NewLeafDirectiveParser(), 60),
			),
			parser.WithInlineParsers(
				util.Prioritized(&strictTaskCheckBoxParser{}, 0),
				util.Prioritized(newAngleAutoLinkParser(), 299),
				util.Prioritized(newStrikethroughParser(), 500),
				util.Prioritized(directive.NewTextDirectiveParser(NewParser), 800),
				util.Prioritized(&colonURLParser{}, 999),
			),
		),
	)
	return md.Parser()
}

// strictTaskCheckBoxRe requires whitespace (or end of line) after the
// checkbox, like micromark; goldmark's own parser accepts "[ ]()".
var strictTaskCheckBoxRe = regexp.MustCompile(`^\[([\sxX])\](?:[ \t]|$)`)

// strictTaskCheckBoxParser is goldmark's task-checkbox inline parser with
// the GFM whitespace-after rule.
type strictTaskCheckBoxParser struct{}

func (*strictTaskCheckBoxParser) Trigger() []byte { return []byte{'['} }

func (*strictTaskCheckBoxParser) Parse(parent gast.Node, block text.Reader, _ parser.Context) gast.Node {
	// A checkbox is only valid as the very first inline of the first
	// block in a list item (mirrors goldmark's checks).
	if parent.Parent() == nil || parent.Parent().FirstChild() != parent {
		return nil
	}
	if parent.HasChildren() {
		return nil
	}
	if _, ok := parent.Parent().(*gast.ListItem); !ok {
		return nil
	}
	line, _ := block.PeekLine()
	m := strictTaskCheckBoxRe.FindSubmatchIndex(line)
	if m == nil {
		return nil
	}
	value := line[m[2]:m[3]][0]
	// Consume the checkbox and one following space/tab (not the newline).
	adv := m[3] + 1
	if adv < len(line) && (line[adv] == ' ' || line[adv] == '\t') {
		adv++
	}
	block.Advance(adv)
	return extast.NewTaskCheckBox(value == 'x' || value == 'X')
}
