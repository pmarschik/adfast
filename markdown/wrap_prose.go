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
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Frontmatter block (---)
		if i == 0 && line == "---" {
			out = append(out, line)
			i++
			for i < len(lines) {
				out = append(out, lines[i])
				if lines[i] == "---" {
					i++
					break
				}
				i++
			}
			continue
		}

		// Fenced code block
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fence := line[:3]
			out = append(out, line)
			i++
			for i < len(lines) {
				out = append(out, lines[i])
				if strings.HasPrefix(lines[i], fence) && strings.TrimSpace(lines[i]) == fence {
					i++
					break
				}
				i++
			}
			continue
		}

		// HTML comment (single or multi-line)
		if strings.HasPrefix(strings.TrimSpace(line), "<!--") {
			out = append(out, line)
			if !strings.Contains(line, "-->") {
				i++
				for i < len(lines) {
					out = append(out, lines[i])
					if strings.Contains(lines[i], "-->") {
						i++
						break
					}
					i++
				}
				continue
			}
			i++
			continue
		}

		// Non-wrappable lines: headings, lists, tables, blockquotes, blank, thematic breaks
		if isNonWrappable(line) {
			out = append(out, line)
			i++
			continue
		}

		// Paragraph: collect contiguous prose lines and wrap
		var para []string
		for i < len(lines) && !isNonWrappable(lines[i]) && !isFence(lines[i]) && !isHTMLComment(lines[i]) {
			para = append(para, lines[i])
			i++
		}
		wrapped := wrapWords(strings.Join(para, " "), width)
		out = append(out, wrapped...)
	}

	return strings.Join(out, "\n")
}

func isNonWrappable(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	// Indented lines (list continuations, etc.) — preserve as-is
	if line != "" && (line[0] == ' ' || line[0] == '\t') {
		return true
	}
	// Headings
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	// Lists (ordered and unordered)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	// Ordered lists: a digit-led line with ". " in the first four characters.
	// The len>=4 guard keeps trimmed[:4] in bounds — a bare "1. " (three
	// chars) would otherwise slice past the string and panic.
	if len(trimmed) >= 4 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:4], ". ") {
		return true
	}
	// Tables
	if strings.HasPrefix(trimmed, "|") {
		return true
	}
	// Blockquotes
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	// Thematic breaks
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return true
	}
	return false
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
