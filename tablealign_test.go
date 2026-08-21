package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// The bug this file exists for: GFM table column alignment used to be
// erased by the markdown route, because the pivot AST had no alignment
// concept. It now rides the ADF leg as a synthetic never-wire carrier.
func TestTableAlignmentSurvivesTheADFRoute(t *testing.T) {
	tests := []struct{ name, md, want string }{
		{
			"left and right",
			"| a | b |\n|:--|--:|\n| 1 | 2 |\n",
			"| a  |  b |\n| :- | -: |\n| 1  |  2 |\n",
		},
		{
			"center",
			"| a | b |\n|:-:|:-:|\n| 1 | 2 |\n",
			"|  a  |  b  |\n| :-: | :-: |\n|  1  |  2  |\n",
		},
		{
			"a bare column between two aligned ones",
			"| a | b | c |\n|:-|-|-:|\n| 1 | 2 | 3 |\n",
			"| a  | b |  c |\n| :- | - | -: |\n| 1  | 2 |  3 |\n",
		},
		{
			"no alignment is untouched",
			"| a | b |\n| - | - |\n| 1 | 2 |\n",
			"| a | b |\n| - | - |\n| 1 | 2 |\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := roundTrip(t, tc.md)
			if got != tc.want {
				t.Fatalf("md → adf → md:\n want %q\n  got %q", tc.want, got)
			}
			if again := roundTrip(t, got); again != got {
				t.Fatalf("not idempotent:\n first %q\nsecond %q", got, again)
			}
		})
	}
}

// The carrier is an attribute on the table node, so it has to reach the
// ADF payload — and only when the markdown asks for alignment, or every
// existing consumer's payload would change.
func TestTableAlignmentIsTheSyntheticTableAttribute(t *testing.T) {
	aligned := adfJSON(t, mdToADF("| a | b |\n|:--|--:|\n| 1 | 2 |\n"))
	if !strings.Contains(aligned, `"align":["left","right"]`) {
		t.Errorf("want the align attr in the payload, got:\n%s", aligned)
	}

	plain := adfJSON(t, mdToADF("| a | b |\n| - | - |\n| 1 | 2 |\n"))
	if strings.Contains(plain, `"align"`) {
		t.Errorf("an unaligned table must carry no align attr, got:\n%s", plain)
	}
}

// Alignment has no ADF form at all, so a document carrying it is not
// wire-safe and StripSynthetic must clear it — the same contract as the
// heading anchor.
func TestTableAlignmentIsNotWireSafe(t *testing.T) {
	doc := ToADF(FromMarkdown("| a | b |\n|:--|--:|\n| 1 | 2 |\n"))
	if adf.IsWireSafe(doc) {
		t.Error("a table with alignment must not be wire-safe")
	}

	stripped := adf.StripSynthetic(doc)
	if !adf.IsWireSafe(stripped) {
		t.Error("StripSynthetic must clear the alignment")
	}
	if strings.Contains(adfJSON(t, stripped), `"align"`) {
		t.Errorf("stripped payload still carries align:\n%s", adfJSON(t, stripped))
	}
	// Stripping is copy-on-write: the original keeps its alignment.
	if !strings.Contains(adfJSON(t, doc), `"align"`) {
		t.Error("StripSynthetic mutated the input document")
	}
}

// The formatter never touches ADF, but it must agree with the conversion
// about alignment or the two would drift (format_contract_test.go pins the
// general obligation; this is the alignment case).
func TestTableAlignmentSurvivesTheFormatter(t *testing.T) {
	md := "| a | b |\n|:-|-:|\n| 1 | 2 |\n"
	formatted := ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())

	if !strings.Contains(formatted, "| :- | -: |") {
		t.Errorf("formatter lost the alignment: %q", formatted)
	}
	if got, want := adfJSON(t, mdToADF(formatted)), adfJSON(t, mdToADF(md)); got != want {
		t.Errorf("format then parse diverges from parse:\n want %s\n  got %s", want, got)
	}
}
