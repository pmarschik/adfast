package markdown

import (
	"regexp"
	"strings"

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

// escapedLinkifyBoundaries is the subset of goldmark's linkify trigger set
// that a backslash can escape: its Trigger() is ' ', '*', '_', '~', '(' and
// only ASCII punctuation is escapable, so the space drops out. A boundary
// NOT in this set stays out on purpose — the escape must not WIDEN the
// verdict, only leave it where the literal character puts it. "a\.http://x"
// therefore keeps linking nothing, exactly as "a.http://x" does.
const escapedLinkifyBoundaries = "*_~("

// escapedLinkifyParser linkifies a bare URL that follows a backslash escape,
// which goldmark's own linkify extension cannot reach.
//
// AN ESCAPE MUST NOT DECIDE WHETHER A URL IS A LINK, and before this parser
// it did — in both directions, so the md→md formatter changed meaning either
// way it moved an escape:
//
//	"a\_http://0"  parsed as plain text, and the formatter DROPS the "\_"
//	               ('_' is outside PreservedEscapes), so the re-parse of
//	               "a_http://0" linkified and the format INVENTED a link.
//	"0*httP://0"   parsed WITH the link, and the formatter ADDS the "\*",
//	               so the re-parse of "0\*httP://0" saw none and the format
//	               DELETED a link.
//
// The reference implementation links all four spellings: micromark's
// gfm-autolink-literal tokenizer reads the RAW SOURCE, and a character
// escape in front of a literal does not stop it.
//
// WHY GOLDMARK CANNOT: its inline dispatch loop skips every parser at an
// ESCAPED punctuation byte (`if (isPunct && !escaped) || …` in
// parser.parseBlock), so linkify — which triggers on the boundary character
// itself — is never called at the '_' of "a\_http://0". The backslash IS
// dispatched, because it is unescaped punctuation, and that is the position
// this parser claims.
//
// WHY NOT relinkifyTexts, the decoded-text scan that already rescues a URL
// goldmark skipped: the information is gone by the time it runs. '_' and '*'
// are outside PreservedEscapes, so the parse decodes them away and leaves
// ast.Text.Raw EMPTY — "a\_http://0" and "a_http://0" arrive there as the
// byte-identical Value "a_http://0" with no provenance to tell them apart.
// That scan also runs on the tree only: Source.Autolinks reports goldmark's
// verdict and nothing else, so a link it added was invisible to every caller
// driving off raw spans (measured: "a\_http://a.b" linked in the tree and
// reported NO autolink span). Fixing it at the parser fixes both views.
//
// ACCEPTANCE IS goldmark's LINKIFY, unchanged and delegated to, so the raw
// host rule, the scheme set, the "www." branch and the trailing-punctuation
// trim cannot drift from the unescaped spelling's. The reader is advanced
// past the backslash so the inner parser sees the boundary character at the
// head of its line, exactly as it would with no escape.
type escapedLinkifyParser struct{ inner parser.InlineParser }

// newEscapedLinkifyParser returns the post-escape linkify parser, delegating
// to a linkify parser configured identically to the one NewParser registers.
func newEscapedLinkifyParser(opts ...extension.LinkifyOption) parser.InlineParser {
	return &escapedLinkifyParser{inner: extension.NewLinkifyParser(opts...)}
}

func (*escapedLinkifyParser) Trigger() []byte { return []byte{'\\'} }

func (p *escapedLinkifyParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, segment := block.PeekLine()
	// "\X" plus at least one byte of URL. line[0] is the backslash by
	// construction of Trigger.
	if len(line) < 3 || strings.IndexByte(escapedLinkifyBoundaries, line[1]) < 0 {
		return nil
	}
	escStart := segment.Start
	savedLine, savedPos := block.Position()
	// Put the reader on the escaped boundary character, which is where the
	// inner parser expects to start.
	block.Advance(1)
	n := p.inner.Parse(parent, block, pc)
	if n == nil {
		// The inner parser appends its boundary text segment only on the
		// success path, so nothing has to be undone but the position.
		block.SetPosition(savedLine, savedPos)
		return nil
	}
	// The inner parser emitted the boundary character alone as text. Replace
	// that segment with one covering the whole "\X" escape, so the escape
	// survives into ast.Text.Raw (PreservedEscapes) and the source offsets
	// stay contiguous — without this the backslash byte would vanish from
	// the tree's coverage of the line.
	if last, ok := parent.LastChild().(*gast.Text); ok && last.Segment.Start == escStart+1 {
		parent.RemoveChild(parent, last)
		gast.MergeOrAppendTextSegment(parent, last.Segment.WithStart(escStart))
	}
	// Point Pos at the address itself. goldmark's dispatch would otherwise
	// stamp the BACKSLASH's offset, and autolinkExtent — which accepts only
	// the address's own offset or the one byte before it, and verifies the
	// source spells the address there — would drop the node, leaving
	// Source.Autolinks silent about a link the tree carries.
	if p, ok := n.(interface{ SetPos(int) }); ok {
		p.SetPos(escStart + 2)
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

// linkifyOptions is the linkify configuration, spelled once because TWO
// parsers run it: the extension NewParser registers, and the post-escape
// escapedLinkifyParser that delegates to a second instance of the same
// parser. Sharing the options is what makes "a\_http://x" and "a_http://x"
// one verdict rather than two.
//
// GFM (micromark) restricts email-autolink local parts to [A-Za-z0-9._+-];
// goldmark's default FindEmailIndex accepts the larger RFC 5322 set
// (backtick, %, …), linkifying text remark leaves alone. goldmark still
// enforces the ≥1-dot domain and trailing-character rules on top of this
// pattern.
//
// The URL pattern is this package's own (see urlliteral.go), because
// goldmark's is narrower than the reference's in two measured ways: it tests
// the scheme case-sensitively, and it requires a dotted host, so
// "HTTPS://EX.COM/x" and an intranet "https://jira/browse/X" both parsed as
// prose.
//
// AllowedProtocols has to be given along with it. goldmark uses that list
// only as a byte-prefix pre-gate before it runs the URL pattern at all, and
// its default gate is the three schemes spelled in lowercase — which would
// drop "HTTPS://" before the pattern ever saw it. The first letter of each
// accepted scheme, in both cases, is therefore the whole list: the pattern
// itself is what decides the scheme.
//
// The SCHEME-LESS "www." pattern is this package's own for the same reason,
// and it has to be given here or the raw recognizer disagrees with the rest
// of the package: goldmark's stock pattern demands a SECOND dot after the
// prefix, so it read "www.x" as prose where urlLiteralWWW — the pattern the
// decoded-text scan already used — reads it as a literal. The tree still
// ended up with the link, because relinkifyTexts put it back; the RAW SPANS
// did not, and Source.Autolinks reports nothing but goldmark's verdict.
// There is no WWW equivalent of AllowedProtocols: goldmark's "www." pre-gate
// is hard-coded and case-sensitive, which is the whole of the remaining
// "WWW." divergence urlLiteralWWW records.
func linkifyOptions() []extension.LinkifyOption {
	return []extension.LinkifyOption{
		extension.WithLinkifyEmailRegexp(gfmEmailRe),
		extension.WithLinkifyURLRegexp(urlLiteralAnchoredRe),
		extension.WithLinkifyWWWRegexp(urlLiteralWWWAnchoredRe),
		extension.WithLinkifyAllowedProtocols([][]byte{
			[]byte("h"), []byte("H"), []byte("f"), []byte("F"),
		}),
	}
}

// NewParser returns a configured goldmark parser with GFM extensions and
// remark-directive-compatible directive support (container, leaf, and text
// directives via github.com/pmarschik/goldmark-directive).
func NewParser() parser.Parser {
	// GFM minus its stock strikethrough: we register a matched-run variant
	// (see strikethrough.go) at goldmark's own priority 500 instead.
	md := goldmark.New(
		goldmark.WithExtensions(
			// See linkifyOptions for why every pattern here is this
			// package's own.
			extension.NewLinkify(linkifyOptions()...),
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
				// The only parser triggering on '\', so its priority is
				// free; it is spelled at linkify's own 999 because it is
				// linkify, one dispatch position earlier.
				util.Prioritized(newEscapedLinkifyParser(linkifyOptions()...), 999),
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
