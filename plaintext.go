package adfast

import (
	"strings"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/markdown"
)

// PlainTextOf projects a Markdown source down to the text a reader sees:
// markup is dropped, text content is kept, and every directive contributes
// the literal ":name" the author typed followed by its label text.
//
// It is the source-level companion of ast.PlainText, which projects an
// already-parsed inline slice. Two things a caller cannot express by
// composing the exported primitives themselves make it its own entry:
//
//   - Directive names survive. FromMarkdown promotes known directive names
//     into their typed dialect kinds, and that promotion is NOT invertible —
//     :::info, :::note, :::warning, :::success and :::error all land on
//     dialect.Panel — so a walk over the promoted tree cannot tell which
//     name was written. PlainTextOf therefore parses with
//     markdown.WithGenericDirectives and reads Name off the generic node.
//     This matters most where a directive was never meant as one: a bare
//     intraword colon such as "deploy:status" or "auth0:user:created" parses
//     as a directive, and dropping it would silently delete text from the
//     middle of a sentence.
//   - Code blocks contribute their code. ast.PlainText walks inline content,
//     where a fenced or indented block never appears; here a block's code is
//     included, space-padded so it does not fuse with the prose around it.
//
// The result is whitespace-tidied for reading: runs of spaces collapse to
// one and the ends are trimmed. Line structure inside a code block survives
// that pass. For the untidied concatenation, walk FromMarkdown yourself.
//
// A leading frontmatter block is metadata, not prose, and contributes
// nothing (see WithFrontmatterProvider for what counts as one).
//
// Options read: WithFrontmatterProvider and WithDiagnostics. WithExtensions
// is deliberately NOT read — the generic parse keeps every directive name
// readable, which is exactly what a registration would take away.
func PlainTextOf(md string, opts ...Option) string {
	o := newOptions(opts)
	var b strings.Builder
	writePlainText(&b, parseMarkdownSource(md, o, markdown.WithGenericDirectives()))
	return collapseSpaces(b.String())
}

// writePlainText appends n's reader-visible text to b.
func writePlainText(b *strings.Builder, n ast.Node) {
	switch v := n.(type) {
	case *ast.Text:
		b.WriteString(v.Value)
		return
	case *ast.InlineCode:
		b.WriteString(v.Value)
		return
	case *ast.Code:
		writeCodeText(b, v.Value)
		return
	case *ast.ContainerDirective:
		b.WriteString(":" + v.Name)
	case *ast.LeafDirective:
		b.WriteString(":" + v.Name)
	case *ast.TextDirective:
		b.WriteString(":" + v.Name)
	}
	for _, c := range ast.Children(n) {
		writePlainText(b, c)
	}
}

// writeCodeText appends a code block's text, one line at a time, each
// padded with a space on either side. The padding is what keeps the code
// from fusing with the prose on either side of the block: nothing else in
// this projection separates one block from the next.
func writeCodeText(b *strings.Builder, code string) {
	for line := range strings.SplitAfterSeq(code, "\n") {
		if line == "" {
			continue
		}
		b.WriteString(" ")
		b.WriteString(line)
		b.WriteString(" ")
	}
}

// collapseSpaces reduces every run of spaces to a single space and trims
// the ends. Only the space character collapses: a newline inside a code
// block is structure, and survives.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
