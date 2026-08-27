# adfast/wasm

This module exposes the Markdown ⇄ ADF conversion of adfast, and its
directive dialect, to JavaScript. An editor integration can convert
documents and locate directives without a second copy of the dialect in
TypeScript. The attribute grammar (quoting, escaping, `parameters` JSON)
is intricate and rots fast, so a compile of the real parser removes that
class of drift.

This is a **separate Go module** (`github.com/pmarschik/adfast/wasm`).
Unlike the other submodules it is a build artifact, not a library: no Go
code imports it.

## Build

```sh
mise run wasm:build     # writes wasm/adfast.wasm
mise run wasm:smoke     # builds, then drives it through Node
```

To build by hand, run these commands from this directory:

```sh
GOOS=js GOARCH=wasm go build -o adfast.wasm .
node smoke.mjs
```

Ship `adfast.wasm` together with `wasm_exec.js`.

## JS surface

```js
globalThis.adfast = {
  scanSpans(md),          // JSON [{start, end, level, name, attrs}]
  codeSpans(md),          // JSON [{start, end}]
  headings(md),           // JSON [{start, end, textStart, textEnd, level}]
  images(md),             // JSON [{start, end, altStart, altEnd, destStart, destEnd}]
  catalog(),              // JSON [{name, level, kind, decodedByCore}]
  toADF(md, opts),        // ADF JSON
  toMarkdown(adf, opts),  // markdown text
  diagnostics(md, opts),  // JSON [{code, message}]
};
```

The keys above are listed for reading, not in emission order: a JSON
object is unordered, so only the key **names** are part of the surface.

Every export returns a plain object that the caller **branches on**. No
export throws:

```js
{ ok: true,  value: "…" }
{ ok: false, error: "…" }
```

A panic that reaches a `js.FuncOf` boundary takes down the whole WASM
instance, and the page can then recover only by a reload. Therefore
every export runs inside a recover and reports the failure as a value.

`value` is always a string. Documents and arrays come back as JSON text
for the caller to `JSON.parse`. `syscall/js` has no bulk marshaling, so
a nested JS object costs one boundary crossing per key. A `JSON.parse`
of one string is a single crossing plus a native parse.

`adf` can be a JSON string or a live JS object. `opts` can be omitted.
When it is present, it is `{product, baseUrl, expandMode}`, where
`product` is `"jira"`, `"confluence"`, or absent for the
platform-neutral behavior. This module does **not** decide which product
a document belongs to. That is the work of the consumer. An unrecognized
`product` or `expandMode` gives an error result, never a silent
fallback.

`toMarkdown` and `diagnostics` have no consumer in the first plugin
release. They are in the initial surface deliberately, so that ADF
import and export and inline diagnostics can land later without a change
to the JS API.

### Span offsets are UTF-16 code units, not bytes

Every export that reports a span returns each offset in **UTF-16 code
units** — `scanSpans`, `codeSpans`, `headings`, and `images` alike. That
is the
unit CodeMirror 6, the DOM, and the `String` type of JavaScript index
by. The offsets are directly usable as CodeMirror positions, and
`md.slice(span.start, span.end)` gives the source text of the directive.

goldmark reports **byte** offsets into the UTF-8 source. The conversion
happens on the Go side, because that side is the only one that holds
both representations at once: `syscall/js` has already transcoded the
UTF-16 string of JS into the UTF-8 of Go on the way in, so the mapping
costs one pass over a string that was walked anyway. In JavaScript the
same work needs a re-encode of the whole document to UTF-8, only to undo
it.

This is not academic. The corpus of adfast is full of emoji and
non-ASCII text, and a silent mismatch shifts every widget after the
first multi-byte character.

`start` and `end` delimit the **full extent** of the directive: the
whole line for a leaf, the whole run for a text directive, and, for a
container, the opening fence through the end of the matching closing
fence. An unclosed container is what the buffer looks like mid-keystroke
and has no closing fence at all. Its `end` is the end of the enclosing
container, or the end of the source.

### `codeSpans()` is the parser's verdict on what is code

`codeSpans(md)` returns the extent of every code block — fenced and
indented alike — in document order, no two overlapping:

```js
{ start: 12, end: 47 }
```

The name follows the Go API (`markdown.CodeSpans`). Note that in
CommonMark a "code span" is _inline_ code; these are code **blocks**, and
inline code is not reported.

A span covers **whole lines**: from the first byte of the line the block
opens on, through the newline of the line it closes on. It therefore
includes the fence delimiter lines, an indented block's indent, and the
container prefix of a blockquote or list item on those lines.

The point of the export is that it is the same walk the conversion path
uses, not a regexp. A line scanner disagrees with CommonMark in both
directions — a closing fence carrying an info string does _not_ close its
block, and four-space content inside a blockquoted list item _is_ code —
and an editor integration that guesses wrong offers completions inside a
documented example.

### `headings()` is the parser's verdict on what is a heading

`headings(md)` returns every heading — ATX (`## Title`) and setext
(`Title` over `-----`) alike — in document order:

```js
{ start: 0, end: 12, textStart: 3, textEnd: 11, level: 2 }
```

`start`/`end` cover **whole lines**, the same rule `codeSpans` follows:
from the first byte of the line the heading opens on, through the
newline of the line it ends on — for a setext heading, its underline
line. `textStart`/`textEnd` are **tight**: the `#` markers, the padding,
an ATX heading's optional closing `###` run, and the setext underline
are all outside them, so `md.slice(textStart, textEnd)` is the heading's
raw written text with its inline markup intact and nothing transformed.
A heading with no text at all (`#`) reports `textStart === textEnd`.

A `#` line inside a code block is not a heading and is not reported, and
neither is `#no-space`, which CommonMark reads as a paragraph. A setext
heading is one, which an ATX-only regexp never sees.

### `images()` reports tight spans, because a caller rewrites in place

`images(md)` returns every image in document order:

```js
{ start: 7, end: 18, altStart: 9, altEnd: 10, destStart: 12, destEnd: 17 }
```

Every offset here is **tight**, which is the opposite of the whole-line
rule `codeSpans` and `headings` follow. Those two name blocks; an image
is an inline, and the only thing a caller does with one is replace a
piece of it in place. `start`/`end` therefore run from the `!` through
one past the closing `)` or `]` and never reach the prose around it —
`md.slice(0, start) + replacement + md.slice(end)` leaves the sentence
intact.

`destStart`/`destEnd` cover the destination **as written**, with any
wrapping `<…>` outside them, so splicing a new path in keeps the
brackets a path with spaces needs. A REFERENCE image (`![alt][id]`,
`![alt][]`, `![alt]`) reports `destStart === destEnd === 0`: its
destination lives at the link definition, so there is nothing here to
rewrite. The sentinel is unambiguous, because at least `![](` precedes
an inline image's destination. An inline image with an empty
destination (`![alt]()`) instead reports an empty range at the offset
where a destination would go, so an insertion there works.

An `![…](…)` written inside inline code, inside a code block, or inside
an HTML comment is not an image and is not reported, and neither is a
reference image whose definition is missing — all four of which a regexp
over the source finds.

### `catalog()` is the semantic half of `scanSpans`

`scanSpans` is purely syntactic: `:::info` and `:::frobnicate` look
alike to it. `catalog()` takes no arguments and returns every directive
the dialect registers, one entry per `(name, level)` pair, sorted by
name and then by level:

```js
{ name: "info", level: 3, kind: "panel", decodedByCore: false }
```

- `name` is the same value `scanSpans` reports, so `(name, level)` joins
  a span to its entry.
- `level` is `1` for text, `2` for a leaf, and `3` for a container. One
  name can register at several levels with a different `kind` at each.
  `media` is `mediaInline` as a text directive and `media` as a leaf or
  a container.
- `kind` is the dialect kind the directive promotes to. All five panel
  names therefore share `"panel"`, and a consumer can style by kind
  instead of by an enumeration of names.
- `decodedByCore` marks the kinds `convert` handles structurally in the
  ADF → Markdown direction (`::colwidths`, `::decisions`, which have no
  ADF node of their own). It has no effect on Markdown → ADF.

The list is derived from `dialect.Registrations()` at call time, so a
plugin that binds names to visuals cannot fall behind the dialect. A
hand-maintained TypeScript table can. Attribute schemas are out of scope
deliberately: they live inside the promote functions, not the
registration, so an entry describes identity and nothing more.

## Layout

| File           | Role                                                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `api.go`       | **No build tag.** Every decision: option mapping, span extraction, offset conversion, JSON and error shaping. Table-tested on darwin/linux. |
| `main.go`      | `//go:build js && wasm`. `js.FuncOf` registration only — no logic.                                                                          |
| `main_host.go` | The `func main()` stub the non-js build needs so `go build ./...` passes.                                                                   |
| `api_test.go`  | Ordinary table tests; they run wherever `go test` runs.                                                                                     |
| `smoke.mjs`    | Node smoke test driving the built `.wasm` through `wasm_exec.js`.                                                                           |
| `wasm_exec.js` | Vendored from the Go toolchain — see below.                                                                                                 |

The split is load-bearing. A package tagged `js && wasm` is invisible to
`go test ./...` on a CI host, so anything behind that tag is effectively
untested. A logic-free `main.go` bounds the untested surface to glue, and
`smoke.mjs` covers that glue.

## Vendored `wasm_exec.js`

|            |                                                                    |
| ---------- | ------------------------------------------------------------------ |
| Source     | `$(go env GOROOT)/lib/wasm/wasm_exec.js`                           |
| Go version | **go1.27.0**                                                       |
| sha256     | `0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14` |

`wasm_exec.js` is **coupled to the Go toolchain that built the binary**.
The host bindings it installs must match what the runtime expects. It is
pinned in three places that must move together: `.config/mise/config.toml`
(`go = "1.27.0"`), the `go-version` of the `wasm` job in
`.github/workflows/ci.yml`, and the table above. CI diffs the vendored
copy against the copy of its own toolchain and fails loudly on drift, so
a Go upgrade cannot break the smoke test in a confusing way. To
re-vendor the file, run

```sh
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/
```

and update the table. Never edit the file by hand.

## Measured

`go1.27.0`, Apple Silicon, Node 26:

|         |                                                         |
| ------- | ------------------------------------------------------- |
| Size    | 8.0 MB raw / 1.9 MB gzipped                             |
| Boot    | ~13 ms instantiate + ~27 ms `go.run`                    |
| `toADF` | 0.46 ms for a 150-character document, 3.6 ms for 1.2 KB |

Sub-millisecond conversion of a typical viewport chunk is inside a
live-preview budget, so a plugin can call in on each update instead of
a cache.
