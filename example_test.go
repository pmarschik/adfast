package adfast_test

import (
	"encoding/json"
	"fmt"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// md→ADF is the composition ToADF(FromMarkdown(md)): FromMarkdown parses
// to the pivot AST, ToADF encodes it to the typed ADF document, and
// json.Marshal produces the wire-format ADF accepted by the REST APIs.
func ExampleToADF() {
	doc := adfast.ToADF(adfast.FromMarkdown("Queen spotted in **Hive B** on the top bars."))
	wire, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(wire))
	// Output:
	// {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Queen spotted in "},{"type":"text","marks":[{"type":"strong"}],"text":"Hive B"},{"type":"text","text":" on the top bars."}]}],"version":1}
}

// adf→md is the composition ToMarkdown(FromADF(doc)): decode the wire
// value with adf.DecodeDoc, FromADF lifts it into the pivot AST, and
// ToMarkdown renders it.
func ExampleToMarkdown() {
	wire := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"Queen spotted in "},
		{"type":"text","marks":[{"type":"strong"}],"text":"Hive B"},
		{"type":"text","text":" on the top bars."}]}]}`
	var v any
	if err := json.Unmarshal([]byte(wire), &v); err != nil {
		panic(err)
	}
	doc, _ := adf.DecodeDoc(v)
	fmt.Print(adfast.ToMarkdown(adfast.FromADF(doc)))
	// Output:
	// Queen spotted in **Hive B** on the top bars.
}

// The prettier md→md formatter is the composition
// ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat(),
// WithPrintWidth(w)): the parse keeps prettier's escapes, ToMarkdown
// canonicalizes with convert.Normalize and serializes with prettier's
// rules.
func ExampleToMarkdown_format() {
	md := "Setup   *notes*: the [stand](https://hive.example.org/stand) keeps landing boards clear of the gravel."
	fmt.Print(adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), adfast.WithPrintWidth(40)),
		adfast.WithPrettierFormat(), adfast.WithPrintWidth(40),
	))
	// Output:
	// Setup _notes_: the
	// [stand](https://hive.example.org/stand)
	// keeps landing boards clear of the
	// gravel.
}

// Non-fatal conversion issues surface through a diagnostics sink; here a
// ::colwidths directive with no following table is dropped and reported
// by ToADF.
func ExampleToADF_diagnostics() {
	var codes []string
	sink := adfast.WithDiagnostics(func(d convert.Diagnostic) { codes = append(codes, d.Code) })
	adfast.ToADF(
		adfast.FromMarkdown("::colwidths[120,80]\n\nNo table follows this directive.", sink),
		sink,
	)
	fmt.Println(codes)
	// Output:
	// [colwidths-orphan]
}

// Ordinary labeled links can use a product-facing href in ADF while keeping
// a stable author-facing destination in Markdown.
func ExampleWithLinkResolver() {
	resolver := adfast.WithLinkResolver(convert.LinkResolver{
		Encode: func(href string) (string, bool) {
			return "/download/attachments/42/report.pdf", href == "report.pdf"
		},
		Decode: func(href string) (string, bool) {
			return "report.pdf", href == "/download/attachments/42/report.pdf"
		},
	})
	doc := adfast.ToADF(adfast.FromMarkdown("[Report](report.pdf)"), resolver)
	fmt.Print(adfast.ToMarkdown(adfast.FromADF(doc, resolver)))
	// Output:
	// [Report](report.pdf)
}
