package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/markdown"
)

// linkVerdicts lists every link the parse of src produced, in document
// order, as "text|url". The whole defect this file guards is a VERDICT
// that moves — a link appearing or vanishing — so the assertion is the
// list of links and nothing else: the surrounding text nodes are free to
// differ between an escaped and an unescaped spelling, and they do.
func linkVerdicts(src string) []string {
	out := []string{}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if l, ok := n.(*ast.Link); ok {
			out = append(out, ast.PlainText(l.Children)+"|"+l.URL)
		}
		if p, ok := n.(interface{ ChildNodes() []ast.Node }); ok {
			for _, c := range p.ChildNodes() {
				walk(c)
			}
		}
	}
	walk(markdown.Parse([]byte(src)))
	return out
}

// escapePairCase is one address written twice: once with a backslash
// escape in front of the boundary character and once without. Both
// spellings must produce want.
type escapePairCase struct {
	name string
	// escaped and literal differ by exactly one backslash.
	escaped, literal string
	// want is the link list both spellings must produce. Every case
	// carries a second, unescaped address that linked before this file
	// existed and still does, so a blanket "no links at all" or "link
	// everything" implementation cannot pass.
	want []string
}

// AN ESCAPE MUST NOT DECIDE WHETHER A URL IS A LINK. Before the
// post-escape linkify parser it did, in both directions, so the md → md
// formatter changed meaning whichever way it moved an escape:
//
//	"a\_http://0"   parsed as plain text, and the formatter DROPS the
//	                "\_" ('_' is outside PreservedEscapes), so the
//	                re-parse of "a_http://0" linkified: a link was
//	                INVENTED.
//	"0*httP://0"    parsed WITH the link, and the formatter ADDS the
//	                "\*", so the re-parse of "0\*httP://0" saw none: a
//	                link was DELETED.
//
// The reference implementation links all four spellings identically —
// micromark's gfm-autolink-literal tokenizer reads the RAW SOURCE, where
// a character escape in front of a literal does not stop it — so the
// escaped column here is also the parity answer, not just an internal
// consistency rule.
//
// WHICH ROWS MOVED, measured against the previous parser: the DOTLESS
// hosts ("http://0") and the email cases below. The DOTTED rows
// ("http://c.d", "www.c.d") already linked, and are PINS: the
// decoded-text rescue scan in goldmark_to_ast.go reaches them, because
// its recognizer demands a dotted host and its boundary set carries
// these characters. That scan is also why the fix could not live there —
// it cannot widen to a dotless host without breaking the split the two
// recognizers are pinned to, it cannot tell "a\_x" from "a_x" ('_' is
// outside PreservedEscapes, so the raw spelling is already gone), and it
// only ever repairs the TREE, never the spans the next test asserts.
var escapePairCases = []escapePairCase{{
	// The first live instance, in the direction that INVENTED a link.
	name:    "underscore boundary, dotless host",
	escaped: "ok http://a.b and a\\_http://0\n",
	literal: "ok http://a.b and a_http://0\n",
	want:    []string{"http://a.b|http://a.b", "http://0|http://0"},
}, {
	// The second live instance, in the direction that DELETED one. The
	// uppercase 'P' is load-bearing: it is what makes the formatter
	// re-render the '*' escaped.
	name:    "asterisk boundary, mixed-case scheme",
	escaped: "ok http://a.b and 0\\*httP://0\n",
	literal: "ok http://a.b and 0*httP://0\n",
	want:    []string{"http://a.b|http://a.b", "httP://0|httP://0"},
}, {
	name:    "tilde boundary",
	escaped: "ok http://a.b and a\\~http://c.d\n",
	literal: "ok http://a.b and a~http://c.d\n",
	want:    []string{"http://a.b|http://a.b", "http://c.d|http://c.d"},
}, {
	// '(' is in goldmark's trigger set and is inside PreservedEscapes,
	// so unlike '_' and '*' this escape survives the render — the two
	// spellings reach the parser as different bytes for good, and the
	// verdict still has to match.
	name:    "open-paren boundary",
	escaped: "ok http://a.b and a\\(http://c.d\n",
	literal: "ok http://a.b and a(http://c.d\n",
	want:    []string{"http://a.b|http://a.b", "http://c.d|http://c.d"},
}, {
	// The row above uses a DOTTED host, and a dotted host is rescued by
	// relinkifyTexts whether or not '(' is a boundary byte — so it
	// passes with '(' deleted from escapedLinkifyBoundaries and cannot
	// pin that byte. This one uses a dotless host, which only this
	// parser reaches: delete '(' from the set and this row alone fails.
	// Every other byte in the set already had a dotless row through one
	// test or another; '(' did not.
	name:    "open-paren boundary, dotless host",
	escaped: "ok http://a.b and a\\(http://0\n",
	literal: "ok http://a.b and a(http://0\n",
	want:    []string{"http://a.b|http://a.b", "http://0|http://0"},
}, {
	// The escape at the very start of the paragraph: there is no
	// preceding text node for the boundary character to merge into.
	name:    "escape at offset zero",
	escaped: "\\_http://0 and http://a.b\n",
	literal: "_http://0 and http://a.b\n",
	want:    []string{"http://0|http://0", "http://a.b|http://a.b"},
}, {
	// The scheme-less "www." branch of the same parser.
	name:    "www literal after an escape",
	escaped: "ok http://a.b and a\\_www.c.d\n",
	literal: "ok http://a.b and a_www.c.d\n",
	want:    []string{"http://a.b|http://a.b", "www.c.d|http://www.c.d"},
}, {
	// The escape follows an inline that already closed, so the
	// backslash is the first byte of its own text run.
	name:    "escape after a closing emphasis marker",
	escaped: "**b**\\_http://0 and http://a.b\n",
	literal: "**b**_http://0 and http://a.b\n",
	want:    []string{"http://0|http://0", "http://a.b|http://a.b"},
}}

func TestEscapedLinkify_AnEscapeDoesNotDecideTheLinkVerdict(t *testing.T) {
	for _, tc := range escapePairCases {
		t.Run(tc.name, func(t *testing.T) {
			lit := linkVerdicts(tc.literal)
			if strings.Join(lit, ";") != strings.Join(tc.want, ";") {
				t.Errorf("literal %q linked %v, want %v", tc.literal, lit, tc.want)
			}
			esc := linkVerdicts(tc.escaped)
			if strings.Join(esc, ";") != strings.Join(tc.want, ";") {
				t.Errorf("escaped %q linked %v, want %v", tc.escaped, esc, tc.want)
			}
		})
	}
}

// TestEscapedLinkify_ReportsThePostEscapeAutolinkSpan is the half of the
// fix that a tree-level repair could not have delivered. Source.Autolinks
// reports the PARSER's verdict against raw offsets and nothing else, so
// before the post-escape parser existed every caller driving off spans —
// rather than off the tree — saw no link here at all. The recorded offset
// has to be the address itself: autolinkExtent accepts only the address's
// own offset or the one byte before it and re-reads the source there, so
// a node still carrying the BACKSLASH's offset is dropped from this view
// even though the tree holds the link.
func TestEscapedLinkify_ReportsThePostEscapeAutolinkSpan(t *testing.T) {
	tests := []autolinkCase{{
		name: "a bare autolink after an escaped underscore",
		src:  "See a\\_http://a.example here\n",
		want: []string{"http://a.example|http://a.example|http://a.example|bare|url"},
	}, {
		name: "a www autolink after an escaped underscore",
		src:  "See a\\_www.a.example here\n",
		want: []string{"www.a.example|www.a.example|http://www.a.example|bare|url"},
	}, {
		// The GOOD case in the same document: an unescaped address that
		// was always reported, alongside one that was not.
		name: "both spellings in one paragraph, in document order",
		src:  "See http://a.example and b\\_http://c.example.\n",
		want: []string{
			"http://a.example|http://a.example|http://a.example|bare|url",
			"http://c.example|http://c.example|http://c.example|bare|url",
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := autolinkTexts(tc.src, markdown.NewSource([]byte(tc.src)).Autolinks())
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("autolinks of %q:\ngot  %v\nwant %v", tc.src, got, tc.want)
			}
		})
	}
}

// TestEscapedLinkify_MatchesTheLiteralSpelling is the invariant this file
// exists for: an escape may only leave the verdict where the LITERAL
// character puts it. Every row is asserted in BOTH spellings, and the two
// must agree — that is the property, not any particular verdict.
//
// THE VERDICT ITSELF MOVED when the boundary set widened. Rows one and two
// used to record a known divergence: this package took goldmark's five-byte
// trigger set as the whole left-boundary rule and left "x.http://c.d" and
// "x!http://c.d" as prose, escaped or not, where the reference links them.
// punctLinkifyParser closes that, and escapedLinkifyBoundaries is derived
// from the same set so the escaped spelling moved with it rather than after
// it. Rows three and four are unchanged and are the good cases: a boundary
// this package still does not claim, and an escape with no address behind
// it, both of which must stay prose in both spellings.
func TestEscapedLinkify_MatchesTheLiteralSpelling(t *testing.T) {
	tests := []escapePairCase{{
		name:    "dot boundary links in both spellings",
		escaped: "ok http://a.b and x\\.http://c.d\n",
		literal: "ok http://a.b and x.http://c.d\n",
		want:    []string{"http://a.b|http://a.b", "http://c.d|http://c.d"},
	}, {
		name:    "bang boundary links in both spellings",
		escaped: "ok http://a.b and x\\!http://c.d\n",
		literal: "ok http://a.b and x!http://c.d\n",
		want:    []string{"http://a.b|http://a.b", "http://c.d|http://c.d"},
	}, {
		// A backslash before a NON-boundary byte must not turn into a
		// boundary either: the escape is inert and the address stays
		// fused to the "x".
		//
		// The LITERAL spelling here is also a standing divergence, named
		// in punctLinkifyBoundaries: "x\\http://c.d" holds a real
		// backslash character, the reference reads it as punctuation and
		// links, and this package cannot claim that byte because
		// escapedLinkifyParser owns it.
		name:    "escaped backslash boundary links in neither spelling",
		escaped: "ok http://a.b and x\\\\http://c.d\n",
		literal: "ok http://a.b and x\\http://c.d\n",
		want:    []string{"http://a.b|http://a.b"},
	}, {
		// The escape is a lone trailing backslash with no URL behind
		// it; the parser must decline and leave the escape alone.
		name:    "escaped boundary with no address behind it",
		escaped: "ok http://a.b and x\\_y\n",
		literal: "ok http://a.b and x_y\n",
		want:    []string{"http://a.b|http://a.b"},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{tc.literal, tc.escaped} {
				got := linkVerdicts(src)
				if strings.Join(got, ";") != strings.Join(tc.want, ";") {
					t.Errorf("%q linked %v, want %v", src, got, tc.want)
				}
			}
		})
	}
}

// TestEscapedLinkify_EmailAfterAnEscape covers the email branch of the
// same delegated parser. Reaching a URL after an escape necessarily
// reaches an address after one, because acceptance is goldmark's whole
// linkify parser rather than a re-spelled copy of its URL half — keeping
// the two in one implementation is what stops the scheme set, the host
// rule and the trailing-punctuation trim from drifting apart.
//
// The second row is a KNOWN divergence, pinned rather than fixed: the
// reference reads the local part from the raw source and keeps the '_',
// while goldmark's linkify is handed the text from the boundary
// character onward and truncates to "x@a.b". It is stable across the
// format on both sides, so it moves no meaning.
func TestEscapedLinkify_EmailAfterAnEscape(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{{
		name: "tilde boundary matches the reference",
		src:  "ok http://a.b and c\\~x@d.e\n",
		want: []string{"http://a.b|http://a.b", "x@d.e|mailto:x@d.e"},
	}, {
		name: "underscore boundary truncates the local part",
		src:  "ok http://a.b and c\\_x@d.e\n",
		want: []string{"http://a.b|http://a.b", "x@d.e|mailto:x@d.e"},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := linkVerdicts(tc.src)
			if strings.Join(got, ";") != strings.Join(tc.want, ";") {
				t.Errorf("%q linked %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

// TestEscapedLinkify_VerdictSurvivesARender closes the loop the defect
// was reported through: the formatter is parse → render → parse, and a
// verdict that only holds on the first parse is not a fix. Every row
// renders back and re-parses to the SAME link list.
//
// The stability half of the assertion earns its place separately. Under
// the previous parser "a_http://0" rendered to "a\_http://0", which
// re-parsed as prose and rendered AGAIN to "a\_http\://0" — the colon
// escape a non-link gets. Two renders, two different documents.
func TestEscapedLinkify_VerdictSurvivesARender(t *testing.T) {
	for _, tc := range escapePairCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{tc.literal, tc.escaped} {
				once := markdown.Render(markdown.Parse([]byte(src)))
				got := linkVerdicts(once)
				if strings.Join(got, ";") != strings.Join(tc.want, ";") {
					t.Errorf("%q rendered to %q, which linked %v, want %v",
						src, once, got, tc.want)
				}
				if twice := markdown.Render(markdown.Parse([]byte(once))); twice != once {
					t.Errorf("render of %q is not stable:\nonce:  %q\ntwice: %q",
						src, once, twice)
				}
			}
		})
	}
}
