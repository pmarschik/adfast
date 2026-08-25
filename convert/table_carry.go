package convert

import "github.com/pmarschik/adfast/ast"

// The rowspan bookkeeping both table conversions share. The ADF decode
// (adf_to_ast.go) and the Normalize canonicalization (normalize.go) each
// walk a table row by row and pad every row to the header row's VISUAL
// column count — a colspan-N cell covers N columns, and a rowspan-M cell
// keeps covering its columns in the M-1 rows below. The two differ only
// in what a cell is made of, so the carry arithmetic lives here once
// instead of drifting in two copies.

// rowspanCarry is a column block a rowspan from an earlier row still
// covers: width visual columns for rowsLeft more rows.
type rowspanCarry struct {
	rowsLeft int
	width    int
}

// carryState tracks the columns standing rowspans cover as a table is
// converted row by row. Embed it in a row converter.
type carryState struct {
	carries []rowspanCarry
}

// carriedWidth is the number of visual columns standing rowspans already
// cover in the row about to be converted.
func (s *carryState) carriedWidth() int {
	width := 0
	for _, carry := range s.carries {
		width += carry.width
	}
	return width
}

// advance consumes one row of every standing rowspan and adds the ones
// the row just converted opened; fresh carries start covering the NEXT
// row, so they skip the aging step.
func (s *carryState) advance(fresh []rowspanCarry) {
	var next []rowspanCarry
	for _, carry := range s.carries {
		if carry.rowsLeft > 1 {
			next = append(next, rowspanCarry{rowsLeft: carry.rowsLeft - 1, width: carry.width})
		}
	}
	next = append(next, fresh...)
	s.carries = next
}

// emptyTableRow is the all-blank header markdown needs when a table
// starts straight into data rows.
func emptyTableRow(colCount int) ast.Node {
	cells := make([]ast.Node, colCount)
	for i := range cells {
		cells[i] = &ast.TableCell{}
	}
	return &ast.TableRow{Children: cells}
}
