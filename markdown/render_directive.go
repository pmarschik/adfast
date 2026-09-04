package markdown

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// Directive rendering: the :::container / ::leaf / :text directive
// forms, their attribute serialization, the extension render contexts,
// and the fence-sizing / label-escaping helpers. Split from render.go.

// renderContainerDirective renders a generic :::name container directive
// (the form-based helper does the work; see writeContainerDirectiveForm).
// The attribute block is written like the leaf form's: a container that
// was authored with attributes has to read back with them, or every
// re-render of the document deletes what the directive was configured
// with. (It did: this branch used to pass nil.)
func (r *mdRenderer) renderContainerDirective(b *strings.Builder, node *ast.ContainerDirective, _ int) {
	r.writeContainerDirectiveForm(b, node.Name, node.Attrs, node.Children)
}

// writeContainerDirectiveForm renders :::name[label]{attrs} fenced
// container directives. remark-stringify separates container children
// with blank lines and grows the fence around nested container
// directives (:::: > :::); attributes serialize on the fence line after
// the label, like the leaf form.
func (r *mdRenderer) writeContainerDirectiveForm(b *strings.Builder, name string, attrs map[string]string, children []ast.Node) {
	mustSpellDirectiveName(name)
	fence := strings.Repeat(":", containerFenceLength(children))
	b.WriteString(fence)
	b.WriteString(name)
	// A DirectiveLabel first paragraph (the :::expand title) renders as
	// the [label] on the fence line instead of body content.
	if len(children) > 0 {
		if p, ok := children[0].(*ast.Paragraph); ok && p.DirectiveLabel {
			b.WriteString("[")
			b.WriteString(escapeDirectiveLabel(ast.PlainText(p.Children)))
			b.WriteString("]")
			children = children[1:]
		}
	}
	writeDirectiveAttrs(b, attrs)
	b.WriteString("\n")
	r.renderBlockSequence(b, children, "\n")
	b.WriteString(fence)
	b.WriteString("\n")
}

// renderLeafDirective renders a generic ::name leaf directive (the
// form-based helper does the work; see writeLeafDirectiveForm).
func renderLeafDirective(b *strings.Builder, node *ast.LeafDirective) {
	writeLeafDirectiveForm(b, node.Name, node.Attrs, node.Children)
}

// writeLeafDirectiveForm renders ::name[label]{attrs} — the label is written
// verbatim (remark only escapes CR/LF inside directive labels) and attributes
// are serialized like mdast-util-directive (quoted, insertion order —
// deterministic here via sorted keys, which matches the fixed layout/width
// order the ADF conversion produces).
func writeLeafDirectiveForm(b *strings.Builder, name string, attrs map[string]string, children []ast.Node) {
	mustSpellDirectiveName(name)
	b.WriteString("::")
	b.WriteString(name)
	if label := escapeDirectiveLabel(ast.PlainText(children)); label != "" {
		b.WriteString("[")
		b.WriteString(label)
		b.WriteString("]")
	}
	writeDirectiveAttrs(b, attrs)
	b.WriteString("\n")
}

// writeDirectiveAttrs serializes a directive attribute block like
// mdast-util-directive: the id attribute uses the {#value} shortcut,
// other attributes are quoted key="value" pairs (sorted; empty-string
// values as a bare attribute name; quote style per writeDirectiveAttrValue);
// nothing for an empty map.
//
// The shortcut is only taken when it can spell the id — see
// shorthandSpells. An id the shortcut cannot hold falls back to the long
// form, exactly the way writeDirectiveAttrValue falls back to the quote
// style that survives: {#a b} re-parses as id="a" plus a bare attribute
// "b", so the id would be silently truncated, while id="a b" reads back
// whole.
//
// An attribute the block cannot spell at all is DROPPED, whether it is
// the key that has no written form (keySpells) or the value (valueSpells).
// Writing it would not spoil that one attribute: the block itself would
// no longer parse, and the directive would come back with none of its
// attributes (or, in the text form, as ordinary paragraph text). Losing
// one attribute is recoverable, losing the block is not, so the guard
// drops rather than corrupts, the same way renderHeading drops an id
// that has no writable anchor form. Only a hand-built or decoded tree can
// carry one; the parser cannot produce it. When nothing survives, no
// block is written at all — an empty "{}" reads back as no attributes
// anyway, so writing one would cost the fixed point.
func writeDirectiveAttrs(b *strings.Builder, attrs map[string]string) {
	if len(attrs) == 0 {
		return
	}
	id, hasID := attrs["id"]
	shortID := hasID && shorthandSpells(id)
	keys := make([]string, 0, len(attrs))
	for k, v := range attrs {
		if k == "id" && shortID {
			continue
		}
		if !keySpells(k) || !valueSpells(v) {
			continue
		}
		keys = append(keys, k)
	}
	if !shortID && len(keys) == 0 {
		return
	}
	sort.Strings(keys)
	b.WriteString("{")
	wrote := false
	if shortID {
		b.WriteString("#")
		b.WriteString(id)
		wrote = true
	}
	for _, k := range keys {
		if wrote {
			b.WriteString(" ")
		}
		b.WriteString(k)
		// Empty-string values serialize as a bare attribute name.
		if v := attrs[k]; v != "" {
			writeDirectiveAttrValue(b, v)
		}
		wrote = true
	}
	b.WriteString("}")
}

// mustSpellDirectiveName stops the render when a directive name has no
// written form, rather than writing one that destroys the node.
//
// This is the one member of the spell-check family (nameSpells,
// keySpells, valueSpells, shorthandSpells) that cannot degrade. An
// unspellable attribute is DROPPED: the directive survives, one
// attribute poorer. A name has no such fallback — a directive without a
// name is not a directive — and writing it anyway is silent corruption:
// the form re-parses as an ordinary paragraph (the node is gone), or, if
// the name carries a line ending, under the truncated prefix with its
// attribute block stranded on the next line (the node comes back
// renamed and empty). Sanitizing is worse still: it renames the
// author's directive without telling anyone.
//
// So the render panics. The name can only have come from a caller
// building the node by hand, because no parse can produce one — the
// parser refuses the whole directive instead — so the panic converts a
// corrupt document into a stack trace pointing at the construction site.
// The renderer's own directive names are literals or come from a
// closed set (the registered directive names, the five panel types, the
// two alignments, the Confluence macro keys), and every value the ADF
// decode leg can put in a name position goes through one of those sets,
// so no document can reach this panic.
func mustSpellDirectiveName(name string) {
	if nameSpells(name) {
		return
	}
	panic("adfast/markdown: cannot render the directive name " + strconv.Quote(name) +
		": a name is one or more ASCII alphanumerics, with '-' or '_' allowed" +
		" inside it but not at either end. No parse produces such a name, so" +
		" this node was BUILT BY A CALLER rather than parsed — fix whatever" +
		" set the Name field. Writing it would destroy the node instead of" +
		" the name (the directive re-parses as plain text, or under a" +
		" truncated name with its attributes gone), and unlike an" +
		" unspellable attribute a name cannot be dropped.")
}

// nameSpells reports whether a directive name can be written into any of
// the three directive forms and read back as itself.
//
// The rule is goldmark-directive's scanDirectiveName: a name opens with
// an ASCII alphanumeric, continues over alphanumerics and '-'/'_' runs,
// and must not END in '-' or '_' — a trailing joiner invalidates the
// whole directive, as it does in micromark. Anything outside that set
// (a space, a brace, a quote, a colon, a line ending, a multi-byte rune,
// the empty string) ends the parser's name scan early, and every form
// then rejects what follows: the marker line refuses trailing
// non-whitespace, and the text form needs a label or an attribute block
// right after the name.
//
// The grammar belongs to goldmark-directive, and this is a second copy
// of it, which is a real risk of drift. It is written here because that
// package exports no predicate for a name (scanDirectiveName and
// isDirectiveAlnum are unexported, and the exported surface is parsers,
// node types and options), and adding one there would tie this fix to a
// release of a separate public module. When goldmark-directive does
// export one, delete this and call it. TestRender_DirectiveNameSpellsMatchesTheParser
// pins the two together in the meantime: it checks the predicate against
// what Parse actually reads back, so a change to the upstream grammar
// fails here rather than drifting silently.
func nameSpells(name string) bool {
	if name == "" || !isDirectiveNameStart(name[0]) {
		return false
	}
	for i := range len(name) {
		if !isDirectiveNameByte(name[i]) {
			return false
		}
	}
	// The closing byte carries the same rule as the opening one: only an
	// alphanumeric may end a name, never a '-' or a '_'.
	return isDirectiveNameStart(name[len(name)-1])
}

// isDirectiveNameByte reports whether c can appear inside a directive
// name (goldmark-directive: an ASCII alphanumeric, a '-' or a '_').
func isDirectiveNameByte(c byte) bool {
	return isDirectiveNameStart(c) || c == '-' || c == '_'
}

// shorthandSpells reports whether the {#id} / {.class} shortcut can spell
// the value v — that is, whether the shortcut re-parses as v and nothing
// else.
//
// The rule is the parser's, not a guess: a shorthand token runs from the
// marker to the first attribute-boundary byte (isAttrBoundary — space,
// tab, CR, LF, an opening or closing brace, and either quote character),
// and an empty token invalidates the whole attribute block. So a value
// carrying any boundary byte is unspellable: the token stops early and
// the tail becomes separate attributes ({#a b} → id="a" + bare "b"), or
// the block is malformed and the directive loses its attributes
// altogether ({#a}b}). An empty value
// is unspellable for the same reason, and takes the bare-key long form
// ({id}) that every other empty-valued attribute already takes.
//
// The long form is always available instead, because a quoted value stops
// only at its own quote and writeDirectiveAttrValue picks the quote that
// survives.
func shorthandSpells(v string) bool {
	if v == "" {
		return false
	}
	for i := range len(v) {
		if isAttrBoundary(v[i]) {
			return false
		}
	}
	return true
}

// keySpells reports whether a directive attribute key k can be written
// into a `{…}` block and read back as itself.
//
// The rule is the parser's, read off two places rather than guessed. The
// key scan (scanAttrKeyValue) runs a key from its first byte to the first
// attribute-boundary byte (isAttrBoundary) or to the "=" that opens its
// value, and it rejects an empty key outright — which invalidates the
// whole block. So {a b="1"} comes back as a bare "a" plus b="1", and a
// key holding a brace, a quote or an "=" leaves the block malformed and
// the directive with no attributes at all.
//
// The block reader (scanDirectiveAttributes) adds the other half: it
// branches on the FIRST byte of each attribute, so a key opening with
// "#" or "." is read as the id/class shortcut instead of a key. That is
// the quieter failure of the two — {#k="v"} takes the block down, but a
// bare {#k} parses cleanly and simply arrives under the wrong name.
//
// Only the first byte carries that meaning, and the boundary set is a set
// of BYTES, so "a#b" is a fine key and no multi-byte rune can trip either
// rule.
func keySpells(k string) bool {
	if k == "" || k[0] == '#' || k[0] == '.' {
		return false
	}
	for i := range len(k) {
		if isAttrBoundary(k[i]) || k[i] == '=' {
			return false
		}
	}
	return true
}

// valueSpells reports whether a directive attribute value v can be
// written into a `{…}` block at all — in either spelling, since an
// attribute the renderer cannot write is dropped rather than corrupted.
//
// The rule is the parser's, not a guess: a quoted value runs to its own
// quote character and stops dead at a CR or an LF (scanAttrValue), and an
// unterminated value is not a bad attribute but a malformed BLOCK — the
// scan gives up and the directive re-parses with no attributes at all.
// writeDirectiveAttrValue already picks the quote character that
// survives, and the dialect has no escape for a line ending inside a
// quoted value, so a line ending is the one byte no long-form spelling
// holds. The shortcut has the same hole and refuses it already, because
// CR and LF are attribute-boundary bytes (see shorthandSpells).
//
// Every other byte is representable: a space, a tab, a brace and an
// equals sign are attribute boundaries only OUTSIDE quotes, and the two
// quote characters are the quote choice's job. A value carrying both of
// them is lossy but not malformed — that documented case stays on
// writeDirectiveAttrValue.
func valueSpells(v string) bool {
	return !strings.ContainsAny(v, "\r\n")
}

// writeDirectiveAttrValue serializes ="value" for a directive attribute,
// choosing the quote style that survives the round trip. A value carrying
// a double quote but no single quote is single-quoted so JSON payloads
// (e.g. extension parameters) stay readable and lossless:
// parameters='{"k":"v"}'. Values with no double quote (the common case)
// render as plain double-quoted attributes.
//
// A value carrying BOTH quote characters has no lossless spelling: the
// dialect has no escape inside a quoted attribute value, so neither quote
// can enclose it. It is double-quoted with every double quote written as
// the &quot; character reference, and that is where the round trip stops
// being lossless — Parse hands the six literal characters "&quot;" back
// as part of the value, because goldmark-directive does not decode
// character references in an attribute value. The one consumer the
// fallback exists for closes the loop itself: dialect.DecodeJSONAttr
// decodes &quot; before unmarshalling a JSON payload, which is why an
// extension's parameters survive both quotes. Any other attribute does
// not, and a caller that must not lose the value has to keep one of the
// two quote characters out of it. (remark decodes the reference for every
// attribute, so the spelling stays remark-compatible either way.)
//
// TestRender_DirectiveAttrValueQuoting pins each shape and what Parse
// reads back for it.
func writeDirectiveAttrValue(b *strings.Builder, v string) {
	if strings.Contains(v, `"`) && !strings.Contains(v, `'`) {
		b.WriteString("='")
		b.WriteString(v)
		b.WriteString("'")
		return
	}
	b.WriteString("=\"")
	b.WriteString(strings.ReplaceAll(v, `"`, "&quot;"))
	b.WriteString("\"")
}

// blockRenderContext implements extension.RenderContext in block
// position; the inline form is a no-op here.
type blockRenderContext struct {
	r *mdRenderer
	b *strings.Builder
}

// WriteContainerDirective implements extension.RenderContext.
func (c *blockRenderContext) WriteContainerDirective(name string, attrs map[string]string, children []ast.Node) {
	c.r.writeContainerDirectiveForm(c.b, name, attrs, children)
}

// WriteLeafDirective implements extension.RenderContext.
func (c *blockRenderContext) WriteLeafDirective(name string, attrs map[string]string, children []ast.Node) {
	writeLeafDirectiveForm(c.b, name, attrs, children)
}

// WriteTextDirective implements extension.RenderContext (inline form —
// no-op in block position).
func (*blockRenderContext) WriteTextDirective(string, map[string]string, []ast.Node) {}

// inlineRenderContext implements extension.RenderContext in inline
// position; the block forms are no-ops here.
type inlineRenderContext struct {
	r  *mdRenderer
	b  *strings.Builder
	st *inlineContext
}

// WriteContainerDirective implements extension.RenderContext (block form
// — no-op in inline position).
func (*inlineRenderContext) WriteContainerDirective(string, map[string]string, []ast.Node) {}

// WriteLeafDirective implements extension.RenderContext (block form —
// no-op in inline position).
func (*inlineRenderContext) WriteLeafDirective(string, map[string]string, []ast.Node) {}

// WriteTextDirective implements extension.RenderContext.
func (c *inlineRenderContext) WriteTextDirective(name string, attrs map[string]string, children []ast.Node) {
	c.r.writeTextDirectiveForm(c.b, name, attrs, children, c.st)
}

// escapeDirectiveLabel escapes the plain-string label of a leaf or
// container directive, so that the label the parse reads back is the
// string that went in.
//
// The whole function exists because of what its callers hand it: the
// label of these two forms is written from ast.PlainText over the label
// content and read back the same way, so the label DENOTES a plain
// string (a page title, a media alt, an expand title). Any byte the
// label's own inline parse reads as syntax is therefore lossy, not
// merely unstable — PlainText has no text for the construct's markers,
// and they never come back. So this escaper is derived from the parse,
// not from remark-stringify's unsafe list, and it diverges from remark
// wherever remark's spelling would lose a character:
//
//   - Brackets. Both are escaped: an unescaped ']' ends the label early
//     and an unescaped '[' leaves goldmark-directive's bracket-balancing
//     label scan unterminated.
//
//   - A backslash, where it could start an escape sequence. remark
//     writes "::media[\!0]" for the alt text "\!0", and re-parsing that
//     consumes the backslash. A trailing backslash is escaped for the
//     same reason — the "]" the caller writes next would become the
//     escaped byte, and the label would never terminate.
//
//   - A ':' that could open a nested text directive, which PlainText has
//     no text for at all. Unlike the prose escaper (which only protects
//     letter-led names, for remark parity) this covers digit-led names as
//     well: they are what goldmark-directive parses.
//
//   - The markdown marks '*', '_', '`' and '~', plus the '<' that opens
//     raw HTML or an autolink, and an '&' that opens a character
//     reference the parse would decode. These are the same case as the
//     backslash and the colon, measured the same way: the ADF title
//     "a *starred* title" was written bare as ":::expand[a *starred*
//     title]" and read back as "a starred title", and "an <html> title"
//     came back as "an  title" — a page title that resolves against no
//     page. (Found while probing a consumer that addresses a Confluence
//     page by its title through the Include Page macro's unnamed
//     parameter.) Measured over a 59-title battery round-tripped
//     ADF -> markdown -> ADF, 17 titles came back changed without these
//     escapes and 1 does with them. The three directive forms now agree:
//     the text form's label goes through the inline escaper, which has
//     always written these escapes (see writeEscapedByte).
//
// A mark that survives here reads back as a literal character, never as
// emphasis: the label is a string, so re-emitting a mark AS a mark would
// change what the directive means rather than preserve it.
//
// The '&' rule is keyed on the reference rather than on the byte because
// only a reference the parse DECODES costs characters. A bare '&' reads
// back as itself, and escaping it would be a divergence that buys
// nothing.
//
// This costs byte-exactness with the reference corpus for one probe,
// deliberately: remark writes a leaf and container label verbatim, so
// its recorded spelling of the title "With *chars* [x] back\slash" is
// itself lossy under the plain-string reading above. Lossless and
// byte-exact are mutually exclusive there, and the round trip wins. See
// the re-pin note in docs/design.md and TestDirectiveLabelRoundTrips.
//
// TRAILING WHITESPACE is the one case a backslash cannot carry, and
// escapeLabelTrailingSpace handles it after the byte pass below.
func escapeDirectiveLabel(s string) string {
	return escapeLabelTrailingSpace(escapeLabelBytes(s))
}

// escapeLabelTrailingSpace keeps a label's last whitespace byte by writing
// it as a character reference, which the label parse decodes back to the
// byte.
//
// The label parse trims the trailing whitespace of its content, so no
// backslash reaches it — a backslash before a space is not even an escape
// in CommonMark. Measured on the ADF expand title "trailing  ": the render
// wrote ":::expand[trailing  ]" and the re-parse read back "trailing ", so
// the title was not a fixpoint and a second format wrote ":::expand[trailing
// ]" — a title a consumer addresses a page by no longer matched the page.
// (A tab loses the whole run rather than one byte, and an all-whitespace
// label empties out.) This is the trailing half of what escapeLabelIndent
// does for a leading run, and it takes the same instrument for the same
// reason.
//
// One byte is enough, and that is the point of escaping the LAST one rather
// than the run: with a character reference at the end the label no longer
// ends in whitespace at all, so nothing is trimmed and the run in front of
// the reference is ordinary content. It also cannot push the label into
// indented-code territory the way an escape at the front could — the
// reference is written where no indent is measured.
//
// The text form does not need this and does not get it: its label is a run
// of inline content written by the inline escaper, which already keeps its
// trailing whitespace (measured: ":sup[trailing  ]" is a fixpoint).
func escapeLabelTrailingSpace(s string) string {
	if s == "" || !isLabelSpace(s[len(s)-1]) {
		return s
	}
	return s[:len(s)-1] + hexRef(rune(s[len(s)-1]))
}

// isLabelSpace reports whether c is a byte the label parse reads as
// whitespace, which is goldmark's own space set (util.IsSpace) minus the
// line endings no single-line label can hold.
func isLabelSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f'
}

// escapeLabelBytes is escapeDirectiveLabel's byte pass; see its doc comment
// for every rule below.
func escapeLabelBytes(s string) string {
	if !strings.ContainsAny(s, "[]\\:*_`~<&") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for i := range len(s) {
		switch s[i] {
		case '[', ']', '*', '_', '`', '~', '<':
			sb.WriteByte('\\')
		case '\\':
			if i+1 == len(s) || isASCIIPunct(s[i+1]) {
				sb.WriteByte('\\')
			}
		case '&':
			// Only a reference the parse decodes costs characters; a bare
			// '&' reads back as itself.
			if startsCharacterReference(s, i) {
				sb.WriteByte('\\')
			}
		case ':':
			// A directive name starts with an alphanumeric; a ':' before
			// anything else cannot open one.
			if i+1 < len(s) && isDirectiveNameStart(s[i+1]) {
				sb.WriteByte('\\')
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// escapeLabelIndent keeps a text-directive label out of indented-code
// territory by writing its first whitespace byte as a character
// reference, which the label parse decodes back to the byte.
//
// A text-directive label is parsed as block content, so a label opening
// with a whitespace run that reaches the 4-column indent is an indented
// code block, where nothing is parsed and escapes stay literal: the
// label ":u[    \*]" reads back a literal backslash, and every re-format
// escapes the survivor again (probe: "00:u[    *]0", whose format grew a
// backslash per pass). The reference is one column wide, so the run that
// follows it can no longer reach four.
//
// Leaf and container labels do not need this: they are read back through
// ast.PlainText over inline content, which resolves the escape.
func escapeLabelIndent(s string) string {
	if !labelIndentsToCode(s) {
		return s
	}
	return hexRef(rune(s[0])) + s[1:]
}

// labelIndentsToCode reports whether a label opens with a whitespace run
// reaching the 4-column indent goldmark reads as an indented code block.
// A tab advances to the next 4-column stop, so a leading tab reaches it
// alone.
func labelIndentsToCode(s string) bool {
	col := 0
	for i := 0; i < len(s) && col < 4; i++ {
		switch s[i] {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return false
		}
	}
	return col >= 4
}

// isDirectiveNameStart reports whether c can begin a directive name
// (goldmark-directive: an ASCII alphanumeric).
func isDirectiveNameStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// codeColonFenceRe matches a code-block line that could read as a
// container close fence when rendered verbatim inside one (up to 3
// leading spaces, only colons and trailing whitespace).
var codeColonFenceRe = regexp.MustCompile(`(?m)^ {0,3}(:{3,})[ \t]*$`)

// containerFenceLength returns the fence length for a container with
// the given body: it must outsize every fence-like line the body
// renders — nested container fences (their own recursive length, like
// remark-stringify's :::: > ::: growth) and close-fence-looking lines
// inside code blocks (which would otherwise close the container
// verbatim on re-parse).
func containerFenceLength(children []ast.Node) int {
	need := 3
	for _, child := range children {
		if threat := fenceThreat(child); threat > 0 {
			need = max(need, threat+1)
		}
	}
	return need
}

// fenceThreat is the longest fence-like line node n renders at
// container-closing indentation (0 when none): a container form's own
// fence, a code block's colon-run lines, or the largest threat among
// the node's children.
func fenceThreat(n ast.Node) int {
	if isContainerDirectiveForm(n) {
		return containerFenceLength(ast.Children(n))
	}
	if code, ok := n.(*ast.Code); ok {
		longest := 0
		for _, m := range codeColonFenceRe.FindAllStringSubmatch(code.Value, -1) {
			longest = max(longest, len(m[1]))
		}
		return longest
	}
	threat := 0
	for _, child := range ast.Children(n) {
		threat = max(threat, fenceThreat(child))
	}
	return threat
}

// escapeImageAlt escapes an image alt for the ![…] label: backslashes
// (which would re-read as escapes) and brackets (which would unbalance
// the label), like remark-stringify's label escaping.
func escapeImageAlt(s string) string {
	if !strings.ContainsAny(s, "[]\\") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for i := range len(s) {
		switch s[i] {
		case '[', ']', '\\':
			sb.WriteByte('\\')
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// isContainerDirectiveForm reports whether the node renders as a
// :::fenced container directive.
func isContainerDirectiveForm(n ast.Node) bool {
	if _, ok := n.(*ast.ContainerDirective); ok {
		return true
	}
	_, ok := n.(extension.ContainerForm)
	return ok
}
