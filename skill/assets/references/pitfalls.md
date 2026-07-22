# Pitfalls and practical constraints

## Local images need an asset store

Without an asset store wired in (`WithMediaAssets` +
`WithAssetIDResolver` + `WithImageDimsResolver`, or the `assets`
package's `MarkdownOptions`/`RenderOptions` bundles), a local image
reference like `![sketch](assets/sketch.png)` has no media id and is
**dropped from the ADF payload** with an `unresolved-asset` diagnostic.
Images with absolute `https://` URLs survive as external media. To push
local attachments: store them through `assets.FSStore`, upload via the
`Uploader` seam (`assets.Sync` / `assets.PushPipeline`), then encode.

## Attachment scope: one container per media id

Attachments have a product-side container boundary — a Jira media id is
bound to one issue, a Confluence one to one page. Bind the store view to
one container (`assets.ForScope(store, "KEY-123")`) so every document
encodes ids valid for _its_ container; local storage stays deduplicated
underneath.

## Diagnostics to watch

Conversions never return errors; lossy or recovered situations flow
through a diagnostics sink. One `WithDiagnostics` wires it into whichever
primitive emits (`FromMarkdown` parse notices, `ToADF` encode notices,
`FromADF` decode notices) — pass it to whichever primitives a composition
runs. Without a sink they are silently dropped. The `convert.Code*`
vocabulary:

- `colwidths-orphan` — a `::colwidths` with no following table; the
  directive is dropped.
- `decisions-orphan` — a `::decisions` with no plain bullet list on the
  following line; the directive is dropped.
- `parse-recovered` — the markdown parser panicked and the source was
  re-parsed in a normalized form.
- `malformed-frontmatter` — the document opened the frontmatter
  convention (a leading `---` fence) but no valid block formed; the
  opening bytes are kept as body rather than silently dropped.
- `span-marker-invalid` — a table span marker (`>`/`^`) whose merge
  cannot apply; kept as literal cell text.
- `unresolved-asset` — an `![alt](assets/…)` reference the asset store
  could not map to a media id.
- `unsupported-code-language` — a fenced code block whose language tag
  is not in the configured `WithCodeLanguages` set; the language still
  encodes verbatim.
- `unsupported-in-product` — the produced ADF uses a node or mark kind
  the target product does not render, per `WithUnsupportedKinds`
  (`jira.MarkdownOptions` wires `jira.UnsupportedKinds` =
  `placeholder`/`multiBodiedExtension`/`extensionFrame`; `confluence`'s =
  `blockTaskItem`). One per distinct kind; conversion output is unchanged
  (diagnostic-only) — the consumer decides severity.
- `fontsize-dropped` — a retired `:fontSize` construct was dropped to
  plain text: no Atlassian product supports the mark, so adfast never
  produces it. Emitted on encode (`:fontSize[text]{size}` unwraps to its
  text) and on decode (a legacy `fontSize` ADF mark becomes bare text).
  Text kept, size lost.
- `before-encode-failed` — a `BeforeEncode` hook error (e.g. a failed
  asset upload) downgraded to a diagnostic by the infallible
  `Pipeline.MarkdownToADF` (`Pipeline.MarkdownToADFAll` returns it).
- `raw-node` — an unknown ADF node reaching the markdown projection;
  projected through its first child or dropped.
- `decode-failed` — a value that could not be decoded into an ADF
  document at all; the conversion produces empty output.
- `unknown-node` / `unknown-mark` / `unknown-attr` — ADF content the
  typed model does not know; kept losslessly (RawNode/RawMark/Extra).
- `depth-exceeded` — input nested deeper than a recursion cap; deeper
  content truncated.

## Depth limits

Both the markdown parse and the ADF decode cap recursion at 1024 levels
of nesting; deeper content is truncated with a `depth-exceeded`
diagnostic instead of crashing.

## Wire safety: tightness-preserving output is NOT pushable

The prettier md → md formatter — the composition
`ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`
— is a pure md → ast → md pass and never builds an ADF document, so
formatter output is just markdown. On the conversion side,
`WithPreserveListTightness` writes the synthetic `tight` attribute onto
ADF list nodes — such documents **must never be submitted to the host
product** — check `adf.IsWireSafe` before submitting a document of
uncertain origin, and use `adf.StripSynthetic` to clean one up.
Canonical `ToADF(FromMarkdown(md))` output (without tightness
preservation) is always wire-safe.

For a lighter touch than the full formatter, `markdown.WrapProse(md,
width)` rewraps only contiguous prose paragraphs to a width, operating
line by line on the raw text; it leaves frontmatter, fenced code,
headings, and everything else it does not wrap byte-for-byte untouched.

## `:fontSize` is retired — do not author it

`:fontSize[text]{size}` still parses (so existing documents read cleanly),
but no Atlassian product supports the ADF `fontSize` mark: Jira's REST
endpoint rejects it (`INVALID_INPUT`) and Confluence strips it on save.
adfast therefore **retires** it — the directive never produces a mark. On
ADF encode it unwraps to its plain-text content; a legacy `fontSize` ADF
mark decodes to bare text; the prettier formatter rewrites the directive
to plain text. Every path emits a `fontsize-dropped` diagnostic. The text
survives; the size annotation is lost. Do not add new `:fontSize`
directives — use a preset text style in the product instead.

## Raw HTML has no ADF mapping

Canonical conversion drops block HTML silently and flattens inline tags
to literal text; only the style-preserving formatter carries HTML
through unchanged. Express structure with directives instead.

## Supported code languages

Code-block language tags encode verbatim, but each product highlights
only its own set. Configure the check with `WithCodeLanguages`:
`jira.CodeLanguages` (Jira Cloud's editor list) or
`confluence.CodeLanguages` (Confluence Cloud's code block macro list —
much smaller; no `go`, `json`, `kotlin`, `rust`, `typescript`, `yaml`).
Unknown languages render as plain, monospaced text in the product; the
`unsupported-code-language` diagnostic flags them at encode time.
