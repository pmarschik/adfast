package markdown

import (
	"regexp"
	"sort"

	gast "github.com/yuin/goldmark/ast"
)

// TextMatches returns the byte span of every match of re that lies inside
// prose. See Source.TextMatches for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func TextMatches(src []byte, re *regexp.Regexp, group int) Spans {
	return NewSource(src).TextMatches(re, group)
}

// TextMatches returns the span of capture group `group` of every match of re
// that lies wholly inside one run of prose, in document order,
// non-overlapping. Group 0 is the whole match.
//
// This is the view for the caller that has a pattern over a document's WORDS
// — an issue key, a ticket reference, a bare URL — rather than over its
// syntax. Such a caller has always had to answer "is this hit real prose or
// is it an example?" for itself, and every hand-written answer in the tree
// answered it as a blocklist: find every hit, then drop the ones inside code.
// A blocklist is only as complete as its author's list of places that are not
// prose, and the ones that got left off are the ones that corrupt a document
// — a key inside a link's URL, rewritten to a URL that 404s; a key inside an
// HTML comment, rewritten in text the author had commented out on purpose.
//
// So the rule here is the other way round, and is stated positively: a match
// counts only when the parser calls every byte of it literal text. Everything
// that is not literal text is excluded BY CONSTRUCTION rather than by being
// remembered — a code block and an indented code block (they hold no text
// node at all), an inline code span, a link's or an image's destination and
// title, an autolink, a link reference definition, an HTML block, inline HTML
// including a comment, and a directive's attribute block. A label IS prose
// and does match, on a link, an image, and a leaf or container directive:
// the author wrote those words to be read.
//
// A RUN is a maximal stretch of source bytes the parser reports as literal
// text with nothing in between. Runs matter because goldmark splits a text
// node at every inline parser trigger, so "blocks PROJ-NEW-2 and more" is
// already two text nodes and matching inside each one separately would miss
// keys that merely straddle a split. Two text nodes join into one run only
// when they are ADJACENT in the source; anything the parser consumed between
// them — a backtick, an emphasis delimiter, a newline — ends the run.
//
// The consequence is deliberate: a match BROKEN BY MARKUP does not match.
// "PROJ-*NEW*-1" is not a key, because a reader does not see one either. The
// same rule silently costs one real spelling: GFM reads "org~repo~NEW-1" as
// strikethrough, so its middle is a separate run and the whole key is not
// reported. That is the safe direction — the document already renders as
// struck-through text, and a caller that rewrote its middle would be editing
// the inside of markup it never looked at.
//
// re is matched against the WHOLE source, not against each run, and the run
// test is then applied to the span this returns. Matching per-run would
// silently change what a pattern means: `^` would fire at every run boundary,
// and a pattern whose left boundary is a character class would see the run's
// first byte instead of the byte the author wrote before it. Testing the
// RETURNED span rather than the whole match is what lets that boundary
// character be markup — a key opening a table cell or wrapped in emphasis is
// still prose, and its match necessarily starts on a byte that is not.
// Callers move a pattern here without rewriting it, and get the same hits
// they got from a scan over the raw text, minus the ones that were never
// prose.
//
// A match whose group did not participate is skipped, and so is one whose
// group is empty: this view exists to feed Apply, and a span covering no byte
// is not an edit anyone here asked for. A group index past the pattern's
// groups yields nothing rather than panicking, because the mismatch is
// between a caller's pattern and its own call and no document is at fault.
func (s *Source) TextMatches(re *regexp.Regexp, group int) Spans {
	if re == nil || group < 0 {
		return nil
	}
	runs := s.proseSpans()
	if len(runs) == 0 {
		return nil
	}
	var out Spans
	for _, m := range re.FindAllSubmatchIndex(s.src, -1) {
		at := 2 * group
		if at+1 >= len(m) || m[at] < 0 {
			continue
		}
		sp := Span{Start: m[at], Stop: m[at+1]}
		if sp.Len() <= 0 || !runs.encloses(sp) {
			continue
		}
		out = append(out, sp)
	}
	return out
}

// encloses reports whether a single span of ss covers all of s. A match
// straddling two spans is NOT enclosed, which is the whole point: the bytes
// between them are markup, and a caller splicing across them would rewrite it.
func (ss Spans) encloses(s Span) bool {
	if s.Len() <= 0 {
		return false
	}
	// Ascending and non-overlapping, so at most one span can reach past
	// s.Start without starting after it.
	i := sort.Search(len(ss), func(i int) bool { return ss[i].Stop > s.Start })
	return i < len(ss) && ss[i].Start <= s.Start && s.Stop <= ss[i].Stop
}

// proseSpans returns the runs of literal text, memoized. It stays unexported
// until a caller needs the runs themselves rather than the matches in them;
// exposing it later is an addition, un-exposing it would not be.
func (s *Source) proseSpans() Spans {
	if s.proseDone {
		return s.prose
	}
	s.prose, s.proseDone = collectProseRuns(s.doc, s.src), true
	return s.prose
}

// collectProseRuns walks doc for text nodes and coalesces the adjacent ones.
//
// The walk does not descend into a code span. Its content is carried as text
// nodes like any other inline content, so the node kind is the only thing
// separating `PROJ-NEW-1` from the prose around it, and reading it as prose
// is exactly the mistake this view exists to make impossible.
//
// Offsets are required to be non-decreasing, and a text node that goes
// backwards is dropped rather than trusted. That does two jobs. It is what
// makes the coalescing test — "does this node start where the last one
// stopped" — mean adjacency rather than coincidence. And it is the guard
// against a subtree parsed against a DETACHED buffer, which is how a text
// directive's label is parsed: such a node's offsets address bytes that are
// not in this document, and handing one to Apply would splice into the middle
// of an unrelated sentence. Today's parser attaches no such node here, so no
// test drives that half of the guard; it is kept because the alternative
// failure is silent corruption of a document nobody edited.
func collectProseRuns(doc gast.Node, src []byte) Spans {
	var out Spans
	next := 0
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if _, code := c.(*gast.CodeSpan); code {
				continue
			}
			if t, text := c.(*gast.Text); text {
				sp := Span{Start: t.Segment.Start, Stop: t.Segment.Stop}
				if sp.Start >= next && sp.Stop <= len(src) && sp.Len() > 0 {
					if len(out) > 0 && out[len(out)-1].Stop == sp.Start {
						out[len(out)-1].Stop = sp.Stop
					} else {
						out = append(out, sp)
					}
					next = sp.Stop
				}
			}
			walk(c)
		}
	}
	walk(doc)
	return out
}
