package markdown

import (
	"strings"
	"unicode"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// Block rendering: the block visitor and the per-kind block emitters
// (headings, code fences, blockquotes). Split from render.go; see that
// file's comment for the renderer contract.

// ---------------------------------------------------------------------------
// Block rendering
// ---------------------------------------------------------------------------

func (r *mdRenderer) renderBlock(b *strings.Builder, node ast.Node, depth int) {
	ast.Visit(node, &blockRenderVisitor{r: r, b: b, depth: depth})
}

// blockRenderVisitor renders one block node into b (the visit result is
// struct{}: rendering is a pure side effect on the builder).
// Implementing ast.Visitor keeps the block renderer exhaustive: a new
// AST kind fails compilation here until it gets an explicit rendering or
// the fallback.
type blockRenderVisitor struct {
	r     *mdRenderer
	b     *strings.Builder
	depth int
}

// VisitParagraph implements ast.Visitor.
func (v *blockRenderVisitor) VisitParagraph(n *ast.Paragraph) struct{} {
	inner := v.r.renderInlineString(n.Children)
	v.b.WriteString(escapeParagraphLeadingMarker(wrapTextProtected(inner, v.r.availWidth()), v.r.cfg.prettierText))
	v.b.WriteString("\n")
	return struct{}{}
}

// VisitHeading implements ast.Visitor.
func (v *blockRenderVisitor) VisitHeading(n *ast.Heading) struct{} {
	v.r.renderHeading(v.b, n)
	return struct{}{}
}

// VisitThematicBreak implements ast.Visitor.
func (v *blockRenderVisitor) VisitThematicBreak(*ast.ThematicBreak) struct{} {
	v.b.WriteString("---\n")
	return struct{}{}
}

// VisitBlockquote implements ast.Visitor.
func (v *blockRenderVisitor) VisitBlockquote(n *ast.Blockquote) struct{} {
	v.r.renderBlockquote(v.b, n.Children)
	return struct{}{}
}

// VisitCode implements ast.Visitor.
func (v *blockRenderVisitor) VisitCode(n *ast.Code) struct{} {
	v.r.renderCodeBlock(v.b, n)
	return struct{}{}
}

// VisitList implements ast.Visitor.
//
// The list renders unindented: every caller (renderBlockSequence, and
// renderItemFirstBlock/renderItemFollowBlock via their childIndent)
// places the block itself. Indenting by v.depth here counted the nesting
// twice, and at depth 2 the four extra columns re-parsed the item's
// first line as indented code ("- - - 0)" round-tripped into a code
// block).
func (v *blockRenderVisitor) VisitList(n *ast.List) struct{} {
	if isTaskList(n) {
		v.r.renderTaskList(v.b, n, "-")
	} else {
		v.r.renderList(v.b, n, "", "-", ".")
	}
	return struct{}{}
}

// VisitContainerDirective implements ast.Visitor.
func (v *blockRenderVisitor) VisitContainerDirective(n *ast.ContainerDirective) struct{} {
	v.r.renderContainerDirective(v.b, n, v.depth)
	return struct{}{}
}

// VisitLeafDirective implements ast.Visitor.
func (v *blockRenderVisitor) VisitLeafDirective(n *ast.LeafDirective) struct{} {
	renderLeafDirective(v.b, n)
	return struct{}{}
}

// VisitTable implements ast.Visitor.
func (v *blockRenderVisitor) VisitTable(n *ast.Table) struct{} {
	v.r.renderTable(v.b, n)
	return struct{}{}
}

// VisitHTML implements ast.Visitor.
func (v *blockRenderVisitor) VisitHTML(n *ast.HTML) struct{} {
	v.b.WriteString(n.Value)
	v.b.WriteString("\n")
	return struct{}{}
}

// VisitFrontmatter implements ast.Visitor.
func (v *blockRenderVisitor) VisitFrontmatter(n *ast.Frontmatter) struct{} {
	// A leading metadata block (YAML frontmatter or a custom
	// FrontmatterProvider's raw header): emitted verbatim, like
	// prettier leaves already-formatted frontmatter untouched.
	v.b.WriteString(n.Value)
	if !strings.HasSuffix(n.Value, "\n") {
		v.b.WriteString("\n")
	}
	return struct{}{}
}

// VisitExtension implements ast.Visitor.
func (v *blockRenderVisitor) VisitExtension(n ast.Node) struct{} {
	if ext, ok := n.(extension.Node); ok {
		// Extension kinds (the typed dialect and consumer-registered
		// ones) render themselves through the controlled primitives.
		ext.RenderMarkdown(&blockRenderContext{r: v.r, b: v.b})
		return struct{}{}
	}
	return v.blockFallback(n)
}

// blockFallback handles the kinds without a dedicated block rendering.
func (v *blockRenderVisitor) blockFallback(node ast.Node) struct{} {
	// Unknown block: try to recurse into content
	if kids := ast.Children(node); len(kids) > 0 {
		return ast.Visit(kids[0], v)
	}
	return struct{}{}
}

// The remaining kinds have no dedicated block rendering: structural
// children (list items, table rows/cells) and inline kinds in block
// position degrade through the recurse-into-first-child fallback.

// VisitRoot implements ast.Visitor.
func (v *blockRenderVisitor) VisitRoot(n *ast.Root) struct{} { return v.blockFallback(n) }

// VisitListItem implements ast.Visitor.
func (v *blockRenderVisitor) VisitListItem(n *ast.ListItem) struct{} { return v.blockFallback(n) }

// VisitTableRow implements ast.Visitor.
func (v *blockRenderVisitor) VisitTableRow(n *ast.TableRow) struct{} { return v.blockFallback(n) }

// VisitTableCell implements ast.Visitor.
func (v *blockRenderVisitor) VisitTableCell(n *ast.TableCell) struct{} { return v.blockFallback(n) }

// VisitText implements ast.Visitor.
func (v *blockRenderVisitor) VisitText(n *ast.Text) struct{} { return v.blockFallback(n) }

// VisitEmphasis implements ast.Visitor.
func (v *blockRenderVisitor) VisitEmphasis(n *ast.Emphasis) struct{} { return v.blockFallback(n) }

// VisitStrong implements ast.Visitor.
func (v *blockRenderVisitor) VisitStrong(n *ast.Strong) struct{} { return v.blockFallback(n) }

// VisitDelete implements ast.Visitor.
func (v *blockRenderVisitor) VisitDelete(n *ast.Delete) struct{} { return v.blockFallback(n) }

// VisitInlineCode implements ast.Visitor.
func (v *blockRenderVisitor) VisitInlineCode(n *ast.InlineCode) struct{} { return v.blockFallback(n) }

// VisitBreak implements ast.Visitor.
func (v *blockRenderVisitor) VisitBreak(n *ast.Break) struct{} { return v.blockFallback(n) }

// VisitLink implements ast.Visitor.
func (v *blockRenderVisitor) VisitLink(n *ast.Link) struct{} { return v.blockFallback(n) }

// VisitImage implements ast.Visitor.
func (v *blockRenderVisitor) VisitImage(n *ast.Image) struct{} { return v.blockFallback(n) }

// VisitTextDirective implements ast.Visitor.
func (v *blockRenderVisitor) VisitTextDirective(n *ast.TextDirective) struct{} {
	return v.blockFallback(n)
}

// renderHeading renders an ATX heading line.
func (r *mdRenderer) renderHeading(b *strings.Builder, node *ast.Heading) {
	level := min(max(node.Depth, 1), 6)
	inner := r.renderInlineStringFrom(node.Children, ' ')
	// The {#id} anchor trails the heading text, separated by a space
	// (nothing to separate it from when the heading is anchor-only).
	// Without one, a heading whose own text ends in the anchor shape must
	// escape its brace or it would re-parse as an anchor.
	// An id outside ast.HeadingIDPattern has no writable suffix form (it
	// would not parse back as an anchor), so it drops here rather than
	// rendering a broken one — the same way ToADF drops a frontmatter node
	// that has no ADF form. Only a hand-built or decoded tree can hold one;
	// the parser cannot produce it.
	suffix := ""
	switch {
	case ast.ValidHeadingID(node.ID):
		suffix = " {#" + node.ID + "}"
		if inner == "" {
			suffix = suffix[1:]
		}
	default:
		inner = escapeHeadingAnchorTail(inner)
	}
	// ATX headings cannot span lines. A heading containing a hard break
	// falls back to the setext form for levels 1–2 (remark does the
	// same); deeper levels encode the line ending as a character
	// reference.
	if strings.ContainsRune(inner, '\n') {
		if level <= 2 {
			underline := "="
			if level == 2 {
				underline = "-"
			}
			b.WriteString(inner)
			b.WriteString(suffix)
			b.WriteString("\n")
			b.WriteString(underline)
			b.WriteString("\n")
			return
		}
		inner = strings.ReplaceAll(inner, "\\\n", "&#xA;")
		inner = strings.ReplaceAll(inner, "  \n", "&#xA;")
		inner = strings.ReplaceAll(inner, "\n", "&#xA;")
	}
	b.WriteString(strings.Repeat("#", level))
	b.WriteString(" ")
	// A trailing '#' would re-parse as an ATX closing sequence; remark
	// escapes the final one. An anchor suffix already ends the line, so
	// the '#' is no longer final and needs no escape.
	if suffix == "" && strings.HasSuffix(inner, "#") {
		inner = inner[:len(inner)-1] + `\#`
	}
	b.WriteString(inner)
	b.WriteString(suffix)
	b.WriteString("\n")
}

// renderCodeBlock renders a fenced code block. The fence must be longer than
// any backtick run in the content (remark-stringify grows it), or the block
// re-parses truncated.
func (r *mdRenderer) renderCodeBlock(b *strings.Builder, node *ast.Code) {
	fence := "```"
	run, maxRun := 0, 0
	for i := range len(node.Value) {
		if node.Value[i] == '`' {
			run++
			maxRun = max(maxRun, run)
		} else {
			run = 0
		}
	}
	if maxRun >= 3 {
		fence = strings.Repeat("`", maxRun+1)
	}
	b.WriteString(fence)
	// Info strings re-escape their backslashes and hex-encode backticks
	// and whitespace (a raw backtick would terminate the fence, a raw
	// backslash would swallow the next character, and raw whitespace
	// would end the language word — remark renders "&#x9;" etc.).
	lang := strings.ReplaceAll(node.Lang, `\`, `\\`)
	lang = strings.ReplaceAll(lang, "`", "&#x60;")
	var lb strings.Builder
	for _, r0 := range lang {
		if unicode.IsSpace(r0) {
			lb.WriteString(hexRef(r0))
			continue
		}
		lb.WriteRune(r0)
	}
	b.WriteString(lb.String())
	b.WriteString("\n")
	if node.Value != "" {
		value := node.Value
		if r.cfg.prettierText {
			// Prettier trims trailing whitespace inside code blocks.
			lines := strings.Split(value, "\n")
			for i, l := range lines {
				lines[i] = strings.TrimRight(l, " \t")
			}
			value = strings.Join(lines, "\n")
		}
		b.WriteString(value)
		b.WriteString("\n")
	}
	b.WriteString(fence)
	b.WriteString("\n")
}

func (r *mdRenderer) renderBlockquote(b *strings.Builder, content []ast.Node) {
	var inner strings.Builder
	r.prefixWidth += 2 // "> "
	defer func() { r.prefixWidth -= 2 }()
	r.renderBlockSequence(&inner, content, "\n")
	for line := range strings.SplitSeq(strings.TrimRight(inner.String(), "\n"), "\n") {
		if line == "" {
			b.WriteString(">\n")
		} else {
			b.WriteString("> ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
}
