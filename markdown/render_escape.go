package markdown

import (
	"strings"
	"unicode"
)

// Text escaping: remark-stringify's (and prettier's) character escape
// rules for plain text, labels, and line-start block markers. Split
// from render.go.

// escapeText escapes special markdown characters in plain text nodes.
// goldmarkToAst's decodeMarkdownEscapes already strips \X sequences from
// goldmark raw bytes before building AST nodes, so by the time text reaches
// here the values are decoded and each backslash is a literal backslash that
// must be re-escaped.
//
// Colons are escaped per mdast-util-directive's unsafe patterns (measured
// against remark-stringify, see testdata/directive_fixtures.json): a ':'
// followed by an ASCII letter and preceded by a non-':' character would
// re-parse as a text directive, and a leading "::" at a block break would
// re-parse as a leaf/container directive.
func (r *mdRenderer) escapeText(s string, st *inlineContext, nextLead byte, encodeLead, encodeTrail bool) string {
	var sb strings.Builder
	sb.Grow(len(s) + 8)
	// Hex-encode the boundary runes adjacent to a non-flankable emphasis
	// marker (remark-stringify behavior); only alphanumerics are encoded —
	// punctuation neighbors already satisfy the flanking rules.
	start := r.writeEncodedLead(&sb, s, st, encodeLead)
	end, trailRef := encodedTrail(s, start, encodeTrail, r.cfg.noSpaceEscapes)
	s = s[start:end]
	// Line-start state at the node's first content byte — the per-char
	// st.prev advances through the loop, but a digit RUN's start position
	// needs the state where the run began.
	nodeAtLineStart := atLineStart(st)
	st.nodePrev, st.nodeHasPrev = st.prev, st.hasPrev
	for i := range len(s) {
		r.writeEscapedByte(&sb, s, i, nextLead, st, nodeAtLineStart)
	}
	if trailRef != "" {
		sb.WriteString(trailRef)
		st.prev, st.hasPrev = ';', true
	}
	return sb.String()
}

// writeEncodedLead hex-encodes the node's leading rune where remark does
// and returns the byte offset the escaped content starts at.
func (r *mdRenderer) writeEncodedLead(sb *strings.Builder, s string, st *inlineContext, encodeLead bool) int {
	start := 0
	// A space at a line start after a hard break would be stripped on
	// re-parse; remark hex-encodes it ("\\\n&#x20;0"). A render that is only
	// compared, never read back, asks for the space itself instead.
	if !r.cfg.prettierText && !r.cfg.noSpaceEscapes && st.hasPrev && st.prev == '\n' {
		if r0 := firstRuneOf(s); r0 != 0 && r0 != '\n' && unicode.IsSpace(r0) {
			sb.WriteString(hexRef(r0))
			start = len(string(r0))
			st.prev, st.hasPrev = ';', true
		}
	}
	if encodeLead {
		if r0 := firstRuneOf(s); r0 != 0 && isEncodableRune(r0, r.cfg.noSpaceEscapes) {
			sb.WriteString(hexRef(r0))
			start = len(string(r0))
			st.prev, st.hasPrev = ';', true
		}
	}
	return start
}

// encodedTrail returns where the escaped content ends and the hex
// reference that replaces the trailing rune (empty when none does).
// noSpaceEscapes leaves a whitespace trailer alone (see
// WithoutSignificantSpaceEscapes); the word-class trailers still encode,
// because those keep the following marker flankable.
func encodedTrail(s string, start int, encodeTrail, noSpaceEscapes bool) (end int, ref string) {
	if !encodeTrail {
		return len(s), ""
	}
	if r1 := lastRuneOf(s[start:]); r1 != 0 && isEncodableRune(r1, noSpaceEscapes) {
		return len(s) - len(string(r1)), hexRef(r1)
	}
	return len(s), ""
}

// writeEscapedByte writes s[i] behind whatever backslashes the active
// rules call for, and advances the per-character state.
func (r *mdRenderer) writeEscapedByte(sb *strings.Builder, s string, i int, nextLead byte, st *inlineContext, nodeAtLineStart bool) {
	ch := s[i]
	// Table-cell pipe escaping applies even where markdown escaping is
	// off (link labels): mdast-util-gfm-table's unsafe rule covers the
	// whole cell.
	if ch == '|' && st.pipes {
		sb.WriteByte('\\')
	}
	// Link labels use remark's restricted escape set (measured):
	// brackets and emphasis-capable markers escape, a backslash only
	// before punctuation, a colon only before a letter; '@', '|',
	// '#', '-' and the atBreak rules do not apply inside labels.
	if st.label && !r.cfg.prettierText && labelEscapes(s, i, nextLead, st) {
		sb.WriteByte('\\')
	}
	// A ']' inside a directive [label] closes the label, so mdast-util-
	// directive's unsafe set escapes it there — the leaf and container
	// forms already do (see escapeDirectiveLabel); this covers the text
	// form, whose label is written through the inline path.
	if ch == ']' && st.directiveLabel {
		sb.WriteByte('\\')
	}
	if st.escape && r.needsEscape(s, i, nextLead, st, nodeAtLineStart) {
		sb.WriteByte('\\')
	}
	sb.WriteByte(ch)
	st.prev, st.hasPrev = ch, true
}

// needsEscape reports whether s[i] needs a backslash under the active
// markdown escaping rules (remark-stringify vs prettier). The cases are
// dispatched to predicates grouped by character class.
func (r *mdRenderer) needsEscape(s string, i int, nextLead byte, st *inlineContext, nodeAtLineStart bool) bool {
	switch s[i] {
	case '_', '\\', '~', '*', '`', '<', '[', '(', ':':
		return r.escapesInlineMarker(s, i, nextLead, st)
	case '=', '#', '>', '-', '+':
		return r.escapesLineStart(s, i, nextLead, st)
	case '|', '.', ')', '&', '!', '@':
		return r.escapesToken(s, i, nextLead, st, nodeAtLineStart)
	}
	return false
}

// escapesInlineMarker covers the inline-construct markers (emphasis,
// code, links, directives) for needsEscape.
func (r *mdRenderer) escapesInlineMarker(s string, i int, nextLead byte, st *inlineContext) bool {
	switch s[i] {
	case '_':
		return r.escapeUnderscore(s, i, nextLead, st)
	case '\\':
		return r.escapeBackslash(s, i, nextLead)
	case '~':
		// Prettier never escapes a tilde (its parser keeps \~
		// literal, handled via the format sentinel); remark always
		// escapes it.
		return !r.cfg.prettierText
	case '*', '`':
		return true
	case '<':
		return r.escapeAngle(s, i, nextLead)
	case '[':
		// remark escapes '[' in phrasing (link ambiguity: "[]()" text
		// would re-parse as a link and vanish); prettier drops the escape
		// again, matching the corpus.
		return !r.cfg.prettierText
	case '(':
		// remark escapes '(' after ']' (it would complete a link).
		return !r.cfg.prettierText && st.hasPrev && st.prev == ']'
	case ':':
		return r.escapesColon(s, i, nextLead, st)
	}
	return false
}

// escapesLineStart covers the atBreak (line-start) block-syntax
// characters for needsEscape. None of them applies away from a line
// start, and prettier escapes none of them at all.
func (r *mdRenderer) escapesLineStart(s string, i int, nextLead byte, st *inlineContext) bool {
	if r.cfg.prettierText || !atLineStart(st) {
		return false
	}
	switch s[i] {
	case '=', '#', '>':
		// remark's atBreak unsafe rules: these characters at a line start
		// (paragraph start or after a break) would re-parse as setext/
		// heading/quote syntax (measured, see the atbreak probes).
		return true
	case '-':
		// '-' at a line start re-parses as a list marker (space/tab/EOL
		// after), setext/thematic ('-' after), or a table delimiter row
		// (':'/'|' after); bare "-x" stays unescaped.
		n := byteAt(s, i+1, nextLead)
		return n == 0 || n == ' ' || n == '\t' || n == '-' || n == '|' || n == ':'
	case '+':
		// '+' at a line start followed by space/tab/EOL re-parses as a
		// list marker.
		n := byteAt(s, i+1, nextLead)
		return n == 0 || n == ' ' || n == '\t'
	}
	return false
}

// escapesToken covers the neighbor-sensitive token characters (table
// pipes, ordered-list punctuation, character references, images, email
// autolinks) for needsEscape.
func (r *mdRenderer) escapesToken(s string, i int, nextLead byte, st *inlineContext, nodeAtLineStart bool) bool {
	switch s[i] {
	case '|':
		return r.escapeTablePipe(s, i, nextLead, st)
	case '.', ')':
		return r.escapeOrderedMarker(s, i, nextLead, nodeAtLineStart)
	case '&':
		return r.escapeAmpersand(s, i, nextLead)
	case '!':
		// '!' before '[' would combine with a following link into image
		// syntax; remark escapes it (probe: "!![]()[0]()").
		return !r.cfg.prettierText && byteAt(s, i+1, nextLead) == '['
	case '@':
		return r.escapeAt(s, i, nextLead, st)
	}
	return false
}

// escapeTablePipe: '|' at a line start followed by [ \t:-] could form a
// table delimiter row; mdast-util-gfm-table escapes it (measured: "|-",
// "| - |", "|:-" escape; "|x" does not).
func (r *mdRenderer) escapeTablePipe(s string, i int, nextLead byte, st *inlineContext) bool {
	if r.cfg.prettierText || !atLineStart(st) {
		return false
	}
	n := byteAt(s, i+1, nextLead)
	return n == ' ' || n == '\t' || n == '-' || n == ':'
}

// escapeOrderedMarker: a digit run from the line start followed by
// '.'/')' and then space/tab/EOL re-parses as an ordered list marker;
// remark escapes the punctuation.
func (r *mdRenderer) escapeOrderedMarker(s string, i int, nextLead byte, nodeAtLineStart bool) bool {
	if r.cfg.prettierText || !digitRunFromLineStart(s, i, nodeAtLineStart) {
		return false
	}
	n := byteAt(s, i+1, nextLead)
	return n == 0 || n == ' ' || n == '\t'
}

// escapeAmpersand: '&' before '#' or a letter could form a character
// reference on re-parse; remark escapes it ("AT\&T", "\&#0;") and leaves
// bare ampersands alone ("a & b"). prettier escapes a narrower set: only an
// '&' that actually opens a reference the parse would decode, which is why
// "AT&T" stays bare while "\&#169;" and "\&amp;" keep their backslash
// (measured against prettier 3.9.6). Without the escape, a text value that
// literally contains e.g. "&#169;" is written bare, and the NEXT parse
// decodes it — a silent character change in prose, and a document with two
// different renderings depending on the pass.
//
// escapeText sees one text node's value plus a single lookahead byte
// (nextLead), so a reference split across two AST nodes would not be
// detected. That is unreachable in practice: any mark boundary inserts a
// marker byte ('*', '`', '[') between the '&' and the rest, which breaks the
// reference, and text-node coalescing merges adjacent same-mark runs first.
func (r *mdRenderer) escapeAmpersand(s string, i int, nextLead byte) bool {
	if r.cfg.prettierText {
		return startsCharacterReference(s, i)
	}
	n := byteAt(s, i+1, nextLead)
	return n == '#' || isASCIILetter(n)
}

// startsCharacterReference reports whether s[i] opens a character reference
// the parse would decode. It is the escaper's half of
// decodeCharacterReference (goldmark_to_ast.go) — the two must agree, by
// construction, or a render is not a fixpoint: what the escaper lets through
// bare, the next parse would decode.
func startsCharacterReference(s string, i int) bool {
	_, _, ok := decodeCharacterReference(s, i)
	return ok
}

// escapeAt reports whether an '@' would form a GFM email autolink literal
// with its neighbors on re-parse. remark escapes on its unsafe rule's
// one-character neighborhood; prettier's pre-CommonMark parser has no
// autolink literals at all and would leave the '@' bare, but the parse
// adfast round-trips against does linkify — so in format mode the escape
// is written exactly where that parse would produce a link, a deliberate
// divergence (see linkifiesAsEmail).
func (r *mdRenderer) escapeAt(s string, i int, nextLead byte, st *inlineContext) bool {
	if st.label {
		return false
	}
	if r.cfg.prettierText {
		return linkifiesAsEmail(s, i, st)
	}
	if !st.hasPrev || !isEmailBeforeByte(st.prev) {
		return false
	}
	return isEmailAfterByte(byteAt(s, i+1, nextLead))
}

// linkifiesAsEmail reports whether goldmark's linkify extension would read
// an email autolink literal over the '@' at s[i], mirroring its parser
// (extension/linkify.go): the literal begins at a line head or right after
// one of the trigger bytes " *_~(", never on ASCII punctuation, its shape
// is gfmEmailRe (the regexp adfast configures), its domain needs a dot,
// and a '-' or '_' directly after the match cancels it.
//
// The check is node-local, which is why normalization joins adjacent text
// atoms first: the local part and the domain must sit in one node for the
// walk to see them.
func linkifiesAsEmail(s string, i int, st *inlineContext) bool {
	lo := i
	for lo > 0 && isEmailLocalByte(s[lo-1]) {
		lo--
	}
	// The local part can also run out of the node and into the markup that
	// precedes it — an emphasis closer is an underscore, which the linkify
	// scan reads as part of the address ("*a*@b.com" renders "_a_@b.com",
	// whose literal is "a_@b.com"). The walk cannot follow it there, so
	// escape whenever it might continue.
	if lo == 0 && st.nodeHasPrev && isEmailLocalByte(st.nodePrev) {
		return true
	}
	for p := lo; p < i; p++ {
		if emailLiteralStarts(s, p, st) && emailLiteralMatches(s[p:], i-p) {
			return true
		}
	}
	return false
}

// emailLiteralStarts reports whether an autolink literal can begin at s[p]:
// goldmark's linkify parser is triggered by one of " *_~(" (or a line head,
// where the block parser hands it the line), and it rejects a candidate
// opening on punctuation.
func emailLiteralStarts(s string, p int, st *inlineContext) bool {
	if isASCIIPunct(s[p]) {
		return false
	}
	if p > 0 {
		return isLinkifyTrigger(s[p-1])
	}
	return !st.nodeHasPrev || st.nodePrev == '\n' || isLinkifyTrigger(st.nodePrev)
}

// isLinkifyTrigger reports whether c is one of the bytes goldmark's
// linkify parser triggers on (linkifyParser.Trigger).
func isLinkifyTrigger(c byte) bool {
	return c == ' ' || c == '*' || c == '_' || c == '~' || c == '('
}

// emailLiteralMatches reports whether cand opens with an email autolink
// literal whose '@' sits at index at.
func emailLiteralMatches(cand string, at int) bool {
	m := gfmEmailRe.FindStringIndex(cand)
	if len(m) != 2 || m[0] != 0 || strings.IndexByte(cand[:m[1]], '@') != at {
		return false
	}
	stop := m[1]
	// The domain must hold a dot; a trailing one is not part of the link,
	// and a '-' or '_' right after the match voids it (linkifyParser.Parse).
	if strings.IndexByte(cand[at:stop-1], '.') < 0 {
		return false
	}
	if cand[stop-1] == '.' {
		stop--
	}
	return stop >= len(cand) || (cand[stop] != '-' && cand[stop] != '_')
}

// isEmailLocalByte matches gfmEmailRe's local-part class [a-zA-Z0-9.+_-].
func isEmailLocalByte(c byte) bool {
	return isWordByte(c) || c == '.' || c == '+' || c == '-'
}

// isEmailBeforeByte matches mdast-util-gfm-autolink-literal's before class
// [+\-.\w] for the '@' unsafe rule.
func isEmailBeforeByte(c byte) bool {
	return c == '+' || c == '-' || c == '.' || isWordByte(c)
}

// isEmailAfterByte matches the after class [-.\w].
func isEmailAfterByte(c byte) bool {
	return c == '-' || c == '.' || isWordByte(c)
}

// escapeUnderscore: prettier only escapes '_' at word boundaries (an
// intraword underscore cannot open emphasis); remark escapes all.
func (r *mdRenderer) escapeUnderscore(s string, i int, nextLead byte, st *inlineContext) bool {
	return !r.cfg.prettierText || !st.hasPrev || !isWordByte(st.prev) || !isWordByteAt(s, i+1, nextLead)
}

// escapeBackslash: prettier keeps a literal backslash bare unless the next
// character is ASCII punctuation (where it would re-parse as an escape);
// remark always escapes it. Escape sequences the formatter parse preserved
// in the text (see PreservedEscapes) are already literal source bytes.
func (r *mdRenderer) escapeBackslash(s string, i int, nextLead byte) bool {
	if !r.cfg.prettierText {
		return true
	}
	next := byteAt(s, i+1, nextLead)
	if strings.IndexByte(PreservedEscapes, next) >= 0 {
		return false
	}
	return isASCIIPunct(next)
}

// escapeAngle: remark always escapes '<' in phrasing ("<!A" would re-parse
// as an HTML block and vanish); prettier only escapes the HTML-forming
// cases.
func (r *mdRenderer) escapeAngle(s string, i int, nextLead byte) bool {
	if !r.cfg.prettierText {
		return true
	}
	n := byteAt(s, i+1, nextLead)
	return n == '!' || n == '/' || n == '?' || isASCIILetter(n)
}

// escapesColon reports whether a ':' would re-parse as a text directive
// (a ':' before an ASCII letter, not preceded by another ':') or as a
// leading "::" at a block break, and so needs a backslash.
//
// Inside a text directive's [label] the rule widens, a deliberate
// divergence from remark: a nested text directive there is LOSSY (the
// label is read back with ast.PlainText, which has no text for a
// directive node), so it also covers digit-led names — goldmark-directive
// parses those — and the label's first character, which has no previous
// character but does follow the '[' the renderer just wrote.
//
// The rule holds in prettier mode too, a deliberate divergence from
// prettier itself (which has no directive grammar and so never writes
// this escape). Without it the prettier render is LOSSY on the ADF path,
// where text carries no source provenance to fall back on: the ADF text
// "the value:status is set" was emitted bare, re-parsed into an empty
// :status the dialect then dropped, and came back as "the value" +
// " is set". A name the dialect does NOT register loses no characters
// but still splits the one text node into three, and ":media" invents a
// mediaInline node. Measured on all three; see
// TestPrettierRenderKeepsDirectiveShapedTextIntact.
func (r *mdRenderer) escapesColon(s string, i int, nextLead byte, st *inlineContext) bool {
	if !st.colons {
		return false
	}
	// A literal backslash the prettier path has just written BARE is a
	// preserved source escape (see PreservedEscapes) and already escapes
	// this colon; a second one would render the author's "\:" as a
	// literal backslash. In remark mode that backslash goes out doubled,
	// so the colon after it still needs its own escape.
	if r.cfg.prettierText && i > 0 && s[i-1] == '\\' && !r.escapeBackslash(s, i-1, nextLead) {
		return false
	}
	next := nextLead
	if i+1 < len(s) {
		next = s[i+1]
	}
	notAfterColon := st.prev != ':' || !st.hasPrev
	escapes := st.hasPrev && st.prev != ':' && isASCIILetter(next)
	if st.directiveLabel {
		escapes = notAfterColon && isDirectiveNameStart(next)
	}
	atBreak := st.hasPrev && st.prev == '\n' && next == ':'
	return escapes || atBreak
}

// labelEscapes reports whether s[i] needs a backslash inside a link label
// (remark's label safety set; see the label probes on the ewyh bead).
func labelEscapes(s string, i int, nextLead byte, st *inlineContext) bool {
	switch s[i] {
	case '[', ']', '*', '_', '~', '\x60', '<':
		return true
	case '\\':
		return isASCIIPunct(byteAt(s, i+1, nextLead))
	case ':':
		return st.hasPrev && st.prev != ':' && isASCIILetter(byteAt(s, i+1, nextLead))
	}
	return false
}

// atLineStart reports whether the next byte written lands at the start of
// an output line: right after a newline, or at a paragraph start (where
// renderInlineStringFrom seeds prev='\n'). Nested constructs (emphasis
// children) start with hasPrev=false and are NOT at a line start — their
// marker precedes them.
func atLineStart(st *inlineContext) bool {
	return st.hasPrev && st.prev == '\n'
}

// digitRunFromLineStart reports whether s[i] is preceded (within the node)
// by one or more digits reaching back to a line start.
func digitRunFromLineStart(s string, i int, nodeAtLineStart bool) bool {
	j := i
	for j > 0 && s[j-1] >= '0' && s[j-1] <= '9' {
		j--
	}
	if j == i {
		return false
	}
	if j == 0 {
		return nodeAtLineStart
	}
	return s[j-1] == '\n'
}

// isASCIIPunct reports CommonMark's ASCII punctuation set (the escapable
// characters).
func isASCIIPunct(c byte) bool {
	return (c >= '!' && c <= '/') || (c >= ':' && c <= '@') || (c >= '[' && c <= '\x60') || (c >= '{' && c <= '~')
}

// byteAt returns s[i], falling back to the next sibling's lead byte at the
// end of the text node.
func byteAt(s string, i int, nextLead byte) byte {
	if i < len(s) {
		return s[i]
	}
	return nextLead
}

// isWordByte mirrors prettier's \w test for underscore escaping.
func isWordByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// isWordByteAt tests the byte at position i in s, falling back to the next
// sibling's lead byte at the end of the text node.
func isWordByteAt(s string, i int, nextLead byte) bool {
	return isWordByte(byteAt(s, i, nextLead))
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// escapeParagraphLeadingMarker escapes block-level marker characters that
// appear at the start of any line within paragraph text. An ADF paragraph may
// contain inline text nodes with embedded "\n- " — characters that look like
// list items. Escaping them prevents Markdown parsers from misinterpreting
// these soft-break lines as list items.
func escapeParagraphLeadingMarker(s string, prettier bool) string {
	if s == "" {
		return s
	}
	if !strings.ContainsAny(s, "-*+>#0123456789") {
		return s
	}
	lines := strings.Split(s, "\n")
	changed := false
	for i, line := range lines {
		escaped := escapeLineLeadingMarker(line, prettier)
		if escaped != line {
			lines[i] = escaped
			changed = true
		}
	}
	if !changed {
		return s
	}
	return strings.Join(lines, "\n")
}

// escapeLineLeadingMarker escapes a leading list/block marker on a single line.
func escapeLineLeadingMarker(s string, prettier bool) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '-':
		return escapeLeadingDash(s, prettier)
	case '*', '+':
		// Bare marker at end of line also parses as an (empty) list item.
		// A leading '*' followed by anything else is emphasis syntax here —
		// literal stars in text are already character-escaped.
		if len(s) == 1 || s[1] == ' ' {
			return `\` + s
		}
	case '>':
		return `\` + s
	case '#':
		return escapeLeadingHash(s)
	default:
		return escapeLeadingOrderedMarker(s)
	}
	return s
}

// escapeLeadingDash escapes a line-leading '-' that would re-parse as a list
// marker or thematic break.
func escapeLeadingDash(s string, prettier bool) string {
	// Prettier leaves line-leading dash runs alone (standalone dashes
	// are already character-escaped in prettier mode).
	if prettier {
		return s
	}
	// A bare marker at end of line parses as an (empty) list item; a
	// dash run ("-- -", "--x") parses as a thematic break or setext-ish
	// noise. Literal dashes are not otherwise escaped, so this fires on
	// text content only.
	if len(s) == 1 || s[1] == ' ' || s[1] == '-' {
		return `\` + s
	}
	return s
}

// escapeLeadingHash escapes a line-leading ATX heading marker: 1-6 '#'
// followed by space or end of line.
func escapeLeadingHash(s string) string {
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	if i <= 6 && (i == len(s) || s[i] == ' ' || s[i] == '\t') {
		return `\` + s
	}
	return s
}

// escapeLeadingOrderedMarker escapes a line-leading ordered-list marker:
// digit(s) followed by ". ".
func escapeLeadingOrderedMarker(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') && (i+1 == len(s) || s[i+1] == ' ') {
		return s[:i] + `\` + s[i:i+1] + s[i+1:]
	}
	return s
}
