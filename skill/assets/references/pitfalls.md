# Pitfalls and practical constraints

## Local images need an asset store

Without an asset store wired in (`WithMediaAssets` or
`WithMediaAssetResolver`, plus `WithAssetIDResolver` and
`WithImageDimsResolver` — or the `assets` package's
`MarkdownOptions`/`RenderOptions` bundles), a local image
reference like `![sketch](assets/sketch.png)` has no media id and is
**dropped from the ADF payload** with an `unresolved-asset` diagnostic.
A **block** image with an absolute `https://` URL survives as external
media. An **inline** one (inside a paragraph, a table cell, or a list
item) does not: ADF's `mediaInline` addresses an uploaded attachment by
id and has no external variant, so it degrades to a link — alt text as
the label, image URL as the href — with an `inline-image-degraded`
diagnostic. To push local attachments: store them through
`assets.FSStore`, upload via the `Uploader` seam (`assets.Sync` /
`assets.PushPipeline`), then encode.

## A store lookup can have side effects

Handing a renderer every asset a store knows is not free. `assets.FSStore`
repairs the friendly file for the id it is asked about, next to the document
being rendered, so resolving an id materializes it. One index can serve every
document under a project root; rendering against the whole of it therefore
leaves a copy of every asset the repository ever downloaded beside whichever
document was rendered. `assets.RenderOptions` uses `WithMediaAssetResolver` for
exactly this reason — the store is asked about the media a document contains
and nothing else. Wire your own store the same way.

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
- `inline-image-degraded` — an inline `![alt](https://…)` rewritten as a
  link, because ADF has no inline image that can carry an external URL.
  The content stays visible and the round trip is stable; only the
  "render this inline" intent is lost. An inline image the asset store
  resolves to a media id is unaffected.
- `unsupported-code-language` — a fenced code block whose language tag
  is not in the configured `WithCodeLanguages` set; the language still
  encodes verbatim.
- `unsupported-in-product` — the produced ADF uses a node or mark kind
  the target product does not render, per `WithUnsupportedKinds`
  (`jira.MarkdownOptions` wires `jira.UnsupportedKinds` =
  `placeholder`/`multiBodiedExtension`/`extensionFrame`; `confluence`'s =
  `blockTaskItem`). One per distinct kind; conversion output is unchanged
  (diagnostic-only) — the consumer decides severity.
- `heading-anchor-dropped` — a heading's `{#id}` anchor was dropped
  because the target product has no anchor construct, per
  `WithoutHeadingAnchors` (`jira.MarkdownOptions` wires it; Jira has
  none). One per dropped anchor, naming the id. Unlike
  `unsupported-in-product` this is NOT diagnostic-only: the anchor is
  really gone, so the rendered page has no such link target. Heading text
  is unaffected.
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

## Wire safety: tightness, heading anchors and table alignment are NOT pushable

The prettier md → md formatter — the composition
`ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`
— is a pure md → ast → md pass and never builds an ADF document, so
formatter output is just markdown. On the conversion side,
`WithPreserveListTightness` writes the synthetic `tight` attribute onto
ADF list nodes, a `## Title {#id}` heading anchor writes the synthetic
`anchor` attribute onto heading nodes, and a table with alignment colons
in its delimiter row (`|:--|--:|`) writes the synthetic `align` attribute
onto table nodes — such documents **must never be submitted to the host
product** — check `adf.IsWireSafe` before submitting a document of
uncertain origin, and use `adf.StripSynthetic` to clean one up. Heading
anchors have a better answer than stripping: the product bundles resolve
them, `confluence.MarkdownOptions` by lowering the anchor to Confluence's
anchor macro and `jira.MarkdownOptions` by dropping it with a diagnostic.
Table alignment has no such answer — no product has a table alignment
attribute, so stripping is the only outcome. Encoding through either
bundle is wire-safe, as is canonical `ToADF(FromMarkdown(md))` output over
markdown that has no anchors and no table alignment, and without
tightness preservation.

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
