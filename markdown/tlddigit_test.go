package markdown_test

import (
	"slices"
	"testing"
)

// A DIGIT IN THE TLD USED TO TRUNCATE THE LITERAL, and the truncated string
// went out as the href. "https://ex.c0m" matched only as far as
// "https://ex.c", so the rendered link text and the address it pointed at
// named two different hosts — and the target was a real-looking but wrong
// one. That is worse than no link at all: a missing link is visible in the
// output, a wrong href is not.
//
// The site is urlLiteralHostDotted's TLD class, the same class the "ex.coſ"
// row in urlliteral_test.go moved for the non-ASCII letter. goldmark spelled
// it `[a-zA-Z]+`; both of the reference's recognizers take a digit —
// micromark's tokenizer domain consumes any code that is neither whitespace
// nor `\p{P}`/`\p{S}`, and mdast-util's decoded-text host is `[-.\w]+`, whose
// `\w` holds the digits — so the class takes one here too.
//
// Measured against the reference (remark-parse 11.0.0 + remark-gfm 4.0.1 +
// remark-stringify 11.0.0), the "was" column being this package before the
// class moved:
//
//	"see https://ex.c0m here"        ref https://ex.c0m         was https://ex.c
//	"see https://ex.co2m/x here"     ref https://ex.co2m/x      was https://ex.co
//	"see www.ex.c0m here"            ref http://www.ex.c0m      was http://www.ex.c
//	"see https://ex.c0m:8080/p b"    ref https://ex.c0m:8080/p  was https://ex.c
//	"a [ https://ex.c0m b"           ref https://ex.c0m         was https://ex.c
//	"see https://ex.a_1 b"           ref (no link)              was https://ex.a
func TestDigitInTLDKeepsTheWholeHost(t *testing.T) {
	const src = "See https://ex.c0m, https://ex.co2m/x, www.ex.c0m and https://ex.c0m:8080/p here.\n" +
		"See https://ex.a_1 here.\n" +
		"See https://ex.com, www.ex.com and https://ex.co here.\n"

	want := []string{
		// Line one: the truncating shapes. TEXT AND HREF ARE ASSERTED
		// TOGETHER, which is the whole point — the defect was a row where the
		// two halves disagreed, so a test reading only the href would have
		// passed on "https://ex.c" twice over.
		"https://ex.c0m|https://ex.c0m",
		"https://ex.co2m/x|https://ex.co2m/x",
		"www.ex.c0m|http://www.ex.c0m",
		"https://ex.c0m:8080/p|https://ex.c0m:8080/p",
		// Line two produces nothing, and it is the row where the digit and
		// the '_' compose. The TLD class takes '_' so that
		// urlLiteralHostAccepted can SEE an underscore in the last segment and
		// refuse the whole address; a digit ending the class one byte early
		// left the '_' trailing, the gate's trailing-punctuation trim then
		// dropped it, and a link to "https://ex.a" went out where the
		// reference reads the line as prose. With the digit in the class the
		// TLD is "a_1", the underscore is still there when the gate looks, and
		// the address is refused.
		//
		// Line three: THE GOOD CASES, in the same document so the boundary is
		// visible. A letters-only TLD, the scheme-less form and a short TLD
		// all linked before the class moved and still do — a change that
		// widened the TLD to swallow the rest of the line would pass every row
		// above and break these.
		"https://ex.com|https://ex.com",
		"www.ex.com|http://www.ex.com",
		"https://ex.co|https://ex.co",
	}
	if got := linkVerdicts(src); !slices.Equal(got, want) {
		t.Errorf("links of\n%s\n got %v\nwant %v", src, got, want)
	}

	// The decoded-text recognizer — the text a dangling link label leaves
	// behind — reads urlLiteralRe, which is built from the same shared host
	// rule. It truncated identically, and it has to move with the raw one or
	// the same address is two different links depending on its surroundings.
	const dangling = "a [ https://ex.c0m b\n"
	wantDangling := []string{"https://ex.c0m|https://ex.c0m"}
	if got := linkVerdicts(dangling); !slices.Equal(got, wantDangling) {
		t.Errorf("links of %q = %v, want %v", dangling, got, wantDangling)
	}
}

// THE ROW THE DIGIT FIX COST IS CLOSED AGAIN, and this test is where the two
// spellings are held together.
//
// A TLD whose FIRST character is a digit and whose host then continued with a
// '-' used to fail the DOTTED host alternative outright and fall through to
// the DOTLESS one, which spans the whole of urlLiteralHostByte — so
// "https://ex.0com-x" linked whole by accident while the letter-led
// "https://ex.com-x" stopped at "https://ex.com". Widening the TLD class for
// the digit made the two agree, on the SHORT answer, and that was pinned here
// as the digit fix's price.
//
// The '-' in the same class then closed it, on the reference's answer this
// time. Measured against the reference (remark-parse 11.0.0 + remark-gfm
// 4.0.1), both recognizers, one body per row:
//
//	"see https://ex.0com-x b"  ref https://ex.0com-x  was https://ex.0com
//	"see https://ex.com-x b"   ref https://ex.com-x   was https://ex.com
//
// The test stays because the two spellings answering ALIKE is the property,
// and it is the property either way the shared class moves: a change that
// takes the '-' back out has to break this row and the punycode rows in
// TestHyphenInTLDKeepsTheWholeHost together.
func TestDigitLedTLDAnswersLikeTheLetterLedOne(t *testing.T) {
	const src = "See https://ex.0com-x and https://ex.com-x here.\n"
	want := []string{
		"https://ex.0com-x|https://ex.0com-x",
		"https://ex.com-x|https://ex.com-x",
	}
	if got := linkVerdicts(src); !slices.Equal(got, want) {
		t.Errorf("links of %q = %v, want %v", src, got, want)
	}
}
