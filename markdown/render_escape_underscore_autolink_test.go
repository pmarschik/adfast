package markdown

import (
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// An intraword '_' that ENDS a text node is escaped, and the byte that
// follows it in the output does not get a vote.
//
// This file exists because the opposite reading is the natural one and it is
// wrong. "a_http://0" formats to "a\_http://0" while the reference leaves it
// bare, which reads like an over-eager escape rule — the underscore sits
// between 'a' and 'h', both word bytes, so an intraword underscore looks like
// it cannot open emphasis and needs no backslash. The measurement below says
// the rule is right and the LOOKAHEAD is the thing that must not be
// "corrected": nextTextLead reports a link's peek byte ('['), not the first
// byte the bare form will actually emit, and every agreeing row in the table
// depends on that.
//
// Measured 2026-09-09 against the authoritative prettier 3.8.1 (the copy the
// frozen TS reference pins) with the parity flags --no-config --parser
// markdown --prose-wrap always --print-width 80
// --embedded-language-formatting off:
//
//	a_http://ex.com    -> a\_http://ex.com     agrees with the format leg
//	a_http://ex.com/x  -> a\_http://ex.com/x   agrees
//	a_www.x.y          -> a\_www.x.y           agrees
//	a_<http://0>       -> a\_<http://0>        agrees
//	a_[x](y)           -> a\_[x](y)            agrees
//	a_b_c              -> a_b_c                agrees (nothing follows)
//	a_ftp://ex.com     -> a_ftp://ex.com       agrees (no link either side)
//	a_http://0         -> a_http://0           DIVERGES
//	a_HTTP://ex.com    -> a_HTTP://ex.com      DIVERGES
//
// BOTH DIVERGENT ROWS ARE A DIVERGENT PARSE, NOT A DIVERGENT ESCAPE, and the
// escape rule is the same rule on the agreeing rows. prettier 3.8.1's markdown
// parser predates CommonMark and declines to linkify a dotless host or an
// uppercase scheme, so its tree for those two rows is ONE text node whose
// underscore really is intraword. mdast-util-from-markdown with
// micromark-extension-gfm — the CommonMark+GFM reference — builds adfast's
// tree instead, measured the same day:
//
//	a_http://0       -> text "a_" + link http://0
//	a_HTTP://ex.com  -> text "a_" + link HTTP://ex.com
//	a_ftp://ex.com   -> one text node, no link
//
// So matching prettier on those two would mean UN-linking a URL the current
// reference links, which is a parse decision (parser.go and the goldmark
// linkify configuration) and a loss rather than a fix. The two rows are
// accepted divergences; the rows around them are parity, and they are what
// this table guards.
//
// The trap the table is built to catch, measured by making the lookahead
// report a bare link's real first byte ('h' of "http") and running the suite:
// the backslash goes away on both divergent rows AND on a_http://ex.com,
// a_http://ex.com/x and a_www.x.y, where the reference wants it. Two rows
// gained, three lost — and the remark leg loses a link as well, which is the
// one part of that trade the suite already caught
// (TestPunctBoundaryLiteralIsARenderFixpoint, on an unrelated "z!www.a.b").
// The prettier-parity half is what this table adds.
var underscoreBeforeAutolinkCases = []struct {
	name string
	src  string
	// fmtWant is the format leg (prettier 3.8.1 is the reference).
	fmtWant string
	// remWant is the remark leg (mdast-util-to-markdown is the reference),
	// which escapes every underscore and so cannot separate these rows.
	remWant string
}{
	{
		name:    "before a bare http autolink",
		src:     "a_http://ex.com\n",
		fmtWant: "a\\_http://ex.com\n",
		remWant: "a\\_http://ex.com\n",
	},
	{
		name:    "before a bare http autolink with a path",
		src:     "a_http://ex.com/x\n",
		fmtWant: "a\\_http://ex.com/x\n",
		remWant: "a\\_http://ex.com/x\n",
	},
	{
		name:    "before a www literal",
		src:     "a_www.x.y\n",
		fmtWant: "a\\_www.x.y\n",
		remWant: "a\\_[www.x.y](http://www.x.y)\n",
	},
	{
		name:    "before an angle autolink",
		src:     "a_<http://0>\n",
		fmtWant: "a\\_<http://0>\n",
		remWant: "a\\_<http://0>\n",
	},
	{
		name:    "before a resource link",
		src:     "a_[x](y)\n",
		fmtWant: "a\\_[x](y)\n",
		remWant: "a\\_[x](y)\n",
	},
	{
		// The accepted divergence, pinned so the next reader sees the
		// measurement rather than re-deriving it: prettier 3.8.1 writes
		// "a_http://0" because it never linkified the dotless host.
		name:    "before a dotless host the reference does not linkify",
		src:     "a_http://0\n",
		fmtWant: "a\\_http://0\n",
		remWant: "a\\_http://0\n",
	},
	{
		// The second accepted divergence, same cause: prettier 3.8.1 does
		// not linkify an uppercase scheme.
		name:    "before an uppercase scheme the reference does not linkify",
		src:     "a_HTTP://ex.com\n",
		fmtWant: "a\\_HTTP://ex.com\n",
		remWant: "a\\_HTTP://ex.com\n",
	},
	{
		// The GOOD rows: an underscore with a word byte after it INSIDE the
		// same text node keeps no backslash on the format leg. A "fix" that
		// escaped every intraword underscore to make the divergent rows go
		// away passes nothing here.
		name:    "inside one word",
		src:     "a_b_c\n",
		fmtWant: "a_b_c\n",
		remWant: "a\\_b\\_c\n",
	},
	{
		name:    "before a scheme neither side linkifies",
		src:     "a_ftp://ex.com\n",
		fmtWant: "a_ftp://ex.com\n",
		remWant: "a\\_ftp\\://ex.com\n",
	},
	{
		name:    "in snake_case ahead of a separate autolink",
		src:     "snake_case http://x.y\n",
		fmtWant: "snake_case http://x.y\n",
		remWant: "snake\\_case http://x.y\n",
	},
}

func TestFormatEscapesAnUnderscoreThatEndsATextNode(t *testing.T) {
	t.Parallel()
	for _, c := range underscoreBeforeAutolinkCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Render(convert.NormalizeFormat(Parse([]byte(c.src))), WithPrettierText())
			if got != c.fmtWant {
				t.Errorf("format %q = %q, want %q", c.src, got, c.fmtWant)
			}
			if twice := Render(convert.NormalizeFormat(Parse([]byte(got))), WithPrettierText()); twice != got {
				t.Errorf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
		})
	}
}

func TestRemarkEscapesEveryUnderscoreBeforeAnAutolink(t *testing.T) {
	t.Parallel()
	for _, c := range underscoreBeforeAutolinkCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Render(Parse([]byte(c.src))); got != c.remWant {
				t.Errorf("remark %q = %q, want %q", c.src, got, c.remWant)
			}
		})
	}
}
