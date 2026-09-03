package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// The two smart-link directives that DO encode — the good case every
// subtest below carries alongside the incomplete one, so a fix for the
// degradation cannot quietly break the shape that already worked. The
// formatter re-spells the embed with its default layout, hence the two
// forms of it.
const (
	goodCardURL   = "https://example.com/good"
	goodCard      = "::linkCard[" + goodCardURL + "]"
	goodEmbed     = "::linkEmbed[" + goodCardURL + "]"
	goodEmbedFull = "::linkEmbed[" + goodCardURL + "]{layout=\"center\"}"
)

// voidKeyLinks is a SmartLinks resolver that KNOWS a key and maps it to
// the empty URL. That is the one way a NON-EMPTY ::linkCard/::linkEmbed
// label resolves to no URL, so it is the shape where the directive has
// real author text to lose.
func voidKeyLinks() Option {
	return WithSmartLinks(convert.SmartLinks{
		URLForKey: func(key string) (string, bool) {
			if key == "VOID-1" {
				return "", true
			}
			return "", false
		},
	})
}

// TestSmartLinkCard_UnresolvedLabelDegradesToText: ADF addresses both
// blockCard and embedCard by url and neither has a URL-less variant, so a
// ::linkCard/::linkEmbed whose label resolves to no URL has no card to
// become. It must not vanish: the LABEL degrades to a paragraph and a
// smartlink-degraded diagnostic reports the lost card (regression:
// EncodeADF returned nil for the unresolvable label, so the directive
// left the document with its text — silently, and mid-document it erased
// the whole line).
func TestSmartLinkCard_UnresolvedLabelDegradesToText(t *testing.T) {
	for _, tc := range []struct {
		name string
		good string
		card adf.Node
		bad  string
		back string
	}{
		{
			name: "linkCard",
			good: goodCard,
			card: &adf.BlockCard{URL: goodCardURL},
			bad:  "::linkCard[VOID-1]",
			back: goodCard,
		},
		{
			name: "linkEmbed",
			good: goodEmbed,
			card: &adf.EmbedCard{URL: goodCardURL, Layout: "center"},
			bad:  "::linkEmbed[VOID-1]",
			back: goodEmbedFull,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One document holding BOTH: the good directive must survive
			// as its card while the unresolvable one degrades beside it.
			md := tc.good + "\n\n" + tc.bad + "\n"

			var diags []convert.Diagnostic
			sink := WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) })
			got := mdToADF(md, voidKeyLinks(), sink)

			// Differential against the AST: the good directive still
			// encodes to its card, the bad one to a paragraph carrying the
			// label verbatim.
			assertSameADF(t, doc(tc.card, p(txt("VOID-1"))), got)

			// Exactly one report, naming the directive and the label so a
			// reader can find the line it is about.
			if len(diags) != 1 || diags[0].Code != convert.CodeSmartLinkDegraded {
				t.Fatalf("want one %s diagnostic, got %+v", convert.CodeSmartLinkDegraded, diags)
			}
			for _, want := range []string{tc.name, "VOID-1"} {
				if !strings.Contains(diags[0].Message, want) {
					t.Errorf("diagnostic does not name %q: %q", want, diags[0].Message)
				}
			}

			// The round trip keeps the label as prose rather than emitting
			// a bare newline, and the good card comes back whole.
			back := adfToMD(got, voidKeyLinks())
			if want := tc.back + "\n\nVOID-1\n"; back != want {
				t.Errorf("round trip lost the label:\n got %q\nwant %q", back, want)
			}

			// The prettier formatter mirrors the encode: it rewrites the
			// unresolvable directive to the same prose instead of erasing
			// the line, and its output encodes to the same ADF.
			formatted := fmtMD(md, voidKeyLinks())
			if want := tc.back + "\n\nVOID-1\n"; formatted != want {
				t.Errorf("format lost the label:\n got %q\nwant %q", formatted, want)
			}
			assertSameADF(t, got, mdToADF(formatted, voidKeyLinks()))
		})
	}
}

// TestSmartLinkCard_LabelWithNoTextDropsLoudly: a label with no text has
// no URL to resolve AND nothing to keep, so it is the one smart-link card
// that still drops. It must not drop in silence — the diagnostic is the
// only trace left of it, and mid-document the drop takes the author's
// line with it.
func TestSmartLinkCard_LabelWithNoTextDropsLoudly(t *testing.T) {
	for _, tc := range []struct {
		name string
		good string
		card adf.Node
		bad  string
	}{
		{"linkCard empty label", goodCard, &adf.BlockCard{URL: goodCardURL}, "::linkCard[]"},
		{"linkCard blank label", goodCard, &adf.BlockCard{URL: goodCardURL}, "::linkCard[   ]"},
		{"linkCard no label", goodCard, &adf.BlockCard{URL: goodCardURL}, "::linkCard{layout=\"center\"}"},
		{"linkEmbed empty label", goodEmbed, &adf.EmbedCard{URL: goodCardURL, Layout: "center"}, "::linkEmbed[]"},
		{"linkEmbed blank label", goodEmbed, &adf.EmbedCard{URL: goodCardURL, Layout: "center"}, "::linkEmbed[  ]"},
		{"linkEmbed no label", goodEmbed, &adf.EmbedCard{URL: goodCardURL, Layout: "center"}, "::linkEmbed{layout=\"wide\"}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := tc.good + "\n\n" + tc.bad + "\n"

			var diags []convert.Diagnostic
			sink := WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) })
			got := mdToADF(md, sink)

			// The good card survives; the empty one contributes no node.
			assertSameADF(t, doc(tc.card), got)

			if len(diags) != 1 || diags[0].Code != convert.CodeSmartLinkDegraded {
				t.Fatalf("want one %s diagnostic, got %+v", convert.CodeSmartLinkDegraded, diags)
			}
			if !strings.Contains(diags[0].Message, "no label text") {
				t.Errorf("diagnostic should say the label carried no text: %q", diags[0].Message)
			}

			// The prettier formatter reports the same drop, on its own
			// path — it erases the line whether or not the encode ran.
			var fdiags []convert.Diagnostic
			fsink := WithDiagnostics(func(d convert.Diagnostic) { fdiags = append(fdiags, d) })
			fmtMD(md, fsink)
			if len(fdiags) != 1 || fdiags[0].Code != convert.CodeSmartLinkDegraded {
				t.Fatalf("formatter: want one %s diagnostic, got %+v", convert.CodeSmartLinkDegraded, fdiags)
			}
		})
	}
}

// TestFormatMarkdown_SmartLinkCardPreserved is a PRESERVED-BEHAVIOR PIN,
// not a fix proof: the complete directives already survived the
// style-preserving formatter and must keep doing so, mid-document and
// solo. It passes both before and after the degradation fix.
func TestFormatMarkdown_SmartLinkCardPreserved(t *testing.T) {
	for _, tc := range []struct{ name, md string }{
		{"linkCard mid-document", "before\n\n" + goodCard + "\n\nafter\n"},
		{"linkCard solo", goodCard + "\n"},
		{"linkEmbed mid-document", "before\n\n" + goodEmbedFull + "\n\nafter\n"},
		{"linkEmbed solo", goodEmbedFull + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fmtMD(tc.md); got != tc.md {
				t.Errorf("format altered the card block:\n got %q\nwant %q", got, tc.md)
			}
		})
	}
}
