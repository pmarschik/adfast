package confluence

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// The named sugar over the Confluence core macros. Every one of these is
// expressible as the generic ::extension{key type parameters} directive,
// but the parameters attribute carries a JSON blob no one wants to hand
// write, so the macros people actually use get a directive of their own:
//
//	::toc{maxLevel="3"}                ⇄ extension        (toc)
//	::children{sort="title"}           ⇄ extension        (children)
//	::pagetree / :pagetree             ⇄ extension        (pagetree)
//	:::excerpt … :::                   ⇄ bodiedExtension  (excerpt)
//	::excerptInclude[Page] / inline    ⇄ extension        (excerpt-include)
//	::includePage[Page]                ⇄ extension        (include)
//
// Each name registers in all three directive positions (leaf, text,
// container) because the same macro key genuinely appears as a block, an
// inline, and a bodied node in live pages; the ADF node type decides the
// form on the way back.
//
// Anything the sugar cannot represent losslessly is NOT claimed: the
// decode hooks return false and the node degrades through the generic
// ::extension path exactly as an unknown macro key does (see
// macroDirective).

// MacroExtensionType is the extensionType every core Confluence macro
// carries.
const MacroExtensionType = "com.atlassian.confluence.macro.core"

// macroSpec is one sugared macro: its ADF extensionKey plus the metadata
// Confluence stores alongside it. schemaVersion and title are constant
// per key (measured across every macro instance in a live space), so
// they are defaults: decode drops them, encode synthesizes them, and only
// a divergent value survives as an explicit attribute.
type macroSpec struct {
	key           string
	schemaVersion string
	title         string
}

// macroSpecs maps directive name → macro. The directive names are
// camelCase where the macro key is not a single word, matching the rest
// of the dialect (::linkCard, ::syncBlock).
var macroSpecs = map[string]macroSpec{
	"toc":            {key: "toc", schemaVersion: "1", title: "Table of Contents"},
	"children":       {key: "children", schemaVersion: "2", title: "Child pages"},
	"pagetree":       {key: "pagetree", schemaVersion: "1", title: "Page Tree"},
	"excerpt":        {key: "excerpt", schemaVersion: "1", title: "Excerpt"},
	"excerptInclude": {key: "excerpt-include", schemaVersion: "1", title: "Insert excerpt"},
	"includePage":    {key: "include", schemaVersion: "1", title: "Include Page"},
}

// macroNames is the reverse index (extensionKey → directive name), built
// from macroSpecs.
var macroNames = func() map[string]string {
	out := make(map[string]string, len(macroSpecs))
	for name, spec := range macroSpecs {
		out[spec.key] = name
	}
	return out
}()

// ---------------------------------------------------------------------------
// Nodes
// ---------------------------------------------------------------------------

// Macro is the block form — ::toc{…} ⇄ ADF extension (a bodiless macro).
// Name is the directive name (a key of macroSpecs), Attrs the macro
// parameters plus the reserved layout/localId/schemaVersion/title, and
// the children are the directive label: the macro's unnamed parameter,
// which is how the include and excerpt-include macros carry their target
// page.
type Macro struct {
	Attrs    map[string]string
	Name     string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*Macro) Kind() string { return "confluenceMacro" }

// ChildNodes implements ast.Parent.
func (n *Macro) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *Macro) SetChildNodes(kids []ast.Node) { n.Children = kids }

// RenderMarkdown implements extension.Node.
func (n *Macro) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective(n.Name, n.Attrs, n.Children)
}

// EncodeADF implements extension.Node: an unknown directive name cannot
// name a macro, so the node drops (it can only arise from a hand-built
// tree — the parser constructs these from the registered names).
func (n *Macro) EncodeADF(_ extension.EncodeContext) []adf.Node {
	spec, ok := macroSpecs[n.Name]
	if !ok {
		return nil
	}
	return []adf.Node{&adf.Extension{
		ExtensionType: MacroExtensionType,
		ExtensionKey:  spec.key,
		Parameters:    macroParameters(spec, n.Attrs, ast.PlainText(n.Children)),
		Layout:        n.Attrs["layout"],
		LocalID:       n.Attrs["localId"],
	}}
}

// InlineMacro is the inline form — :pagetree{…} ⇄ ADF inlineExtension.
// The same macro key appears inline and as a block in live pages, so
// every name registers in both positions.
type InlineMacro struct {
	Attrs    map[string]string
	Name     string
	Children []ast.Node
}

// Kind implements ast.Node.
func (*InlineMacro) Kind() string { return "confluenceInlineMacro" }

// ChildNodes implements ast.Parent.
func (n *InlineMacro) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *InlineMacro) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*InlineMacro) MarkdownLead() byte { return ':' }

// RenderMarkdown implements extension.Node.
func (n *InlineMacro) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective(n.Name, n.Attrs, n.Children)
}

// EncodeADF implements extension.Node (see Macro.EncodeADF). The inline
// node has no layout.
func (n *InlineMacro) EncodeADF(_ extension.EncodeContext) []adf.Node {
	spec, ok := macroSpecs[n.Name]
	if !ok {
		return nil
	}
	return []adf.Node{&adf.InlineExtension{
		ExtensionType: MacroExtensionType,
		ExtensionKey:  spec.key,
		Parameters:    macroParameters(spec, n.Attrs, ast.PlainText(n.Children)),
		LocalID:       n.Attrs["localId"],
	}}
}

// BodiedMacro is the container form — :::excerpt … ::: ⇄ ADF
// bodiedExtension. A leading label paragraph carries the unnamed
// parameter, the remaining children the macro body.
type BodiedMacro struct {
	Attrs    map[string]string
	Name     string
	Children []ast.Node
	ast.BlockSpacing
}

// Kind implements ast.Node.
func (*BodiedMacro) Kind() string { return "confluenceBodiedMacro" }

// ChildNodes implements ast.Parent.
func (n *BodiedMacro) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *BodiedMacro) SetChildNodes(kids []ast.Node) { n.Children = kids }

// ContainerDirectiveForm implements extension.ContainerForm.
func (*BodiedMacro) ContainerDirectiveForm() {}

// RenderMarkdown implements extension.Node.
func (n *BodiedMacro) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteContainerDirective(n.Name, n.Attrs, n.Children)
}

// EncodeADF implements extension.Node: an unknown directive name
// dissolves the container into its body rather than dropping it (the
// body is real content).
func (n *BodiedMacro) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	spec, ok := macroSpecs[n.Name]
	if !ok {
		return ctx.EncodeBlocks(n.Children)
	}
	children, label := n.Children, ""
	if p, hasLabel := labelParagraph(children); hasLabel {
		label = ast.PlainText(p.Children)
		children = children[1:]
	}
	return []adf.Node{&adf.BodiedExtension{
		ExtensionType: MacroExtensionType,
		ExtensionKey:  spec.key,
		Parameters:    macroParameters(spec, n.Attrs, label),
		Layout:        n.Attrs["layout"],
		LocalID:       n.Attrs["localId"],
		Content:       ctx.EncodeBlocks(children),
	}}
}

// labelParagraph returns the leading directive-label paragraph, if any.
func labelParagraph(children []ast.Node) (*ast.Paragraph, bool) {
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
// Registration
// ---------------------------------------------------------------------------

// Macros is the extension registration for the sugared Confluence
// macros. MarkdownOptions and RenderOptions install it; supply it
// directly (adfast.WithExtensions(confluence.Macros())) when composing
// options by hand.
func Macros() extension.Registration {
	reg := extension.Registration{
		Kind:         "confluenceMacro",
		Leaves:       make(map[string]func(*ast.LeafDirective) extension.Node, len(macroSpecs)),
		Texts:        make(map[string]func(*ast.TextDirective) extension.Node, len(macroSpecs)),
		Containers:   make(map[string]func(*ast.ContainerDirective) extension.Node, len(macroSpecs)),
		DecodeBlock:  decodeMacroBlock,
		DecodeInline: decodeMacroInline,
	}
	for name := range macroSpecs {
		reg.Leaves[name] = func(d *ast.LeafDirective) extension.Node {
			return &Macro{Name: d.Name, Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
		}
		reg.Texts[name] = func(d *ast.TextDirective) extension.Node {
			return &InlineMacro{Name: d.Name, Attrs: d.Attrs, Children: d.Children}
		}
		reg.Containers[name] = func(d *ast.ContainerDirective) extension.Node {
			return &BodiedMacro{Name: d.Name, Attrs: d.Attrs, Children: d.Children, BlockSpacing: d.BlockSpacing}
		}
	}
	return reg
}
