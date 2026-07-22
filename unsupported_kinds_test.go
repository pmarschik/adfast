package adfast

import (
	"bytes"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// jiraUnsupported is an illustrative, synthetic product set used only to
// exercise the product-neutral diagnostic mechanism from the root suite
// (which cannot import the jira submodule — it imports this package).
// It is deliberately NOT the production jira.UnsupportedKinds: the live
// render probe (2026-07-22) confirmed Jira actually renders most of the
// kinds listed here (layoutSection, extension, alignment, …), so the
// real Jira set is far smaller (placeholder, multiBodiedExtension,
// extensionFrame — see jira.UnsupportedKinds).
// These entries are just fixtures proving "whatever kinds a product set
// names get flagged".
var jiraUnsupported = []string{
	"placeholder", "layoutSection", "layoutColumn",
	"extension", "bodiedExtension", "inlineExtension",
	"syncBlock", "bodiedSyncBlock",
	"alignment", "indentation", "breakout",
	"annotation", "dataConsumer", "fragment",
}

func codes(diags []convert.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func TestDiagnostics_UnsupportedInProduct(t *testing.T) {
	jiraOpt := WithUnsupportedKinds("jira", jiraUnsupported)
	confOpt := WithUnsupportedKinds("confluence", nil) // empty Confluence set

	// A leaf extension (Confluence-only) targeted at Jira flags once,
	// naming the kind and the product.
	t.Run("extension flags for jira", func(t *testing.T) {
		var diags []convert.Diagnostic
		sink := func(d convert.Diagnostic) { diags = append(diags, d) }
		mdToADF("::extension{key=\"chart\" type=\"com.example.charts\"}\n", jiraOpt, WithDiagnostics(sink))
		if len(diags) != 1 ||
			diags[0].Code != convert.CodeUnsupportedInProduct ||
			diags[0].Message != "extension is not available in jira" {
			t.Fatalf("diagnostics: %+v", diags)
		}
	})

	// A layoutSection (with its layoutColumn children) and a dataConsumer
	// mark: distinct offending kinds each report once.
	t.Run("layout and dataConsumer flag distinct kinds", func(t *testing.T) {
		var diags []convert.Diagnostic
		sink := func(d convert.Diagnostic) { diags = append(diags, d) }
		md := ":::dataConsumer{sources=\"frag-1,frag-2\"}\n" +
			"::extension{key=\"chart\" type=\"com.example.charts\"}\n:::\n\n" +
			"::::section\n:::column{width=\"50\"}\nleft\n:::\n\n" +
			":::column{width=\"50\"}\nright\n:::\n::::\n"
		mdToADF(md, jiraOpt, WithDiagnostics(sink))
		got := map[string]int{}
		for _, d := range diags {
			if d.Code != convert.CodeUnsupportedInProduct {
				t.Fatalf("unexpected code: %+v", d)
			}
			got[d.Message]++
		}
		want := []string{
			"extension is not available in jira",
			"dataConsumer is not available in jira",
			"layoutSection is not available in jira",
			"layoutColumn is not available in jira",
		}
		for _, w := range want {
			if got[w] != 1 {
				t.Errorf("want exactly one %q, got %d (all: %+v)", w, got[w], diags)
			}
		}
		if len(diags) != len(want) {
			t.Errorf("want %d distinct diagnostics, got %d: %+v", len(want), len(diags), diags)
		}
	})

	// The SAME document under the (empty) Confluence set flags nothing.
	t.Run("empty product set flags nothing", func(t *testing.T) {
		var diags []convert.Diagnostic
		sink := func(d convert.Diagnostic) { diags = append(diags, d) }
		mdToADF("::extension{key=\"chart\" type=\"com.example.charts\"}\n", confOpt, WithDiagnostics(sink))
		if len(diags) != 0 {
			t.Fatalf("empty set must flag nothing: %+v", diags)
		}
	})

	// A Jira-supported document flags nothing even under the Jira set.
	t.Run("supported kinds flag nothing", func(t *testing.T) {
		var diags []convert.Diagnostic
		sink := func(d convert.Diagnostic) { diags = append(diags, d) }
		mdToADF("# Heading\n\nA **paragraph** with a [link](https://x).\n\n- item\n", jiraOpt, WithDiagnostics(sink))
		if len(diags) != 0 {
			t.Fatalf("supported-only document must flag nothing: %+v", diags)
		}
	})

	// Product-neutral: an arbitrary set/label flags the kind it names.
	t.Run("arbitrary product set", func(t *testing.T) {
		var diags []convert.Diagnostic
		sink := func(d convert.Diagnostic) { diags = append(diags, d) }
		mdToADF("# Heading\n", WithUnsupportedKinds("widgetco", []string{"heading"}), WithDiagnostics(sink))
		if len(codes(diags)) != 1 ||
			diags[0].Message != "heading is not available in widgetco" {
			t.Fatalf("arbitrary set: %+v", diags)
		}
	})
}

// TestDiagnostics_UnsupportedInProduct_ByteGate proves the option is
// diagnostic-only: the produced ADF document is byte-identical with and
// without WithUnsupportedKinds.
func TestDiagnostics_UnsupportedInProduct_ByteGate(t *testing.T) {
	md := ":::dataConsumer{sources=\"frag-1\"}\n" +
		"::extension{key=\"chart\" type=\"com.example.charts\"}\n:::\n\n" +
		"::::section\n:::column{width=\"100\"}\nx\n:::\n::::\n"

	without := mustJSON(mdToADF(md))
	with := mustJSON(mdToADF(md, WithUnsupportedKinds("jira", jiraUnsupported),
		WithDiagnostics(func(convert.Diagnostic) {})))
	if !bytes.Equal(without, with) {
		t.Fatalf("ToADF output changed with WithUnsupportedKinds:\nwithout: %s\nwith:    %s", without, with)
	}
}
