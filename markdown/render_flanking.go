package markdown

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
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
func (r *mdRenderer) renderedChildLead(node ast.Node, st *inlineContext) rune {
	var tmp strings.Builder
	child := inlineContext{escape: st.escape, colons: st.colons, pipes: st.pipes, prevRune: '_'}
	r.writeInlines(&tmp, ast.Children(node), &child)
	return firstRuneOf(tmp.String())
}

// renderedChildTrail is renderedChildLead's counterpart for the last rune.
func (r *mdRenderer) renderedChildTrail(node ast.Node, st *inlineContext) rune {
	var tmp strings.Builder
	child := inlineContext{escape: st.escape, colons: st.colons, pipes: st.pipes, prevRune: '_'}
	r.writeInlines(&tmp, ast.Children(node), &child)
	return lastRuneOf(tmp.String())
}

// siblingLeadRune is the first rune rendered by nodes[i] (0 = end of the
// phrasing run, which flanking treats as whitespace).
func siblingLeadRune(nodes []ast.Node, i int) rune {
	if i >= len(nodes) {
		return 0
	}
	return nodeLeadRune(nodes[i])
}

func linkLeadRune(node *ast.Link) rune {
	if len(node.Children) == 1 {
		if t, ok := node.Children[0].(*ast.Text); ok {
			text := t.Value
			if text == node.URL || (strings.HasPrefix(node.URL, "mailto:") && text == strings.TrimPrefix(node.URL, "mailto:")) {
				return '<'
			}
		}
	}
	return '['
}

// isEncodableRune reports whether remark-stringify would hex-encode this
// rune next to a non-flankable emphasis marker: alphanumerics make the
// marker intraword, whitespace makes it non-flanking outright.
func isEncodableRune(r rune) bool {
	// Word-class neighbors break emphasis flanking and get hex-encoded.
	// The class mirrors the flanking checks exactly (anything neither
	// whitespace nor punctuation — alphanumerics, combining marks, format
	// characters …), so every detected open/close problem is fixable; the
	// boundary space is the remaining encodable case.
	if r == 0 || r == '\n' {
		return false
	}
	return unicode.IsSpace(r) || !flankPunct(r)
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
	if text == node.URL || (isMailto && text == mailtoAddr) {
		return text, true
	}
	return "", false
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
		link.WriteString(" \"")
		link.WriteString(r.escapeTitle(node.Title))
		link.WriteString("\"")
	}
	link.WriteString(")")
	masked := strings.ReplaceAll(link.String(), " ", string(wrapMask))
	b.WriteString(strings.ReplaceAll(masked, "\t", string(wrapMaskTab)))
}

// escapeTitle applies remark's directive colon escaping inside link/image
// titles (a ':' before an ASCII letter re-parses as a text directive);
// prettier mode leaves titles verbatim (source escapes ride the
// preserved-escape sentinels).
func (r *mdRenderer) escapeTitle(s string) string {
	if r.cfg.prettierText || !strings.ContainsRune(s, ':') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	for i := range len(s) {
		if s[i] == ':' && i+1 < len(s) && isASCIILetter(s[i+1]) && (i == 0 || s[i-1] != ':') {
			sb.WriteByte('\\')
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
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
	case *ast.Link:
		return '['
	case *ast.Image:
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
// ends with a backtick or space — mdast-util-to-markdown's inline-code rules
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
	if s != "" && (s[0] == '`' || s[len(s)-1] == '`' ||
		(s[0] == ' ' && s[len(s)-1] == ' ' && strings.Trim(s, " ") != "")) {
		pad = " "
	}
	return fence + pad + s + pad + fence
}
