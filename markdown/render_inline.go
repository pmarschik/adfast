package markdown

import (
	"strings"
	"unicode"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// Inline rendering: the inline visitor, mark constructs, links,
// images, code spans, and the CommonMark flanking checks that drive
// remark's character-reference encoding. Split from render.go.

// ---------------------------------------------------------------------------
// Inline rendering
// ---------------------------------------------------------------------------

// inlineContext tracks the serializer state that remark-stringify's escape
// rules depend on: the previously emitted character ('\n' at block start;
// unset at the start of emphasis/strong/delete/link constructs) and which
// escape families are active in the current construct.
type inlineContext struct {
	// prevRune is the last rune actually emitted (syntax included), used for
	// CommonMark flanking checks; '\n' at block start counts as whitespace.
	prevRune rune
	// directiveHazard is the rune that would fuse onto a text directive
	// rendered at this position, 0 when none. writeTextDirectiveForm
	// answers it with the empty attribute block. See needsPunctTrail.
	directiveHazard rune
	prev            byte
	hasPrev         bool
	// nodePrev is the output byte preceding the text node being escaped
	// (st.prev advances byte by byte through it), so a check anchored at
	// the node's first byte can still see its left boundary. See
	// emailLiteralStarts.
	nodePrev    byte
	nodeHasPrev bool
	escape      bool // markdown character escaping (off inside link labels)
	colons      bool // directive colon escaping (off in table cells + link labels)
	pipes       bool // '|' escaping inside table cells (mdast-util-gfm-table)
	label       bool // inside a link label (atomic: no marker-risk escaping)
	// encodeLead asks the next text node to hex-encode its first
	// alphanumeric rune (set when an adjacent emphasis marker would not be
	// flankable — remark-stringify does the same with &#xNN; references).
	encodeLead bool
	// encodeTrail asks the LAST text node of the current child list to
	// hex-encode its final alphanumeric rune (problematic closer).
	encodeTrail bool
	// afterLead is the first text byte following the current (nested)
	// construct — mdast-util-to-markdown threads the parent's "after"
	// character into the last child's safety checks, so escapes like the
	// gfm-autolink '@' rule see across a closing emphasis marker.
	afterLead byte
	// directiveLabel marks content written inside a text directive's
	// [label], where a nested text directive is lossy rather than merely
	// unstable: the label is read back with ast.PlainText, which has no
	// text for a directive node. See escapesColon.
	directiveLabel bool
}

// renderInlineString renders inline nodes at the start of a block line
// (paragraph, heading, list item, blockquote line).
func (r *mdRenderer) renderInlineString(nodes []ast.Node) string {
	return r.renderInlineStringFrom(nodes, '\n')
}

// renderInlineStringFrom renders inline nodes with a given preceding output
// byte: '\n' for content at a line start (paragraphs), ' ' for content that
// follows syntax on the same line (ATX heading text), disabling the
// atBreak escape rules there like remark's tracker does.
func (r *mdRenderer) renderInlineStringFrom(nodes []ast.Node, prev byte) string {
	var b strings.Builder
	st := inlineContext{prev: prev, hasPrev: true, escape: true, colons: true, prevRune: rune(prev)}
	r.writeInlines(&b, nodes, &st)
	out := b.String()
	// Boundary whitespace would be stripped (or blank the line) on
	// re-parse; remark hex-encodes it ("0 " renders "0&#x20;", a
	// non-breaking-space paragraph renders "&#xA0;"). A caller that renders
	// only to compare, and never reads the result back, asks for the space
	// itself instead (WithoutSignificantSpaceEscapes).
	if !r.cfg.noSpaceEscapes {
		if r := lastRuneOf(out); r != 0 && r != '\n' && unicode.IsSpace(r) {
			out = out[:len(out)-len(string(r))] + hexRef(r)
		}
		if r := firstRuneOf(out); r != 0 && r != '\n' && unicode.IsSpace(r) {
			out = hexRef(r) + out[len(string(r)):]
		}
	}
	return out
}

// renderCellString renders inline nodes inside a table cell, where
// remark-stringify does not apply the phrasing colon-escape rule.
func (r *mdRenderer) renderCellString(nodes []ast.Node) string {
	var b strings.Builder
	st := inlineContext{prev: '\n', hasPrev: true, escape: true, colons: false, pipes: true, prevRune: '\n'}
	r.writeInlines(&b, nodes, &st)
	return b.String()
}

func (r *mdRenderer) writeInlines(b *strings.Builder, nodes []ast.Node, st *inlineContext) {
	nodes = joinAdjacentCodeSpans(nodes)
	v := &inlineWriteVisitor{r: r, b: b, st: st, nodes: nodes}
	for i := range nodes {
		v.i = i
		st.directiveHazard = r.needsPunctTrail(nodes, i, st)
		ast.Visit(nodes[i], v)
	}
	st.directiveHazard = 0
}

// joinAdjacentCodeSpans concatenates neighboring inline code nodes into
// one. Markdown cannot write two code spans back to back: the closing
// fence of the first and the opening fence of the second are one
// backtick run to the parser, and no fence length or padding splits it,
// so the spans "a" and "b" written in sequence re-parse as the single
// span holding "a", two backticks and "b". Joining is the only
// representable form, and it is the faithful one: adjacent code content
// with equal marks is one run in ADF too.
//
// The adjacency is reachable because the code mark is exclusive — an
// emphasis wrapping nothing but a code span drops in normalization, and
// its span lands beside its neighbor (probe: "`a`*`b`*", which
// normalizes to two sibling code spans).
func joinAdjacentCodeSpans(nodes []ast.Node) []ast.Node {
	joins := false
	for i := 1; i < len(nodes); i++ {
		if isInlineCode(nodes[i-1]) && isInlineCode(nodes[i]) {
			joins = true
			break
		}
	}
	if !joins {
		return nodes
	}
	out := make([]ast.Node, 0, len(nodes))
	for _, n := range nodes {
		code, ok := n.(*ast.InlineCode)
		if ok && len(out) > 0 {
			if prev, prevOK := out[len(out)-1].(*ast.InlineCode); prevOK {
				// A fresh node: the input tree is not the renderer's to edit.
				out[len(out)-1] = &ast.InlineCode{Value: prev.Value + code.Value}
				continue
			}
		}
		out = append(out, n)
	}
	return out
}

// isInlineCode reports whether n is an inline code span.
func isInlineCode(n ast.Node) bool {
	_, ok := n.(*ast.InlineCode)
	return ok
}

// inlineWriteVisitor writes the inline node at nodes[i] into b under the
// escape state st. The sibling slice and index live on the receiver
// because several constructs peek at their neighbors (emphasis flanking,
// tilde-run separation, afterLead threading). Implementing ast.Visitor
// keeps the inline writer exhaustive over the kind set.
type inlineWriteVisitor struct {
	r     *mdRenderer
	b     *strings.Builder
	st    *inlineContext
	nodes []ast.Node
	i     int
}

// The optional visitor interfaces are asserted, not inferred: without
// this the footnote kinds would silently fall through to VisitExtension.
var _ ast.FootnoteVisitor[struct{}] = (*inlineWriteVisitor)(nil)

// VisitText implements ast.Visitor.
func (v *inlineWriteVisitor) VisitText(*ast.Text) struct{} {
	v.r.writeTextInline(v.b, v.nodes, v.i, v.st)
	return struct{}{}
}

// VisitInlineCode implements ast.Visitor.
func (v *inlineWriteVisitor) VisitInlineCode(n *ast.InlineCode) struct{} {
	span := formatCodeSpan(n.Value)
	if v.st.pipes {
		// mdast-util-gfm-table escapes '|' even inside code spans
		// when serializing a table cell.
		span = strings.ReplaceAll(span, "|", "\\|")
	}
	v.b.WriteString(span)
	v.st.prev, v.st.hasPrev = '`', true
	v.st.prevRune, v.st.encodeLead = '`', false
	return struct{}{}
}

// VisitBreak implements ast.Visitor.
func (v *inlineWriteVisitor) VisitBreak(n *ast.Break) struct{} {
	v.r.writeHardBreak(v.b, n, v.st)
	return struct{}{}
}

// VisitStrong implements ast.Visitor.
func (v *inlineWriteVisitor) VisitStrong(*ast.Strong) struct{} {
	v.r.writeWrapped(v.b, v.nodes, v.i, "**", v.st)
	return struct{}{}
}

// VisitEmphasis implements ast.Visitor.
func (v *inlineWriteVisitor) VisitEmphasis(*ast.Emphasis) struct{} {
	v.r.writeWrapped(v.b, v.nodes, v.i, "_", v.st)
	return struct{}{}
}

// VisitDelete implements ast.Visitor.
func (v *inlineWriteVisitor) VisitDelete(*ast.Delete) struct{} {
	v.r.writeWrapped(v.b, v.nodes, v.i, "~~", v.st)
	return struct{}{}
}

// VisitLink implements ast.Visitor.
func (v *inlineWriteVisitor) VisitLink(n *ast.Link) struct{} {
	v.r.writeLink(v.b, n, v.st)
	v.st.prev, v.st.hasPrev = ')', true
	v.st.prevRune, v.st.encodeLead = ')', false
	return struct{}{}
}

// VisitTextDirective implements ast.Visitor.
func (v *inlineWriteVisitor) VisitTextDirective(n *ast.TextDirective) struct{} {
	v.r.writeTextDirective(v.b, n, v.st)
	return struct{}{}
}

// VisitFootnoteDef implements ast.FootnoteVisitor: a definition is a
// block, so in inline position it degrades to its content.
func (v *inlineWriteVisitor) VisitFootnoteDef(n *ast.FootnoteDef) struct{} {
	return v.inlineFallback(n)
}

// VisitFootnoteRef implements ast.FootnoteVisitor. The label is written
// verbatim — it is the identifier the definition pairs on, so nothing in
// it may be escaped away — with its spaces masked against wrapping (a
// line break inside the brackets would not re-parse as a reference) and
// its pipes escaped inside a table cell.
func (v *inlineWriteVisitor) VisitFootnoteRef(n *ast.FootnoteRef) struct{} {
	label := n.Label
	if v.st.pipes {
		label = strings.ReplaceAll(label, "|", `\|`)
	}
	label = strings.ReplaceAll(label, " ", string(wrapMask))
	label = strings.ReplaceAll(label, "\t", string(wrapMaskTab))
	v.b.WriteString("[^" + label + "]")
	v.st.prev, v.st.hasPrev = ']', true
	v.st.prevRune, v.st.encodeLead = ']', false
	return struct{}{}
}

// VisitImage implements ast.Visitor.
func (v *inlineWriteVisitor) VisitImage(n *ast.Image) struct{} {
	v.r.writeImage(v.b, n, v.st)
	return struct{}{}
}

// VisitHTML implements ast.Visitor.
func (v *inlineWriteVisitor) VisitHTML(n *ast.HTML) struct{} {
	// Inline raw HTML is written verbatim (prettier keeps it).
	v.b.WriteString(n.Value)
	if n.Value != "" {
		last := n.Value[len(n.Value)-1]
		v.st.prev, v.st.hasPrev = last, true
		v.st.prevRune = rune(last)
	}
	v.st.encodeLead = false
	return struct{}{}
}

// VisitExtension implements ast.Visitor.
func (v *inlineWriteVisitor) VisitExtension(n ast.Node) struct{} {
	if ext, ok := n.(extension.Node); ok {
		// Extension kinds render themselves through the controlled
		// primitives (state updates included).
		ext.RenderMarkdown(&inlineRenderContext{r: v.r, b: v.b, st: v.st})
		return struct{}{}
	}
	return v.inlineFallback(n)
}

// inlineFallback handles the kinds without a dedicated inline rendering:
// block kinds in inline position degrade by writing their children.
func (v *inlineWriteVisitor) inlineFallback(node ast.Node) struct{} {
	v.r.writeInlines(v.b, ast.Children(node), v.st)
	return struct{}{}
}

// VisitRoot implements ast.Visitor.
func (v *inlineWriteVisitor) VisitRoot(n *ast.Root) struct{} { return v.inlineFallback(n) }

// VisitParagraph implements ast.Visitor.
func (v *inlineWriteVisitor) VisitParagraph(n *ast.Paragraph) struct{} { return v.inlineFallback(n) }

// VisitHeading implements ast.Visitor.
func (v *inlineWriteVisitor) VisitHeading(n *ast.Heading) struct{} { return v.inlineFallback(n) }

// VisitThematicBreak implements ast.Visitor.
func (v *inlineWriteVisitor) VisitThematicBreak(n *ast.ThematicBreak) struct{} {
	return v.inlineFallback(n)
}

// VisitBlockquote implements ast.Visitor.
func (v *inlineWriteVisitor) VisitBlockquote(n *ast.Blockquote) struct{} {
	return v.inlineFallback(n)
}

// VisitList implements ast.Visitor.
func (v *inlineWriteVisitor) VisitList(n *ast.List) struct{} { return v.inlineFallback(n) }

// VisitListItem implements ast.Visitor.
func (v *inlineWriteVisitor) VisitListItem(n *ast.ListItem) struct{} { return v.inlineFallback(n) }

// VisitCode implements ast.Visitor.
func (v *inlineWriteVisitor) VisitCode(n *ast.Code) struct{} { return v.inlineFallback(n) }

// VisitFrontmatter implements ast.Visitor.
func (v *inlineWriteVisitor) VisitFrontmatter(n *ast.Frontmatter) struct{} {
	return v.inlineFallback(n)
}

// VisitTable implements ast.Visitor.
func (v *inlineWriteVisitor) VisitTable(n *ast.Table) struct{} { return v.inlineFallback(n) }

// VisitTableRow implements ast.Visitor.
func (v *inlineWriteVisitor) VisitTableRow(n *ast.TableRow) struct{} { return v.inlineFallback(n) }

// VisitTableCell implements ast.Visitor.
func (v *inlineWriteVisitor) VisitTableCell(n *ast.TableCell) struct{} { return v.inlineFallback(n) }

// VisitContainerDirective implements ast.Visitor.
func (v *inlineWriteVisitor) VisitContainerDirective(n *ast.ContainerDirective) struct{} {
	return v.inlineFallback(n)
}

// VisitLeafDirective implements ast.Visitor.
func (v *inlineWriteVisitor) VisitLeafDirective(n *ast.LeafDirective) struct{} {
	return v.inlineFallback(n)
}

// writeTextInline writes nodes[i] (a text node), deciding the boundary
// character-reference encodings against its neighbors: a problematic
// emphasis opener on the following construct encodes this text's final
// word-class rune, and a backslash-escaped tilde touching an adjacent
// strike marker run becomes a character reference so the tilde runs stay
// apart for goldmark's delimiter scan (see writeWrapped).
func (r *mdRenderer) writeTextInline(b *strings.Builder, nodes []ast.Node, i int, st *inlineContext) {
	node, ok := nodes[i].(*ast.Text)
	if !ok {
		return
	}
	lead := st.encodeLead
	st.encodeLead = false
	trail := st.encodeTrail && i == len(nodes)-1
	if i+1 < len(nodes) {
		if m := emphasisMarkerByte(nodes[i+1]); m != 0 {
			if !canOpenMarker(m, lastRuneOf(node.Value), r.renderedChildLead(nodes[i+1], st)) {
				trail = true
			}
		}
	}
	nl := nextTextLead(nodes, i)
	if nl == 0 && i == len(nodes)-1 {
		nl = st.afterLead
	}
	escaped := r.escapeText(node.Value, st, nl, lead, trail)
	if i+1 < len(nodes) && strings.HasSuffix(escaped, `\~`) {
		if _, ok := nodes[i+1].(*ast.Delete); ok {
			escaped = escaped[:len(escaped)-2] + "&#x7E;"
		}
	}
	if i > 0 && strings.HasPrefix(escaped, `\~`) {
		if _, ok := nodes[i-1].(*ast.Delete); ok {
			escaped = "&#x7E;" + escaped[2:]
		}
	}
	b.WriteString(escaped)
	if node.Value != "" {
		st.prevRune = lastRuneOf(node.Value)
	}
}

// writeHardBreak writes a hard line break: the trailing-space form when the
// source used it (prettier keeps it) or when it follows an escaped
// backslash (goldmark reads "\\\" + newline as a soft break; remark parses
// it per spec, and the trailing-space form parses identically in both);
// otherwise remark's backslash form.
//
// At a line start the trailing-space form cannot be kept: nothing precedes
// the two spaces, so they are the line's leading whitespace and are
// stripped on re-parse (probe: "  \n0" is one paragraph "0"). The source
// only reaches that shape when whatever preceded the break rendered to
// nothing (":emoji  \n0" — the emoji has no shortName to write), and the
// backslash form carries the break there instead.
func (r *mdRenderer) writeHardBreak(b *strings.Builder, node *ast.Break, st *inlineContext) {
	switch {
	case r.cfg.prettierText && node.Value == "  " && !atLineStart(st):
		b.WriteString("  \n")
	case strings.HasSuffix(b.String(), "\\"):
		b.WriteString("  \n")
	default:
		b.WriteString("\\\n")
	}
	st.prev, st.hasPrev = '\n', true
	st.prevRune, st.encodeLead = '\n', false
}

// writeImage serializes ![alt](url "title") as one unbreakable wrap unit,
// like links. Backslashes and brackets in the alt text are escaped
// (remark-stringify does the same) — unescaped they change the label
// reading on re-parse.
func (r *mdRenderer) writeImage(b *strings.Builder, node *ast.Image, st *inlineContext) {
	img := "![" + escapeImageAlt(ast.PlainText(node.Children)) + "](" + formatLinkURL(node.URL, r.cfg.prettierText)
	if node.Title != "" {
		img += " \"" + r.escapeTitle(node.Title) + "\""
	}
	img += ")"
	img = strings.ReplaceAll(img, " ", string(wrapMask))
	b.WriteString(strings.ReplaceAll(img, "\t", string(wrapMaskTab)))
	st.prev, st.hasPrev = ')', true
	st.prevRune, st.encodeLead = ')', false
}

// writeTextDirective serializes a generic text directive (the form-based
// helper does the work; see writeTextDirectiveForm).
func (r *mdRenderer) writeTextDirective(b *strings.Builder, node *ast.TextDirective, st *inlineContext) {
	r.writeTextDirectiveForm(b, node.Name, node.Attrs, node.Children, st)
}

// writeTextDirectiveForm serializes :name[label]{attrs} the way
// mdast-util-directive does: the id attribute uses the {#value} shortcut and
// other attributes are quoted key="value" pairs. The attribute block is
// one unbreakable wrap unit: goldmark-directive (unlike micromark's
// attribute factory) rejects line endings inside {…}, so a prose wrap
// there would break the re-parse — a deliberate, documented divergence
// from remark's wrapping.
func (r *mdRenderer) writeTextDirectiveForm(b *strings.Builder, name string, attrs map[string]string, children []ast.Node, st *inlineContext) {
	b.WriteString(":")
	b.WriteString(name)
	last := byte(0)
	if len(children) > 0 {
		b.WriteString("[")
		child := inlineContext{escape: st.escape, colons: st.colons, pipes: st.pipes, prevRune: '[', directiveLabel: true}
		// Into a temp builder: the label's own leading whitespace decides
		// whether the parse reads it as an indented code block, and that
		// is only knowable once the label is written. See escapeLabelIndent.
		var lb strings.Builder
		r.writeInlines(&lb, children, &child)
		b.WriteString(escapeLabelIndent(lb.String()))
		b.WriteString("]")
		last = ']'
	}
	if len(attrs) > 0 {
		var ab strings.Builder
		writeDirectiveAttrs(&ab, attrs)
		masked := strings.ReplaceAll(ab.String(), " ", string(wrapMask))
		b.WriteString(strings.ReplaceAll(masked, "\t", string(wrapMaskTab)))
		last = '}'
	}
	// A form that stopped short of an attribute block stays open to whatever
	// follows it, and remark's hex-encoding repair cannot reach back into a
	// directive without renaming it. The empty attribute block closes the
	// form: it is semantically inert (`:name{}` and `:name` parse to the same
	// node for every registered kind), it consumes the '{' that a following
	// brace would otherwise donate, and it ends the form in '}', which is
	// punctuation and so satisfies left-flanking. See needsPunctTrail.
	//
	// The bare form is exposed to every hazard — its tail is a name rune,
	// word class for every dialect name. The labeled form already ends in
	// ']', so only a following '{' can still reach it.
	if len(attrs) == 0 && (st.directiveHazard == '{' || (last == 0 && st.directiveHazard != 0)) {
		b.WriteString("{}")
		last = '}'
	}
	if last == 0 {
		last = lastRuneByteOf(name)
	}
	st.prev, st.hasPrev = last, true
	st.prevRune, st.encodeLead = rune(last), false
}

func lastRuneByteOf(s string) byte {
	if s == "" {
		return 0
	}
	return s[len(s)-1]
}

// writeWrapped renders the mark construct at nodes[i]. Children start
// without a "previous character" for escape purposes (remark-stringify does
// not apply before-dependent escapes at construct starts: `_:name_` keeps
// its colon unescaped while `(\:name)` does not), but flanking checks use
// the marker as the real preceding rune.
//
// When the marker would not be flankable in CommonMark ('_' cannot open or
// close intraword; '*' fails next to mixed punctuation), remark-stringify
// hex-encodes the adjacent alphanumeric runes so the emphasis survives
// re-parsing — mirrored here via encodeLead/encodeTrail.
func (r *mdRenderer) writeWrapped(b *strings.Builder, nodes []ast.Node, i int, marker string, st *inlineContext) {
	node := nodes[i]
	openProblem, closeProblem := false, false
	if m := emphasisMarkerByte(node); m != 0 {
		openProblem = !canOpenMarker(m, st.prevRune, r.renderedChildLead(node, st))
		closeProblem = !canCloseMarker(m, r.renderedChildTrail(node, st), siblingLeadRune(nodes, i+1))
	}
	after := nextTextLead(nodes, i)
	if after == 0 && i == len(nodes)-1 {
		after = st.afterLead
	}
	child := inlineContext{
		escape:         st.escape,
		colons:         st.colons,
		pipes:          st.pipes,
		prevRune:       rune(marker[len(marker)-1]),
		encodeLead:     openProblem,
		encodeTrail:    closeProblem,
		afterLead:      after,
		directiveLabel: st.directiveLabel,
	}
	var inner strings.Builder
	r.writeInlines(&inner, ast.Children(node), &child)
	content := inner.String()
	if marker == "~~" {
		// A backslash-escaped tilde touching the strike markers merges
		// into one tilde run for goldmark's delimiter scan (remark's
		// parser resolves the escape first); a character reference keeps
		// the runs apart.
		if strings.HasSuffix(content, `\~`) {
			content = content[:len(content)-2] + "&#x7E;"
		}
		if strings.HasPrefix(content, `\~`) {
			content = "&#x7E;" + content[2:]
		}
	}
	b.WriteString(marker)
	b.WriteString(content)
	b.WriteString(marker)
	st.prev, st.hasPrev = marker[len(marker)-1], true
	st.prevRune = rune(marker[len(marker)-1])
	st.encodeLead = closeProblem
}
