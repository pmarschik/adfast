package markdown

import (
	"strings"
	"testing"
)

func TestWrapProse_BasicParagraph(t *testing.T) {
	input := "This is a long paragraph that should be wrapped at eighty characters because that is the default width."
	result := WrapProse(input, 40)
	for line := range strings.SplitSeq(result, "\n") {
		if len(line) > 40 && !strings.Contains(line, " ") {
			continue // single long word
		}
		if len(line) > 45 { // allow slight overshoot for word boundaries
			t.Errorf("line too long (%d chars): %q", len(line), line)
		}
	}
}

func TestWrapProse_PreservesFrontmatter(t *testing.T) {
	input := "---\nstatus: Open\nsummary: A very long summary that should not be wrapped ever\n---\n\nSome text."
	result := WrapProse(input, 40)
	if !strings.HasPrefix(result, "---\nstatus: Open\nsummary: A very long summary that should not be wrapped ever\n---") {
		t.Errorf("frontmatter was modified:\n%s", result)
	}
}

func TestWrapProse_PreservesCodeBlock(t *testing.T) {
	input := "```\nthis is a very long line inside a code block that should never be wrapped at all ever\n```"
	result := WrapProse(input, 40)
	if result != input {
		t.Errorf("code block was modified:\ngot:  %q\nwant: %q", result, input)
	}
}

func TestWrapProse_PreservesHeading(t *testing.T) {
	input := "# This is a very long heading that should not be wrapped at all by the prose wrapper"
	result := WrapProse(input, 40)
	if result != input {
		t.Errorf("heading was wrapped:\n%s", result)
	}
}

func TestWrapProse_PreservesList(t *testing.T) {
	input := "- This is a list item that is quite long and should not be wrapped by the wrapper\n- Another item"
	result := WrapProse(input, 40)
	if result != input {
		t.Errorf("list was modified:\n%s", result)
	}
}

func TestWrapProse_PreservesTable(t *testing.T) {
	input := "| Column A | Column B | Very Long Column C That Exceeds Width |\n| --- | --- | --- |"
	result := WrapProse(input, 40)
	if result != input {
		t.Errorf("table was modified:\n%s", result)
	}
}

func TestWrapProse_PreservesHTMLComment(t *testing.T) {
	input := "<!-- adfast:lint-ignore-next-line some-rule -->\nSome text."
	result := WrapProse(input, 40)
	if !strings.HasPrefix(result, "<!-- adfast:lint-ignore-next-line some-rule -->") {
		t.Errorf("HTML comment was modified:\n%s", result)
	}
}

func TestWrapProse_MultiLineHTMLComment(t *testing.T) {
	input := "<!--\nThis is a long multi-line comment that should be preserved as-is\n-->\nSome text."
	result := WrapProse(input, 40)
	if !strings.HasPrefix(result, "<!--\nThis is a long multi-line comment that should be preserved as-is\n-->") {
		t.Errorf("multi-line HTML comment was modified:\n%s", result)
	}
}

func TestWrapProse_FullDocument(t *testing.T) {
	input := `---
status: Open
---

# PROJ-1 Hello World

This is a paragraph with enough words that it should definitely be wrapped at the default width of eighty characters.

## Description

Another paragraph here that is also very long and needs to be wrapped properly at the specified width.

- List item one
- List item two

` + "```go\nfunc main() { fmt.Println(\"this is a long line in code\") }\n```"

	result := WrapProse(input, 80)

	// Frontmatter, heading, list, code should be preserved
	if !strings.Contains(result, "status: Open") {
		t.Error("frontmatter lost")
	}
	if !strings.Contains(result, "# PROJ-1 Hello World") {
		t.Error("heading lost")
	}
	if !strings.Contains(result, "- List item one") {
		t.Error("list lost")
	}
	if !strings.Contains(result, "```go") {
		t.Error("code block lost")
	}
}

func TestWrapProse_ZeroWidth(t *testing.T) {
	input := "Short text."
	result := WrapProse(input, 0)
	if result != input {
		t.Errorf("got %q, want %q", result, input)
	}
}

// TestWrapProse_ThreeCharDigitLine is a regression test for a slice-out-of-range
// panic: isNonWrappable formerly guarded the ordered-list check with only
// len(trimmed) > 2 before slicing trimmed[:4], which panicked on a three-char
// line that starts with a digit (e.g. "1. " or "3 m").
func TestWrapProse_ThreeCharDigitLine(t *testing.T) {
	for _, input := range []string{"1. ", "3 m", "9.x", "12."} {
		func(in string) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("WrapProse panicked on %q: %v", in, r)
				}
			}()
			_ = WrapProse(in, 40)
		}(input)
	}
}
