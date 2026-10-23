package markdown

import "regexp"

// emailLiteralRe is gfmEmailRe's own literal body with the leading "^"
// anchor dropped, letting FindAllStringIndex scan for a candidate anywhere
// in a merged run of text instead of only at its start — the same relation
// findURLLiterals's urlLiteralRe has to the parser's anchored scheme
// pattern. The two email patterns must not drift apart: this one is copied
// from gfmEmailRe rather than re-typed, so a future grammar change only has
// one pattern to edit.
var emailLiteralRe = regexp.MustCompile(`[a-zA-Z0-9.+_-]+@[a-zA-Z0-9_-]+(?:\.[a-zA-Z0-9_-]+)*`)

// findEmailLiterals is emailLiteralRe.FindAllStringIndex with the GFM
// validity gate applied: gfmEmailValid requires a dot in the domain, no
// empty label, and a final ASCII letter — the same check
// convertGoldmarkAutoLink uses to reject a goldmark-native email autolink
// micromark would not have linkified. A rejected match is dropped whole
// rather than retried shorter, matching the reference transform's own
// behavior (see findURLLiterals, which drops the same way for the URL
// family).
func findEmailLiterals(s string) [][]int {
	var out [][]int
	for _, loc := range emailLiteralRe.FindAllStringIndex(s, -1) {
		if gfmEmailValid(s[loc[0]:loc[1]]) {
			out = append(out, loc)
		}
	}
	return out
}
