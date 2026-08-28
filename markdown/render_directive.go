package markdown

import (
	"regexp"
	"sort"
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
func writeDirectiveAttrs(b *strings.Builder, attrs map[string]string) {
	if len(attrs) == 0 {
		return
	}
	b.WriteString("{")
	wrote := false
	id, hasID := attrs["id"]
	shortID := hasID && shorthandSpells(id)
	if shortID {
		b.WriteString("#")
		b.WriteString(id)
		wrote = true
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		if k != "id" || !shortID {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
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

// escapeDirectiveLabel escapes a directive [label]: remark-stringify only
// treats brackets as unsafe inside labels (markdown marks like * stay
// verbatim, and are flattened away by ast.PlainText on re-parse anyway —
// see the directive fixtures).
//
// A backslash is verbatim-safe only where it cannot start an escape
// sequence. One that can is escaped here, a deliberate divergence:
// remark writes "::media[\!0]" for the alt text "\!0", and re-parsing
// that consumes the backslash, so the label is LOSSY rather than merely
// unstable. A trailing backslash is escaped for the same reason — the
// "]" this function's caller writes next would be the escaped byte, and
// the label would never terminate.
//
// A ':' that could open a nested text directive is escaped too, a second
// deliberate divergence. Label content is parsed as inline markdown, so
// ":0" inside a label becomes a text directive node, and the label is
// read back from ast.PlainText, which has no text for it — the content
// vanishes. Unlike the prose escaper (which only protects letter-led
// names, for remark parity) this covers digit-led names as well: they
// are what goldmark-directive parses, and inside a label the divergence
// is lossy rather than cosmetic.
func escapeDirectiveLabel(s string) string {
	if !strings.ContainsAny(s, `[]\:`) {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for i := range len(s) {
		switch s[i] {
		case '[', ']':
			sb.WriteByte('\\')
		case '\\':
			if i+1 == len(s) || isASCIIPunct(s[i+1]) {
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
