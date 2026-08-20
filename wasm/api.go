// Command adfast-wasm exposes adfast's Markdown ⇄ ADF conversion and its
// directive dialect to JavaScript, so editor integrations can convert
// documents and locate directives without re-implementing the dialect in
// TypeScript.
//
// The module is deliberately split in two:
//
//   - api.go (this file) carries EVERY decision — option mapping, span
//     extraction, offset conversion, JSON and error shaping — and has NO
//     build tag, so it compiles and is table-tested on darwin/linux like
//     any other package.
//   - main.go is `//go:build js && wasm` and holds nothing but the
//     js.FuncOf registration. A js/wasm-tagged file is invisible to
//     `go test ./...` on a CI host, so anything behind that tag is
//     effectively untested; keeping it logic-free bounds the risk to
//     glue, which the Node smoke test covers.
//   - main_host.go supplies the `func main()` the non-js build needs so
//     `go build ./...` stays green on the host.
//
// # JS surface
//
//	globalThis.adfast = {
//	  scanSpans(md)                 -> Result   // JSON [{start,end,level,name,attrs}]
//	  catalog()                     -> Result   // JSON [{name,level,kind,decodedByCore}]
//	  toADF(md, opts)               -> Result   // ADF JSON
//	  toMarkdown(adf, opts)         -> Result   // markdown text
//	  diagnostics(md, opts)         -> Result   // JSON [{code,message}]
//	}
//
// Every export returns a plain object the caller branches on rather than
// throwing:
//
//	{ok: true,  value: "…"}
//	{ok: false, error: "…"}
//
// `value` is always a string. Documents and arrays are returned as JSON
// text for the caller to `JSON.parse`: syscall/js has no bulk marshaling,
// so building nested JS objects costs one boundary crossing per key,
// while `JSON.parse` of one string is a single crossing plus a native
// parse.
//
// `adf` may be passed to toMarkdown as either a JSON string or a live JS
// object (main.go runs `JSON.stringify` on anything that is not a
// string). `opts` may be omitted.
package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	directive "github.com/pmarschik/goldmark-directive"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/confluence"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/jira"
	"github.com/pmarschik/adfast/markdown"
)

// Directive nesting levels, matching the dialect's own vocabulary (see
// skill/assets/references/syntax.md) and the colon count that opens each
// form.
const (
	// LevelText is an inline `:name[label]{attrs}` text directive.
	LevelText = 1
	// LevelLeaf is a standalone `::name[label]{attrs}` line.
	LevelLeaf = 2
	// LevelContainer is a `:::name` … `:::` block.
	LevelContainer = 3
)

// Span locates one directive in a Markdown source.
//
// # Offsets are UTF-16 code units, NOT bytes
//
// Start and End are offsets in UTF-16 code units — the unit CodeMirror 6,
// the DOM, and JavaScript's own String index by — measured from the start
// of the source string. They are directly usable as CodeMirror positions;
// `md.slice(span.start, span.end)` in JavaScript yields the directive's
// source text.
//
// This is a DELIBERATE conversion: goldmark reports byte offsets into the
// UTF-8 source, and ScanSpans converts them here. The conversion belongs
// on this side because this side is the only one holding both
// representations at once — syscall/js has already transcoded the JS
// UTF-16 string into Go's UTF-8 on the way in, so the mapping costs one
// pass over a string that was walked anyway. Doing it in JavaScript would
// mean re-encoding the whole document to UTF-8 just to undo it. Every
// emoji, accented letter, and CJK character before a directive would
// otherwise shift its widget, and adfast's corpus is full of all three.
//
// Start and End delimit the directive's FULL extent: the whole
// `::name[…]{…}` line for a leaf, the whole `:name[…]{…}` run for a text
// directive, and — for a container — the opening fence through the end of
// the matching closing fence. An unclosed container has no closing fence
// at all; its End is the end of the enclosing container, or the end of the
// source when it is top level.
//
// Spans arrive in document order (ascending Start), which is the order
// CodeMirror's RangeSetBuilder wants. A container is emitted before the
// directives nested inside it.
//
//nolint:govet // fieldalignment: the declaration order IS the published JSON key order (start, end, level, name, attrs); saving 8 bytes on a struct that exists to be marshaled is not worth reordering a documented wire surface.
type Span struct {
	// Start is the UTF-16 code unit offset of the directive's first
	// character.
	Start int `json:"start"`
	// End is the UTF-16 code unit offset one past the directive's last
	// character.
	End int `json:"end"`
	// Level is LevelContainer (3), LevelLeaf (2), or LevelText (1).
	Level int `json:"level"`
	// Name is the directive name, without its colons ("info", "status", …).
	Name string `json:"name"`
	// Attrs are the directive's parsed attributes, exactly as the dialect
	// grammar reads them (quoting, escaping, and `#id`/bare shorthands
	// already resolved). Never nil — a directive without attributes has an
	// empty object, so `span.attrs.color` needs no guard.
	Attrs map[string]string `json:"attrs"`
}

// CatalogEntry names one directive the dialect registers, at one level.
//
// ScanSpans is purely SYNTACTIC: a span carries a name, a level and
// attributes, but nothing about what the directive means — `:::info` and
// `:::frobnicate` look alike to a consumer. The catalog is the semantic
// half, so an editor integration can bind names to visuals from the
// dialect itself instead of keeping a parallel table in TypeScript that
// drifts silently.
//
// Attribute schemas are deliberately out of scope: they live inside the
// promote functions, not in the registration, so an entry describes the
// directive's identity (name, level, ADF kind) and nothing more.
//
//nolint:govet // fieldalignment: as with Span, the declaration order is the published JSON key order.
type CatalogEntry struct {
	// Name is the directive name, without its colons ("info", "status", …)
	// — the same value Span.Name carries.
	Name string `json:"name"`
	// Level is LevelContainer (3), LevelLeaf (2), or LevelText (1). A name
	// can be registered at more than one level, with a different kind at
	// each ("media" is the media kind as a leaf or container and the
	// mediaInline kind as a text directive), so (name, level) — the pair a
	// span carries — is what identifies an entry.
	Level int `json:"level"`
	// Kind is the dialect kind the directive promotes to
	// (extension.Registration.Kind), e.g. "panel" for every panel name.
	Kind string `json:"kind"`
	// DecodedByCore reports that the ADF → Markdown direction is handled
	// structurally by convert rather than by the kind's own decode hook
	// (extension.Registration.DecodedByCore) — true for the cross-sibling
	// kinds ":colwidths" and "::decisions", which have no ADF node of
	// their own. It does NOT affect the Markdown → ADF direction.
	DecodedByCore bool `json:"decodedByCore"`
}

// Catalog returns every directive name the dialect registers, one entry
// per (name, level) pair, sorted by name and then level.
//
// It is derived from dialect.Registrations() at call time — there is no
// hand-maintained table here to fall behind the dialect. Where two
// registrations claim the same name at the same level the LAST one wins,
// mirroring the parser's own promotion index (see markdown.Parse), so the
// catalog always describes the promotion that actually happens.
func Catalog() []CatalogEntry {
	type key struct {
		name  string
		level int
	}
	seen := map[key]CatalogEntry{}
	for _, reg := range dialect.Registrations() {
		for name := range reg.Texts {
			seen[key{name, LevelText}] = CatalogEntry{name, LevelText, reg.Kind, reg.DecodedByCore}
		}
		for name := range reg.Leaves {
			seen[key{name, LevelLeaf}] = CatalogEntry{name, LevelLeaf, reg.Kind, reg.DecodedByCore}
		}
		for name := range reg.Containers {
			seen[key{name, LevelContainer}] = CatalogEntry{name, LevelContainer, reg.Kind, reg.DecodedByCore}
		}
	}
	out := make([]CatalogEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b CatalogEntry) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), a.Level-b.Level)
	})
	return out
}

// Diagnostic is one notice raised while converting a document.
//
// Start and End are omitted: adfast's diagnostics do not currently carry
// source positions, and inventing offsets would be worse than having
// none. They are declared so positioned diagnostics can arrive later
// without changing the JS surface; when present they follow Span's
// UTF-16 code unit contract.
//
//nolint:govet // fieldalignment: as with Span, the declaration order is the published JSON key order.
type Diagnostic struct {
	// Code is the stable diagnostic code (convert.Code* / adf.Code*), e.g.
	// "unsupported-in-product" or "unknown-node".
	Code string `json:"code"`
	// Message is the human-readable explanation.
	Message string `json:"message"`
	// Start is the diagnostic's UTF-16 start offset when known.
	Start *int `json:"start,omitempty"`
	// End is the diagnostic's UTF-16 end offset when known.
	End *int `json:"end,omitempty"`
}

// Product names accepted by Options.Product.
const (
	// ProductJira selects jira.MarkdownOptions / jira.RenderOptions.
	ProductJira = "jira"
	// ProductConfluence selects confluence.MarkdownOptions /
	// confluence.RenderOptions.
	ProductConfluence = "confluence"
)

// Options is the JS `opts` object: the product bundle to convert under.
//
// This module does NOT decide which product a document belongs to — that
// is the consumer's job (storysmith-md resolves it from doc roots and
// issue buckets). Options is the only place the baseline knows product
// names at all; an absent Product means the platform-neutral root
// behavior, which is the correct default for a document that is neither.
type Options struct {
	// Product is ProductJira, ProductConfluence, or "" for neither.
	Product string `json:"product"`
	// BaseURL is the product site the smart-link conventions resolve
	// against, e.g. "https://hive.atlassian.net".
	BaseURL string `json:"baseUrl"`
	// ExpandMode is the Jira bare-issue-key expansion mode ("auto", "all",
	// or "explicit"); it defaults to "auto" and is ignored for other
	// products.
	ExpandMode string `json:"expandMode"`
}

// encodeOptions returns the facade options for the md → ADF direction
// (FromMarkdown + ToADF), and an error for an unrecognized product or
// expand mode — a typo that silently fell back to the neutral bundle
// would be exactly the drift this module exists to remove.
func (o Options) encodeOptions() ([]adfast.Option, error) {
	switch o.Product {
	case "":
		return nil, nil
	case ProductConfluence:
		return confluence.MarkdownOptions(o.BaseURL), nil
	case ProductJira:
		mode, err := o.expandMode()
		if err != nil {
			return nil, err
		}
		return jira.MarkdownOptions(o.BaseURL, mode), nil
	default:
		return nil, fmt.Errorf(
			"unknown product %q: want %q, %q, or an empty string",
			o.Product, ProductJira, ProductConfluence,
		)
	}
}

// renderOptions returns the facade options for the ADF → md direction
// (FromADF + ToMarkdown). The product bundles split by direction — the
// decode side is RenderOptions, which labels smart-link cards and (for
// Confluence) decodes the core macros back to their sugared directives —
// so the same `opts` object selects a different bundle here.
func (o Options) renderOptions() ([]adfast.Option, error) {
	switch o.Product {
	case "":
		return nil, nil
	case ProductConfluence:
		return confluence.RenderOptions(), nil
	case ProductJira:
		// Validated for symmetry: the same opts object drives both
		// directions, so a bad mode must not pass silently one way.
		if _, err := o.expandMode(); err != nil {
			return nil, err
		}
		return jira.RenderOptions(), nil
	default:
		return nil, fmt.Errorf(
			"unknown product %q: want %q, %q, or an empty string",
			o.Product, ProductJira, ProductConfluence,
		)
	}
}

// expandMode resolves Options.ExpandMode, defaulting to jira.ExpandAuto.
func (o Options) expandMode() (jira.ExpandMode, error) {
	switch m := jira.ExpandMode(o.ExpandMode); m {
	case "":
		return jira.ExpandAuto, nil
	case jira.ExpandAuto, jira.ExpandAll, jira.ExpandExplicit:
		return m, nil
	default:
		return "", fmt.Errorf(
			"unknown expandMode %q: want %q, %q, %q, or an empty string",
			o.ExpandMode, jira.ExpandAuto, jira.ExpandAll, jira.ExpandExplicit,
		)
	}
}

// ScanSpans locates every dialect directive in md.
//
// Offsets in the returned spans are UTF-16 code units, not bytes — see
// Span for why the conversion happens here.
//
// The parse is the same goldmark assembly the conversion path uses, so
// what ScanSpans reports as a directive is exactly what ToADF will treat
// as one. Directives nested inside a TEXT directive's label are not
// reported: goldmark parses a label against its own detached source, so
// their offsets do not refer to md.
func ScanSpans(md string) []Span {
	src := []byte(md)
	tree := markdown.NewParser().Parse(text.NewReader(src))
	spans := collectSpans(tree, len(src))
	toUTF16(md, spans)
	return spans
}

// containerEnd resolves a container directive's full extent.
// ContainerDirective.Span covers the OPENING FENCE LINE ONLY — the block's
// end is not known when it opens — so the extent ends at the matching
// CloseFence, which the parser emits as the container's next sibling. An
// unclosed container emits no CloseFence at all (a real input while the
// user is still typing the block), and falls back to the enclosing extent.
func containerEnd(cd *directive.ContainerDirective, fallback int) int {
	if fence, ok := cd.NextSibling().(*directive.CloseFence); ok {
		return fence.Span.Stop
	}
	return fallback
}

// collectSpans walks the goldmark tree in document order and emits one
// span per directive node, with BYTE offsets. enclosingEnd is the extent
// a container nested here falls back to when it is unclosed — the source
// length at the top level.
//
// The traversal is written out rather than delegated to gast.Walk because
// it threads that fallback down the tree, and because a text directive's
// label is parsed against its own detached source: goldmark keeps those
// inlines off the node's child list, so they are never reached here and
// their offsets (which do not refer to this source) can never leak out.
func collectSpans(n gast.Node, enclosingEnd int) []Span {
	var out []Span
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		childEnd := enclosingEnd
		switch v := c.(type) {
		case *directive.ContainerDirective:
			childEnd = containerEnd(v, enclosingEnd)
			out = append(out, Span{
				Start: v.Span.Start, End: childEnd,
				Level: LevelContainer, Name: v.Name, Attrs: attrsOf(v.Attrs),
			})
		case *directive.LeafDirective:
			out = append(out, Span{
				Start: v.Span.Start, End: v.Span.Stop,
				Level: LevelLeaf, Name: v.Name, Attrs: attrsOf(v.Attrs),
			})
		case *directive.TextDirective:
			out = append(out, Span{
				Start: v.Span.Start, End: v.Span.Stop,
				Level: LevelText, Name: v.Name, Attrs: attrsOf(v.Attrs),
			})
		}
		out = append(out, collectSpans(c, childEnd)...)
	}
	return out
}

// attrsOf normalizes a directive's attribute map so the JSON always
// carries an object rather than null.
func attrsOf(a map[string]string) map[string]string {
	if a == nil {
		return map[string]string{}
	}
	return a
}

// toUTF16 rewrites the byte offsets in spans to UTF-16 code unit offsets.
// An all-ASCII source needs no work: the two units coincide.
func toUTF16(src string, spans []Span) {
	if len(spans) == 0 || isASCII(src) {
		return
	}
	wanted := make([]int, 0, len(spans)*2)
	for _, s := range spans {
		wanted = append(wanted, s.Start, s.End)
	}
	slices.Sort(wanted)
	wanted = slices.Compact(wanted)

	idx := utf16Offsets(src, wanted)
	for i := range spans {
		spans[i].Start = idx[spans[i].Start]
		spans[i].End = idx[spans[i].End]
	}
}

// utf16Offsets maps each byte offset in wanted (which must be sorted and
// deduplicated) to its UTF-16 code unit offset, in a single pass over src.
// Offsets at or past the end of src map to the source's total UTF-16
// length; an offset inside a multi-byte rune maps to that rune's start.
func utf16Offsets(src string, wanted []int) map[int]int {
	out := make(map[int]int, len(wanted))
	w, u16 := 0, 0
	for b, r := range src {
		for w < len(wanted) && wanted[w] <= b {
			out[wanted[w]] = u16
			w++
		}
		if r > 0xFFFF {
			// Beyond the BMP: JavaScript stores it as a surrogate pair.
			u16 += 2
		} else {
			u16++
		}
	}
	for w < len(wanted) {
		out[wanted[w]] = u16
		w++
	}
	return out
}

// isASCII reports whether s is all single-byte runes, in which case byte
// and UTF-16 offsets are identical.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// ToADF converts a Markdown document to an ADF document under the given
// product options, returning the wire-format ADF JSON.
func ToADF(md string, opts Options) (string, error) {
	o, err := opts.encodeOptions()
	if err != nil {
		return "", err
	}
	doc := adfast.ToADF(adfast.FromMarkdown(md, o...), o...)
	b, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("encoding ADF to JSON: %w", err)
	}
	return string(b), nil
}

// ErrNotADocument reports that ToMarkdown's input parsed as JSON but is
// not a JSON object, so it cannot be an ADF document. adf.DecodeDoc is
// deliberately permissive about an object's contents (unknown nodes,
// marks, and attributes survive as RawNode/RawMark/Extra), so an object
// that merely lacks content renders as an empty document rather than an
// error.
var ErrNotADocument = errors.New(
	`value is not an ADF document (want a JSON object like {"type":"doc","version":1,"content":[…]})`,
)

// ToMarkdown converts a wire-format ADF JSON document back to Markdown
// under the given product options.
func ToMarkdown(adfJSON string, opts Options) (string, error) {
	o, err := opts.renderOptions()
	if err != nil {
		return "", err
	}
	var v any
	if err := json.Unmarshal([]byte(adfJSON), &v); err != nil {
		return "", fmt.Errorf("parsing ADF JSON: %w", err)
	}
	doc, ok := adf.DecodeDoc(v)
	if !ok {
		return "", ErrNotADocument
	}
	return adfast.ToMarkdown(adfast.FromADF(doc, o...), o...), nil
}

// Diagnostics reports the notices raised converting md to ADF under the
// given product options — the md → ADF direction, which is where the
// product-aware notices (unsupported-in-product,
// unsupported-code-language, unresolved-asset, …) arise.
//
// The returned slice is never nil, so the JSON is always an array.
func Diagnostics(md string, opts Options) ([]Diagnostic, error) {
	o, err := opts.encodeOptions()
	if err != nil {
		return nil, err
	}
	out := []Diagnostic{}
	sink := adfast.WithDiagnostics(func(d convert.Diagnostic) {
		out = append(out, Diagnostic{Code: d.Code, Message: d.Message})
	})
	all := make([]adfast.Option, 0, len(o)+1)
	all = append(all, o...)
	all = append(all, sink)
	adfast.ToADF(adfast.FromMarkdown(md, all...), all...)
	return out, nil
}

// ---------------------------------------------------------------------------
// Bridge layer — the string-in/string-out shapes main.go's js.FuncOf
// wrappers dispatch through. Keeping the argument decoding and result
// shaping here (rather than in the js-tagged file) is what makes them
// testable on the host.
// ---------------------------------------------------------------------------

// bridgeResult shapes an export's outcome as the JS object every export
// returns. syscall/js converts the map on the way out, so the caller sees
// `{ok: true, value: "…"}` or `{ok: false, error: "…"}` and branches on
// `ok` instead of catching.
func bridgeResult(value string, err error) map[string]any {
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "value": value}
}

// bridgeGuard runs one export and converts a panic into an error result.
// A panic that reaches a js.FuncOf boundary takes down the whole WASM
// instance, and the page can only recover by reloading — so every export
// goes through here.
func bridgeGuard(fn func() (string, error)) (out map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			out = bridgeResult("", fmt.Errorf("adfast panicked: %v", r))
		}
	}()
	return bridgeResult(fn())
}

// bridgeOptions decodes the JSON form of the JS `opts` object. An empty
// string (an omitted, undefined, or null argument) means no options.
func bridgeOptions(optsJSON string) (Options, error) {
	var o Options
	if optsJSON == "" || optsJSON == "null" || optsJSON == "undefined" {
		return o, nil
	}
	if err := json.Unmarshal([]byte(optsJSON), &o); err != nil {
		return o, fmt.Errorf("parsing opts: %w", err)
	}
	return o, nil
}

// bridgeScanSpans backs globalThis.adfast.scanSpans.
func bridgeScanSpans(md string) (string, error) {
	return marshalJSON(ScanSpans(md))
}

// bridgeCatalog backs globalThis.adfast.catalog.
func bridgeCatalog() (string, error) {
	return marshalJSON(Catalog())
}

// bridgeToADF backs globalThis.adfast.toADF.
func bridgeToADF(md, optsJSON string) (string, error) {
	o, err := bridgeOptions(optsJSON)
	if err != nil {
		return "", err
	}
	return ToADF(md, o)
}

// bridgeToMarkdown backs globalThis.adfast.toMarkdown.
func bridgeToMarkdown(adfJSON, optsJSON string) (string, error) {
	o, err := bridgeOptions(optsJSON)
	if err != nil {
		return "", err
	}
	return ToMarkdown(adfJSON, o)
}

// bridgeDiagnostics backs globalThis.adfast.diagnostics.
func bridgeDiagnostics(md, optsJSON string) (string, error) {
	o, err := bridgeOptions(optsJSON)
	if err != nil {
		return "", err
	}
	diags, err := Diagnostics(md, o)
	if err != nil {
		return "", err
	}
	return marshalJSON(diags)
}

// marshalJSON renders a bridge payload as JSON text.
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encoding result to JSON: %w", err)
	}
	return string(b), nil
}
