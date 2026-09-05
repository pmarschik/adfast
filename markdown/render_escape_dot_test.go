package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
)

// dotAfterWCase is one text body with the byte each render mode writes for
// its dot. The bodies are built as an ast.Text rather than parsed, so no
// parse step can reinterpret the string before the escape table sees it —
// the same shape the reference measurement used (a hand-built mdast text
// node handed straight to mdast-util-to-markdown).
type dotAfterWCase struct {
	name string
	// in is the paragraph's whole text.
	in string
	// remark is what remark mode writes: the reference's own answer, which
	// is where every row's want comes from.
	remark string
	// prettier is what prettier mode writes: prettier's own answer, which
	// for every row below is the text unchanged.
	prettier string
}

// THE REFERENCE ESCAPES A DOT BETWEEN A 'w'-OR-'W' AND A WORD CHARACTER, and
// its rule is far cruder than the "www." host it is named for. Its GFM
// autolink-literal extension contributes one unsafe row —
//
//	{character: '.', before: '[Ww]', after: '[\\-.\\w]', inConstruct: 'phrasing',
//	 notInConstruct: ['autolink', 'link', 'image', 'label']}
//
// — so a single 'w' is enough, either case is enough, nothing has to look
// like a host, and a dot in ordinary prose after any word ending in 'w' is
// caught. Every remark column below is that rule's measured output; every
// prettier column is prettier's, which writes none of these escapes.
//
// BOTH ENDS OF BOTH CLASSES ARE FENCED HERE. The `before` class has its
// positives ('w', 'W') and its negatives ('v', a digit, a '-', a '_', no
// previous byte at all); the `after` class has its positives (a letter of
// either case, a digit, a '-', another '.') and its negatives (a space, a
// '!', a '#', a '+', a ':', a quote, a paren, a '/', the end of the text).
// A rule that fenced only the first character of either class would pass a
// positive row and fail one of these.
var dotAfterWCases = []dotAfterWCase{{
	// THE ROW THE RULE IS NAMED FOR: a host that would linkify.
	name:     "a dot inside a www host",
	in:       "see www.x b",
	remark:   "see www\\.x b\n",
	prettier: "see www.x b\n",
}, {
	// ONE 'w' IS ENOUGH, which is the clearest proof that the rule is a
	// character class and not a host test.
	name:     "one w is enough",
	in:       "see w.x b",
	remark:   "see w\\.x b\n",
	prettier: "see w.x b\n",
}, {
	// EITHER CASE, unlike the sibling ':' row whose `[ps]` is lowercase only.
	name:     "an uppercase W too",
	in:       "see W.x b",
	remark:   "see W\\.x b\n",
	prettier: "see W.x b\n",
}, {
	name:     "an uppercase W and an uppercase after",
	in:       "see W.X b",
	remark:   "see W\\.X b\n",
	prettier: "see W.X b\n",
}, {
	// THE RUN NEED NOT START THE WORD, so ordinary prose is caught.
	name:     "a w that does not start the word",
	in:       "see xw.x b",
	remark:   "see xw\\.x b\n",
	prettier: "see xw.x b\n",
}, {
	name:     "a word merely ending in w",
	in:       "see wow.x b",
	remark:   "see wow\\.x b\n",
	prettier: "see wow.x b\n",
}, {
	// AFTER: A DIGIT.
	name:     "a digit after the dot",
	in:       "see www.0 b",
	remark:   "see www\\.0 b\n",
	prettier: "see www.0 b\n",
}, {
	// AFTER: A HYPHEN. It is in the class as `\-`, and it is NOT canceled
	// the way '_' is (see dotAfterWwwEscapes): remark's own '-' rule is
	// atBreak-conditional, so a '-' mid-line is not an escaped position.
	name:     "a hyphen after the dot",
	in:       "see www.- b",
	remark:   "see www\\.- b\n",
	prettier: "see www.- b\n",
}, {
	// AFTER: ANOTHER DOT. Only the first dot of the pair is escaped, because
	// the second one's `before` is a '.' and not a 'w'.
	name:     "another dot after the dot",
	in:       "see w.. b",
	remark:   "see w\\.. b\n",
	prettier: "see w.. b\n",
}, {
	name:     "a dot run that continues into a word",
	in:       "see w..x b",
	remark:   "see w\\..x b\n",
	prettier: "see w..x b\n",
}, {
	// ORDINARY PROSE, AND THE GOOD CASE IN THE SAME DOCUMENT: the first dot
	// is escaped because 'W' precedes it, the second is not because 'C'
	// does. One string that pins the rule firing and standing down at once.
	name:     "initials, where only the first dot follows a W",
	in:       "W.C. Fields",
	remark:   "W\\.C. Fields\n",
	prettier: "W.C. Fields\n",
}, {
	// The prose shape the corpus actually contains.
	name:     "an abbreviation ending in w",
	in:       "w.r.t. the plan",
	remark:   "w\\.r.t. the plan\n",
	prettier: "w.r.t. the plan\n",
}, {
	// TWO DOTS, ONE RULED IN AND ONE RULED OUT, in one paragraph.
	name:     "a caught dot beside an uncaught one",
	in:       "see w.x and b.c d",
	remark:   "see w\\.x and b.c d\n",
	prettier: "see w.x and b.c d\n",
}, {
	name:     "a caught host beside an uncaught letter",
	in:       "see www.x and see v.y b",
	remark:   "see www\\.x and see v.y b\n",
	prettier: "see www.x and see v.y b\n",
}, {
	// BEFORE, NEGATIVE: 'v' is next to 'w' in the alphabet and not in the
	// class.
	name:     "a v before the dot writes nothing",
	in:       "see v.x b",
	remark:   "see v.x b\n",
	prettier: "see v.x b\n",
}, {
	name:     "an ordinary word before the dot writes nothing",
	in:       "a b.c d",
	remark:   "a b.c d\n",
	prettier: "a b.c d\n",
}, {
	name:     "a digit before the dot writes nothing",
	in:       "see 9.x b",
	remark:   "see 9.x b\n",
	prettier: "see 9.x b\n",
}, {
	// BEFORE, NEGATIVE: a '-' is in the AFTER class and not in the before
	// one, which is the pair most easily conflated.
	name:     "a hyphen before the dot writes nothing",
	in:       "see w-.x b",
	remark:   "see w-.x b\n",
	prettier: "see w-.x b\n",
}, {
	// BEFORE, NEGATIVE: '_' is in the AFTER class (through \w) and not in
	// the before one. Its own escape is remark's unconditional '_' rule.
	name:     "an underscore before the dot writes nothing",
	in:       "see _.x b",
	remark:   "see \\_.x b\n",
	prettier: "see \\_.x b\n",
}, {
	// BEFORE, NEGATIVE: no previous byte at all.
	name:     "a dot opening the text writes nothing",
	in:       ".x b",
	remark:   ".x b\n",
	prettier: ".x b\n",
}, {
	// AFTER, NEGATIVE: a space ends the class.
	name:     "a space after the dot writes nothing",
	in:       "see www. x b",
	remark:   "see www. x b\n",
	prettier: "see www. x b\n",
}, {
	// AFTER, NEGATIVE: nothing at all after the dot.
	name:     "a dot ending the text writes nothing",
	in:       "trailing w.",
	remark:   "trailing w.\n",
	prettier: "trailing w.\n",
}, {
	// AFTER, NEGATIVES: punctuation is outside [\-.\w], one row per
	// character that a looser class would let in.
	name:     "a bang after the dot writes nothing",
	in:       "see w.! b",
	remark:   "see w.! b\n",
	prettier: "see w.! b\n",
}, {
	name:     "a hash after the dot writes nothing",
	in:       "see w.# b",
	remark:   "see w.# b\n",
	prettier: "see w.# b\n",
}, {
	name:     "a plus after the dot writes nothing",
	in:       "see w.+ b",
	remark:   "see w.+ b\n",
	prettier: "see w.+ b\n",
}, {
	// A COLON AFTER THE DOT, which is where the two unsafe rows of the same
	// extension meet: neither fires, because this ':' has no '/' after it
	// and this '.' has no word character.
	name:     "a colon after the dot writes nothing",
	in:       "see w.: b",
	remark:   "see w.: b\n",
	prettier: "see w.: b\n",
}, {
	name:     "a slash after the dot writes nothing",
	in:       "see w./ b",
	remark:   "see w./ b\n",
	prettier: "see w./ b\n",
}, {
	name:     "a paren after the dot writes nothing",
	in:       "see w.( b",
	remark:   "see w.( b\n",
	prettier: "see w.( b\n",
}, {
	name:     "a quote after the dot writes nothing",
	in:       "see w.\" b",
	remark:   "see w.\" b\n",
	prettier: "see w.\" b\n",
}, {
	// A TILDE AFTER THE DOT is outside the class. In remark mode the
	// tilde's own unconditional escape appears instead; prettier writes
	// neither byte, which is prettier's own measured answer.
	name:     "a tilde after the dot writes nothing for the dot",
	in:       "see w.~ b",
	remark:   "see w.\\~ b\n",
	prettier: "see w.~ b\n",
}, {
	// AN ORDERED-MARKER DOT, the other rule that owns this character. It is
	// checked at the same call site and in BOTH modes, so this row pins that
	// splitting the '.' arm did not take the marker rule with it. Unlike
	// every row above, its want is not a prettier measurement: a source
	// "1. x" is a LIST to prettier, so a text node holding those bytes is
	// not a document prettier can be handed. The escape is what keeps this
	// node from BECOMING a list on the next parse, which both modes need.
	name:     "an ordered-list marker dot is the other rule, in both modes",
	in:       "1. not a list",
	remark:   "1\\. not a list\n",
	prettier: "1\\. not a list\n",
}, {
	// AND THE MARKER RULE'S OWN NEGATIVE, which the 'w' rule must not
	// rescue: not at the start of a line, so neither rule fires.
	name:     "a marker dot that is not at a line start writes nothing",
	in:       "w1. x",
	remark:   "w1. x\n",
	prettier: "w1. x\n",
}, {
	// THE MARKER ROW THAT IS ACTUALLY LOAD-BEARING, and the reason the two
	// rules had to keep sharing one arm. A marker escape is written twice
	// over: once per character here, and once per rendered LINE by
	// escapeLeadingOrderedMarker. The line pass covers a marker followed by
	// a space, so removing this arm's marker call leaves every row above
	// unchanged — and this row, whose separator is a TAB, is one the line
	// pass does not reach. Measured against the frozen reference, which
	// escapes it; prettier's column is adfast's own, since a source "1.\tx"
	// is a list and not a text node prettier can be handed.
	//
	// Both columns escape. Prettier's used to be a bare "1.\tx", which was a
	// LOSS and not a measurement: a tab is a legal ordered-marker separator,
	// so that output re-parses as an orderedList and the text node is gone.
	// The character rule now runs in prettier mode as well — see
	// escapeOrderedMarker, which needs to run before the wrapper rather than
	// after it — and this is the row where that shows as a repair.
	name:     "a marker dot before a tab, which only the character rule catches",
	in:       "1.\tx",
	remark:   "1\\.\tx\n",
	prettier: "1\\.\tx\n",
}}

func TestRender_DotAfterWFollowsTheReferenceUnsafeRule(t *testing.T) {
	t.Parallel()
	for _, c := range dotAfterWCases {
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

// An '_' AFTER THE DOT IS THE ONE BYTE OF THE AFTER CLASS WHOSE ANSWER
// DEPENDS ON WHERE IT COMES FROM, so it gets its own pair of rows. The
// reference's escaper skips an after-conditional escape when the next
// character is being escaped unconditionally anyway; an '_' in a text VALUE
// is such a character, an '_' that is the next sibling's emphasis MARKER is
// not. Both measured against the frozen reference. A rule that read the
// class alone would escape both, and one that excluded '_' outright would
// escape neither.
func TestRender_DotAfterWUnderscoreDependsOnWhoOwnsIt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		want     string
		children []ast.Node
	}{{
		name:     "an underscore in the text value cancels the dot escape",
		children: []ast.Node{&ast.Text{Value: "see www._ b"}},
		want:     "see www.\\_ b\n",
	}, {
		name:     "and so does one that opens a word",
		children: []ast.Node{&ast.Text{Value: "see www._q b"}},
		want:     "see www.\\_q b\n",
	}, {
		name:     "an authored backslash before it cancels it too",
		children: []ast.Node{&ast.Text{Value: "see w.\\_ b"}},
		want:     "see w.\\\\\\_ b\n",
	}, {
		name: "an emphasis marker does not cancel it",
		children: []ast.Node{
			&ast.Text{Value: "see www."},
			&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "q"}}},
		},
		want: "see www\\._q_\n",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: c.children}}}
			if got := Render(root); got != c.want {
				t.Errorf("Render() = %q, want %q", got, c.want)
			}
		})
	}
}

// The unsafe row is keyed on 'phrasing' and excluded from 'label', and those
// two words decide every construct below. A TABLE CELL, a HEADING, the three
// marks, a BLOCKQUOTE and a LIST ITEM are all phrasing. A LINK LABEL is the
// one place the rule does not reach, and an IMAGE ALT and an INLINE CODE
// value are not phrasing at all. All measured against the frozen reference,
// rendering a hand-built mdast for each construct.
func TestRender_DotAfterWPerConstruct(t *testing.T) {
	t.Parallel()
	body := func() []ast.Node {
		return []ast.Node{&ast.Text{Value: "see www.x b"}}
	}
	cases := []struct {
		name string
		root ast.Node
		want string
	}{{
		name: "a table cell escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Table{Children: []ast.Node{
			&ast.TableRow{Children: []ast.Node{
				&ast.TableCell{Children: []ast.Node{&ast.Text{Value: "h"}}},
			}},
			&ast.TableRow{Children: []ast.Node{&ast.TableCell{Children: body()}}},
		}}}},
		want: "| h            |\n| ------------ |\n| see www\\.x b |\n",
	}, {
		name: "a heading escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Heading{Depth: 2, Children: body()}}},
		want: "## see www\\.x b\n",
	}, {
		name: "emphasis escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Emphasis{Children: body()},
		}}}},
		want: "_see www\\.x b_\n",
	}, {
		name: "strong escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Strong{Children: body()},
		}}}},
		want: "**see www\\.x b**\n",
	}, {
		name: "a strikethrough escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Delete{Children: body()},
		}}}},
		want: "~~see www\\.x b~~\n",
	}, {
		name: "a blockquote escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.Blockquote{Children: []ast.Node{
			&ast.Paragraph{Children: body()},
		}}}},
		want: "> see www\\.x b\n",
	}, {
		name: "a list item escapes it",
		root: &ast.Root{Children: []ast.Node{&ast.List{Children: []ast.Node{
			&ast.ListItem{Children: []ast.Node{&ast.Paragraph{Children: body()}}},
		}}}},
		want: "- see www\\.x b\n",
	}, {
		name: "a link label does not",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Link{URL: "u", Children: body()},
		}}}},
		want: "[see www.x b](u)\n",
	}, {
		name: "an image alt does not",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.Image{URL: "u", Children: body()},
		}}}},
		want: "![see www.x b](u)\n",
	}, {
		name: "an inline code value does not",
		root: &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
			&ast.InlineCode{Value: "see www.x b"},
		}}}},
		want: "`see www.x b`\n",
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

// BOTH SIDES OF THE RULE CAN LIE IN A DIFFERENT NODE than the dot, and the
// reference reads them anyway: its containerPhrasing threads the previous
// sibling's last byte in as `before` and the next sibling's first byte in as
// `after` (the handler peek). So the class test has to see across the node
// boundary in both directions, and which byte a sibling contributes depends
// on the sibling's own syntax — an emphasis leads with '_' (in the class), a
// strong with '*' and a code span with a backtick (both outside it). Every
// row measured against the frozen reference.
func TestRender_DotAfterWAtANodeBoundary(t *testing.T) {
	t.Parallel()
	lead := func() ast.Node { return &ast.Text{Value: "see w."} }
	cases := []struct {
		name     string
		want     string
		children []ast.Node
	}{{
		name:     "a following emphasis leads with an underscore, which is in the class",
		children: []ast.Node{lead(), &ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "q"}}}},
		want:     "see w\\._q_\n",
	}, {
		name:     "a following text node contributes its first byte",
		children: []ast.Node{lead(), &ast.Text{Value: "q b"}},
		want:     "see w\\.q b\n",
	}, {
		name:     "a following strong leads with a star, which is not",
		children: []ast.Node{lead(), &ast.Strong{Children: []ast.Node{&ast.Text{Value: "q"}}}},
		want:     "see w.**q**\n",
	}, {
		name:     "a following code span leads with a backtick",
		children: []ast.Node{lead(), &ast.InlineCode{Value: "q"}},
		want:     "see w.`q`\n",
	}, {
		name:     "a following link leads with a bracket",
		children: []ast.Node{lead(), &ast.Link{URL: "u", Children: []ast.Node{&ast.Text{Value: "q"}}}},
		want:     "see w.[q](u)\n",
	}, {
		name:     "a following strikethrough leads with a tilde",
		children: []ast.Node{lead(), &ast.Delete{Children: []ast.Node{&ast.Text{Value: "q"}}}},
		want:     "see w.~~q~~\n",
	}, {
		name:     "a following hard break leads with a backslash",
		children: []ast.Node{lead(), &ast.Break{}, &ast.Text{Value: "q"}},
		want:     "see w.\\\nq\n",
	}, {
		name:     "a newline inside the value ends the class",
		children: []ast.Node{&ast.Text{Value: "see w.\nq"}},
		want:     "see w.\nq\n",
	}, {
		name:     "the w may come from the previous text node",
		children: []ast.Node{&ast.Text{Value: "see w"}, &ast.Text{Value: ".x b"}},
		want:     "see w\\.x b\n",
	}, {
		name:     "but a preceding emphasis ends with its marker, not its w",
		children: []ast.Node{&ast.Emphasis{Children: []ast.Node{&ast.Text{Value: "w"}}}, &ast.Text{Value: ".x b"}},
		want:     "_w_.x b\n",
	}, {
		name:     "and a preceding code span ends with a backtick",
		children: []ast.Node{&ast.InlineCode{Value: "w"}, &ast.Text{Value: ".x b"}},
		want:     "`w`.x b\n",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: c.children}}}
			if got := Render(root); got != c.want {
				t.Errorf("Render() = %q, want %q", got, c.want)
			}
		})
	}
}

// The point of the escape is that the byte survives a round trip. remark
// mode used to DROP an authored "www\.x" — it decoded the escape and then
// had no rule to write it back — so a document the reference had formatted
// lost the backslash on the next remark pass, and the reference put it back
// on the pass after that. Both spellings now settle on the reference's.
//
// REMARK MODE ONLY, and not because prettier mode is untested: what prettier
// mode writes for a bare dot is the prettier column of dotAfterWCases, and
// what it does with an AUTHORED escape is the test below.
func TestRender_DotAfterWRoundTripsStablyInRemarkMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want string
	}{{
		name: "an authored escape is written back",
		src:  "see www\\.x b\n",
		want: "see www\\.x b\n",
	}, {
		name: "a bare dot gains the escape",
		src:  "see www.x b\n",
		want: "see www\\.x b\n",
	}, {
		name: "a dot the rule does not reach stays bare",
		src:  "see v.x b\n",
		want: "see v.x b\n",
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

// Both legs now keep an authored "\.", by two different routes: remark
// mode re-derives the escape from its own www rule, and prettier mode
// writes back the escape provenance the parse recorded (see
// PreservedEscapes, which holds '.' and '\'). Prettier is a source-slice
// printer, so keeping the authored backslash is its answer too.
//
// The second row is the case that makes the provenance TOTAL rather than a
// guess: an author's LITERAL backslash before a dot, which must stay two
// backslashes. A PreservedEscapes holding '.' but NOT '\' — the naive
// widening this bug's earlier attempts kept reaching for — writes it back
// as "a b\.c d", whose value on the next parse is "a b.c d": a byte the
// author wrote, silently gone. Both rows therefore belong in one test; a
// change that fixes either alone breaks the other.
//
// The prettier leg is rendered from the escape-preserving SOURCE FORM,
// because that is what prettier mode consumes (WithPrettierText). The
// formatter's canonicalization is what puts that form where the renderer
// reads it — convert.NormalizeFormat moves ast.Text.Raw onto ast.Text.Value
// — so the tree goes through it here exactly as it does in the facade. A
// bare Parse tree still holds the fully decoded value on Value, where every
// backslash is a literal one; rendering that under prettier's rules would
// read the author's literal backslashes as escapes. The remark leg takes
// the decoded tree straight, which is the form ITS rules are written for.
//
// Writing the escape unconditionally in prettier mode remains wrong, and is
// not what happens here: prettier leaves a BARE dot bare (dotAfterWCases'
// prettier column), so an unconditional rule would rewrite ordinary prose —
// "W.C. Fields" would come back "W\.C. Fields".
func TestRender_DotAfterWKeepsAnAuthoredEscapeInBothModes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		src      string
		remark   string
		prettier string
	}{{
		name:     "an authored dot escape survives in both modes",
		src:      "see www\\.x b\n",
		remark:   "see www\\.x b\n",
		prettier: "see www\\.x b\n",
	}, {
		name:     "and an authored LITERAL backslash before a dot is kept in both",
		src:      "a b\\\\.c d\n",
		remark:   "a b\\\\.c d\n",
		prettier: "a b\\\\.c d\n",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Render(Parse([]byte(c.src))); got != c.remark {
				t.Errorf("remark mode: %q -> %q, want %q", c.src, got, c.remark)
			}
			got := Render(convert.NormalizeFormat(Parse([]byte(c.src))), WithPrettierText())
			if got != c.prettier {
				t.Errorf("prettier mode: %q -> %q, want %q", c.src, got, c.prettier)
			}
		})
	}
}
