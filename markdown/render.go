package markdown

import (
	"slices"
	"strings"

	"github.com/pmarschik/adfast/ast"
)

// This file renders an AST tree to Markdown text. It is the only place in
// the package that produces Markdown syntax; every remark-stringify parity
// quirk (escaping, wrapping, bullet alternation, lazy ordered numbering,
// loose/tight list spacing) is implemented here, keyed by AST node type.

type renderConfig struct {
	blockSep  string // separator written between top-level blocks (default "\n")
	wrapWidth int    // max paragraph width; 0 = no wrap (default 80)
	// prettierText switches text escaping from remark's rules to
	// prettier's (intraword '_' stays bare). Used by the prettier-format render mode.
	prettierText bool
}

// RenderOption configures Render.
type RenderOption func(*renderConfig)

// WithBlockSeparator sets the string written between consecutive top-level
// blocks. The default is "\n", producing a blank line (each block already
// ends with "\n"). Pass "" to suppress blank lines between blocks.
func WithBlockSeparator(sep string) RenderOption {
	return func(c *renderConfig) { c.blockSep = sep }
}

// WithNoWrap disables the 80-column paragraph wrapping, preserving long
// lines without wrapping like remark-stringify's default output.
func WithNoWrap() RenderOption {
	return func(c *renderConfig) { c.wrapWidth = 0 }
}

// WithPrintWidth sets a custom paragraph wrapping width. Pass 0 to disable wrapping.
func WithPrintWidth(width int) RenderOption {
	return func(c *renderConfig) { c.wrapWidth = width }
}

// WithPrettierText switches text escaping and inline serialization from
// remark's rules to prettier's. The prettier formatter enables it while
// rendering the escape-preserving source form carried on ast.Text.Raw (see
// PreservedEscapes).
func WithPrettierText() RenderOption {
	return func(c *renderConfig) { c.prettierText = true }
}

// Render serializes an AST tree to Markdown text: the render half of
// ToMarkdown and the counterpart of Parse.
func Render(root ast.Node, opts ...RenderOption) string {
	cfg := renderConfig{blockSep: "\n", wrapWidth: 80}
	for _, o := range opts {
		o(&cfg)
	}
	r := mdRenderer{cfg: cfg}
	return r.render(root)
}

type mdRenderer struct {
	cfg renderConfig
	// prefixWidth counts the columns consumed by enclosing blockquote
	// markers and list indentation. Paragraph wrapping budgets the final
	// rendered line (like prettier), not the bare inline text.
	prefixWidth int
}

// availWidth returns the wrap width left after enclosing prefixes.
func (r *mdRenderer) availWidth() int {
	if r.cfg.wrapWidth <= 0 {
		return 0
	}
	return max(1, r.cfg.wrapWidth-r.prefixWidth)
}

// render converts an AST root node to a Markdown string.
func (r *mdRenderer) render(root ast.Node) string {
	children := ast.Children(root)
	if len(children) == 0 {
		return "\n"
	}
	var b strings.Builder
	r.renderBlockSequence(&b, children, r.cfg.blockSep)
	// Restore spaces masked as unbreakable during rendering (links) and
	// ensure a single trailing newline.
	out := strings.ReplaceAll(b.String(), string(wrapMask), " ")
	out = strings.ReplaceAll(out, string(wrapMaskTab), "\t")
	return strings.TrimRight(out, "\n") + "\n"
}

// breakSafeBullet flips a '-' bullet to '*' when any item starts with a
// thematic break ("- ---" would re-parse as one long thematic break);
// remark flips the whole list the same way (see the "* ___" probes in the
// ewyh bead).
func breakSafeBullet(bullet string, node *ast.List) string {
	if bullet != "-" || node.Ordered {
		return bullet
	}
	for i := range node.Children {
		kids := ast.Children(node.Children[i])
		if len(kids) > 0 {
			if _, ok := kids[0].(*ast.ThematicBreak); ok {
				return "*"
			}
		}
	}
	return bullet
}

// currentLine returns the builder's content since the last newline.
func currentLine(b *strings.Builder) string {
	out := b.String()
	if i := strings.LastIndexByte(out, '\n'); i >= 0 {
		return out[i+1:]
	}
	return out
}

// fixMarkerBreakSuffix flips the final bullet marker of suffix when
// prefix+suffix forms a line of three or more identical bullet markers
// separated only by spaces (a thematic break on re-parse). The innermost
// marker always sits in suffix, which ends a marker-only chain.
func fixMarkerBreakSuffix(prefix, suffix string) string {
	line := prefix + suffix
	var marker byte
	count := 0
	for i := range len(line) {
		switch c := line[i]; c {
		case ' ':
		case '-', '*':
			if marker == 0 {
				marker = c
			} else if c != marker {
				return suffix
			}
			count++
		default:
			return suffix
		}
	}
	if count < 3 || suffix == "" || suffix[len(suffix)-1] != marker {
		return suffix
	}
	other := byte('*')
	if marker == '*' {
		other = '-'
	}
	return suffix[:len(suffix)-1] + string(other)
}

// withoutBlankParagraphs drops the paragraphs that would render to nothing.
//
// ADF carries empty paragraphs — Confluence's editor emits them as spacing —
// but Markdown has no way to write one: a blank line between blocks is
// separation, not content, so parsing can never give one back. Rendering them
// as the blank lines they turn into makes the output re-parse to a different
// tree, which shows up as a spurious diff on the first format pass. Dropping
// them is what makes rendering a fixed point, wherever the run of blocks sits
// (a container body, a blockquote, the document itself).
//
// A directive label is never dropped: it is not body content, and the container
// renderer peels it off the front before it gets here.
func withoutBlankParagraphs(children []ast.Node) []ast.Node {
	blank := func(node ast.Node) bool {
		p, ok := node.(*ast.Paragraph)
		return ok && !p.DirectiveLabel && ast.PlainText(p.Children) == "" && !hasVisibleInline(p.Children)
	}
	if !slices.ContainsFunc(children, blank) {
		return children
	}
	out := make([]ast.Node, 0, len(children))
	for _, node := range children {
		if !blank(node) {
			out = append(out, node)
		}
	}
	return out
}

// hasVisibleInline reports whether a run of inline nodes renders as anything at
// all — a paragraph holding only an empty text node is blank, one holding an
// image or a media chip is not, and neither carries plain text.
func hasVisibleInline(children []ast.Node) bool {
	for _, node := range children {
		switch node.(type) {
		case *ast.Text, *ast.Emphasis, *ast.Strong, *ast.Delete, *ast.Link:
			if ast.PlainText([]ast.Node{node}) != "" {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// renderBlockSequence renders sibling blocks separated by sep, alternating
// list markers for consecutive lists (unordered bullets '-'/'*', ordered
// delimiters '.'/')') like remark-stringify so adjacent lists don't merge
// on re-parse. Used at the document root and inside blockquotes and
// container directives.
func (r *mdRenderer) renderBlockSequence(b *strings.Builder, children []ast.Node, sep string) {
	children = withoutBlankParagraphs(children)
	prevBullet := ""
	prevOrderedList := false
	for i := range children {
		node := children[i]
		if i > 0 {
			b.WriteString(sep)
		}
		list, isList := node.(*ast.List)
		switch {
		case isList && !list.Ordered:
			// Plain and task lists share one bullet alternation chain in
			// remark-stringify: flip to the other bullet when the previous
			// list used the primary one OR the list needs the break-safe
			// bullet (one flag, so both reasons together still yield '*',
			// like remark).
			bullet := "-"
			if prevBullet == "-" || breakSafeBullet("-", list) != "-" {
				bullet = "*"
			}
			if isTaskList(list) {
				r.renderTaskList(b, list, bullet)
			} else {
				r.renderList(b, list, "", bullet, ".")
			}
			prevBullet = bullet
			prevOrderedList = false
		case isList && list.Ordered && !isTaskList(list):
			delim := "."
			if prevOrderedList {
				delim = ")"
			}
			r.renderList(b, list, "", "-", delim)
			prevOrderedList = !prevOrderedList
			prevBullet = ""
		default:
			r.renderBlock(b, node, 0)
			prevBullet = ""
			if isList && isTaskList(list) {
				prevBullet = "-"
			}
			prevOrderedList = false
		}
	}
}

// isTaskList reports whether any list item carries a task checkbox state.
func isTaskList(node *ast.List) bool {
	for i := range node.Children {
		if item, ok := node.Children[i].(*ast.ListItem); ok && item.Checked != nil {
			return true
		}
	}
	return false
}
