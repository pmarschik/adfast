package convert

import (
	"github.com/pmarschik/adfast/ast"
)

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
// the three nesting-mark predicates, the code-span predicate, the text and
// the setter inferAcrossCode needs, and the leaf constructor. Predicates
// and setters take *T so the caller can read and mutate items in place
// without T having to satisfy an interface.
type spanOps[T any] struct {
	strong func(*T) bool
	em     func(*T) bool
	strike func(*T) bool
	isCode func(*T) bool
	text   func(*T) string
	set    func(*T, spanMark)
	leaf   func(*T) ast.Node
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

// spanMarks are the three nesting marks, in the order a wrapper opens
// them. Ranging over this is how a rule stays exhaustive when a fourth
// one is added.
var spanMarks = [...]spanMark{spanStrong, spanEm, spanStrike}

// marked reports whether an item carries any nesting mark of its own.
func marked[T any](ops spanOps[T], item *T) bool {
	return ops.strong(item) || ops.em(item) || ops.strike(item)
}

// bareCode reports whether items[i] is a code span that carries no
// nesting mark — the shape a code span always has in ADF, whatever it was
// written inside, because the code mark is exclusive there.
func bareCode[T any](items []T, ops spanOps[T], i int) bool {
	if i < 0 || i >= len(items) {
		return false
	}
	item := &items[i]
	return ops.isCode(item) && !marked(ops, item)
}

// codeRunEnd returns the exclusive end of the run of bare code spans
// starting at start — how far a re-inferred mark may reach forward when
// the source marks are exact (see inferAfterCode's lax parameter). This
// mirrors codeRunStart: only a further bare code span extends the run,
// not an unmarked non-code item, because a genuinely unmarked item after
// the code span was never inside the construct that is being inferred.
func codeRunEnd[T any](items []T, ops spanOps[T], start int) int {
	end := start
	for bareCode(items, ops, end) {
		end++
	}
	return end
}

// unmarkedRunEnd returns the exclusive end of the run starting at start
// whose items carry no nesting mark of their own — how far a re-inferred
// mark may reach forward when the source marks are only a best-effort
// reconstruction (see inferAfterCode's lax parameter).
func unmarkedRunEnd[T any](items []T, ops spanOps[T], start int) int {
	end := start
	for end < len(items) && !marked(ops, &items[end]) {
		end++
	}
	return end
}

// codeRunStart returns the first index of the run of bare code spans
// ending at i — how far a re-inferred mark may reach backward.
func codeRunStart[T any](items []T, ops spanOps[T], i int) int {
	start := i
	for bareCode(items, ops, start-1) {
		start--
	}
	return start
}

// inferAcrossCode restores the nesting marks a code span shed on the way
// into ADF.
//
// ADF's code mark is exclusive, so `**text `code` more**` arrives as a
// strong run, a bare code span, and another strong run — the emphasis is
// still there on both sides, but nothing says the code span was inside
// it. Left alone the decode closes the emphasis at the code span and
// opens it again after. Reading the mark back off the neighboring run is
// what keeps the round trip on the form the author wrote.
//
// lax selects how far the forward pass (inferAfterCode) may reach past
// the code span; see there.
func inferAcrossCode[T any](items []T, ops spanOps[T], lax bool) {
	inferBeforeCode(items, ops)
	inferAfterCode(items, ops, lax)
}

// inferAfterCode carries the marks of an item forward over the bare code
// span that follows it.
//
// lax controls how far past the code span the mark may reach:
//
//   - false (the prettier formatter's Normalize pass, over an exact parsed
//     markdown tree): only a further bare code span extends the run: a
//     genuinely unmarked item right after the code was never inside the
//     construct being re-inferred — its marks are exact, not lossy — and
//     swallowing it would change the document's meaning (the fuzz
//     repro "~0`0`~!" formatted the trailing "!" into the strike run).
//   - true (the ADF decode, over marks Confluence/Jira's editor may have
//     dropped around a code span): the run also swallows the unmarked text
//     after the code span, recovering the form an author more likely wrote
//     (TestAdfToMarkdown_CodeMarkInference: "AC3: `GET /healthz` Endpoint"
//     decodes with "Endpoint" still inside the strong run).
func inferAfterCode[T any](items []T, ops spanOps[T], lax bool) {
	for i := range items {
		item := &items[i]
		if ops.isCode(item) || !marked(ops, item) || !bareCode(items, ops, i+1) {
			continue
		}
		var end int
		if lax {
			end = unmarkedRunEnd(items, ops, i+1)
		} else {
			end = codeRunEnd(items, ops, i+1)
		}
		for _, mark := range spanMarks {
			if !ops.has(item, mark) {
				continue
			}
			for k := i + 1; k < end; k++ {
				ops.set(&items[k], mark)
			}
		}
	}
}

// inferBeforeCode carries the marks of an item backward over the bare
// code spans that precede it, for the emphasis that opened on one.
//
// The evidence is the space the item starts with. `**`a` b**` is stored
// as a bare code span and a strong run holding " b": the space sat
// between the code span and the word inside the emphasis, so an emphasis
// whose content opens on whitespace is one that began before the code
// span. Markdown cannot write that space where ADF holds it — `** b**` is
// not emphasis to a parser — so the decode used to emit the character
// reference for it and hand back "`a`**&#x20;b**".
//
// Without the space the two readings are the same bytes: “ `a`**b** “
// and “ **`a`b** “ both store one bare code span beside one strong run,
// and the plain one is what the author more likely wrote.
//
// This is a deliberate divergence from remark, which has no notion of the
// exclusive code mark and so renders the run as written, character
// reference and all. The corpus entry for it in
// testdata/directive_fixtures.json is re-pinned to this form: the ADF the
// two spellings encode to is byte-identical, so only the markdown surface
// moved. inferAfterCode is the same divergence in the other direction.
func inferBeforeCode[T any](items []T, ops spanOps[T]) {
	for i := range items {
		item := &items[i]
		if ops.isCode(item) || !marked(ops, item) || !opensOnSpace(ops, item) {
			continue
		}
		if !bareCode(items, ops, i-1) {
			continue
		}
		start := codeRunStart(items, ops, i-1)
		for _, mark := range spanMarks {
			if !ops.has(item, mark) {
				continue
			}
			for k := start; k < i; k++ {
				ops.set(&items[k], mark)
			}
		}
	}
}

// opensOnSpace reports whether an item's text begins with the whitespace
// that a shed code mark leaves at the head of an emphasis.
func opensOnSpace[T any](ops spanOps[T], item *T) bool {
	text := ops.text(item)
	return text != "" && (text[0] == ' ' || text[0] == '\t')
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
