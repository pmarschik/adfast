package markdown

import (
	"bytes"
	"slices"

	directive "github.com/pmarschik/goldmark-directive"
	gast "github.com/yuin/goldmark/ast"
)

// Autolink is one autolink of a Markdown source: where it is written, what
// it addresses, and whether the author wrote the angle brackets or the
// parser inferred the link from running text.
//
// Every span here is TIGHT, for the reason Link documents: an autolink is an
// INLINE, and a span widened to its line would name the sentence around it.
type Autolink struct {
	// Target is the URL the autolink resolves to, which is what a renderer
	// puts in the href. It is USUALLY the bytes Text covers, and differs
	// from them in exactly one case: a `www.` autolink has no scheme
	// written, and the parser completes it — "www.a.example" targets
	// "http://www.a.example". A caller rewriting the source rather than
	// rendering it must therefore compare Target against Text before it
	// assumes the written form is a whole URL. See Autolinks.
	Target string
	// Span covers the autolink as written: the `<` through one past the
	// `>` when the author wrote the brackets, and exactly the address when
	// the parser inferred the link. It equals Text for a bare autolink.
	Span Span
	// Text covers the address as written, angle brackets excluded. It is
	// never empty: neither form of autolink can hold an empty address.
	Text Span
	// Bare reports whether the autolink was inferred from running text
	// ("see https://a.example") rather than written in angle brackets
	// ("see <https://a.example>"). It is the distinction a normalizer
	// rewriting bare URLs into the bracketed form needs, because rewriting
	// the bracketed form again would double its brackets.
	//
	// goldmark's own node does not carry this: it reports one AutoLink kind
	// for both spellings. What answers it is an attribute this package's
	// autolink parser stamps on the bracketed form — see autolinkIsAngle —
	// which is inside the parse and not something a caller holding a tree
	// can reconstruct from the bytes around the node.
	Bare bool
	// Email reports whether the address is an email address rather than a
	// URL. An email autolink is bracketed the same way and reported the
	// same way; the flag is here because a caller's policy for the two
	// usually differs.
	Email bool
}

// Autolinks returns every autolink in src, in document order. See
// Source.Autolinks for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func Autolinks(src []byte) []Autolink { return NewSource(src).Autolinks() }

// Autolinks returns every autolink in the source, in document order.
//
// Both forms are reported, and Bare tells them apart: the ANGLE form the
// author wrote as `<https://a.example>`, and the BARE form the linkify
// extension inferred from running text. Links deliberately reports neither
// — an autolink's destination IS its text, so there is nothing to rewrite
// separately there — and this is the view for a caller that cares where an
// autolink is written rather than where it points.
//
// What counts as an autolink is goldmark's verdict, so it is the verdict the
// conversion path reaches, and that is the whole reason this view exists. A
// pattern over the raw source disagrees with it in both directions, and a
// bare-URL normalizer steered by one gets all of the following wrong:
//
//   - TRAILING PUNCTUATION. "https://a.example/x, and" addresses
//     "https://a.example/x"; the comma is the sentence's, not the URL's. A
//     character class that accepts `,` `.` `;` swallows it, and a rewriter
//     that wraps what the class matched has MOVED THE LINK.
//   - A LINK LABEL. No autolink is built inside `[…]`, so a URL written in
//     a link's text stays as the author wrote it. A pattern sees prose.
//   - A URL FLUSH AGAINST A TAG. "<span>https://a.example</span>" holds a
//     bare autolink; a pattern requiring whitespace before a URL misses it.
//   - CODE, HTML, AND DEFINITIONS. No autolink is built inside inline code,
//     a code block, an HTML block, inline HTML, or a link reference
//     definition's destination. A caller steering a pattern away from those
//     needs CodeSpans, InlineCodeSpans, HTMLSpans, InlineHTMLSpans and
//     Definitions to do it; a caller asking this view needs none of them,
//     because the parse already answered.
//
// TWO THINGS THE PARSER ANSWERS AND THIS VIEW PASSES THROUGH UNJUDGED, both
// of which a caller mirroring another Markdown implementation has to decide
// for itself:
//
//   - The SCHEME set. The parser is configured to the GFM set — `http://`,
//     `https://` and `www.` — so a bare `ftp://`, `mailto:`, `file://` or
//     `ssh://` is prose and no span for it reaches this view. It used to
//     linkify a bare `ftp://` and this note told a GFM-matching caller to
//     filter on the scheme it found in Text; that filter is no longer
//     needed. The angle form `<ftp://a.example>` is a different
//     construct and still an autolink, as it is in GFM.
//   - The `www.` form, where Target is not the written text. Wrapping such
//     an address in angle brackets would produce `<www.a.example>`, which
//     is a link to a relative path and not to the site. Comparing Target
//     against Text is the test, and it is why Target is reported at all.
//
// An autolink is reported only when its written form can be resolved back
// to the exact bytes goldmark read, and a mismatch DROPS it rather than
// reporting a span that would splice into the wrong place — the same
// resolve-or-drop rule Links and Definitions follow, for the same reason:
// under-reporting costs a rewriter a missed normalization, and a wrong
// offset costs it the document. UnlocatedAutolinks counts what was dropped,
// so a caller can tell an empty view from an incomplete one.
func (s *Source) Autolinks() []Autolink {
	if s.autolinksDone {
		return s.autolinks
	}
	s.autolinks, s.autolinksUnlocated = collectAutolinks(s.doc, s.src)
	s.autolinksDone = true
	return s.autolinks
}

// UnlocatedAutolinks returns how many autolinks of the source Autolinks
// could NOT resolve to a written extent, and therefore left out of its
// result. A caller can tell an empty view from an incomplete one with it.
//
// NO INPUT IS KNOWN TO MAKE IT NONZERO — a measurement over the inputs this
// package tests, and deliberately not phrased as a guarantee, because the
// same sentence was once written here as one and was FALSIFIED. It claimed
// soundness from a parser's trigger set: autolinkExtent decided an autolink's
// written form by sniffing the byte at Node.Pos for a '<', and the note said
// that was safe because no linkify parser triggers on '<'. Widening
// punctLinkifyBoundaries to the '<' the reference accepts — one constant, one
// file over — made "z<http://a.b c" build a correct AutoLink that this view
// then read as the angle form, found no closing '>' for, and dropped:
// Autolinks() empty, this count 1, and a tree that carried the link all
// along. The form is carried on the node now (see autolinkIsAngle) and that
// input resolves, but the lesson is that a zero here is a thing to re-measure
// and not to assume. TestBoundaryLiteralsAllResolveToASpan re-measures it for
// every boundary byte the parsers claim.
//
// One shape is known NOT to reach an autolink, which is worth stating because
// a DEFINITION's resolver has to discount it: a container prefix ending in a
// partly consumed tab, whose leftover columns the parser pads with spaces
// that stand for no byte. An autolink is an inline of a paragraph or a
// heading, and those trim each line's leading whitespace, padding included,
// before the inline pass reads them. Every tab-prefixed shape measured for
// this view resolved.
//
// The count is the honest half of resolve-or-drop, and what it reports is the
// three gates: the address bytes must be spelled at the offset the parser
// recorded, an ANGLE form's brackets must be spelled around them, and a text
// directive's label must be spelled where its own extent says it is. None of
// the three is a gate this package can prove unreachable from outside
// goldmark, so a caller that must distinguish an empty view from an
// incomplete one reads this rather than trusting the paragraph above.
func (s *Source) UnlocatedAutolinks() int {
	s.Autolinks()
	return s.autolinksUnlocated
}

// collectAutolinks walks doc for autolinks and resolves each to its written
// extent. It reports the autolinks it located and how many it did not.
//
// Like collectInlineCodeSpans this descends into inline subtrees. Unlike it,
// it also descends into a TEXT DIRECTIVE'S LABEL, which is the one subtree
// blockNodes warns about: goldmark cannot re-enter an arbitrary range, so
// the directive parser parses a label against a DETACHED COPY of its bytes,
// and the offsets under that root address the copy and not this source.
// Every other view skips those roots for exactly that reason.
//
// This view descends anyway, because a bare URL written in a label is an
// autolink like any other and a caller that skipped it would leave the one
// URL of the document untouched. What makes it safe is that the label's
// offset INTO THIS SOURCE is recovered and byte-verified first (see
// textDirectiveLabel), and the recursion then runs in the copy's own
// coordinates and is shifted by that one offset. A label that does not
// verify contributes its autolinks to the unlocated count instead — the
// same resolve-or-drop rule, one level up.
func collectAutolinks(doc gast.Node, src []byte) (out []Autolink, unlocated int) {
	var walk func(gast.Node)
	walk = func(n gast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if al, ok := c.(*gast.AutoLink); ok {
				if got, ok := autolinkExtent(al, src); ok {
					out = append(out, got)
				} else {
					unlocated++
				}
			}
			if td, ok := c.(*directive.TextDirective); ok && td.LabelRoot != nil {
				inner, u := collectAutolinks(td.LabelRoot, td.LabelSource)
				unlocated += u
				if at, ok := textDirectiveLabel(src, td); ok {
					for _, a := range inner {
						out = append(out, shiftAutolink(a, at))
					}
				} else {
					unlocated += len(inner)
				}
			}
			walk(c)
		}
	}
	walk(doc)
	slices.SortFunc(out, func(a, b Autolink) int { return a.Span.Start - b.Span.Start })
	return out, unlocated
}

// textDirectiveLabel reports where a text directive's label content begins
// in src, and whether that offset could be verified.
//
// The directive node carries its own extent in THIS source (Span, the whole
// `:name[label]{attrs}` run) and the label's bytes as a separate copy
// (LabelSource). A directive name is alphanumeric with `-`/`_` runs and the
// attribute block follows the label, so the FIRST `[` of the run is the
// label's opening bracket and nothing else can be. The offset is accepted
// only when src spells the label's bytes just after that bracket and closes
// them with `]` — the same rule autolinkExtent applies to an address, for
// the same reason: a label copy that does not match the source it came from
// is a copy the parser padded, and shifting by its offset would move every
// span under it.
func textDirectiveLabel(src []byte, td *directive.TextDirective) (int, bool) {
	span := td.Span
	if span.Start < 0 || span.Stop > len(src) || span.Start >= span.Stop {
		return 0, false
	}
	open := bytes.IndexByte(src[span.Start:span.Stop], '[')
	if open < 0 {
		return 0, false
	}
	start := span.Start + open + 1
	stop := start + len(td.LabelSource)
	if stop >= len(src) || !bytes.Equal(src[start:stop], td.LabelSource) || src[stop] != ']' {
		return 0, false
	}
	return start, true
}

// shiftAutolink moves an autolink resolved against a detached label copy
// into the coordinates of the source that copy was taken from.
func shiftAutolink(a Autolink, by int) Autolink {
	a.Span = Span{Start: a.Span.Start + by, Stop: a.Span.Stop + by}
	a.Text = Span{Start: a.Text.Start + by, Stop: a.Text.Stop + by}
	return a
}

// autolinkIsAngle reports whether n is the ANGLE form — written
// `<https://a.example>` — rather than a literal the linkify parsers inferred
// from running text.
//
// THE FORM IS CARRIED, NOT SNIFFED, and that is the whole point of this
// helper. angleAutoLinkParser wraps goldmark's core autolink parser — the
// only parser in this package that accepts the bracketed spelling — and
// stamps the node it produces with the `angleAutoLink` attribute. Presence is
// the answer, which is how convertGoldmarkAutoLink already reads it, so the
// span view and the AST conversion agree on the form by construction.
//
// WHAT IT REPLACED, because the replacement is the fix and not a tidy-up:
// this used to be `src[Node.Pos()] == '<'`, justified by no linkify parser
// triggering on `<`. That made the view's correctness a hostage of a
// PARSER'S TRIGGER SET one file over. Widening punctLinkifyBoundaries to the
// `<` the reference accepts built a correct AutoLink at a Pos pointing at
// that `<` with no `>` behind the address, so the sniff called it the angle
// form, the `>` check failed, and the node was DROPPED: the tree answered
// "there is a link here" and this view answered "there is none". Measured
// with `<` in the set and the sniff still in place, `z<http://a.b c` gave
// Autolinks() == [] and UnlocatedAutolinks() == 1.
func autolinkIsAngle(n *gast.AutoLink) bool {
	_, angle := n.AttributeString("angleAutoLink")
	return angle
}

// autolinkExtent resolves one autolink to its written extent, in the form
// autolinkIsAngle reports for it.
//
// THE PROBLEM. goldmark keeps an autolink's source segment in an unexported
// field, and exposes only its VALUE (AutoLink.Label) — so the address is
// readable and the offset it was read from is not. What is left is
// Node.Pos, which the inline parser sets to the offset it began reading at,
// and that offset is one BEFORE the address for both forms:
//
//	See <https://a.example>   Pos is the `<`
//	See https://a.example     Pos is the space
//
// So Pos alone distinguishes nothing, which is why a caller cannot do this
// itself and why the view exists.
//
// THE RESOLUTION. Pos is one before the address, or the address itself. The
// second case is the linkify parser being invoked at the head of a fragment
// — at the start of a line, and after every inline the reader just closed,
// which is what puts a bare URL flush against a `</span>` in this view at
// all. So the address begins at Pos or at Pos+1, and the candidate is
// accepted only when the source SPELLS THE ADDRESS THERE. Nothing is
// assumed: an offset that does not verify drops the node.
//
// The form narrows that to one offset each. The ANGLE form's address is at
// Pos+1 by construction — Pos is the bracket — so both brackets are
// verified, and the BARE form is the two-offset case above. The bracket
// check is VERIFICATION of a span this function is about to claim covers a
// `<`, not the derivation of the form: a bare literal whose boundary byte
// happens to be `<`, which is what "z<http://a.b" is, reaches the bare
// branch and resolves to the address alone.
func autolinkExtent(n *gast.AutoLink, src []byte) (Autolink, bool) {
	pos := n.Pos()
	if pos < 0 || pos > len(src) {
		return Autolink{}, false
	}
	addr := n.Label(src)
	if len(addr) == 0 {
		return Autolink{}, false
	}
	out := Autolink{
		Target: string(n.URL(src)),
		Email:  n.AutoLinkType == gast.AutoLinkEmail,
	}
	if autolinkIsAngle(n) {
		if pos >= len(src) || src[pos] != '<' {
			return Autolink{}, false
		}
		text, ok := addressAt(src, pos+1, addr)
		if !ok || text.Stop >= len(src) || src[text.Stop] != '>' {
			return Autolink{}, false
		}
		out.Span, out.Text = Span{Start: pos, Stop: text.Stop + 1}, text
		return out, true
	}
	text, ok := addressAt(src, pos, addr)
	if !ok {
		if text, ok = addressAt(src, pos+1, addr); !ok {
			return Autolink{}, false
		}
	}
	out.Span, out.Text, out.Bare = text, text, true
	return out, true
}

// addressAt reports the span addr occupies at off, and whether the source
// really spells it there.
func addressAt(src []byte, off int, addr []byte) (Span, bool) {
	stop := off + len(addr)
	if off < 0 || stop > len(src) {
		return Span{}, false
	}
	if !bytes.Equal(src[off:stop], addr) {
		return Span{}, false
	}
	return Span{Start: off, Stop: stop}, true
}
