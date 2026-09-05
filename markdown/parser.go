package markdown

import (
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
	// The anchored pattern is the whole gate. A scheme pre-check used to sit
	// in front of it, spelled with three case-SENSITIVE byte prefixes, and
	// it silently narrowed this parser back below urlLiteralAnchoredRe: the
	// pattern accepts "HTTPS://" and the pre-check dropped it first.
	m := urlLiteralAnchoredRe.FindIndex(rest)
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
			//
			// The URL pattern is this package's own (see urlliteral.go),
			// because goldmark's is narrower than the reference's in two
			// measured ways: it tests the scheme case-sensitively, and it
			// requires a dotted host, so "HTTPS://EX.COM/x" and an intranet
			// "https://jira/browse/X" both parsed as prose.
			//
			// AllowedProtocols has to be given along with it. goldmark uses
			// that list only as a byte-prefix pre-gate before it runs the URL
			// pattern at all, and its default gate is the three schemes
			// spelled in lowercase — which would drop "HTTPS://" before the
			// pattern ever saw it. The first letter of each accepted scheme,
			// in both cases, is therefore the whole list: the pattern itself
			// is what decides the scheme.
			//
			// The SCHEME-LESS "www." pattern is this package's own for the
			// same reason, and it has to be passed here or the raw
			// recognizer disagrees with the rest of the package: goldmark's
			// stock pattern demands a SECOND dot after the prefix, so it
			// read "www.x" as prose where urlLiteralWWW — the pattern the
			// decoded-text scan already used — reads it as a literal. The
			// tree still ended up with the link, because relinkifyTexts put
			// it back; the RAW SPANS did not, and Source.Autolinks reports
			// nothing but goldmark's verdict. There is no WWW equivalent of
			// AllowedProtocols: goldmark's "www." pre-gate is hard-coded and
			// case-sensitive, which is the whole of the remaining "WWW."
			// divergence urlLiteralWWW records.
			extension.NewLinkify(
				extension.WithLinkifyEmailRegexp(gfmEmailRe),
				extension.WithLinkifyURLRegexp(urlLiteralAnchoredRe),
				extension.WithLinkifyWWWRegexp(urlLiteralWWWAnchoredRe),
				extension.WithLinkifyAllowedProtocols([][]byte{
					[]byte("h"), []byte("H"), []byte("f"), []byte("F"),
				}),
			),
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
				// Ahead of the paragraph parser (1000), which would
				// otherwise read "[^1]: note" as a link reference
				// definition (see footnote.go).
				util.Prioritized(&footnoteDefParser{}, 999),
			),
			parser.WithInlineParsers(
				util.Prioritized(&strictTaskCheckBoxParser{}, 0),
				// Ahead of goldmark's link parser (200).
				util.Prioritized(&footnoteRefParser{}, 101),
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
