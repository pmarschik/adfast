package convert

// Normalize is the shared AST→AST canonicalization pass, the single
// implementation of the pivot-AST canonical form the library renders and
// encodes from. It absorbs what the former root-package formatter
// normalizer did and mirrors, on the pivot AST, the canonicalizations
// ToADF performs as a side effect of ADF's data model:
//
//   - inline content flattens to (text, mark-set) atoms and regroups
//     into canonical strong/em/delete nesting (the spanning algorithm
//     ToADF/FromADF share), which also bounds adversarial nesting depth
//     for the renderer;
//   - code spans keep only their link mark; strong/em re-infer across
//     code boundaries the way ADF mark-stripping behaves;
//   - links wrap each atom separately (ADF stores the link as a per-text
//     mark), and empty links vanish;
//   - unknown text directives flatten to ":name" + label, unknown leaf
//     directives drop, unknown containers dissolve into a single child
//     (or drop);
//   - ::colwidths resolves onto the following table (orphans drop),
//     ::decisions marks the following plain bullet list (orphans drop);
//   - task lists collapse to the canonical tight form;
//   - the typed dialect kinds re-derive their canonical attribute
//     payloads (the equivalent of their ADF encode∘decode), including
//     media→image conversion against a configured asset map.
//
// Normalize is idempotent, and ToADF is invariant under it
// (ToADF(Normalize(n)) == ToADF(n) for every parsed AST). The prettier
// md→md formatter is the composition Render∘Normalize∘Parse; that render
// is byte-for-byte what routing the parse AST through ADF and back used
// to produce.

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/extension"
)

// Normalize returns the canonical form of the pivot AST rooted at n (see
// the package-level discussion). The options it reads are the conversion
// options that parameterize canonicalization: WithSmartLinks (both the
// URLForKey expansion of bare card labels and the KeyFromURL shortening
// of card URLs), WithMediaAssets (file media whose recorded dimensions
// match a configured asset collapse to plain images), WithCodeLanguages
// and WithDiagnostics (the language check and the orphan-directive
// notices). Options for the ADF representation only (resolvers,
// list-tightness) are ignored.
//
// The input tree is normalized in place and returned; pass a fresh parse
// if the caller must retain the faithful tree. A nil or non-root node is
// treated as a document whose children are n's children.
func Normalize(n ast.Node, opts ...Option) ast.Node {
	cfg := config{}
	for _, o := range opts {
		o(&cfg)
	}
	fn := &normalizer{
		assets:      newMediaAssets(cfg),
		encodeSL:    cfg.smartLinks,
		decodeSL:    cfg.smartLinks,
		diagnostics: cfg.diagnostics,
		codeLangs:   cfg.codeLanguages,
	}
	root, ok := n.(*ast.Root)
	if !ok {
		root = &ast.Root{Children: ast.Children(n)}
	}
	fn.normalizeRoot(root)
	return root
}

// normalizer carries the per-format configuration through the pass.
type normalizer struct {
	assets      mediaAssets
	encodeSL    SmartLinks // parse-side resolver (URLForKey)
	decodeSL    SmartLinks // render-side resolver (KeyFromURL)
	diagnostics func(Diagnostic)
	codeLangs   map[string]bool
}

func (fn *normalizer) diag(code, message string) {
	if fn.diagnostics != nil {
		fn.diagnostics(Diagnostic{Code: code, Message: message})
	}
}

// normalizeRoot rewrites the parsed document in place.
func (fn *normalizer) normalizeRoot(root *ast.Root) {
	children := fn.normalizeBlocks(root.Children)
	if len(children) == 0 {
		children = []ast.Node{&ast.Paragraph{}}
	}
	root.Children = children
}

// ---------------------------------------------------------------------------
// Inline normalization: flatten to mark atoms, regroup canonically
// ---------------------------------------------------------------------------

// fmtMarks is the inherited inline mark context (convert's markCtx).
type fmtMarks struct {
	href         string
	linkTitle    string
	textColor    string
	bgColor      string
	subsup       string
	annotations  []extension.Annotation // outermost first
	link         bool
	linkBare     bool
	linkExplicit bool
	strong       bool
	em           bool
	strike       bool
	underline    bool
}

// fmtAtom is one flattened inline item (convert's flatInline).
type fmtAtom struct {
	node        ast.Node // opaque leaf (image, html, typed inline node)
	text        string
	m           fmtMarks
	isCode      bool
	isBreak     bool
	spacesBreak bool
}

// normalizeInlines runs the full inline normalization on a phrasing run.
func (fn *normalizer) normalizeInlines(nodes []ast.Node) []ast.Node {
	return regroupAtoms(fn.flattenInlines(nodes, fmtMarks{}))
}

// fmtAtomSpanOps adapts fmtAtom to the shared flat→nested spanning
// algorithm (spanning.go): the nesting marks live under the atom's
// inherited mark context, the code flag on the atom itself.
var fmtAtomSpanOps = spanOps[fmtAtom]{
	strong:    func(a *fmtAtom) bool { return a.m.strong },
	em:        func(a *fmtAtom) bool { return a.m.em },
	strike:    func(a *fmtAtom) bool { return a.m.strike },
	isCode:    func(a *fmtAtom) bool { return a.isCode },
	setStrong: func(a *fmtAtom) { a.m.strong = true },
	setEm:     func(a *fmtAtom) { a.m.em = true },
	leaf:      func(a *fmtAtom) ast.Node { return atomLeaf(*a) },
}

// regroupAtoms re-infers marks across code spans and regroups a flat atom
// run into canonical strong/em/delete nesting.
func regroupAtoms(atoms []fmtAtom) []ast.Node {
	inferAcrossCode(atoms, fmtAtomSpanOps)
	return groupSpans(joinTextAtoms(atoms), fmtAtomSpanOps, false, false, false)
}

// joinTextAtoms concatenates neighboring plain-text atoms carrying the
// same marks. Flattening leaves two of them adjacent whenever what stood
// between them normalized to nothing (an empty link, a ':u' with no
// content), and the renderer's escaping is node-local: a one-byte
// lookahead cannot see that "0@A" and ".A" are written contiguously and
// re-parse as one email autolink literal (probe: "0@A:u.A"). Joining them
// hands the renderer the run the parser will see.
func joinTextAtoms(atoms []fmtAtom) []fmtAtom {
	out := make([]fmtAtom, 0, len(atoms))
	for i := range atoms {
		if n := len(out) - 1; n >= 0 && joinableText(&out[n], &atoms[i]) {
			out[n].text += atoms[i].text
			continue
		}
		out = append(out, atoms[i])
	}
	return out
}

// joinableText reports whether two adjacent atoms are plain text under the
// same marks, so their content can be written as one node.
func joinableText(a, b *fmtAtom) bool {
	return a.node == nil && b.node == nil &&
		!a.isCode && !b.isCode && !a.isBreak && !b.isBreak &&
		sameMarks(&a.m, &b.m)
}

// sameMarks compares two mark contexts field by field; the annotation
// list keeps fmtMarks out of reach of the == operator.
func sameMarks(a, b *fmtMarks) bool {
	return a.href == b.href && a.linkTitle == b.linkTitle &&
		a.textColor == b.textColor && a.bgColor == b.bgColor &&
		a.subsup == b.subsup &&
		a.link == b.link && a.linkBare == b.linkBare &&
		a.linkExplicit == b.linkExplicit &&
		a.strong == b.strong && a.em == b.em &&
		a.strike == b.strike && a.underline == b.underline &&
		slices.Equal(a.annotations, b.annotations)
}

func (fn *normalizer) flattenInlines(nodes []ast.Node, ctx fmtMarks) []fmtAtom {
	var out []fmtAtom
	for _, n := range nodes {
		out = append(out, fn.flattenInline(n, ctx)...)
	}
	return out
}

//nolint:cyclop,funlen // flat type-switch dispatch over the inline dialect kinds
func (fn *normalizer) flattenInline(n ast.Node, ctx fmtMarks) []fmtAtom {
	switch v := n.(type) {
	case *ast.Text:
		if v.Value == "" {
			return nil
		}
		// Normalize runs only for the prettier formatter, whose renderer
		// consumes the escape-preserving source form (ast.Text.Raw); feed
		// that through so atomLeaf rebuilds the leaf with the escapes intact.
		// Plain (non-prettier) rendering and the ADF encode never reach here
		// and read the decoded Value directly.
		return []fmtAtom{{text: v.Rendered(), m: ctx}}
	case *ast.InlineCode:
		// The code mark is exclusive in ADF: only the link survives, and
		// only its href (title/source-form drop).
		m := fmtMarks{}
		if ctx.link {
			m.link, m.href = true, ctx.href
		}
		return []fmtAtom{{text: v.Value, isCode: true, m: m}}
	case *ast.Strong:
		next := ctx
		next.strong = true
		return fn.flattenInlines(v.Children, next)
	case *ast.Emphasis:
		next := ctx
		next.em = true
		return fn.flattenInlines(v.Children, next)
	case *ast.Delete:
		next := ctx
		next.strike = true
		return fn.flattenInlines(v.Children, next)
	case *ast.Link:
		next := ctx
		next.link, next.href = true, v.URL
		next.linkTitle, next.linkBare, next.linkExplicit = v.Title, v.Bare, v.Explicit
		return fn.flattenInlines(v.Children, next)
	case *ast.Image:
		// Inline images ride as opaque atoms; inherited marks drop (ADF
		// carried them as a synthetic node without a mark slot).
		img := &ast.Image{URL: v.URL, Title: v.Title, Children: fn.normalizeInlines(v.Children)}
		return []fmtAtom{{node: img}}
	case *ast.FootnoteRef:
		// A reference rides as an opaque atom under its marks: the label
		// is the identifier the definition pairs on, so nothing in it may
		// be rewritten (see footnote.go).
		return []fmtAtom{{node: &ast.FootnoteRef{Label: v.Label}, m: ctx}}
	case *ast.Break:
		return []fmtAtom{{isBreak: true, spacesBreak: v.Value == "  "}}
	case *ast.HTML:
		if v.Value == "" {
			return nil
		}
		return []fmtAtom{{node: &ast.HTML{Value: v.Value}}}
	case *ast.TextDirective:
		// Generic (unknown) text directive: the colon-prefixed name as
		// plain text followed by the label; attributes drop.
		out := []fmtAtom{{text: ":" + v.Name, m: ctx}}
		return append(out, fn.flattenInlines(v.Children, ctx)...)
	case *dialect.Color:
		next := ctx
		next.textColor = v.Attrs["color"]
		return fn.flattenInlines(v.Children, next)
	case *dialect.Bg:
		next := ctx
		next.bgColor = v.Attrs["color"]
		return fn.flattenInlines(v.Children, next)
	case *dialect.Underline:
		next := ctx
		next.underline = true
		return fn.flattenInlines(v.Children, next)
	case *dialect.Sub:
		next := ctx
		next.subsup = "sub"
		return fn.flattenInlines(v.Children, next)
	case *dialect.Sup:
		next := ctx
		next.subsup = "sup"
		return fn.flattenInlines(v.Children, next)
	case *dialect.FontSize:
		// fontSize is retired: no product supports the mark, so the
		// formatter dissolves the directive to its inline text (matching
		// the ADF encode) and reports the drop.
		fn.diag(CodeFontSizeDropped, fontSizeDroppedMessage)
		return fn.flattenInlines(v.Children, ctx)
	case *dialect.Annotation:
		id := v.Attrs["id"]
		if id == "" {
			return fn.flattenInlines(v.Children, ctx)
		}
		annotationType := v.Attrs["annotationType"]
		if annotationType == "" {
			annotationType = "inlineComment"
		}
		next := ctx
		next.annotations = append(slices.Clone(ctx.annotations),
			extension.Annotation{ID: id, AnnotationType: annotationType})
		return fn.flattenInlines(v.Children, next)
	case *dialect.Mention:
		return atomOrNone(normalizeMention(v))
	case *dialect.Status:
		return atomOrNone(normalizeStatus(v))
	case *dialect.MediaInline:
		return atomOrNone(normalizeMediaInline(v))
	case *dialect.Emoji:
		return flattenEmoji(v)
	case *dialect.Date:
		return atomOrNone(normalizeDate(v))
	case *dialect.Placeholder:
		return atomOrNone(normalizePlaceholder(v))
	case *dialect.InlineExtension:
		return atomOrNone(normalizeInlineExtension(v))
	case *dialect.Panel, *dialect.Expand, *dialect.Media, *dialect.MediaCaption,
		*dialect.JQL, *dialect.LinkCard, *dialect.LinkEmbed, *dialect.Colwidths,
		*dialect.Decisions, *dialect.Extension, *dialect.SyncBlock,
		*dialect.BodiedExtension, *dialect.Frame, *dialect.BodiedSyncBlock,
		*dialect.Section, *dialect.Column, *dialect.Align, *dialect.Indent,
		*dialect.Breakout, *dialect.DataConsumer, *dialect.Fragment:
		// Dialect block kinds in inline position have no inline ADF form
		// and drop (they encode to block nodes the inline decode ignores).
		return nil
	}
	if _, isExt := n.(extension.Node); isExt {
		// Foreign extension kinds pass through untouched: without their
		// ADF leg the formatter cannot re-derive their canonical payload,
		// and rendering the parsed node is the identity.
		return []fmtAtom{{node: n}}
	}
	// Block kinds in inline position degrade like convert's fallback:
	// recurse into children, else keep the raw text value.
	if kids := ast.Children(n); len(kids) > 0 {
		return fn.flattenInlines(kids, ctx)
	}
	if val := blockNodeValue(n); val != "" {
		return []fmtAtom{{text: val, m: ctx}}
	}
	return nil
}

// atomOrNone wraps a normalized inline node as one opaque atom (nil
// drops it).
func atomOrNone(n ast.Node) []fmtAtom {
	if n == nil {
		return nil
	}
	return []fmtAtom{{node: n}}
}

// blockNodeValue mirrors convert's nodeValue fallback.
func blockNodeValue(n ast.Node) string {
	switch v := n.(type) {
	case *ast.Code:
		return v.Value
	case *ast.Frontmatter:
		return v.Value
	}
	return ""
}

// atomLeaf converts one atom to its AST leaf, wrapping the non-native
// marks in their typed directive kinds in convert's canonical nesting
// order (outside → inside): annotations, :color, :bg, :u, :sub/:sup;
// the link wraps innermost around the text/code node. (fontSize is
// retired — dropped to bare text — so it never wraps here.)
func atomLeaf(item fmtAtom) ast.Node {
	if item.isBreak {
		if item.spacesBreak {
			return &ast.Break{Value: "  "}
		}
		return &ast.Break{}
	}
	if item.node != nil {
		if ref, isRef := item.node.(*ast.FootnoteRef); isRef {
			// A footnote reference keeps its inherited marks, unlike the
			// other opaque atoms (an image carries none in ADF): the
			// marks around a reference are the source's own, and the ADF
			// encode puts them on the superscript the reference becomes,
			// so dropping them here would break the invariant Normalize
			// owes ToADF.
			return wrapAtomMarks(ref, item.m)
		}
		return item.node
	}

	var node ast.Node
	if item.isCode {
		node = &ast.InlineCode{Value: item.text}
	} else {
		node = &ast.Text{Value: item.text}
	}
	return wrapAtomMarks(node, item.m)
}

// wrapAtomMarks wraps a leaf in the atom's non-native marks, in convert's
// canonical nesting order (see atomLeaf).
func wrapAtomMarks(node ast.Node, m fmtMarks) ast.Node {
	if m.link {
		node = &ast.Link{
			URL:      m.href,
			Title:    m.linkTitle,
			Bare:     m.linkBare,
			Explicit: m.linkExplicit,
			Children: []ast.Node{node},
		}
	}
	if m.subsup != "" {
		if m.subsup == "sup" {
			node = &dialect.Sup{Children: []ast.Node{node}}
		} else {
			node = &dialect.Sub{Children: []ast.Node{node}}
		}
	}
	if m.underline {
		node = &dialect.Underline{Children: []ast.Node{node}}
	}
	if m.bgColor != "" {
		node = &dialect.Bg{Color: m.bgColor, Attrs: map[string]string{"color": m.bgColor}, Children: []ast.Node{node}}
	}
	if m.textColor != "" {
		node = &dialect.Color{Color: m.textColor, Attrs: map[string]string{"color": m.textColor}, Children: []ast.Node{node}}
	}
	for _, a := range slices.Backward(m.annotations) {
		node = &dialect.Annotation{
			ID:       a.ID,
			Attrs:    map[string]string{"id": a.ID, "annotationType": a.AnnotationType},
			Children: []ast.Node{node},
		}
	}
	return node
}

// ---------------------------------------------------------------------------
// Typed inline kinds: canonical payload re-derivation
// ---------------------------------------------------------------------------

func normalizeMention(v *dialect.Mention) ast.Node {
	label := strings.TrimSpace(ast.PlainText(v.Children))
	if label == "" {
		return nil
	}
	attrs := map[string]string{}
	if v.Attrs["id"] != "" {
		attrs["id"] = v.Attrs["id"]
	}
	if v.Attrs["accessLevel"] != "" {
		attrs["accessLevel"] = v.Attrs["accessLevel"]
	}
	return &dialect.Mention{
		AccountID: attrs["id"],
		Attrs:     attrs,
		Children:  []ast.Node{&ast.Text{Value: label}},
	}
}

func normalizeStatus(v *dialect.Status) ast.Node {
	label := strings.TrimSpace(ast.PlainText(v.Children))
	if label == "" {
		return nil
	}
	color := v.Attrs["color"]
	if color == "" {
		color = "neutral"
	}
	attrs := map[string]string{"color": color}
	if v.Attrs["style"] != "" {
		attrs["style"] = v.Attrs["style"]
	}
	return &dialect.Status{
		Color:    attrs["color"],
		Attrs:    attrs,
		Children: []ast.Node{&ast.Text{Value: label}},
	}
}

func normalizeMediaInline(v *dialect.MediaInline) ast.Node {
	mtype := v.Attrs["type"]
	if mtype == "" {
		mtype = "file"
	}
	attrs := map[string]string{}
	// Omit the default media type ("file") — encode re-infers it, and the
	// ADF decode omits it too (mirrors dialect's decodeMediaInline).
	if mtype != "file" {
		attrs["type"] = mtype
	}
	// A collection is not a default: mediaInline encode carries an empty
	// collection through as one, so its presence is the caller's to keep.
	if c, ok := v.Attrs["collection"]; ok {
		attrs["collection"] = c
	}
	if v.Attrs["id"] != "" {
		attrs["id"] = v.Attrs["id"]
	}
	var children []ast.Node
	if alt := strings.TrimSpace(ast.PlainText(v.Children)); alt != "" {
		children = []ast.Node{&ast.Text{Value: alt}}
	}
	return &dialect.MediaInline{
		// The effective type, as decode reports it, even where the
		// rendered attributes leave the default out.
		MediaType:  mtype,
		ID:         attrs["id"],
		Collection: attrs["collection"],
		Attrs:      attrs,
		Children:   children,
	}
}

// flattenEmoji mirrors Emoji.EncodeADF ∘ convert's VisitEmoji: emojis
// with rendered text (or a known shortname) become plain text atoms
// WITHOUT inherited marks; the directive survives only for custom ones.
func flattenEmoji(v *dialect.Emoji) []fmtAtom {
	shortName := v.Attrs["shortName"]
	if shortName == "" {
		return nil
	}
	if text, ok := v.Attrs["text"]; ok {
		return []fmtAtom{{text: text}}
	}
	if unicode, ok := dialect.EmojiUnicode(shortName); ok {
		return []fmtAtom{{text: unicode}}
	}
	attrs := map[string]string{"shortName": shortName}
	if v.Attrs["id"] != "" {
		attrs["id"] = v.Attrs["id"]
	}
	return []fmtAtom{{node: &dialect.Emoji{Attrs: attrs}}}
}

func normalizeDate(v *dialect.Date) ast.Node {
	ts := v.Attrs["timestamp"]
	if ts == "" {
		t, err := parseDateLabel(ast.PlainText(v.Children))
		if err != nil {
			return nil
		}
		ts = t
	}
	attrs := map[string]string{"timestamp": ts}
	if v.Attrs["localId"] != "" {
		attrs["localId"] = v.Attrs["localId"]
	}
	var children []ast.Node
	if label, ok := dateLabelFromMillis(ts); ok {
		children = []ast.Node{&ast.Text{Value: label}}
	}
	return &dialect.Date{Attrs: attrs, Children: children}
}

// parseDateLabel mirrors Date.EncodeADF's label fallback: the
// YYYY-MM-DD label parsed as a UTC day, in epoch milliseconds.
func parseDateLabel(label string) (string, error) {
	t, err := time.Parse(time.DateOnly, label)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(t.UnixMilli(), 10), nil
}

// dateLabelFromMillis mirrors decodeDate's label derivation.
func dateLabelFromMillis(ts string) (string, bool) {
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return "", false
	}
	return time.UnixMilli(ms).UTC().Format(time.DateOnly), true
}

func normalizePlaceholder(v *dialect.Placeholder) ast.Node {
	text := ast.PlainText(v.Children)
	if text == "" {
		return nil
	}
	attrs := map[string]string{}
	if v.Attrs["localId"] != "" {
		attrs["localId"] = v.Attrs["localId"]
	}
	return &dialect.Placeholder{Attrs: attrs, Children: []ast.Node{&ast.Text{Value: text}}}
}

func normalizeInlineExtension(v *dialect.InlineExtension) ast.Node {
	if v.Attrs["type"] == "" || v.Attrs["key"] == "" {
		return nil
	}
	attrs := extensionAttrPayload(v.Attrs, false)
	return &dialect.InlineExtension{Attrs: attrs}
}

// extensionAttrPayload mirrors dialect's extensionDirectiveAttrs over a
// directive attribute map: the parameters attribute re-encodes through
// the canonical JSON form, empty attributes drop. Layout rides only on
// the block forms.
func extensionAttrPayload(src map[string]string, withLayout bool) map[string]string {
	attrs := map[string]string{}
	if src["type"] != "" {
		attrs["type"] = src["type"]
	}
	if src["key"] != "" {
		attrs["key"] = src["key"]
	}
	if v, ok := src["parameters"]; ok && v != "" {
		if encoded, ok := dialect.EncodeJSONAttr(dialect.DecodeJSONAttr(v)); ok {
			attrs["parameters"] = encoded
		}
	}
	if src["text"] != "" {
		attrs["text"] = src["text"]
	}
	if withLayout && src["layout"] != "" {
		attrs["layout"] = src["layout"]
	}
	if src["localId"] != "" {
		attrs["localId"] = src["localId"]
	}
	return attrs
}

// ---------------------------------------------------------------------------
// Block normalization
// ---------------------------------------------------------------------------

// encKind tags the intermediate items of the block pass — the stand-ins
// for the top-level ADF nodes the former encode phase produced.
type encKind int

const (
	encNormal encKind = iota
	encTable
	encColwidths
	encMediaGroup
	encMediaSingle
	encDecisions
)

// fmtWrapper is one block-mark wrapper (:::center, :::indent, …) applied
// to an item; the stack is inner-first.
type fmtWrapper struct {
	wrap func(kids []ast.Node) ast.Node
}

// encItem is one intermediate block item.
type encItem struct {
	node     ast.Node // encNormal: decoded node; encTable/encDecisions: the table/list
	media    *fmtMedia
	caption  []fmtAtom
	medias   []*fmtMedia
	widths   []float64
	wrappers []fmtWrapper
	kind     encKind
	gap      bool
}

func (fn *normalizer) normalizeBlocks(nodes []ast.Node) []ast.Node {
	items := fn.resolveColwidths(fn.encodeBlocks(nodes))
	var out []ast.Node
	for i := range items {
		out = append(out, fn.decodeItem(items[i])...)
	}
	return out
}

// encodeBlocks mirrors convert's convertBlocks over a sibling run.
func (fn *normalizer) encodeBlocks(nodes []ast.Node) []encItem {
	var out []encItem
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		gap := ast.GapBefore(node)
		if _, isDecisions := node.(*dialect.Decisions); isDecisions {
			if list := decisionTargetList(nodes, i); list != nil {
				out = appendEncItem(out, encItem{kind: encDecisions, node: fn.decisionsList(list)}, gap)
				i++ // list consumed
				continue
			}
			fn.diag(CodeDecisionsOrphan,
				"::decisions directive with no bullet list on the following line was dropped")
			continue
		}
		encoded := fn.encodeBlockNode(node)
		for j := range encoded {
			out = appendEncItem(out, encoded[j], j == 0 && gap)
		}
	}
	return out
}

// appendEncItem mirrors convert's appendBlock: the gap flag lands on the
// item, and adjacent unwrapped ::media{group} items reassemble one group.
func appendEncItem(out []encItem, it encItem, gap bool) []encItem {
	if gap {
		it.gap = true
	}
	if it.kind == encMediaGroup && len(it.wrappers) == 0 && len(out) > 0 {
		if prev := &out[len(out)-1]; prev.kind == encMediaGroup && len(prev.wrappers) == 0 {
			prev.medias = append(prev.medias, it.medias...)
			return out
		}
	}
	return append(out, it)
}

// resolveColwidths attaches a widths item to the immediately following
// table; orphans drop. It shares the cross-sibling matcher with the ADF
// encode (resolveColwidthTargets); only the payload — copying the widths
// onto the encTable item — is normalize-specific.
func (fn *normalizer) resolveColwidths(items []encItem) []encItem {
	return resolveColwidthTargets(items,
		func(it encItem) ([]float64, bool) {
			if it.kind == encColwidths {
				return it.widths, true
			}
			return nil, false
		},
		func(it encItem) bool { return it.kind == encTable },
		func(table encItem, widths []float64) encItem {
			table.widths = widths
			return table
		},
		func() {
			fn.diag(CodeColwidthsOrphan,
				"::colwidths directive with no table on the following line was dropped")
		},
	)
}

// encodeBlockNode converts one AST block to its intermediate items.
//
//nolint:cyclop,funlen,maintidx // flat type-switch dispatch over the block dialect kinds
func (fn *normalizer) encodeBlockNode(node ast.Node) []encItem {
	switch v := node.(type) {
	case *ast.Paragraph:
		return normalItem(&ast.Paragraph{Children: fn.normalizeInlines(v.Children)})
	case *ast.Heading:
		level := min(max(v.Depth, 1), 6)
		return normalItem(&ast.Heading{Depth: level, ID: v.ID, Children: fn.normalizeInlines(v.Children)})
	case *ast.ThematicBreak:
		return normalItem(&ast.ThematicBreak{})
	case *ast.Blockquote:
		return normalItem(&ast.Blockquote{Children: fn.normalizeBlocks(v.Children)})
	case *ast.FootnoteDef:
		// Footnotes survive Normalize: only the ADF leg flattens them
		// (see footnote.go), and the invariant Normalize owes ToADF holds
		// because the definition passes through unchanged. Keeping it is
		// what makes the md → md formatter footnote-preserving.
		return normalItem(&ast.FootnoteDef{Label: v.Label, Children: fn.normalizeBlocks(v.Children)})
	case *ast.Code:
		fn.checkCodeLanguage(v.Lang)
		return normalItem(&ast.Code{Lang: v.Lang, Value: strings.TrimRight(v.Value, "\n")})
	case *ast.HTML:
		return normalItem(&ast.HTML{Value: v.Value})
	case *ast.Frontmatter:
		return normalItem(&ast.Frontmatter{Value: v.Value})
	case *ast.List:
		return normalItem(fn.normalizeList(v))
	case *ast.Table:
		t := fn.normalizeTable(v)
		if t == nil {
			return nil
		}
		return []encItem{{kind: encTable, node: t}}
	case *ast.ContainerDirective:
		// Generic (unknown) container: a single converted child replaces
		// it; anything else drops.
		sub := fn.resolveColwidths(fn.encodeBlocks(v.Children))
		if len(sub) == 1 {
			return sub
		}
		return nil
	case *ast.LeafDirective:
		// Generic (unknown) leaf directives drop.
		return nil
	case *dialect.Colwidths:
		widths := parseColwidths(ast.PlainText(v.Children))
		if len(widths) == 0 {
			return nil
		}
		return []encItem{{kind: encColwidths, widths: widths}}
	case *dialect.Media:
		m := mediaFromAttrs(v.Attrs, ast.PlainText(v.Children))
		if v.Attrs["group"] == "true" {
			return []encItem{{kind: encMediaGroup, medias: []*fmtMedia{m}}}
		}
		m.applySingle(v.Attrs)
		return []encItem{{kind: encMediaSingle, media: m}}
	case *dialect.MediaCaption:
		return fn.encodeMediaCaption(v)
	case *dialect.JQL:
		return normalOrDrop(fn.normalizeJQL(v))
	case *dialect.LinkCard:
		return normalOrDrop(fn.normalizeLinkCard(v))
	case *dialect.LinkEmbed:
		return normalOrDrop(fn.normalizeLinkEmbed(v))
	case *dialect.Decisions:
		// A ::decisions in a non-sibling position (unreachable through
		// encodeBlocks) drops like an orphan.
		return nil
	case *dialect.Panel:
		return normalItem(&dialect.Panel{PanelType: v.PanelType, Children: fn.normalizeBlocks(v.Children)})
	case *dialect.Expand:
		return normalItem(fn.normalizeExpand(v))
	case *dialect.Extension:
		if v.Attrs["type"] == "" || v.Attrs["key"] == "" {
			return nil
		}
		return normalItem(&dialect.Extension{Attrs: extensionAttrPayload(v.Attrs, true)})
	case *dialect.BodiedExtension:
		return fn.encodeBodiedExtension(v)
	case *dialect.Frame:
		return normalItem(&dialect.Frame{Children: fn.normalizeBlocks(v.Children)})
	case *dialect.SyncBlock:
		if v.Attrs["resourceId"] == "" {
			return nil
		}
		return normalItem(&dialect.SyncBlock{Attrs: syncAttrPayload(v.Attrs)})
	case *dialect.BodiedSyncBlock:
		if v.Attrs["resourceId"] == "" {
			return fn.resolveColwidths(fn.encodeBlocks(v.Children))
		}
		return normalItem(&dialect.BodiedSyncBlock{
			Attrs:    syncAttrPayload(v.Attrs),
			Children: fn.normalizeBlocks(v.Children),
		})
	case *dialect.Section:
		attrs := map[string]string{}
		if v.Attrs["columnRuleStyle"] != "" {
			attrs["columnRuleStyle"] = v.Attrs["columnRuleStyle"]
		}
		if v.Attrs["localId"] != "" {
			attrs["localId"] = v.Attrs["localId"]
		}
		return normalItem(&dialect.Section{Attrs: attrs, Children: fn.normalizeBlocks(v.Children)})
	case *dialect.Column:
		attrs := map[string]string{}
		if v.Attrs["localId"] != "" {
			attrs["localId"] = v.Attrs["localId"]
		}
		if v.Attrs["valign"] != "" {
			attrs["valign"] = v.Attrs["valign"]
		}
		if f, err := strconv.ParseFloat(v.Attrs["width"], 64); err == nil {
			attrs["width"] = formatJSNumber(f)
		}
		return normalItem(&dialect.Column{Attrs: attrs, Children: fn.normalizeBlocks(v.Children)})
	case *dialect.Align:
		if v.Align != "center" && v.Align != "end" {
			return fn.resolveColwidths(fn.encodeBlocks(v.Children))
		}
		align := v.Align
		return fn.wrapItems(v.Children, func(kids []ast.Node) ast.Node {
			return &dialect.Align{Align: align, Children: kids}
		})
	case *dialect.Indent:
		level, err := strconv.Atoi(v.Level())
		if err != nil || level < 1 {
			return fn.resolveColwidths(fn.encodeBlocks(v.Children))
		}
		return fn.wrapItems(v.Children, func(kids []ast.Node) ast.Node {
			return &dialect.Indent{Attrs: map[string]string{strconv.Itoa(level): ""}, Children: kids}
		})
	case *dialect.Breakout:
		mode := v.Mode()
		if mode == "" {
			return fn.resolveColwidths(fn.encodeBlocks(v.Children))
		}
		attrs := map[string]string{mode: ""}
		if f, err := strconv.ParseFloat(v.Attrs["width"], 64); err == nil {
			attrs["width"] = formatJSNumber(f)
		}
		return fn.wrapItems(v.Children, func(kids []ast.Node) ast.Node {
			return &dialect.Breakout{Attrs: cloneAttrs(attrs), Children: kids}
		})
	case *dialect.DataConsumer:
		sources := dialect.ParseSources(v.Attrs["sources"])
		if len(sources) == 0 {
			return fn.resolveColwidths(fn.encodeBlocks(v.Children))
		}
		return fn.wrapItems(v.Children, func(kids []ast.Node) ast.Node {
			return &dialect.DataConsumer{Attrs: map[string]string{"sources": dialect.EncodeSources(sources)}, Children: kids}
		})
	case *dialect.Fragment:
		if v.Attrs["localId"] == "" {
			return fn.resolveColwidths(fn.encodeBlocks(v.Children))
		}
		attrs := map[string]string{"localId": v.Attrs["localId"]}
		if v.Attrs["name"] != "" {
			attrs["name"] = v.Attrs["name"]
		}
		return fn.wrapItems(v.Children, func(kids []ast.Node) ast.Node {
			return &dialect.Fragment{Attrs: cloneAttrs(attrs), Children: kids}
		})
	}
	if _, isExt := node.(extension.Node); isExt {
		// Foreign extension kinds pass through untouched (see the inline
		// counterpart above).
		return normalItem(node)
	}
	// Everything else (inline kinds or structural children in block
	// position) degrades like convert's blockFallback: recurse into the
	// children and keep the first converted block.
	if kids := ast.Children(node); len(kids) > 0 {
		sub := fn.resolveColwidths(fn.encodeBlocks(kids))
		if len(sub) > 0 {
			return sub[:1]
		}
	}
	return nil
}

// wrapItems encodes a block-mark wrapper's children and pushes the
// wrapper onto every produced item (convert appends the ADF mark to each
// encoded block; the decode side then wraps each marked block).
func (fn *normalizer) wrapItems(children []ast.Node, wrap func([]ast.Node) ast.Node) []encItem {
	items := fn.resolveColwidths(fn.encodeBlocks(children))
	for i := range items {
		items[i].wrappers = append(items[i].wrappers, fmtWrapper{wrap: wrap})
	}
	return items
}

func normalItem(n ast.Node) []encItem {
	return []encItem{{kind: encNormal, node: n}}
}

func normalOrDrop(n ast.Node) []encItem {
	if n == nil {
		return nil
	}
	return normalItem(n)
}

func cloneAttrs(attrs map[string]string) map[string]string {
	return maps.Clone(attrs)
}

// decodeItem turns one intermediate item into its final AST nodes,
// mirroring convert's convertAdfBlocks decode dispatch (including which
// paths restore the blank-line gap and which lose it).
func (fn *normalizer) decodeItem(it encItem) []ast.Node {
	var group []ast.Node
	gap := it.gap
	switch it.kind {
	case encColwidths:
		// Unreachable: resolveColwidths consumed or dropped every widths
		// item before decoding.
	case encNormal:
		group = []ast.Node{it.node}
	case encTable:
		if ws := tableWidthsLabel(it.node, it.widths); ws != "" {
			group = append(group, &dialect.Colwidths{Children: []ast.Node{&ast.Text{Value: ws}}})
		}
		group = append(group, it.node)
		gap = false // the table decode branch never restores the gap
	case encDecisions:
		group = []ast.Node{&dialect.Decisions{}, it.node}
		gap = false // ditto for the decision-list branch
	case encMediaGroup:
		for _, m := range it.medias {
			group = append(group, fn.mediaLeafNode(m, true))
		}
		gap = false // the one-to-many media-group decode never reads it
	case encMediaSingle:
		group = []ast.Node{fn.decodeMediaSingle(it)}
	}
	for _, w := range it.wrappers {
		if len(group) == 0 {
			break
		}
		group = []ast.Node{w.wrap(group)}
	}
	if gap && len(group) > 0 {
		ast.SetGapBefore(group[0], true)
	}
	return group
}

// checkCodeLanguage mirrors convert's diagnostic of the same name.
func (fn *normalizer) checkCodeLanguage(lang string) {
	if lang == "" || fn.codeLangs == nil || fn.diagnostics == nil {
		return
	}
	if fn.codeLangs[strings.ToLower(lang)] {
		return
	}
	fn.diag(CodeUnsupportedCodeLanguage,
		"code block language "+strconv.Quote(lang)+" is not in the configured supported set")
}

// ---------------------------------------------------------------------------
// Lists
// ---------------------------------------------------------------------------

func (fn *normalizer) normalizeList(v *ast.List) ast.Node {
	items := make([]*ast.ListItem, 0, len(v.Children))
	isTask := false
	for i := range v.Children {
		if item, ok := v.Children[i].(*ast.ListItem); ok {
			items = append(items, item)
			if item.Checked != nil {
				isTask = true
			}
		}
	}
	if isTask {
		return fn.normalizeTaskItems(items)
	}
	listItems := make([]ast.Node, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, &ast.ListItem{
			Spread:   item.Spread,
			GapAfter: item.GapAfter,
			Children: fn.normalizeBlocks(item.Children),
		})
	}
	start := 1
	if v.Ordered {
		start = v.Start
	}
	return &ast.List{
		Ordered:       v.Ordered,
		Start:         start,
		Spread:        v.Spread,
		PerItemSpread: v.PerItemSpread,
		Increment:     v.Increment,
		OrderedGap:    v.OrderedGap,
		Children:      listItems,
	}
}

// normalizeTaskItems mirrors convert's convertTaskItems ∘
// convertAdfTaskList: task lists collapse to the canonical tight form,
// every item gains an explicit checkbox state, and items without a
// leading paragraph flatten their paragraph inlines.
func (fn *normalizer) normalizeTaskItems(items []*ast.ListItem) ast.Node {
	out := make([]ast.Node, 0, len(items))
	for _, item := range items {
		checked := item.Checked != nil && *item.Checked
		c := checked
		if p, single := singleParagraphTaskItem(item); single {
			out = append(out, &ast.ListItem{
				Checked:  &c,
				Children: []ast.Node{&ast.Paragraph{Children: fn.normalizeInlines(p.Children)}},
			})
			continue
		}
		if taskItemLead(item.Children) {
			blocks := fn.normalizeBlocks(item.Children)
			if !taskBlocksRenderable(blocks) {
				blocks = []ast.Node{&ast.Paragraph{Children: fn.flattenedParagraphInlines(item.Children)}}
			}
			out = append(out, &ast.ListItem{Checked: &c, Children: blocks})
			continue
		}
		out = append(out, &ast.ListItem{
			Checked:  &c,
			Children: []ast.Node{&ast.Paragraph{Children: fn.flattenedParagraphInlines(item.Children)}},
		})
	}
	return &ast.List{Children: out}
}

// flattenedParagraphInlines normalizes each paragraph child's inlines
// separately and concatenates the results (the historical
// paragraph-flattening projection).
func (fn *normalizer) flattenedParagraphInlines(children []ast.Node) []ast.Node {
	var out []ast.Node
	for _, c := range children {
		if p, ok := c.(*ast.Paragraph); ok {
			out = append(out, fn.normalizeInlines(p.Children)...)
		}
	}
	return out
}

func singleParagraphTaskItem(item *ast.ListItem) (*ast.Paragraph, bool) {
	if len(item.Children) == 0 {
		return &ast.Paragraph{}, true
	}
	if len(item.Children) != 1 {
		return nil, false
	}
	p, ok := item.Children[0].(*ast.Paragraph)
	return p, ok
}

func taskItemLead(children []ast.Node) bool {
	if len(children) == 0 {
		return false
	}
	p, ok := children[0].(*ast.Paragraph)
	return ok && len(p.Children) > 0
}

func taskBlocksRenderable(blocks []ast.Node) bool {
	if len(blocks) == 0 {
		return false
	}
	p, ok := blocks[0].(*ast.Paragraph)
	return ok && len(p.Children) > 0
}

// ---------------------------------------------------------------------------
// Decisions
// ---------------------------------------------------------------------------

// decisionsList mirrors convertDecisionItems ∘ convertAdfDecisionList:
// each item's paragraph inlines flatten into one run, other blocks drop,
// and the list collapses to the canonical tight plain form.
func (fn *normalizer) decisionsList(list *ast.List) ast.Node {
	var items []ast.Node
	for i := range list.Children {
		item, ok := list.Children[i].(*ast.ListItem)
		if !ok {
			continue
		}
		var atoms []fmtAtom
		for _, c := range item.Children {
			if p, isPara := c.(*ast.Paragraph); isPara {
				atoms = append(atoms, fn.flattenInlines(p.Children, fmtMarks{})...)
			}
		}
		items = append(items, &ast.ListItem{
			Children: []ast.Node{&ast.Paragraph{Children: regroupAtoms(atoms)}},
		})
	}
	return &ast.List{Children: items}
}

// ---------------------------------------------------------------------------
// Tables
// ---------------------------------------------------------------------------

// normalizeTable mirrors convertTable ∘ convertAdfTable: non-row/cell
// children drop, spans stay only when >1, and every row pads to the
// header row's visual column count (rowspan carries included). A table
// without rows drops.
func (fn *normalizer) normalizeTable(v *ast.Table) ast.Node {
	type encCell struct {
		atoms  []fmtAtom
		cs, rs int
	}
	var rows [][]encCell
	for i := range v.Children {
		row, ok := v.Children[i].(*ast.TableRow)
		if !ok {
			continue
		}
		var cells []encCell
		for j := range row.Children {
			cell, ok := row.Children[j].(*ast.TableCell)
			if !ok {
				continue
			}
			cells = append(cells, encCell{
				atoms: fn.flattenInlines(cell.Children, fmtMarks{}),
				cs:    spanValue(cell.ColSpan),
				rs:    spanValue(cell.RowSpan),
			})
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return nil
	}

	isHeader := len(rows[0]) > 0
	colCount := 0
	for _, c := range rows[0] {
		colCount += max(c.cs, 1)
	}

	type rowCarry struct{ rowsLeft, width int }
	var carries []rowCarry

	convertRow := func(cells []encCell) ast.Node {
		var mdCells []ast.Node
		var newCarries []rowCarry
		width := 0
		for _, c := range carries {
			width += c.width
		}
		for _, cell := range cells {
			mdCell := &ast.TableCell{Children: regroupAtoms(cell.atoms)}
			cs := max(cell.cs, 1)
			if cs > 1 {
				mdCell.ColSpan = cs
			}
			if cell.rs > 1 {
				mdCell.RowSpan = cell.rs
				newCarries = append(newCarries, rowCarry{rowsLeft: cell.rs - 1, width: cs})
			}
			width += cs
			mdCells = append(mdCells, mdCell)
		}
		for width < colCount {
			mdCells = append(mdCells, &ast.TableCell{})
			width++
		}
		var next []rowCarry
		for _, c := range carries {
			if c.rowsLeft > 1 {
				next = append(next, rowCarry{rowsLeft: c.rowsLeft - 1, width: c.width})
			}
		}
		next = append(next, newCarries...)
		carries = next
		return &ast.TableRow{Children: mdCells}
	}

	var mdRows []ast.Node
	if !isHeader {
		emptyHeader := &ast.TableRow{Children: make([]ast.Node, colCount)}
		for i := range emptyHeader.Children {
			emptyHeader.Children[i] = &ast.TableCell{}
		}
		mdRows = append(mdRows, emptyHeader)
	}
	for _, row := range rows {
		mdRows = append(mdRows, convertRow(row))
	}
	// Alignment survives the ADF leg on the synthetic carrier, so the
	// formatter must carry it too or format and conversion would disagree
	// (see format_contract_test.go).
	return &ast.Table{Children: mdRows, Align: v.Align}
}

// spanValue keeps a span only when it spans (>1).
func spanValue(v int) int {
	if v > 1 {
		return v
	}
	return 0
}

// parseColwidths mirrors Colwidths.EncodeADF's label parse: positive
// numbers only.
func parseColwidths(label string) []float64 {
	var widths []float64
	for part := range strings.SplitSeq(label, ",") {
		if f, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil && f > 0 {
			widths = append(widths, f)
		}
	}
	return widths
}

// tableWidthsLabel mirrors applyTableColwidths ∘ tableColwidths for the
// re-emitted ::colwidths label: the widths the FIRST row's cells cover,
// in JS number formatting.
func tableWidthsLabel(table ast.Node, widths []float64) string {
	if len(widths) == 0 {
		return ""
	}
	t, ok := table.(*ast.Table)
	if !ok || len(t.Children) == 0 {
		return ""
	}
	row, ok := t.Children[0].(*ast.TableRow)
	if !ok {
		return ""
	}
	var parts []string
	found := false
	col := 0
	for _, cellNode := range row.Children {
		cell, ok := cellNode.(*ast.TableCell)
		if !ok {
			continue
		}
		cs := max(cell.ColSpan, 1)
		if col < len(widths) {
			for _, f := range widths[col:min(col+cs, len(widths))] {
				parts = append(parts, formatJSNumber(f))
				found = true
			}
		}
		col += cs
	}
	if !found {
		return ""
	}
	return strings.Join(parts, ",")
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

// normalizeExpand mirrors Expand.EncodeADF ∘ decodeExpand: the label
// paragraph's plain text becomes the title, re-emitted as a plain-text
// label (rich label formatting flattens).
func (fn *normalizer) normalizeExpand(v *dialect.Expand) ast.Node {
	children := v.Children
	title := ""
	if p, ok := expandLabelParagraph(children); ok {
		title = ast.PlainText(p.Children)
		children = children[1:]
	}
	decoded := fn.normalizeBlocks(children)
	if strings.TrimSpace(title) != "" {
		label := &ast.Paragraph{DirectiveLabel: true, Children: []ast.Node{&ast.Text{Value: title}}}
		decoded = append([]ast.Node{label}, decoded...)
	}
	return &dialect.Expand{Children: decoded}
}

// expandLabelParagraph mirrors dialect's labelParagraph.
func expandLabelParagraph(children []ast.Node) (*ast.Paragraph, bool) {
	if len(children) == 0 {
		return nil, false
	}
	p, ok := children[0].(*ast.Paragraph)
	if !ok || !p.DirectiveLabel {
		return nil, false
	}
	return p, true
}

// ---------------------------------------------------------------------------
// Cards (::jql, ::linkCard, ::linkEmbed)
// ---------------------------------------------------------------------------

// smartLinkURL mirrors convert's encode-side resolver.
func (fn *normalizer) smartLinkURL(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return ""
	}
	if fn.encodeSL.URLForKey != nil {
		if url, ok := fn.encodeSL.URLForKey(trimmed); ok {
			return url
		}
	}
	return trimmed
}

// smartLinkLabel mirrors convert's decode-side labeling.
// smartLinkLabel shortens a card URL to its key ONLY when the shortening
// is reversible under the configured resolver — the encode side can
// expand the key back to the exact same URL (URLForKey(KeyFromURL(url))
// == url). Canonicalization must never be lossy: without a matching
// expansion the short label could not rebuild the URL, so the full URL
// is kept. (The adf->md render path shortens for readability regardless;
// its round trip is recovered by the paired encode config.)
func (fn *normalizer) smartLinkLabel(url string) string {
	if fn.decodeSL.KeyFromURL == nil || fn.encodeSL.URLForKey == nil {
		return url
	}
	key, ok := fn.decodeSL.KeyFromURL(url)
	if !ok {
		return url
	}
	if back, ok := fn.encodeSL.URLForKey(key); !ok || back != url {
		return url
	}
	return key
}

// normalizeJQL mirrors JQL.EncodeADF ∘ decodeDatasource (with the
// ::linkCard fallback for shapes the datasource decode rejects).
func (fn *normalizer) normalizeJQL(v *dialect.JQL) ast.Node {
	jql := ast.PlainText(v.Children)
	id := v.Attrs["datasource"]
	cloudID := v.Attrs["cloudId"]
	if jql == "" || id == "" || cloudID == "" {
		return nil
	}
	attrs := map[string]string{"cloudId": cloudID, "datasource": id}
	if v.Attrs["url"] != "" {
		attrs["url"] = v.Attrs["url"]
	}
	if columns, ok := v.Attrs["columns"]; ok && columns != "" {
		keys := strings.Split(columns, ",")
		for _, key := range keys {
			if key == "" || strings.ContainsAny(key, ",\"\n") {
				// The datasource decode rejects this view shape; the
				// blockCard fallback keeps only the url.
				return fn.blockCardFallback(v.Attrs["url"])
			}
		}
		attrs["columns"] = strings.Join(keys, ",")
	}
	return &dialect.JQL{
		CloudID:    attrs["cloudId"],
		Datasource: attrs["datasource"],
		Columns:    attrs["columns"],
		URL:        attrs["url"],
		Attrs:      attrs,
		Children:   []ast.Node{&ast.Text{Value: jql}},
	}
}

// blockCardFallback mirrors decodeBlockCard for a datasource card the
// ::jql decode rejected: a URL-less card is consumed without output.
func (fn *normalizer) blockCardFallback(url string) ast.Node {
	if url == "" {
		return nil
	}
	return &dialect.LinkCard{Children: []ast.Node{&ast.Text{Value: fn.smartLinkLabel(url)}}}
}

func (fn *normalizer) normalizeLinkCard(v *dialect.LinkCard) ast.Node {
	url := fn.smartLinkURL(ast.PlainText(v.Children))
	if url == "" {
		return nil
	}
	return &dialect.LinkCard{Children: []ast.Node{&ast.Text{Value: fn.smartLinkLabel(url)}}}
}

func (fn *normalizer) normalizeLinkEmbed(v *dialect.LinkEmbed) ast.Node {
	url := fn.smartLinkURL(ast.PlainText(v.Children))
	if url == "" {
		return nil
	}
	layout := v.Attrs["layout"]
	if layout == "" {
		layout = "center"
	}
	attrs := map[string]string{"layout": layout}
	if f, err := strconv.ParseFloat(v.Attrs["width"], 64); err == nil {
		attrs["width"] = formatJSNumber(f)
	}
	return &dialect.LinkEmbed{
		Layout:   attrs["layout"],
		Width:    parseFloatAttr(attrs, "width"),
		Attrs:    attrs,
		Children: []ast.Node{&ast.Text{Value: fn.smartLinkLabel(url)}},
	}
}

// parseFloatAttr mirrors dialect's floatAttr (0 when absent/invalid).
func parseFloatAttr(attrs map[string]string, key string) float64 {
	f, err := strconv.ParseFloat(attrs[key], 64)
	if err != nil {
		return 0
	}
	return f
}

// syncAttrPayload mirrors dialect's syncBlockAttrs from a directive map.
func syncAttrPayload(src map[string]string) map[string]string {
	attrs := map[string]string{}
	if src["resourceId"] != "" {
		attrs["resourceId"] = src["resourceId"]
	}
	if src["localId"] != "" {
		attrs["localId"] = src["localId"]
	}
	return attrs
}

// ---------------------------------------------------------------------------
// Bodied extension
// ---------------------------------------------------------------------------

func (fn *normalizer) encodeBodiedExtension(v *dialect.BodiedExtension) []encItem {
	if v.Attrs["type"] == "" || v.Attrs["key"] == "" {
		return fn.resolveColwidths(fn.encodeBlocks(v.Children))
	}
	_, multi := v.Attrs["multi"]
	if !multi && len(v.Children) > 0 {
		multi = true
		for _, child := range v.Children {
			if _, isFrame := child.(*dialect.Frame); !isFrame {
				multi = false
				break
			}
		}
	}
	content := fn.normalizeBlocks(v.Children)
	attrs := extensionAttrPayload(v.Attrs, true)
	if multi && len(content) == 0 {
		attrs["multi"] = ""
	}
	return normalItem(&dialect.BodiedExtension{Attrs: attrs, Children: content})
}

// ---------------------------------------------------------------------------
// Media
// ---------------------------------------------------------------------------

// fmtMedia mirrors the adf.Media (+ optional mediaSingle wrapper) shape
// the media directives encoded to.
type fmtMedia struct {
	collection    *string
	width         *float64
	height        *float64
	layout        *string
	layoutWidth   *float64
	widthType     *string
	mtype         string
	id            string
	path          string
	alt           string
	url           string
	occurrenceKey string
	borderColor   string
	borderSize    int
	hasBorder     bool
	hasSingle     bool
}

// mediaFromAttrs mirrors dialect's mediaFromAttrs.
func mediaFromAttrs(attrs map[string]string, alt string) *fmtMedia {
	m := &fmtMedia{mtype: "file"}
	if t := attrs["type"]; t != "" {
		m.mtype = t
	}
	m.id = attrs["id"]
	// A local asset carries its markdown-relative path (id may be omitted);
	// keep it so the canonical form round-trips without re-resolving.
	m.path = attrs["path"]
	if alt != "" {
		m.alt = alt
	}
	if v, ok := attrs["collection"]; ok {
		m.collection = &v
	}
	if v, ok := attrs["height"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.height = &f
		}
	}
	if v := attrs["occurrenceKey"]; v != "" {
		m.occurrenceKey = v
	}
	if v := attrs["url"]; v != "" {
		m.url = v
	}
	if v, ok := attrs["width"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.width = &f
		}
	}
	color, hasColor := attrs["borderColor"]
	sizeStr, hasSize := attrs["borderSize"]
	if hasColor || hasSize {
		m.hasBorder = true
		m.borderColor = color
		if size, err := strconv.Atoi(sizeStr); err == nil {
			m.borderSize = size
		}
	}
	return m
}

// applySingle mirrors dialect's mediaSingleFromAttrs.
func (m *fmtMedia) applySingle(attrs map[string]string) {
	m.hasSingle = true
	if v := attrs["layout"]; v != "" {
		m.layout = &v
	}
	if v, ok := attrs["layoutWidth"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.layoutWidth = &f
		}
	}
	if v := attrs["widthType"]; v != "" {
		m.widthType = &v
	}
}

// encodeMediaCaption mirrors MediaCaption.EncodeADF: the label is the
// alt, the body blocks flatten to caption inlines joined by hard breaks.
func (fn *normalizer) encodeMediaCaption(v *dialect.MediaCaption) []encItem {
	children := v.Children
	alt := ""
	if p, ok := expandLabelParagraph(children); ok {
		alt = ast.PlainText(p.Children)
		children = children[1:]
	}
	m := mediaFromAttrs(v.Attrs, alt)
	m.applySingle(v.Attrs)
	var caption []fmtAtom
	for _, block := range children {
		atoms := fn.flattenInlines(ast.Children(block), fmtMarks{})
		if len(atoms) == 0 {
			continue
		}
		if len(caption) > 0 {
			caption = append(caption, fmtAtom{isBreak: true})
		}
		caption = append(caption, atoms...)
	}
	return []encItem{{kind: encMediaSingle, media: m, caption: caption}}
}

// decodeMediaSingle mirrors decodeMediaBlock/decodeCaptionedMedia for a
// mediaSingle item.
func (fn *normalizer) decodeMediaSingle(it encItem) ast.Node {
	if len(it.caption) > 0 {
		return fn.decodeCaptionedMedia(it)
	}
	m := it.media
	if img := mediaAsImage(m); img != nil {
		return img
	}
	if img := fn.fileMediaAsImage(m); img != nil {
		return img
	}
	return fn.mediaLeafNode(m, false)
}

// decodeCaptionedMedia mirrors dialect's decodeCaptionedMedia: a plain
// text caption on image-expressible media becomes the image title,
// anything richer keeps the :::media container form.
func (fn *normalizer) decodeCaptionedMedia(it encItem) ast.Node {
	m := it.media
	inlines := regroupAtoms(it.caption)
	if title, ok := plainCaptionLabel(inlines); ok {
		if img := mediaAsImage(m); img != nil {
			setImageTitle(img, title)
			return img
		}
		if img := fn.fileMediaAsImage(m); img != nil {
			setImageTitle(img, title)
			return img
		}
	}
	leaf := fn.mediaLeafNode(m, false)
	var children []ast.Node
	if len(leaf.Children) > 0 {
		children = append(children, &ast.Paragraph{DirectiveLabel: true, Children: leaf.Children})
	}
	if len(inlines) > 0 {
		children = append(children, &ast.Paragraph{Children: inlines})
	}
	return &dialect.MediaCaption{Attrs: leaf.Attrs, Children: children}
}

// plainCaptionLabel mirrors dialect's plainCaptionText.
func plainCaptionLabel(inlines []ast.Node) (string, bool) {
	if len(inlines) != 1 {
		return "", false
	}
	text, ok := inlines[0].(*ast.Text)
	if !ok || text.Value == "" || strings.ContainsAny(text.Value, "\n\r\"\\") {
		return "", false
	}
	return text.Value, true
}

// setImageTitle mirrors dialect's setImageTitle.
func setImageTitle(n ast.Node, title string) {
	p, ok := n.(*ast.Paragraph)
	if !ok {
		return
	}
	for _, child := range p.Children {
		if img, ok := child.(*ast.Image); ok {
			img.Title = title
			return
		}
	}
}

// singleBlocksImage mirrors dialect's singleBlocksImage.
func (m *fmtMedia) singleBlocksImage(defaultLayout string) bool {
	if !m.hasSingle {
		return false
	}
	if m.layoutWidth != nil || m.widthType != nil {
		return true
	}
	return m.layout != nil && *m.layout != defaultLayout
}

// mediaAsImage mirrors dialect's mediaAsImage.
func mediaAsImage(m *fmtMedia) ast.Node {
	if m.mtype != "external" {
		return nil
	}
	if !strings.HasPrefix(m.url, "http://") && !strings.HasPrefix(m.url, "https://") {
		return nil
	}
	if m.width != nil || m.height != nil || m.occurrenceKey != "" {
		return nil
	}
	if m.collection != nil && *m.collection != "" {
		return nil
	}
	if m.hasBorder {
		return nil
	}
	if m.singleBlocksImage("center") {
		return nil
	}
	img := &ast.Image{URL: m.url}
	if m.alt != "" {
		img.Children = []ast.Node{&ast.Text{Value: m.alt}}
	}
	return &ast.Paragraph{Children: []ast.Node{img}}
}

// fileMediaAsImage mirrors dialect's fileMediaAsImage against the
// configured asset map.
func (fn *normalizer) fileMediaAsImage(m *fmtMedia) ast.Node {
	if m.mtype != "file" {
		return nil
	}
	asset, ok := fn.assets.lookup(m.id)
	if m.id == "" || !ok || !asset.HasDim {
		return nil
	}
	if m.width == nil || m.height == nil ||
		float64(asset.Width) != *m.width || float64(asset.Height) != *m.height {
		return nil
	}
	if m.collection != nil && *m.collection != "" {
		return nil
	}
	if m.occurrenceKey != "" {
		return nil
	}
	if m.hasBorder {
		return nil
	}
	if m.singleBlocksImage("align-start") {
		return nil
	}
	img := &ast.Image{URL: asset.Path}
	if m.alt != "" {
		img.Children = []ast.Node{&ast.Text{Value: m.alt}}
	}
	return &ast.Paragraph{Children: []ast.Node{img}}
}

// mediaOmissions mirrors dialect's mediaOmissions: the facts a canonical
// ::media leaves attributes out on.
type mediaOmissions struct {
	asset        MediaAsset
	isLocal      bool
	dimsMatch    bool
	naturalWidth bool
}

// mediaOmissionsOf derives them for one media shape.
func (fn *normalizer) mediaOmissionsOf(m *fmtMedia) mediaOmissions {
	var om mediaOmissions
	om.asset, om.isLocal = fn.assets.lookup(m.id)
	// A downloaded asset whose intrinsic dimensions match the file lets us omit
	// width/height — encode re-derives them (mirrors dialect's mediaLeafNode).
	om.dimsMatch = om.isLocal && om.asset.HasDim &&
		m.width != nil && m.height != nil &&
		float64(om.asset.Width) == *m.width && float64(om.asset.Height) == *m.height
	// A pixel display width equal to the intrinsic width is a no-op resize;
	// drop the redundant layoutWidth/widthType (mirrors dialect's
	// mediaLeafNode natural-width normalization).
	om.naturalWidth = m.layoutWidth != nil && m.width != nil &&
		*m.layoutWidth == *m.width && m.widthType != nil && *m.widthType == "pixel"
	return om
}

// mediaBorderAttrs writes the border attributes.
func mediaBorderAttrs(m *fmtMedia, attrs map[string]string) {
	if !m.hasBorder {
		return
	}
	if m.borderColor != "" {
		attrs["borderColor"] = m.borderColor
	}
	if m.borderSize != 0 {
		attrs["borderSize"] = strconv.Itoa(m.borderSize)
	}
}

// mediaShapeAttrs writes what the media says about itself: its container, its
// intrinsic size, its occurrence key.
func mediaShapeAttrs(m *fmtMedia, om mediaOmissions, group bool, attrs map[string]string) {
	// Omit an empty collection on file media (the attachment default).
	if m.collection != nil && (m.mtype != "file" || *m.collection != "") {
		attrs["collection"] = *m.collection
	}
	if group {
		attrs["group"] = "true"
	}
	if m.height != nil && !om.dimsMatch {
		attrs["height"] = formatJSNumber(*m.height)
	}
	if m.width != nil && !om.dimsMatch {
		attrs["width"] = formatJSNumber(*m.width)
	}
	if m.occurrenceKey != "" {
		attrs["occurrenceKey"] = m.occurrenceKey
	}
}

// mediaSingleAttrs writes what the mediaSingle wrapper says: alignment and
// display size.
func mediaSingleAttrs(m *fmtMedia, om mediaOmissions, attrs map[string]string) {
	if !m.hasSingle {
		return
	}
	// Omit the file-media default layout ("align-start") — encode re-infers
	// it (mirrors dialect's mediaLeafNode).
	if m.layout != nil && *m.layout != "" && (m.mtype != "file" || *m.layout != "align-start") {
		attrs["layout"] = *m.layout
	}
	if m.layoutWidth != nil && !om.naturalWidth {
		attrs["layoutWidth"] = formatJSNumber(*m.layoutWidth)
	}
	if m.widthType != nil && *m.widthType != "" && !om.naturalWidth {
		attrs["widthType"] = *m.widthType
	}
}

// mediaSourceAttrs writes where the media comes from.
func mediaSourceAttrs(m *fmtMedia, om mediaOmissions, attrs map[string]string) {
	// A locally-downloaded asset emits its path and OMITS the explicit id
	// (encode resolves the id from the path via the scoped store); otherwise
	// keep the id (nothing can resolve it). Mirrors dialect's mediaLeafNode.
	path := m.path
	if om.isLocal {
		path = om.asset.Path
	}
	if path != "" {
		attrs["path"] = path
	} else if m.id != "" {
		attrs["id"] = m.id
	}
	// Omit the default media type ("file") — encode re-infers it.
	if m.mtype != "" && m.mtype != "file" {
		attrs["type"] = m.mtype
	}
	if m.url != "" {
		attrs["url"] = m.url
	}
}

// mediaLeafNode mirrors dialect's mediaLeafNode: the canonical ::media
// payload re-derived from the media shape.
func (fn *normalizer) mediaLeafNode(m *fmtMedia, group bool) *dialect.Media {
	om := fn.mediaOmissionsOf(m)
	attrs := map[string]string{}
	mediaBorderAttrs(m, attrs)
	mediaShapeAttrs(m, om, group, attrs)
	mediaSingleAttrs(m, om, attrs)
	mediaSourceAttrs(m, om, attrs)
	var children []ast.Node
	if m.alt != "" {
		children = []ast.Node{&ast.Text{Value: m.alt}}
	}
	return &dialect.Media{
		MediaType:     attrs["type"],
		URL:           attrs["url"],
		Collection:    attrs["collection"],
		Path:          attrs["path"],
		Layout:        attrs["layout"],
		ID:            attrs["id"],
		OccurrenceKey: attrs["occurrenceKey"],
		WidthType:     attrs["widthType"],
		Width:         parseFloatAttr(attrs, "width"),
		Height:        parseFloatAttr(attrs, "height"),
		LayoutWidth:   parseFloatAttr(attrs, "layoutWidth"),
		Group:         attrs["group"] == "true",
		Attrs:         attrs,
		Children:      children,
	}
}
