package markdown

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"

	"github.com/yuin/goldmark/util"
)

// Link serialization and the CommonMark flanking checks that drive
// remark's character-reference encoding around emphasis markers. Split
// from render.go.

// ---------------------------------------------------------------------------
// CommonMark flanking (for remark's character-reference encoding)
// ---------------------------------------------------------------------------

// emphasisMarkerByte returns the delimiter byte for constructs whose markers
// have flanking restrictions ('_' emphasis, '*' strong). Strikethrough runs
// are intraword-legal in GFM and remark never encodes around them.
func emphasisMarkerByte(node ast.Node) byte {
	switch node.(type) {
	case *ast.Emphasis:
		return '_'
	case *ast.Strong:
		return '*'
	case *ast.Delete:
		return '~'
	}
	return 0
}

// emphasisMarkerAfter returns the delimiter byte the renderer will actually
// put around the construct at nodes[i] when the rune prev precedes it, which
// is emphasisMarkerByte's answer for everything except an emphasis whose '_'
// would not flank where it sits. It is 0 for a node that is not a mark.
//
// '_' has no intraword form in CommonMark, so an emphasis with a word
// character on either side cannot be written with it at all. The repair the
// rest of this file implements — hex-encode the neighboring rune so the
// marker gets the punctuation it needs — then reaches for three plain
// letters: "a*b*c" went out as "&#x61;_&#x62;_&#x63;". That is valid and it
// round-trips, which is why nothing caught it, but it is unreadable in a file
// an author is expected to hand-edit, and partial-word emphasis is ordinary
// in prose.
//
// Both references answer it the way the spec intends, by using the delimiter
// that HAS an intraword form. Measured on "a*b*c": prettier 3.8.1 (md -> md)
// writes "a*b*c", and mdast-util-to-markdown, given the same emphasis mark,
// writes "a*b*c". prettier reaches it by choosing '*' when a word touches the
// emphasis and '_' otherwise; mdast-util-to-markdown by preferring '*'
// outright.
//
// So '*' is taken only where '_' would otherwise force the encoding and '*'
// is clean on BOTH sides — an emphasis '_' can carry keeps it (prettier's
// spaced "a _b_ c" is unchanged), and an emphasis neither delimiter can
// carry keeps the existing repair rather than trading one bad marker for
// another. "Clean" is more than flanking: see asteriskRunMerges.
func (r *mdRenderer) emphasisMarkerAfter(nodes []ast.Node, i int, prev rune, st *inlineContext) byte {
	marker := emphasisMarkerByte(nodes[i])
	if marker != '_' {
		return marker
	}
	next := siblingLeadRune(nodes, i+1)
	tail := textSiblingRun(nodes, i+1)
	// Each candidate is measured against ITS OWN rendering: the marker is
	// the preceding rune for the children, and since the choice made here is
	// what a nested emphasis reads as that rune, '_' and '*' do not always
	// render the same content. See renderedChildLead.
	for _, cand := range [2]byte{'_', '*'} {
		content := r.renderChildScratch(nodes[i], cand, st)
		lead := firstRuneOf(content)
		trail := lastRuneOf(content)
		if !canOpenMarker(cand, prev, lead) || !canCloseMarker(cand, trail, next) {
			continue
		}
		if cand == '*' && asteriskRunMerges(prev, lead, trail, next) {
			continue
		}
		if closerLinkifies(cand, content, tail) {
			continue
		}
		return cand
	}
	return '_'
}

// closerLinkifies reports whether the CLOSING marker written after content
// would be swallowed by a bare-URL literal when the output is parsed again.
//
// An underscore is both a byte goldmark's linkify extension triggers on and a
// byte its host class accepts, so an emphasis whose content ends inside a
// half-finished literal can have its closer annexed by the address. Measured
// on the frozen prettier 3.8.1 install, 2026-09-05, on the format leg:
//
//	"*www.*..A"        prettier _www._..A     adfast (before) _www._..A
//	"*www.*.a.A"       prettier _www._.a.A    adfast (before) _www._.a.A
//	"*https://ex.*.a.A" prettier _https://ex._.a.A
//
// The two agree on the bytes, and for prettier that is a fixpoint: its
// pre-CommonMark parser has no autolink literals, so "_www._..A" is still an
// emphasis to it. The parse adfast round-trips against DOES linkify, and it
// reads "www._..A" as one host — micromark's domain rule only refuses an
// underscore in the last two dot-separated segments, and an EMPTY segment
// resets that counter, so the reference's parser links these too. So the
// second format pass emitted "\_[www.\_..A](http://www._..A)": the emphasis
// was gone and a link the author never wrote was in its place. This is a
// DELIBERATE divergence from prettier's bytes of the same kind escapeAt
// already makes for '@' (see linkifiesAsEmail) — the format leg is written
// for the parser that reads its output back, and silently deleting an
// author's emphasis is not a byte nit.
//
// Only '_' is asked about. A '*' or '~' closer is a linkify TRIGGER too, but
// neither is in the host class, so a literal always stops in front of it —
// which is exactly why swapping the delimiter repairs the case at all.
//
// The check reads the RENDERED content, so it is self-canceling on the
// remark leg: an escape written there ("www\.") is not a "www." prefix any
// more and no candidate matches, which keeps the remark-stringify byte pins
// untouched without a mode flag. And it applies the parser's own trailing
// trim, so a closer the address gives back — "www.x" plus '_' matches
// "www.x_", which trims to "www.x" and leaves the '_' outside — is not a
// hazard and keeps its preferred delimiter.
func closerLinkifies(marker byte, content, tail string) bool {
	if marker != '_' || content == "" {
		return false
	}
	s := content + string(marker) + tail
	for p := 0; p <= len(content); p++ {
		// p == 0 sits right after the OPENING marker, itself a trigger byte.
		if p > 0 && !urlLiteralOpensAfter(s[p-1]) {
			continue
		}
		rest := []byte(s[p:])
		m := urlLiteralCandidate(rest)
		if m == nil || !urlLiteralHostAcceptedAt(rest, len(m)) {
			continue
		}
		if lit := trimURLLiteralEnd(string(m)); lit != "" && p+len(lit) > len(content) {
			return true
		}
	}
	return false
}

// urlLiteralOpensAfter reports whether a bare-URL literal may begin one byte
// after c: goldmark's own trigger set plus the widened ASCII-punctuation
// boundary punctLinkifyParser adds, and a line ending. It is the render leg's
// reading of the same rule those two parsers apply.
func urlLiteralOpensAfter(c byte) bool {
	return c == '\n' ||
		strings.IndexByte(linkifyBoundaryBytes, c) >= 0 ||
		strings.IndexByte(punctLinkifyBoundaries, c) >= 0
}

// textSiblingRun returns the plain values of the text nodes that start at
// nodes[i], stopping at the first sibling that is not one. It is the lookahead
// closerLinkifies needs: a host runs on past a marker into the text behind it,
// and every other inline kind opens with a bracket or a marker byte that ends
// the host anyway.
func textSiblingRun(nodes []ast.Node, i int) string {
	var b strings.Builder
	for ; i < len(nodes); i++ {
		t, ok := nodes[i].(*ast.Text)
		if !ok {
			break
		}
		b.WriteString(t.Value)
	}
	return b.String()
}

// asteriskRunMerges reports whether an emphasis written with '*' here would
// have one of its markers swallowed by a neighboring asterisk.
//
// canOpenMarker and canCloseMarker answer a FLANKING question — what CLASS
// the runes on either side of the marker fall into — and an asterisk is
// punctuation, so they are perfectly happy next to one. But the parser scans
// a maximal run of the delimiter character FIRST and only then asks about
// flanking, so a '*' marker touching another '*' is not a marker of its own
// length at all: the two fuse into a single longer run that pairs off
// differently. '_' never met this, because it is a different character from
// the '*' that strong is always written with, and because the neighbors that
// could have been asterisks were being hex-encoded away by the repair this
// choice exists to avoid.
//
// Found by FuzzRoundTripIdempotent on "0*0****0*0***", whose emphasis holds a
// trailing strong and is followed by another: taking '*' wrote
// "0*0**0*****0**\*", where the strong's closer, the emphasis's closer and
// the next strong's opener are one run of five asterisks. It re-parsed as a
// different tree, so a second render did not reproduce it. The '~~' handling
// in writeWrapped guards the same class of fusion for strikethrough.
//
// The four neighbors are the ones the flanking checks already use, so a
// rejection here only ever falls back to the '_' the renderer wrote before.
func asteriskRunMerges(prev, lead, trail, next rune) bool {
	return prev == '*' || lead == '*' || trail == '*' || next == '*'
}

func flankWS(r rune) bool {
	return r == 0 || unicode.IsSpace(r)
}

func flankPunct(r rune) bool {
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// canOpenMarker/canCloseMarker implement CommonMark's left/right-flanking
// rules for a delimiter run with the given adjacent runes.
func canOpenMarker(marker byte, prev, next rune) bool {
	leftFlanking := !flankWS(next) && (!flankPunct(next) || flankWS(prev) || flankPunct(prev))
	if marker == '*' || marker == '~' {
		return leftFlanking
	}
	rightFlanking := !flankWS(prev) && (!flankPunct(prev) || flankWS(next) || flankPunct(next))
	return leftFlanking && (!rightFlanking || flankPunct(prev))
}

func canCloseMarker(marker byte, prev, next rune) bool {
	rightFlanking := !flankWS(prev) && (!flankPunct(prev) || flankWS(next) || flankPunct(next))
	if marker == '*' || marker == '~' {
		return rightFlanking
	}
	leftFlanking := !flankWS(next) && (!flankPunct(next) || flankWS(prev) || flankPunct(prev))
	return rightFlanking && (!leftFlanking || flankPunct(next))
}

// needsPunctTrail returns the rune that would fuse onto a text directive
// rendered at nodes[i], or 0 when nodes[i+1] is harmless there.
// writeTextDirectiveForm answers a hazard by emitting the semantically
// inert `{}` — `:name{}` and `:name` decode to the same node for every
// registered kind — which both terminates the directive token and ends
// the form in punctuation.
//
// This is a DELIBERATE divergence from mdast-util-directive, which emits
// the bare form unconditionally and is unstable on every input below.
//
// Two hazards, both of which leave the round trip non-idempotent:
//
//   - Token fusion. The bare form ends in a name rune, and the name
//     grammar keeps running: `:media` before "0" renders ":media0", which
//     re-parses as the (unregistered) directive "media0" and degrades to
//     escaped text. A text node backslash-escapes '_', '[' and '{', so
//     only the unescapable continuations are hazards there; syntax runes
//     from other kinds — a link's '[' — are not escaped at all. A '{'
//     fuses onto the LABELED form too (it is read as that directive's
//     attribute block), which is why the hazard rune is reported rather
//     than a bare yes/no.
//
//   - Emphasis flanking. remark repairs a non-flankable marker by
//     hex-encoding one of the two runes touching it (writeWrapped's
//     encodeLead, which leaves a '&#xNN;' reference starting with '&'),
//     or by encoding a preceding TEXT node's tail. Neither reaches a
//     preceding directive, whose tail is its name — encoding that would
//     rename the directive. `*:media!*` renders ":media_!_", where the
//     '_' cannot open after "a", so it re-parses as literal text.
func (r *mdRenderer) needsPunctTrail(nodes []ast.Node, i int, st *inlineContext) rune {
	if i+1 >= len(nodes) {
		return 0
	}
	next := nodes[i+1]
	// The delimiter the emphasis will be written with, not the preferred
	// one: an emphasis that falls back to '*' after a word rune (see
	// emphasisMarkerAfter) opens straight after the directive's name and
	// needs no attribute block to separate them.
	if marker := r.emphasisMarkerAfter(nodes, i+1, directiveTailStandIn, st); marker != 0 {
		if r.markerNeedsPunctBefore(marker, next, st) {
			return rune(marker)
		}
		return 0
	}
	text, isText := next.(*ast.Text)
	lead := nodeLeadRune(next)
	// The formatter adds no escapes of its own — it writes the source
	// form the parse captured (ast.Text.Raw, which normalization has
	// already moved onto Value here) — so a '[' or '_' the source left
	// bare stays bare and fuses onto the name (probe: ":media[\n]"
	// formatted to ":media[ ]", whose label swallowed the text).
	escapable := isText && st.escape && !r.cfg.prettierText
	if isText && r.hexEncodesLead(text, nodes, i+1, st) {
		return 0
	}
	nextLead := nextTextLead(nodes, i+1)
	colonEscaped := isText && r.escapesLeadColon(text, nextLead, st)
	underscoreEscaped := isText && st.escape && r.escapesLeadUnderscore(text, nextLead, st)
	if fusesOntoDirectiveName(lead, escapable, colonEscaped, underscoreEscaped) {
		return lead
	}
	return 0
}

// hexEncodesLead reports whether the emphasis repair will replace the
// leading rune of the text node at nodes[j] with a '&#xNN;' reference.
// The reference starts with '&', which is punctuation: it cannot fuse
// onto the directive name, and it ends the form's neighborhood in
// punctuation, so both hazards needsPunctTrail exists for are already
// answered and the empty attribute block must not go out on top.
//
// Emitting it anyway is unstable, the same way the colon and underscore
// cases are: the block appears on one format and not the next. The fuzzer
// found it on "*:*0*0*0", where the first format writes
// "_:0&#x30;_&#x30;" (goldmark reads the bare source as one text node
// ":00", with no directive in it) and a second format of that output —
// which now DOES parse a ":0" directive — added the block, giving
// "_:0{}&#x30;_&#x30;".
//
// Only the encode that reaches the LEAD counts, so the node has to be a
// single encodable rune: writeTextInline's repair replaces the trailing
// rune, and the two are the same rune only then. The lead-side repair
// (writeWrapped's encodeLead) never reaches here — a directive form
// clears it as it writes, because hex-encoding a directive's own tail
// would rename it.
func (r *mdRenderer) hexEncodesLead(text *ast.Text, nodes []ast.Node, j int, st *inlineContext) bool {
	if text == nil {
		return false
	}
	lead := firstRuneOf(text.Value)
	if lead == 0 || len(text.Value) != len(string(lead)) {
		return false
	}
	if !isEncodableRune(lead, r.cfg.noSpaceEscapes) {
		return false
	}
	// The two places writeTextInline turns the trail encode on: the
	// enclosing construct asked for it (closeProblem, and only the last
	// child carries it), or the next sibling is an emphasis marker this
	// rune would leave unopenable.
	if st.encodeTrail && j == len(nodes)-1 {
		return true
	}
	if j+1 < len(nodes) {
		if m := r.emphasisMarkerAfter(nodes, j+1, lead, st); m != 0 {
			return !canOpenMarker(m, lead, r.renderedChildLead(nodes[j+1], m, st))
		}
	}
	return false
}

// escapesLeadUnderscore reports whether the renderer's own underscore
// escape will separate a following text node from the directive name just
// written. Prettier is not escape-free about '_': escapeUnderscore writes
// "\_" at a word boundary, which is exactly where a directive name ends,
// so the escape often already does the separating that the empty
// attribute block would otherwise be emitted for.
//
// Emitting the block anyway is not merely redundant, it is unstable, in
// the same way the colon case documents: the block goes out on one format
// and not on the next. The fuzzer found it on "0:0_", where the first
// format writes "0:0\_" (goldmark reads the bare source as one text node,
// with no directive in it at all) and a second format of that output —
// which now DOES parse a ":0" directive — added the block, giving
// "0:0{}\_".
//
// It asks escapeUnderscore the question escapeUnderscore will be asked at
// render time, with the same directive-tail stand-in escapesLeadColon
// uses: the character before the '_' is the directive's own tail, which is
// a name rune, ']' or '}'. Only "is it a word byte" is asked of it, and
// that is settled — if the block does not go out the tail is the name
// rune, which is one.
func (r *mdRenderer) escapesLeadUnderscore(text *ast.Text, nextLead byte, st *inlineContext) bool {
	if text == nil || !strings.HasPrefix(text.Value, "_") {
		return false
	}
	after := *st
	after.prev, after.hasPrev = directiveTailStandIn, true
	return r.escapeUnderscore(text.Value, 0, nextLead, &after)
}

// escapesLeadColon reports whether the renderer's own colon escape will
// separate a following text node from the directive name just written:
// ":media" before ":A" renders ":media\:A", which reads back as the
// directive plus the text, with no help from an attribute block.
//
// It asks escapesColon the question escapesColon will be asked at render
// time, with the one piece of context that is already settled here: the
// character before the colon is the directive's own tail, which is a name
// rune, ']' or '}' — never a colon and never a newline, so which of the
// three it turns out to be cannot change the answer. That the tail is not
// yet decided is the reason for the stand-in: whether the attribute block
// goes out is what this call is deciding.
func (r *mdRenderer) escapesLeadColon(text *ast.Text, nextLead byte, st *inlineContext) bool {
	after := *st
	after.prev, after.hasPrev = directiveTailStandIn, true
	return r.escapesColon(text.Value, 0, nextLead, &after)
}

// directiveTailStandIn stands for the last character a directive form
// writes, for the escape questions that only care that it is neither a
// colon nor a newline. See escapesLeadColon.
const directiveTailStandIn = 'x'

// markerNeedsPunctBefore asks whether an emphasis marker's opener is
// unsalvageable at a word-class predecessor — no encodeLead assignment
// makes it flank — while a punctuation predecessor would work. writeWrapped
// still applies encodeLead on top once the predecessor changes.
func (r *mdRenderer) markerNeedsPunctBefore(marker byte, next ast.Node, st *inlineContext) bool {
	lead := r.renderedChildLead(next, marker, st)
	leads := []rune{lead}
	if isEncodableRune(lead, r.cfg.noSpaceEscapes) {
		leads = append(leads, '&') // what encodeLead would leave in its place
	}
	worksSomewhere := false
	for _, l := range leads {
		if canOpenMarker(marker, 'a', l) {
			return false // a word-class predecessor is already fine
		}
		worksSomewhere = worksSomewhere || canOpenMarker(marker, '.', l)
	}
	return worksSomewhere
}

// fusesOntoDirectiveName reports whether rune r, emitted directly after a
// bare `:name`, is read as part of that directive token on re-parse.
// escapable says the rune comes from a text node, which backslash-escapes
// the punctuation members of the set (goldmark-directive's name grammar is
// alphanumerics plus '-'/'_' runs; '[' opens a label and '{' an attribute
// block).
func fusesOntoDirectiveName(r rune, escapable, colonEscaped, underscoreEscaped bool) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '-':
		// A backslash before '-' is dropped by the renderer, so a text
		// node cannot separate this one either.
		return true
	case r == '_':
		// Prettier keeps an INTRAWORD underscore bare and escapes the
		// rest, so the escape has to be asked for rather than assumed
		// away with the rest of prettier's (see escapesLeadUnderscore).
		return !escapable && !underscoreEscaped
	case r == '[':
		return !escapable
	case r == '{':
		// Unlike '_' and '[', a brace is not in remark's escape set, so a
		// text node emits it raw: ":media" before "{ }" renders
		// ":media{ }", whose brace block is read as the directive's
		// (empty) attributes and the text is lost.
		return true
	case r == ':':
		// goldmark-directive does not open a bare text directive whose
		// name butts straight into a following colon, so ":media:u[x]"
		// and ":media:" both lose the first directive. The colon escape
		// covers only a colon that leads into a name (`\:x`) and only in
		// a text node, so ":media" before ":9", before a bare ":", or
		// before another directive is still a hazard.
		//
		// Where the escape does fire, the attribute block is not just
		// redundant but wrong: it goes out on the first render and not on
		// the second, because the escape it duplicates has by then become
		// part of the text the second parse reads back. ":media[]:A"
		// formatted to ":media{}\:A" and then to ":media\:A".
		return !colonEscaped
	}
	return false
}

func lastRuneOf(s string) rune {
	r := rune(0)
	for _, c := range s {
		r = c
	}
	return r
}

func firstRuneOf(s string) rune {
	for _, c := range s {
		return c
	}
	return 0
}

// nodeLeadRune returns the first rune of a node's rendered output
// (markers/syntax included); 0 when the node renders nothing.
func nodeLeadRune(node ast.Node) rune {
	switch n := node.(type) {
	case *ast.Text:
		return firstRuneOf(n.Value)
	case *ast.InlineCode:
		return '`'
	case *ast.Break:
		return '\\'
	case *ast.Emphasis:
		return '_'
	case *ast.Strong:
		return '*'
	case *ast.Delete:
		return '~'
	case *ast.Link:
		return linkLeadRune(n)
	case *ast.Image:
		// Its own marker, not its alt text: the walk below would have
		// answered with the first rune INSIDE the brackets, which is
		// what "![0](p.png)" after a directive turned into a "{}" that
		// went out on the second format and not the first.
		return '!'
	case *ast.TextDirective:
		return ':'
	case extension.InlineLead:
		return rune(n.MarkdownLead())
	}
	for _, child := range ast.Children(node) {
		if r := nodeLeadRune(child); r != 0 {
			return r
		}
	}
	return 0
}

// childrenLeadRune is the first rendered rune INSIDE a construct — the
// character right after its opening marker.
// renderedChildLead returns the first rune the construct's children will
// actually render — inner constructs may hex-encode their boundary runes
// ("0" becomes "&#x30;"), which changes the flanking class the re-parser
// sees. Rendering into a scratch context has no side effects.
//
// marker is the delimiter byte the construct is being written with, which is
// the rune its children see as their predecessor. It used to be a hardcoded
// '_' stand-in, and that was sound while the flanking checks were the only
// readers of it: they classify the predecessor as whitespace, punctuation or
// neither, and every marker byte ('_', '*', '~') is punctuation, so which one
// it was could not change an answer. emphasisMarkerAfter reads the byte
// itself — a '*' predecessor is the one thing that rules '*' out — so the
// stand-in now has to be the real marker or the scratch render disagrees with
// the real one about which delimiter a nested emphasis gets. On
// "***0*0**0" it did: the scratch trail said '0' where the render wrote ';',
// the strong's closer was then judged flankable when it was not, and the
// round trip lost the marks (found by FuzzRoundTripIdempotent).
func (r *mdRenderer) renderedChildLead(node ast.Node, marker byte, st *inlineContext) rune {
	return firstRuneOf(r.renderChildScratch(node, marker, st))
}

// renderedChildTrail is renderedChildLead's counterpart for the last rune.
func (r *mdRenderer) renderedChildTrail(node ast.Node, marker byte, st *inlineContext) rune {
	return lastRuneOf(r.renderChildScratch(node, marker, st))
}

// renderChildScratch renders a construct's children into a throwaway builder
// under the state they would have inside it.
func (r *mdRenderer) renderChildScratch(node ast.Node, marker byte, st *inlineContext) string {
	var tmp strings.Builder
	// afterLead is the marker for the same reason prevRune is: the scratch has
	// to write the bytes the real pass will write, and writeWrapped threads the
	// closing marker to the last child. Without it the scratch misses every
	// escape that depends on what follows — "www." came out unescaped here and
	// "www\." in the real render, and closerLinkifies read the wrong one.
	child := inlineContext{
		escape:         st.escape,
		colons:         st.colons,
		pipes:          st.pipes,
		prevRune:       rune(marker),
		afterLead:      marker,
		directiveLabel: st.directiveLabel,
	}
	r.writeInlines(&tmp, ast.Children(node), &child)
	return tmp.String()
}

// siblingLeadRune is the first rune rendered by nodes[i] (0 = end of the
// phrasing run, which flanking treats as whitespace).
func siblingLeadRune(nodes []ast.Node, i int) rune {
	if i >= len(nodes) {
		return 0
	}
	return nodeLeadRune(nodes[i])
}

// linkLeadRune is the first rune writeLink puts down for this link. It asks
// autolinkText rather than repeating its test, so the two can never
// disagree about which links reach the autolink form.
func linkLeadRune(node *ast.Link) rune {
	if _, ok := autolinkText(node); ok {
		return '<'
	}
	return '['
}

// isEncodableRune reports whether remark-stringify would hex-encode this
// rune next to a non-flankable emphasis marker: alphanumerics make the
// marker intraword, whitespace makes it non-flanking outright.
// noSpaceEscapes excludes the whitespace half, which is what
// WithoutSignificantSpaceEscapes gives up — the word-class half is not
// negotiable, since without it the marker beside it stops being a marker.
func isEncodableRune(r rune, noSpaceEscapes bool) bool {
	// Word-class neighbors break emphasis flanking and get hex-encoded.
	// The class mirrors the flanking checks exactly (anything neither
	// whitespace nor punctuation — alphanumerics, combining marks, format
	// characters …), so every detected open/close problem is fixable; the
	// boundary space is the remaining encodable case.
	if r == 0 || r == '\n' {
		return false
	}
	if unicode.IsSpace(r) {
		return !noSpaceEscapes
	}
	return !flankPunct(r)
}

// hexRef renders a character reference the way remark-stringify does
// (lowercase x, uppercase hex digits).
func hexRef(r rune) string {
	return fmt.Sprintf("&#x%X;", r)
}

// autolinkText returns the link's plain-text label when it can render as an
// autolink: a single text child matching the URL (including mailto: links
// where the label is the bare email address) on a non-explicit link with an
// autolinkable URL.
func autolinkText(node *ast.Link) (string, bool) {
	if len(node.Children) != 1 || node.Explicit || !autolinkableURL(node.URL) {
		return "", false
	}
	t, ok := node.Children[0].(*ast.Text)
	if !ok {
		return "", false
	}
	text := t.Value
	isMailto := strings.HasPrefix(node.URL, "mailto:")
	mailtoAddr := strings.TrimPrefix(node.URL, "mailto:")
	if text == node.URL {
		return text, true
	}
	// The mailto: form emits the bare ADDRESS, not the URL, so the URI
	// grammar above is not the grammar that has to accept it: an address
	// the email autolink parser stops short of would come back as plain
	// text and the render would not be a fixpoint.
	if isMailto && text == mailtoAddr && autolinkableEmail(mailtoAddr) {
		return text, true
	}
	return "", false
}

// bareWWWLiteral returns the source spelling of a linkified "www." literal.
//
// It is the one bare form whose URL is NOT its text: the parser completes it
// with the scheme goldmark's own www branch prepends (see the literal
// conversion), so "www.x" arrives carrying "http://www.x" and misses
// autolinkText's text-equals-url gate. The format leg then wrote it out as
// "[www.x](http://www.x)" — link syntax the author never typed, in ordinary
// prose like "see www.example.com for details".
//
// Measured 2026-09-05 on the frozen prettier 3.8.1 install with the parity
// flags: "see www.x b", "z-www.a.b c" and "a www.x.y, b" are all fixpoints
// there, while an EXPLICIT link keeps its brackets, so the literal spelling is
// the reference answer and not a shortcut that loses the link.
//
// Only the format leg takes it. The other reference, mdast-util-to-markdown,
// really does bracket a www literal — "see www.x b" comes back as
// "see [www.x](http://www.x) b" — so the remark leg keeps that, and the two
// legs are measured against the tool each one is a port of.
func bareWWWLiteral(node *ast.Link) (string, bool) {
	if !node.Bare || node.Explicit || len(node.Children) != 1 {
		return "", false
	}
	t, ok := node.Children[0].(*ast.Text)
	if !ok || !hasWWWPrefix([]byte(t.Value)) {
		return "", false
	}
	if node.URL != "http://"+t.Value {
		return "", false
	}
	return t.Value, true
}

func (r *mdRenderer) writeLink(b *strings.Builder, node *ast.Link, st *inlineContext) {
	// Auto-link: emit <url> angle-bracket form when the label is plain text
	// matching the URL (including mailto: links where the label is the bare
	// email address).
	if text, ok := autolinkText(node); ok {
		if node.Bare {
			// Linkified bare URL: prettier keeps the source form.
			b.WriteString(text)
			return
		}
		b.WriteByte('<')
		b.WriteString(text)
		b.WriteByte('>')
		return
	}
	// A linkified "www." literal: prettier keeps the source spelling, so the
	// format leg does too. See bareWWWLiteral.
	if r.cfg.prettierText {
		if text, ok := bareWWWLiteral(node); ok {
			b.WriteString(text)
			return
		}
	}
	// The whole [label](url) construct is one unbreakable wrap unit
	// (prettier moves it wholly to the next line), so its spaces are
	// masked here and restored at the end of render.
	var link strings.Builder
	link.WriteString("[")
	// Link labels are written verbatim (no markdown or colon escaping),
	// matching the previous hand-rolled renderer. Table-cell pipe escaping
	// still applies inside labels and destinations (mdast-util-gfm-table).
	// afterLead ']' lets end-of-label escapes see the closing bracket
	// (remark's tracker does: a trailing backslash escapes before ']').
	child := inlineContext{pipes: st.pipes, escape: r.cfg.prettierText, label: true, afterLead: ']'}
	r.writeInlines(&link, node.Children, &child)
	link.WriteString("](")
	url := formatLinkURL(node.URL, r.cfg.prettierText)
	if st.pipes {
		url = strings.ReplaceAll(url, "|", "\\|")
	}
	link.WriteString(url)
	if node.Title != "" {
		link.WriteString(r.titleSegment(node.Title))
	}
	link.WriteString(")")
	masked := strings.ReplaceAll(link.String(), " ", string(wrapMask))
	b.WriteString(strings.ReplaceAll(masked, "\t", string(wrapMaskTab)))
}

// autolinkableURLRe is CommonMark's absolute-URI autolink grammar (scheme
// then no whitespace or angle brackets).
var autolinkableURLRe = regexp.MustCompile("^[A-Za-z][A-Za-z0-9+.-]{1,31}:[^\x00-\x20<>]*$")

// autolinkableURL reports whether the <url> shortcut form would re-parse as
// an autolink (a text-equals-url link with an invalid URL must keep the
// [label](url) form).
func autolinkableURL(url string) bool {
	return autolinkableURLRe.MatchString(url)
}

// autolinkableEmail reports whether <addr> would re-parse as an email
// autolink. The parser's own scanner answers, rather than a restatement of
// CommonMark's email grammar, so the two cannot drift: the angle parser
// takes the address up to util.FindEmailIndex and then demands a '>' there,
// so the whole address has to be consumed. A domain label opening on '_' or
// '-' is the case this rejects ("00@0._AA"): goldmark's linkify pass will
// still take it as a GFM literal, and remark stringifies it as an autolink
// and then cannot read it back either, gaining a bracket pair per render.
func autolinkableEmail(addr string) bool {
	return util.FindEmailIndex([]byte(addr)) == len(addr)
}

// formatLinkURL serializes a link/image destination. Prettier wraps it in
// angle brackets when it contains a space or ')'; remark-stringify instead
// backslash-escapes parentheses and uses angle brackets only for whitespace.
func formatLinkURL(url string, prettier bool) string {
	if prettier {
		if strings.ContainsAny(url, " )") {
			return "<" + url + ">"
		}
		return url
	}
	if strings.ContainsAny(url, " \t\n") {
		// Inside an angle destination a backslash or angle bracket would
		// change the parse; remark escapes them (probe: "[0](< \\>)").
		var sb strings.Builder
		sb.WriteByte('<')
		for i := range len(url) {
			if url[i] == '\\' || url[i] == '<' || url[i] == '>' {
				sb.WriteByte('\\')
			}
			sb.WriteByte(url[i])
		}
		sb.WriteByte('>')
		return sb.String()
	}
	if strings.ContainsAny(url, "()\\") {
		var sb strings.Builder
		for i := range len(url) {
			if url[i] == '(' || url[i] == ')' || url[i] == '\\' {
				sb.WriteByte('\\')
			}
			sb.WriteByte(url[i])
		}
		return sb.String()
	}
	return url
}

// nextTextLead returns the first byte of the next sibling text node, used
// for the colon-escape lookahead when a ':' ends the current text node
// (remark tracks "after" context across node boundaries).
func nextTextLead(nodes []ast.Node, i int) byte {
	if i+1 < len(nodes) {
		return peekLead(nodes[i+1])
	}
	return 0
}

// peekLead is the first byte a construct will emit — mdast-util-to-markdown
// exposes the same via each handler's peek() and feeds it to the previous
// sibling's safety checks as the "after" character.
func peekLead(node ast.Node) byte {
	switch n := node.(type) {
	case *ast.Text:
		if n.Value != "" {
			return n.Value[0]
		}
		return 0
	case *ast.HTML:
		if n.Value != "" {
			return n.Value[0]
		}
		return 0
	case *ast.Emphasis, *ast.Strong, *ast.Delete:
		return emphasisMarkerByte(node)
	case *ast.Link, *ast.FootnoteRef, *ast.LinkRef:
		return '['
	case *ast.Image, *ast.ImageRef:
		return '!'
	case *ast.InlineCode:
		return '\x60'
	case *ast.TextDirective:
		return ':'
	case extension.InlineLead:
		return n.MarkdownLead()
	}
	return 0
}

// formatCodeSpan wraps text in the shortest backtick fence that does not
// appear as a run inside it, padding with a space when the content begins or
// ends with a backtick, or when writing it verbatim would lose its edge
// bytes to the parser's trim — mdast-util-to-markdown's inline-code rules
// (so `0“0` round-trips instead of merging with a neighboring span).
func formatCodeSpan(s string) string {
	runs := map[int]bool{}
	cur := 0
	for i := range len(s) {
		if s[i] == '`' {
			cur++
		} else if cur > 0 {
			runs[cur] = true
			cur = 0
		}
	}
	if cur > 0 {
		runs[cur] = true
	}
	n := 1
	for runs[n] {
		n++
	}
	fence := strings.Repeat("`", n)
	pad := ""
	if s != "" && (s[0] == '`' || s[len(s)-1] == '`' || codeSpanTrims(s)) {
		pad = " "
	}
	return fence + pad + s + pad + fence
}

// codeSpanTrims reports whether a code span holding s verbatim loses one
// byte from each end on re-parse, so the content needs a pad byte there
// to survive.
//
// goldmark trims when both edge bytes are a space or a newline and the
// content is not blank — and its blank test counts a tab and a carriage
// return as whitespace, which CommonMark's "consists entirely of space
// characters" does not. A span holding " \t " therefore keeps its spaces
// on re-parse, and padding it would grow a space per format pass instead
// of round-tripping (probe: "` \t `0", whose format grew one).
func codeSpanTrims(s string) bool {
	return isCodeSpanEdgeSpace(s[0]) && isCodeSpanEdgeSpace(s[len(s)-1]) && !isBlankRun(s)
}

// isCodeSpanEdgeSpace reports whether c is one of the two bytes goldmark
// trims from a code span's edges (parser.isSpaceOrNewline).
func isCodeSpanEdgeSpace(c byte) bool {
	return c == ' ' || c == '\n'
}

// isBlankRun reports whether s is entirely whitespace by goldmark's
// reckoning (util.IsBlank: space, tab, newline, carriage return).
func isBlankRun(s string) bool {
	return strings.Trim(s, " \t\n\r") == ""
}
