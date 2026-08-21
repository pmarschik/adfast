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
  catalog(),              // JSON [{name, level, kind, decodedByCore}]
  toADF(md, opts),        // ADF JSON
  toMarkdown(adf, opts),  // markdown text
  diagnostics(md, opts),  // JSON [{code, message}]
};
```

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

`scanSpans` returns each offset in **UTF-16 code units**. That is the
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
