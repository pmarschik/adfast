package markdown

import (
	"slices"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The bug this file exists for: "[^1]: note" used to parse as a link
// reference definition and "a[^1]" as the reference to it, so a footnote
// pair rendered back as "a[^1](note)" — the definition text swallowed
// into a link. A footnote must survive md → ast → md as itself.
func TestFootnoteIsNotSwallowedIntoALink(t *testing.T) {
	got := renderMD(t, "a[^1]\n\n[^1]: note\n")
	if got != "a[^1]\n\n[^1]: note\n" {
		t.Errorf("footnote pair mangled: %q", got)
	}
}

// renderMD parses src and renders it back.
func renderMD(t *testing.T, src string) string {
	t.Helper()
	return Render(Parse([]byte(src)))
}

// TestFootnote_RoundTrip pins md → ast → md byte-for-byte against
// remark-stringify with remark-gfm (measured with remark 15 /
// mdast-util-gfm-footnote). Every expectation in the table is that
// measurement, including the cases where the source is NOT a footnote:
// the label rules are micromark's, and they are exactly what makes
// "[^a b]: x" a link reference definition instead.
func TestFootnote_RoundTrip(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"pair", "a[^1]\n\n[^1]: note\n", "a[^1]\n\n[^1]: note\n"},
		// A reference with no definition is literal text, so the bracket
		// escapes like any other.
		{"unmatched reference", "a[^1]\n", "a\\[^1]\n"},
		// A definition nothing references stays where it is: unlike
		// goldmark's footnote extension, adfast never deletes one.
		{"unreferenced definition", "[^1]: orphan\n", "[^1]: orphan\n"},
		// The identifier case-folds, so "[^A]" pairs with "[^a]".
		{"case-folded label", "a[^A]\n\n[^a]: x\n", "a[^A]\n\n[^a]: x\n"},
		{"case-folded non-ASCII", "a[^Ä]\n\n[^ä]: x\n", "a[^Ä]\n\n[^ä]: x\n"},
		// An escaped bracket is part of the label (a raw one is not — see
		// TestFootnote_NotAFootnote).
		{"escaped bracket in label", "a[^a\\[b]\n\n[^a\\[b]: x\n", "a[^a\\[b]\n\n[^a\\[b]: x\n"},
		// The label is opaque: emphasis inside it is not emphasis.
		{"markup in label", "a[^*b*]\n\n[^*b*]: x\n", "a[^*b*]\n\n[^*b*]: x\n"},
		// "!" does not make an image out of a reference.
		{"bang before reference", "![^1]\n\n[^1]: x\n", "\\![^1]\n\n[^1]: x\n"},
		// The whitespace run after the colon is the marker's, not content
		// (without eating it, "     x" would be an indented code block).
		{"padded definition", "a[^1]\n\n[^1]:     x\n", "a[^1]\n\n[^1]: x\n"},
		{"empty definition", "[^1]:\n\na[^1]\n", "[^1]:\n\na[^1]\n"},
		// A continuation line joins its paragraph: adfast reflows prose
		// (remark keeps the source line break here, "[^1]: one\n    two"),
		// which is the renderer's general soft-break behavior and not a
		// footnote rule.
		{"lazy continuation", "a[^1]\n\n[^1]: one\n    two\n", "a[^1]\n\n[^1]: one two\n"},
		// A second block indents by four spaces, the continuation width
		// the parser accepts.
		{"two blocks", "a[^1]\n\n[^1]: one\n\n    two\n", "a[^1]\n\n[^1]: one\n\n    two\n"},
		// The bullet marker is adfast's own ("-", where remark writes
		// "*"); the four-space continuation is the measured shape.
		{"list inside", "a[^1]\n\n[^1]: - one\n    - two\n", "a[^1]\n\n[^1]: - one\n    - two\n"},
		// A definition is a block like any other and stays where the
		// source put it, nested containers included.
		{"inside a blockquote", "a[^1]\n\n> [^1]: q\n", "a[^1]\n\n> [^1]: q\n"},
		{"definition first", "[^1]: x\n\na[^1]\n", "[^1]: x\n\na[^1]\n"},
		{"content after", "a[^1]\n\n[^1]: x\n\nafter\n", "a[^1]\n\n[^1]: x\n\nafter\n"},
		// Adjacent definition lines are separate blocks (a definition can
		// interrupt a paragraph, and another definition).
		{
			"adjacent definitions",
			"a[^1]\n\n[^1]: x\n[^2]: y\n\nb[^2]\n",
			"a[^1]\n\n[^1]: x\n\n[^2]: y\n\nb[^2]\n",
		},
		{
			"two references",
			"a[^1]b[^2]\n\n[^1]: x\n\n[^2]: y\n",
			"a[^1]b[^2]\n\n[^1]: x\n\n[^2]: y\n",
		},
		{"reference in a definition", "a[^1]\n\n[^1]: see [^2]\n\n[^2]: y\n", "a[^1]\n\n[^1]: see [^2]\n\n[^2]: y\n"},
		{"reference in a heading", "# h[^1]\n\n[^1]: x\n", "# h[^1]\n\n[^1]: x\n"},
		{"reference in emphasis", "**a[^1]**\n\n[^1]: x\n", "**a[^1]**\n\n[^1]: x\n"},
		{"reference in a link", "[a[^1]](u)\n\n[^1]: x\n", "[a[^1]](u)\n\n[^1]: x\n"},
		// The delimiter row widens to the header, as remark-gfm's
		// markdown-table does; the reference itself is untouched.
		{"reference in a table cell", "| a[^1] |\n| - |\n\n[^1]: x\n", "| a[^1] |\n| ----- |\n\n[^1]: x\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderMD(t, tc.src)
			if got != tc.want {
				t.Errorf("render = %q, want %q", got, tc.want)
			}
			// Every render must re-parse to itself, or the formatter is
			// not stable.
			if again := renderMD(t, got); again != got {
				t.Errorf("render is not idempotent: %q then %q", got, again)
			}
		})
	}
}

// hasFootnote reports whether the tree holds a footnote node of either
// kind.
func hasFootnote(n ast.Node) bool {
	switch n.(type) {
	case *ast.FootnoteDef, *ast.FootnoteRef:
		return true
	}
	return slices.ContainsFunc(ast.Children(n), hasFootnote)
}

// TestFootnote_NotAFootnote pins the label rules from the other side: a
// "[^…]" shape micromark rejects must not produce a footnote node here
// either. Each of these is a link reference definition (or plain text)
// in remark, measured; what adfast renders for them is the link
// reference behavior, not a footnote concern, so only the absence of the
// footnote is pinned.
func TestFootnote_NotAFootnote(t *testing.T) {
	repeat := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = 'x'
		}
		return string(b)
	}
	long := repeat(footnoteLabelMax + 1)
	tests := []struct{ name, src string }{
		{"space in label", "a[^a b]\n\n[^a b]: x\n"},
		{"escaped space in label", "a[^a\\ b]\n\n[^a\\ b]: x\n"},
		{"leading space in label", "a[^ a]\n\n[^ a]: x\n"},
		{"tab in label", "a[^a\tb]\n\n[^a\tb]: x\n"},
		{"raw bracket in label", "[^a[b]]: x\n"},
		{"empty label", "[^]: x\n"},
		{"unterminated label", "[^1: x\n"},
		{"trailing backslash label", "[^\\]: x\n"},
		{"over the label cap", "a[^" + long + "]\n\n[^" + long + "]: x\n"},
		// A definition needs its colon, and a reference its definition.
		{"no colon", "[^1] x\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if hasFootnote(Parse([]byte(tc.src))) {
				t.Errorf("%q parsed as a footnote", tc.src)
			}
		})
	}
	// The label cap is a cap, not a ban: one character less is a
	// footnote (micromark's link-reference size limit, measured).
	atCap := repeat(footnoteLabelMax)
	if !hasFootnote(Parse([]byte("a[^" + atCap + "]\n\n[^" + atCap + "]: x\n"))) {
		t.Errorf("a %d-character label is a footnote label", footnoteLabelMax)
	}
}

// The parsed pair carries the source label verbatim on both ends, and
// pairs on the normalized identifier.
func TestFootnote_ParsedShape(t *testing.T) {
	kids := ast.Children(Parse([]byte("a[^Ref]\n\n[^ref]: note\n")))
	if len(kids) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(kids))
	}
	para, ok := kids[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("want a paragraph, got %T", kids[0])
	}
	ref, ok := para.Children[len(para.Children)-1].(*ast.FootnoteRef)
	if !ok {
		t.Fatalf("want an *ast.FootnoteRef, got %T", para.Children[len(para.Children)-1])
	}
	if ref.Label != "Ref" {
		t.Errorf("reference label = %q, want %q", ref.Label, "Ref")
	}
	def, ok := kids[1].(*ast.FootnoteDef)
	if !ok {
		t.Fatalf("want an *ast.FootnoteDef, got %T", kids[1])
	}
	if def.Label != "ref" {
		t.Errorf("definition label = %q, want %q", def.Label, "ref")
	}
	if ast.NormalizeFootnoteLabel(ref.Label) != ast.NormalizeFootnoteLabel(def.Label) {
		t.Errorf("labels %q and %q do not pair", ref.Label, def.Label)
	}
	if got := ast.PlainText(def.Children); got != "note" {
		t.Errorf("definition content = %q, want %q", got, "note")
	}
}
