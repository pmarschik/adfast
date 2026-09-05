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

// escapedLinkifyBoundaries is the set of boundary bytes a backslash can
// escape in front of a bare URL literal: goldmark's own five plus the widened
// set punctLinkifyParser claims, minus the space (only ASCII punctuation is
// escapable) and minus ':' (colonURLParser owns that byte, and it is not in
// this chain).
//
// IT IS THE SAME SET AS THE UNESCAPED ONE ON PURPOSE. An escape must not
// decide whether a URL is a link, in either direction, so this set has to
// track the boundaries the literal spelling accepts rather than lag behind
// them: while it read "*_~(" alone, "a\.http://x" stayed prose after
// "a.http://x" had started linking, and the formatter's move of an escape
// would then have added or removed a link.
const escapedLinkifyBoundaries = "*_~(" + punctLinkifyBoundaries

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
	return newPunctLinkifyParser(
		newWWWCaseLinkifyParser(newLinkifyHostGate(extension.NewLinkifyParser(opts...))))
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

// punctLinkifyBoundaries is every ASCII punctuation byte this package claims
// as a left boundary for a bare URL literal. The omissions from the ASCII
// punctuation set fall into three groups, and only the last one is a gap.
//
// ALREADY OWNED, so claiming them here would be a second recognizer for a
// shape that already works:
//
//   - '(' '*' '_' '~' are goldmark's own trigger set, handled by the wrapped
//     parser at goldmark's own extent.
//   - ':' belongs to colonURLParser, which reads the same patterns. Two
//     parsers registered at one priority with one trigger byte run in an
//     unspecified order (goldmark sorts with sort.Slice, which is not
//     stable), so the byte has exactly one owner.
//   - '\\' belongs to escapedLinkifyParser, which advances past the escape
//     and delegates into this chain, so an escaped boundary reaches the rule
//     through that parser rather than around it.
//
// UNREACHABLE, because another parser answers first: '[' is the link
// parser's, and it returns the bracket as text rather than declining, so a
// parser behind it is never asked. "z[http://a.b" stays prose here and links
// in the reference.
//
// HELD BACK, and these are the gaps. '&' '#' ';' are the bytes of a CHARACTER
// REFERENCE, and adding a trigger at any of them changes how the document is
// ESCAPED, not just how it is linked. goldmark ends a line by appending its
// trailing run with parent.AppendChild rather than MergeOrAppendTextSegment
// (parser.parseBlock), so every trigger byte splits the text node there — and
// the renderer's character-reference rule reads the bytes around a '&' inside
// ONE text node, so a split it did not expect makes it defuse a reference that
// needs no defusing. Measured with '&' in this set, whole bodies:
//
//	"x&#x20;"     formatted "x\&#x20;"     (base: "x&#x20;", a fixpoint)
//	"***0*0**0"   formatted to a non-fixpoint, "**_&#x30;_&#x30;**&#x30;"
//	              re-rendering as "**_&#x30;_&#x30;**\&#x30;"
//
// '#' and ';' break the same way from the other two positions in "&#x20;".
// Closing this needs the escape rule to read the rendered output rather than
// the node it is inside, which is render_escape.go's business and not this
// parser's, so the three bytes stay out and "z&http://a.b" stays prose.
//
// '<' is held back for a different reason, and it is the one omission that
// trades one divergence for another rather than avoiding a regression.
// Source.Autolinks resolves an autolink's written extent from Node.Pos, which
// is the byte BEFORE the address, and it reads a '<' there as the ANGLE form —
// the form whose address must be closed by a '>'. Its doc states outright that
// this is sound BECAUSE no linkify parser triggers on '<'. Registering one
// makes "z<http://a.b c" build an AutoLink whose Pos is that '<' with no '>'
// behind the address, so the extent fails to resolve and the node is DROPPED
// from that view instead of reported. Measured both ways, whole bodies:
//
//	                  '<' in this set          '<' out (here)
//	"z<http://a.b c"  tree link, span view []  no link at all
//	                  UnlocatedAutolinks() 1   UnlocatedAutolinks() 0
//	"z<www.a.b c"     tree link, span view []  no link at all
//	                  UnlocatedAutolinks() 1   UnlocatedAutolinks() 0
//
// The reference links both — "z<http://a.b c" reports "http://a.b" at 2-12 —
// so NEITHER column matches it. The byte stays out because closing the gap
// properly is source_autolinks.go's business: that view has to tell the two
// written forms apart by something other than the byte at Pos before a parser
// may trigger here. TestBoundaryLiteralsAllResolveToASpan holds the line in the
// meantime — it fails on the unlocated count the moment a byte with this hazard
// is added.
const punctLinkifyBoundaries = "!\"$%'),+-./=>?@[]^`{|}"

// punctLinkifyParser linkifies a bare URL literal that follows an ASCII
// punctuation byte outside goldmark's five-byte trigger set.
//
// THE BOUNDARY IS A RULE ABOUT THE PREVIOUS CHARACTER, and the reference
// spells it three ways rather than one. Read out of the frozen install:
//
//   - micromark-extension-gfm-autolink-literal's tokenizer, for a SCHEMED
//     literal: `previousProtocol = !asciiAlpha(code)`. Every byte that is
//     not an ASCII letter opens one — punctuation, but digits and non-ASCII
//     letters too.
//   - the same tokenizer, for a "www." literal: `previousWww` is the closed
//     set `null | '(' | '*' | '_' | '[' | ']' | '~' | line ending | space`,
//     which is goldmark's five plus '[' and ']' — NARROWER than punctuation.
//   - mdast-util-gfm-autolink-literal's transform, for both: start of input,
//     Unicode whitespace, or Unicode punctuation.
//
// A literal links when ANY of the three accepts it, so the union is what a
// round trip has to reproduce. Swept over all 32 ASCII punctuation bytes in
// "z<punct>http://a.b c" and "z<punct>www.a.b c" the reference links 32 of 32
// in each; this package agreed on 12 of those 64 bodies before this parser
// and on 53 after it. The eleven that remain are the bytes
// punctLinkifyBoundaries names as owned or held back.
//
// WHAT IT DOES NOT REACH, and why that is not fixable here: goldmark
// dispatches an inline parser only at an unescaped ASCII punctuation byte, a
// space, or the line head (`util.IsPunct` is a byte test — see
// parser.parseBlock), so a boundary that is a DIGIT or a NON-ASCII character
// never reaches any Trigger at all. Measured, still divergent after this
// parser: "1http://a.b" and "参http://a.b" link in the reference (neither
// previous byte is an ASCII letter) and stay prose here, as does "a—www.a.b".
// That residual is a boundary CLASS this package cannot see, not a byte it
// chose to leave out. Closing it needs a scan outside goldmark's dispatch,
// which would report links in the tree that Source.Autolinks cannot see — the
// split escapedLinkifyParser exists to avoid.
//
// ACCEPTANCE IS THIS PACKAGE'S, not a fourth copy: urlLiteralCandidate runs
// the same two anchored patterns in goldmark's own order, urlLiteralHostAccepted
// applies the same domain rule, and trimURLLiteralEnd is goldmark's trailing
// trim spelled out. Only the byte in front of the address is new.
type punctLinkifyParser struct{ inner parser.InlineParser }

// newPunctLinkifyParser returns the widened-boundary linkify parser wrapping
// inner, whose triggers it also serves.
func newPunctLinkifyParser(inner parser.InlineParser) parser.InlineParser {
	return &punctLinkifyParser{inner: inner}
}

func (p *punctLinkifyParser) Trigger() []byte {
	return append([]byte(punctLinkifyBoundaries), p.inner.Trigger()...)
}

func (p *punctLinkifyParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	if n := p.parseAfterPunct(parent, block, pc); n != nil {
		return n
	}
	return p.inner.Parse(parent, block, pc)
}

// parseAfterPunct returns the autolink for a literal opening one byte after a
// widened boundary, nil for anything else. It never advances the reader on
// the nil path, so the caller may delegate straight afterwards.
func (*punctLinkifyParser) parseAfterPunct(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	if pc.IsInLinkLabel() {
		return nil
	}
	line, segment := block.PeekLine()
	if len(line) < 2 || strings.IndexByte(punctLinkifyBoundaries, line[0]) < 0 {
		return nil
	}
	rest := line[1:]
	m := urlLiteralCandidate(rest)
	if m == nil || !urlLiteralHostAcceptedAt(rest, len(m)) {
		return nil
	}
	lit := trimURLLiteralEnd(string(m))
	if lit == "" {
		return nil
	}
	// The boundary byte is not part of the address; it goes out as text,
	// exactly as goldmark emits its own trigger byte.
	gast.MergeOrAppendTextSegment(parent, segment.WithStop(segment.Start+1))
	block.Advance(1 + len(lit))
	start := segment.Start + 1
	addr := gast.NewTextSegment(text.NewSegment(start, start+len(lit)))
	link := gast.NewAutoLink(gast.AutoLinkURL, addr)
	if hasWWWPrefix([]byte(lit)) {
		// The scheme goldmark completes for its own www branch; without it
		// AutoLink.URL reports a schemeless, relative address.
		link.Protocol = []byte("http")
	}
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
