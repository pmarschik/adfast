package convert

import "github.com/pmarschik/adfast/ast"

// GFM table column alignment across the ADF leg. ADF tables have no
// alignment attribute, so the alignment rides as adf.Table.Align, the
// synthetic never-wire carrier (see adf.IsWireSafe and the same pattern
// in ast.Heading.ID ↔ adf.Heading.Anchor). Both directions answer nil for
// a table with no alignment, so an unaligned table's ADF payload is
// exactly what it was before alignment existed.

// lowerTableAlign maps the pivot AST's per-column alignment onto the
// synthetic ADF attribute's strings.
func lowerTableAlign(align []ast.Alignment) []string {
	if !ast.AnyAligned(align) {
		return nil
	}
	out := make([]string, len(align))
	for i, a := range align {
		out[i] = string(a)
	}
	return out
}

// liftTableAlign maps the synthetic ADF attribute back to the pivot AST's
// per-column alignment. An unknown string reads as no alignment, the same
// way an unknown panelType degrades: a hand-built document cannot make the
// renderer write a delimiter row it has no spelling for.
func liftTableAlign(align []string) []ast.Alignment {
	out := make([]ast.Alignment, len(align))
	for i, s := range align {
		switch a := ast.Alignment(s); a {
		case ast.AlignLeft, ast.AlignRight, ast.AlignCenter:
			out[i] = a
		case ast.AlignNone:
		default:
		}
	}
	if !ast.AnyAligned(out) {
		return nil
	}
	return out
}
