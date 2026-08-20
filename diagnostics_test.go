package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

func TestDiagnostics_ColwidthsOrphan(t *testing.T) {
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }

	mdToADF("::colwidths[100,200]\n\nno table here\n", WithDiagnostics(sink))
	if len(diags) != 1 || diags[0].Code != "colwidths-orphan" {
		t.Fatalf("diagnostics: %+v", diags)
	}

	diags = nil
	mdToADF("::colwidths[100,200]\n| a | b |\n| - | - |\n", WithDiagnostics(sink))
	if len(diags) != 0 {
		t.Fatalf("no diagnostic expected with a table: %+v", diags)
	}
}

func TestDiagnostics_DecisionsOrphan(t *testing.T) {
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }

	// No list at all, an ordered list, and a task list are all orphans:
	// only a plain bullet list can become a decisionList.
	for _, md := range []string{
		"::decisions\n\nno list here\n",
		"::decisions\n\n1. ordered\n",
		"::decisions\n\n- [ ] task\n",
		"::decisions\n",
	} {
		diags = nil
		mdToADF(md, WithDiagnostics(sink))
		if len(diags) != 1 || diags[0].Code != convert.CodeDecisionsOrphan {
			t.Errorf("mdToADF(%q) diagnostics: %+v", md, diags)
		}
	}

	diags = nil
	mdToADF("::decisions\n\n- decided\n", WithDiagnostics(sink))
	if len(diags) != 0 {
		t.Fatalf("no diagnostic expected with a bullet list: %+v", diags)
	}
}

func TestDiagnostics_ParseNoRecoveryNeeded(t *testing.T) {
	var diags []convert.Diagnostic
	// goldmark <=1.8.4 panicked on this input. Keep the former crasher as a
	// regression case; current versions parse it without invoking parseGuarded's
	// recovery path.
	mdToADF("*\n  \t\x60", WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))
	for _, d := range diags {
		if d.Code == convert.CodeParseRecovered {
			t.Fatalf("unexpected parse recovery diagnostic: %+v", diags)
		}
	}
}

func TestDiagnostics_UnsupportedCodeLanguage(t *testing.T) {
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }
	langs := []string{"go", "Python"}

	md := "```go\nx\n```\n\n```brainfuck\ny\n```\n\n```\nplain\n```\n\n```PYTHON\nz\n```\n"
	mdToADF(md, WithDiagnostics(sink), WithCodeLanguages(langs))
	// Only the unknown tag reports: matching is case-insensitive in both
	// directions and an empty language never reports.
	if len(diags) != 1 || diags[0].Code != convert.CodeUnsupportedCodeLanguage ||
		!strings.Contains(diags[0].Message, `"brainfuck"`) {
		t.Fatalf("diagnostics: %+v", diags)
	}

	// Conversion output is unchanged: the language encodes verbatim.
	withOpt := adfToMD(mdToADF(md, WithCodeLanguages(langs)))
	withoutOpt := adfToMD(mdToADF(md))
	if withOpt != withoutOpt {
		t.Errorf("conversion changed:\nwith:    %q\nwithout: %q", withOpt, withoutOpt)
	}

	// Without the option there is no language checking.
	diags = nil
	mdToADF(md, WithDiagnostics(sink))
	if len(diags) != 0 {
		t.Fatalf("no diagnostics expected without WithCodeLanguages: %+v", diags)
	}
}

func TestDiagnostics_UnresolvedAsset(t *testing.T) {
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }

	// A local image whose path the asset store cannot resolve (typical
	// markdown-first flow: file added before upload) reports itself.
	mdToADF("![new diagram](assets/new-diagram.png)\n",
		WithDiagnostics(sink),
		WithAssetIDResolver(func(string) (string, bool) { return "", false }),
	)
	found := false
	for _, d := range diags {
		if d.Code == "unresolved-asset" && strings.Contains(d.Message, "assets/new-diagram.png") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unresolved-asset diagnostic, got %+v", diags)
	}

	// External images are not assets — no diagnostic.
	diags = nil
	mdToADF("![chart](https://example.com/chart.png)\n", WithDiagnostics(sink))
	for _, d := range diags {
		if d.Code == "unresolved-asset" {
			t.Fatalf("external image must not report unresolved-asset: %+v", diags)
		}
	}
}
