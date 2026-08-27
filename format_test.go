package adfast

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// The prettier render must not let directive-shaped prose decay. On the
// ADF path the text carries no source provenance to fall back on, so an
// unescaped colon re-parses: a name the dialect registers is promoted and
// then DROPPED (":status" becomes an empty status node, so "the
// value:status is set" came back as "the value" + " is set"), ":media"
// invents a mediaInline node, and even a name nothing registers splits
// the one text node into three. Prettier itself has no directive grammar
// and writes none of these escapes, so the escape is a deliberate
// divergence — see markdown.escapesColon.
func TestPrettierRenderKeepsDirectiveShapedTextIntact(t *testing.T) {
	t.Parallel()
	// Registered names first (the lossy ones), then names the dialect
	// leaves generic — those keep every character but still resegment.
	names := []string{"status", "media", "u", "emoji", "date", "scream", "statuses", "x"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := map[string]any{
				"type": "doc", "version": 1,
				"content": []any{map[string]any{
					"type": "paragraph",
					"content": []any{map[string]any{
						"type": "text", "text": "the value:" + name + " is set",
					}},
				}},
			}
			md := adfToMD(src, WithPrettierFormat())
			want, ok := adf.DecodeDocOpts(src, adf.DecodeOptions{})
			if !ok {
				t.Fatal("could not decode the source document")
			}
			if got, want := marshalDoc(t, mdToADF(md)), marshalDoc(t, want); got != want {
				t.Errorf("prettier render changed the text\n rendered: %q\n after:    %s\n before:   %s", md, got, want)
			}
		})
	}
}

// Each case pins FormatMarkdown against measured prettier 3.8 output
// (--prose-wrap always --print-width 80 --embedded-language-formatting off).
func TestFormatMarkdown_PrettierParity(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"frontmatter verbatim", "---\ntitle: x\n---\n\ntext\n", "---\ntitle: x\n---\n\ntext\n"},
		{"tight list stays tight", "- a\n- b\n", "- a\n- b\n"},
		{"loose gap preserved per item", "- a\n\n- b\n- c\n", "- a\n\n- b\n- c\n"},
		{"item internal blank spreads", "- x\n\n  second para of x\n- y\n- z\n", "- x\n\n  second para of x\n\n- y\n- z\n"},
		{"nested list attaches, spread kept", "- a\n\n  - sub\n- b\n", "- a\n  - sub\n\n- b\n"},
		{"ordered all-ones preserved", "1. one\n1. two\n", "1. one\n1. two\n"},
		{"ordered increment preserved", "1. one\n2. two\n", "1. one\n2. two\n"},
		{"ordered two-space gap aligns", "1.  two spaces\n2. one\n", "1.  two spaces\n2.  one\n"},
		{"ordered gap aligns wide markers", "9.  two\n10.  also\n", "9.  two\n10. also\n"},
		{"bare autolink stays bare", "see https://example.com/x now\n", "see https://example.com/x now\n"},
		{"angle autolink stays angled", "see <https://example.com/x> now\n", "see <https://example.com/x> now\n"},
		{"explicit self link not shortened", "[https://x.com/a](https://x.com/a)\n", "[https://x.com/a](https://x.com/a)\n"},
		{"image preserved", "![Sketch](./assets/a.png)\n", "![Sketch](./assets/a.png)\n"},
		{"html comments stay stacked", "<!-- a -->\n<!-- b -->\n\nx\n", "<!-- a -->\n<!-- b -->\n\nx\n"},
		{"html comments blank kept", "<!-- a -->\n\n<!-- b -->\n\nx\n", "<!-- a -->\n\n<!-- b -->\n\nx\n"},
		{"inline html kept", "text with <br/> inline html\n", "text with <br/> inline html\n"},
		{"intraword underscore bare", "blocked_account here\n", "blocked_account here\n"},
		{"boundary underscore escaped", "trail_ word\n", "trail\\_ word\n"},
		{"tilde escape preserved", "requires \\~51 pages\n", "requires \\~51 pages\n"},
		{"bare tilde stays bare", "requires ~51 pages\n", "requires ~51 pages\n"},
		// The strike run ends right at a code span, and
		// unmarked text follows immediately. The re-inferred strike mark
		// used to leak past the code span onto that trailing "!" (measured
		// against remark-parse 11 + remark-stringify 11 + remark-gfm 4,
		// which closes the strike right after the code span).
		{"strike at code boundary does not leak onto trailing text", "~0`0`~!\n", "~~0`0`~~!\n"},
		{"colon escape preserved", "1\\:yes and https\\://x\n", "1\\:yes and https\\://x\n"},
		// A DELIBERATE divergence from prettier, which has no directive
		// grammar and leaves this colon bare. Bare, the text re-parses as
		// a ":scream" directive, so the one text node comes back as three
		// — and for a name the dialect registers the text is dropped
		// outright. See markdown.escapesColon and
		// TestPrettierRenderKeepsDirectiveShapedTextIntact.
		{"bare colon-word escapes, unlike prettier", "update:scream: here\n", "update\\:scream: here\n"},
		{"dash escape preserved", "a \\- b\n", "a \\- b\n"},
		{"literal backslash before letter bare", "use \\App\\Services here\n", "use \\App\\Services here\n"},
		{"space hard break preserved", "a  \nb\n", "a  \nb\n"},
		{"backslash hard break preserved", "a\\\nb\n", "a\\\nb\n"},
		{"code fence trailing space trimmed", "```\nx   \n```\n", "```\nx\n```\n"},
		{
			"table attaches to a paragraph in a tight item",
			"- a\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- b\n",
			"- a\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- b\n",
		},
		{"numeric reference escape preserved", "a \\&#169; b\n", "a \\&#169; b\n"},
		{"named entity escape preserved", "a \\&amp; b\n", "a \\&amp; b\n"},
		{"hex reference escape preserved", "a \\&#xA9; b\n", "a \\&#xA9; b\n"},
		{"bare ampersand stays bare", "AT&T x\n", "AT&T x\n"},
		{"non-reference ampersand stays bare", "AT&T; x\n", "AT&T; x\n"},
		{"reference in a code span untouched", "code `&#169;` x\n", "code `&#169;` x\n"},
		{
			"prefix-aware wrap in blockquote",
			"> the quick brown fox jumps over the lazy dog and keeps going until the line has to wrap somewhere\n",
			"> the quick brown fox jumps over the lazy dog and keeps going until the line has\n> to wrap somewhere\n",
		},
		{
			"code span moves wholly to next line",
			"endpoints and all three new hierarchy endpoints. Serve the thing at `GET /partner/x`\n",
			"endpoints and all three new hierarchy endpoints. Serve the thing at\n`GET /partner/x`\n",
		},
		{
			"link moves wholly to next line",
			"Lorem ipsum dolor sit amet consectetur adipiscing elit sed [quick brown fox link](https://example.com/x) jumps.\n",
			"Lorem ipsum dolor sit amet consectetur adipiscing elit sed\n[quick brown fox link](https://example.com/x) jumps.\n",
		},
		{
			"marker word glues to predecessor",
			"Any guid absent from the cache in general and every case here always then returns 404. The rest follows.\n",
			"Any guid absent from the cache in general and every case here always then\nreturns 404. The rest follows.\n",
		},
		{
			"task item wraps with aligned continuation",
			"- [ ] We should be able to see all connections from the master tenant in auth0 attached\n",
			"- [ ] We should be able to see all connections from the master tenant in auth0\n      attached\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fmtMD(tc.in); got != tc.want {
				t.Errorf("FormatMarkdown mismatch\n in:  %q\n got: %q\n want:%q", tc.in, got, tc.want)
			}
			// Idempotence: formatting the output again must be stable.
			if once := fmtMD(tc.in); fmtMD(once) != once {
				t.Errorf("not idempotent for %q", tc.in)
			}
		})
	}
}

// A literal "&#169;" in prose is not a character reference; written bare it
// becomes one on re-parse and decodes to "©" — a silent character change,
// and a document with two different renderings depending on the pass. See
// markdown.escapeAmpersand.
func TestCharacterReferenceInTextSurvivesAReformat(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, in, want string }{
		{"numeric reference", "a \\&#169; b\n", "a \\&#169; b\n"},
		{"named entity", "a \\&amp; b\n", "a \\&amp; b\n"},
		{"hex reference", "a \\&#xA9; b\n", "a \\&#xA9; b\n"},
		{"inside emphasis", "**\\&#169;** x\n", "**\\&#169;** x\n"},
		{"inside a link label", "[lab \\&#169;](http://x)\n", "[lab \\&#169;](http://x)\n"},
		{"bare ampersand unaffected", "AT&T x\n", "AT&T x\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtMD(tc.in)
			if got != tc.want {
				t.Errorf("FormatMarkdown mismatch\n in:  %q\n got: %q\n want:%q", tc.in, got, tc.want)
			}
			if again := fmtMD(got); again != got {
				t.Errorf("not idempotent\n got:   %q\n again: %q", got, again)
			}
			// The escape must not change what the ADF says the text is.
			if wantADF, gotADF := marshalADF(t, tc.in), marshalADF(t, got); wantADF != gotADF {
				t.Errorf("format changed ADF meaning\n before: %s\n after:  %s", wantADF, gotADF)
			}
		})
	}
}

// An item the renderer had to break open with a blank line is spread when
// the next parse reads it back, so it needs its separator from the FIRST
// render. Without that separator the render was a fixpoint only after two
// passes: pass 1 wrote the forced internal blank but no separator before the
// next item; pass 2 read that blank back as Spread and inserted the
// separator pass 1 owed. Scoped to goldmark-sourced (PerItemSpread) lists
// only — the ADF path keeps the old non-fixpoint behavior to preserve
// remark parity, see the runListRoundTrips-based tests in
// list_nesting_test.go for that side.
//
// A GFM table with an ORDINARY header no longer forces a gap at all (see
// adjacencyIsUnsafe, the narrower adjacency fix), so the first two cases are
// pure fixpoints with no blank line anywhere — matching prettier exactly.
// The third case keeps the gap, and therefore still needs the separator,
// because its table's OWN header row is itself delimiter-shaped (the real
// hazard 1B narrows down to); it is the pinned fuzz repro from
// TestTableAfterParagraphInAListItemKeepsItsHeader, extended with a second
// item to exercise the separator this test is about.
func TestForcedItemGapSeparatesTheNextItem(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, in, want string }{
		{
			"tight list, paragraph then ordinary-header table: no gap needed",
			"- a\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- b\n",
			"- a\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- b\n",
		},
		{
			"three items, ordinary-header table attaches tight throughout",
			"- a\n- b\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- c\n",
			"- a\n- b\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- c\n",
		},
		{
			"delimiter-shaped header still forces the gap and the separator",
			"*     0\n  0\n\n  --\n--\n0\n- b\n",
			"- ```\n  0\n  ```\n  0\n\n  | -- |\n  | -- |\n  | 0  |\n\n* b\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtMD(tc.in)
			if got != tc.want {
				t.Fatalf("first render = %q, want %q", got, tc.want)
			}
			if second := fmtMD(got); second != got {
				t.Fatalf("not idempotent:\n first:  %q\n second: %q", got, second)
			}
		})
	}
}

// Format mode and the ADF decode must agree on what a canonical inline
// :media looks like. The default media type ("file") is re-inferred on
// encode, so neither path writes it — format used to, which left the
// formatter and the ADF round trip disagreeing about the same node. A
// bare `collection` is NOT a default: mediaInline encode keeps the
// difference between an empty collection and no collection at all, so it
// survives both paths.
func TestFormatMediaInlineOmitsTheDefaultType(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bare id", "kit: :media{#7c1e}\n", "kit: :media{#7c1e}\n"},
		{"explicit default type dropped", "kit: :media{#7c1e type=\"file\"}\n", "kit: :media{#7c1e}\n"},
		{"empty collection kept", "kit: :media{#7c1e collection type=\"file\"}\n", "kit: :media{#7c1e collection}\n"},
		{"other type kept", "kit: :media{#7c1e type=\"link\"}\n", "kit: :media{#7c1e type=\"link\"}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fmtMD(tc.in); got != tc.want {
				t.Errorf("FormatMarkdown mismatch\n in:  %q\n got: %q\n want:%q", tc.in, got, tc.want)
			}
			// The same document taken through ADF must land on the same text.
			if got := adfToMD(mdToADF(tc.in)); got != tc.want {
				t.Errorf("ADF round trip disagrees with format\n in:  %q\n got: %q\n want:%q", tc.in, got, tc.want)
			}
		})
	}
}
