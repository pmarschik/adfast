package main

import "fmt"

// These examples exercise the api.go surface — the layer that carries
// every decision. The JavaScript exports themselves live behind
// `//go:build js && wasm` and cannot run on the host toolchain at all;
// wasm/smoke.mjs drives those through the vendored wasm_exec.js instead.

// ScanSpans locates every directive in a document so an editor can
// decorate it, without a second directive parser in TypeScript.
func ExampleScanSpans() {
	md := ":::info[Hive check]\nFeed :status[Done]{color=\"green\"}.\n:::\n"
	for _, s := range ScanSpans(md) {
		fmt.Printf("%d %-6s [%d,%d) %v\n", s.Level, s.Name, s.Start, s.End, s.Attrs)
	}
	// Output:
	// 3 info   [0,58) map[]
	// 1 status [25,53) map[color:green]
}

// Span offsets are UTF-16 code units, so they index the source the way
// JavaScript and CodeMirror do, not the way Go does. Here the accented
// letter costs an extra BYTE that the offsets deliberately do not count.
func ExampleScanSpans_utf16Offsets() {
	md := "Héllo :date[2026-04-12]\n"
	s := ScanSpans(md)[0]
	fmt.Println(s.Start, s.End, len(md))
	// Output:
	// 6 23 25
}

// ToADF converts Markdown to wire-format ADF JSON. The opts bundle
// selects a product; the zero value is the platform-neutral behavior.
func ExampleToADF() {
	adfJSON, err := ToADF("Feed :status[Done]{color=\"green\"}.\n", Options{})
	if err != nil {
		panic(err)
	}
	fmt.Println(adfJSON)
	// Output:
	// {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Feed "},{"type":"status","attrs":{"color":"green","text":"Done"}},{"type":"text","text":"."}]}],"version":1}
}

// ToMarkdown is the inverse; the same opts object selects the product,
// and api.go picks the decode-side bundle for it.
func ExampleToMarkdown() {
	adfJSON := `{"type":"doc","version":1,"content":[` +
		`{"type":"panel","attrs":{"panelType":"note"},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Mind the bees."}]}]}]}`
	md, err := ToMarkdown(adfJSON, Options{})
	if err != nil {
		panic(err)
	}
	fmt.Print(md)
	// Output:
	// :::note
	// Mind the bees.
	// :::
}

// Diagnostics drains the conversion's notice sink. Product-aware notices
// only arise once opts selects a product.
func ExampleDiagnostics() {
	diags, err := Diagnostics("```klingon\nqapla'\n```\n", Options{Product: ProductConfluence})
	if err != nil {
		panic(err)
	}
	for _, d := range diags {
		fmt.Println(d.Code)
	}
	// Output:
	// unsupported-code-language
}

// An unrecognized product is an error rather than a silent fallback: the
// JavaScript caller sees {ok: false, error: "…"} and can say so.
func ExampleOptions_unknownProduct() {
	_, err := ToADF("hello\n", Options{Product: "bitbucket"})
	fmt.Println(err)
	// Output:
	// unknown product "bitbucket": want "jira", "confluence", or an empty string
}
