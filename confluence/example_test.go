package confluence_test

import (
	"encoding/json"
	"fmt"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
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

// RepairReadBack restores what a page read drops. Here the ADF read gave
// the link mark alone, and the storage body of the same page version
// still holds the <code> around the link.
func ExampleRepairReadBack() {
	read := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"errors.Is","marks":[` +
		`{"type":"link","attrs":{"href":"https://pkg.go.dev/errors"}}]}]}]}`
	storage := `<p><code><a href="https://pkg.go.dev/errors">errors.Is</a></code></p>`

	var doc adf.Doc
	if err := json.Unmarshal([]byte(read), &doc); err != nil {
		panic(err)
	}
	doc = confluence.RepairReadBack(doc, storage)
	fmt.Print(adfast.ToMarkdown(adfast.FromADF(doc, confluence.RenderOptions()...), confluence.RenderOptions()...))
	// Output:
	// [`errors.Is`](https://pkg.go.dev/errors)
}
