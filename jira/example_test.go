package jira_test

import (
	"fmt"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/jira"
)

// MarkdownOptions bundles the Jira conventions: issue links whose text is
// the bare key encode as inlineCards, and RenderOptions labels them with
// the key again on the way back.
func ExampleMarkdownOptions() {
	md := "See [BEE-42](https://hive.example.org/browse/BEE-42) for the stand design."
	enc := jira.MarkdownOptions("https://hive.example.org", "explicit")
	doc := adfast.ToADF(adfast.FromMarkdown(md), enc...)
	fmt.Print(adfast.ToMarkdown(adfast.FromADF(doc, jira.RenderOptions()...), jira.RenderOptions()...))
	// Output:
	// See [BEE-42](https://hive.example.org/browse/BEE-42) for the stand design.
}
