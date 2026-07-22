package jira

import (
	adfast "github.com/pmarschik/adfast"
)

// RichTextFormat identifies how a Jira rich-text field value is encoded:
// ADF documents on cloud v3 APIs, plain text elsewhere. The underlying
// strings are the historical wire-facing identifiers, so existing
// serialized configuration keeps working.
type RichTextFormat string

const (
	// RichTextADF is a rich-text field carrying an ADF document
	// (map[string]any with type "doc").
	RichTextADF RichTextFormat = "adf"
	// RichTextText is a rich-text field carrying a plain string; the
	// markdown source is submitted as-is (trailing whitespace trimmed).
	RichTextText RichTextFormat = "text"
)

// InferRichTextFormat inspects an existing Jira field value and returns
// RichTextADF when it looks like an ADF document (a JSON object with
// type "doc"), RichTextText otherwise — including for nil, so absent
// fields default to plain text.
func InferRichTextFormat(existingValue any) RichTextFormat {
	m, ok := existingValue.(map[string]any)
	if !ok {
		return RichTextText
	}
	if m["type"] == "doc" {
		return RichTextADF
	}
	return RichTextText
}

// EncodeRichText encodes markdown for a Jira rich-text field in the
// given format: RichTextADF converts through
// adfast.ToADF(adfast.FromMarkdown(...)) (with the supplied options),
// anything else — RichTextText or an unknown format — submits the
// markdown as a plain string with trailing whitespace trimmed. Pair with
// InferRichTextFormat to match whatever format the field currently holds.
func EncodeRichText(markdown string, format RichTextFormat, opts ...adfast.Option) any {
	if format == RichTextADF {
		return adfast.ToADF(adfast.FromMarkdown(markdown, opts...), opts...)
	}
	return trimEnd(markdown)
}

// trimEnd trims trailing spaces, tabs, and line endings.
func trimEnd(s string) string {
	for s != "" && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
