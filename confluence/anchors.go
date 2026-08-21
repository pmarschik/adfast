package confluence

import (
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
)

// Heading anchors: the markdown "## Title {#my-anchor}" surface ⇄ the
// Confluence anchor macro.
//
// ADF has no heading-anchor construct, so the root module carries the id
// as the synthetic never-wire attribute adf.Heading.Anchor (see
// adf.IsWireSafe) and leaves the lowering to the host product's addon.
// Confluence spells an anchor as the anchor macro — an inlineExtension
// sitting INSIDE the heading's content:
//
//	{"type": "inlineExtension", "attrs": {
//	  "extensionType": "com.atlassian.confluence.macro.core",
//	  "extensionKey": "anchor",
//	  "parameters": {"macroParams": {"": {"value": "my-anchor"}},
//	                 "macroMetadata": {"schemaVersion": {"value": "1"},
//	                                   "title": "Anchor"}}}}
//
// measured on a live page (read-only, 2026-08-21). Links to the anchor
// use the unnamed parameter as the URL fragment.
//
// It is NOT heading.attrs.localId: adf-schema documents that attribute as
// "an optional UUID for unique identification of the node" (it renders to
// DOM as data-local-id), it creates no link target, and live pages carry
// real UUIDs there that a human slug would collide with.
//
// Two parameters Confluence writes are deliberately NOT synthesized on
// encode, for the same reason macroparams.go drops macroId: they are
// derived, not authored. macroId is server-generated, and legacyAnchorId
// is "<PageTitle>-<name>" — a value adfast cannot know (the page title is
// not part of the document) and Confluence regenerates. Neither affects
// where the anchor points.
//
// Because the lowering moves a node between a heading's content and the
// heading's own attributes, it cannot be an extension decode hook (those
// map one ADF node to one AST node). It runs as a whole-document pass on
// each side: LowerAnchors before submission and LiftAnchors before decode.

// anchorMacro is the anchor macro's spec, also registered in macroSpecs so
// that a standalone anchor — one outside a heading, which has no heading
// to become an attribute of — still round trips through the generic
// :anchor[name] directive sugar.
var anchorMacro = macroSpecs["anchor"]

// LowerAnchors rewrites every heading's synthetic anchor attribute into an
// anchor-macro inlineExtension at the end of the heading's content, which
// is the shape Confluence stores and the position the markdown suffix
// occupies. Headings without an anchor are untouched, and the result is
// wire-safe (no synthetic attribute survives).
//
// MarkdownOptions installs it as a document transform; supply it directly
// (adfast.WithDocTransforms(confluence.LowerAnchors)) when composing
// options by hand.
func LowerAnchors(doc adf.Doc) adf.Doc {
	return adf.Transform(doc, func(n adf.Node) ([]adf.Node, bool) {
		h, ok := n.(*adf.Heading)
		if !ok || h.Anchor == "" {
			return nil, false
		}
		lowered := *h
		lowered.Anchor = ""
		lowered.Content = append(append([]adf.Node{}, h.Content...), &adf.InlineExtension{
			ExtensionType: MacroExtensionType,
			ExtensionKey:  anchorMacro.key,
			Parameters:    macroParameters(anchorMacro, nil, h.Anchor),
		})
		return []adf.Node{&lowered}, true
	})
}

// LiftAnchors is the inverse: a heading whose content holds exactly one
// anchor macro loses that node and gains the macro's name as its anchor
// attribute, so the decode renders it as the "{#name}" heading suffix.
//
// Anchors it declines stay in place and decode through the :anchor[name]
// directive instead — the lossless fallback. That covers a heading with
// several anchor macros (only one can be a "{#name}" suffix, and dropping
// the others would lose link targets), a name outside
// ast.HeadingIDPattern (Confluence accepts anchor names the markdown
// suffix cannot spell, such as one containing a space), and an anchor
// carrying anything the suffix cannot express, such as a diverging
// schemaVersion.
//
// RenderOptions installs it as an ADF transform; supply it directly
// (adfast.WithADFTransforms(confluence.LiftAnchors)) when composing
// options by hand.
func LiftAnchors(doc adf.Doc) adf.Doc {
	return adf.Transform(doc, func(n adf.Node) ([]adf.Node, bool) {
		h, ok := n.(*adf.Heading)
		if !ok {
			return nil, false
		}
		at, name := -1, ""
		for i, kid := range h.Content {
			ext, isExt := kid.(*adf.InlineExtension)
			if !isExt || ext.ExtensionType != MacroExtensionType || ext.ExtensionKey != anchorMacro.key {
				continue
			}
			if at >= 0 {
				return nil, false // more than one: leave them all as macros
			}
			liftable, isLiftable := anchorName(ext)
			if !isLiftable {
				return nil, false
			}
			at, name = i, liftable
		}
		if at < 0 {
			return nil, false
		}
		lifted := *h
		lifted.Anchor = name
		lifted.Content = trimAnchorGap(append(append([]adf.Node{}, h.Content[:at]...), h.Content[at+1:]...), at)
		return []adf.Node{&lifted}, true
	})
}

// trimAnchorGap removes the whitespace that separated the heading text
// from the anchor macro now removed at index at.
//
// A page written by hand holds that space inside the preceding text node
// ("Title " + the macro), because that is what the author typed. The
// markdown renderer supplies its own separator in the " {#id}" suffix, so
// leaving the space would both duplicate it and make it TRAILING heading
// text — which the renderer has to escape as "&#x20;" to preserve. Trimming
// is therefore the faithful inverse, not a cosmetic tidy-up.
func trimAnchorGap(content []adf.Node, at int) []adf.Node {
	if at == 0 {
		return content
	}
	prev, ok := content[at-1].(*adf.Text)
	if !ok {
		return content
	}
	trimmed := strings.TrimRight(prev.Text, " \t")
	if trimmed == prev.Text {
		return content
	}
	if trimmed == "" {
		return append(content[:at-1], content[at:]...)
	}
	shorter := *prev
	shorter.Text = trimmed
	content[at-1] = &shorter
	return content
}

// anchorName reads the anchor name out of an anchor-macro node, reporting
// false when the macro carries more than a "{#name}" suffix can hold.
// legacyAnchorId is the one attribute dropped rather than declined: it is
// derived from the page title and Confluence regenerates it (see the
// package comment above).
func anchorName(ext *adf.InlineExtension) (string, bool) {
	f, ok := macroSugar(ext.ExtensionKey, ext.Text, "", ext.LocalID, ext.Extra, ext.Parameters)
	if !ok || !ast.ValidHeadingID(f.label) {
		return "", false
	}
	for name := range f.attrs {
		if name != "legacyAnchorId" {
			return "", false
		}
	}
	return f.label, true
}
