package adfast

import (
	"slices"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/markdown"
)

// The statement this file exists for: SplitFrontmatter and the facade parse
// answer the same question with the same bytes. A consumer that only wants
// the boundary used to have two ways to get it — round-trip the document
// through ADF, or hand-roll a "---" scan — and the second one has to agree
// on a byte order mark, a CR line ending, an indented fence, an
// unterminated block and a leading "---" that is really a thematic break.
// The rows below are exactly those shapes, and each is asserted against
// FromMarkdown rather than against a second expectation written by hand, so
// a split that drifts from what a conversion acts on fails here.

// splitEdgeCases are the shapes a hand-rolled splitter gets wrong. The
// outcome is pinned too, so the table says what each shape MEANS and not
// only that the two paths agree about it.
var splitEdgeCases = []struct {
	name    string
	src     string
	front   string
	outcome FrontmatterOutcome
}{
	{
		name:    "no metadata at all",
		src:     "# Heading\n\nBody.\n",
		outcome: FrontmatterAbsent,
	},
	{
		name:    "a well-formed block",
		src:     "---\nstatus: Open\n---\nBody.\n",
		front:   "---\nstatus: Open\n---\n",
		outcome: FrontmatterFound,
	},
	{
		// The mark is peeled before the provider runs, so the fence is
		// still at column 0 for it. A splitter that skips the peel sees
		// "\ufeff---" and finds nothing.
		name:    "a byte order mark before the fence",
		src:     "\ufeff---\nstatus: Open\n---\nBody.\n",
		front:   "---\nstatus: Open\n---\n",
		outcome: FrontmatterFound,
	},
	{
		name:    "a byte order mark and no metadata",
		src:     "\ufeff# Heading\n",
		outcome: FrontmatterAbsent,
	},
	{
		// CRLF normalizes to LF first, so the closing fence line is
		// "\n---\n" and not "\r\n---\r\n". A splitter that skips the
		// normalization finds nothing, and one that finds the block anyway
		// hands back a body still carrying raw CR bytes.
		name:    "CRLF line endings",
		src:     "---\r\nstatus: Open\r\n---\r\nBody.\r\n",
		front:   "---\nstatus: Open\n---\n",
		outcome: FrontmatterFound,
	},
	{
		// A lone CR is a line ending to remark and not to goldmark, which
		// is why the normalization covers it too.
		name:    "lone CR line endings",
		src:     "---\rstatus: Open\r---\rBody.\r",
		front:   "---\nstatus: Open\n---\n",
		outcome: FrontmatterFound,
	},
	{
		// An indented fence is not a fence. It opens the convention (there
		// is a later "---" line), so it is malformed rather than absent:
		// the bytes stay in the body and a diagnostic fires.
		name:    "an indented opening fence",
		src:     "  ---\nstatus: Open\n---\nBody.\n",
		outcome: FrontmatterMalformed,
	},
	{
		name:    "trailing text on the closing fence",
		src:     "---\nstatus: Open\n---0\n",
		outcome: FrontmatterMalformed,
	},
	{
		name:    "no newline after the closing fence",
		src:     "---\nstatus: Open\n---",
		outcome: FrontmatterMalformed,
	},
	{
		// A leading "---" with no second fence is an ordinary thematic
		// break, so nothing is extracted and nothing is reported.
		name:    "a thematic break that looks like an opening fence",
		src:     "---\n\nBody.\n",
		outcome: FrontmatterAbsent,
	},
	{
		name:    "a lone fence line",
		src:     "---\n",
		outcome: FrontmatterAbsent,
	},
	{
		name:    "an unterminated block",
		src:     "---\nstatus: Open\n",
		outcome: FrontmatterAbsent,
	},
	{
		name:    "an empty document",
		src:     "",
		outcome: FrontmatterAbsent,
	},
}

// normalizedSource is the contract Front+Body partitions: the source with
// its line endings normalized and any leading byte order mark peeled. It is
// written out here rather than taken from the implementation, because the
// point of the assertion is that the exported split still keeps this
// promise.
func normalizedSource(src string) string {
	s := strings.TrimPrefix(src, markdown.ByteOrderMark)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// facadeFront returns the metadata block FromMarkdown kept, read back off
// the tree: a found block is the leading ast.Frontmatter node's value, and
// anything else leaves no such node.
func facadeFront(root *ast.Root) string {
	if len(root.Children) == 0 {
		return ""
	}
	if fm, ok := root.Children[0].(*ast.Frontmatter); ok {
		return fm.Value
	}
	return ""
}

// facadeBodyRender renders everything the parse produced EXCEPT a leading
// frontmatter node, which is the half SplitFrontmatter calls Body.
func facadeBodyRender(root *ast.Root) string {
	body := *root
	if facadeFront(root) != "" {
		body.Children = root.Children[1:]
	}
	// The mark rides ast.Root and not the body, and a parse of Body alone
	// has none, so clear it on the copy or every marked row diverges on a
	// byte that is not part of either body.
	body.ByteOrderMark = false
	return ToMarkdown(&body)
}

func TestSplitFrontmatterAgreesWithTheFacadeParse(t *testing.T) {
	for _, tc := range splitEdgeCases {
		t.Run(tc.name, func(t *testing.T) {
			split := SplitFrontmatter(tc.src)
			assertSplitMatchesTheTable(t, tc.src, tc.front, tc.outcome, split)
			assertSplitMatchesTheParse(t, tc.src, split)
		})
	}
}

// assertSplitMatchesTheTable checks the split against the row: what the
// shape MEANS, and the Front+Body partition that says nothing was dropped
// or added.
func assertSplitMatchesTheTable(t *testing.T, src, front string, outcome FrontmatterOutcome, split FrontmatterSplit) {
	t.Helper()
	if split.Outcome != outcome {
		t.Errorf("outcome = %d, want %d", split.Outcome, outcome)
	}
	if split.Front != front {
		t.Errorf("Front = %q, want %q", split.Front, front)
	}
	if got, want := split.Found(), outcome == FrontmatterFound; got != want {
		t.Errorf("Found() = %v, want %v", got, want)
	}
	if got, want := split.Front+split.Body, normalizedSource(src); got != want {
		t.Errorf("Front+Body = %q, want the normalized source %q", got, want)
	}
	if got, want := split.ByteOrderMark, strings.HasPrefix(src, markdown.ByteOrderMark); got != want {
		t.Errorf("ByteOrderMark = %v, want %v", got, want)
	}
}

// assertSplitMatchesTheParse is the agreement half: the facade parse of the
// SAME source must have split it the same way.
func assertSplitMatchesTheParse(t *testing.T, src string, split FrontmatterSplit) {
	t.Helper()
	root, ok := FromMarkdown(src).(*ast.Root)
	if !ok {
		t.Fatal("FromMarkdown did not return an *ast.Root")
	}
	if got := facadeFront(root); got != split.Front {
		t.Errorf("FromMarkdown kept the block %q; SplitFrontmatter reported %q", got, split.Front)
	}
	if root.ByteOrderMark != split.ByteOrderMark {
		t.Errorf("FromMarkdown recorded ByteOrderMark=%v; SplitFrontmatter reported %v",
			root.ByteOrderMark, split.ByteOrderMark)
	}
	// And the body it parsed must be the body reported: rendering the tree
	// without its frontmatter node has to equal rendering a parse of Body
	// alone.
	if got, want := facadeBodyRender(root), ToMarkdown(FromMarkdown(split.Body)); got != want {
		t.Errorf("FromMarkdown parsed a different body:\nfrom the source %q\nfrom Body       %q", got, want)
	}
}

// TestSplitFrontmatterReportsTheSameMalformedNotice pins the one option
// besides the provider that the split reads: a malformed block reports the
// diagnostic FromMarkdown reports, so a consumer already classifying
// diagnostic codes needs no second rule for the split.
func TestSplitFrontmatterReportsTheSameMalformedNotice(t *testing.T) {
	codes := func(run func(sink func(convert.Diagnostic))) []string {
		var out []string
		run(func(d convert.Diagnostic) { out = append(out, d.Code) })
		return out
	}
	for _, tc := range splitEdgeCases {
		t.Run(tc.name, func(t *testing.T) {
			fromSplit := codes(func(sink func(convert.Diagnostic)) {
				SplitFrontmatter(tc.src, WithDiagnostics(sink))
			})
			fromParse := codes(func(sink func(convert.Diagnostic)) {
				FromMarkdown(tc.src, WithDiagnostics(sink))
			})
			wantMalformed := tc.outcome == FrontmatterMalformed
			if got := len(fromSplit) == 1 && fromSplit[0] == convert.CodeMalformedFrontmatter; got != wantMalformed {
				t.Errorf("SplitFrontmatter reported %v, want malformed-frontmatter = %v", fromSplit, wantMalformed)
			}
			// The parse can add notices of its own (a recovered parse, a
			// depth cap), so the comparison is over this one code.
			has := func(cs []string) bool {
				return slices.Contains(cs, convert.CodeMalformedFrontmatter)
			}
			if has(fromSplit) != has(fromParse) {
				t.Errorf("split reported %v and the parse reported %v", fromSplit, fromParse)
			}
		})
	}
}

// TestSplitFrontmatterRunsTheConfiguredProvider proves the export is not
// hardwired to the YAML default: a caller-defined convention splits the
// same way through both entry points.
func TestSplitFrontmatterRunsTheConfiguredProvider(t *testing.T) {
	provider := func(md string) (string, string, FrontmatterOutcome) {
		if rest, ok := strings.CutPrefix(md, "<!-- meta -->\n"); ok {
			return "<!-- meta -->\n", rest, FrontmatterFound
		}
		return "", md, FrontmatterAbsent
	}
	const src = "<!-- meta -->\nBody.\n"

	split := SplitFrontmatter(src, WithFrontmatterProvider(provider))
	if !split.Found() || split.Front != "<!-- meta -->\n" || split.Body != "Body.\n" {
		t.Fatalf("SplitFrontmatter = %+v, want the comment header split off", split)
	}
	// Without the option the same source has no metadata at all, so the row
	// above cannot pass by accident.
	if plain := SplitFrontmatter(src); plain.Found() {
		t.Errorf("the YAML default found a block in %q: %+v", src, plain)
	}

	root, ok := FromMarkdown(src, WithFrontmatterProvider(provider)).(*ast.Root)
	if !ok {
		t.Fatal("FromMarkdown did not return an *ast.Root")
	}
	if got := facadeFront(root); got != split.Front {
		t.Errorf("FromMarkdown kept %q; SplitFrontmatter reported %q", got, split.Front)
	}
}
