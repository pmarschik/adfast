package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The bug this file exists for: the markdown route lost footnotes
// entirely — "[^1]: note" read as a link reference definition, so the
// pair reached ADF as "a[^1](note)", a link labeled with the reference
// and pointing at the note text. ADF has no footnote construct, so the
// pair cannot survive as itself; what it must never do again is vanish
// or turn into something that was not in the source.
func TestFootnoteReachesADF(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF("a[^1]\n\n[^1]: note\n",
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	if strings.Contains(got, `"href"`) {
		t.Errorf("the footnote became a link:\n%s", got)
	}
	if !strings.Contains(got, `"note"`) {
		t.Errorf("the definition text is gone:\n%s", got)
	}
	if !contains(codes, convert.CodeFootnoteFlattened) {
		t.Errorf("want a %s diagnostic, got %v", convert.CodeFootnoteFlattened, codes)
	}
}

// The flattened shape in full: the reference becomes its number as
// superscript text, and the definitions collect behind a rule at the end
// of the document as an ordered list, so the item numbers are the
// superscripts.
func TestFootnoteFlattensToSuperscriptAndAList(t *testing.T) {
	got := adfJSON(t, mdToADF("a[^1] b[^two]\n\n[^1]: first\n\n[^two]: second\n"))

	for _, want := range []string{
		`{"type":"text","marks":[{"attrs":{"type":"sup"},"type":"subsup"}],"text":"1"}`,
		`{"type":"text","marks":[{"attrs":{"type":"sup"},"type":"subsup"}],"text":"2"}`,
		`{"type":"rule"}`,
		`"type":"orderedList","attrs":{"order":1}`,
		`{"type":"paragraph","content":[{"type":"text","text":"first"}]}`,
		`{"type":"paragraph","content":[{"type":"text","text":"second"}]}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in\n%s", want, got)
		}
	}
}

// Every reference to one definition carries that definition's number,
// and the numbers follow definition order — the order of the list the
// reader sees at the end of the document.
func TestFootnoteNumbersFollowDefinitionOrder(t *testing.T) {
	// The references appear in the order 2, 1; the definitions in the
	// order 1, 2.
	got := adfJSON(t, mdToADF("a[^b] c[^a] d[^b]\n\n[^a]: first\n\n[^b]: second\n"))

	want := `"content":[{"type":"text","text":"a"},` +
		`{"type":"text","marks":[{"attrs":{"type":"sup"},"type":"subsup"}],"text":"2"},` +
		`{"type":"text","text":" c"},` +
		`{"type":"text","marks":[{"attrs":{"type":"sup"},"type":"subsup"}],"text":"1"},` +
		`{"type":"text","text":" d"},` +
		`{"type":"text","marks":[{"attrs":{"type":"sup"},"type":"subsup"}],"text":"2"}]`
	if !strings.Contains(got, want) {
		t.Errorf("want the numbering\n  %s\ngot\n  %s", want, got)
	}
}

// A definition the source nested (a blockquote here) still lands in the
// list at the end of the document — it is the reference's target, not
// the blockquote's content.
func TestNestedFootnoteDefinitionMovesToTheTail(t *testing.T) {
	got := adfJSON(t, mdToADF("a[^1]\n\n> [^1]: quoted\n"))

	if !strings.Contains(got, `{"type":"rule"},{"type":"orderedList"`) {
		t.Errorf("want the definition list at the end of the document:\n%s", got)
	}
	if strings.Contains(got, `"blockquote","content"`) {
		t.Errorf("the definition stayed inside the blockquote:\n%s", got)
	}
}

// The marks around a reference are the source's own, so they stay on the
// superscript the reference becomes.
func TestFootnoteReferenceKeepsItsMarks(t *testing.T) {
	got := adfJSON(t, mdToADF("**a[^1]**\n\n[^1]: x\n"))

	want := `{"type":"text","marks":[{"type":"strong"},{"attrs":{"type":"sup"},"type":"subsup"}],"text":"1"}`
	if !strings.Contains(got, want) {
		t.Errorf("want\n  %s\ngot\n  %s", want, got)
	}
}

// One diagnostic per definition, naming the label and the number, because
// the round trip returns the flattened form and a caller may want to say
// so before a push.
func TestFootnoteDiagnosticNamesLabelAndNumber(t *testing.T) {
	var msgs []string
	mdToADF("a[^one] b[^two]\n\n[^one]: x\n\n[^two]: y\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeFootnoteFlattened {
				msgs = append(msgs, d.Message)
			}
		}))

	if len(msgs) != 2 {
		t.Fatalf("want 2 diagnostics, got %d: %v", len(msgs), msgs)
	}
	for i, want := range []string{"[^one] flattened to superscript 1", "[^two] flattened to superscript 2"} {
		if !strings.Contains(msgs[i], want) {
			t.Errorf("diagnostic %d = %q, want it to name %q", i, msgs[i], want)
		}
	}
}

// The formatter is the other route, and it must not flatten anything: a
// footnote is a Markdown construct the md → md pass carries through
// (convert.Normalize keeps both kinds).
func TestFootnoteSurvivesTheFormatter(t *testing.T) {
	src := "a[^1]\n\n[^1]: note\n"
	if got := fmtMD(src); got != src {
		t.Errorf("format = %q, want %q", got, src)
	}
}

// The ADF leg is one-way: nothing in ADF decodes back to a footnote, so
// the md → ADF → md round trip returns the flattened form — and it is
// stable, which is what the round-trip fuzzer requires.
func TestFlattenedFootnoteRoundTripIsStable(t *testing.T) {
	once := roundTrip(t, "a[^1]\n\n[^1]: note\n")
	if !strings.Contains(once, ":sup[1]") {
		t.Errorf("want the superscript reference, got %q", once)
	}
	if twice := roundTrip(t, once); twice != once {
		t.Errorf("round trip is not idempotent:\n  %q\n  %q", once, twice)
	}
}

// A document that defines a footnote nothing references still shows it:
// adfast never deletes an unreferenced definition (goldmark's own
// footnote extension does).
func TestUnreferencedFootnoteDefinitionIsKept(t *testing.T) {
	got := adfJSON(t, mdToADF("[^1]: orphan\n"))

	if !strings.Contains(got, `"orphan"`) {
		t.Errorf("the unreferenced definition is gone:\n%s", got)
	}
}

// Two definitions of one label are two list items (remark keeps both
// definition nodes), and the reference resolves to the first, like GFM.
func TestDuplicateFootnoteDefinitionsBothSurvive(t *testing.T) {
	got := adfJSON(t, mdToADF("a[^1]\n\n[^1]: first\n\n[^1]: second\n"))

	for _, want := range []string{
		`"first"`, `"second"`,
		`{"type":"text","marks":[{"attrs":{"type":"sup"},"type":"subsup"}],"text":"1"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in\n%s", want, got)
		}
	}
}

// The mixed document the "keep" decision rests on: one definition a
// reference resolves to, and one nothing references, in the same source.
// Measured against remark on 2026-09-04 because it looks like a
// divergence, and the two remark legs disagree — remark's md → md render
// keeps both definitions (as the formatter case below does, byte for
// byte), remark-rehype's HTML render drops the unreferenced one and
// renumbers by first-reference order. The ADF encode follows the md leg:
// it is a push, not a view, so a dropped definition is gone for good.
// convert/footnote.go carries the full argument.
func TestUnreferencedFootnoteDefinitionSurvivesBesideAReferencedOne(t *testing.T) {
	const src = "[^n]: Note one.\n\n[^m]: Note two.\n\nBody uses only one.[^n]\n"

	// The GOOD case first, and the precondition for everything below:
	// the parse really does read "[^m]" as a definition nothing
	// references. If the body carried a second reference, "kept" would
	// be trivially true and the assertions would prove nothing.
	if got, want := fmtMD(src), src; got != want {
		t.Fatalf("md→md formatter: got %q, want %q (remark returns the same)", got, want)
	}
	if strings.Count(src, "[^m]") != 1 {
		t.Fatalf("fixture broken: %q must mention [^m] once, as its definition", src)
	}

	var msgs []string
	got := adfJSON(t, mdToADF(src, WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeFootnoteFlattened {
			msgs = append(msgs, d.Message)
		}
	})))

	// Exactly one superscript, so "[^m]" is confirmed unreferenced on
	// the ADF side too, not merely unreferenced in the source text.
	if n := strings.Count(got, `"subsup"`); n != 1 {
		t.Fatalf("want 1 superscript in the encoded document, got %d:\n%s", n, got)
	}
	// Both definitions reach the list, the unreferenced one included.
	for _, want := range []string{`"Note one."`, `"Note two."`} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in\n%s", want, got)
		}
	}

	// And the diagnostic tells the two apart, which is the hook a caller
	// needs to apply GFM's drop for itself.
	if len(msgs) != 2 {
		t.Fatalf("want 2 diagnostics, got %d: %v", len(msgs), msgs)
	}
	if want := "[^n] flattened to superscript 1"; !strings.Contains(msgs[0], want) {
		t.Errorf("referenced definition: %q, want it to name %q", msgs[0], want)
	}
	if want := "[^m] is defined but never referenced"; !strings.Contains(msgs[1], want) {
		t.Errorf("unreferenced definition: %q, want it to say %q", msgs[1], want)
	}
	if bad := "flattened to superscript 2"; strings.Contains(msgs[1], bad) {
		t.Errorf("unreferenced definition: %q must not claim %q — there is no superscript 2 in the document", msgs[1], bad)
	}
}

// A duplicate definition of a referenced label is itself unreachable: the
// reference resolves to the FIRST definition sharing the label, so the
// second one has no superscript pointing at it either, and the
// diagnostic must not claim one.
func TestDuplicateFootnoteDefinitionReportsTheSecondAsUnreferenced(t *testing.T) {
	var msgs []string
	mdToADF("a[^1]\n\n[^1]: first\n\n[^1]: second\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeFootnoteFlattened {
				msgs = append(msgs, d.Message)
			}
		}))

	if len(msgs) != 2 {
		t.Fatalf("want 2 diagnostics, got %d: %v", len(msgs), msgs)
	}
	if want := "flattened to superscript 1"; !strings.Contains(msgs[0], want) {
		t.Errorf("first definition: %q, want it to name %q", msgs[0], want)
	}
	if want := "is defined but never referenced"; !strings.Contains(msgs[1], want) {
		t.Errorf("second definition: %q, want it to say %q", msgs[1], want)
	}
}

// The numbering pin that goes with the keep: an unreferenced definition
// takes a number of its own, so putting it FIRST shifts the referenced
// footnote's superscript to 2 where remark-rehype's HTML shows 1. That
// is the definition-order numbering convert/footnote.go documents, and
// the list order still agrees with the superscript.
func TestUnreferencedFootnoteDefinitionTakesItsOwnNumber(t *testing.T) {
	const src = "[^m]: Note two.\n\n[^n]: Note one.\n\nBody uses only one.[^n]\n"

	if got, want := fmtMD(src), src; got != want {
		t.Fatalf("md→md formatter: got %q, want %q", got, want)
	}

	got := ToMarkdown(FromADF(mdToADF(src)), WithPrintWidth(80))
	want := "Body uses only one.:sup[2]\n\n---\n\n1. Note two.\n1. Note one.\n"
	if got != want {
		t.Fatalf("md→ADF→md: got %q, want %q", got, want)
	}
}
