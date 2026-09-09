package markdown

import "testing"

// TestAutolinks_FormIsCarriedNotSniffed is a FIX test for the defect that
// Source.Autolinks decided an autolink's written FORM from the byte before the
// address: a '<' at Node.Pos meant the ANGLE form, whose address has to be
// closed by a '>'.
//
// THE DEFECT WAS A CROSS-FILE ONE, which is why the document below mixes the
// forms rather than testing one. punctLinkifyBoundaries — a constant in
// parser.go — claims every ASCII punctuation byte that may open a bare URL
// literal, and the reference opens one after '<' ("z<http://a.b c" links
// "http://a.b" at 2-12). Claiming the byte made a correct AutoLink whose Pos
// was that '<', the sniff read it as the angle form, the closing '>' was not
// there, and the span view DROPPED the node: the tree said "there is a link
// here" and Autolinks() said "there is none". Measured with '<' in the set and
// the sniff still in place:
//
//	"z<http://a.b c"   tree link present, Autolinks() = [], UnlocatedAutolinks() = 1
//	"z<www.a.b c"      tree link present, Autolinks() = [], UnlocatedAutolinks() = 1
//
// The form is carried on the node now — angleAutoLinkParser stamps the
// bracketed spelling and autolinkIsAngle reads the attribute — so no
// neighboring byte is consulted to decide it.
//
// THE GOOD CASES ARE IN THE SAME DOCUMENT ON PURPOSE. A "fix" that simply
// stopped believing in the angle form would satisfy the two rows above and
// wreck every bracketed autolink in the corpus, so a genuine `<https://c.d>`
// and a genuine flush-against-a-space `http://e.f` are asserted beside them,
// each with the span and the Bare flag it had before the fix. The last case is
// the discriminator the sniff could never have got right in both directions:
// "z<http://a.b>" is the ANGLE form even though a bare literal starts at the
// same offset, because goldmark's autolink parser accepts it first.
func TestAutolinks_FormIsCarriedNotSniffed(t *testing.T) {
	t.Parallel()

	// One paragraph, four autolinks, both forms, and a '<' in front of two of
	// the bare ones.
	const mixed = "z<http://a.b and <https://c.d> and http://e.f and y<www.g.h end\n"
	want := []Autolink{
		// The defect's own input: a bare literal whose left boundary is '<'.
		{Target: "http://a.b", Span: Span{2, 12}, Text: Span{2, 12}, Bare: true},
		// GOOD CASE: a genuine angle autolink. Span covers the brackets, Text
		// does not, and Bare is false.
		{Target: "https://c.d", Span: Span{17, 30}, Text: Span{18, 29}},
		// GOOD CASE: a genuine bare literal after a space.
		{Target: "http://e.f", Span: Span{35, 45}, Text: Span{35, 45}, Bare: true},
		// The scheme-less half of the defect's input: Target is completed, so
		// it is not the bytes Text covers.
		{Target: "http://www.g.h", Span: Span{52, 59}, Text: Span{52, 59}, Bare: true},
	}
	s := NewSource([]byte(mixed))
	got := s.Autolinks()
	if n := s.UnlocatedAutolinks(); n != 0 {
		t.Errorf("UnlocatedAutolinks(%q) = %d, want 0: the span view dropped an autolink it could not place", mixed, n)
	}
	if len(got) != len(want) {
		t.Fatalf("Autolinks(%q) = %+v, want %d autolinks", mixed, got, len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Autolinks(%q)[%d] = %+v, want %+v", mixed, i, got[i], w)
		}
	}
	// The spans have to name the bytes, not just line up numerically.
	for i, w := range []struct{ span, text string }{
		{"http://a.b", "http://a.b"},
		{"<https://c.d>", "https://c.d"},
		{"http://e.f", "http://e.f"},
		{"www.g.h", "www.g.h"},
	} {
		if span, text := string(s.Text(got[i].Span)), string(s.Text(got[i].Text)); span != w.span || text != w.text {
			t.Errorf("Autolinks(%q)[%d] covers span %q text %q, want %q and %q",
				mixed, i, span, text, w.span, w.text)
		}
	}

	// THE DISCRIMINATOR. The same "z<" prefix, but a closing '>' makes this the
	// angle form — goldmark's autolink parser is registered ahead of the
	// widened-boundary linkify parser, so it answers first. A fix that decided
	// the form by the parser that produced the node rather than by a byte has
	// to get both of these right, and only one of them is "z<".
	const angled = "z<http://a.b> end\n"
	as := NewSource([]byte(angled))
	agot := as.Autolinks()
	awant := Autolink{Target: "http://a.b", Span: Span{1, 13}, Text: Span{2, 12}}
	if n := as.UnlocatedAutolinks(); n != 0 {
		t.Errorf("UnlocatedAutolinks(%q) = %d, want 0", angled, n)
	}
	if len(agot) != 1 || agot[0] != awant {
		t.Fatalf("Autolinks(%q) = %+v, want exactly %+v", angled, agot, awant)
	}
	if span := string(as.Text(agot[0].Span)); span != "<http://a.b>" {
		t.Errorf("Autolinks(%q)[0].Span covers %q, want %q", angled, span, "<http://a.b>")
	}
}
