package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The markdown leg keeps a reference-style link as a reference and keeps
// the definition that feeds it (markdown/linkref_test.go pins that). ADF
// has neither construct: one link mark with a URL on it, and nowhere to
// write a destination down for later. So this leg must RESOLVE — the
// reader gets exactly the link they would have got from "[text](url)" —
// and drop the definition, reporting only the real loss: a definition
// nothing referenced.
//
// This file measures that. The strongest statement of it is the first
// test: a reference document's ADF must be byte-identical to the ADF of
// the same document written inline.

// TestLinkRefADFIsTheInlineFormsADF pins the resolution as an equality
// rather than a shape: every reference form converts to what the author
// would have got by writing the link inline, so a reference automatically
// takes every link path (marks, smart links, the resolver, file cards)
// that an inline link takes.
func TestLinkRefADFIsTheInlineFormsADF(t *testing.T) {
	tests := []struct {
		name, ref, inline string
	}{
		{
			"shortcut link",
			"See the [spec].\n\n[spec]: https://e.com/spec\n",
			"See the [spec](https://e.com/spec).\n",
		},
		{
			"collapsed link",
			"See the [spec][].\n\n[spec]: https://e.com/spec\n",
			"See the [spec](https://e.com/spec).\n",
		},
		{
			"full link",
			"See [the spec][spec].\n\n[spec]: https://e.com/spec\n",
			"See [the spec](https://e.com/spec).\n",
		},
		{
			"title travels with the destination",
			"See [the spec][spec].\n\n[spec]: https://e.com/spec \"T\"\n",
			"See [the spec](https://e.com/spec \"T\").\n",
		},
		{
			"shortcut image",
			"![logo]\n\n[logo]: https://e.com/logo.png\n",
			"![logo](https://e.com/logo.png)\n",
		},
		{
			"collapsed image",
			"![logo][]\n\n[logo]: https://e.com/logo.png\n",
			"![logo](https://e.com/logo.png)\n",
		},
		{
			"full image",
			"![the logo][logo]\n\n[logo]: https://e.com/logo.png\n",
			"![the logo](https://e.com/logo.png)\n",
		},
		{
			// A lone image is block media in ADF, not a paragraph, and the
			// title becomes its caption. The promotion is what the
			// substitution in convert/linkref.go exists for.
			"lone image is promoted with its caption",
			"![the logo][logo]\n\n[logo]: https://e.com/logo.png \"Cap\"\n",
			"![the logo](https://e.com/logo.png \"Cap\")\n",
		},
		{
			"image reference beside text is inline, like the inline form",
			"See ![the logo][logo] here.\n\n[logo]: https://e.com/logo.png\n",
			"See ![the logo](https://e.com/logo.png) here.\n",
		},
		{
			"marks around the reference are the source's own",
			"**Bold [spec] here.**\n\n[spec]: https://e.com/spec\n",
			"**Bold [spec](https://e.com/spec) here.**\n",
		},
		{
			"two uses of one definition",
			"Use [x] and [x].\n\n[x]: https://e.com/x\n",
			"Use [x](https://e.com/x) and [x](https://e.com/x).\n",
		},
		{
			// The label is an identifier, so the two ends pair after
			// normalization and the use keeps the text the author wrote.
			"label pairs after normalization",
			"See [A].\n\n[ a ]: https://e.com/a\n",
			"See [A](https://e.com/a).\n",
		},
		{
			// A definition is visible to the whole document wherever it
			// sits, and it leaves nothing behind where it sat.
			"definition nested in a blockquote",
			"> Quote [q].\n>\n> [q]: https://e.com/q\n",
			"> Quote [q](https://e.com/q).\n",
		},
		{
			"definition nested in a list item",
			"- Item [i].\n\n  [i]: https://e.com/i\n",
			"- Item [i](https://e.com/i).\n",
		},
		{
			"definition written before the use",
			"[x]: https://e.com/x\n\nUse [x].\n",
			"Use [x](https://e.com/x).\n",
		},
		{
			// Nested: the image reference inside the link reference. The
			// outer href loss here is the inline form's own behavior (ADF
			// has no inline image for an external URL, so the image
			// degrades to a link), which is exactly the point of the
			// equality.
			"image reference inside a link reference",
			"[![the logo][logo]][home]\n\n[logo]: https://e.com/l.png\n[home]: https://e.com/\n",
			"[![the logo](https://e.com/l.png)](https://e.com/)\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := adfJSON(t, mdToADF(tc.ref))
			want := adfJSON(t, mdToADF(tc.inline))
			if got != want {
				t.Errorf("reference form:\n  %s\ninline form:\n  %s", got, want)
			}
		})
	}
}

// The store-backed image path is the one the push side actually uses: a
// local destination the asset resolver knows becomes a real media node.
// A reference-style image must reach it too, or the same picture would
// upload from "![a](p)" and vanish from "![a]".
func TestLinkRefImageReachesTheAssetStore(t *testing.T) {
	resolver := WithAssetIDResolver(func(ref string) (string, bool) {
		if ref == "assets/shot.png" {
			return "abc-123", true
		}
		return "", false
	})
	got := adfJSON(t, mdToADF("![the shot][shot]\n\n[shot]: assets/shot.png\n", resolver))
	want := adfJSON(t, mdToADF("![the shot](assets/shot.png)\n", resolver))
	if got != want {
		t.Errorf("reference form:\n  %s\ninline form:\n  %s", got, want)
	}
	if !strings.Contains(got, `"id":"abc-123"`) {
		t.Errorf("the reference did not reach the store:\n%s", got)
	}
}

// A definition something references drops silently: it renders as nothing
// on the page, and its destination has already traveled to every use, so
// nothing is lost.
func TestUsedDefinitionDropsWithoutADiagnostic(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF("See the [spec].\n\n[spec]: https://e.com/spec\n",
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	if strings.Contains(got, "[spec]:") || strings.Contains(got, `"definition"`) {
		t.Errorf("the definition reached ADF:\n%s", got)
	}
	if !strings.Contains(got, `{"attrs":{"href":"https://e.com/spec"},"type":"link"}`) {
		t.Errorf("the destination did not travel to the use:\n%s", got)
	}
	if len(codes) != 0 {
		t.Errorf("want no diagnostics for a used definition, got %v", codes)
	}
}

// A definition nothing references is the one real loss on this leg — the
// destination travels nowhere — so it reports, naming the label and the
// destination the way the footnote diagnostics name their pair.
func TestUnusedDefinitionDiagnosticNamesLabelAndDestination(t *testing.T) {
	var msgs []string
	mdToADF("Body [used].\n\n[used]: https://e.com/u\n\n[first]: https://e.com/1\n\n[second]: https://e.com/2\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeUnusedDefinitionDropped {
				msgs = append(msgs, d.Message)
			}
		}))

	if len(msgs) != 2 {
		t.Fatalf("want 2 diagnostics, got %d: %v", len(msgs), msgs)
	}
	// Document order, not map order.
	for i, want := range []string{"[first]: https://e.com/1", "[second]: https://e.com/2"} {
		if !strings.Contains(msgs[i], want) {
			t.Errorf("diagnostic %d = %q, want it to name %q", i, msgs[i], want)
		}
	}
}

// A document that is only definitions carries no content at all, so it
// converts to the same empty document an empty input does — and says why
// once per definition.
func TestDefinitionOnlyDocumentIsAnEmptyDocument(t *testing.T) {
	var codes []string
	got := adfJSON(t, mdToADF("[only]: https://e.com/o\n",
		WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })))

	if want := adfJSON(t, mdToADF("")); got != want {
		t.Errorf("definition-only document:\n  %s\nempty document:\n  %s", got, want)
	}
	if !contains(codes, convert.CodeUnusedDefinitionDropped) {
		t.Errorf("want a %s diagnostic, got %v", convert.CodeUnusedDefinitionDropped, codes)
	}
}

// Two definitions of one label: the uses resolve to the FIRST, which is
// CommonMark's rule and the rule the markdown parse already paired them
// by. The second is then referenced by nothing, so it reports.
func TestDuplicateDefinitionsResolveToTheFirst(t *testing.T) {
	var msgs []string
	got := adfJSON(t, mdToADF("Use [x].\n\n[x]: https://e.com/first\n\n[x]: https://e.com/second\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeUnusedDefinitionDropped {
				msgs = append(msgs, d.Message)
			}
		})))

	if !strings.Contains(got, `"href":"https://e.com/first"`) {
		t.Errorf("want the first definition's destination:\n%s", got)
	}
	if strings.Contains(got, "second") {
		t.Errorf("the second definition won:\n%s", got)
	}
	// Both definitions carry the same identifier, and that identifier IS
	// used, so neither counts as an unused definition.
	if len(msgs) != 0 {
		t.Errorf("want no unused-definition diagnostics, got %v", msgs)
	}
}

// A "[label]" no definition matches is not a reference at all — the
// markdown parse leaves it as text — so it reaches ADF as the text the
// author typed rather than as a link with an empty href.
func TestUnmatchedReferenceReachesADFAsText(t *testing.T) {
	for _, src := range []string{"Missing [ref] here.\n", "Missing ![ref] here.\n"} {
		got := adfJSON(t, mdToADF(src))
		if strings.Contains(got, `"href"`) {
			t.Errorf("%q became a link:\n%s", src, got)
		}
		if !strings.Contains(got, strings.TrimSuffix(src, "\n")) {
			t.Errorf("%q lost its text:\n%s", src, got)
		}
	}
}

// The formatter is the other route, and it must not resolve anything: a
// reference is a Markdown construct the md → md pass carries through
// (convert.Normalize keeps all three kinds), including the definition
// nothing uses.
func TestLinkRefSurvivesTheFormatter(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"shortcut", "See the [spec].\n\n[spec]: ./spec.md\n", "See the [spec].\n\n[spec]: ./spec.md\n"},
		{"collapsed with a title", "See the [spec][].\n\n[spec]: ./spec.md \"The Spec\"\n", "See the [spec][].\n\n[spec]: ./spec.md \"The Spec\"\n"},
		{"full", "See [the spec][spec].\n\n[spec]: ./spec.md\n", "See [the spec][spec].\n\n[spec]: ./spec.md\n"},
		{"shortcut image", "![logo]\n\n[logo]: ./logo.png\n", "![logo]\n\n[logo]: ./logo.png\n"},
		{"full image", "![the logo][logo]\n\n[logo]: ./logo.png\n", "![the logo][logo]\n\n[logo]: ./logo.png\n"},
		{"unused definition", "Body.\n\n[unused]: ./u.md\n", "Body.\n\n[unused]: ./u.md\n"},
		{"definition only", "[only]: ./o.md\n", "[only]: ./o.md\n"},
		{"label not rewritten to its pair", "See [A].\n\n[ a ]: ./a.md\n", "See [A].\n\n[ a ]: ./a.md\n"},
		{"in a blockquote", "> Quote [q].\n>\n> [q]: ./q.md\n", "> Quote [q].\n>\n> [q]: ./q.md\n"},
		{"in a list item", "- Item [i].\n\n  [i]: ./i.md\n", "- Item [i].\n\n  [i]: ./i.md\n"},
		{
			"in a table cell",
			"| a | b |\n| - | - |\n| [x] | ![y] |\n\n[x]: ./x.md\n[y]: ./y.png\n",
			"| a   | b    |\n| --- | ---- |\n| [x] | ![y] |\n\n[x]: ./x.md\n\n[y]: ./y.png\n",
		},
		{
			// The wrapper must treat the whole construct as one unit, or
			// the line break would land inside the brackets and the
			// reference would stop being one.
			"prose wrap keeps the construct whole",
			"Some long prose that goes on and on and on so the wrapper has to think about [the spec][spec] here.\n\n[spec]: ./spec.md\n",
			"Some long prose that goes on and on and on so the wrapper has to think about\n[the spec][spec] here.\n\n[spec]: ./spec.md\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtMD(tc.src)
			if got != tc.want {
				t.Errorf("format = %q, want %q", got, tc.want)
			}
			if again := fmtMD(got); again != got {
				t.Errorf("format is not idempotent: %q then %q", got, again)
			}
		})
	}
}

// The ADF leg is one-way: nothing in ADF decodes back to a reference, so
// the md → ADF → md round trip returns the inline form — and it is
// stable, which is what the round-trip fuzzer requires.
func TestResolvedLinkRefRoundTripIsStable(t *testing.T) {
	once := roundTrip(t, "See the [spec].\n\n[spec]: https://e.com/spec\n")
	if once != "See the [spec](https://e.com/spec).\n" {
		t.Errorf("round trip = %q", once)
	}
	if twice := roundTrip(t, once); twice != once {
		t.Errorf("round trip is not idempotent:\n  %q\n  %q", once, twice)
	}
}

// PlainTextOf reads what a reader sees: the reference's own text (its
// label, when the label is the text), and nothing at all for a
// definition — which is bookkeeping, not content.
func TestPlainTextOfLinkRef(t *testing.T) {
	tests := []struct{ src, want string }{
		{"See the [spec].\n\n[spec]: ./spec.md\n", "See the spec."},
		{"See the [spec][].\n\n[spec]: ./spec.md\n", "See the spec."},
		{"See [the spec][spec].\n\n[spec]: ./spec.md\n", "See the spec."},
		{"![the logo][logo]\n\n[logo]: ./l.png\n", "the logo"},
		{"Body.\n\n[unused]: ./u.md\n", "Body."},
		{"[only]: ./o.md\n", ""},
	}
	for _, tc := range tests {
		if got := PlainTextOf(tc.src); got != tc.want {
			t.Errorf("PlainTextOf(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}
