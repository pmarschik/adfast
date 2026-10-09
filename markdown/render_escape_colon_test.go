package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// colonSlashCase is one text body with the byte each render mode writes for
// its colon. The bodies are built as an ast.Text rather than parsed, so no
// parse step can reinterpret the string before the escape table sees it —
// the same shape the reference measurement used (a hand-built mdast text
// node handed straight to mdast-util-to-markdown).
type colonSlashCase struct {
	name string
	// in is the paragraph's whole text.
	in string
	// remark is what remark mode writes. It is the reference's own measured
	// answer on every row but the two uppercase ones, which are this
	// package's one deliberate divergence from the unsafe table and say so
	// in place.
	remark string
	// prettier is what prettier mode writes: prettier 3.9.6's own answer,
	// measured with the flags TestFormatMarkdown_PrettierParity pins.
	prettier string
}

// THE REFERENCE ESCAPES A COLON BETWEEN 'p'-OR-'s' AND A SLASH, and its rule
// is far cruder than the scheme-shaped thing it is named for. Its GFM
// autolink-literal extension contributes one unsafe row —
//
//	{character: ':', before: '[ps]', after: '\/', inConstruct: 'phrasing',
//	 notInConstruct: ['autolink', 'link', 'image', 'label']}
//
// — so a single letter is enough and a single slash is enough. Every remark
// column below is that rule's measured output, WITH ONE DELIBERATE
// DIVERGENCE: the reference applies `[ps]` case-sensitively and this package
// folds the byte, so the two uppercase rows below carry an escape the
// reference does not write. They are the only two rows in the repo that
// differ, and colonBeforeSlashEscapes records why the fold is closer to the
// table's intent than the table's own spelling. Every prettier column is
// prettier's, which writes none of these escapes and merely preserves an
// authored one.
var colonSlashCases = []colonSlashCase{{
	// THE ROW THE RULE IS NAMED FOR: a scheme with no host.
	name:     "a schemed colon before two slashes",
	in:       "see https:// bare",
	remark:   "see https\\:// bare\n",
	prettier: "see https:// bare\n",
}, {
	name:     "the http scheme too",
	in:       "see http:// bare",
	remark:   "see http\\:// bare\n",
	prettier: "see http:// bare\n",
}, {
	// ONE SLASH IS ENOUGH. The unsafe row's `after` is `\/`, not `\/\/`, so
	// a rule spelled "a scheme, then //" writes nothing here and diverges.
	name:     "one slash is enough",
	in:       "see https:/one bare",
	remark:   "see https\\:/one bare\n",
	prettier: "see https:/one bare\n",
}, {
	// NOT A SCHEME AT ALL. `before` is one character, so any word ending in
	// 'p' or 's' triggers it — "ftp" ends in 'p' and so does "sip".
	name:     "a non-scheme word ending in p",
	in:       "a sip:/foo b",
	remark:   "a sip\\:/foo b\n",
	prettier: "a sip:/foo b\n",
}, {
	name:     "a non-scheme word ending in s",
	in:       "a oops:/x b",
	remark:   "a oops\\:/x b\n",
	prettier: "a oops:/x b\n",
}, {
	name:     "the ftp scheme ends in p",
	in:       "a ftp:// b",
	remark:   "a ftp\\:// b\n",
	prettier: "a ftp:// b\n",
}, {
	// ONE LETTER IS ENOUGH, which is the clearest proof that the rule is a
	// character class and not a scheme test.
	name:     "a lone s before the slash",
	in:       "a s:/ d",
	remark:   "a s\\:/ d\n",
	prettier: "a s:/ d\n",
}, {
	name:     "a lone p before the slash",
	in:       "a p:/ d",
	remark:   "a p\\:/ d\n",
	prettier: "a p:/ d\n",
}, {
	// THE TWO ROWS THIS PACKAGE DELIBERATELY DIVERGES ON, and the only two
	// in the repo the divergence costs. The reference applies `[ps]` case
	// -sensitively and writes nothing for the uppercase pair; adfast folds
	// the byte and writes the escape, because keeping the reference's
	// literal spelling lost a link mark on the ADF round trip — an
	// uppercase final scheme letter went out bare and the next parse
	// linkified plain text ("_httP://0_" came back as "_<httP://0>_").
	// The reference is stable on the LOWERCASE spelling of the identical
	// tree, so its own escape works and it merely fails to write it; see
	// colonBeforeSlashEscapes for that measurement.
	//
	// These two colons could never have linkified anything, so the two
	// bytes below are the whole price. What they pin is that the price is
	// paid HERE and nowhere wider: the fold must still reject every
	// negative row that follows.
	name:     "an uppercase S is escaped too, diverging from the reference",
	in:       "a S:/ d",
	remark:   "a S\\:/ d\n",
	prettier: "a S:/ d\n",
}, {
	name:     "an uppercase P is escaped too, diverging from the reference",
	in:       "a P:/ d",
	remark:   "a P\\:/ d\n",
	prettier: "a P:/ d\n",
}, {
	// THE NEGATIVES, which the fold must leave alone: it widens `[ps]` by
	// exactly the uppercase pair and by nothing else.
	name:     "a scheme not ending in p or s writes nothing",
	in:       "a file:// b",
	remark:   "a file:// b\n",
	prettier: "a file:// b\n",
}, {
	name:     "another letter before the slash writes nothing",
	in:       "a xyz:/c d",
	remark:   "a xyz:/c d\n",
	prettier: "a xyz:/c d\n",
}, {
	// THE FOLD'S OWN NEGATIVE: an uppercase letter outside the pair is
	// still nothing. Both rules agree here without a measurement of their
	// own — case-sensitively 'E' is not in `[ps]`, and folded 'e' is not
	// either — which is what makes this the row that catches a fold gone
	// wide (a fold to uppercase, or a comparison against the wrong pair,
	// changes some row above but a fold that widens the class changes
	// this one).
	name:     "an uppercase letter outside the pair writes nothing",
	in:       "a FILE:// b",
	remark:   "a FILE:// b\n",
	prettier: "a FILE:// b\n",
}, {
	// NO SLASH, NO RULE — and the row that proves the two colon rules stay
	// separate. remark and prettier both write a backslash here, but for
	// the DIRECTIVE grammar (a ':' before an ASCII letter), which is
	// adfast's own deliberate divergence: the reference leaves this colon
	// bare (measured, "see https:x bare" comes back unchanged). See
	// escapesColon.
	name:     "a colon before a letter is the directive rule, not this one",
	in:       "see https:x bare",
	remark:   "see https\\:x bare\n",
	prettier: "see https\\:x bare\n",
}, {
	// A BARE SCHEME COLON WITH NOTHING AFTER IT is escaped by neither side,
	// which is what keeps this from being "the reference escapes colons".
	name:     "a host-less scheme colon at the end of a word",
	in:       "scheme mailto: alone",
	remark:   "scheme mailto: alone\n",
	prettier: "scheme mailto: alone\n",
}}

func TestRender_ColonBeforeSlashFollowsTheReferenceUnsafeRule(t *testing.T) {
	t.Parallel()
	for _, c := range colonSlashCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := &ast.Root{Children: []ast.Node{
				&ast.Paragraph{Children: []ast.Node{&ast.Text{Value: c.in}}},
			}}
			if got := Render(root); got != c.remark {
				t.Errorf("remark mode: Render(%q) = %q, want %q", c.in, got, c.remark)
			}
			if got := Render(root, WithPrettierText()); got != c.prettier {
				t.Errorf("prettier mode: Render(%q) = %q, want %q", c.in, got, c.prettier)
			}
		})
	}
}

// The unsafe row is keyed on 'phrasing' and excluded from 'label', and those
// two words decide four constructs. A TABLE CELL is phrasing, so the escape
// belongs there even though adfast's own directive colon rule stands down in
// a cell (inlineContext.colons is false) — which is why the rule is checked
// before that flag. A LINK LABEL is the one place the rule does not reach.
// All four measured against the frozen reference, rendering a hand-built
// mdast for each construct.
func TestRender_ColonBeforeSlashPerConstruct(t *testing.T) {
	t.Parallel()
	text := func() []ast.Node {
		return []ast.Node{&ast.Text{Value: "see https:// bare"}}
	}
	cases := []struct {
		name string
		root ast.Node
		want string
	}{{
		name: "a table cell escapes it, where the directive rule stands down",
		root: &ast.Root{Children: []ast.Node{&ast.Table{Children: []ast.Node{
			&ast.TableRow{Children: []ast.Node{
				&ast.TableCell{Children: []ast.Node{&ast.Text{Value: "h"}}},
			}},
			&ast.TableRow{Children: []ast.Node{&ast.TableCell{Children: text()}}},
		}}}},
		want: "| h                  |\n| ------------------ |\n| see https\\:// bare |\n",
	}, {
		name: "emphasis escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Emphasis{Children: text()},
		}}}},
		want: "_see https\\:// bare_\n",
	}, {
		name: "a heading escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Heading{Depth: 2, Children: text()}}},
		want: "## see https\\:// bare\n",
	}, {
		name: "a link label does not",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Link{URL: "u", Children: text()},
		}}}},
		want: "[see https:// bare](u)\n",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Render(c.root); got != c.want {
				t.Errorf("Render() = %q, want %q", got, c.want)
			}
		})
	}
}

// The point of the escape is that the byte survives a round trip. remark
// mode used to DROP an authored "https\://" — it decoded the escape and then
// had no rule to write it back — so a document the reference had formatted
// lost the backslash on the next remark pass, and the reference put it back
// on the pass after that. Both spellings now settle on the reference's.
//
// REMARK MODE ONLY, and not because prettier mode is untested: prettier's
// authored-escape preservation runs off ast.Text.Raw, which the prettier
// FORMATTER swaps in and a bare Render does not read, so a Parse→Render pair
// cannot exercise it here. What prettier mode WRITES for a bare colon is the
// prettier column of colonSlashCases above.
func TestRender_ColonBeforeSlashRoundTripsStablyInRemarkMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want string
	}{{
		name: "an authored escape is written back",
		src:  "see https\\:// bare\n",
		want: "see https\\:// bare\n",
	}, {
		name: "a bare colon gains the escape",
		src:  "see https:// bare\n",
		want: "see https\\:// bare\n",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Render(Parse([]byte(c.src)))
			if got != c.want {
				t.Errorf("%q -> %q, want %q", c.src, got, c.want)
			}
			if again := Render(Parse([]byte(got))); again != got {
				t.Errorf("not idempotent: %q -> %q", got, again)
			}
		})
	}
}
