package markdown

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	directive "github.com/pmarschik/goldmark-directive"

	"github.com/pmarschik/adfast/ast"

	gast "github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// goldmarkToAst converts a parsed goldmark AST into an AST root node.
// This is the parse-side AST→AST half of FromMarkdown: goldmark nodes
// reference raw source bytes, so all source extraction and CommonMark escape
// decoding happens here; the resulting AST tree is source-independent.
// urlLiteralRe matches GFM literal-autolink URLs (goldmark's linkify
// pattern; the path part optional).
var urlLiteralRe = regexp.MustCompile(
	"(?:(?:https?|ftp)://|www\\.)[-a-zA-Z0-9@:%._\\+~#=]{1,256}\\.[a-z]+(?::\\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\\(\\);,'\">\\^{}\\[\\]`]*)?",
)

// relinkifyTexts converts bare URL literals inside plain text nodes into
// autolinks. Goldmark's linkify skips URLs while inside a potential link
// label (a dangling "[ http://…"), where remark still linkifies — and the
// re-parse would too, so leaving them as text is round-trip unstable.
func relinkifyTexts(nodes []ast.Node) []ast.Node {
	nodes = foldDirectivesIntoURLs(nodes)
	var out []ast.Node
	for i := range nodes {
		switch node := nodes[i].(type) {
		case *ast.Text:
			out = append(out, splitURLLiterals(node)...)
			continue
		case *ast.Link, *ast.Image, *ast.InlineCode, *ast.TextDirective:
			// already a link context (or verbatim content)
		default:
			ast.SetChildren(node, relinkifyTexts(ast.Children(node)))
		}
		out = append(out, nodes[i])
	}
	return out
}

// foldDirectivesIntoURLs dissolves bare text directives (":name" — no
// label, no attributes) back into plain text when a URL literal in the
// concatenated run of text+directive nodes spans them. Inside a dangling
// link label goldmark skips linkify, so ":0" in "[ http://:0.a" parses as
// a directive; remark (and any re-parse of the escaped output) treats the
// whole run as one autolink, so the directive must dissolve for a stable
// round trip.
func foldDirectivesIntoURLs(nodes []ast.Node) []ast.Node {
	changed := false
	for start := 0; start < len(nodes); {
		if !foldableNode(nodes[start]) {
			start++
			continue
		}
		end := start
		for end < len(nodes) && foldableNode(nodes[end]) {
			end++
		}
		// Concatenate the run and note each directive's span.
		var sb strings.Builder
		type span struct {
			name          string
			idx, from, to int
		}
		var spans []span
		for i := start; i < end; i++ {
			if d, ok := nodes[i].(*ast.TextDirective); ok {
				raw := ":" + d.Name
				spans = append(spans, span{name: d.Name, idx: i, from: sb.Len(), to: sb.Len() + len(raw)})
				sb.WriteString(raw)
				continue
			}
			if t, ok := nodes[i].(*ast.Text); ok {
				sb.WriteString(t.Value)
			}
		}
		merged := sb.String()
		for _, loc := range urlLiteralRe.FindAllStringIndex(merged, -1) {
			// The literal ends where the parser would end it, not where
			// the regexp does (see trimURLLiteralEnd).
			stop := loc[0] + len(trimURLLiteralEnd(merged[loc[0]:loc[1]]))
			for _, sp := range spans {
				if sp.from >= loc[0] && sp.to <= stop && (sp.from > loc[0] || sp.to < stop) {
					nodes[sp.idx] = &ast.Text{Value: ":" + sp.name}
					changed = true
				}
			}
		}
		start = end
	}
	if !changed {
		return nodes
	}
	// Re-merge adjacent plain text nodes so URL splitting sees one run.
	var out []ast.Node
	for i := range nodes {
		if t, ok := nodes[i].(*ast.Text); ok && len(out) > 0 {
			if prev, prevOK := out[len(out)-1].(*ast.Text); prevOK {
				appendText(prev, t)
				continue
			}
		}
		out = append(out, nodes[i])
	}
	return out
}

// appendText merges src into dst, concatenating both the decoded Value and
// the escape-preserving Raw provenance (see ast.Text.Raw) so a merged run's
// Rendered form stays the byte-for-byte concatenation of its parts. Raw is
// set only when either side carried escapes, keeping escape-free runs cheap.
func appendText(dst, src *ast.Text) {
	if dst.Raw != "" || src.Raw != "" {
		dst.Raw = dst.Rendered() + src.Rendered()
	}
	dst.Value += src.Value
}

// foldableNode reports whether a node can join a fold run: plain text or a
// bare text directive (no label content, no attributes).
func foldableNode(n ast.Node) bool {
	if _, ok := n.(*ast.Text); ok {
		return true
	}
	d, ok := n.(*ast.TextDirective)
	return ok && len(d.Children) == 0 && len(d.Attrs) == 0
}

// textSlice builds a text node for node.Value[a:b], carrying the matching
// slice of the escape-preserving Raw provenance (see rawSliceFor) so a URL
// split next to a preserved escape still renders the escape.
func textSlice(node *ast.Text, value string, a, b int) *ast.Text {
	t := &ast.Text{Value: value[a:b]}
	if node.Raw != "" {
		if raw := rawSliceFor(node.Raw, value, a, b); raw != t.Value {
			t.Raw = raw
		}
	}
	return t
}

// rawSliceFor returns the substring of raw corresponding to value[a:b]. raw
// is value with an extra backslash inserted before some characters (the
// preserved escapes), so a parallel walk maps each Value byte to its Raw
// position: a matching "\X" in raw consumes two bytes for one Value byte.
func rawSliceFor(raw, value string, a, b int) string {
	ri, rStart, rEnd := 0, -1, -1
	for vi := 0; vi <= len(value); vi++ {
		if vi == a {
			rStart = ri
		}
		if vi == b {
			rEnd = ri
			break
		}
		if vi < len(value) && rawEscapeAt(raw, ri, value[vi]) {
			ri += 2
		} else {
			ri++
		}
	}
	if rStart < 0 || rEnd < 0 || rStart > rEnd || rEnd > len(raw) {
		return value[a:b] // fall back to the decoded slice on any mismatch
	}
	return raw[rStart:rEnd]
}

// rawEscapeAt reports whether raw[i:] opens a preserved escape standing for
// the single Value byte c. Only PreservedEscapes survive undecoded in Raw,
// so a backslash before anything else is a literal one — without that test
// two literal backslashes in Value ("\\\0…", where the second escapes
// nothing) read as one escape pair and the walk desynchronizes, handing a
// URL split a Raw slice one byte too long (probe: "\\\0+\+\(www.0.a0").
func rawEscapeAt(raw string, i int, c byte) bool {
	return i+1 < len(raw) && raw[i] == '\\' && raw[i+1] == c &&
		strings.IndexByte(PreservedEscapes, c) >= 0
}

// splitURLLiterals splits a text node around URL literals at GFM autolink
// boundaries (start of text or after whitespace / '*' '_' '~' '(').
func splitURLLiterals(node *ast.Text) []ast.Node {
	value := node.Value
	locs := urlLiteralRe.FindAllStringIndex(value, -1)
	if locs == nil {
		return []ast.Node{node}
	}
	var out []ast.Node
	pos := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start < pos {
			continue
		}
		if start > 0 {
			switch value[start-1] {
			case ' ', '\t', '\n', '*', '_', '~', '(', ':':
				// ':' mirrors the colonURLParser boundary (a URL directly
				// after a colon linkifies outside labels, so the in-label
				// text must split the same way).
			default:
				continue
			}
		}
		if start > pos {
			out = append(out, textSlice(node, value, pos, start))
		}
		url := trimURLLiteralEnd(value[start:end])
		end = start + len(url)
		href := url
		if strings.HasPrefix(url, "www.") {
			href = "http://" + url
		}
		out = append(out, &ast.Link{
			URL:  href,
			Bare: true,
			// A URL literal carries no preserved-escape backslashes, so the
			// label's Raw equals its Value; leave Raw empty.
			Children: []ast.Node{&ast.Text{Value: url}},
		})
		pos = end
	}
	if pos < len(value) {
		out = append(out, textSlice(node, value, pos, len(value)))
	}
	if len(out) == 0 {
		return []ast.Node{node}
	}
	return out
}

// trimURLLiteralEnd shortens a URL literal to the end goldmark's linkify
// parser gives it, which its regexp alone does not: a trailing '.', an
// unbalanced ')' run and an entity-closing ';' come off, and then so does
// the trailing run of "?!.,:*_~" (extension/linkify.go). adfast
// re-linkifies decoded text where the parser was inside a link label, so
// the two boundaries have to agree — otherwise the render writes a literal
// the re-parse cuts shorter ("http://0.a#!" links only through the '#';
// probe: "http:\//0.a#!", whose escape sent it down this path).
func trimURLLiteralEnd(s string) string {
	switch s[len(s)-1] {
	case '.':
		s = s[:len(s)-1]
	case ')':
		closing := 0
		for i := range len(s) {
			switch s[i] {
			case ')':
				closing++
			case '(':
				closing--
			}
		}
		if closing > 0 {
			s = s[:len(s)-closing]
		}
	case ';':
		// A trailing "&…;" is a character reference, not part of the link.
		i := len(s) - 2
		for ; i >= 0; i-- {
			if !isAlphaNumericByte(s[i]) {
				break
			}
		}
		if i >= 0 && i != len(s)-2 && s[i] == '&' {
			s = s[:i]
		}
	}
	i := len(s) - 1
	for i > 0 && isURLTrailPunct(s[i]) {
		i--
	}
	return s[:i+1]
}

// isURLTrailPunct lists the bytes goldmark strips from the end of an
// autolink literal (never all of them: one byte always remains).
func isURLTrailPunct(c byte) bool {
	return c == '?' || c == '!' || c == '.' || c == ',' ||
		c == ':' || c == '*' || c == '_' || c == '~'
}

// isAlphaNumericByte mirrors goldmark's util.IsAlphaNumeric for ASCII.
func isAlphaNumericByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// maxLiftDepth caps the lift recursion. Legitimate documents nest a few
// dozen levels; adversarial nesting (thousands of blockquote markers on
// one line) would otherwise overflow the stack, which Go cannot recover
// from. Content below the cap is truncated and surfaced through the
// liftCtx depth notice (the facade converts it into a depth-exceeded
// diagnostic).
const maxLiftDepth = 1024

// liftCtx carries the per-parse lift parameters (the depth-cap and span
// notices) alongside the depth counter threaded through the conversion.
type liftCtx struct {
	depthNotice func()
	// spanNotice reports a table span marker (">"/"^") whose merge cannot
	// apply (see resolveTableSpans); row/col are 1-based within the table.
	spanNotice func(marker string, row, col int)
	depthFired bool
}

// depthExceeded fires the depth notice once per parse.
func (lc *liftCtx) depthExceeded() {
	if lc.depthFired {
		return
	}
	lc.depthFired = true
	if lc.depthNotice != nil {
		lc.depthNotice()
	}
}

func goldmarkToAst(tree gast.Node, src []byte, depthNotice func(), spanNotice func(marker string, row, col int)) ast.Node {
	lc := &liftCtx{depthNotice: depthNotice, spanNotice: spanNotice}
	root := &ast.Root{Children: convertGoldmarkBlocks(tree, src, lc, 0)}
	root.Children = relinkifyTexts(root.Children)
	return root
}

// ---------------------------------------------------------------------------
// Block conversion
// ---------------------------------------------------------------------------

func convertGoldmarkBlocks(parent gast.Node, src []byte, lc *liftCtx, depth int) []ast.Node {
	if depth > maxLiftDepth {
		lc.depthExceeded()
		return nil
	}
	var nodes []ast.Node
	prevHTMLEnd := -1
	var prevChild gast.Node
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		n := convertGoldmarkBlock(child, src, lc, depth)
		if n == nil {
			continue
		}
		if prevChild != nil {
			ast.SetGapBefore(n, blankLineBetween(prevChild, child, src))
		}
		prevChild = child
		// Contiguous HTML blocks (e.g. stacked <!-- --> comment lines with
		// no blank line between) merge into one node so the renderer keeps
		// them adjacent, the way prettier preserves their source spacing.
		if hb, ok := child.(*gast.HTMLBlock); ok {
			start, end := htmlBlockSpan(hb, src)
			if html, isHTML := n.(*ast.HTML); isHTML && prevHTMLEnd >= 0 && start >= prevHTMLEnd && len(nodes) > 0 {
				if prev, prevOK := nodes[len(nodes)-1].(*ast.HTML); prevOK && !hasBlankLine(src[prevHTMLEnd:start]) {
					prev.Value += "\n" + html.Value
					prevHTMLEnd = end
					continue
				}
			}
			prevHTMLEnd = end
		} else {
			prevHTMLEnd = -1
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// htmlBlockSpan returns the source byte range covered by an HTML block's
// lines (closure included).
func htmlBlockSpan(n *gast.HTMLBlock, src []byte) (start, end int) {
	start, end = -1, -1
	if n.Lines().Len() > 0 {
		start = n.Lines().At(0).Start
		end = n.Lines().At(n.Lines().Len() - 1).Stop
	}
	if n.HasClosure() {
		if start < 0 {
			start = n.ClosureLine.Start
		}
		end = n.ClosureLine.Stop
	}
	_ = src
	return start, end
}

func convertGoldmarkBlock(node gast.Node, src []byte, lc *liftCtx, depth int) ast.Node {
	switch n := node.(type) {
	case *gast.Paragraph:
		out := &ast.Paragraph{Children: convertGoldmarkInlines(n, src, lc, depth+1)}
		if _, ok := n.AttributeString("directiveLabel"); ok {
			out.DirectiveLabel = true
		}
		return out

	case *gast.TextBlock:
		// Tight list items use TextBlock instead of Paragraph
		return &ast.Paragraph{Children: convertGoldmarkInlines(n, src, lc, depth+1)}

	case *gast.Heading:
		// The anchor strip mutates n, so it must precede the inline read.
		id := splitHeadingAnchor(n, src)
		return &ast.Heading{Depth: n.Level, ID: id, Children: convertGoldmarkInlines(n, src, lc, depth+1)}

	case *gast.ThematicBreak:
		return &ast.ThematicBreak{}

	case *gast.Blockquote:
		return &ast.Blockquote{Children: convertGoldmarkBlocks(n, src, lc, depth+1)}

	case *gast.FencedCodeBlock:
		return &ast.Code{
			// Info strings carry character references and escapes
			// ("\x600&#x60;" round-trips a backtick), like remark.
			// goldmark's Language() splits only on space; remark cuts the
			// language at any whitespace, so trim a tab tail too.
			Lang:  decodeMarkdownEscapes(fenceLanguage(string(n.Language(src))), ""),
			Value: codeBlockValue(n, src),
		}

	case *gast.CodeBlock:
		return &ast.Code{Value: codeBlockValue(n, src)}

	case *gast.List:
		return convertGoldmarkList(n, src, lc, depth)

	case *gast.HTMLBlock:
		var buf bytes.Buffer
		for i := range n.Lines().Len() {
			line := n.Lines().At(i)
			buf.Write(line.Value(src))
		}
		if n.HasClosure() {
			buf.Write(n.ClosureLine.Value(src))
		}
		return &ast.HTML{Value: strings.TrimRight(buf.String(), "\n")}

	case *directive.ContainerDirective:
		// The label, when present, is already the node's first child
		// paragraph (matching remark's representation).
		return &ast.ContainerDirective{
			Name:     n.Name,
			Attrs:    n.Attrs,
			Children: convertGoldmarkBlocks(n, src, lc, depth+1),
		}

	case *directive.LeafDirective:
		return &ast.LeafDirective{
			Name:     n.Name,
			Attrs:    n.Attrs,
			Children: convertGoldmarkInlines(n, src, lc, depth+1),
		}

	case *east.Table:
		return convertGoldmarkTable(n, src, lc, depth)

	default:
		// Unknown block — try children
		if node.HasChildren() {
			content := convertGoldmarkBlocks(node, src, lc, depth+1)
			if len(content) > 0 {
				return content[0]
			}
		}
		return nil
	}
}

// fenceLanguage truncates a fence info word at the first tab — micromark
// ends the language at any whitespace while goldmark only splits on space.
func fenceLanguage(lang string) string {
	before, _, _ := strings.Cut(lang, "\t")
	return before
}

func codeBlockValue(n interface{ Lines() *text.Segments }, src []byte) string {
	var buf bytes.Buffer
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		buf.Write(line.Value(src))
	}
	return strings.TrimRight(buf.String(), "\n")
}

func convertGoldmarkList(n *gast.List, src []byte, lc *liftCtx, depth int) *ast.List {
	// Goldmark reports the literal first marker number for ordered lists —
	// including a genuine "0)" (remark keeps order 0); unordered lists have
	// no start.
	start := n.Start
	if !n.IsOrdered() {
		start = 1
	}

	var items []ast.Node
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		li, ok := child.(*gast.ListItem)
		if !ok {
			continue
		}
		spread := itemInternalBlank(li, src)
		gapAfter := false
		if next := child.NextSibling(); next != nil {
			gapAfter = blankLineBetween(li, next, src)
		}
		blocks := convertGoldmarkBlocks(li, src, lc, depth+1)
		cb := scanTaskCheckbox(li, src)
		switch {
		case cb.valid:
			item := &ast.ListItem{Children: blocks, Spread: spread, GapAfter: gapAfter}
			checked := cb.checked
			item.Checked = &checked
			items = append(items, item)
		case cb.present:
			// goldmark parses "[x]" without trailing whitespace/content as a
			// checkbox; remark does not. Restore the literal bracket text.
			literal := &ast.Text{Value: cb.raw}
			if para, ok := firstParagraphOf(blocks); ok {
				para.Children = coalesceTextNodes(append([]ast.Node{literal}, para.Children...))
			} else {
				blocks = append([]ast.Node{&ast.Paragraph{Children: []ast.Node{literal}}}, blocks...)
			}
			fallthrough
		default:
			items = append(items, &ast.ListItem{Children: blocks, Spread: spread, GapAfter: gapAfter})
		}
	}

	// Prettier's git-diff-friendly rule: when the first two source markers
	// carry the same number the style is preserved (every item repeats it);
	// otherwise items are renumbered sequentially from start.
	increment, orderedGap := orderedListStyle(n, src)

	return &ast.List{
		Ordered:       n.IsOrdered(),
		Start:         start,
		Spread:        !n.IsTight,
		PerItemSpread: true,
		Increment:     increment,
		OrderedGap:    orderedGap,
		Children:      items,
	}
}

// firstParagraphOf returns blocks[0] when it is a paragraph.
func firstParagraphOf(blocks []ast.Node) (*ast.Paragraph, bool) {
	if len(blocks) == 0 {
		return nil, false
	}
	para, ok := blocks[0].(*ast.Paragraph)
	return para, ok
}

// orderedListStyle derives prettier's git-diff-friendly numbering style
// from the first two source markers: when both carry the same number the
// style is preserved (increment=false, every item repeats it); otherwise
// items renumber sequentially. orderedGap is the marker-content gap width.
func orderedListStyle(n *gast.List, src []byte) (increment bool, orderedGap int) {
	if !n.IsOrdered() || n.FirstChild() == nil {
		return false, 0
	}
	n0, gap, ok := orderedItemMarker(n.FirstChild(), src)
	if !ok {
		return false, 0
	}
	orderedGap = min(max(gap, 1), 2)
	if second := n.FirstChild().NextSibling(); second != nil {
		if n1, _, ok1 := orderedItemMarker(second, src); ok1 {
			increment = n1 != n0
		}
	}
	return increment, orderedGap
}

// nodeSpan returns the source byte range covered by a block node's lines
// (descendants included); ok is false when no line segments exist.
func nodeSpan(node gast.Node) (start, end int, ok bool) {
	start, end = -1, -1
	var walk func(n gast.Node)
	walk = func(n gast.Node) {
		if n.Type() != gast.TypeInline && n.Lines().Len() > 0 {
			if s := n.Lines().At(0).Start; start < 0 || s < start {
				start = s
			}
			if e := n.Lines().At(n.Lines().Len() - 1).Stop; e > end {
				end = e
			}
		}
		if hb, isHTML := n.(*gast.HTMLBlock); isHTML && hb.HasClosure() {
			if s := hb.ClosureLine.Start; start < 0 || s < start {
				start = s
			}
			if e := hb.ClosureLine.Stop; e > end {
				end = e
			}
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(node)
	return start, end, start >= 0
}

// blankLineBetween reports whether a blank line separates two sibling block
// nodes in the source. Goldmark segment conventions differ (a paragraph's
// final line is stored without its newline, HTML block lines keep theirs),
// so the gap is normalized to complete lines first.
func blankLineBetween(a, b gast.Node, src []byte) bool {
	_, aEnd, ok1 := nodeSpan(a)
	bStart, _, ok2 := nodeSpan(b)
	if !ok1 || !ok2 || bStart < aEnd {
		return false
	}
	i := aEnd
	if i == 0 || src[i-1] != '\n' {
		for i < len(src) && src[i] != '\n' {
			i++
		}
		if i < len(src) {
			i++
		}
	}
	j := bStart
	for j > i && src[j-1] != '\n' {
		j--
	}
	if i >= j {
		return false
	}
	return hasBlankLine(src[i:j])
}

// itemInternalBlank reports whether any two consecutive child blocks of a
// list item are separated by a blank line (AST listItem spread).
func itemInternalBlank(li gast.Node, src []byte) bool {
	for c := li.FirstChild(); c != nil && c.NextSibling() != nil; c = c.NextSibling() {
		if blankLineBetween(c, c.NextSibling(), src) {
			return true
		}
	}
	return false
}

// hasBlankLine reports whether the slice contains a complete line with only
// whitespace. Goldmark line segments include their trailing newline, so a
// gap between two blocks starts at a line boundary; a blank separator shows
// up as an all-whitespace line terminated by a newline within the gap.
func hasBlankLine(s []byte) bool {
	blank := true
	for _, c := range s {
		switch c {
		case '\n':
			if blank {
				return true
			}
			blank = true
		case ' ', '\t':
		default:
			blank = false
		}
	}
	return false
}

// orderedItemMarker reads the literal marker number and following gap width
// of a list item from the source; goldmark only stores the list-level start.
// The marker line is the line containing the item's first content segment,
// or the line above it when the first block starts on the next line (e.g. a
// fence-first item).
func orderedItemMarker(li gast.Node, src []byte) (num, gap int, ok bool) {
	node := li
	for node != nil && node.Lines().Len() == 0 {
		node = node.FirstChild()
	}
	if node == nil || node.Lines().Len() == 0 {
		return 0, 0, false
	}
	contentStart := node.Lines().At(0).Start
	lineStart := lineStartBefore(src, contentStart)
	if n, g, found := parseOrderedMarker(src[lineStart:contentStart]); found {
		return n, g, true
	}
	if lineStart > 0 {
		prev := lineStartBefore(src, lineStart-1)
		if n, g, found := parseOrderedMarker(src[prev:lineStart]); found {
			return n, g, true
		}
	}
	return 0, 0, false
}

// lineStartBefore returns the index just after the previous newline.
func lineStartBefore(src []byte, pos int) int {
	for pos > 0 && src[pos-1] != '\n' {
		pos--
	}
	return pos
}

// parseOrderedMarker matches leading "[indent]N[.)][spaces]" in a line slice.
func parseOrderedMarker(line []byte) (num, gap int, ok bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	ds := i
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		num = num*10 + int(line[i]-'0')
		i++
	}
	if i == ds || i >= len(line) || (line[i] != '.' && line[i] != ')') {
		return 0, 0, false
	}
	i++
	for i < len(line) && line[i] == ' ' {
		gap++
		i++
	}
	return num, gap, true
}

// taskCheckboxValidRe is remark-gfm's task marker shape: bracketed state
// followed by whitespace and actual content on the same line.
var taskCheckboxValidRe = regexp.MustCompile(`^\[[ xX]\][ \t]+[^ \t\r\n]`)

type taskCheckbox struct {
	raw     string // literal "[x]"/"[X]"/"[ ]" source text
	present bool   // goldmark produced a TaskCheckBox node
	valid   bool   // it also satisfies remark's marker shape
	checked bool
}

// scanTaskCheckbox inspects a list item's first block (a Paragraph in loose
// lists, a TextBlock in tight ones) for a leading GFM task checkbox.
func scanTaskCheckbox(li *gast.ListItem, src []byte) taskCheckbox {
	block := li.FirstChild()
	if block == nil {
		return taskCheckbox{}
	}
	switch block.(type) {
	case *gast.Paragraph, *gast.TextBlock:
	default:
		return taskCheckbox{}
	}
	cb, ok := block.FirstChild().(*east.TaskCheckBox)
	if !ok {
		return taskCheckbox{}
	}
	var line []byte
	if lines := block.Lines(); lines.Len() > 0 {
		seg := lines.At(0)
		line = seg.Value(src)
	}
	raw := "[ ]"
	if cb.IsChecked {
		raw = "[x]"
	}
	if len(line) >= 3 {
		raw = string(line[:3])
	}
	return taskCheckbox{
		present: true,
		valid:   taskCheckboxValidRe.Match(line),
		checked: cb.IsChecked,
		raw:     raw,
	}
}

func convertGoldmarkTable(table *east.Table, src []byte, lc *liftCtx, depth int) ast.Node {
	var rows []*ast.TableRow
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch row := child.(type) {
		case *east.TableHeader:
			rows = append(rows, &ast.TableRow{Children: convertGoldmarkTableCells(row, src, lc, depth)})
		case *east.TableRow:
			rows = append(rows, &ast.TableRow{Children: convertGoldmarkTableCells(row, src, lc, depth)})
		}
	}

	if len(rows) == 0 {
		return nil
	}
	resolveTableSpans(rows, lc.spanNotice)
	children := make([]ast.Node, len(rows))
	for i, row := range rows {
		children[i] = row
	}
	return &ast.Table{Children: children, Align: liftTableAlign(table.Alignments)}
}

// liftTableAlign maps goldmark's per-column alignments onto the pivot AST's.
// It answers nil when no column asks for one, so a delimiter row without a
// colon leaves no trace to carry (see ast.AnyAligned).
func liftTableAlign(alignments []east.Alignment) []ast.Alignment {
	out := make([]ast.Alignment, len(alignments))
	for i, a := range alignments {
		switch a {
		case east.AlignLeft:
			out[i] = ast.AlignLeft
		case east.AlignRight:
			out[i] = ast.AlignRight
		case east.AlignCenter:
			out[i] = ast.AlignCenter
		case east.AlignNone:
			out[i] = ast.AlignNone
		}
	}
	if !ast.AnyAligned(out) {
		return nil
	}
	return out
}

// spanMarkerCell is the intermediate node for a remark-extended-table merge
// marker cell (">" extends the following cell leftward, "^" the cell above
// downward) between cell conversion and span resolution; it never escapes
// the table conversion.
type spanMarkerCell struct{ marker string }

// Kind implements ast.Node.
func (*spanMarkerCell) Kind() string { return "spanMarker" }

func convertGoldmarkTableCells(parent gast.Node, src []byte, lc *liftCtx, depth int) []ast.Node {
	var cells []ast.Node
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		cell, ok := child.(*east.TableCell)
		if !ok {
			continue
		}
		converted := &ast.TableCell{Children: convertGoldmarkInlines(cell, src, lc, depth+1)}
		// Merge markers are the raw, unescaped cell texts ">" (extend the
		// following cell leftward — colspan) and "^" (extend the cell
		// above downward — rowspan); "\>" stays a literal cell.
		switch raw := rawCellText(cell, src); raw {
		case ">", "^":
			cells = append(cells, &spanMarkerCell{marker: raw})
		default:
			cells = append(cells, converted)
		}
	}
	return cells
}

// rawCellText is the trimmed raw source of a table cell.
func rawCellText(cell gast.Node, src []byte) string {
	var buf bytes.Buffer
	for i := range cell.Lines().Len() {
		seg := cell.Lines().At(i)
		buf.Write(seg.Value(src))
	}
	return strings.TrimSpace(buf.String())
}

// resolveTableSpans folds remark-extended-table merge markers into
// ColSpan/RowSpan on the surviving cells, using a visual-column grid: a run
// of ">" markers extends the next content cell leftward, and "^" markers
// extend the spanning cell covering the same visual column in an earlier
// row downward. Marker cells are removed. Unresolvable markers (nothing to
// merge with) revert to literal text; each reverted marker is reported
// through notify (1-based row/column) when a notice is registered.
func resolveTableSpans(rows []*ast.TableRow, notify func(marker string, row, col int)) {
	report := func(marker string, rowIdx, col int) {
		if notify != nil {
			notify(marker, rowIdx+1, col+1)
		}
	}
	// owner[col] points at the cell currently spanning that visual column
	// from an earlier row (for rowspan extension).
	owner := map[int]*ast.TableCell{}
	for rowIdx, row := range rows {
		cells := row.Children
		var kept []ast.Node
		col := 0
		pendingCols := 0 // ">" markers awaiting their content cell
		revertPending := func() {
			// The pending ">" run occupies the columns just before col.
			for c := col - pendingCols; c < col; c++ {
				kept = append(kept, literalMarkerCell(">"))
				report(">", rowIdx, c)
			}
			pendingCols = 0
		}
		for i := range cells {
			if marker, ok := cells[i].(*spanMarkerCell); ok {
				switch marker.marker {
				case ">":
					pendingCols++
					col++
					continue
				case "^":
					if pendingCols > 0 {
						// mixed markers — treat the whole pending run as literal
						revertPending()
					}
					if span, spanOK := owner[col]; spanOK {
						if span.RowSpan == 0 {
							span.RowSpan = 1
						}
						span.RowSpan++
						col++
						continue
					}
					kept = append(kept, literalMarkerCell("^"))
					report("^", rowIdx, col)
					col++
					continue
				}
			}
			cell, cellOK := cells[i].(*ast.TableCell)
			if !cellOK {
				continue
			}
			if pendingCols > 0 {
				cell.ColSpan = pendingCols + 1
				pendingCols = 0
			}
			startCol := col - max(cell.ColSpan-1, 0)
			kept = append(kept, cell)
			for c := startCol; c <= col; c++ {
				owner[c] = cell
			}
			col++
		}
		revertPending()
		row.Children = kept
	}
}

// literalMarkerCell restores an unresolvable merge marker as cell text.
func literalMarkerCell(marker string) ast.Node {
	return &ast.TableCell{Children: []ast.Node{&ast.Text{Value: marker}}}
}

// ---------------------------------------------------------------------------
// Inline conversion
// ---------------------------------------------------------------------------

func convertGoldmarkInlines(parent gast.Node, src []byte, lc *liftCtx, depth int) []ast.Node {
	if depth > maxLiftDepth {
		lc.depthExceeded()
		return nil
	}
	var nodes []ast.Node
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		nodes = append(nodes, convertGoldmarkInline(child, src, lc, depth)...)
	}
	return coalesceTextNodes(nodes)
}

// coalesceTextNodes merges adjacent plain text nodes. Goldmark flushes text
// segments at every inline trigger attempt (spaces via linkify, colons via
// the directive parsers), while remark keeps text merged and only splits at
// real construct boundaries — which survive here as non-text nodes.
func coalesceTextNodes(nodes []ast.Node) []ast.Node {
	out := nodes[:0]
	for i := range nodes {
		if t, ok := nodes[i].(*ast.Text); ok && len(out) > 0 {
			if prev, prevOK := out[len(out)-1].(*ast.Text); prevOK {
				appendText(prev, t)
				continue
			}
		}
		out = append(out, nodes[i])
	}
	return out
}

func convertGoldmarkInline(node gast.Node, src []byte, lc *liftCtx, depth int) []ast.Node {
	switch n := node.(type) {
	case *gast.Text:
		return convertGoldmarkText(n, src)

	case *gast.String:
		if len(n.Value) == 0 {
			return nil
		}
		return []ast.Node{&ast.Text{Value: string(n.Value)}}

	case *gast.CodeSpan:
		// CommonMark: line endings inside a code span are converted to
		// spaces (a span may wrap across source lines).
		return []ast.Node{&ast.InlineCode{Value: strings.ReplaceAll(string(textContent(n, src)), "\n", " ")}}

	case *gast.Emphasis:
		children := convertGoldmarkInlines(n, src, lc, depth+1)
		if n.Level >= 2 {
			return []ast.Node{&ast.Strong{Children: children}}
		}
		return []ast.Node{&ast.Emphasis{Children: children}}

	case *east.Strikethrough:
		return []ast.Node{&ast.Delete{Children: convertGoldmarkInlines(n, src, lc, depth+1)}}

	case *gast.Link:
		// An explicit [label](url) resource link keeps that form even when
		// the label equals the URL (prettier does not shorten it); only
		// autolinks collapse to <url>. Goldmark stores titles raw, escapes
		// included.
		return []ast.Node{&ast.Link{
			URL:      decodeMarkdownEscapes(string(n.Destination), ""),
			Title:    decodeMarkdownEscapes(string(n.Title), ""),
			Explicit: true,
			Children: convertGoldmarkInlines(n, src, lc, depth+1),
		}}

	case *gast.Image:
		return []ast.Node{&ast.Image{
			URL:      decodeMarkdownEscapes(string(n.Destination), ""),
			Title:    decodeMarkdownEscapes(string(n.Title), ""),
			Children: convertGoldmarkInlines(n, src, lc, depth+1),
		}}

	case *gast.AutoLink:
		href := string(n.URL(src))
		_, angle := n.AttributeString("angleAutoLink")
		// micromark's email autolink literal requires a dot in the domain
		// and a final letter (goldmark's regex path absorbs trailing
		// digits; see gfmEmailRe) — invalid matches stay plain text.
		if n.AutoLinkType == gast.AutoLinkEmail && !angle && !gfmEmailValid(href) {
			return []ast.Node{&ast.Text{Value: string(n.Label(src))}}
		}
		// www literals keep their bare label with the http:// href
		// (goldmark's URL() prepends the protocol; remark renders
		// "[www.x](http://www.x)").
		label := string(n.Label(src))
		if label == "" {
			label = href
		}
		return []ast.Node{&ast.Link{
			URL:      href,
			Bare:     !angle,
			Children: []ast.Node{&ast.Text{Value: label}},
		}}

	case *directive.TextDirective:
		return convertGoldmarkTextDirective(n, lc, depth)

	case *gast.RawHTML:
		var buf bytes.Buffer
		for i := range n.Segments.Len() {
			seg := n.Segments.At(i)
			buf.Write(seg.Value(src))
		}
		if buf.Len() == 0 {
			return nil
		}
		return []ast.Node{&ast.HTML{Value: buf.String()}}

	case *gast.HTMLBlock:
		return nil

	default:
		// Try to recurse into children (drops leaf markers like TaskCheckBox)
		if node.HasChildren() {
			return convertGoldmarkInlines(node, src, lc, depth+1)
		}
		return nil
	}
}

// gfmEmailValid reports whether a linkified email address satisfies
// micromark's gfm-autolink-literal constraints: a dot somewhere in the
// domain and a letter as the final character.
func gfmEmailValid(addr string) bool {
	addr = strings.TrimPrefix(addr, "mailto:")
	at := strings.LastIndexByte(addr, '@')
	if at < 0 {
		return false
	}
	labels := strings.Split(addr[at+1:], ".")
	if len(labels) < 2 {
		return false
	}
	if slices.Contains(labels, "") {
		return false
	}
	last := addr[len(addr)-1]
	return (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z')
}

// convertGoldmarkText converts a goldmark text node, decoding CommonMark
// escapes and translating soft/hard line breaks.
func convertGoldmarkText(n *gast.Text, src []byte) []ast.Node {
	raw := string(n.Segment.Value(src))
	// One faithful parse: Value is fully decoded (the ADF currency); rawVal
	// keeps prettier's literal escapes as escape provenance for the
	// formatter (see PreservedEscapes). rawVal is stored on ast.Text.Raw
	// only when it differs, so the common escape-free text pays nothing.
	value := decodeMarkdownEscapes(raw, "")
	rawVal := decodeMarkdownEscapes(raw, PreservedEscapes)
	if n.SoftLineBreak() {
		value += " " // Normalize soft breaks to spaces
		rawVal += " "
	}
	var nodes []ast.Node
	if value != "" {
		t := &ast.Text{Value: value}
		if rawVal != value {
			t.Raw = rawVal
		}
		nodes = append(nodes, t)
	}
	if n.HardLineBreak() {
		// Preserve the source break style (trailing spaces vs
		// backslash) for the formatter; prettier keeps it.
		br := &ast.Break{}
		if n.Segment.Stop < len(src) && src[n.Segment.Stop] != '\\' {
			br.Value = "  "
		}
		nodes = append(nodes, br)
	}
	return nodes
}

// convertGoldmarkTextDirective converts a parsed text directive node.
// The label was parsed with a nested parser over its own source slice;
// convert its inlines against that source. A label that did not parse as
// inline content falls back to raw text.
func convertGoldmarkTextDirective(n *directive.TextDirective, lc *liftCtx, depth int) []ast.Node {
	var children []ast.Node
	if n.LabelRoot != nil {
		children = convertGoldmarkInlines(n.LabelRoot, n.LabelSource, lc, depth+1)
		// Block parsing strips label-edge whitespace that remark keeps
		// verbatim (":color[has ]" must round-trip its trailing space).
		label := string(n.LabelSource)
		if lead := label[:len(label)-len(strings.TrimLeft(label, " \t"))]; lead != "" {
			children = coalesceTextNodes(append([]ast.Node{&ast.Text{Value: lead}}, children...))
		}
		if trail := label[len(strings.TrimRight(label, " \t")):]; trail != "" && strings.TrimSpace(label) != "" {
			children = coalesceTextNodes(append(children, &ast.Text{Value: trail}))
		}
	} else if len(n.LabelSource) > 0 {
		children = []ast.Node{&ast.Text{Value: string(n.LabelSource)}}
	}
	return []ast.Node{&ast.TextDirective{
		Name:     n.Name,
		Attrs:    n.Attrs,
		Children: children,
	}}
}

func textContent(node gast.Node, src []byte) []byte {
	var buf bytes.Buffer
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch ch := child.(type) {
		case *gast.Text:
			buf.Write(ch.Segment.Value(src))
		case *gast.String:
			buf.Write(ch.Value)
		default:
			if child.HasChildren() {
				buf.Write(textContent(child, src))
			}
		}
	}
	return buf.Bytes()
}

// decodeMarkdownEscapes removes CommonMark backslash escapes and character
// references from raw markdown source text. Goldmark's gast.Text.Segment holds
// raw source bytes, so `\[` appears verbatim instead of `[` and `&#x61;`
// instead of `a` (goldmark resolves references only in its HTML renderer).
// remark decodes both at parse time; matching that here keeps ADF content
// identical to what Jira stores and lets the renderer's flanking character
// references (&#xNN;) round-trip.
func decodeMarkdownEscapes(s, keep string) string {
	// CommonMark: NUL is replaced with U+FFFD at parse time (micromark does
	// this; goldmark leaves it, which breaks flanking checks on re-parse).
	if strings.ContainsRune(s, 0) {
		s = strings.ReplaceAll(s, "\x00", string(utf8.RuneError))
	}
	if !strings.ContainsRune(s, '\\') && !strings.ContainsRune(s, '&') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			// Escapes in the keep set stay literal source bytes; the
			// formatter uses this to carry escape provenance through the
			// AST the way prettier's parser does (see preservedEscapes).
			if keep != "" && strings.IndexByte(keep, next) >= 0 {
				sb.WriteByte('\\')
				sb.WriteByte(next)
				i++
				continue
			}
			if isEscapableASCIIPunct(next) {
				sb.WriteByte(next)
				i++
				continue
			}
		}
		if s[i] == '&' {
			if decoded, next, ok := decodeCharacterReference(s, i); ok {
				sb.WriteString(decoded)
				i = next - 1
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// isEscapableASCIIPunct reports whether the byte is CommonMark ASCII
// punctuation that can be backslash-escaped.
func isEscapableASCIIPunct(c byte) bool {
	switch c {
	case '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',',
		'-', '.', '/', ':', ';', '<', '=', '>', '?', '@', '[', '\\',
		']', '^', '_', '`', '{', '|', '}', '~':
		return true
	default:
		return false
	}
}

// decodeCharacterReference decodes a CommonMark character reference starting
// at s[i] == '&': numeric (&#123; / &#x1F;) or named HTML5 entities. Returns
// the decoded string and the index after the closing ';'.
func decodeCharacterReference(s string, i int) (decoded string, next int, ok bool) {
	j := i + 1
	if j < len(s) && s[j] == '#' {
		return decodeNumericReference(s, j+1)
	}
	start := j
	for j < len(s) && j-start <= 48 && isDirectiveAlnumByte(s[j]) {
		j++
	}
	if j == start || j >= len(s) || s[j] != ';' {
		return "", 0, false
	}
	if entity, found := util.LookUpHTML5EntityByName(s[start:j]); found {
		return string(entity.Characters), j + 1, true
	}
	return "", 0, false
}

// decodeNumericReference decodes the &#123;/&#x1F; numeric forms, starting
// just after the '#'.
func decodeNumericReference(s string, j int) (decoded string, next int, ok bool) {
	hex := false
	if j < len(s) && (s[j] == 'x' || s[j] == 'X') {
		hex = true
		j++
	}
	start := j
	maxDigits := 7
	if hex {
		maxDigits = 6
	}
	for j < len(s) && j-start <= maxDigits && isRefDigit(s[j], hex) {
		j++
	}
	if j == start || j-start > maxDigits || j >= len(s) || s[j] != ';' {
		return "", 0, false
	}
	base := 10
	if hex {
		base = 16
	}
	n, err := strconv.ParseInt(s[start:j], base, 32)
	if err != nil {
		return "", 0, false
	}
	r := rune(n)
	if r == 0 || !utf8.ValidRune(r) {
		r = utf8.RuneError
	}
	return string(r), j + 1, true
}

func isRefDigit(c byte, hex bool) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	if !hex {
		return false
	}
	return (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isDirectiveAlnumByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
