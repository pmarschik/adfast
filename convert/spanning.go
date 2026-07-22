package convert

import "github.com/pmarschik/adfast/ast"

// This file is the single implementation of the flat→nested inline mark
// regrouping the library performs in two places: the ADF decode
// (adf_to_ast.go: ADF stores marks as flat per-text arrays) and the
// Normalize canonicalization (normalize.go: the faithful parse tree is
// flattened to atoms and regrouped into canonical nesting). Both feed a
// flat run of items carrying strong/em/strike/code state through the same
// spanning algorithm; the only per-caller differences — the item struct
// and its leaf constructor — are supplied through spanOps.

// spanMark identifies a nestable inline wrapper for the spanning
// algorithm (strong/em/delete are the three marks that nest in the AST;
// every other mark rides on the leaf).
type spanMark int

const (
	spanStrong spanMark = iota
	spanEm
	spanStrike
)

// spanOps adapts one flat inline item type T to the spanning algorithm:
// the three nesting-mark predicates, the code-span predicate, the two
// setters inferAcrossCode needs, and the leaf constructor. Predicates and
// setters take *T so the caller can read and mutate items in place
// without T having to satisfy an interface.
type spanOps[T any] struct {
	strong    func(*T) bool
	em        func(*T) bool
	strike    func(*T) bool
	isCode    func(*T) bool
	setStrong func(*T)
	setEm     func(*T)
	leaf      func(*T) ast.Node
}

func (ops spanOps[T]) has(item *T, mark spanMark) bool {
	switch mark {
	case spanStrong:
		return ops.strong(item)
	case spanEm:
		return ops.em(item)
	default: // spanStrike
		return ops.strike(item)
	}
}

// inferAcrossCode propagates strong/em marks across code boundaries: ADF
// strips strong/em from code spans, so `**text `code` more**` re-infers
// the marks onto the code span and the trailing run from the preceding
// item.
func inferAcrossCode[T any](items []T, ops spanOps[T]) {
	for i := 0; i < len(items); i++ {
		item := &items[i]
		if ops.isCode(item) {
			continue
		}
		if !ops.strong(item) && !ops.em(item) {
			continue
		}
		if i+1 >= len(items) {
			continue
		}
		next := &items[i+1]
		if !ops.isCode(next) || ops.strong(next) || ops.em(next) {
			continue
		}
		end := i + 1
		for end < len(items) {
			it := &items[end]
			if ops.strong(it) || ops.em(it) || ops.strike(it) {
				break
			}
			end++
		}
		for k := i + 1; k < end; k++ {
			if ops.strong(item) {
				ops.setStrong(&items[k])
			}
			if ops.em(item) {
				ops.setEm(&items[k])
			}
		}
		i = end - 1
	}
}

// groupSpans builds nested strong/em/delete wrappers from a flat run of
// items. Instead of grouping by identical mark sets, it finds the
// widest-spanning mark at the current position, opens it once around that
// whole span, and recurses for the marks nested inside it. openStrong/
// openEm/openStrike record the marks an ancestor wrapper already opened
// in the current recursion.
//
// Example: [{strike}, {strike+strong}, {strike}] →
// delete[text1, strong[text2], text3].
func groupSpans[T any](items []T, ops spanOps[T], openStrong, openEm, openStrike bool) []ast.Node {
	var out []ast.Node
	idx := 0
	for idx < len(items) {
		item := &items[idx]

		wantStrong := ops.strong(item) && !openStrong
		wantEm := ops.em(item) && !openEm
		wantStrike := ops.strike(item) && !openStrike

		if !wantStrong && !wantEm && !wantStrike {
			out = append(out, ops.leaf(item))
			idx++
			continue
		}

		// Find the mark with the widest contiguous span starting at idx;
		// strong > em > strike breaks ties (insertion order, strict >).
		type cand struct {
			mark spanMark
			end  int
		}
		var cands []cand
		if wantStrong {
			cands = append(cands, cand{spanStrong, spanEnd(items, ops, idx, spanStrong, openStrong, openEm, openStrike)})
		}
		if wantEm {
			cands = append(cands, cand{spanEm, spanEnd(items, ops, idx, spanEm, openStrong, openEm, openStrike)})
		}
		if wantStrike {
			cands = append(cands, cand{spanStrike, spanEnd(items, ops, idx, spanStrike, openStrong, openEm, openStrike)})
		}

		best := cands[0]
		for _, c := range cands[1:] {
			if c.end > best.end {
				best = c
			}
		}

		ns, ne, nst := openStrong, openEm, openStrike
		switch best.mark {
		case spanStrong:
			ns = true
		case spanEm:
			ne = true
		case spanStrike:
			nst = true
		}

		children := groupSpans(items[idx:best.end], ops, ns, ne, nst)
		var wrapper ast.Node
		switch best.mark {
		case spanStrong:
			wrapper = &ast.Strong{Children: children}
		case spanEm:
			wrapper = &ast.Emphasis{Children: children}
		default:
			wrapper = &ast.Delete{Children: children}
		}
		out = append(out, wrapper)
		idx = best.end
	}
	return out
}

// spanEnd returns the exclusive end of the contiguous run starting at idx
// whose items all carry the given mark plus every currently-open mark.
func spanEnd[T any](items []T, ops spanOps[T], idx int, mark spanMark, openStrong, openEm, openStrike bool) int {
	end := idx + 1
	for end < len(items) {
		it := &items[end]
		if !ops.has(it, mark) {
			break
		}
		if openStrong && !ops.strong(it) {
			break
		}
		if openEm && !ops.em(it) {
			break
		}
		if openStrike && !ops.strike(it) {
			break
		}
		end++
	}
	return end
}
