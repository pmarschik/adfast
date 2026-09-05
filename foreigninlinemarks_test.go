package adfast

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// The format leg carries the enclosing marks over an inline extension
// node. It used to drop them, so the emphasis that enclosed a run
// containing one moved onto whatever came after it: "*:media!*"
// formatted to ":media{}_!_". The faithful render, the ADF leg and
// prettier 3.8.1 all keep "_:media!_".
//
// The subject that makes this reachable from a parse is the dialect's
// bare-name fallback: ":media" written with no payload is not a media
// chip, it is the word ":media", and that node is foreign to convert's
// typed inline categories.

func TestFormatKeepsMarksAroundForeignInline(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		// The bare dialect name is the foreign inline node a parse
		// produces. Every mark the core spells must survive it.
		{"em around a bare name and a word", "*:media!*", "_:media!_\n"},
		{"em around a bare name alone", "*:media*", "_:media_\n"},
		{"em mid-sentence", "a *b :media c* d", "a _b :media c_ d\n"},
		{"strong", "**:media!**", "**:media!**\n"},
		{"strikethrough", "~~:media!~~", "~~:media!~~\n"},
		// The link is the mark whose loss was total: without it the
		// destination had nowhere to go and the whole link disappeared.
		{"link", "[:media](https://e.com)", "[:media](https://e.com)\n"},
		// A mark-backed dialect name degrades to a bare name too.
		{"em around a bare mark name", "*:u!*", "_:u!_\n"},
		// The non-native marks are wrapped back on per atom rather than
		// regrouped across the run, so they need rows of their own.
		{"underline", ":u[a :media b]", ":u[a :media b]\n"},
		{"text color", ":color[a :media b]{color=red}", ":color[a :media b]{color=\"red\"}\n"},
		{"superscript", ":sup[a :media b]", ":sup[a :media b]\n"},

		// GOOD ROWS — shapes that already worked and must not move.
		//
		// A directive name the dialect does not know stays a generic
		// *ast.TextDirective, which the case beside the foreign one
		// always handed its marks. It is the proof the fix did not just
		// move the bug.
		{"unknown text directive keeps its em", "*:foo!*", "_:foo!_\n"},
		{"unknown text directive under a link", "[:foo](https://e.com)", "[:foo](https://e.com)\n"},
		// A directive the dialect DOES read becomes a typed atom, and
		// ADF's status node carries no em, so the mark stays off it and
		// lands on the text beside it. That is the ADF leg's answer and
		// the format leg must keep matching it.
		{"typed dialect atom still sheds the em", "*:status[Done]!*", ":status[Done]{color=\"neutral\"}_!_\n"},
		{"typed media chip still sheds the em", "*:media{id=1}!*", ":media{#1}_!_\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtMD(tc.src)
			if got != tc.want {
				t.Errorf("format %q\n got  %q\n want %q", tc.src, got, tc.want)
			}
			// Totality is a fixpoint claim: a leg that loses the mark on
			// the second pass is still lossy.
			if again := fmtMD(got); again != got {
				t.Errorf("format is not a fixpoint for %q\n once  %q\n twice %q", tc.src, got, again)
			}
		})
	}
}

func TestFormatKeepsMarksAroundRegisteredInlineExtension(t *testing.T) {
	// The rule is a property of the inline position, not of the
	// dialect's own fallback: a caller's registered text kind is foreign
	// to convert in exactly the same way and keeps its marks too.
	reg := testInlineReg("hl", nil, func(adf.Node, extension.DecodeContext) ([]ast.Node, bool) {
		return nil, false
	})
	const src = "a *b :hl[c] d* e"
	const want = "a _b :hl[c] d_ e\n"
	got := fmtMD(src, WithExtensions(reg))
	if got != want {
		t.Errorf("format %q\n got  %q\n want %q", src, got, want)
	}
	if again := fmtMD(got, WithExtensions(reg)); again != got {
		t.Errorf("format is not a fixpoint\n once  %q\n twice %q", got, again)
	}
}

// blockFormExt is an extension kind with a BLOCK markdown form: it
// renders a "::name" leaf directive line and, like every leaf and
// container kind, does not implement extension.InlineLead. Only a
// hand-built tree can put one in inline position; no parse does.
type blockFormExt struct{ children []ast.Node }

func (*blockFormExt) Kind() string                                 { return "blockFormExt" }
func (n *blockFormExt) ChildNodes() []ast.Node                     { return n.children }
func (n *blockFormExt) SetChildNodes(kids []ast.Node)              { n.children = kids }
func (*blockFormExt) EncodeADF(extension.EncodeContext) []adf.Node { return nil }
func (n *blockFormExt) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteLeafDirective("blockext", nil, n.children)
}

func TestFormatLeavesABlockFormExtensionBare(t *testing.T) {
	// The marks go only to a node that DECLARES an inline form. A block
	// form has no inline spelling for an emphasis run to enclose, so
	// wrapping it would write a "::name" line inside emphasis
	// delimiters. It keeps riding bare instead.
	if _, inline := any(&blockFormExt{}).(extension.InlineLead); inline {
		t.Fatal("blockFormExt must not implement extension.InlineLead")
	}
	tree := &ast.Root{Children: []ast.Node{
		&ast.Paragraph{Children: []ast.Node{
			&ast.Text{Value: "a "},
			&ast.Emphasis{Children: []ast.Node{&blockFormExt{}}},
			&ast.Text{Value: " b"},
		}},
	}}
	const want = "a  b\n"
	if got := ToMarkdown(tree, WithPrettierFormat()); got != want {
		t.Errorf("block-form extension in inline position\n got  %q\n want %q", got, want)
	}
}
