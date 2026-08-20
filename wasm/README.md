# adfast/wasm

adfast's Markdown ⇄ ADF conversion and its directive dialect, exposed to
JavaScript. Editor integrations can convert documents and locate
directives without re-implementing the dialect in TypeScript — the
attribute grammar (quoting, escaping, `parameters` JSON) is non-trivial
and rots fast, so compiling the real parser removes that class of drift
entirely.

This is a **separate Go module**
(`github.com/pmarschik/adfast/wasm`) and, unlike the other submodules,
it is a build artifact rather than a library: nothing imports it from Go.

## Build

```sh
mise run wasm:build     # writes wasm/adfast.wasm
mise run wasm:smoke     # builds, then drives it through Node
```

or by hand, from this directory:

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

Every export returns a plain object the caller **branches on** rather
than throwing:

```js
{ ok: true,  value: "…" }
{ ok: false, error: "…" }
```

A panic that reached a `js.FuncOf` boundary would take down the whole
WASM instance, and the page could only recover by reloading — so every
export runs inside a recover and reports the failure as a value.

`value` is always a string. Documents and arrays come back as JSON text
for the caller to `JSON.parse`: `syscall/js` has no bulk marshaling, so
building nested JS objects costs one boundary crossing per key, while
`JSON.parse` of one string is a single crossing plus a native parse.

`adf` may be a JSON string or a live JS object. `opts` may be omitted;
when present it is `{product, baseUrl, expandMode}`, where `product` is
`"jira"`, `"confluence"`, or absent for the platform-neutral behavior.
This module does **not** decide which product a document is — that is the
consumer's job. An unrecognized `product` or `expandMode` is an error
result, not a silent fallback.

`toMarkdown` and `diagnostics` have no consumer in the first plugin
release. They are in the initial surface deliberately, so ADF
import/export and inline diagnostics can land later without changing the
JS API.

### Span offsets are UTF-16 code units, not bytes

`scanSpans` returns offsets in **UTF-16 code units** — the unit
CodeMirror 6, the DOM, and JavaScript's own `String` index by. They are
directly usable as CodeMirror positions, and
`md.slice(span.start, span.end)` yields the directive's source text.

goldmark reports **byte** offsets into the UTF-8 source; the conversion
happens on the Go side because that side is the only one holding both
representations at once — `syscall/js` has already transcoded the JS
UTF-16 string into Go's UTF-8 on the way in, so the mapping costs one
pass over a string that was walked anyway. Doing it in JavaScript would
mean re-encoding the whole document to UTF-8 just to undo it.

This is not academic: adfast's corpus is full of emoji and non-ASCII, and
a silent mismatch shifts every widget after the first multi-byte
character.

`start`/`end` delimit the directive's **full extent** — the whole line
for a leaf, the whole run for a text directive, and for a container the
opening fence through the end of the matching closing fence. An unclosed
container (what the buffer looks like mid-keystroke) has no closing fence
at all; its `end` is the end of the enclosing container, or of the
source.

### `catalog()` is the semantic half of `scanSpans`

`scanSpans` is purely syntactic: `:::info` and `:::frobnicate` look alike
to it. `catalog()` takes no arguments and returns every directive the
dialect registers, one entry per `(name, level)` pair, sorted by name
then level:

```js
{ name: "info", level: 3, kind: "panel", decodedByCore: false }
```

- `name` is the same value `scanSpans` reports, so `(name, level)` joins
  a span to its entry.
- `level` is `1` text, `2` leaf, `3` container. A name can be registered
  at several levels with a different `kind` at each — `media` is
  `mediaInline` as a text directive and `media` as a leaf or container.
- `kind` is the dialect kind the directive promotes to, so all five panel
  names share `"panel"` and a consumer can style by kind instead of
  enumerating names.
- `decodedByCore` marks the kinds `convert` handles structurally in the
  ADF → Markdown direction (`::colwidths`, `::decisions`, which have no
  ADF node of their own). It does not affect Markdown → ADF.

The list is derived from `dialect.Registrations()` at call time, so a
plugin binding names to visuals cannot fall behind the dialect the way a
hand-maintained TypeScript table does. Attribute schemas are deliberately
out of scope — they live inside the promote functions, not the
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

The split is load-bearing: a `js && wasm`-tagged package is invisible to
`go test ./...` on a CI host, so anything behind that tag is effectively
untested. Keeping `main.go` logic-free bounds the untested surface to
glue, and `smoke.mjs` covers that.

## Vendored `wasm_exec.js`

|            |                                                                    |
| ---------- | ------------------------------------------------------------------ |
| Source     | `$(go env GOROOT)/lib/wasm/wasm_exec.js`                           |
| Go version | **go1.26.4**                                                       |
| sha256     | `0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14` |

`wasm_exec.js` is **coupled to the Go toolchain that built the binary**:
the host bindings it installs must match what the runtime expects. It is
pinned in three places that must move together — `.config/mise/config.toml`
(`go = "1.26.4"`), the `wasm` job's `go-version` in
`.github/workflows/ci.yml`, and the table above. CI diffs the vendored
copy against its own toolchain's and fails loudly on drift, so a Go
upgrade cannot break the smoke test in a confusing way; re-vendor with

```sh
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/
```

and update the table. Never hand-edit the file.

## Measured

`go1.26.4`, Apple Silicon, Node 26:

|         |                                                         |
| ------- | ------------------------------------------------------- |
| Size    | 8.0 MB raw / 1.9 MB gzipped                             |
| Boot    | ~13 ms instantiate + ~27 ms `go.run`                    |
| `toADF` | 0.46 ms for a 150-character document, 3.6 ms for 1.2 KB |

Sub-millisecond conversion of a typical viewport chunk is inside a
live-preview budget, so a plugin can call in per update rather than
caching aggressively.
