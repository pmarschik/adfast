package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// hasCode reports whether the collected diagnostics contain the code.
func hasCode(diags []convert.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestFromMarkdown_DepthCapOnNestedBlockquotes(t *testing.T) {
	// ~5000 levels of blockquote nesting on one line: without the lift
	// depth cap this overflows the stack (unrecoverable in Go).
	md := strings.Repeat("> ", 5000) + "deep\n"
	var diags []convert.Diagnostic
	doc := mdToADF(md, WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))
	if !hasCode(diags, convert.CodeDepthExceeded) {
		t.Errorf("no depth-exceeded diagnostic; got %v", diags)
	}
	// The truncated document must render without crashing.
	_ = adfToMD(doc)
}

func TestToMarkdown_DepthCapOnNestedADF(t *testing.T) {
	// A deeply nested blockquote ADF document (as JSON bytes, the
	// untrusted wire form): decode must truncate at the cap and diagnose
	// instead of overflowing the stack. 3000 node levels ≈ 6000 JSON
	// nesting levels — well past our 1024 cap while staying under
	// encoding/json's own 10000-level limit (which rejects the document
	// outright as a decode failure, a separate guard).
	const depth = 3000
	var b strings.Builder
	b.WriteString(`{"type":"doc","version":1,"content":[`)
	for range depth {
		b.WriteString(`{"type":"blockquote","content":[`)
	}
	b.WriteString(`{"type":"paragraph","content":[{"type":"text","text":"deep"}]}`)
	for range depth {
		b.WriteString(`]}`)
	}
	b.WriteString(`]}`)

	var diags []convert.Diagnostic
	out := adfToMD([]byte(b.String()), WithDiagnostics(func(d convert.Diagnostic) { diags = append(diags, d) }))
	if !hasCode(diags, convert.CodeDepthExceeded) {
		t.Errorf("no depth-exceeded diagnostic; got %d diagnostics", len(diags))
	}
	if out == "" {
		t.Error("truncated document rendered empty")
	}
}
