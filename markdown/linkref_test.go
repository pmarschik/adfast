package markdown

import (
	"slices"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// The bug this file exists for: goldmark resolves a reference-style link
// against its reference map and copies the destination onto the link
// node, and the lift used to keep only that — so "[spec]" with
// "[spec]: ./spec.md" round-tripped as the inline link
// "[spec](./spec.md)", and a definition nothing referenced was DELETED
// (a definition-only document rendered as "\n"). A link written as a
// reference must survive md → ast → md as a reference, and a definition
// must survive whether or not anything uses it.
func TestLinkRefIsNotInlinedAndItsDefinitionSurvives(t *testing.T) {
	const src = "See the [spec].\n\n[spec]: ./spec.md\n"
	if got := renderMD(t, src); got != src {
		t.Errorf("reference-style link mangled: %q", got)
	}
	const orphan = "[unused]: ./u.md\n"
	if got := renderMD(t, orphan); got != orphan {
		t.Errorf("unreferenced definition deleted: %q", got)
	}
}

// TestLinkRef_RoundTrip pins md → ast → md for the link reference kinds.
// Every expectation is a measurement of this build, and the rows are
// deliberately mixed: a document with a reference in it also carries the
// inline link or the plain text that must not change.
func TestLinkRef_RoundTrip(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		// --- the three written forms, links then images ---
		{"shortcut link", "See the [spec].\n\n[spec]: ./spec.md\n", "See the [spec].\n\n[spec]: ./spec.md\n"},
		{"collapsed link", "See the [spec][].\n\n[spec]: ./spec.md\n", "See the [spec][].\n\n[spec]: ./spec.md\n"},
		{"full link", "See [the spec][spec].\n\n[spec]: ./spec.md\n", "See [the spec][spec].\n\n[spec]: ./spec.md\n"},
		{"shortcut image", "![logo]\n\n[logo]: ./logo.png\n", "![logo]\n\n[logo]: ./logo.png\n"},
		{"collapsed image", "![logo][]\n\n[logo]: ./logo.png\n", "![logo][]\n\n[logo]: ./logo.png\n"},
		{"full image", "![the logo][logo]\n\n[logo]: ./logo.png\n", "![the logo][logo]\n\n[logo]: ./logo.png\n"},
		// --- titles: the source delimiter is not recorded, so a title
		// that needs no escape comes back in double quotes whichever
		// delimiter it was written with, exactly as an inline link's
		// title does (render_title_test.go covers the ones that do need
		// an escape) ---
		{"double-quoted title", "Use [x].\n\n[x]: ./x.md \"T\"\n", "Use [x].\n\n[x]: ./x.md \"T\"\n"},
		{"single-quoted title", "Use [x].\n\n[x]: ./x.md 'T'\n", "Use [x].\n\n[x]: ./x.md \"T\"\n"},
		{"parenthesized title", "Use [x].\n\n[x]: ./x.md (T)\n", "Use [x].\n\n[x]: ./x.md \"T\"\n"},
		// --- one definition, more than one use: the destination stays in
		// the one place the author wrote it ---
		{"two uses one definition", "Use [x] and [x] twice.\n\n[x]: ./x.md\n", "Use [x] and [x] twice.\n\n[x]: ./x.md\n"},
		// --- a definition nothing uses, and a document that is only
		// definitions ---
		{"unused definition", "Body.\n\n[unused]: ./u.md\n", "Body.\n\n[unused]: ./u.md\n"},
		{"definition only", "[only]: ./o.md\n", "[only]: ./o.md\n"},
		// Adjacent definition lines are separate blocks, and the renderer
		// separates every block with a blank line (as it does for adjacent
		// footnote definitions).
		{
			"adjacent definitions",
			"[a]: ./a.md\n[b]: ./b.md\n\nUse [a] and [b].\n",
			"[a]: ./a.md\n\n[b]: ./b.md\n\nUse [a] and [b].\n",
		},
		// --- label normalization: the identifier folds case and collapses
		// whitespace, so the use and the definition need not match
		// byte-for-byte, and NEITHER side is rewritten to the other ---
		{"case-folded label", "See [A].\n\n[ a ]: ./a.md\n", "See [A].\n\n[ a ]: ./a.md\n"},
		{"space in label", "Use [a b].\n\n[a b]: ./x.md\n", "Use [a b].\n\n[a b]: ./x.md\n"},
		{"escaped bracket in label", "Use [a\\[b].\n\n[a\\[b]: ./x.md\n", "Use [a\\[b].\n\n[a\\[b]: ./x.md\n"},
		// The label is opaque: emphasis inside it is not emphasis, so it
		// must not be re-escaped as prose either.
		{"markup in label", "Use [*em*].\n\n[*em*]: ./x.md\n", "Use [*em*].\n\n[*em*]: ./x.md\n"},
		// --- a definition is a block, and stays where the source put it ---
		{"inside a blockquote", "> Quote [q].\n>\n> [q]: ./q.md\n", "> Quote [q].\n>\n> [q]: ./q.md\n"},
		{"inside a list item", "- Item [i].\n\n  [i]: ./i.md\n", "- Item [i].\n\n  [i]: ./i.md\n"},
		{"definition first", "[x]: ./x.md\n\nUse [x].\n", "[x]: ./x.md\n\nUse [x].\n"},
		// --- a reference with no definition is not a reference at all: it
		// is literal text, and the bracket escapes like any other ---
		{"unmatched reference", "No def here [missing].\n", "No def here \\[missing].\n"},
		{"unmatched image reference", "No def here ![missing].\n", "No def here \\!\\[missing].\n"},
		// --- destinations serialize like an inline link's ---
		{"angle destination", "Use [s].\n\n[s]: <./a b.md>\n", "Use [s].\n\n[s]: <./a b.md>\n"},
		{"parens in destination", "Use [p].\n\n[p]: ./a(b).md\n", "Use [p].\n\n[p]: ./a\\(b\\).md\n"},
		// --- a reference is inline content, so it goes wherever inline
		// content goes ---
		{"inside emphasis", "**Bold [x] here.**\n\n[x]: ./x.md\n", "**Bold [x] here.**\n\n[x]: ./x.md\n"},
		{"inside a heading", "# Heading [h]\n\n[h]: ./h.md\n", "# Heading [h]\n\n[h]: ./h.md\n"},
		{
			"inside a table cell",
			"| [x] | ![y] |\n| - | - |\n| a | b |\n\n[x]: ./x.md\n[y]: ./y.png\n",
			"| [x] | ![y] |\n| --- | ---- |\n| a   | b    |\n\n[x]: ./x.md\n\n[y]: ./y.png\n",
		},
		{
			"image reference inside a link reference",
			"[![the logo][logo]][home]\n\n[logo]: ./l.png\n[home]: ./index.md\n",
			"[![the logo][logo]][home]\n\n[logo]: ./l.png\n\n[home]: ./index.md\n",
		},
		// --- a reference next to an inline link: the inline one keeps its
		// destination, the reference keeps its label ---
		{
			"reference beside an inline link",
			"Text with [ref] and [inline](./i.md).\n\n[ref]: ./r.md\n",
			"Text with [ref] and [inline](./i.md).\n\n[ref]: ./r.md\n",
		},
		// --- the control: a footnote is a different pair with the same
		// bracket shape, and this change must not touch it ---
		{"footnote control", "Text.[^1]\n\n[^1]: A note.\n", "Text.[^1]\n\n[^1]: A note.\n"},
		{
			"footnote and reference together",
			"Text.[^1] See [spec].\n\n[^1]: A note.\n\n[spec]: ./spec.md\n",
			"Text.[^1] See [spec].\n\n[^1]: A note.\n\n[spec]: ./spec.md\n",
		},
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

// TestLinkRef_ParsedShape pins the nodes the parse produces: the source
// label verbatim on both ends, the written form recorded, and the
// destination in one place only.
func TestLinkRef_ParsedShape(t *testing.T) {
	kids := ast.Children(Parse([]byte("See [the spec][Spec].\n\n[ spec ]: ./spec.md \"T\"\n")))
	if len(kids) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(kids))
	}
	para, ok := kids[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("want a paragraph, got %T", kids[0])
	}
	ref, ok := para.Children[1].(*ast.LinkRef)
	if !ok {
		t.Fatalf("want an *ast.LinkRef, got %T", para.Children[1])
	}
	if ref.Label != "Spec" {
		t.Errorf("reference label = %q, want %q", ref.Label, "Spec")
	}
	if ref.ReferenceType != ast.ReferenceFull {
		t.Errorf("reference type = %q, want %q", ref.ReferenceType, ast.ReferenceFull)
	}
	if got := ast.PlainText(ref.Children); got != "the spec" {
		t.Errorf("link text = %q, want %q", got, "the spec")
	}
	def, ok := kids[1].(*ast.Definition)
	if !ok {
		t.Fatalf("want an *ast.Definition, got %T", kids[1])
	}
	if def.Label != " spec " {
		t.Errorf("definition label = %q, want %q", def.Label, " spec ")
	}
	if def.URL != "./spec.md" || def.Title != "T" {
		t.Errorf("definition destination = %q title = %q", def.URL, def.Title)
	}
	if ref.Identifier() != def.Identifier() {
		t.Errorf("labels %q and %q do not pair", ref.Label, def.Label)
	}
}

// TestLinkRef_ImageParsedShape pins the image side of the same shape,
// including that the shortcut form's children are the label doubling as
// the alt text (which is what ast.PlainText and the ADF leg read).
func TestLinkRef_ImageParsedShape(t *testing.T) {
	kids := ast.Children(Parse([]byte("![logo][]\n\n[logo]: ./logo.png\n")))
	para, ok := kids[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("want a paragraph, got %T", kids[0])
	}
	ref, ok := para.Children[0].(*ast.ImageRef)
	if !ok {
		t.Fatalf("want an *ast.ImageRef, got %T", para.Children[0])
	}
	if ref.Label != "logo" || ref.ReferenceType != ast.ReferenceCollapsed {
		t.Errorf("image reference = %q / %q", ref.Label, ref.ReferenceType)
	}
	if got := ast.PlainText(ref.Children); got != "logo" {
		t.Errorf("alt text = %q, want %q", got, "logo")
	}
}

// TestLinkRef_EmptyDestinationKeepsTheBlockADefinition pins the one
// destination form the renderer cannot write literally: "[a]: " with
// nothing after the colon is not a definition, so the block would vanish
// on the next parse. Only a hand-built tree can hold one — the parse
// never produces it, because the source it would come from is not a
// definition either.
func TestLinkRef_EmptyDestinationKeepsTheBlockADefinition(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{&ast.Definition{Label: "e", URL: ""}}}
	got := Render(root)
	if got != "[e]: <>\n" {
		t.Fatalf("render = %q, want %q", got, "[e]: <>\n")
	}
	if !hasDefinition(Parse([]byte(got))) {
		t.Errorf("%q did not re-parse as a definition", got)
	}
}

// TestLinkRef_UnknownReferenceTypeRendersAsShortcut pins the fallback for
// a reference node built without a written form (the zero ReferenceType):
// the shortcut render is the least destructive, because the label alone
// still pairs.
func TestLinkRef_UnknownReferenceTypeRendersAsShortcut(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{
		&ast.Paragraph{Children: []ast.Node{&ast.LinkRef{Label: "x"}}},
		&ast.Definition{Label: "x", URL: "./x.md"},
	}}
	if got := Render(root); got != "[x]\n\n[x]: ./x.md\n" {
		t.Errorf("render = %q", got)
	}
}

// TestLinkRef_DefinitionRunTakesTheBlockSeparator PINS PRESERVED
// BEHAVIOR, not a fix: a run of adjacent definitions is written one per
// paragraph, so a tight run in the source gains a blank line on the first
// render and is stable from there. See renderDefinition's SPACING note for
// why the run is not tightened — briefly: this is remark-stringify's
// output, which is what this renderer targets, and prettier (measured at
// 3.8.1) tightens only the LINK REFERENCE run while keeping the blank line
// between adjacent FOOTNOTE definitions and between the two kinds, so
// following it would add a rule for one kind rather than replace two with
// one.
//
// The footnote rows and the mixed row are here because they are the half a
// tightening must NOT move: they are already what prettier writes.
func TestLinkRef_DefinitionRunTakesTheBlockSeparator(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "adjacent link reference definitions",
			src:  "[a]: ./a.md\n[b]: ./b.md\n",
			want: "[a]: ./a.md\n\n[b]: ./b.md\n",
		},
		{
			name: "a definition run after a paragraph",
			src:  "see [a] and [b]\n\n[a]: ./a.md\n[b]: ./b.md\n",
			want: "see [a] and [b]\n\n[a]: ./a.md\n\n[b]: ./b.md\n",
		},
		{
			name: "adjacent footnote definitions",
			src:  "a[^1] b[^2]\n\n[^1]: one\n[^2]: two\n",
			want: "a[^1] b[^2]\n\n[^1]: one\n\n[^2]: two\n",
		},
		{
			name: "a link definition beside a footnote definition",
			src:  "[a]: ./a.md\n[^1]: note\n",
			want: "[a]: ./a.md\n\n[^1]: note\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderMD(t, tt.src)
			if got != tt.want {
				t.Errorf("render = %q, want %q", got, tt.want)
			}
			// The re-spacing happens once. A caller pays one diff, not
			// churn on every render.
			if twice := renderMD(t, got); twice != got {
				t.Errorf("not a fixpoint:\n once:  %q\n twice: %q", got, twice)
			}
		})
	}
}

// TestLinkRef_TitleOnItsOwnLine covers the CommonMark rule the lift used to
// break: a title written on the line AFTER the destination is a title only
// when nothing else follows it on that line. The spec (0.31.2) ends a
// definition after "an optional link title … No further character may
// occur", and the reference implementation (commonmark.js 0.31.2) resolves
// `[foo]: /url` + `"title" ok` to `<a href="/url">foo</a>` — no title
// attribute — while keeping `"title" ok` as a paragraph.
//
// goldmark v1.8.5 leaves the excluded line in the paragraph but still
// records the title it scanned off it, so the lift copied a title the
// author never wrote AND the line stayed prose: the same text appeared
// TWICE in the render. The "tail" rows are the FIX PROOF; the rows above
// them are the good cases that keep the fix honest, since "reject every
// next-line title" would pass the tail rows alone.
func TestLinkRef_TitleOnItsOwnLine(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		// --- a title alone on its own line IS the definition's title ---
		{
			"next line holds the title alone",
			"[a]: x.md\n\"t\"\n\nUse [a].\n",
			"[a]: x.md \"t\"\n\nUse [a].\n",
		},
		{
			"next line holds the title and trailing spaces",
			"[a]: x.md\n\"t\"   \n\nUse [a].\n",
			"[a]: x.md \"t\"\n\nUse [a].\n",
		},
		{
			"next line holds an indented title",
			"[a]: x.md\n   \"t\"\n\nUse [a].\n",
			"[a]: x.md \"t\"\n\nUse [a].\n",
		},
		// --- a title with anything after it on the line is NO title, and
		// the line is a paragraph of its own ---
		{
			"next line holds a title and a tail",
			"[a]: x.md\n\"t\" tail\n\nUse [a].\n",
			"[a]: x.md\n\n\"t\" tail\n\nUse [a].\n",
		},
		{
			"the tail follows a single-quoted title",
			"[a]: x.md\n't' tail\n\nUse [a].\n",
			"[a]: x.md\n\n't' tail\n\nUse [a].\n",
		},
		{
			"the tail follows a parenthesized title",
			"[a]: x.md\n(t) tail\n\nUse [a].\n",
			"[a]: x.md\n\n(t) tail\n\nUse [a].\n",
		},
		// A destination whose own last byte is a title closer: the check
		// cannot be "does the definition end in a quote or a paren".
		{
			"the destination ends in a closing paren",
			"[a]: a(b)\n\"t\" tail\n\nUse [a].\n",
			"[a]: a\\(b\\)\n\n\"t\" tail\n\nUse [a].\n",
		},
		{
			"the destination is angle-bracketed",
			"[a]: <x y>\n\"t\" tail\n\nUse [a].\n",
			"[a]: <x y>\n\n\"t\" tail\n\nUse [a].\n",
		},
		{
			"the definition sits in a list item",
			"- [a]: x.md\n  \"t\" tail\n\nUse [a].\n",
			"- [a]: x.md\n  \"t\" tail\n\nUse [a].\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderMD(t, tc.src); got != tc.want {
				t.Errorf("render of %q\n got %q\nwant %q", tc.src, got, tc.want)
			}
		})
	}
}

// TestLinkRef_ExcludedTitleIsNotOnTheNode pins the same rule one layer
// down, on the pivot AST rather than the render, because the AST is what
// the ADF leg reads: a definition whose title line CommonMark excludes
// must carry NO title, or every consumer of the tree invents one.
func TestLinkRef_ExcludedTitleIsNotOnTheNode(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"title alone", "[a]: x.md\n\"t\"\n", "t"},
		{"title with a tail", "[a]: x.md\n\"t\" tail\n", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := firstDefinition(Parse([]byte(tc.src)))
			if def == nil {
				t.Fatalf("%q produced no definition node", tc.src)
			}
			if def.Title != tc.want {
				t.Errorf("definition title = %q, want %q", def.Title, tc.want)
			}
		})
	}
}

// TestLinkRef_PaddingIsNotPartOfTheValue is the FIX PROOF for the second
// recorded value that cannot be copied unchecked (see
// definitionPartsAsWritten): inside a container whose prefix ends in a TAB,
// goldmark v1.8.5 hands out a label and a title with the leftover tab columns
// expanded into real spaces, and the lift wrote those spaces into the
// document. A title is a VALUE, so `"  t"` is a title the author never typed
// and it survives into whatever reads the tree.
//
// Every "want" here is the reference implementation's answer, checked against
// commonmark.js 0.31.2 on the same source:
//
//	> [a]: x.md      >\t[a]: x.md "t"      > [a]: x.md
//	>\t"t"           >                     >\t"  t"
//	  → title="t"      → title="t"           → title="  t"
//	                     text a               (the two written spaces stay)
//
// The rows above the padded ones are the GOOD CASES, and they are what keeps
// the fix from being "strip the leading spaces off every value": a title
// written WITH leading spaces, and a label folded across a line that begins
// with spaces of its own, both keep every byte. The padded-line row with a
// tab inside the quotes pins the boundary from the other end — the padding
// goes, the author's tab stays.
func TestLinkRef_PaddingIsNotPartOfTheValue(t *testing.T) {
	tests := []struct {
		name, src, wantLabel, wantTitle, wantMD string
	}{
		// --- good cases: no partly consumed tab, nothing to correct ---
		{
			"a title on its own line in a blockquote",
			"> [a]: x.md\n> \"t\"\n>\n> [a]\n",
			"a", "t",
			"> [a]: x.md \"t\"\n>\n> [a]\n",
		},
		{
			"the author's own leading spaces inside the title",
			"[a]: x.md \"  t  \"\n\n[a]\n",
			"a", "  t  ",
			"[a]: x.md \"  t  \"\n\n[a]\n",
		},
		{
			"a folded label whose second line begins with spaces",
			"[a\n   b]: x.md\n\n[a\nb]\n",
			"a\n   b", "",
			"[a    b]: x.md\n\n[a b]\n",
		},
		// --- the padded routes, one row each ---
		{
			"the title sits on a line a tab prefixed",
			"> [a]: x.md\n>\t\"t\"\n>\n> [a]\n",
			"a", "t",
			"> [a]: x.md \"t\"\n>\n> [a]\n",
		},
		{
			"the whole definition sits on a line a tab prefixed",
			">\t[a]: x.md \"t\"\n>\n> [a]\n",
			"a", "t",
			"> [a]: x.md \"t\"\n>\n> [a]\n",
		},
		{
			"the label folds onto a line a tab prefixed",
			"> [a\n>\tb]: x.md\n>\n> [a\nb]\n",
			"a\nb", "",
			"> [a b]: x.md\n>\n> [a b]\n",
		},
		// --- both at once: the padding goes, the written bytes stay ---
		{
			"a padded line carrying a title with its own spaces",
			"> [a]: x.md\n>\t\"  t\"\n>\n> [a]\n",
			"a", "  t",
			"> [a]: x.md \"  t\"\n>\n> [a]\n",
		},
		{
			"a padded line carrying a title that starts with a tab",
			"> [a]: x.md\n>\t\"\tt\"\n>\n> [a]\n",
			"a", "\tt",
			"> [a]: x.md \"\tt\"\n>\n> [a]\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := firstDefinition(Parse([]byte(tc.src)))
			if def == nil {
				t.Fatalf("%q produced no definition node", tc.src)
			}
			if def.Label != tc.wantLabel {
				t.Errorf("label of %q = %q, want %q", tc.src, def.Label, tc.wantLabel)
			}
			if def.Title != tc.wantTitle {
				t.Errorf("title of %q = %q, want %q", tc.src, def.Title, tc.wantTitle)
			}
			if got := renderMD(t, tc.src); got != tc.wantMD {
				t.Errorf("render of %q\n got %q\nwant %q", tc.src, got, tc.wantMD)
			}
		})
	}
}

// firstDefinition returns the first link reference definition of the tree,
// or nil when it holds none.
func firstDefinition(n ast.Node) *ast.Definition {
	if def, ok := n.(*ast.Definition); ok {
		return def
	}
	for _, c := range ast.Children(n) {
		if got := firstDefinition(c); got != nil {
			return got
		}
	}
	return nil
}

// hasDefinition reports whether the tree holds a link reference
// definition node.
func hasDefinition(n ast.Node) bool {
	if _, ok := n.(*ast.Definition); ok {
		return true
	}
	return slices.ContainsFunc(ast.Children(n), hasDefinition)
}

// TestLinkRef_NotAReferenceUse pins the other side: a "[…]" shape
// CommonMark does not read as a reference must not produce a reference
// node here either.
func TestLinkRef_NotAReferenceUse(t *testing.T) {
	tests := []struct{ name, src string }{
		// No definition, so no reference: the brackets are text.
		{"no definition", "Use [x].\n"},
		// An inline link is not a reference even when a definition with
		// the same label exists.
		{"inline link wins", "Use [x](./inline.md).\n\n[x]: ./def.md\n"},
		// A definition needs its colon, so its label defines nothing.
		{"no colon", "[x] ./x.md\n\nUse [x].\n"},
		// An empty destination is not a definition (measured: the whole
		// line stays a paragraph), so the use below it is text.
		{"empty destination", "[e]: \n\nUse [e].\n"},
		// A footnote label is a footnote's, not a link reference's.
		{"footnote pair", "Text.[^1]\n\n[^1]: A note.\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if hasReferenceUse(Parse([]byte(tc.src))) {
				t.Errorf("%q parsed as a link reference", tc.src)
			}
		})
	}
}

// TestLinkRef_NotADefinition pins which "[label]: …" shapes actually
// define: the lift must not invent a definition node for a paragraph that
// merely looks like one, or the render would turn it into a real one.
func TestLinkRef_NotADefinition(t *testing.T) {
	tests := []struct{ name, src string }{
		{"no colon", "[x] ./x.md\n\nUse [x].\n"},
		{"empty destination", "[e]: \n\nUse [e].\n"},
		{"footnote definition", "Text.[^1]\n\n[^1]: A note.\n"},
		{"inside a fence", "```\n[x]: ./x.md\n```\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if hasDefinition(Parse([]byte(tc.src))) {
				t.Errorf("%q parsed as a definition", tc.src)
			}
		})
	}
}

// hasReferenceUse reports whether the tree holds a link or image
// reference node.
func hasReferenceUse(n ast.Node) bool {
	switch n.(type) {
	case *ast.LinkRef, *ast.ImageRef:
		return true
	}
	return slices.ContainsFunc(ast.Children(n), hasReferenceUse)
}
