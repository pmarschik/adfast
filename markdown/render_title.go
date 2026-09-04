package markdown

import "strings"

// Writing the title of a link, an image or a link reference definition.
//
// A title is written between delimiters, so the delimiter is the one
// character that can end it early — and the parse hands the renderer the
// characters themselves, not the source spelling: goldmark_to_ast.go
// decodes a title's escapes with no keep set, so a '"' the author wrote as
// '\"' arrives as a bare quote. Written back verbatim it closes the title,
// the bytes after it are no longer part of one, and the NEXT parse reads
// the whole construct as prose: a link stops being a link, and a
// definition stops being a definition — which also unresolves every
// reference that paired with it. Parse → Render therefore has to put the
// escapes back for the round trip to be a fixed point, and that is what
// this file is for.
//
// The two render modes disagree on how, because they follow different
// upstream tools: mdast-util-to-markdown always writes the '"' form and
// escapes what it must inside it, while prettier picks a delimiter the
// title does not contain and escapes only when no delimiter is free. Both
// rules below are measurements of those tools.

// titleSegment returns the whole title part of a link, an image or a
// definition — the separating space and the delimiters included. Callers
// hold a non-empty title: an empty one writes nothing at all, since the
// delimiters alone would be a title of "".
func (r *mdRenderer) titleSegment(title string) string {
	if r.cfg.prettierText {
		return " " + prettierTitle(title)
	}
	return " \"" + remarkTitle(title) + "\""
}

// remarkTitle escapes a title for the double-quoted form, the only form
// mdast-util-to-markdown writes:
//
//   - a '"' becomes '\"', so it cannot end the title;
//   - a '\' before ASCII punctuation is doubled, so it stays a literal
//     backslash instead of escaping the character after it — and the
//     closing delimiter counts as that character, which is why a title
//     ENDING in a backslash needs the doubling too;
//   - a ':' before an ASCII letter becomes '\:'. That one is this
//     dialect's own rather than remark's: a text directive opens with
//     ':' plus a letter, so an unescaped one would re-parse as a
//     directive.
func remarkTitle(s string) string {
	if !strings.ContainsAny(s, "\":\\") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 8)
	for i := range len(s) {
		switch {
		case s[i] == '"':
			sb.WriteString("\\\"")
		case s[i] == '\\' && isASCIIPunct(byteAt(s, i+1, '"')):
			sb.WriteString("\\\\")
		case s[i] == ':' && i+1 < len(s) && isASCIILetter(s[i+1]) && (i == 0 || s[i-1] != ':'):
			sb.WriteString("\\:")
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// prettierTitle writes a title the way prettier's printTitle does, so the
// formatter and prettier agree on the delimiter as well as the content:
//
//   - every '\' is doubled, because the parse already decoded the source's
//     escapes and every backslash still standing is a literal one;
//   - a title holding BOTH quote characters is written between
//     parentheses, which needs no escape at all;
//   - otherwise the delimiter is the quote the title holds fewer of ('"'
//     on a tie, prettier's default), and any occurrence of it is escaped.
//
// Two divergences from prettier, both in cases where prettier's own output
// does not survive its own re-parse. Prettier writes the parenthesized
// form verbatim, which turns a '\' before punctuation into an escape and
// drops the title outright when the title ends in one; the doubling above
// happens first here, so the form stays reversible. And prettier uses that
// form for a title holding a '(' — which this dialect's parser rejects as
// a title, demoting the construct on the next parse — so the guard below
// covers both parentheses.
//
// That last one is a PARSE divergence from micromark, and the escape
// above is its consequence rather than a preference. CommonMark's
// parenthesized title admits a '(' or a ')' "only if it is
// backslash-escaped", and goldmark enforces it; micromark's title
// scanner takes the first ')' as the closer and lets an unescaped '('
// through as content. Measured on '[a]: ./a.md (a ( b)': micromark
// yields definition(title: "a ( b"), goldmark yields no definition at
// all — the line stays a paragraph and every reference that paired with
// the label loses its link. Escape the paren ('[a]: ./a.md (a \( b)')
// and both read the same title. The stricter side is the
// spec-conforming one, so this is not a bug to fix here; but a parser
// that accepted micromark's shape would let prettierTitle drop the
// "()" half of its guard and pick the delimiter by prettier's rule
// alone, which is the only reason to want it relaxed.
func prettierTitle(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	if strings.Contains(s, "\"") && strings.Contains(s, "'") && !strings.ContainsAny(s, "()") {
		return "(" + s + ")"
	}
	quote := "\""
	if strings.Count(s, "\"") > strings.Count(s, "'") {
		quote = "'"
	}
	return quote + strings.ReplaceAll(s, quote, "\\"+quote) + quote
}
