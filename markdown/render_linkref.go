package markdown

import (
	"strings"

	"github.com/pmarschik/adfast/ast"
)

// Rendering the link reference kinds (see ast/linkref.go): the definition
// block and the three written forms of a reference.
//
// The label is an IDENTIFIER, not prose: the definition and its uses pair
// on it, so it is written back verbatim through maskReferenceLabel and
// never through the escaping rules — an escape added here would be a
// character there, and the pairing would break. Only the full form has
// link text distinct from the label, and only there do the reference
// nodes' children render.

// renderDefinition writes "[label]: url" plus the optional title, the one
// line a link reference definition occupies. It is a block, so it may sit
// anywhere a block may (renderBlockSequence handles the surroundings), and
// it renders whether or not anything references it: an unused definition
// is still content the author wrote.
func (r *mdRenderer) renderDefinition(b *strings.Builder, node *ast.Definition) {
	b.WriteString("[")
	b.WriteString(maskReferenceLabel(node.Label, false))
	b.WriteString("]: ")
	b.WriteString(definitionURL(node.URL, r.cfg.prettierText))
	if node.Title != "" {
		b.WriteString(" \"")
		b.WriteString(r.escapeTitle(node.Title))
		b.WriteString("\"")
	}
	b.WriteString("\n")
}

// definitionURL serializes a definition's destination. It is an inline
// link's destination serializer, with one addition: the empty destination
// has to be written as the empty angle pair, because "[a]: " with nothing
// after the colon is not a definition at all and the whole block would
// vanish on the next parse.
func definitionURL(url string, prettier bool) string {
	if url == "" {
		return "<>"
	}
	return formatLinkURL(url, prettier)
}

// writeLinkRef serializes a reference-style link in the form the source
// used. The construct is one unbreakable wrap unit, like an inline link:
// a line break inside the brackets stops it re-parsing as a reference.
func (r *mdRenderer) writeLinkRef(b *strings.Builder, node *ast.LinkRef, st *inlineContext) {
	var ref strings.Builder
	switch node.ReferenceType {
	case ast.ReferenceFull:
		ref.WriteString("[")
		// Only here is the link text separate from the label, so only here
		// do the children render — through the same label context an
		// inline link's text uses.
		child := inlineContext{pipes: st.pipes, escape: r.cfg.prettierText, label: true, afterLead: ']'}
		r.writeInlines(&ref, node.Children, &child)
		ref.WriteString("][")
		ref.WriteString(maskReferenceLabel(node.Label, st.pipes))
		ref.WriteString("]")
	case ast.ReferenceCollapsed:
		ref.WriteString("[")
		ref.WriteString(maskReferenceLabel(node.Label, st.pipes))
		ref.WriteString("][]")
	default:
		// The shortcut form, and the safe answer for a ReferenceType this
		// build does not know (see ast.ReferenceType).
		ref.WriteString("[")
		ref.WriteString(maskReferenceLabel(node.Label, st.pipes))
		ref.WriteString("]")
	}
	b.WriteString(maskWrapUnit(ref.String()))
	// ']' is the last byte written: remark escapes a '(' that follows a
	// ']', so the next sibling has to see the bracket.
	st.prev, st.hasPrev = ']', true
	st.prevRune, st.encodeLead = ']', false
}

// writeImageRef serializes a reference-style image. It is writeLinkRef
// with the leading '!', and — like writeImage — the full form's alt text
// is the children flattened to plain text, since an image's label cannot
// carry inline markup. That flattening needs no renderer state, which is
// why this one is a plain function.
func writeImageRef(b *strings.Builder, node *ast.ImageRef, st *inlineContext) {
	var ref strings.Builder
	ref.WriteString("![")
	switch node.ReferenceType {
	case ast.ReferenceFull:
		ref.WriteString(maskAltPipes(escapeImageAlt(ast.PlainText(node.Children)), st.pipes))
		ref.WriteString("][")
		ref.WriteString(maskReferenceLabel(node.Label, st.pipes))
		ref.WriteString("]")
	case ast.ReferenceCollapsed:
		ref.WriteString(maskReferenceLabel(node.Label, st.pipes))
		ref.WriteString("][]")
	default:
		ref.WriteString(maskReferenceLabel(node.Label, st.pipes))
		ref.WriteString("]")
	}
	b.WriteString(maskWrapUnit(ref.String()))
	st.prev, st.hasPrev = ']', true
	st.prevRune, st.encodeLead = ']', false
}

// maskReferenceLabel prepares a label for writing: its line breaks become
// spaces (a label may be written across lines in the source, and the
// pairing rule collapses whitespace, so this cannot change which
// definition it finds) and its pipes are escaped inside a table cell.
// Nothing else is touched — see the file comment on why.
func maskReferenceLabel(label string, pipes bool) string {
	label = strings.ReplaceAll(label, "\r\n", " ")
	label = strings.NewReplacer("\n", " ", "\r", " ").Replace(label)
	if pipes {
		label = strings.ReplaceAll(label, "|", `\|`)
	}
	return label
}

// maskAltPipes escapes the pipes in an image's alt text inside a table
// cell (mdast-util-gfm-table); writeImage's inline path gets this from the
// same rule applied to the whole cell.
func maskAltPipes(alt string, pipes bool) string {
	if !pipes {
		return alt
	}
	return strings.ReplaceAll(alt, "|", `\|`)
}

// maskWrapUnit hides a construct's spaces and tabs from the prose wrapper,
// which restores them at the end of the render. Links and images do the
// same inline; this is that step named, for the reference forms.
func maskWrapUnit(s string) string {
	masked := strings.ReplaceAll(s, " ", string(wrapMask))
	return strings.ReplaceAll(masked, "\t", string(wrapMaskTab))
}
