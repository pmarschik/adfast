package ast

// Alignment is a GFM table column alignment, as the delimiter row spells
// it with colons: ":--" left, "--:" right, ":-:" center, "---" none. The
// values mirror mdast's table `align` list, where a column with no colons
// is null — AlignNone here.
type Alignment string

// The alignments a GFM delimiter row can express.
const (
	// AlignNone is a column whose delimiter cell carries no colon. It is
	// the zero value, so an absent alignment reads as "none".
	AlignNone Alignment = ""
	// AlignLeft is ":--".
	AlignLeft Alignment = "left"
	// AlignRight is "--:".
	AlignRight Alignment = "right"
	// AlignCenter is ":-:".
	AlignCenter Alignment = "center"
)

// ColumnAlign answers the alignment of visual column ci in align, which
// may be nil or shorter than the table is wide: every column the list
// does not reach is AlignNone. Renderers and converters use it instead of
// indexing, because a table's column count comes from its widest row and
// not from the delimiter row alone (a span marker can widen a row).
func ColumnAlign(align []Alignment, ci int) Alignment {
	if ci < 0 || ci >= len(align) {
		return AlignNone
	}
	return align[ci]
}

// AnyAligned reports whether align asks for anything at all. A table
// whose every column is AlignNone carries no alignment, and the parser
// leaves Align nil for it: that keeps a table without a colon in its
// delimiter row byte-identical through every leg, and keeps the ADF
// payload free of the synthetic carrier (see adf.Table.Align).
func AnyAligned(align []Alignment) bool {
	for _, a := range align {
		if a != AlignNone {
			return true
		}
	}
	return false
}
