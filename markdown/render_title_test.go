package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The bug this file exists for: a title reached the output verbatim, so a
// double quote inside one closed the title early and everything after it
// stopped being part of the construct. The FIRST render still looked
// plausible; the SECOND read the line as prose — the link stopped being a
// link, the definition stopped being a definition, and every reference
// that paired with that definition quietly unresolved.
//
// So the statement here is a fixed point, measured on a tree rather than
// on source text: whatever the title holds, the render has to re-parse to
// the same title in the same kind of node, and rendering that output again
// has to reproduce it byte for byte. The rows include titles with no quote
// at all, which must come out unchanged — an escape rule that fires too
// widely fails those, just as no rule at all fails the quoted ones.

// titleConstruct is one of the three nodes that carry a title. Each builds
// a document holding the title, and reads the title back out of a parse of
// the rendered document.
type titleConstruct struct {
	// build returns a tree holding this title.
	build func(title string) *ast.Root
	// readBack returns the title the parse found, or ok=false when the
	// construct did not survive as itself.
	readBack func(root ast.Node) (string, bool)
	name     string
}

var titleConstructs = []titleConstruct{
	{
		name: "inline link",
		build: func(title string) *ast.Root {
			return &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "See "},
				&ast.Link{URL: "./a.md", Title: title, Children: []ast.Node{&ast.Text{Value: "it"}}},
				&ast.Text{Value: "."},
			}}}}
		},
		readBack: func(root ast.Node) (string, bool) {
			n, ok := findNode[*ast.Link](root)
			if !ok {
				return "", false
			}
			return n.Title, true
		},
	},
	{
		// The definition form, with a use, because the definition's
		// demotion also costs the reference its destination.
		name: "link reference definition",
		build: func(title string) *ast.Root {
			return &ast.Root{Children: []ast.Node{
				&ast.Paragraph{Children: []ast.Node{
					&ast.Text{Value: "See "},
					&ast.LinkRef{Label: "a", ReferenceType: ast.ReferenceShortcut},
					&ast.Text{Value: "."},
				}},
				&ast.Definition{Label: "a", URL: "./a.md", Title: title},
			}}
		},
		readBack: func(root ast.Node) (string, bool) {
			def, ok := findNode[*ast.Definition](root)
			if !ok {
				return "", false
			}
			// The use has to still pair with it, or the reader lost the
			// destination even though the definition survived.
			if _, ok := findNode[*ast.LinkRef](root); !ok {
				return "", false
			}
			return def.Title, true
		},
	},
	{
		name: "inline image",
		build: func(title string) *ast.Root {
			return &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.Text{Value: "See "},
				&ast.Image{URL: "./a.png", Title: title, Children: []ast.Node{&ast.Text{Value: "it"}}},
				&ast.Text{Value: "."},
			}}}}
		},
		readBack: func(root ast.Node) (string, bool) {
			n, ok := findNode[*ast.Image](root)
			if !ok {
				return "", false
			}
			return n.Title, true
		},
	},
}

// The two render modes escape titles by different rules (remark's and
// prettier's), so every row runs through both.
var titleRenderModes = []struct {
	name string
	opts []RenderOption
}{
	{name: "remark"},
	{name: "prettier", opts: []RenderOption{WithPrettierText()}},
}

func TestTitleRenderIsAFixedPoint(t *testing.T) {
	titles := []struct{ name, title string }{
		// The good rows: nothing here needs an escape, and an over-eager
		// rule shows up as a changed output rather than as a lost title.
		{"no delimiter at all", "The Spec"},
		{"a colon before a letter", "note:see this"},
		{"a colon before a space", "Spec: the sequel"},
		{"a backslash before a letter", `back\slash`},
		// The reported case, and the rest of the delimiter matrix.
		{"a double quote", `He said "hi"`},
		{"only a double quote", `"`},
		{"a single quote", "it's fine"},
		{"both quotes", `both " and '`},
		{"parentheses", "paren (x)"},
		{"a closing paren only", "paren ) x"},
		{"an opening paren only", "paren ( x"},
		{"all three delimiters", `all three " ' ( )`},
		// A backslash is the other character that reads differently on the
		// way back in: before punctuation it escapes it, and at the end it
		// escapes the closing delimiter itself.
		{"a backslash before a quote", `a\"b`},
		{"a trailing backslash", `trailing\`},
		{"both quotes and a trailing backslash", `it's "q" trail\`},
		{"both quotes and a backslash before punctuation", `it's "q" a\-b`},
	}
	for _, mode := range titleRenderModes {
		for _, con := range titleConstructs {
			for _, tc := range titles {
				t.Run(mode.name+"/"+con.name+"/"+tc.name, func(t *testing.T) {
					once := Render(con.build(tc.title), mode.opts...)
					got, ok := con.readBack(Parse([]byte(once)))
					if !ok {
						t.Fatalf("%q did not re-parse as a %s", once, con.name)
					}
					if got != tc.title {
						t.Errorf("title %q rendered as %q and re-parsed as %q", tc.title, once, got)
					}
					if twice := Render(Parse([]byte(once)), mode.opts...); twice != once {
						t.Errorf("render is not a fixed point:\n first: %q\nsecond: %q", once, twice)
					}
				})
			}
		}
	}
}

// TestTitleDelimiterChoice pins the written form itself, which the fixed
// point above cannot see: remark writes every title in double quotes and
// escapes the quote inside, while prettier picks the delimiter that needs
// no escape (and parenthesizes when neither quote is free).
func TestTitleDelimiterChoice(t *testing.T) {
	tests := []struct{ title, remark, prettier string }{
		{"The Spec", `"The Spec"`, `"The Spec"`},
		{`He said "hi"`, `"He said \"hi\""`, `'He said "hi"'`},
		{"it's fine", `"it's fine"`, `"it's fine"`},
		{`both " and '`, `"both \" and '"`, `(both " and ')`},
		{`all three " ' ( )`, `"all three \" ' ( )"`, `"all three \" ' ( )"`},
		{`back\slash`, `"back\slash"`, `"back\\slash"`},
		{`trailing\`, `"trailing\\"`, `"trailing\\"`},
		{`a\"b`, `"a\\\"b"`, `'a\\"b'`},
		{"note:see this", `"note\:see this"`, `"note:see this"`},
		{"Spec: the sequel", `"Spec: the sequel"`, `"Spec: the sequel"`},
	}
	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			if got := (&mdRenderer{}).titleSegment(tc.title); got != " "+tc.remark {
				t.Errorf("remark title = %q, want %q", got, " "+tc.remark)
			}
			r := &mdRenderer{cfg: renderConfig{prettierText: true}}
			if got := r.titleSegment(tc.title); got != " "+tc.prettier {
				t.Errorf("prettier title = %q, want %q", got, " "+tc.prettier)
			}
		})
	}
}

// TestParenTitleRejectsAnUnescapedParen is a PRESERVED-BEHAVIOR PIN for
// the parse premise prettierTitle's "()" guard rests on, and a record of
// the divergence documented in render_title.go: CommonMark admits a "("
// or ")" inside a parenthesized title only backslash-escaped, and
// goldmark enforces that, while micromark's title scanner closes at the
// first ")" and lets an unescaped "(" through as content. Measured with
// mdast-util-from-markdown 2.x, "[a]: ./a.md (a ( b)" yields
// definition(title: "a ( b") there and no definition at all here.
//
// Nothing in the parser changed. The pin exists so that if goldmark ever
// accepts the lenient form, this test fails and points at the guard that
// could then be dropped, rather than the guard quietly outliving its
// reason.
func TestParenTitleRejectsAnUnescapedParen(t *testing.T) {
	// The GOOD case first: escaped, the same title parses fine, so the
	// rejection below is about the escape and not about parens at all.
	if def, ok := findNode[*ast.Definition](Parse([]byte(`[a]: ./a.md (a \( b)` + "\n"))); !ok {
		t.Fatal("the escaped form did not parse as a definition; the rejection below would prove nothing")
	} else if def.Title != "a ( b" {
		t.Errorf("escaped paren title = %q, want %q", def.Title, "a ( b")
	}

	for _, src := range []string{
		"[a]: ./a.md (a ( b)\n",
		"[a]: ./a.md (both \" and ' ()\n",
	} {
		if def, ok := findNode[*ast.Definition](Parse([]byte(src))); ok {
			t.Errorf("%q parsed as a definition with title %q; this dialect rejects "+
				"an unescaped paren, and prettierTitle's guard depends on it", src, def.Title)
		}
	}
}

// findNode returns the first node of type T in document order.
func findNode[T ast.Node](n ast.Node) (T, bool) {
	if hit, ok := n.(T); ok {
		return hit, true
	}
	for _, kid := range ast.Children(n) {
		if hit, ok := findNode[T](kid); ok {
			return hit, true
		}
	}
	var zero T
	return zero, false
}
