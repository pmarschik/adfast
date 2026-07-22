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
// Generic containers render without attributes, matching the historical
// degradation path; the typed dialect kinds pass their attrs explicitly.
func (r *mdRenderer) renderContainerDirective(b *strings.Builder, node *ast.ContainerDirective, _ int) {
	r.writeContainerDirectiveForm(b, node.Name, nil, node.Children)
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
func writeDirectiveAttrs(b *strings.Builder, attrs map[string]string) {
	if len(attrs) == 0 {
		return
	}
	b.WriteString("{")
	wrote := false
	if id := attrs["id"]; id != "" {
		b.WriteString("#")
		b.WriteString(id)
		wrote = true
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		if k != "id" {
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

// writeDirectiveAttrValue serializes ="value" for a directive attribute,
// choosing a quote style that survives the round trip. A value carrying
// a double quote but no single quote is single-quoted so JSON payloads
// (e.g. extension parameters) stay readable and lossless:
// parameters='{"k":"v"}'. Otherwise the value is double-quoted, with any
// double quote written as the &quot; character reference (the fallback
// the parser decodes back — used when the value also contains a single
// quote, so single-quoting would not be lossless). Values with no double
// quote (the common case) render as plain double-quoted attributes.
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
// treats brackets as unsafe inside labels (markdown marks like * and even
// backslashes stay verbatim — see the directive fixtures).
func escapeDirectiveLabel(s string) string {
	if !strings.ContainsAny(s, "[]") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for i := range len(s) {
		switch s[i] {
		case '[', ']':
			sb.WriteByte('\\')
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
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
