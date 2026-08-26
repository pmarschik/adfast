package adfast

import "testing"

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
		{"colon escape preserved", "1\\:yes and https\\://x\n", "1\\:yes and https\\://x\n"},
		{"bare colon-word stays", "update:scream: here\n", "update:scream: here\n"},
		{"dash escape preserved", "a \\- b\n", "a \\- b\n"},
		{"literal backslash before letter bare", "use \\App\\Services here\n", "use \\App\\Services here\n"},
		{"space hard break preserved", "a  \nb\n", "a  \nb\n"},
		{"backslash hard break preserved", "a\\\nb\n", "a\\\nb\n"},
		{"code fence trailing space trimmed", "```\nx   \n```\n", "```\nx\n```\n"},
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
