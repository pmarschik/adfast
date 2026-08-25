package markdown

import "strings"

// WrapProse rewraps prose paragraphs in already-rendered Markdown text to
// the given width, operating line by line on the raw text rather than
// through the parse/render pivot. Only contiguous runs of plain paragraph
// lines are rewrapped (joined on spaces, then re-split at word boundaries
// to fit width); it never reformats content it does not wrap.
//
// The following are left byte-for-byte untouched: a leading YAML
// frontmatter block (--- fences), fenced code blocks (``` or ~~~),
// HTML comments (single- and multi-line), ATX headings, ordered and
// unordered list items, indented lines (list continuations and the like),
// tables, blockquotes, thematic breaks, and blank lines.
//
// A width of zero or less defaults to 80. Words longer than width are kept
// intact on their own line rather than being broken.
func WrapProse(md string, width int) string {
	if width <= 0 {
		width = 80
	}
	lines := strings.Split(md, "\n")
	var out []string
	for i := 0; i < len(lines); {
		out, i = wrapProseBlock(out, lines, i, width)
	}
	return strings.Join(out, "\n")
}

// wrapProseBlock consumes one block starting at lines[i] — a verbatim
// region, a single non-wrappable line, or a prose paragraph — appends its
// output form to out and returns the index just past it.
func wrapProseBlock(out, lines []string, i, width int) (appended []string, next int) {
	line := lines[i]
	switch {
	case i == 0 && line == "---":
		// Frontmatter block: verbatim through the closing fence.
		out = append(out, line)
		return copyVerbatim(out, lines, i+1, func(l string) bool { return l == "---" })
	case isFence(line):
		fence := line[:3]
		out = append(out, line)
		return copyVerbatim(out, lines, i+1, func(l string) bool {
			return strings.HasPrefix(l, fence) && strings.TrimSpace(l) == fence
		})
	case isHTMLComment(line):
		out = append(out, line)
		if strings.Contains(line, "-->") {
			return out, i + 1
		}
		return copyVerbatim(out, lines, i+1, func(l string) bool { return strings.Contains(l, "-->") })
	case isNonWrappable(line):
		// Headings, lists, tables, blockquotes, blank lines, thematic breaks.
		return append(out, line), i + 1
	}
	// Paragraph: collect the contiguous prose lines and wrap them as one.
	var para []string
	for i < len(lines) && !isNonWrappable(lines[i]) && !isFence(lines[i]) && !isHTMLComment(lines[i]) {
		para = append(para, lines[i])
		i++
	}
	return append(out, wrapWords(strings.Join(para, " "), width)...), i
}

// copyVerbatim appends lines from `from` onward, through the first line
// closes accepts (the closer included), and returns the index just past
// it. An unterminated region runs to the end of the input.
func copyVerbatim(out, lines []string, from int, closes func(string) bool) (appended []string, next int) {
	for from < len(lines) {
		line := lines[from]
		out = append(out, line)
		from++
		if closes(line) {
			break
		}
	}
	return out, from
}

// isNonWrappable reports whether the line is one the rewrap leaves
// byte-for-byte alone.
func isNonWrappable(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	// Indented lines (list continuations, etc.) — preserve as-is
	if line[0] == ' ' || line[0] == '\t' {
		return true
	}
	return isListLine(trimmed) || isBlockSyntaxLine(trimmed)
}

// isListLine reports whether the trimmed line opens a list item.
func isListLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	// Ordered lists: a digit-led line with ". " in the first four characters.
	// The len>=4 guard keeps trimmed[:4] in bounds — a bare "1. " (three
	// chars) would otherwise slice past the string and panic.
	return len(trimmed) >= 4 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:4], ". ")
}

// isBlockSyntaxLine reports whether the trimmed line opens one of the
// remaining block constructs: a heading, a table, a blockquote, or a
// thematic break.
func isBlockSyntaxLine(trimmed string) bool {
	switch trimmed[0] {
	case '#', '|', '>':
		return true
	}
	return trimmed == "---" || trimmed == "***" || trimmed == "___"
}

func isFence(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func isHTMLComment(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "<!--")
}

// wrapWords splits text at word boundaries to fit within width.
func wrapWords(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		if len(current)+1+len(w) > width {
			lines = append(lines, current)
			current = w
		} else {
			current += " " + w
		}
	}
	lines = append(lines, current)
	return lines
}
