package markdown_test

import (
	"slices"
	"testing"
)

// THE BARE-URL SCHEME SET IS GFM's, and this is the one document that draws
// the whole boundary rather than one row of it.
//
// GFM's autolink literal recognizes "http://", "https://", the scheme-less
// "www." form and the email form, and NOTHING else. goldmark's linkify
// extension adds "ftp://" on top of that set, and this package inherited the
// addition along with the extension — so a bare ftp address became a link
// node here while every other bare scheme an author might write stayed prose,
// and while the reference left the ftp one as prose too. That is a link the
// author did not write appearing in the tree, which is the costly direction:
// a missing link is visible in the rendered output, an invented one is markup
// this package put there and no diff against the source can show.
//
// Measured against the reference (remark-parse 11.0.0 + remark-gfm 4.0.1 +
// remark-stringify 11.0.0), both of its recognizers — the raw-source one and
// the decoded-text transform reached through a dangling link label:
//
//	"see ftp://ex.com here"   ref (no link)   was ftp://ex.com
//	"a [ ftp://ex.com b"      ref (no link)   was ftp://ex.com
//
// WHY ALL FIVE SCHEMES IN ONE DOCUMENT. Read one at a time, the ftp row looks
// like an arbitrary subtraction; read beside "mailto:", "file://" and "ssh://"
// — none of which linkify here or in the reference, and none of which ever
// did — it is plainly the removal of the single exception. A later change that
// re-adds a scheme to urlLiteralScheme has to explain itself against this list
// rather than against one line.
func TestBareSchemeSetIsGFM(t *testing.T) {
	const src = "See ftp://ex.com, mailto:x@y.z, file:///tmp/x and ssh://h/x here.\n" +
		"See https://ex.com, http://ex.com, www.ex.com and <ftp://ex.com> here.\n"

	// THE GOOD CASES SHARE THE DOCUMENT ON PURPOSE. Line two is what stops a
	// "stop linkifying anything" implementation from passing: the two GFM
	// schemes and the scheme-less form still link, and so does an ftp address
	// the author wrote in angle brackets — the scheme set gates the BARE form
	// only, which is where an author who wants an ftp link still goes.
	want := []string{
		"https://ex.com|https://ex.com",
		"http://ex.com|http://ex.com",
		"www.ex.com|http://www.ex.com",
		"ftp://ex.com|ftp://ex.com", // the angle-bracket autolink
	}
	if got := linkVerdicts(src); !slices.Equal(got, want) {
		t.Errorf("links of\n%s\n got %v\nwant %v", src, got, want)
	}

	// The decoded-text recognizer reads the text a dangling link label left
	// behind, and it has its own copy of the scheme rule. It has to agree, or
	// the same address is prose in one spelling and a link in the other.
	const dangling = "a [ ftp://ex.com b\n"
	if got := linkVerdicts(dangling); len(got) != 0 {
		t.Errorf("links of %q = %v, want none", dangling, got)
	}
}

// The bare ftp address survives the round trip as the prose it now is. The
// remark leg writes the ':' escape its unsafe table asks for (see
// colonBeforeSlashEscapes), and the re-parse of that escape must not linkify
// the address back — which is the shape that would turn "no link" into "a
// link, one format pass later".
func TestBareFTPStaysProseAcrossTheEscape(t *testing.T) {
	for _, src := range []string{
		"a ftp://ex.com b\n",
		"a ftp\\://ex.com b\n",
		"a_ftp://ex.com\n", // '_' is a linkify boundary byte
		"a*ftp://ex.com\n",
	} {
		if got := linkVerdicts(src); len(got) != 0 {
			t.Errorf("links of %q = %v, want none", src, got)
		}
	}
	// The good case for the same three spellings: an https address linkifies
	// through each of them, so this is the scheme and not the boundary.
	for _, src := range []string{
		"a https://ex.com b\n",
		"a https\\://ex.com b\n",
		"a_https://ex.com\n",
		"a*https://ex.com\n",
	} {
		if got := linkVerdicts(src); len(got) != 1 || got[0] != "https://ex.com|https://ex.com" {
			t.Errorf("links of %q = %v, want one https://ex.com", src, got)
		}
	}
}
