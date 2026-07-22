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
	start, end := 0, len(s)
	// A space at a line start after a hard break would be stripped on
	// re-parse; remark hex-encodes it ("\\\n&#x20;0").
	if !r.cfg.prettierText && st.hasPrev && st.prev == '\n' {
		if r0 := firstRuneOf(s); r0 != 0 && r0 != '\n' && unicode.IsSpace(r0) {
			sb.WriteString(hexRef(r0))
			start = len(string(r0))
			st.prev, st.hasPrev = ';', true
		}
	}
	if encodeLead {
		if r0 := firstRuneOf(s); r0 != 0 && isEncodableRune(r0) {
			sb.WriteString(hexRef(r0))
			start = len(string(r0))
			st.prev, st.hasPrev = ';', true
		}
	}
	trailRef := ""
	if encodeTrail {
		if r1 := lastRuneOf(s[start:]); r1 != 0 && isEncodableRune(r1) {
			trailRef = hexRef(r1)
			end = len(s) - len(string(r1))
		}
	}
	s = s[start:end]
	// Line-start state at the node's first content byte — the per-char
	// st.prev advances through the loop, but a digit RUN's start position
	// needs the state where the run began.
	nodeAtLineStart := atLineStart(st)
	for i := range len(s) {
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
		if st.escape {
			r.writeEscapePrefix(&sb, s, i, nextLead, st, nodeAtLineStart)
		}
		sb.WriteByte(ch)
		st.prev, st.hasPrev = ch, true
	}
	if trailRef != "" {
		sb.WriteString(trailRef)
		st.prev, st.hasPrev = ';', true
	}
	return sb.String()
}

// writeEscapePrefix writes the backslash escape needed before s[i] under the
// active markdown escaping rules (remark-stringify vs prettier). The cases
// are dispatched to helpers grouped by character class.
func (r *mdRenderer) writeEscapePrefix(sb *strings.Builder, s string, i int, nextLead byte, st *inlineContext, nodeAtLineStart bool) {
	switch s[i] {
	case '_', '\\', '~', '*', '`', '<', '[', '(', ':':
		r.writeInlineMarkerEscapePrefix(sb, s, i, nextLead, st)
	case '=', '#', '>', '-', '+':
		r.writeLineStartEscapePrefix(sb, s, i, nextLead, st)
	case '|', '.', ')', '&', '!', '@':
		r.writeTokenEscapePrefix(sb, s, i, nextLead, st, nodeAtLineStart)
	}
}

// writeInlineMarkerEscapePrefix handles the inline-construct markers
// (emphasis, code, links, directives) for writeEscapePrefix.
func (r *mdRenderer) writeInlineMarkerEscapePrefix(sb *strings.Builder, s string, i int, nextLead byte, st *inlineContext) {
	switch s[i] {
	case '_':
		if r.escapeUnderscore(s, i, nextLead, st) {
			sb.WriteByte('\\')
		}
	case '\\':
		if r.escapeBackslash(s, i, nextLead) {
			sb.WriteByte('\\')
		}
	case '~':
		// Prettier never escapes a tilde (its parser keeps \~
		// literal, handled via the format sentinel); remark always
		// escapes it.
		if !r.cfg.prettierText {
			sb.WriteByte('\\')
		}
	case '*', '`':
		sb.WriteByte('\\')
	case '<':
		if r.escapeAngle(s, i, nextLead) {
			sb.WriteByte('\\')
		}
	case '[':
		// remark escapes '[' in phrasing (link ambiguity: "[]()" text
		// would re-parse as a link and vanish); prettier drops the escape
		// again, matching the corpus.
		if !r.cfg.prettierText {
			sb.WriteByte('\\')
		}
	case '(':
		// remark escapes '(' after ']' (it would complete a link).
		if !r.cfg.prettierText && st.hasPrev && st.prev == ']' {
			sb.WriteByte('\\')
		}
	case ':':
		r.writeColonEscapePrefix(sb, s, i, nextLead, st)
	}
}

// writeLineStartEscapePrefix handles the atBreak (line-start) block-syntax
// characters for writeEscapePrefix.
func (r *mdRenderer) writeLineStartEscapePrefix(sb *strings.Builder, s string, i int, nextLead byte, st *inlineContext) {
	switch s[i] {
	case '=', '#', '>':
		// remark's atBreak unsafe rules: these characters at a line start
		// (paragraph start or after a break) would re-parse as setext/
		// heading/quote syntax (measured, see the atbreak probes).
		if !r.cfg.prettierText && atLineStart(st) {
			sb.WriteByte('\\')
		}
	case '-':
		// '-' at a line start re-parses as a list marker (space/tab/EOL
		// after), setext/thematic ('-' after), or a table delimiter row
		// (':'/'|' after); bare "-x" stays unescaped.
		if !r.cfg.prettierText && atLineStart(st) {
			if n := byteAt(s, i+1, nextLead); n == 0 || n == ' ' || n == '\t' || n == '-' || n == '|' || n == ':' {
				sb.WriteByte('\\')
			}
		}
	case '+':
		// '+' at a line start followed by space/tab/EOL re-parses as a
		// list marker.
		if !r.cfg.prettierText && atLineStart(st) {
			if n := byteAt(s, i+1, nextLead); n == 0 || n == ' ' || n == '\t' {
				sb.WriteByte('\\')
			}
		}
	}
}

// writeTokenEscapePrefix handles the neighbor-sensitive token characters
// (table pipes, ordered-list punctuation, character references, images,
// email autolinks) for writeEscapePrefix.
func (r *mdRenderer) writeTokenEscapePrefix(sb *strings.Builder, s string, i int, nextLead byte, st *inlineContext, nodeAtLineStart bool) {
	switch s[i] {
	case '|':
		// '|' at a line start followed by [ \t:-] could form a table
		// delimiter row; mdast-util-gfm-table escapes it (measured:
		// "|-", "| - |", "|:-" escape; "|x" does not).
		if !r.cfg.prettierText && atLineStart(st) {
			if n := byteAt(s, i+1, nextLead); n == ' ' || n == '\t' || n == '-' || n == ':' {
				sb.WriteByte('\\')
			}
		}
	case '.', ')':
		// A digit run from the line start followed by '.'/')' and then
		// space/tab/EOL re-parses as an ordered list marker; remark
		// escapes the punctuation.
		if !r.cfg.prettierText && digitRunFromLineStart(s, i, nodeAtLineStart) {
			if n := byteAt(s, i+1, nextLead); n == 0 || n == ' ' || n == '\t' {
				sb.WriteByte('\\')
			}
		}
	case '&':
		// '&' before '#' or a letter could form a character reference on
		// re-parse; remark escapes it ("AT\\&T", "\\&#0;") and leaves
		// bare ampersands alone ("a & b").
		if !r.cfg.prettierText {
			if n := byteAt(s, i+1, nextLead); n == '#' || isASCIILetter(n) {
				sb.WriteByte('\\')
			}
		}
	case '!':
		// '!' before '[' would combine with a following link into image
		// syntax; remark escapes it (probe: "!![]()[0]()").
		if !r.cfg.prettierText && byteAt(s, i+1, nextLead) == '[' {
			sb.WriteByte('\\')
		}
	case '@':
		// mdast-util-gfm-autolink-literal's unsafe rule: an '@' between
		// email-literal characters would linkify on re-parse (before:
		// [+\-.\w], after: [-.\w]); does not apply inside labels.
		if r.escapeAt(s, i, nextLead, st) {
			sb.WriteByte('\\')
		}
	}
}

// escapeAt reports whether an '@' would form a GFM email autolink literal
// with its neighbors on re-parse (remark escapes it; prettier's
// pre-CommonMark parser has no autolink literals and drops the escape).
func (r *mdRenderer) escapeAt(s string, i int, nextLead byte, st *inlineContext) bool {
	if r.cfg.prettierText || st.label {
		return false
	}
	if !st.hasPrev || !isEmailBeforeByte(st.prev) {
		return false
	}
	return isEmailAfterByte(byteAt(s, i+1, nextLead))
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

// writeColonEscapePrefix writes the backslash before a ':' that would
// re-parse as a text directive (a ':' before an ASCII letter, not preceded by
// another ':') or a leading "::" at a block break.
func (r *mdRenderer) writeColonEscapePrefix(sb *strings.Builder, s string, i int, nextLead byte, st *inlineContext) {
	if st.colons && !r.cfg.prettierText {
		next := nextLead
		if i+1 < len(s) {
			next = s[i+1]
		}
		escapes := st.hasPrev && st.prev != ':' && isASCIILetter(next)
		atBreak := st.hasPrev && st.prev == '\n' && next == ':'
		if escapes || atBreak {
			sb.WriteByte('\\')
		}
	}
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
