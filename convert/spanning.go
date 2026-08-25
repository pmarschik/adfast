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

// openMarks records the nesting marks an ancestor wrapper already opened
// in the current groupSpans recursion. It travels as one value rather
// than three bools so a fourth nesting mark does not have to be threaded
// through every signature again.
type openMarks struct {
	strong bool
	em     bool
	strike bool
}

func (o openMarks) has(mark spanMark) bool {
	switch mark {
	case spanStrong:
		return o.strong
	case spanEm:
		return o.em
	default: // spanStrike
		return o.strike
	}
}

// with returns o with mark added.
func (o openMarks) with(mark spanMark) openMarks {
	switch mark {
	case spanStrong:
		o.strong = true
	case spanEm:
		o.em = true
	default: // spanStrike
		o.strike = true
	}
	return o
}

// carriesAcrossCode reports whether items[i] is a marked non-code item
// followed immediately by an unmarked code span — the shape ADF produces
// for `**text `code`**`, whose marks inferAcrossCode re-infers.
func carriesAcrossCode[T any](items []T, ops spanOps[T], i int) bool {
	item := &items[i]
	if ops.isCode(item) || (!ops.strong(item) && !ops.em(item)) {
		return false
	}
	if i+1 >= len(items) {
		return false
	}
	next := &items[i+1]
	return ops.isCode(next) && !ops.strong(next) && !ops.em(next)
}

// unmarkedRunEnd returns the exclusive end of the run starting at start
// whose items carry no nesting mark of their own — how far a re-inferred
// mark may reach.
func unmarkedRunEnd[T any](items []T, ops spanOps[T], start int) int {
	end := start
	for end < len(items) {
		it := &items[end]
		if ops.strong(it) || ops.em(it) || ops.strike(it) {
			break
		}
		end++
	}
	return end
}

// inferAcrossCode propagates strong/em marks across code boundaries: ADF
// strips strong/em from code spans, so `**text `code` more**` re-infers
// the marks onto the code span and the trailing run from the preceding
// item.
func inferAcrossCode[T any](items []T, ops spanOps[T]) {
	for i := 0; i < len(items); i++ {
		if !carriesAcrossCode(items, ops, i) {
			continue
		}
		item := &items[i]
		end := unmarkedRunEnd(items, ops, i+1)
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
// whole span, and recurses for the marks nested inside it. open records
// the marks an ancestor wrapper already opened in the current recursion.
//
// Example: [{strike}, {strike+strong}, {strike}] →
// delete[text1, strong[text2], text3].
func groupSpans[T any](items []T, ops spanOps[T], open openMarks) []ast.Node {
	var out []ast.Node
	idx := 0
	for idx < len(items) {
		mark, end, ok := widestSpan(items, ops, idx, open)
		if !ok {
			out = append(out, ops.leaf(&items[idx]))
			idx++
			continue
		}
		out = append(out, wrapSpan(mark, groupSpans(items[idx:end], ops, open.with(mark))))
		idx = end
	}
	return out
}

// widestSpan picks the not-yet-open mark on items[idx] whose contiguous
// run reaches furthest, and returns it with that run's exclusive end.
// Ties go to the first in strong > em > strike order. ok is false when
// the item has no mark left to open, i.e. it is a plain leaf here.
func widestSpan[T any](items []T, ops spanOps[T], idx int, open openMarks) (spanMark, int, bool) {
	bestMark, bestEnd, found := spanStrong, 0, false
	for _, mark := range []spanMark{spanStrong, spanEm, spanStrike} {
		if open.has(mark) || !ops.has(&items[idx], mark) {
			continue
		}
		if end := spanEnd(items, ops, idx, mark, open); !found || end > bestEnd {
			bestMark, bestEnd, found = mark, end, true
		}
	}
	return bestMark, bestEnd, found
}

// wrapSpan builds the AST wrapper for one nesting mark.
func wrapSpan(mark spanMark, children []ast.Node) ast.Node {
	switch mark {
	case spanStrong:
		return &ast.Strong{Children: children}
	case spanEm:
		return &ast.Emphasis{Children: children}
	default: // spanStrike
		return &ast.Delete{Children: children}
	}
}

// spanEnd returns the exclusive end of the contiguous run starting at idx
// whose items all carry the given mark plus every currently-open mark.
func spanEnd[T any](items []T, ops spanOps[T], idx int, mark spanMark, open openMarks) int {
	end := idx + 1
	for end < len(items) {
		it := &items[end]
		if !ops.has(it, mark) || !keepsOpen(ops, it, open) {
			break
		}
		end++
	}
	return end
}

// keepsOpen reports whether the item still carries every open mark, i.e.
// whether an ancestor wrapper can stay open across it.
func keepsOpen[T any](ops spanOps[T], it *T, open openMarks) bool {
	if open.strong && !ops.strong(it) {
		return false
	}
	if open.em && !ops.em(it) {
		return false
	}
	return !open.strike || ops.strike(it)
}
