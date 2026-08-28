package adfast

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	ast2 "github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/markdown"
)

// The formatter's contract, replacing the structural guarantee the old
// md→adf→md composition provided (FormatMarkdown is now pure md→ast→md):
//
//	(a) SEMANTIC COHERENCE — formatting never changes meaning:
//	    mdToADF(fmtMD(md)) marshals to the same ADF as
//	    mdToADF(md), byte-identical after merging adjacent
//	    same-mark text runs (the one representation difference a
//	    reparse introduces; ADF treats the two shapes as the same
//	    document).
//	(b) IDEMPOTENCE — FormatMarkdown∘FormatMarkdown == FormatMarkdown.
//
// Both run over the directive fixture corpus and the fuzz seeds here,
// and continuously as the FuzzFormatSemanticsPreserved target. The
// md⇄ast⇄adf edges themselves are pinned by the fixture corpus tests
// (both directions), the ADF lossless suite, and
// FuzzRoundTripIdempotent.

// formatContractInputs collects the corpus the contract tests run over:
// every directive-fixture markdown entry (source, its reference
// round-trip, and the ADF-derived renders) plus the curated fuzz seeds.
func formatContractInputs(t *testing.T) map[string]string {
	t.Helper()
	inputs := map[string]string{}
	fixtures := loadDirectiveFixtures(t)
	for i, f := range fixtures.Markdown {
		inputs["md/"+strconv.Itoa(i)] = f.Md
		inputs["roundtrip/"+strconv.Itoa(i)] = f.Roundtrip
	}
	for i, f := range fixtures.Adf {
		inputs["adf/"+strconv.Itoa(i)] = f.Md
	}
	for i, s := range fuzzSeeds {
		inputs["seed/"+strconv.Itoa(i)] = s
	}
	return inputs
}

func marshalADF(t *testing.T, md string) string {
	t.Helper()
	b, err := json.Marshal(mdToADF(md))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return canonicalizeAdjacentText(t, b)
}

// canonicalizeAdjacentText merges consecutive text nodes carrying
// identical marks — the one representation difference formatting may
// introduce: reparsing formatted output coalesces text runs the source
// parse had split (e.g. around an unknown inline directive's flattened
// name), which ADF treats as the same document.
func canonicalizeAdjacentText(t *testing.T, raw []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	mergeTextRuns(v)
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return string(b)
}

func mergeTextRuns(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	content, ok := m["content"].([]any)
	if !ok {
		return
	}
	var out []any
	for _, child := range content {
		mergeTextRuns(child)
		if prev, prevText, curText, ok := mergeableTexts(out, child); ok {
			prev["text"] = prevText + curText
			continue
		}
		out = append(out, child)
	}
	m["content"] = out
}

// mergeableTexts reports whether child continues the previous text node
// with identical marks, returning both text values.
func mergeableTexts(out []any, child any) (prev map[string]any, prevText, curText string, ok bool) {
	if len(out) == 0 {
		return nil, "", "", false
	}
	prev, pOK := out[len(out)-1].(map[string]any)
	cur, cOK := child.(map[string]any)
	if !pOK || !cOK || prev["type"] != "text" || cur["type"] != "text" {
		return nil, "", "", false
	}
	prevText, pStr := prev["text"].(string)
	curText, cStr := cur["text"].(string)
	if !pStr || !cStr {
		return nil, "", "", false
	}
	pm, pErr := json.Marshal(prev["marks"])
	cm, cErr := json.Marshal(cur["marks"])
	if pErr != nil || cErr != nil || !bytes.Equal(pm, cm) {
		return nil, "", "", false
	}
	return prev, prevText, curText, true
}

// TestFormatSemanticCoherence_Corpus: formatting never changes document
// meaning — the canonical ADF of the formatted document equals the
// canonical ADF of the original, byte for byte.
func TestFormatSemanticCoherence_Corpus(t *testing.T) {
	for name, md := range formatContractInputs(t) {
		formatted := fmtMD(md)
		if got, want := marshalADF(t, formatted), marshalADF(t, md); got != want {
			t.Errorf("%s: FormatMarkdown changed meaning\n in:        %q\n formatted: %q\n adf(fmt):  %s\n adf(src):  %s", name, md, formatted, got, want)
		}
	}
}

// TestFormatIdempotence_Corpus: formatting a formatted document is a
// no-op.
func TestFormatIdempotence_Corpus(t *testing.T) {
	for name, md := range formatContractInputs(t) {
		once := fmtMD(md)
		if twice := fmtMD(once); twice != once {
			t.Errorf("%s: FormatMarkdown not idempotent\n in:    %q\n once:  %q\n twice: %q", name, md, once, twice)
		}
	}
}

// A hard break whose two-space marker would land at a line start keeps
// its meaning only in the backslash form: nothing precedes the spaces,
// so they are the line's leading whitespace and are stripped on
// re-parse. The formatter otherwise keeps the source's trailing-space
// form. See markdown.writeHardBreak.
func TestHardBreakAtLineStartKeepsItsMeaning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: the emoji has no shortName to write, so
			// the break is the paragraph's first rendered content.
			name: "after a directive that renders nothing",
			md:   ":emoji  \n0",
			want: "\\\n0\n",
		},
		{
			// With content before it the source form is kept.
			name: "mid-line break keeps the source form",
			md:   "x  \n0",
			want: "x  \n0\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// Markdown cannot write two code spans back to back — the closing fence
// of the first and the opening fence of the second are one backtick run
// to the parser — so the renderer joins their content. The adjacency is
// reachable because the code mark is exclusive, which drops an emphasis
// wrapping nothing but a code span. See markdown.joinAdjacentCodeSpans.
func TestAdjacentCodeSpansAreJoined(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: both spans hold a lone space, and the
			// joined span keeps both (`  ` is all-space, so CommonMark
			// strips nothing).
			name: "spaces",
			md:   "` `*` `* 0",
			want: "`  ` 0\n",
		},
		{
			name: "words",
			md:   "`a`*`b`* 0",
			want: "`ab` 0\n",
		},
		{
			// remark's own bytes for the joined pair, "`0``0`", are the
			// bytes it writes for the single span 0``0 as well.
			name: "the ambiguous reference form stays readable",
			md:   "`0``0`",
			want: "`0``0`\n",
		},
		{
			// Different links keep the spans apart: the label and target
			// bytes stand between the fences.
			name: "separated by link syntax",
			md:   "[`a`](x)[`b`](y)",
			want: "[`a`](x)[`b`](y)\n",
		},
		{
			// An emphasis with other content keeps its markers, and they
			// separate the fences.
			name: "emphasis with more than the code span",
			md:   "`a`*x`b`*",
			want: "`a`_x`b`_\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// The formatter adds no escapes of its own — it writes back the source
// form the parse captured — so a '[' or '_' that the source left bare
// still fuses onto a preceding directive name. The empty attribute block
// terminates the name there, the same repair the renderer applies to the
// unescapable continuations. See markdown.needsPunctTrail.
func TestDirectiveBeforeUnescapedSyntaxIsTerminated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: the label cannot span the line break, so
			// the brackets are text — until the formatter joins the
			// lines and the directive swallows them as its label.
			name: "bracket across a soft break",
			md:   ":media[\n]",
			want: ":media{}[ ]\n",
		},
		{
			// prettier drops the source's '[' escape, so the terminator
			// is what keeps the brackets out of the directive.
			name: "escaped bracket loses its escape",
			md:   ":media\\[x]",
			want: ":media{}[x]\n",
		},
		{
			// The trailing '_' keeps this out of the directive grammar
			// entirely: the formatter parse hands the renderer one plain
			// text node, ":media_x_". Written bare it would re-parse as a
			// directive, so the colon escapes (see markdown.escapesColon)
			// and no terminator is involved.
			name: "intraword underscore",
			md:   ":media_x_",
			want: "\\:media_x\\_\n",
		},
		{
			// A space separates the name from the marker; no repair.
			name: "separated by a space",
			md:   ":media _x_",
			want: ":media _x_\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// A ':' after a directive is a hazard the renderer already has an answer
// for: the colon escape (see markdown.escapesColon) writes "\:" wherever
// the colon leads into a name, which separates the two on re-parse. Adding
// the empty attribute block on top of that escape is not merely redundant,
// it is unstable — the block goes out on the first format and not on the
// second, because the escape it duplicates is by then part of the text the
// second parse reads back (":media[]:A" formatted to ":media{}\:A" and
// then to ":media\:A"). The terminator is for the colons the escape
// declines. See markdown.needsPunctTrail.
func TestDirectiveBeforeColonLeansOnTheColonEscape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: an empty label writes no brackets, so the
			// form ends in its name and the following ":A" reaches it.
			name: "colon into a letter-led name",
			md:   ":media[]:A",
			want: ":media\\:A\n",
		},
		{
			// The source's own empty attribute block is not a terminator
			// the renderer keeps: it carries no attributes, so the same
			// decision is made from scratch.
			name: "empty attribute block in the source",
			md:   ":media{}:A",
			want: ":media\\:A\n",
		},
		{
			// The prose escape covers only a letter-led name, so a
			// digit-led one is left bare and the terminator is what keeps
			// the directive from butting into it.
			name: "colon into a digit-led name",
			md:   ":media[]:9",
			want: ":media{}:9\n",
		},
		{
			name: "colon at the end of the paragraph",
			md:   ":media[]:",
			want: ":media{}:\n",
		},
		{
			// The escape belongs to a text node, and this colon opens a
			// directive instead — nothing escapes it, so the terminator
			// stands.
			name: "colon that opens the next directive",
			md:   ":media[]:media[x]",
			want: ":media{}:media[x]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// TestDirectiveLabelIndentStaysOutOfCode pins the character reference
// that keeps a text-directive label from opening as an indented code
// block, where escapes stay literal and grow a backslash per format.
func TestDirectiveLabelIndentStaysOutOfCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: the label's four leading spaces made the
			// escape literal, so each pass escaped the survivor again.
			name: "four spaces",
			md:   "00:u[    *]0",
			want: "00:u[&#x20;   \\*]0\n",
		},
		{
			// Already indented code in the source: the backslash is
			// content, and the reference keeps it that way.
			name: "four spaces before an escape",
			md:   ":u[    \\*]",
			want: ":u[&#x20;   \\\\\\*]\n",
		},
		{
			// A tab reaches the indent on its own.
			name: "leading tab",
			md:   ":u[\t\\*]",
			want: ":u[&#x9;\\\\\\*]\n",
		},
		{
			// Two spaces then a tab: the tab advances to column 4.
			name: "spaces then a tab",
			md:   ":u[  \t\\*]",
			want: ":u[&#x20; \t\\\\\\*]\n",
		},
		{
			// Three spaces stop short of the indent; no repair.
			name: "three spaces",
			md:   ":u[   \\*]",
			want: ":u[   \\*]\n",
		},
		{
			// The run has to lead the label to indent it.
			name: "run after content",
			md:   ":u[a   \\*]",
			want: ":u[a   \\*]\n",
		},
		{
			// Leaf labels read back through ast.PlainText over inline
			// content, which resolves the escape; nothing to repair.
			name: "leaf label",
			md:   "::media[    \\*]{url=\"x\"}",
			want: "::media[    *]{url=\"x\"}\n",
		},
		{
			name: "container label",
			md:   ":::expand[    \\*]\nx\n:::",
			want: ":::expand[    *]\nx\n:::\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// TestBlankCodeSpanKeepsItsPadding pins the code-span pad against
// goldmark's trim rule: whitespace-only content is left alone, because
// padding it would grow a space per format pass.
func TestBlankCodeSpanKeepsItsPadding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: content that is blank to goldmark (a tab
			// counts) is never trimmed, so it needs no pad.
			name: "spaces around a tab",
			md:   "` \t `0",
			want: "` \t `0\n",
		},
		{
			name: "wider blank run",
			md:   "`  \t  `0",
			want: "`  \t  `0\n",
		},
		{
			name: "only spaces",
			md:   "`  `0",
			want: "`  `0\n",
		},
		{
			// Not blank: the trim applies, so the pad has to survive it.
			name: "spaces around content",
			md:   "`  a  `0",
			want: "`  a  `0\n",
		},
		{
			// One space each side is the pad itself; the value is "a".
			name: "padded content",
			md:   "` a `0",
			want: "`a`0\n",
		},
		{
			// Only one edge is a space: nothing is trimmed.
			name: "leading space only",
			md:   "` a`0",
			want: "` a`0\n",
		},
		{
			// A backtick edge is padded whatever the trim does.
			name: "backtick content",
			md:   "`` ` ``0",
			want: "`` ` ``0\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// TestURLLiteralEndsWhereTheParserEndsIt pins adfast's own re-linkification
// (relinkifyTexts, for text goldmark skipped inside a link label) to the
// boundary goldmark's linkify parser gives a literal: its regexp match
// minus the trailing punctuation the parser strips. Each case pairs a
// source that takes the relinkify path with the plain one that goldmark
// linkifies; both must reach the same ADF.
func TestURLLiteralEndsWhereTheParserEndsIt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: the escaped '/' suppressed goldmark's
			// linkify, and relinkify kept the '!' the parser strips.
			name: "escaped slash, trailing bang",
			md:   "http:\\//0.a#!",
			want: "http://0.a#!\n",
		},
		{
			name: "trailing bang in a label",
			md:   "[ http://0.a#!",
			want: "[ http://0.a#!\n",
		},
		{
			name: "trailing dot in a label",
			md:   "[ http://0.a#/x.",
			want: "[ http://0.a#/x.\n",
		},
		{
			name: "trailing punctuation run in a label",
			md:   "[ http://0.a#*_~",
			want: "[ http://0.a#\\*\\_~\n",
		},
		{
			// Unbalanced closing parens are not part of the link; a
			// balanced pair is.
			name: "unbalanced parens in a label",
			md:   "[ http://0.a#/x))",
			want: "[ http://0.a#/x))\n",
		},
		{
			name: "balanced parens in a label",
			md:   "[ http://0.a#/x(y)",
			want: "[ http://0.a#/x(y)\n",
		},
		{
			// A trailing "&…;" is a character reference, not link text.
			name: "entity after a label URL",
			md:   "[ http://0.a#/x&y;",
			want: "[ http://0.a#/x&y;\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// TestEmailLiteralStaysUnlinkedAcrossFormat pins the escape the formatter
// writes where its own output would otherwise re-parse as a GFM email
// autolink literal. Prettier, whose text escaping the formatter mirrors,
// has no autolink literals and would leave the '@' bare; adfast escapes it
// against the parser it round-trips against (see markdown.linkifiesAsEmail).
func TestEmailLiteralStaysUnlinkedAcrossFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: the empty ':u' normalizes away and leaves the
			// local part beside the domain, fusing into one literal.
			name: "fused across a dropped directive",
			md:   "0@A:u.A",
			want: "0\\@A.A\n",
		},
		{
			name: "fused after a word",
			md:   "x a@b:u.com",
			want: "x a\\@b.com\n",
		},
		{
			name: "fused inside parentheses",
			md:   "(a@b:u.com)",
			want: "(a\\@b.com)\n",
		},
		{
			// The local part runs out of the node and into the emphasis
			// closer, which the linkify scan reads as an address byte.
			name: "fused with an emphasis closer",
			md:   "*a*@b:u.com",
			want: "_a_\\@b.com\n",
		},
		{
			// An address opening on punctuation never linkifies, so the
			// '@' stays bare.
			name: "candidate opens on punctuation",
			md:   ".a@b:u.com",
			want: ".a@b.com\n",
		},
		{
			// No dot in the domain: not a literal.
			name: "no domain dot",
			md:   "a@b:u",
			want: "a@b\n",
		},
		{
			// A code span ends in a backtick, which is neither an address
			// byte nor a linkify trigger.
			name: "after a code span",
			md:   "`a@b`:u.com",
			want: "`a@b`.com\n",
		},
		{
			// Inside a link label the text is atomic: no literal forms.
			name: "inside a link label",
			md:   "[a@b:u.com](x)",
			want: "[a@b.com](x)\n",
		},
		{
			// A real autolink literal keeps its link mark and renders in
			// the explicit form.
			name: "genuine autolink literal",
			md:   "a@b.com",
			want: "[a@b.com](a@b.com)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// TestEscapeProvenanceSurvivesAURLSplit pins the Value ↔ Raw alignment the
// re-linkifier relies on when it cuts a URL literal out of a text node that
// also carries preserved escapes: only PreservedEscapes stand undecoded in
// Raw, so a literal backslash pair must not read as one escape (see
// markdown.rawEscapeAt).
func TestEscapeProvenanceSurvivesAURLSplit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			// The fuzz repro: "\\" and the escape-less "\0" put two
			// literal backslashes in the value ahead of the '\+' the
			// formatter keeps, so the walk over Raw desynchronized and the
			// text kept the literal's first byte.
			name: "literal backslash pair before a preserved escape",
			md:   "\\\\\\0+\\+\\(www.0.a0",
			want: "\\\\\\0+\\+([www.0.a](http://www.0.a)0\n",
		},
		{
			name: "preserved escape alone",
			md:   "\\+\\(www.0.a0",
			want: "\\+([www.0.a](http://www.0.a)0\n",
		},
		{
			name: "literal backslash alone",
			md:   "\\\\\\(www.0.a0",
			want: "\\\\([www.0.a](http://www.0.a)0\n",
		},
		{
			// Every preserved escape, each standing for one value byte.
			name: "all preserved escapes",
			md:   "\\~\\:\\-\\+ www.0.a0",
			want: "\\~\\:\\-\\+ [www.0.a](http://www.0.a)0\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tt.md)
			if got != tt.want {
				t.Fatalf("format = %q, want %q", got, tt.want)
			}
			if twice := fmtMD(got); twice != got {
				t.Fatalf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tt.md); adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}

// FuzzFormatSemanticsPreserved fuzzes the two contract properties: the
// formatted document keeps the source's canonical ADF (semantic
// coherence) and formats to itself (idempotence). Skip classes mirror
// FuzzRoundTripIdempotent — content that cannot occur in host documents
// or where the reference pipeline (prettier wrap) is equally unstable.
//
// The skip ladder lives in the skipFmt* classifiers below, grouped by the
// stage a class is visible at: the parsed document, the source bytes, and
// the formatted output. Each classifier returns the FIRST documented
// class that matches, so a skip always names the reason it fired, and the
// groups run in that stage order so that no group inspects a stage the
// input has not reached.
func FuzzFormatSemanticsPreserved(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if reason, skip := skipRawInput(md); skip {
			t.Skip(reason)
		}
		normalizedMd := strings.ReplaceAll(strings.ReplaceAll(md, "\r\n", "\n"), "\r", "\n")
		doc := mdToADF(md)
		for _, classify := range []func(adf.Doc) (string, bool){
			skipDocClasses,
			skipDocLinkClasses,
			skipFmtWhitespaceClasses,
			skipFmtStructureClasses,
			skipFmtEscapeClasses,
			skipFmtDelimiterClasses,
			skipFmtDirectiveTextClasses,
			skipFmtLinkClasses,
		} {
			if reason, skip := classify(doc); skip {
				t.Skip(reason)
			}
		}
		if reason, skip := skipFmtSourceClasses(md, normalizedMd); skip {
			t.Skip(reason)
		}
		formatted := fmtMD(md)
		for _, classify := range []func(string) (string, bool){
			skipRenderedClasses,
			skipFmtRenderedEscapeClasses,
			skipFmtRenderedLineClasses,
		} {
			if reason, skip := classify(formatted); skip {
				t.Skip(reason)
			}
		}
		// Frontmatter detection no longer diverges between the directions:
		// both md→adf and the formatter share one FrontmatterProvider, so a
		// malformed block is treated as body identically on each side (the
		// former lenient-vs-exact splitter residual is gone). No skip needed.
		if got, want := marshalADF(t, formatted), marshalADF(t, md); got != want {
			t.Errorf("format changed meaning for %q:\nformatted: %q\n adf(fmt): %s\n adf(src): %s", md, formatted, got, want)
		}
		if twice := fmtMD(formatted); twice != formatted {
			t.Errorf("format not idempotent for %q:\nonce:  %q\ntwice: %q", md, formatted, twice)
		}
	})
}

// skipFmtWhitespaceClasses reports document classes whose whitespace does
// not survive the formatter's trimming and prose wrapping.
func skipFmtWhitespaceClasses(doc adf.Doc) (reason string, skip bool) {
	// The formatter deliberately trims trailing whitespace on code
	// lines (prettier parity; see the "code fence trailing space
	// trimmed" pin), which IS a code-text change — the one sanctioned
	// semantic edit.
	if hasCodeTrailingWhitespace(doc) {
		return "trailing whitespace in code lines; the formatter trims it by design", true
	}
	// A tab inside prose can sit at a wrap point, become a newline,
	// and collapse to a space on re-parse; prettier is equally
	// unstable on this class (tabs inside code spans are fine).
	if docHasProseTab(doc) {
		return "tab in prose; the reference pipeline is equally unstable under wrapping", true
	}
	// An interior space run in text (e.g. left by a degraded
	// directive label) collapses under prose wrapping; remark wraps
	// identically.
	if docHasDoubleSpaceText(doc) {
		return "interior space run in text; wrapping collapses it", true
	}
	// Trailing whitespace at the end of a block's inline run is
	// trimmed by the prose renderer (a decoded "&#XA;" can leave one
	// behind); pre-existing trim behavior.
	if docHasTrailingSpaceRun(doc) {
		return "trailing whitespace in a block's inline run; the renderer trims it", true
	}
	// More than two trailing spaces before a hard break leave a
	// space-suffixed text node; re-parse absorbs the extra spaces
	// into the break marker ("0    \n" → "0   \n" → "0  \n") — a
	// pre-existing quirk of the space-form hard-break preservation.
	if docHasSpaceBeforeHardBreak(doc) {
		return "extra spaces before a hard break; pre-existing space-form break quirk", true
	}
	return "", false
}

// skipFmtStructureClasses reports document classes whose block structure
// or marker shape shifts between rounds.
func skipFmtStructureClasses(doc adf.Doc) (reason string, skip bool) {
	// An empty list item's rendered marker line interacts with
	// indented-code formation in nested lists between rounds;
	// pre-existing renderer corner.
	if docHasEmptyListItem(doc) {
		return "empty list item; pre-existing marker/indented-code corner", true
	}
	// An empty paragraph (e.g. holding only a dropped empty link)
	// renders as bare blank lines that vanish on re-parse; remark
	// renders the identical bytes.
	if docHasEmptyParagraph(doc) {
		return "empty paragraph; renders as blank lines that vanish on re-parse", true
	}
	// An empty code block inside a list item collapses its trailing
	// blank line on re-parse, flipping the following block's gap;
	// pre-existing spacing quirk.
	if docHasEmptyCodeBlock(doc) {
		return "empty code block; pre-existing list-gap quirk", true
	}
	// A soft-break collapse can butt a literal "[x]" against its list
	// marker, manufacturing task syntax on re-parse ("* [X]\n0" →
	// "- [X] 0"); remark renders the identical bytes and is equally
	// unstable.
	if docHasCheckboxLeadText(doc) {
		return "task marker formed by soft-break collapse; remark is equally unstable", true
	}
	// Paragraph text leading with a list-marker character ("* --" ->
	// item text "--") collides with the renderer's marker-collision
	// rewriting and re-parses differently; pre-existing renderer
	// behavior on this class.
	if docHasMarkerLeadParagraph(doc) {
		return "marker character leading paragraph text; pre-existing marker-collision rewriting", true
	}
	// Emoji projection is deliberately lossy across markdown
	// persistence (see convert's VisitEmoji): a known shortname
	// renders as its unicode text, shedding the emoji node.
	if docHasEmoji(doc) {
		return "emoji node; the markdown projection is deliberately lossy", true
	}
	// ::media without a layout attribute converts to a plain image,
	// whose re-encode materializes the default layout="center" — the
	// pre-existing media⇄image default-layout asymmetry (identical
	// before the md→ast→md rewrite).
	if docHasLayoutlessMediaSingle(doc) {
		return "layout-less mediaSingle; image re-encode materializes the default layout", true
	}
	return "", false
}

// skipFmtEscapeClasses reports document classes the prettier escape model
// does not carry back through a re-parse.
func skipFmtEscapeClasses(doc adf.Doc) (reason string, skip bool) {
	// A literal backslash directly before a preserved-escape
	// character ("\\+" → text "\+") collides with the formatter's
	// preserved-escape channel (markdown.PreservedEscapes) and sheds
	// one escape level on re-parse — a pre-existing quirk of the
	// prettier escape model, byte-identical before the md→ast→md
	// rewrite.
	if docHasBackslashBeforePreserved(doc) {
		return "backslash before a preserved-escape character; pre-existing escape-channel collision", true
	}
	// A literal backslash before a space can end up at a wrap point,
	// where the line-trailing backslash re-parses as a hard break;
	// the reference pipeline wraps identically.
	if docHasBackslashSpaceText(doc) {
		return "backslash before a space; wrap can turn it into a hard break", true
	}
	// A text run ending in a literal backslash escapes whatever
	// delimiter or character follows it on re-parse ("*0\\\\*" ->
	// "_0\\_"); remark is equally unstable on this class.
	if docHasTrailingBackslashText(doc) {
		return "text ending in a literal backslash; remark is equally unstable", true
	}
	// A character-reference-shaped token in plain text ("&quot;",
	// its '&' escaped in the source) renders bare and decodes on
	// re-parse; pre-existing escape gap.
	if docHasEntityShapedText(doc) {
		return "character-reference-shaped token in text; pre-existing escape gap", true
	}
	// An email-shaped token in plain text (its '@' was escaped in the
	// source, which the render does not reproduce) gets linkified on
	// re-parse ("0\\@0.A" -> "0@0.A"); pre-existing escape gap.
	if docHasEmailShapedText(doc) {
		return "email-shaped token in plain text; pre-existing escape gap", true
	}
	// A literal '!' rendered directly before a [label](url) link
	// turns it into an image on re-parse ("\\![0]()" -> "![0]()");
	// remark escapes this, the prettier rules do not — pre-existing.
	if docHasBangBeforeLink(doc) {
		return "'!' before a link; re-parses as an image", true
	}
	// A '<' in prose interacts with wrapping and the HTML-atom
	// handling in unstable ways: emphasis drops around HTML atoms
	// ("*0<A>*" -> "_0_<A>"), and a wrap point inside an HTML-ish
	// token flips the escape decision between rounds; pre-existing.
	if docHasAngleText(doc) {
		return "'<' in prose; pre-existing HTML-atom and escape instabilities", true
	}
	return "", false
}

// skipFmtDelimiterClasses reports document classes where a literal
// character merges with the emphasis or strike delimiters around it.
func skipFmtDelimiterClasses(doc adf.Doc) (reason string, skip bool) {
	// Literal tildes in or next to strike content merge with the ~~
	// delimiters on re-parse ("0~0~~0~" → "0~~0~~0~~"); remark is
	// equally unstable on this class.
	if docHasLiteralTilde(doc) {
		return "literal tilde in text; strike-delimiter flanking is unstable on this class", true
	}
	// A literal underscore inside emphasis content closes the _
	// delimiters early on re-parse ("0*00_0*0000"); pre-existing
	// renderer instability of the underscore emphasis form.
	if docHasUnderscoreInEmphasis(doc) {
		return "underscore inside emphasis content; pre-existing renderer instability", true
	}
	// Emphasis directly before a code span triggers the formatter's
	// mark re-inference across code boundaries (ported verbatim from
	// the ADF decode, where code spans genuinely shed their marks),
	// which can extend the emphasis over following unmarked text
	// (probe: "*0`0`* 0" → "_0`0` 0_") — pre-existing behavior.
	if docHasEmphasisBeforeCode(doc) {
		return "emphasis adjoining a code span; pre-existing mark re-inference", true
	}
	return "", false
}

// skipFmtDirectiveTextClasses reports document classes where plain text
// assembles into directive syntax on re-parse. Labels are written
// verbatim by design, so a colon-name token inside one re-reads as a
// nested directive.
func skipFmtDirectiveTextClasses(doc adf.Doc) (reason string, skip bool) {
	// A ":::" run inside plain text can end up alone on a wrapped
	// line and re-parse as a container-directive fence; the reference
	// pipeline wraps identically.
	if docHasTextColonFence(doc) {
		return "container-fence token in text; wrap can isolate it into a fence line", true
	}
	// Text ending in ':' directly before a typed inline directive
	// doubles the colon in the render, promoting the token to a LEAF
	// directive on re-parse (":*:media*" -> "::media{…}");
	// pre-existing gluing behavior.
	if docHasColonBeforeInline(doc) {
		return "colon before an inline directive; re-parses as a leaf directive", true
	}
	// A colon-name token inside emphasis content can absorb the
	// closing marker on re-parse (directive names allow '_'/'-':
	// "*0:A0*-0" -> "_0:A0_-0" reads as directive "A0_-0");
	// pre-existing label parsing.
	if docHasColonNameInEmphasis(doc) {
		return "colon-name token inside emphasis; the name can absorb the closing marker", true
	}
	// A colon-name token inside a styled (mark-directive) label
	// re-parses as a nested directive and splits the wrapper
	// (":u[0:A0]" -> ":u[0]:u[:A0]"); pre-existing label parsing.
	if docHasColonNameInStyledText(doc) {
		return "colon-name token in a mark-directive label; pre-existing label parsing", true
	}
	// A colon inside a typed inline directive's rendered label
	// (":placeholder[:0]") can derail the whole directive on
	// re-parse; the reference pipeline degrades identically.
	if docHasColonInInlineLabel(doc) {
		return "colon in an inline directive label; the reference pipeline is equally unstable", true
	}
	return "", false
}

// skipFmtLinkClasses reports document classes where a link's label or
// destination does not survive the prettier link forms.
func skipFmtLinkClasses(doc adf.Doc) (reason string, skip bool) {
	// A link destination containing parentheses renders bare in the
	// prettier URL form (angle brackets only wrap on space or ')'),
	// so unbalanced '(' breaks the link on re-parse ("[0](( )");
	// pre-existing prettier URL-form gap. The rendered half of this
	// class is parenDestRe in skipFmtRenderedLineClasses.
	if docHasParenInLinkHref(doc) {
		return "parenthesis in link destination; pre-existing prettier URL-form gap", true
	}
	// A colon-name token inside a link label re-parses as a text
	// directive and splits the link ("www.:A.a"); the label rules
	// deliberately write labels verbatim — pre-existing.
	if docHasColonNameInLinkLabel(doc) {
		return "colon-name token inside a link label; pre-existing label-escape gap", true
	}
	// A literal bracket inside a link label renders unescaped under
	// the prettier label rules and derails the [label](url) form on
	// re-parse; pre-existing prettier-escape gap.
	if docHasBracketInLinkLabel(doc) {
		return "bracket inside a link label; pre-existing prettier-escape gap", true
	}
	// A link-shaped token inside plain text ("[label](url)", e.g.
	// from an escaped bracket the parse decoded) is rendered verbatim
	// by the prettier text rules and re-parses as a real link;
	// pre-existing renderer behavior.
	if docHasLinkShapedText(doc) {
		return "link-shaped token in plain text; pre-existing prettier-escape gap", true
	}
	// A dropped construct's rejoin can glue preceding text directly
	// onto a bare autolink ("…]http://…" → "000http://…"), whose
	// linkification then fails on re-parse for lack of a word
	// boundary; remark degrades identically.
	if docHasBareLinkGluedToText(doc) {
		return "bare autolink glued to preceding text; remark is equally unstable", true
	}
	return "", false
}

// skipFmtSourceClasses reports classes recognized on the SOURCE bytes,
// before any render: content that the parse itself decodes or drops, so
// the formatted document can never carry it back. normalizedMd is md with
// its line endings folded to "\n"; md is the raw input, which the link
// title rules read verbatim.
func skipFmtSourceClasses(md, normalizedMd string) (reason string, skip bool) {
	// Ordered-marker numbers that are neither all-equal nor a strict
	// +1 progression flip the prettier increment inference between
	// rounds; pre-existing numbering-style inference limits.
	if hasIrregularOrderedNumbers(normalizedMd) {
		return "irregular ordered-list numbering; increment inference is not a fixpoint", true
	}
	// A wide gap after an ordered-list marker ("0)  0) 0") feeds the
	// prettier marker-gap alignment, which is not a fixpoint when
	// nested markers interact; pre-existing renderer quirk.
	if orderedGapMarkerRe.MatchString(normalizedMd) {
		return "wide ordered-marker gap; pre-existing gap-alignment quirk", true
	}
	// A link/image title containing a quote is written verbatim
	// between the renderer's own quotes and derails on re-parse
	// ('[0](0 (\"))' -> '[0](0 \"\"\")'); pre-existing title-escape gap.
	if mdHasQuirkyTitle(md) {
		return "quote or backslash inside a link title; pre-existing title-escape gap", true
	}
	// A bare directive attribute literally named "id" collides with
	// the {#id} shortcut and renders as "{}" (pre-existing
	// writeDirectiveAttrs corner), losing the payload on re-parse.
	if strings.Contains(normalizedMd, "{id}") {
		return "bare 'id' directive attribute; collides with the {#id} shortcut", true
	}
	// A character reference in the source decodes at parse time and
	// re-renders as raw bytes (an encoded newline becomes a soft
	// break, an encoded space collapses, …); the reference pipeline
	// degrades identically. Skipped by input shape.
	if strings.Contains(normalizedMd, "&#") {
		return "character reference in source; decodes to raw content", true
	}
	// An escaped quote in the source can sit inside a link/image
	// title, which the prettier renderer writes verbatim between its
	// own quotes; the title derails on re-parse. Skipped by input
	// shape.
	if strings.Contains(normalizedMd, "\\\"") {
		return "escaped quote; may sit inside a verbatim-rendered link title", true
	}
	// A link-reference-definition-shaped line ("[0]:0") drops in the
	// parse; inside a loose list the emptied item collapses the list
	// spacing between rounds. Skipped by input shape.
	if refDefShapedLineRe.MatchString(normalizedMd) {
		return "link-reference-definition-shaped line; drops and shifts list spacing", true
	}
	// A leaf-directive-shaped line ("::name…") that drops (unknown
	// name or invalid payload) can leave its neighbor blocks adjacent
	// in a tight list item, where they rejoin on re-parse; the
	// reference pipeline degrades identically. Skipped by input shape
	// (stable known directives lose fuzz coverage here, but the
	// corpus tests pin those).
	if leafDirectiveLineRe.MatchString(normalizedMd) {
		return "leaf-directive-shaped line; dropped directives shift tight-list joins", true
	}
	return "", false
}

// skipFmtRenderedEscapeClasses reports classes recognized on the
// FORMATTED bytes where an escape or character reference sits next to a
// token that changes meaning on re-parse.
func skipFmtRenderedEscapeClasses(formatted string) (reason string, skip bool) {
	// An underscore emphasis marker rendered adjacent to a literal
	// underscore merges into a longer delimiter run on re-parse
	// (probe: "0000 0*0000*_00" → "…&#x30;000__00"). A pre-existing
	// prettier-mode renderer instability, byte-identical before and
	// after the md→ast→md rewrite; conservatively skipped.
	if strings.Contains(formatted, "__") {
		return "underscore delimiter run in rendered output; pre-existing renderer instability", true
	}
	// A literal backslash rendered directly before a
	// flanking-hex-encoded rune escapes the reference's ampersand on
	// re-parse ("[*0\\0*000" -> "[_0\\&#x30;_..."); the same
	// reference-adjacency family as skipRenderedTokenClasses' "@&#x"
	// rule, pre-existing.
	if strings.Contains(formatted, "\\&#x") {
		return "backslash adjoining a character reference; pre-existing renderer instability", true
	}
	// A backslash escape rendered right after a directive-name-shaped
	// token terminates the name on re-parse, so a previously inert
	// token can become a known directive (":emoji_" -> ":emoji\\_" ->
	// dropped :emoji); the reference pipeline degrades identically.
	if nameThenEscapeRe.MatchString(formatted) || nameThenHexRe.MatchString(formatted) {
		return "escape after a directive-name-shaped token; the reference pipeline is equally unstable", true
	}
	// An '@' with a flanking-hex-encoded rune later in the same word
	// ("0@0.A&#x30;…") flips GFM email linkification between rounds —
	// the wider form of skipRenderedTokenClasses' "@&#x" rule.
	if emailHexRe.MatchString(formatted) {
		return "email-like token adjoining a character reference; remark is equally unstable", true
	}
	return "", false
}

// skipFmtRenderedLineClasses reports classes recognized on the FORMATTED
// bytes where a token's position on its line — set by wrapping, or by a
// neighbor that dropped — changes how the line re-parses.
func skipFmtRenderedLineClasses(formatted string) (reason string, skip bool) {
	// An emphasis marker directly adjoining an image opener flips the
	// marker's flanking classification between rounds ("_0_![…]");
	// pre-existing flanking instability.
	if emphasisImageRe.MatchString(formatted) {
		return "emphasis marker adjoining an image; pre-existing flanking instability", true
	}
	// An image alt holding directive syntax flattens differently in
	// the formatter (colon-name kept as text) than in the canonical
	// alt projection (plain text only); pre-existing alt-flattening
	// divergence.
	if imageAltColonRe.MatchString(formatted) {
		return "directive-ish token in image alt; pre-existing alt-flattening divergence", true
	}
	// A line-leading ']' in the render (a bracket split across a hard
	// break) can assemble a link-reference definition on re-parse;
	// remark renders the identical bytes.
	if bracketLeadLineRe.MatchString(formatted) {
		return "line-leading bracket in rendered output; may assemble a reference definition", true
	}
	// A bare "::name" token left alone on its line (e.g. after an
	// empty :u dropped behind it) re-parses as a leaf directive and
	// vanishes ("::0:u" -> "::0" -> ""); the reference pipeline
	// degrades identically.
	if bareLeafTokenLineRe.MatchString(formatted) {
		return "bare leaf-directive token line; the reference pipeline is equally unstable", true
	}
	// Inline HTML that lands at a line start after formatting (its
	// enclosing emphasis drops it to an unmarked atom) re-parses as
	// block-level HTML and is stripped ("*<A>*" -> "<A>");
	// pre-existing behavior of the HTML atom handling.
	if htmlLeadLineRe.MatchString(formatted) {
		return "line-leading inline HTML; re-parses as block HTML", true
	}
	// The rendered half of skipFmtLinkClasses' parenthesis class: an
	// unbalanced '(' in a bare-rendered link destination breaks the
	// link on re-parse.
	if parenDestRe.MatchString(formatted) {
		return "parenthesis in link destination; pre-existing prettier URL-form gap", true
	}
	return "", false
}

// hasCodeTrailingWhitespace reports whether any code block line ends in
// spaces or tabs (the content the formatter trims by design).
func hasCodeTrailingWhitespace(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			cb, ok := n.(*adf.CodeBlock)
			if !ok {
				continue
			}
			for _, c := range cb.Content {
				for line := range strings.SplitSeq(adf.NodeText(c), "\n") {
					if trimmed := strings.TrimRight(line, " \t"); trimmed != line {
						return true
					}
				}
			}
		}
	}
	return false
}

// checkboxLeadRe matches a literal checkbox token leading a text run.
var checkboxLeadRe = regexp.MustCompile(`^\[[ xX]\]( |$)`)

// docHasCheckboxLeadText reports a paragraph whose text starts with a
// literal checkbox token — rendered behind a list marker it re-parses
// as a real task item.
func docHasCheckboxLeadText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			var content []adf.Node
			switch t := n.(type) {
			case *adf.Paragraph:
				content = t.Content
			case *adf.Heading:
				content = t.Content
			default:
				continue
			}
			if len(content) == 0 {
				continue
			}
			if text, isText := content[0].(*adf.Text); isText && checkboxLeadRe.MatchString(text.Text) {
				return true
			}
		}
	}
	return false
}

// docHasLayoutlessMediaSingle reports a mediaSingle without a layout
// attribute (whose image round trip materializes the default).
func docHasLayoutlessMediaSingle(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if ms, ok := n.(*adf.MediaSingle); ok && ms.Layout == nil {
				return true
			}
		}
	}
	return false
}

// docHasBackslashBeforePreserved reports a text node carrying a literal
// backslash immediately before a preserved-escape character.
func docHasBackslashBeforePreserved(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			var s string
			switch t := n.(type) {
			case *adf.Text:
				s = t.Text
			case *adf.CodeBlock:
				// A fence info string keeps its preserved escapes too, so
				// any preserved character in the language diverges.
				if strings.ContainsAny(t.Language, markdown.PreservedEscapes+"\\") {
					return true
				}
				continue
			default:
				continue
			}
			for i := 0; i+1 < len(s); i++ {
				if s[i] == '\\' && strings.ContainsRune(markdown.PreservedEscapes, rune(s[i+1])) {
					return true
				}
			}
		}
	}
	return false
}

// docHasSpaceBeforeHardBreak reports whitespace touching a hardBreak:
// a text node ending in whitespace before the break, or one starting
// with whitespace after it (the line boundary swallows either side on
// re-parse).
func docHasSpaceBeforeHardBreak(doc adf.Doc) bool {
	return anyContentRun(doc, func(content []adf.Node) bool {
		for i := range content {
			if _, isBreak := content[i].(*adf.HardBreak); !isBreak {
				continue
			}
			if spaceTouchesBreak(content, i) {
				return true
			}
		}
		return false
	})
}

// anyContentRun reports whether pred accepts the content slice of any node
// in the document. The sibling-adjacency predicates all walk the tree the
// same way and differ only in what they look for within one node's
// children, so the walk lives here once.
func anyContentRun(doc adf.Doc, pred func(content []adf.Node) bool) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if pred(adf.NodeContent(n)) {
				return true
			}
		}
	}
	return false
}

// spaceTouchesBreak reports whether the text on either side of the hard
// break at index i carries whitespace at that boundary.
func spaceTouchesBreak(content []adf.Node, i int) bool {
	if i > 0 {
		if text, ok := content[i-1].(*adf.Text); ok &&
			strings.TrimRight(text.Text, " \t") != text.Text {
			return true
		}
	}
	if i+1 < len(content) {
		if text, ok := content[i+1].(*adf.Text); ok &&
			strings.TrimLeft(text.Text, " \t") != text.Text {
			return true
		}
	}
	return false
}

// docHasEmphasisBeforeCode reports a strong/em-marked text node directly
// followed by a code-marked one (the mark re-inference trigger).
func docHasEmphasisBeforeCode(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			content := adf.NodeContent(n)
			for i := 0; i+1 < len(content); i++ {
				cur, isText := content[i].(*adf.Text)
				next, nextText := content[i+1].(*adf.Text)
				if !isText || !nextText {
					continue
				}
				if !adf.HasMark(next.Marks, "code") {
					continue
				}
				if adf.HasMark(cur.Marks, "strong") || adf.HasMark(cur.Marks, "em") {
					return true
				}
			}
		}
	}
	return false
}

// docHasBareLinkGluedToText reports a bare autolink (label == href)
// whose preceding sibling text ends in a word character, or whose
// following sibling text starts with a non-space character — either
// side glues onto the bare URL when rendered and shifts linkification.
func docHasBareLinkGluedToText(doc adf.Doc) bool {
	return anyContentRun(doc, func(content []adf.Node) bool {
		for i := 0; i+1 < len(content); i++ {
			cur, isText := content[i].(*adf.Text)
			next, nextText := content[i+1].(*adf.Text)
			if isText && nextText && bareLinkGlued(cur, next) {
				return true
			}
		}
		return false
	})
}

// bareLinkGlued reports whether the adjacent text pair renders with a bare
// autolink glued to its neighbor.
func bareLinkGlued(cur, next *adf.Text) bool {
	if isBareLinkText(next) && cur.Text != "" && blocksLinkify(cur.Text[len(cur.Text)-1]) {
		return true
	}
	return isBareLinkText(cur) && next.Text != "" &&
		next.Text[0] != ' ' && next.Text[0] != '\t'
}

// isBareLinkText reports whether the text node renders as a bare autolink
// — its label is its own href.
func isBareLinkText(text *adf.Text) bool {
	link, hasLink := adf.FindMark[*adf.Link](text.Marks)
	return hasLink && link.Href != nil && *link.Href == text.Text
}

// blocksLinkify reports whether the byte b, rendered directly before a
// bare URL, keeps GFM from linkifying it: linkification requires
// whitespace or one of *_~( there.
func blocksLinkify(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '*', '_', '~', '(':
		return false
	}
	return true
}

// docHasLiteralTilde reports a literal tilde anywhere in the document
// text: rendered next to strike delimiters (or where a dropped
// construct used to sit) the runs merge or re-flank on re-parse.
func docHasLiteralTilde(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if text, ok := n.(*adf.Text); ok && strings.ContainsRune(text.Text, '~') {
				return true
			}
		}
	}
	return false
}

// docHasTrailingBackslashText reports a text node ending in a literal
// backslash.
func docHasTrailingBackslashText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if text, ok := n.(*adf.Text); ok && strings.HasSuffix(text.Text, "\\") {
				return true
			}
		}
	}
	return false
}

// nameThenEscapeRe matches a colon-name token immediately followed by a
// backslash escape (see the skip above).
var nameThenEscapeRe = regexp.MustCompile(`:[A-Za-z][A-Za-z0-9_-]*\\`)

// bareLeafTokenLineRe matches a line holding only a bare ::name token
// (no label or attributes).
var bareLeafTokenLineRe = regexp.MustCompile(`(?m)^\s*::[A-Za-z0-9_-]+\s*$`)

// linkShapedTextRe matches a link-shaped token inside plain text: a
// "](" or "]:" (reference definition) with any opening bracket before
// it (nested brackets included).
var linkShapedTextRe = regexp.MustCompile(`\[.*\][(:]`)

// docHasLinkShapedText reports a link-shaped token in the document's
// joined text runs. Hard breaks join the run too — a bracket pair can
// assemble across the rendered line boundary.
func docHasLinkShapedText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			var run strings.Builder
			for _, child := range adf.NodeContent(n) {
				if text, ok := child.(*adf.Text); ok {
					run.WriteString(text.Text)
					continue
				}
				if _, isBreak := child.(*adf.HardBreak); isBreak {
					continue
				}
				if linkShapedTextRe.MatchString(run.String()) {
					return true
				}
				run.Reset()
			}
			if linkShapedTextRe.MatchString(run.String()) {
				return true
			}
		}
	}
	return false
}

// docHasMarkerLeadParagraph reports a paragraph whose text starts with
// a list-marker character.
func docHasMarkerLeadParagraph(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			p, ok := n.(*adf.Paragraph)
			if !ok || len(p.Content) == 0 {
				continue
			}
			text, ok := p.Content[0].(*adf.Text)
			if !ok || text.Text == "" {
				continue
			}
			switch text.Text[0] {
			case '-', '*', '+':
				return true
			}
		}
	}
	return false
}

// docHasUnderscoreInEmphasis reports an em/strong-marked text node
// containing a literal underscore.
func docHasUnderscoreInEmphasis(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			text, ok := n.(*adf.Text)
			if !ok || !strings.ContainsRune(text.Text, '_') {
				continue
			}
			if adf.HasMark(text.Marks, "em") || adf.HasMark(text.Marks, "strong") {
				return true
			}
		}
	}
	return false
}

// docHasParenInLinkHref reports a link mark whose destination contains
// a parenthesis or backslash (both derail the bare prettier URL form).
func docHasParenInLinkHref(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if media, ok := n.(*adf.Media); ok && strings.ContainsAny(media.URL, "()\\") {
				return true
			}
			for _, m := range adf.NodeMarks(n) {
				if link, ok := m.(*adf.Link); ok && link.Href != nil &&
					strings.ContainsAny(*link.Href, "()\\") {
					return true
				}
			}
		}
	}
	return false
}

// parenDestRe matches a rendered link/image destination containing an
// opening parenthesis or backslash (see the paren-in-destination skip).
var parenDestRe = regexp.MustCompile(`\]\([^)\s]*[(\\]`)

// docHasColonInInlineLabel reports a typed inline node whose rendered
// label carries a colon.
func docHasColonInInlineLabel(doc adf.Doc) bool {
	hasColon := func(p *string) bool { return p != nil && strings.ContainsRune(*p, ':') }
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			switch t := n.(type) {
			case *adf.Placeholder:
				if strings.ContainsRune(t.Text, ':') {
					return true
				}
			case *adf.Status:
				if hasColon(t.Text) {
					return true
				}
			case *adf.Mention:
				if hasColon(t.Text) {
					return true
				}
			}
		}
	}
	return false
}

// htmlLeadLineRe matches a rendered line whose content (past any
// blockquote/list markers) starts with an HTML tag.
var htmlLeadLineRe = regexp.MustCompile(`(?m)^[>\s]*(?:[-*+] |\d+[.)] )*<[A-Za-z!/?]`)

// docHasAngleText reports a non-code text node containing an angle
// bracket (inline HTML or an HTML-ish token in the markdown source).
func docHasAngleText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			text, ok := n.(*adf.Text)
			if !ok || adf.HasMark(text.Marks, "code") {
				continue
			}
			if strings.ContainsAny(text.Text, "<>") {
				return true
			}
		}
	}
	return false
}

// docHasBackslashSpaceText reports a text node containing a literal
// backslash directly before a space.
func docHasBackslashSpaceText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if text, ok := n.(*adf.Text); ok && strings.Contains(text.Text, "\\ ") {
				return true
			}
		}
	}
	return false
}

// docHasEmoji reports an emoji node anywhere in the document.
func docHasEmoji(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if _, ok := n.(*adf.Emoji); ok {
				return true
			}
		}
	}
	return false
}

// docHasEmptyParagraph reports a paragraph with no inline content.
func docHasEmptyParagraph(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if p, ok := n.(*adf.Paragraph); ok && len(p.Content) == 0 {
				return true
			}
		}
	}
	return false
}

// emailHexRe matches an '@' followed in the same word by a character
// reference (see the email-linkification skip).
var emailHexRe = regexp.MustCompile(`@\S*&#x`)

// docHasBangBeforeLink reports a text node ending in '!' whose next
// sibling carries a link mark.
func docHasBangBeforeLink(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			content := adf.NodeContent(n)
			for i := 0; i+1 < len(content); i++ {
				cur, isText := content[i].(*adf.Text)
				next, nextText := content[i+1].(*adf.Text)
				if !isText || !nextText || !strings.HasSuffix(cur.Text, "!") {
					continue
				}
				if _, hasLink := adf.FindMark[*adf.Link](next.Marks); hasLink {
					return true
				}
			}
		}
	}
	return false
}

// docHasProseTab reports a tab inside a non-code text node.
func docHasProseTab(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if text, ok := n.(*adf.Text); ok &&
				!adf.HasMark(text.Marks, "code") && strings.ContainsRune(text.Text, '\t') {
				return true
			}
		}
	}
	return false
}

// docHasBracketInLinkLabel reports a link-marked text node containing a
// literal bracket.
func docHasBracketInLinkLabel(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			text, ok := n.(*adf.Text)
			if !ok || !strings.ContainsAny(text.Text, "[]") {
				continue
			}
			if _, hasLink := adf.FindMark[*adf.Link](text.Marks); hasLink {
				return true
			}
		}
	}
	return false
}

// nameThenHexRe matches a colon-name token immediately followed by a
// character reference (the encoded rune terminates the name on
// re-parse, like the backslash-escape case).
var nameThenHexRe = regexp.MustCompile(`:[A-Za-z][A-Za-z0-9_-]*&#x`)

// imageAltColonRe matches an image whose alt text carries markup
// (colon, emphasis, code, or strike markers).
var imageAltColonRe = regexp.MustCompile("!\\[[^\\]]*[:*_`~\\\\]") // dynamic escape: contains a backtick

// docHasTextColonFence reports a text node containing a ":::" run.
func docHasTextColonFence(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if text, ok := n.(*adf.Text); ok && strings.Contains(text.Text, ":::") {
				return true
			}
		}
	}
	return false
}

// emailShapedRe matches a GFM email-autolink-shaped token.
var emailShapedRe = regexp.MustCompile(`[A-Za-z0-9._+-]+@[A-Za-z0-9-]+\.`)

// docHasEmailShapedText reports an email-shaped token inside an
// unlinked text node.
func docHasEmailShapedText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			text, ok := n.(*adf.Text)
			if !ok || !emailShapedRe.MatchString(text.Text) {
				continue
			}
			if _, hasLink := adf.FindMark[*adf.Link](text.Marks); !hasLink {
				return true
			}
		}
	}
	return false
}

// docHasDoubleSpaceText reports a space run in the document's joined
// text (a dropped construct can leave the run split across siblings).
func docHasDoubleSpaceText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			var run strings.Builder
			for _, child := range adf.NodeContent(n) {
				if text, ok := child.(*adf.Text); ok && !adf.HasMark(text.Marks, "code") {
					run.WriteString(text.Text)
					continue
				}
				if strings.Contains(run.String(), "  ") {
					return true
				}
				run.Reset()
			}
			if strings.Contains(run.String(), "  ") {
				return true
			}
		}
	}
	return false
}

// colonNameRe matches a colon-name token (directive candidate).
var colonNameRe = regexp.MustCompile(`:[A-Za-z]`)

// docHasColonNameInLinkLabel reports a link-marked text node whose
// label carries a colon-name token.
func docHasColonNameInLinkLabel(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			text, ok := n.(*adf.Text)
			if !ok || !colonNameRe.MatchString(text.Text) {
				continue
			}
			if _, hasLink := adf.FindMark[*adf.Link](text.Marks); hasLink {
				return true
			}
		}
	}
	return false
}

// orderedGapMarkerRe matches an ordered-list marker followed by two or
// more spaces (the prettier gap-alignment input).
var orderedGapMarkerRe = regexp.MustCompile(`(?m)^[ \t]*(?:[-*+]|\d+[.)])(?:[ \t]+(?:[-*+]|\d+[.)]))*[ \t][ \t]`)

// docHasEmptyCodeBlock reports a code block with no content.
func docHasEmptyCodeBlock(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if cb, ok := n.(*adf.CodeBlock); ok && len(cb.Content) == 0 {
				return true
			}
		}
	}
	return false
}

// leafDirectiveLineRe matches a leaf- or container-directive-shaped
// input line.
var leafDirectiveLineRe = regexp.MustCompile(`(?m)^[ \t]*::`)

// entityShapedRe matches a character-reference-shaped token.
var entityShapedRe = regexp.MustCompile(`&#?[A-Za-z0-9]+;`)

// docHasEntityShapedText reports a character-reference-shaped token in
// the document's joined text runs (mark boundaries the render drops can
// assemble one).
func docHasEntityShapedText(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			var run strings.Builder
			for _, child := range adf.NodeContent(n) {
				if text, ok := child.(*adf.Text); ok && !adf.HasMark(text.Marks, "code") {
					run.WriteString(text.Text)
					continue
				}
				if entityShapedRe.MatchString(run.String()) {
					return true
				}
				run.Reset()
			}
			if entityShapedRe.MatchString(run.String()) {
				return true
			}
		}
	}
	return false
}

// emphasisImageRe matches an emphasis marker directly before an image
// opener in rendered output.
var emphasisImageRe = regexp.MustCompile(`[_*]!\[`)

// docHasTrailingSpaceRun reports a block whose last inline is a text
// node ending in whitespace.
func docHasTrailingSpaceRun(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			content := adf.NodeContent(n)
			if len(content) == 0 {
				continue
			}
			if text, ok := content[len(content)-1].(*adf.Text); ok &&
				strings.TrimRight(text.Text, " \t") != text.Text {
				return true
			}
		}
	}
	return false
}

// colonNameSuffixRe matches a colon (or colon-name token) ending a
// text run.
var colonNameSuffixRe = regexp.MustCompile(`:[A-Za-z0-9_-]*$`)

// docHasColonBeforeInline reports a joined plain-text run ending in a
// colon(-name) token directly before a construct that renders with
// directive-label-capable syntax (a non-text node's :name form, or a
// link's [label] brackets).
func docHasColonBeforeInline(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			var run strings.Builder
			for _, child := range adf.NodeContent(n) {
				text, isText := child.(*adf.Text)
				plain := isText && len(text.Marks) == 0
				if plain {
					run.WriteString(text.Text)
					continue
				}
				if colonNameSuffixRe.MatchString(run.String()) {
					return true
				}
				run.Reset()
			}
		}
	}
	return false
}

// docHasColonNameInStyledText reports a colon token inside text carrying
// a mark that renders as a :name[label] directive.
func docHasColonNameInStyledText(doc adf.Doc) bool {
	styled := func(marks []adf.Mark) bool {
		for _, m := range marks {
			switch m.Kind() {
			case "underline", "subsup", "textColor", "backgroundColor", "fontSize", "annotation":
				return true
			}
		}
		return false
	}
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			if text, ok := n.(*adf.Text); ok && styled(text.Marks) &&
				strings.ContainsRune(text.Text, ':') {
				return true
			}
		}
	}
	return false
}

// refDefShapedLineRe matches a link-reference-definition-shaped line
// (possibly behind list markers).
var refDefShapedLineRe = regexp.MustCompile(`(?m)^[ \t]*(?:(?:[-*+]|\d+[.)])[ \t]+)*\[[^\]\n]*\]:`)

// docHasEmptyListItem reports a list/task item with no content.
func docHasEmptyListItem(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			switch t := n.(type) {
			case *adf.ListItem:
				if len(t.Content) == 0 {
					return true
				}
			case *adf.TaskItem:
				if len(t.Content) == 0 {
					return true
				}
			}
		}
	}
	return false
}

// bracketLeadLineRe matches a rendered line starting with a closing
// bracket.
var bracketLeadLineRe = regexp.MustCompile(`(?m)^\]`)

// markerChainStepRe matches one leading list marker (bullet or
// ordered) plus its gap at the start of a line remainder.
var markerChainStepRe = regexp.MustCompile(`^(?:(\d+)[.)]|[-*+])[ \t]+`)

// hasIrregularOrderedNumbers reports ordered-marker numbers (including
// every marker of a same-line chain) that are neither all equal nor a
// strict +1 progression.
func hasIrregularOrderedNumbers(md string) bool {
	var nums []int
	for line := range strings.SplitSeq(md, "\n") {
		rest := strings.TrimLeft(line, " \t")
		for {
			m := markerChainStepRe.FindStringSubmatch(rest)
			if m == nil {
				break
			}
			if m[1] != "" {
				n, err := strconv.Atoi(m[1])
				if err != nil {
					return true
				}
				nums = append(nums, n)
			}
			rest = rest[len(m[0]):]
		}
	}
	if len(nums) < 2 {
		return false
	}
	allEqual, increments := true, true
	for i := 1; i < len(nums); i++ {
		if nums[i] != nums[0] {
			allEqual = false
		}
		if nums[i] != nums[i-1]+1 {
			increments = false
		}
	}
	return !allEqual && !increments
}

// mdHasQuirkyTitle reports a link/image title carrying a quote or
// backslash (the prettier renderer writes titles verbatim between its
// own quotes, which cannot represent these).
func mdHasQuirkyTitle(md string) bool {
	root := markdown.Parse([]byte(md))
	found := false
	var walk func(n ast2.Node)
	walk = func(n ast2.Node) {
		switch v := n.(type) {
		case *ast2.Link:
			if strings.ContainsAny(v.Title, "\"\\") {
				found = true
			}
		case *ast2.Image:
			if strings.ContainsAny(v.Title, "\"\\") {
				found = true
			}
		}
		for _, c := range ast2.Children(n) {
			walk(c)
		}
	}
	walk(root)
	return found
}

// docHasColonNameInEmphasis reports a colon-name token inside
// em/strong/strike-marked text.
func docHasColonNameInEmphasis(doc adf.Doc) bool {
	for _, root := range doc.Content {
		for n := range adf.Walk(root) {
			text, ok := n.(*adf.Text)
			if !ok || !colonNameRe.MatchString(text.Text) {
				continue
			}
			if adf.HasMark(text.Marks, "em") || adf.HasMark(text.Marks, "strong") ||
				adf.HasMark(text.Marks, "strike") {
				return true
			}
		}
	}
	return false
}
