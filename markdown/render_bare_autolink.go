package markdown

import (
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pmarschik/adfast/ast"
)

// linkSite is where in its sibling run a link is being written: the run
// itself, the link's index in it, and the offset in the output builder where
// the run's own output starts.
//
// writeLink needs it for one question only — whether a bare autolink, put
// down between the bytes that actually surround it, still parses back as
// the same link. See bareAutolinkRoundTrips.
type linkSite struct {
	nodes []ast.Node
	i     int
	start int
}

// bareAutolinkRoundTrips reports whether writing the autolink text bare at
// this position re-parses as the same tree the angle-bracket form <text>
// re-parses as. The remark leg asks it before dropping the brackets.
//
// A bare autolink has no delimiters of its own: the parser decides where it
// starts and ends from the bytes around it, with GFM's literal boundary
// rules, and those rules know nothing about where the ADF link mark began
// and ended. ADF carries no "was it written in angle brackets" bit, so the
// ADF leg marks every http(s) and bare-address mailto link Bare (see
// isBareAutolinkScheme in convert), and before this check the renderer
// wrote all of them bare. Measured on that renderer, ADF built from:
//
//	"<http://>"      -> "http://"      re-parses as text, then "http\://"
//	"<http://0*>"    -> "http://0*"    re-parses as link "http://0" + "*"
//	"x<http://a.b>"  -> "xhttp://a.b"  re-parses as text, the link is lost
//	"<http://a.b>x"  -> "http://a.bx"  re-parses as link "http://a.bx"
//	"a<b@c.d>"       -> "ab@c.d"       re-parses as link "mailto:ab@c.d"
//	"<b@c.d>-x"      -> "b@c.d-x"      re-parses as link "mailto:b@c.d-x"
//	"*<a@b.co>*-x"   -> "_a@b.co_-x"   re-parses as link "mailto:a@b.co_-x"
//
// Those that retarget are worse than a non-idempotent render: the link's
// target silently changes.
//
// THE PARSER ANSWERS, NOT A RESTATEMENT OF ITS RULES. The literal boundary
// rules are spread across goldmark's linkify, the email scanner, and this
// package's punctLinkifyParser, escapedLinkifyParser and colonURLParser, and
// the relinkification goldmarkToAst does afterwards; a second hand-written
// notion of "what a reparse links" would drift from them, and patching
// isBareAutolinkScheme for one input only moves the fuzzer to the next one.
// So the check builds the text the reparse will actually see around the
// link and parses it with Parse, twice: once with the link bare and once
// with the same link in angle brackets. The angle form is always safe —
// '<' and '>' are ordinary punctuation that no neighbor can swallow — so it
// is the reference: bare is taken only when the two trees are identical
// apart from the form bit (ast.Link.Bare). One comparison covers the link's
// own end ("http://0*"), a neighbor in the window merging into it, and the
// link changing a neighbor's parse.
//
// THE WINDOW IS THE WHITESPACE-DELIMITED WORD the link sits in, as the
// renderer will write it, stopped early at a NEIGHBORING AUTOLINK'S OWN '<'
// (see autolinkWallRune) OR at bareAutolinkWindowCap bytes, whichever comes
// first. No URL or email literal spans whitespace, and goldmark's linkify
// triggers fire after whitespace, so the bytes beyond the nearest whitespace
// on either side cannot move the link's boundaries. The word crosses
// emphasis, strong and strikethrough markers: the left half is this run's
// output back to its last whitespace, extended when there is none by
// inlineContext.wordBefore (the enclosing runs' output through the opening
// marker); the right half is rendered ahead (see wordAhead), and extended
// the same way by inlineContext.wordAfter. A text directive's label, a
// table cell and the block's own start and end close the word: the parser
// reads a label between its brackets, and a run there starts with empty
// edges, as a block does.
//
// A whitespace-free word can still hold many autolinks — bold links joined
// only by punctuation is a common Jira/Confluence shape — and each one's
// check used to walk the window all the way to the nearest whitespace,
// rendering or rescanning every sibling link in between; across N autolinks
// in one word that is quadratic (measured: a 56KB realistic document took
// minutes). A neighboring autolink's own angle bracket ends the window just
// as whitespace does, so the scan need not, and must not, cross it — but
// that alone is not enough: a long run of autolinks that each legitimately
// round-trip bare writes NO angle bracket anywhere, so the look-behind would
// still walk back through the whole rendered document so far looking for a
// wall that is never there (measured: this was still quadratic, a 4000-link
// chain of all-bare links took minutes, after the '<'-stop alone). The byte
// cap is the bound that actually holds regardless of what neighbors decide:
// past bareAutolinkWindowCap bytes with no wall found, bareAutolinkPrefix and
// wordAhead give up rather than keep scanning, and bareAutolinkRoundTrips
// treats a give-up as "not safe" — the same answer as a real mismatch, never
// a different one, so this never OKs a bare form the full window would have
// rejected; it can only keep brackets the full window would have dropped.
// Forcing angle on a give-up also plants a '<' at that point, which is
// itself a wall for the NEXT autolink's own scan, so the giving-up itself
// keeps the total cost across a long run linear rather than letting it
// re-pay the same unbounded scan at every link.
//
// The guarantee is as wide as that window and no wider. Two known gaps: a
// flanking peek made from outside a construct (renderChildScratch) cannot
// see the window and assumes the angle form; and a neighbor peeking at a
// link sibling directly (linkLeadRune) assumes the bare form, so
// "*a*<http://>" re-renders as "_a_<http://>" — same tree, but not a
// byte fixpoint.
//
// The look-ahead writes later bare autolinks in angle form rather than
// checking them too (bareProbe), so a link followed in the same word by
// another one is judged against "<...>" there, and keeps its brackets where
// the second link would in fact have gone bare and the pair would have
// round-tripped: "<http://a.b>*<http://c.d>*" renders
// "<http://a.b>_http://c.d_" where the all-bare "http://a.b*http://c.d*"
// would also have come back. That costs a bracket, never a link.
//
// Only the remark leg asks. The format leg keeps whatever form the author
// wrote, which the source already proved parses, and is measured against
// prettier, which never adds brackets the author did not write.
func (r *mdRenderer) bareAutolinkRoundTrips(b *strings.Builder, text string, st *inlineContext, site linkSite) bool {
	prefix, ok := bareAutolinkPrefix(b.String()[site.start:], st.wordBefore, !st.wordBeforeCapped)
	if !ok {
		return false
	}
	after := *st
	after.prev, after.hasPrev = ')', true
	after.prevRune, after.encodeLead = lastRuneOf(text), false
	suffix, ok := r.wordAhead(site.nodes, site.i+1, after)
	if !ok {
		return false
	}
	bare := unmaskWrap(prefix + text + suffix)
	angle := unmaskWrap(prefix + "<" + text + ">" + suffix)
	return sameTreeIgnoringLinkForm(Parse([]byte(bare)), Parse([]byte(angle)))
}

// autolinkWallRune reports whether r is a byte neither a URL nor an email
// literal can be read across, from either side: whitespace, which no
// linkify grammar in this package or goldmark's own crosses, and '<'. '<' is
// not in any linkify character class (http(s), www or email), and the
// remark leg always backslash-escapes a literal one in phrasing text (see
// escapeAngle) — a raw '<' is therefore always the opening bracket of an
// angle-form autolink, one a neighbor is already known not to swallow (see
// the "angle form is always safe" note above). That makes it as hard a stop
// as whitespace for the bare-autolink window's look-behind
// (bareAutolinkPrefix, below, uses this predicate directly): a neighboring
// autolink in the same whitespace-free word is either written in angle form
// for real, or — ahead of the link under test — forced into it by wordAhead's
// own probe, so there is never anything further back, past a preceding
// link's own '<', that the window still needs.
//
// It is not, on its own, a bound on how far either side has to look: a long
// run of autolinks that each legitimately round-trip bare writes no '<' at
// all, so the look-behind would walk back through the whole document with
// nothing to stop it. bareAutolinkWindowCap is the bound that holds
// regardless; this predicate only decides where, within that bound, a real
// wall was found.
//
// wordAhead (the look-ahead) stops at the same '<', but — unlike here —
// keeps the rest of the sibling node that wrote it rather than cutting at
// the byte; see wordAhead for why that side needs the whole node.
//
// '>', the angle form's CLOSING bracket, is deliberately not included. It is
// escaped only at a line start (escapesLineStart), not everywhere '<' is, and
// goldmark's own URL and www path-and-query character classes accept it
// mid-literal — so a '>' a neighbor wrote as ordinary text, or as part of an
// already-open literal's path, cannot be trusted to end anything. Only '<'
// carries the guarantee in both directions.
func autolinkWallRune(r rune) bool {
	return unicode.IsSpace(r) || r == '<'
}

// bareAutolinkWindowCap bounds, in bytes, how far bareAutolinkPrefix and
// wordAhead look for a wall before giving up on this side of the window —
// see the "A whitespace-free word can still hold many autolinks" paragraph
// on bareAutolinkRoundTrips for why a wall-only stop is not enough on its
// own. Generous for any realistic run of markers and short punctuation
// between autolinks (the Jira/Confluence shape this guards is many SHORT
// links joined by a character or two, not one long word), small enough that
// giving up costs only a bounded, constant-size scan.
const bareAutolinkWindowCap = 256

// runReachesWindowStart reports whether the backslash run immediately
// before src[pos] extends back to src[0] with no other byte in between —
// including the vacuous case pos == 0, where there is no byte of src left
// to rule out a backslash just outside it. escapedAt counts that same run
// but only ever reports its parity; this reports whether the count could be
// incomplete, which matters only when src was truncated (see its call site
// in bareAutolinkPrefix, the only place a backslash run can be cut off
// rather than genuinely start there).
func runReachesWindowStart(src []byte, pos int) bool {
	i := pos - 1
	for i >= 0 && src[i] == '\\' {
		i--
	}
	return i < 0
}

// bareAutolinkPrefix returns the left half of the window: the run's output
// from its last whitespace (exclusive) or a neighboring autolink's own
// opening '<' (inclusive, autolinkWallRune — unless that '<' is itself
// backslash-escaped, in which case it is exclusive like whitespace; see
// escapedAt), found within the last bareAutolinkWindowCap bytes of written;
// or, when the run has written neither within that distance and has not
// itself exceeded the cap, all of it preceded by what the enclosing runs
// wrote of the same word (before, valid only when beforeOK). ok is false
// when no wall was found before the cap was reached, on this side or (via
// beforeOK) an enclosing one: the caller must then treat the window as
// unproven, not substitute a truncated one.
func bareAutolinkPrefix(written, before string, beforeOK bool) (prefix string, ok bool) {
	tail := written
	capped := false
	if len(tail) > bareAutolinkWindowCap {
		tail = tail[len(tail)-bareAutolinkWindowCap:]
		for tail != "" && !utf8.RuneStart(tail[0]) {
			tail = tail[1:]
		}
		capped = true
	}
	if i := strings.LastIndexFunc(tail, autolinkWallRune); i >= 0 {
		return bareAutolinkWallPrefix(tail, i, capped)
	}
	if capped || !beforeOK || len(before)+len(written) > bareAutolinkWindowCap {
		return "", false
	}
	return before + written, true
}

// bareAutolinkWallPrefix decides what bareAutolinkPrefix returns once
// autolinkWallRune has matched tail[i] — split out so that function's own
// nesting stays flat (nestif).
func bareAutolinkWallPrefix(tail string, i int, capped bool) (prefix string, ok bool) {
	r, size := utf8.DecodeRuneInString(tail[i:])
	if r != '<' {
		return tail[i+size:], true
	}
	// escapedAt's parity only counts the backslashes still inside tail. When
	// the window was capped, a run that reaches all the way back to tail's
	// own start (runReachesWindowStart) may continue further left, outside
	// the window, where a byte this call never sees could flip odd to even
	// or back — see TestBareLinkFromADFKeepsAngleWhenEscapeRunCrossesTheWindowCap,
	// where a long escaping run reads with the wrong parity inside a capped
	// window and the '<' is wrongly kept. Treat that as unproven, same as
	// finding no wall at all before the cap.
	if capped && runReachesWindowStart([]byte(tail), i) {
		return "", false
	}
	if !escapedAt([]byte(tail), i, 0) {
		// Unlike whitespace, the wall itself is kept: a neighboring
		// angle-form autolink's opening '<' is part of what makes that
		// neighbor safe. Cutting past it (as whitespace is cut past)
		// drops the '<' and leaves only the neighbor's inside text in
		// the window, which can read as a different, bare-autolink-safe
		// literal than the real "<...>" ever was — see
		// TestBareLinkFromADFKeepsAngleWhenBareWouldNotRoundTrip.
		return tail[i:], true
	}
	// A backslash-escaped '<' (an odd run of '\' right before it, see
	// escapedAt in heading_anchor.go) is not a wall at all: the renderer
	// wrote it as a literal "\<", not as an autolink's opening bracket,
	// so keeping it would plant a real '<' in the window the actual
	// output never had — see
	// TestBareLinkFromADFKeepsAngleWhenBareWouldNotRoundTrip's escaped case.
	return tail[i+size:], true
}

// wordAhead returns the right half of a window: what nodes[from:] render to
// under after — the run's state once nodes[from-1] is written — up to the
// first whitespace, or through the end of the first sibling node whose own
// output holds a neighboring autolink's opening '<', continued past the
// run's end with after.wordAfter. ok is false once bareAutolinkWindowCap
// bytes have been rendered with no wall found, on this side or (via
// after.wordAfter) a further one: the caller must then treat the window as
// unproven, not substitute a truncated one — see bareAutolinkPrefix, its
// look-behind counterpart, for why a wall-only stop cannot bound this alone.
//
// Whitespace truncates mid-node, same as always: no literal crosses it, so
// nothing past it is needed. A '<' does not truncate mid-node — the node
// that wrote it may still have, past it, its own closing delimiter this
// link's flanking check needs (e.g. "*<http://a.b>*"'s trailing '*'; see
// bareAutolinkRoundTrips for why dropping that broke a kept-brackets case) —
// so that whole node's output is kept and only the NEXT sibling is dropped.
//
// The siblings are rendered ONE AT A TIME and the scan stops at the first
// wall or at the cap, so the cost is bounded by whichever comes first, not
// by the length of the word or the run: a paragraph of N space-separated
// links costs N short probes, and — since bareProbe (below) forces every
// sibling autolink in THIS probe to angle form — a whitespace-free word
// holding N autolinks joined by punctuation (a common Jira/Confluence shape)
// now also costs N short probes, not the O(N) siblings-per-call that made
// the total O(N^2): rendering a full word of N autolinks used to take each
// one's own check all the way to the nearest whitespace, re-rendering every
// sibling link still ahead of it. Each node is still visited with the whole
// run as its sibling slice, so the neighbor peeks it makes see what the real
// pass sees.
//
// The probe renders with bareProbe set, so a later bare autolink is written
// in angle form there instead of running its own check. That keeps the cost
// linear rather than exponential, and it is sound: the later link runs its
// real check with THIS link's actual bytes in its own prefix, so if its bare
// form would disturb this link, its trees differ and it keeps the brackets
// this probe assumed.
func (r *mdRenderer) wordAhead(nodes []ast.Node, from int, after inlineContext) (string, bool) {
	probe := *r
	probe.bareProbe = true
	var tmp strings.Builder
	v := &inlineWriteVisitor{r: &probe, b: &tmp, st: &after, nodes: nodes}
	for j := from; j < len(nodes); j++ {
		scanned := tmp.Len()
		v.i = j
		after.directiveHazard = probe.needsPunctTrail(nodes, j, &after)
		ast.Visit(nodes[j], v)
		delta := tmp.String()[scanned:]
		space, wall := strings.IndexFunc(delta, unicode.IsSpace), strings.IndexByte(delta, '<')
		switch {
		case space >= 0 && (wall < 0 || space <= wall):
			// Whitespace ends the word mid-node: truncate there, same as
			// before this function knew about '<' at all.
			return tmp.String()[:scanned+space], true
		case wall >= 0:
			// A neighboring autolink's '<' ends the word, but not inside
			// this node's own output: the node may still have, past that
			// '<', its own closing delimiter this run's flanking check
			// needs (e.g. "*<http://a.b>*"'s trailing '*'), so the whole
			// node is kept and only the NEXT sibling is dropped.
			return tmp.String(), true
		}
		if tmp.Len() > bareAutolinkWindowCap {
			return "", false
		}
	}
	if after.wordAfter != nil {
		rest, ok := after.wordAfter()
		if !ok {
			return "", false
		}
		return tmp.String() + rest, true
	}
	return tmp.String(), true
}

// wordEdges is the part of a bare-autolink window that lies outside a run:
// see inlineContext.wordBefore and wordAfter. beforeCapped mirrors
// inlineContext.wordBeforeCapped: false (the zero value) says before is a
// real wall or a true construct edge, usable as is; true says
// bareAutolinkWindowCap was reached building it with no wall found, so it
// must not be trusted. after returns the same kind of ok alongside its
// string.
type wordEdges struct {
	after        func() (string, bool)
	before       string
	beforeCapped bool
}

// markerWord is the window edge of a construct whose surroundings are not
// known: its own opening and closing marker and nothing further out.
func markerWord(marker string) wordEdges {
	return wordEdges{before: marker, after: func() (string, bool) { return marker, true }}
}

// wrappedWord returns the window edges for the children of the construct at
// nodes[i], which is about to be written with marker into b, whose run began
// at start: before is the run's word so far plus the opening marker, and
// after (on demand) is the closing marker plus what the run's later siblings
// write of the same word. st is the run's state, copied now; the closure
// renders under that copy as it will stand once the construct is written,
// with *closeProblem as the construct's encodeLead hand-off at call time.
func (r *mdRenderer) wrappedWord(
	b *strings.Builder, nodes []ast.Node, i, start int, marker string, closeProblem *bool, st *inlineContext,
) wordEdges {
	next := *st
	next.prev, next.hasPrev = marker[len(marker)-1], true
	next.prevRune = rune(marker[len(marker)-1])
	before, ok := bareAutolinkPrefix(b.String()[start:], st.wordBefore, !st.wordBeforeCapped)
	return wordEdges{
		before:       before + marker,
		beforeCapped: !ok,
		after: func() (string, bool) {
			ahead := next
			ahead.encodeLead = *closeProblem
			rest, ok := r.wordAhead(nodes, i+1, ahead)
			if !ok {
				return "", false
			}
			return marker + rest, true
		},
	}
}

// unmaskWrap restores the spaces and tabs the inline writers masked against
// paragraph wrapping (see wrapMask), so the window parses as the final
// output will.
func unmaskWrap(s string) string {
	s = strings.ReplaceAll(s, string(wrapMask), " ")
	return strings.ReplaceAll(s, string(wrapMaskTab), "\t")
}

// sameTreeIgnoringLinkForm reports whether two parsed trees are identical
// once the one field that records a link's source form, ast.Link.Bare, is
// cleared on both. The trees are fresh from Parse, so clearing in place
// touches nothing the caller holds.
func sameTreeIgnoringLinkForm(a, b ast.Node) bool {
	clearLinkForm(a)
	clearLinkForm(b)
	return reflect.DeepEqual(a, b)
}

// clearLinkForm resets ast.Link.Bare throughout n's subtree.
func clearLinkForm(n ast.Node) {
	if l, ok := n.(*ast.Link); ok {
		l.Bare = false
	}
	for _, c := range ast.Children(n) {
		clearLinkForm(c)
	}
}
