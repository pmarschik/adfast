package markdown

import (
	"regexp"
	"strings"
	"unicode"
)

// Prose wrapping: prettier's prose-wrap always behavior — protected
// spans (directives, code spans, links), the wrap masks, and the
// display-width budget. Split from render.go.

// directiveSpanRe matches :name[label]{attrs} / ::name[label]{attrs} spans
// whose internal spaces must never be wrapped across lines (a newline inside
// a directive label or attribute block ends the directive on re-parse).
var directiveSpanRe = regexp.MustCompile(`::?[A-Za-z0-9][A-Za-z0-9_-]*(?:\[[^\]\n]*\])?(?:\{[^}\n]*\})?`)

// wrapTextProtected wraps like wrapText but keeps directive labels,
// attribute blocks, and inline code spans intact by masking their spaces
// during wrapping. Prettier treats a code span as one unbreakable unit and
// moves it wholly to the next line; a newline inside a directive label
// would end the directive on re-parse.
func wrapTextProtected(s string, maxWidth int) string {
	if maxWidth <= 0 || len(s) <= maxWidth {
		return wrapText(s, maxWidth)
	}
	masked := s
	if strings.ContainsRune(masked, ':') {
		masked = directiveSpanRe.ReplaceAllStringFunc(masked, func(span string) string {
			return strings.Map(func(r rune) rune {
				if r == ' ' {
					return wrapMask
				}
				return r
			}, span)
		})
	}
	if strings.ContainsRune(masked, '`') {
		masked = maskCodeSpanSpaces(masked)
	}
	wrapped := wrapText(masked, maxWidth)
	if masked != s {
		wrapped = strings.ReplaceAll(wrapped, string(wrapMask), " ")
		wrapped = strings.ReplaceAll(wrapped, string(wrapMaskTab), "\t")
	}
	return wrapped
}

// wrapMask replaces spaces that must survive wrapping; NUL cannot occur in
// rendered text (it is replaced with U+FFFD at parse).
const wrapMask = '\x00'

// wrapMaskTab likewise protects tabs (code spans can contain them; a wrap
// break there would re-parse the span differently).
const wrapMaskTab = '\x01'

// maskCodeSpanSpaces masks the spaces inside backtick code spans. Fences are
// unescaped backtick runs; a span closes at the next run of exactly the same
// length (CommonMark). Escaped backticks (\`) outside spans are skipped.
func maskCodeSpanSpaces(s string) string {
	b := []byte(s)
	i := 0
	for i < len(b) {
		switch b[i] {
		case '\\':
			i += 2
		case '`':
			n := 0
			for i+n < len(b) && b[i+n] == '`' {
				n++
			}
			j := i + n
			for j < len(b) {
				if b[j] == '\\' {
					j += 2
					continue
				}
				if b[j] != '`' {
					j++
					continue
				}
				m := 0
				for j+m < len(b) && b[j+m] == '`' {
					m++
				}
				if m == n {
					break
				}
				j += m
			}
			if j < len(b) {
				for k := i; k < j; k++ {
					switch b[k] {
					case ' ':
						b[k] = byte(wrapMask)
					case '\t':
						b[k] = byte(wrapMaskTab)
					}
				}
				i = j + n
			} else {
				i += n
			}
		default:
			i++
		}
	}
	return string(b)
}

// wrapText wraps a string at maxWidth characters, breaking on word boundaries.
// Existing newlines in the input are preserved. Matches remark-stringify's
// default 80-column wrapping of the reference pipeline.
func wrapText(s string, maxWidth int) string {
	if maxWidth <= 0 || len(s) <= maxWidth {
		return s
	}
	var result strings.Builder
	first := true
	for line := range strings.SplitSeq(s, "\n") {
		if !first {
			result.WriteByte('\n')
		}
		first = false
		if displayWidth(line) <= maxWidth {
			result.WriteString(line)
			continue
		}
		// Preserve any leading whitespace from the original line. Jira ADF text nodes
		// sometimes begin with a space (a known Jira quirk); stripping it here would
		// turn a list item's double-space marker into a single-space one, losing the
		// structural information that normalizeDoubleSpaceListMarkers relies on.
		leadLen := len(line) - len(strings.TrimLeft(line, " \t"))
		lead := line[:leadLen]
		// A trailing-space hard break must survive word splitting; it is
		// re-attached to the last wrapped line of this segment.
		hardBreak := strings.HasSuffix(line, "  ")
		words := strings.FieldsFunc(line[leadLen:], func(r rune) bool {
			// Only space and tab split words; other whitespace (e.g. \v)
			// stays inside words like prettier's fill.
			return r == ' ' || r == '\t'
		})
		// A word that would re-parse as a block marker at the start of a
		// wrapped line ("12.", "-", ">") glues to its predecessor; the pair
		// wraps as one unit (prettier's behavior).
		for i := 1; i < len(words); {
			if wordLooksLikeMarker(words[i]) {
				words[i-1] += " " + words[i]
				words = append(words[:i], words[i+1:]...)
				continue
			}
			i++
		}
		current := lead // first group includes leading whitespace
		width := displayWidth(lead)
		for _, word := range words {
			switch {
			case current == lead:
				current = lead + word
				width += displayWidth(word)
			case width+1+displayWidth(word) <= maxWidth:
				current += " " + word
				width += 1 + displayWidth(word)
			default:
				result.WriteString(current)
				result.WriteByte('\n')
				current = word // continuation lines: no leading whitespace
				width = displayWidth(word)
			}
		}
		result.WriteString(current)
		if hardBreak {
			result.WriteString("  ")
		}
	}
	return result.String()
}

// wordLooksLikeMarker reports whether a word placed at the start of a
// wrapped continuation line would re-parse as a block marker ("12.", "7)",
// "-", "*", "+", ">", "#").
func wordLooksLikeMarker(word string) bool {
	switch word {
	case "-", "*", "+", ">", "#":
		return true
	}
	if len(word) < 2 {
		return false
	}
	last := word[len(word)-1]
	if last != '.' && last != ')' {
		return false
	}
	for i := range len(word) - 1 {
		if word[i] < '0' || word[i] > '9' {
			return false
		}
	}
	return true
}

// displayWidth measures a string the way prettier's wrap budget does:
// one column per rune, two for East Asian wide/fullwidth runes, zero for
// combining marks.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r >= 0x1F000 && r <= 0x1FAFF, r >= 0x2600 && r <= 0x27BF:
			// emoji (string-width counts them double)
			w += 2
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r):
			// combining marks
		case isEastAsianWide(r):
			w += 2
		default:
			w++
		}
	}
	return w
}

// isEastAsianWide covers the common Wide/Fullwidth ranges (CJK, Hangul,
// fullwidth forms) — the subset string-width treats as double width.
func isEastAsianWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x303E, // CJK Radicals .. CJK Symbols
		r >= 0x3041 && r <= 0x33FF, // Hiragana .. CJK Compatibility
		r >= 0x3400 && r <= 0x4DBF, // CJK Ext A
		r >= 0x4E00 && r <= 0x9FFF, // CJK Unified
		r >= 0xA000 && r <= 0xA4CF, // Yi
		r >= 0xAC00 && r <= 0xD7A3, // Hangul Syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK Compatibility Ideographs
		r >= 0xFE30 && r <= 0xFE4F, // CJK Compatibility Forms
		r >= 0xFF00 && r <= 0xFF60, // Fullwidth Forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x3FFFD: // CJK Ext B+
		return true
	}
	return false
}
