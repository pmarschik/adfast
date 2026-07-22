package confluence_test

import (
	"fmt"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/confluence"
)

// MarkdownOptions bundles the Confluence conventions: page links whose
// text is the SPACE/pageID key encode as inlineCards, and
// RenderOptions labels them with the key again on the way back.
func ExampleMarkdownOptions() {
	md := "Stand design: [GDN/98304](https://hive.example.org/wiki/spaces/GDN/pages/98304)"
	doc := adfast.ToADF(adfast.FromMarkdown(md), confluence.MarkdownOptions("https://hive.example.org")...)
	fmt.Print(adfast.ToMarkdown(adfast.FromADF(doc, confluence.RenderOptions()...), confluence.RenderOptions()...))
	// Output:
	// Stand design: [GDN/98304](https://hive.example.org/wiki/spaces/GDN/pages/98304)
}
