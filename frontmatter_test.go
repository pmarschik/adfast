package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

func TestWithFrontmatterProvider_HTMLCommentHeaders(t *testing.T) {
	// A mark-CLI-style metadata convention: leading <!-- Key: value -->
	// comment lines.
	var captured string
	provider := func(md string) (string, string, FrontmatterOutcome) {
		rest := md
		var front strings.Builder
		for strings.HasPrefix(rest, "<!--") {
			end := strings.Index(rest, "-->")
			if end < 0 {
				break
			}
			line := rest[:end+3]
			rest = strings.TrimLeft(rest[end+3:], "\n")
			front.WriteString(line)
			front.WriteString("\n")
		}
		if front.Len() == 0 {
			return "", md, FrontmatterAbsent
		}
		captured = front.String()
		return front.String(), rest, FrontmatterFound
	}

	md := "<!-- Space: DEV -->\n<!-- Title: My Page -->\n\n# Heading\n\nbody\n"
	doc := mdToADF(md, WithFrontmatterProvider(provider))

	if !strings.Contains(captured, "Space: DEV") || !strings.Contains(captured, "Title: My Page") {
		t.Fatalf("provider did not capture metadata: %q", captured)
	}
	out := adfToMD(doc)
	if strings.Contains(out, "Space: DEV") {
		t.Errorf("metadata leaked into the document: %q", out)
	}
	if !strings.Contains(out, "# Heading") {
		t.Errorf("body content missing: %q", out)
	}
}

func TestWithFrontmatterProvider_StylePreserving(t *testing.T) {
	// A custom provider's front block rides the formatter round trip
	// verbatim, like default YAML frontmatter.
	provider := func(md string) (string, string, FrontmatterOutcome) {
		if rest, ok := strings.CutPrefix(md, "<!-- meta -->\n"); ok {
			return "<!-- meta -->\n", rest, FrontmatterFound
		}
		return "", md, FrontmatterAbsent
	}
	in := "<!-- meta -->\ntext\n"
	out := fmtMD(in, WithFrontmatterProvider(provider))
	if !strings.HasPrefix(out, "<!-- meta -->\n") {
		t.Errorf("front block not re-emitted: %q", out)
	}
	if !strings.Contains(out, "text") {
		t.Errorf("body missing: %q", out)
	}
}

func TestDefaultFrontmatter_Canonical(t *testing.T) {
	doc := mdToADF("---\ntitle: x\n---\n\nbody\n")
	out := adfToMD(doc)
	if strings.Contains(out, "title: x") {
		t.Errorf("frontmatter leaked: %q", out)
	}
	if !strings.Contains(out, "body") {
		t.Errorf("body missing: %q", out)
	}
}

// TestDefaultFrontmatterProvider_Outcomes pins the three-way outcome of the
// built-in provider across well-formed, absent, and malformed shapes.
func TestDefaultFrontmatterProvider_Outcomes(t *testing.T) {
	cases := []struct {
		name    string
		md      string
		front   string
		rest    string
		outcome FrontmatterOutcome
	}{
		{"wellformed", "---\ntitle: x\n---\nbody\n", "---\ntitle: x\n---\n", "body\n", FrontmatterFound},
		{"wellformed-empty-body", "---\ntitle: x\n---\n", "---\ntitle: x\n---\n", "", FrontmatterFound},
		{"absent-plain", "# heading\n\nbody\n", "", "# heading\n\nbody\n", FrontmatterAbsent},
		{"absent-thematic-break", "---\n\ncontent\n", "", "---\n\ncontent\n", FrontmatterAbsent},
		{"absent-lone-fence", "---", "", "---", FrontmatterAbsent},
		// Malformed: opens "---\n" but the close line carries trailing text.
		{"malformed-close-trailing", "---\ntitle: x\n---0\n", "", "---\ntitle: x\n---0\n", FrontmatterMalformed},
		// Malformed: opens "---\n" but never closes (no trailing newline).
		{"malformed-no-trailing-nl", "---\ntitle: x\n---", "", "---\ntitle: x\n---", FrontmatterMalformed},
		// Malformed: trailing text on the OPEN fence, second "---" line present.
		{"malformed-open-trailing", "---0\n---0", "", "---0\n---0", FrontmatterMalformed},
		// Malformed: leading whitespace before the fence.
		{"malformed-leading-ws", "  ---\nx\n---\ny", "", "  ---\nx\n---\ny", FrontmatterMalformed},
	}
	for _, c := range cases {
		front, rest, outcome := defaultFrontmatterProvider(c.md)
		if outcome != c.outcome || front != c.front || rest != c.rest {
			t.Errorf("%s: defaultFrontmatterProvider(%q) = (%q, %q, %d), want (%q, %q, %d)",
				c.name, c.md, front, rest, outcome, c.front, c.rest, c.outcome)
		}
	}
}

// TestMalformedFrontmatter_Diagnostic verifies FromMarkdown surfaces a
// malformed-frontmatter diagnostic and keeps the block as body (not
// silently dropped) for malformed input, and stays quiet otherwise.
func TestMalformedFrontmatter_Diagnostic(t *testing.T) {
	var diags []convert.Diagnostic
	sink := func(d convert.Diagnostic) { diags = append(diags, d) }

	md := "---\ntitle: x\n---0\n"
	doc := mdToADF(md, WithDiagnostics(sink))
	if len(diags) != 1 || diags[0].Code != convert.CodeMalformedFrontmatter {
		t.Fatalf("expected one malformed-frontmatter diagnostic, got %+v", diags)
	}
	// Content preserved as body rather than dropped.
	out := adfToMD(doc)
	if !strings.Contains(out, "title: x") {
		t.Errorf("malformed frontmatter body was dropped: %q", out)
	}

	// Well-formed frontmatter emits no diagnostic.
	diags = nil
	mdToADF("---\ntitle: x\n---\nbody\n", WithDiagnostics(sink))
	for _, d := range diags {
		if d.Code == convert.CodeMalformedFrontmatter {
			t.Fatalf("well-formed frontmatter must not report malformed: %+v", diags)
		}
	}

	// A leading thematic break emits no diagnostic.
	diags = nil
	mdToADF("---\n\ncontent\n", WithDiagnostics(sink))
	for _, d := range diags {
		if d.Code == convert.CodeMalformedFrontmatter {
			t.Fatalf("thematic break must not report malformed: %+v", diags)
		}
	}
}

// TestFormat_RenderOnly_MalformedFrontmatter proves the format flag is now
// purely render-side: passing WithPrettierFormat to the render call alone
// is byte-identical to passing it to both halves, even for malformed
// frontmatter (the former parse-side residual is gone).
func TestFormat_RenderOnly_MalformedFrontmatter(t *testing.T) {
	for _, md := range []string{
		"---\ntitle: x\n---0\n",
		"---\ntitle: x\n---",
		"---0\n---0",
		"  ---\nx\n---\ny\n",
		"---\ntitle: x\n---\n\nbody\n", // well-formed, for good measure
		"# heading\n\nbody\n",          // absent
	} {
		bothHalves := ToMarkdown(FromMarkdown(md, WithPrettierFormat(), WithPrintWidth(80)), WithPrettierFormat(), WithPrintWidth(80))
		renderAlone := ToMarkdown(FromMarkdown(md), WithPrettierFormat(), WithPrintWidth(80))
		if bothHalves != renderAlone {
			t.Errorf("format not render-only for %q:\nboth halves: %q\nrender alone: %q", md, bothHalves, renderAlone)
		}
	}
}
