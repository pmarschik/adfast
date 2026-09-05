package markdown

import (
	"bytes"
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
// Trigger fires on ':'. When ':' is immediately followed by a bare URL literal
// — schemed or scheme-less "www." — the ':' is emitted as text and the literal
// is returned as an AutoLink.
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
	// The anchored patterns are the whole gate. A scheme pre-check used to
	// sit in front of them, spelled with three case-SENSITIVE byte prefixes,
	// and it silently narrowed this parser back below urlLiteralAnchoredRe:
	// the pattern accepts "HTTPS://" and the pre-check dropped it first.
	//
	// BOTH SHAPES, not the schemed one alone. urlLiteralCandidate runs the
	// scheme-less "www." pattern when the schemed one finds nothing, in
	// goldmark's own order. Reading only the schemed pattern left this byte
	// half-served — measured against the frozen reference, "z:www.a.b c"
	// comes back as "z:[www.a.b](http://www.a.b) c" while "z:http://a.b c"
	// already linked here.
	m := urlLiteralCandidate(rest)
	if m == nil {
		return nil
	}
	// The host gate is the second half of the pattern (see
	// urlLiteralHostAccepted): a host with an underscore in either of its
	// last two segments is not a literal at all, here as in the reference.
	if !urlLiteralHostAcceptedAt(rest, len(m)) {
		return nil
	}
	// The literal ends where the parser would end it, not where the pattern
	// does (see trimURLLiteralEnd).
	lit := trimURLLiteralEnd(string(m))
	if lit == "" {
		return nil
	}
	urlLen := len(lit)
	// Emit the ':' as a text segment, then return the URL as an autolink.
	colonSeg := segment.WithStop(segment.Start + 1)
	gast.MergeOrAppendTextSegment(parent, colonSeg)
	// Advance past ':' + URL.
	block.Advance(1 + urlLen)
	urlStart := segment.Start + 1
	textNode := gast.NewTextSegment(text.NewSegment(urlStart, urlStart+urlLen))
	link := gast.NewAutoLink(gast.AutoLinkURL, textNode)
	if hasWWWPrefix([]byte(lit)) {
		// The scheme goldmark completes for its own www branch; without it
		// AutoLink.URL reports a schemeless, relative address.
		link.Protocol = []byte("http")
	}
	return link
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
// to a linkify parser configured identically to the one NewParser registers —
// host gate included, so an escape cannot buy a literal the unescaped
// spelling is refused.
func newEscapedLinkifyParser(opts ...extension.LinkifyOption) parser.InlineParser {
	return &escapedLinkifyParser{inner: newLinkifyRecognizer(opts...)}
}

// newLinkifyRecognizer builds the linkify parser this package registers: the
// stock one, the host gate in front of it, and the mixed-case "www." branch in
// front of that. Spelled once because two registrations need it — the one
// NewParser makes and the second instance escapedLinkifyParser delegates to —
// and an escape must not decide whether a URL is a link, so both need the same
// stack.
func newLinkifyRecognizer(opts ...extension.LinkifyOption) parser.InlineParser {
	return newWWWCaseLinkifyParser(newLinkifyHostGate(extension.NewLinkifyParser(opts...)))
}

// linkifyBoundaryBytes is goldmark's linkify trigger set. Its Parse advances
// one byte past any of these before it applies a URL pattern, so the gate
// below has to read its candidate from the same place. The set is spelled as
// bytes rather than reused from escapedLinkifyBoundaries because that one is
// deliberately the SMALLER set — the escapable subset — and the two answer
// different questions.
const linkifyBoundaryBytes = " *_~("

// linkifyHostGate refuses a bare-URL literal whose host micromark's domain
// production rejects — an underscore in either of the last two dot-separated
// segments — and delegates every other decision to the linkify parser it
// wraps. See urlLiteralHostAccepted for the rule and for why it cannot be a
// pattern.
//
// IT LOOKS BEFORE THE INNER PARSER RUNS rather than undoing afterwards.
// goldmark's linkify appends the boundary character as a text segment on its
// success path, through MergeOrAppendTextSegment, which may EXTEND the
// preceding Text node instead of appending a child — so a rejection taken
// after the fact has nothing clean to undo. Reading the same line with the
// same two patterns and returning nil first leaves the reader untouched, and
// returning nil is what the inner parser itself does for an address it
// refuses.
//
// A CANDIDATE THAT IS NEITHER PATTERN IS PASSED THROUGH, not refused: the
// inner parser's third branch is the EMAIL literal, whose host rule is its
// own (goldmark's `-`/`_` tail test plus adf's validation) and not this one.
type linkifyHostGate struct{ inner parser.InlineParser }

// newLinkifyHostGate wraps a linkify parser with the host gate.
func newLinkifyHostGate(inner parser.InlineParser) parser.InlineParser {
	return &linkifyHostGate{inner: inner}
}

func (p *linkifyHostGate) Trigger() []byte { return p.inner.Trigger() }

func (p *linkifyHostGate) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, _ := block.PeekLine()
	if len(line) > 0 && strings.IndexByte(linkifyBoundaryBytes, line[0]) >= 0 {
		line = line[1:]
	}
	if m := urlLiteralCandidate(line); m != nil && !urlLiteralHostAcceptedAt(line, len(m)) {
		return nil
	}
	return p.inner.Parse(parent, block, pc)
}

// wwwCaseLinkifyParser linkifies a scheme-less "www." literal whose prefix is
// not spelled in lowercase, which goldmark's linkify extension cannot reach.
//
// THE PREFIX IS A CASE RULE IN THE REFERENCE, and it was a case-sensitive byte
// compare here. Measured against the frozen reference, whole bodies:
//
//	"see WWW.ex.com b"   ref links "http://WWW.ex.com"   here prose
//	"see WWW.x b"        ref links "http://WWW.x"        here prose
//	"see Www.Ex.Com b"   ref links "http://Www.Ex.Com"   here prose
//
// Both of the reference's recognizers fold the prefix: micromark's tokenizer
// lowercases before it tests, and mdast-util-gfm-autolink-literal's transform
// matches `www.` with /i. So this is a link-versus-text divergence with a
// payload difference — a push sent prose where the reference sends a link —
// and the renderer's own defusing rule already treated the uppercase spelling
// as dangerous ("see WWW.ex.com b" rendered "see WWW\.ex.com b", an escape
// for a link that never formed).
//
// WHY GOLDMARK CANNOT: its linkify parser reaches the WWW pattern only through
// `if m == nil && bytes.HasPrefix(line, domainWWW)`, where domainWWW is the
// package-level `[]byte("www.")`. There is no option for it, unlike the
// schemed branch's AllowedProtocols pre-gate, which linkifyOptions already
// opens for both cases. So the pattern can be widened all it likes and the
// stock parser will never be handed a "WWW." to run it on.
//
// IT CLAIMS ONLY WHAT THE PRE-GATE REFUSES. A lowercase "www." is handed
// straight to the wrapped parser, so every literal that linked before still
// takes goldmark's own path, at goldmark's own extent. That is what keeps this
// from being a second recognizer: the pattern (urlLiteralWWWAnchoredRe), the
// end-trim (trimURLLiteralEnd, which is goldmark's own trim spelled out) and
// the host gate (urlLiteralHostAcceptedAt) are the ones the rest of the
// package runs.
//
// ONE REGISTRATION, NOT TWO, and that is deliberate rather than tidy.
// goldmark sorts its inline parsers with sort.Slice, which is not stable, so
// two parsers registered at the same priority with the same trigger set would
// run in an unspecified order. Wrapping the gate puts the two in a fixed one.
type wwwCaseLinkifyParser struct{ inner parser.InlineParser }

// newWWWCaseLinkifyParser wraps a linkify parser with the mixed-case "www."
// branch.
func newWWWCaseLinkifyParser(inner parser.InlineParser) parser.InlineParser {
	return &wwwCaseLinkifyParser{inner: inner}
}

func (p *wwwCaseLinkifyParser) Trigger() []byte { return p.inner.Trigger() }

func (p *wwwCaseLinkifyParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	if n := p.parseMixedCaseWWW(parent, block, pc); n != nil {
		return n
	}
	return p.inner.Parse(parent, block, pc)
}

// parseMixedCaseWWW returns the autolink for a "www." literal goldmark's
// lowercase pre-gate refuses, nil for anything else. It never advances the
// reader on the nil path, so the caller may delegate straight afterwards.
func (*wwwCaseLinkifyParser) parseMixedCaseWWW(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	if pc.IsInLinkLabel() {
		return nil
	}
	line, segment := block.PeekLine()
	consumes, start := 0, segment.Start
	// goldmark's own boundary step: the trigger byte is not part of the
	// address, and the literal begins one byte later. A line head has no
	// boundary byte to skip.
	if len(line) > 0 && strings.IndexByte(linkifyBoundaryBytes, line[0]) >= 0 {
		consumes, start, line = 1, start+1, line[1:]
	}
	// The lowercase spelling is goldmark's, and it keeps it. Only the
	// spellings the pre-gate drops are this parser's.
	if !hasWWWPrefix(line) || bytes.HasPrefix(line, urlLiteralWWWPrefix) {
		return nil
	}
	// A schemed literal wins over the scheme-less one, exactly as it does in
	// goldmark's own order — "wwW.x" is not a scheme, but the check costs
	// nothing and keeps the two orders one order.
	if urlLiteralAnchoredRe.Match(line) {
		return nil
	}
	m := urlLiteralWWWAnchoredRe.Find(line)
	if m == nil || !urlLiteralHostAcceptedAt(line, len(m)) {
		return nil
	}
	lit := trimURLLiteralEnd(string(m))
	if lit == "" {
		return nil
	}
	if consumes != 0 {
		gast.MergeOrAppendTextSegment(parent, segment.WithStop(segment.Start+1))
	}
	block.Advance(consumes + len(lit))
	addr := gast.NewTextSegment(text.NewSegment(start, start+len(lit)))
	link := gast.NewAutoLink(gast.AutoLinkURL, addr)
	// The scheme goldmark completes for its own www branch. AutoLink.URL
	// prepends it, which is what makes Source.Autolinks report the same
	// Target shape for both spellings.
	link.Protocol = []byte("http")
	return link
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
				// LINKIFY IS REGISTERED HERE rather than through
				// extension.NewLinkify, at that extension's own priority and
				// with the same options: the gate has to wrap the parser, and
				// the extension exposes no seam to wrap it through. Its
				// Extend does nothing else — one AddOptions with one inline
				// parser at 999 — so this registration is the whole of it.
				// See linkifyOptions for why every pattern here is this
				// package's own, and newLinkifyRecognizer for what wraps it.
				util.Prioritized(newLinkifyRecognizer(linkifyOptions()...), 999),
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
