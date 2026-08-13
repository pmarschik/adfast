# adfast

adfast converts Markdown to and from
[ADF](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)
(Atlassian Document Format — the JSON document model behind Jira Cloud and
Confluence Cloud) at the **AST level**: round-trip-stable, remark-compatible
output, plus a typed ADF document model that losslessly preserves everything
it does not understand.

[![Go Reference](https://pkg.go.dev/badge/github.com/pmarschik/adfast.svg)](https://pkg.go.dev/github.com/pmarschik/adfast)
[![Go Report Card](https://goreportcard.com/badge/github.com/pmarschik/adfast)](https://goreportcard.com/report/github.com/pmarschik/adfast)
[![Go version](https://img.shields.io/github/go-mod/go-version/pmarschik/adfast)](go.mod)
[![CI](https://github.com/pmarschik/adfast/actions/workflows/ci.yml/badge.svg)](https://github.com/pmarschik/adfast/actions/workflows/ci.yml)

## Install

```sh
go get github.com/pmarschik/adfast
go get github.com/pmarschik/adfast/jira        # Jira link conventions
go get github.com/pmarschik/adfast/confluence  # Confluence page links + code macro languages
go get github.com/pmarschik/adfast/skill       # the dialect as an embeddable agent skill
go get github.com/pmarschik/adfast/frontmatter # optional YAML frontmatter parse/render/patch
```

Requires Go 1.25+. Note that [`jira/`](jira/), [`confluence/`](confluence/),
[`skill/`](skill/), and [`frontmatter/`](frontmatter/) are **separate Go
modules** (product-specific addons ship as submodules so consumers only pull
what they use).

## Quickstart

```go
md := `# Rooftop apiary — season plan

First inspection :date[2026-04-12] — feed status :status[Done]{color="green"}.`

node := adfast.FromMarkdown(md)          // parse to the pivot AST (ast.Node)
doc := adfast.ToADF(node)                // encode: typed ADF document (adf.Doc)
wire, _ := json.Marshal(doc)             // wire-format ADF JSON, ready for the REST API

back := adfast.FromADF(doc)              // decode ADF to the pivot AST
out := adfast.ToMarkdown(back)           // render back to markdown

// The prettier md → md formatter is a composition (same opts to both):
pretty := adfast.ToMarkdown(
    adfast.FromMarkdown(md, adfast.WithPrettierFormat()),
    adfast.WithPrettierFormat(),
)
```

md→adf is `ToADF(FromMarkdown(md))` and adf→md is
`ToMarkdown(FromADF(doc))` — the four primitives meet at the pivot AST
(`ast.Node`), and each reads the subset of the shared `adfast.Option` it
needs. The reverse edge into `FromADF` is `adf.DecodeDoc`: it turns any
JSON-decoded ADF value into the typed `adf.Doc`. Runnable examples live
in [`example_test.go`](example_test.go) and on
[pkg.go.dev](https://pkg.go.dev/github.com/pmarschik/adfast#pkg-examples).

## Key properties

- **Dialect**: CommonMark + GFM, plus
  [remark-directive](https://github.com/remarkjs/remark-directive)-style
  directives (via [goldmark-directive](https://github.com/pmarschik/goldmark-directive))
  for ADF features without native syntax — see
  [Supported Markdown](#supported-markdown).
- **remark-compatible rendering**: escaping, list marker alternation, prose
  wrapping, and character-reference encoding are measured against
  remark-stringify and byte-match it on the covered corpus.
- **Round-trip stable**: the md → adf → md round trip
  (`ToMarkdown(FromADF(ToADF(FromMarkdown(md))))`) is idempotent,
  enforced by a continuously grown fuzz corpus
  (`FuzzRoundTripIdempotent`).
- **Formatter**: the prettier md → md formatter is the composition
  `ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`
  — a pure md → ast → md pass (prettier-compatible output) that never
  routes through ADF, with semantic coherence against the ADF conversion
  enforced by tests.
- **Extensible**: custom node kinds plug into all four pipeline paths
  through one public contract — see [Extending adfast](#extending-adfast).

## How it works

Both conversion directions pivot through one source-independent tree — the
adfast AST (its semantics mirror remark's mdast):

```
markdown text ──goldmark──▶ goldmark AST      (source-anchored parse tree)
                               │  lift: decode escapes, resolve spans,
                               │        drop source anchoring
                               ▼
                             adfast AST      (shared pivot, remark semantics)
                               │▲
                               ▼│
                             ADF tree (Doc)  ──json──▶ serialized ADF
```

The pivot is built once from the goldmark parse (normalizing parser quirks
in the process), is what the remark-compatible renderer consumes, and the
ADF `Doc`/`Node` types are the ADF AST — JSON only happens at the very edge.

The facade is FOUR primitives with the pivot AST (`ast.Node`) as the
explicit currency — named by their non-AST end, `From*`/`To*` being
inverses at the AST boundary — plus one shared `adfast.Option` type each
primitive reads the subset of:

| Primitive                            | Shape                  | Role                       |
| ------------------------------------ | ---------------------- | -------------------------- |
| `adfast.FromMarkdown(md, ...Option)` | md → `ast.Node`        | parse (faithful pivot AST) |
| `adfast.ToADF(n, ...Option)`         | `ast.Node` → `adf.Doc` | encode                     |
| `adfast.FromADF(doc, ...Option)`     | `adf.Doc` → `ast.Node` | decode                     |
| `adfast.ToMarkdown(n, ...Option)`    | `ast.Node` → md        | render                     |

The common conversions are compositions (pass the same options to both
halves — each primitive ignores what it does not read):

| Conversion | Composition                                                                |
| ---------- | -------------------------------------------------------------------------- |
| md → adf   | `ToADF(FromMarkdown(md))`                                                  |
| adf → md   | `ToMarkdown(FromADF(doc))`                                                 |
| md → md    | `ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())` |

`FromMarkdown` parses to the faithful pivot AST and stops — no ADF, no
canonicalization — so it is the currency the `To*` primitives consume;
the subpackage `markdown.Parse`/`markdown.Render` and
`convert.ToADF`/`convert.FromADF` sit one layer down under the same
shapes. The `To*` primitives normalize on the way out: `ToADF` encodes
through the canonicalizing projection onto ADF's data model, and
`ToMarkdown`'s prettier-format mode (`WithPrettierFormat`) runs the
shared `convert.Normalize` pass before rendering.

The prettier md → md formatter is therefore the composition
`ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`
(add `WithPrintWidth(w)` to both calls for a custom width). `FromMarkdown`
is a single faithful parse in both directions: text values are fully
decoded (the ADF currency) while prettier's literal escapes ride
separately on `ast.Text.Raw` as escape provenance, so formatting re-emits
them byte-for-byte without polluting the semantic value. Escaping is thus
a render-only concern — `WithPrettierFormat` now has NO parse-side effect
at all: both directions share one `FrontmatterProvider`, so frontmatter
detection can never diverge and the flag is read only on the render call.
It is a pure
md → ast → md pass: it parses to the pivot AST, applies the
`convert.Normalize` canonicalization (unknown directives degrade,
`::colwidths`/`::decisions` resolve, inline marks canonicalize, media
re-derives its canonical payload), and renders back with prettier's text
rules. Nothing routes through ADF, so the formatter never loses
constructs ADF cannot model (frontmatter, raw HTML, inline images pass
straight through). `WithASTTransforms` is its content-rewrite seam.
Instead of a structural guarantee, two test obligations keep format and
conversion from drifting (see `format_contract_test.go`): semantic
coherence — formatting then parsing produces the same ADF as parsing the
original — and idempotence, both run over the fixture corpus and fuzzed
continuously (`FuzzFormatSemanticsPreserved`).

Canonical `ToADF(FromMarkdown(md))` output is wire-safe unless
`WithPreserveListTightness` is enabled — `adf.IsWireSafe` is the guard
to run before submitting a document of uncertain origin, and
`adf.StripSynthetic` the corresponding cleanup.

Media/attachment resolution is pluggable via `WithMediaAssets`,
`WithAssetIDResolver`, and `WithImageDimsResolver`. Where the collection
of downloaded files is large, or producing an entry costs something,
`WithMediaAssetResolver` answers the same question one media id at a
time: the conversion asks only about the media it actually meets.

ADF records no ordered-list marker style, so `FromADF` renders the
reference form — the start number repeated on every item, matching
remark-stringify with `incrementListMarker` off. Add
`WithIncrementListMarkers` where the Markdown is written and read by
people: items renumber `1. 2. 3.`, and a list a document already spelled
that way survives the round trip unchanged.

Automatic link handling makes **no assumptions about the host product**:
`WithSmartLinks(convert.SmartLinks{KeyFromURL, URLForKey})` teaches the
conversion a URL scheme once for both directions (links whose text equals
the derived key encode as inlineCards; bare `::linkCard[KEY]` labels
expand; cards render back to the short key), and `WithDocTransforms`
hooks document-level rewrites. The [`jira/`](jira/) submodule bundles the
Jira conventions — `jira.MarkdownOptions`/`jira.RenderOptions` each return
a `[]adfast.Option` slice; pass the encode-side bundle to both halves of
the md→adf composition and the decode-side bundle to both halves of the
adf→md one:

```go
mdOpts := jira.MarkdownOptions(baseURL, jira.ExpandAuto)
doc := adfast.ToADF(adfast.FromMarkdown(md, mdOpts...), mdOpts...)

rOpts := jira.RenderOptions()
out := adfast.ToMarkdown(adfast.FromADF(doc, rOpts...), rOpts...)
```

Bare-key expansion is selected with the typed `jira.ExpandMode` constants
(`ExpandAuto`, `ExpandAll`, `ExpandExplicit`). The submodule also ships
`jira.EncodeRichText` with the typed `jira.RichTextFormat` constants
(`RichTextADF`, `RichTextText`; `InferRichTextFormat` matches whatever an
existing field holds) and `jira.CodeLanguages`, the code-block language
set of Jira Cloud's editor for the `WithCodeLanguages` check.

The [`confluence/`](confluence/) submodule bundles the Confluence
conventions the same way: `confluence.MarkdownOptions(baseURL)` /
`confluence.RenderOptions()` wire smart links for Confluence Cloud page
URLs (`…/wiki/spaces/KEY/pages/123456789/Title` ⇄ the stable
`KEY/123456789` key — the mutable title slug is deliberately not part of
the key) plus `confluence.CodeLanguages`, the language set of Confluence
Cloud's code block macro (a much smaller set than Jira's editor list).

The [`skill/`](skill/) submodule ships the markdown dialect as an
**agent skill**: an embedded `SKILL.md` + `references/` bundle
(complete syntax, ADF coverage, a format-stable worked example, and
pitfalls) that teaches AI coding agents to read and write
adfast-flavored markdown. Hosts serve it via `skill.Files()` (an
`fs.FS`) or materialize it with `skill.Install(dir)` into their
agent-skills directory (e.g. `.claude/skills/`).

Leading document metadata is pluggable via `WithFrontmatterProvider`: the
default handles YAML `---` frontmatter; supply your own provider for other
conventions (e.g. the `<!-- Space: X -->` HTML-comment headers used by
Confluence sync tools). The same provider drives BOTH directions (md→adf
and the formatter), so detection can never diverge between them. A found
block never reaches the parser and is re-emitted verbatim by the
style-preserving formatter; a provider may also report a block as
_malformed_ (it opens the convention but does not close validly), in which
case the bytes are kept as body and a `malformed-frontmatter` diagnostic
fires.

The core stays YAML-neutral — the front block is opaque bytes, kept
verbatim on `ast.Frontmatter.Value` (delimiters included). Consumers who
want _structured_ access to a YAML block opt into the
[`frontmatter/`](frontmatter/) submodule, which turns the raw block ⇄ a
`map[string]any` without coupling the core to a YAML implementation:
`frontmatter.Parse`/`Render`, `frontmatter.Patch` (merge under a
caller-supplied top-level key order; nil values delete), `Replace`, the
nested dot-path helpers `Get`/`Set`/`Remove`, and `KeyOrder` (read the
authored order back out). It never re-implements boundary detection —
that stays with the `FrontmatterProvider`; `frontmatter.ParseNode`
bridges straight from an `ast.Frontmatter` node. For hand-authored blocks
where formatting matters, `frontmatter.PatchPreserving` edits only the
changed keys on the YAML CST, preserving the original key order, comments,
and scalar styles of everything it does not touch.

### Reusable pipelines

The composition halves are easy to drift apart — an extension registered
only on the parse call parses but never decodes back. A `Pipeline`
registers cross-cutting options once for BOTH directions and exposes the
composed one-shot conveniences, so every call reuses the same
configuration (immutable, safe for concurrent use):

```go
pipe := adfast.NewPipeline(adfast.WithPipelineOptions(
    adfast.WithExtensions(youtubenode.Registration()), // parse AND decode
    adfast.WithSmartLinks(jira.SmartLinks(baseURL)),   // encode AND render
    adfast.WithDiagnostics(sink),                      // every direction
    adfast.WithCodeLanguages(jira.CodeLanguages),      // encode-side check
))
doc := pipe.MarkdownToADF(md)        // ToADF(FromMarkdown(md)) under the config
out := pipe.ADFToMarkdown(doc)       // ToMarkdown(FromADF(doc)) under the config
pretty := pipe.Format(md)            // the prettier md → md formatter, same config
```

Because there is one shared option type, every cross-cutting option goes
through `WithPipelineOptions`; there are no direction-specific pipeline
constructors. `pipe.MarkdownToADFAll(mds)` is the batched variant — it
parses every document, runs the `WithBeforeEncode(hooks…)` hooks over the
whole set of parsed ASTs, then encodes each (so cross-document work such
as a single batched asset upload happens before anything encodes), and
`pipe.ADFBytesToMarkdown(v)` decodes raw ADF JSON (or any decoded value)
first. The free primitives stay as sugar for one-off calls.

## When not to use adfast

- **ADF → HTML rendering** — adfast targets markdown, not display HTML;
  use Atlassian's frontend tooling to render ADF for viewing.
- **Jira Data Center / Server** — those APIs speak wiki markup, not ADF;
  adfast covers Cloud ADF only.
- **Full API clients** — adfast converts documents, it does not talk to
  Atlassian APIs; pair it with a client library such as go-atlassian or
  go-jira for the transport.

## Error handling

The four primitives never return errors and never panic:
`FromMarkdown`, `FromADF`, `ToADF`, and `ToMarkdown` always produce a
result. Lossy or recovered situations — a dropped orphan `::colwidths`
or `::decisions` (`colwidths-orphan`, `decisions-orphan`), a table span
marker whose merge cannot apply (`span-marker-invalid`), a code-block
language outside a configured `WithCodeLanguages` set
(`unsupported-code-language`), a node or mark the target product does not
render (`unsupported-in-product`, see below), a recovered parser panic
(`parse-recovered`), an unknown ADF node reaching the markdown projection
(`raw-node`), a retired `:fontSize` dropped to plain text
(`fontsize-dropped`) — flow through a diagnostics sink instead. One
`WithDiagnostics(func(convert.Diagnostic))` wires the sink into whichever
primitive emits (parse notices on `FromMarkdown`, encode notices on
`ToADF`, decode notices on `FromADF`); pass it to whichever primitives a
composition runs. Without a sink, diagnostics are silently dropped.
`Pipeline.MarkdownToADFAll` is the errable batch variant: a
`BeforeEncode` hook failure (e.g. a batched asset upload) aborts the call
and returns the error.

### Product availability (`unsupported-in-product`)

The core conversion is universal — it round-trips a Confluence document
faithfully — so product availability is enforced as an authoring-side
**diagnostic**, not a conversion change. `WithUnsupportedKinds(product,
kinds)` declares the ADF node/mark kinds a target product does not
render; after `ToADF` produces the document it walks both nodes and
marks and emits one `unsupported-in-product` diagnostic per distinct
offending kind (e.g. `placeholder is not available in jira`). No node is
dropped or altered — the output is byte-identical with and without the
option; the **consumer decides severity** (e.g. treat it as a blocking
error before a Jira-targeted push).

The product sets are scoped to **render-confirmed non-support** — kinds
a full live probe (2026-07-22) showed dropped, shown as an
unsupported-content block, rejected by the product's ADF endpoint, or
stripped/downgraded on save — not documentation-by-omission, which
proved unreliable (Jira's docs are non-exhaustive, its REST accepts most
of the shared schema, and it renders most omitted kinds first-class incl.
layoutSection, cards, status, the extension family, syncBlock, and the
alignment/indentation/breakout/annotation/fragment/dataConsumer marks).
So `jira.UnsupportedKinds` is `placeholder` (dropped by the render) plus
`multiBodiedExtension` and `extensionFrame` (rejected by the Jira REST
endpoint with INVALID_INPUT), and `confluence.UnsupportedKinds` is
`blockTaskItem` (downgraded to a plain taskItem); both are wired via
`jira`/`confluence.MarkdownOptions`. `fontSize` is in neither set even
though both products reject it: adfast **retires** the mark (it never
produces one — see the `fontSize` note below), so an
`unsupported-in-product` check for it would be moot.
Adding a kind requires a live-probe confirmation, not a missing docs
page. The evidence and the full availability data live in
`docs/adf-coverage.md` + `docs/adf-availability.json`.

## Supported Markdown

The base dialect is **CommonMark + GFM**: pipe tables (padded to column
width, plus [remark-extended-table](https://github.com/wataru-chocola/remark-extended-table)
cell merging — a cell containing only `>` merges into the cell to its right,
a cell containing only `^` extends the cell above; literal `>`/`^` cell
content is escaped), task lists (`- [ ]` / `- [x]`), strikethrough, and
autolink literals. On top of that: decision lists (a `::decisions` leaf
directive marks the immediately following plain bullet list, exactly like
`::colwidths` marks the following table), YAML frontmatter (pluggable via
`WithFrontmatterProvider`), and the directive dialect below —
`:name[label]{attrs}` inline, `::name[label]{attrs}` as a block leaf, and
`:::name … :::` as a container. Everything below round-trips losslessly
through ADF.

### Container directives (block elements)

| Markdown                               | ADF                          | Notes                                                                                                                                                                  |
| -------------------------------------- | ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `:::info` … `:::`                      | panel                        | Also `note`, `warning`, `success`, `error`                                                                                                                             |
| `:::expand[Title]` … `:::`             | expand                       | Title is optional; nests inside panels as nestedExpand                                                                                                                 |
| `:::media[alt]{…}` … `:::`             | mediaSingle + caption        | The `::media` attrs on the fence line, the caption as the body; a plain-text caption on image-expressible media uses the image title instead: `![alt](path "caption")` |
| `:::extension{…}` … `:::`              | bodiedExtension              | Same attrs as `::extension`; when every child is a `:::frame` container (extensionFrame) it encodes as multiBodiedExtension (a frameless one carries the bare `multi`) |
| `:::syncBlock{resourceId localId}` …   | bodiedSyncBlock              | The source body of a synced block                                                                                                                                      |
| `:::section` + `:::column{width="…"}`  | layoutSection / layoutColumn | Page layouts; `columnRuleStyle`/`localId` on the section, `width`/`valign`/`localId` on each column                                                                    |
| `:::center` / `:::end` … `:::`         | alignment mark               | Block mark on each wrapped paragraph/heading                                                                                                                           |
| `:::indent{2}` … `:::`                 | indentation mark             | The bare value is the level (1–6)                                                                                                                                      |
| `:::breakout{wide}` … `:::`            | breakout mark                | Modes `wide`/`full-width`; optional `width="1200"`                                                                                                                     |
| `:::dataConsumer{sources="id1,id2"}` … | dataConsumer mark            | `sources` is a comma-separated list of source ids (opaque strings; parsed by splitting on commas and trimming)                                                         |
| `:::fragment{localId="…" name?}` …     | fragment mark                | Stable references to tables/extensions                                                                                                                                 |

Nested containers grow the outer fence (`::::`), like remark. The
mark-wrapper containers (`:::center`/`:::end`, `:::indent`,
`:::breakout`, `:::dataConsumer`, `:::fragment`) put the ADF **block
mark** on every block they wrap; wrappers compose by nesting, and the
ADF mark array maps inside-out onto the nesting (first mark innermost),
so round trips preserve mark order. Single-valued directives take the
**bare-value attribute form** (`{2}`, `{wide}`, `{small}`): exactly one
attribute with an empty value is the directive's value (a named
`level=`/`mode=`/`size=` attribute wins when both are present).
Arbitrary-JSON payloads (`parameters` on extensions) use a **canonical
JSON attr encoding**: `json.Marshal` output (sorted keys, no
insignificant whitespace). Because that JSON contains `"`, the attribute
is **single-quoted** so it stays readable and lossless —
`parameters='{"station":"rooftop"}'`. When the JSON value itself contains
a `'`, single-quoting would not be lossless, so the attribute falls back
to double quotes with every `"` written as `&quot;` (remark-compatible,
since remark decodes character references in attribute values). The
`dataConsumer` `sources` attribute is a plain comma-separated list of
source ids, not JSON.

### Leaf directives (standalone lines)

| Markdown                                                                                                 | ADF                        | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| -------------------------------------------------------------------------------------------------------- | -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `::linkCard[ABC-123]`                                                                                    | blockCard                  | Bare keys expand via the configured `SmartLinks` resolver; full URLs also work: `::linkCard[https://…]`                                                                                                                                                                                                                                                                                                                                |
| `::linkEmbed[https://…]{layout="center" width="80"}`                                                     | embedCard                  | `layout`/`width` mirror the embed attributes                                                                                                                                                                                                                                                                                                                                                                                           |
| `::media[shot.png]{#<media-uuid> collection height="551" layout="align-start" type="file" width="2308"}` | mediaSingle / media        | Attachments; the label is the alt text (all attrs optional). `type` is `file`\|`external` (default `file`); `#<id>` is the media id; `url` links `type="external"` media; `collection`/`occurrenceKey` are opaque strings kept when present; `width`/`height` are intrinsic pixel dimensions, `layoutWidth`/`widthType` carry display sizing; `group="true"` items reassemble a mediaGroup; `path` points at the downloaded local file |
| `::colwidths[79,320,200]`                                                                                | table column widths        | Placed directly before a table; widths re-apply to every row on encode. A `::colwidths` with no following table is dropped with a `colwidths-orphan` diagnostic. Counts visual columns — a colspan cell carries one width per covered column                                                                                                                                                                                           |
| `::jql[project = X AND status = Open]{cloudId="…" datasource="…" columns="summary,status"}`              | blockCard (JQL datasource) | Live JQL tables (Jira); `columns` lists the table-view keys, `url` is kept when present                                                                                                                                                                                                                                                                                                                                                |
| `::extension{key="…" type="…" parameters='…' layout? localId? text?}`                                    | extension                  | Bodiless macros; `key`/`type` are the ADF extensionKey/extensionType; `parameters` carries arbitrary JSON in the canonical attr encoding                                                                                                                                                                                                                                                                                               |
| `::syncBlock{localId="…" resourceId="…"}`                                                                | syncBlock                  | A reference to a synced block                                                                                                                                                                                                                                                                                                                                                                                                          |

Media directives additionally carry `borderColor`/`borderSize`
attributes for the ADF border mark on the media node.

### Text directives (inline elements)

| Markdown                                                | ADF                  | Notes                                                                                                                                                    |
| ------------------------------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `:mention[Jane Doe]{#712020:aa…}`                       | mention              | `#…` is the account id; `accessLevel` kept when present; a legacy leading `@` in the label is accepted (stripped)                                        |
| `:status[In Progress]{color="blue"}`                    | status               | Lozenge; `style` kept when present                                                                                                                       |
| `:date[2026-07-15]{timestamp="1784073600000"}`          | date                 | `timestamp` (ms since epoch) is authoritative; the label is the UTC day derived from it (and parses without one); `localId` kept when present            |
| `:placeholder[Type something…]`                         | placeholder          | Template placeholder text; `localId` kept when present                                                                                                   |
| `:emoji{#custom-id shortName=":team_logo:"}`            | emoji                | Fallback for custom/site emojis only — see the emoji row in the coverage table. `shortName` required; `#<id>` and `text` (rendered fallback) optional    |
| `:extension{key="…" type="…" …}`                        | inlineExtension      | Inline macros; same attrs as `::extension` minus `layout`                                                                                                |
| `:annotation[text]{#id annotationType="inlineComment"}` | annotation mark      | Confluence inline-comment anchor — pushing a body without it orphans the comment thread, so the mark must survive                                        |
| `:color[text]{color="#ff5630"}`                         | textColor mark       |                                                                                                                                                          |
| `:bg[text]{color="#fffae6"}`                            | backgroundColor mark |                                                                                                                                                          |
| `:u[text]`                                              | underline mark       |                                                                                                                                                          |
| `:sub[text]` / `:sup[text]`                             | subsup mark          |                                                                                                                                                          |
| `:fontSize[text]{small}`                                | _retired_            | Parses (the bare value is the size; `size="…"` also parses) but is dropped to plain text — no product supports the mark. See the coverage table          |
| `:media{#<media-uuid> collection}`                      | mediaInline          | Inline attachment chip. `type` defaults to `file` and is left out when canonical; a bare `collection` is an empty collection, and its absence means none |

Mark directives nest with regular emphasis:
`:color[**bold red**]{color="#ff5630"}`. Inline mark directives wrap
per text run in fixed nesting order (outside → inside): `:annotation`,
`:color`, `:bg`, `:u`, `:sub`/`:sup`. (`:fontSize` is retired — it parses
but drops to plain text.) Directive labels cannot nest brackets, so
overlapping annotation marks on one text run degrade to the outermost
anchor.

Every known directive parses into a **typed AST node** (package
[`dialect/`](dialect/)) implementing the public extension contract; unknown
directive names keep the generic directive kinds and degrade exactly like
remark (containers dissolve into their content, unknown leaves drop,
unknown text directives flatten to text).

### Related conventions (no directive needed)

- **Attachments as images** — with a media-asset store wired in
  (`WithMediaAssets` or `WithMediaAssetResolver`, plus `WithAssetIDResolver`
  and `WithImageDimsResolver`),
  file media whose local copy carries every ADF property renders as a plain
  `![alt](assets/shot.png)` and maps back to its media id on encode.
  Anything richer (PDFs, resized media, non-default layouts) keeps the
  `::media` directive.
- **Issue links** — a link whose text equals the resolver-derived key
  (e.g. `[ABC-123](https://…/browse/ABC-123)`) becomes an inlineCard.
- **Image titles as captions** — `![alt](path "caption")` maps the title
  to a mediaSingle caption child; richer captions (formatting, hard
  breaks) use the `:::media` container form.
- **Tables** — GFM pipe tables; a header row is synthesized when the ADF
  table has none.

## A complete example

A single document exercising most of the dialect — usable as a template.
It round-trips through `FromMarkdown` → `ToMarkdown` unchanged (a test
extracts this block and asserts it):

<!-- tutorial:begin -->

````markdown
---
title: Rooftop apiary — season plan
labels: [bees, community-garden]
---

# Rooftop apiary — season plan

Inline directives annotate prose without leaving the line. This plan is kept by :mention[Maya Winters]{#712020:aa11}, a mention that links to a person; its :status[In Progress]{color="blue"} shows a colored status lozenge; and the first inspection :date[2026-04-12]{timestamp="1775952000000"} renders as a real date chip 🐝

:::info
An `info` panel frames helpful context in a colored callout. Everything inside is ordinary markdown that survives the round trip to ADF and back, including the :annotation[inline comments]{#c9e1 annotationType="inlineComment"} your co-keepers leave — an annotation anchors a Confluence comment thread to a span of text, so the thread stays attached across edits.
:::

## Season scope

Text marks add formatting inline: **bold** and _italic_ for emphasis, ~~three hives~~ struck through for a retraction, and `varroa` in code for a literal term. A ratio reads :sub[1] as subscript to :sup[1] as superscript; :color[red]{color="#ff5630"} sets the text color and :bg[highlight]{color="#fff0b3"} the background; and :u[underline] underlines a run.

A trailing backslash forces a hard line break:\
so this clause starts on its own line. New keepers sign the rota where a :placeholder[your name here…] marks an empty template field to fill in later.

A task list tracks work with checkboxes — `[ ]` is open and `[x]` is done:

- [ ] assemble the new brood boxes
- [x] order spring sugar syrup
- [ ] paint the new stands

  A loose item keeps indented follow-up blocks: use the leftover green from the shed door.

The `::decisions` marker turns the bullet list that immediately follows it into a decision list, so each item reads as a recorded decision:

::decisions

- we requeen Hive B this season, Hive A next year
- no honey harvest before the summer solstice

An ordered list numbers its steps in sequence, and a thematic break closes the section:

1. Clean and scorch the empty boxes
2. Split the strongest colony
3. Merge the nucleus before winter

---

## Hive setup

:::expand[Why a vertical hive stand?]
An `:::expand` is a collapsible section — readers click the title to reveal the body, which keeps long asides out of the way. Links work inside it: see [BEE-42](https://hive.example.org/browse/BEE-42) and the club wiki at https://wiki.example.org/apiary.
:::

::::warning
A `warning` panel flags something to be careful about: mind the parapet ledge when hauling supers. Panels nest, so a collapsible section fits inside one:

:::expand[Storage map]
Frames live in the attic crates; smoker fuel stays in the metal locker. A :emoji{#1f9a9-custom shortName=":county_bee:"} falls back to a custom emoji, resolved by its short name when it is not a standard unicode glyph.
:::
::::

A fenced code block keeps source verbatim, tagged with its language for highlighting:

```python
if colony.strength() > SPLIT_THRESHOLD:
    apiary.split(colony)
```

## Inspection rota

`::colwidths` pins each column's pixel width for the table that follows; the table itself supports spans, where `>` merges a cell leftward (colspan) and `^` merges it upward (rowspan):

::colwidths[120,80,220]

| Keeper | Week | Notes                       |
| ------ | ---- | --------------------------- |
| Maya   | 15   | queen spotting              |
| >      | Sam  | mite count                  |
| Priya  | ^    | shares the mite-count sheet |

## Task board

A `::jql` block embeds a live Jira query as a datasource table, naming the columns to display:

::jql[project = BEE AND fixVersion = season-2026 ORDER BY rank]{cloudId="abc-123" columns="summary,status,assignee" datasource="d8b52e33-6a5d-4c6e-8f6a-1b2c3d4e5f60"}

A `::linkCard` renders a URL as a rich preview card:

::linkCard[https://hive.example.org/browse/BEE-42]

A `::linkEmbed` embeds the target inline, sized by its layout and width attributes:

::linkEmbed[https://wiki.example.org/apiary/map]{layout="center" width="80"}

Extensions host third-party macros. An inline one drops a widget mid-sentence — hive scale: :extension{key="scale" type="com.example"} — where `key` and `type` name the macro and `parameters` carries its JSON config:

::extension{key="weather-widget" parameters='{"station":"rooftop"}' type="com.example.apiary"}

A bodied extension wraps block content that the macro renders:

:::extension{key="inspection-log" type="com.example.apiary"}
Entries in this body render inside the inspection-log macro.
:::

A multi-bodied extension gives the macro several `:::frame` bodies — here one tab per season:

::::extension{key="season-tabs" type="com.example.apiary"}
:::frame
Spring: feed, inspect, split.
:::
:::frame
Summer: supers on, harvest after solstice.
:::
::::

## Shared checklists

A `:::syncBlock` defines reusable block content that other pages embed by its `resourceId`:

:::syncBlock{localId="safety-1" resourceId="ari:cloud:example:page/123"}
Zip the suit before opening any hive.
:::

The leaf `::syncBlock` embeds that shared content here by reference:

::syncBlock{localId="safety-1" resourceId="ari:cloud:example:page/123"}

## Layout

A `:::section` lays `:::column` blocks out side by side, each sized by `width`, and `:::center` centers a column's content:

:::section
:::column{width="50"}
:::center
**Before**: two weathered hives
:::
:::
:::column{width="50"}
:::center
**After**: painted stands, three colonies
:::
:::
:::

A `:::indent` shifts a block to the right by a level:

:::indent{2}
The nucleus stands two paces in from the parapet edge.
:::

A `:::end` aligns its content to the end of the column:

:::end
Wind readings align to the end of the content column.
:::

A `:::dataConsumer` marks a block as reading from named sources — here the task-board datasource above, referenced by its id:

:::dataConsumer{sources="d8b52e33-6a5d-4c6e-8f6a-1b2c3d4e5f60"}
This summary re-reads the task-board datasource above.
:::

A `:::fragment` gives a block a stable id and name so other macros can reference it:

:::fragment{localId="rota-fragment" name="Inspection rota"}
Other macros reference this block by its fragment name.
:::

## Attachments

An image with a title becomes a captioned figure:

![Mite counts by week](https://static.example.org/mite-counts.png "Varroa counts, spring 2026")

A leaf `::media` attaches a file by its media id:

::media[hive-inspection-sheet.pdf]{#b5773183-5f9a-481f-b1b8-8fe286bba8e9}

A bodied `:::media` adds a caption beneath the attachment:

:::media[hive stand sketch]{#0f4b9a2c-3d5e-4f60-8a71-92b3c4d5e6f7 height="480" layout="center" width="640"}
Sketch of the **vertical** stand — drawn by Sam.
:::

An inline :media{#7c1e0d2a-4b3f-45e8-9a2b-6c5d4e3f2a1b collection} drops an attachment mid-sentence — here the field kit.

Finally, a `:::breakout` lets a block escape the content column's width:

:::breakout{wide}

> This wide quote breaks out of the content column.

:::
````

<!-- tutorial:end -->

## ADF coverage

Every node and mark type in Atlassian's ADF schema
([`@atlaskit/adf-schema`](https://unpkg.com/browse/@atlaskit/adf-schema/dist/json-schema/v1/)
full + stage-0, cross-checked against the
[ADF reference](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)),
whether it can occur in that product's documents, and how adfast treats
it.

> **Fully cited matrix:** every row below — with each kind's upstream
> schema-definition link (pinned to a mirror commit SHA) and the exact
> Jira/Confluence evidence behind its marker — lives in
> [docs/adf-coverage.md](docs/adf-coverage.md). Machine-readable form:
> [docs/adf-availability.json](docs/adf-availability.json). Both columns
> were confirmed empirically (2026-07-22) against a live Cloud site: every
> node and mark was written to a Jira issue and a Confluence page and the
> product-rendered DOM inspected, and the Confluence page was additionally
> read back to see what survives save. See the "Empirical validation"
> section of the cited matrix.

**Per-product marker** (whether the kind can occur in that product's
documents; as of 2026-07-22 the markers reflect live render/round-trip
evidence, not just documentation):

- **✓** — available: the product renders it first-class, or renders it
  degraded-but-present (Jira), or preserves it on save (Confluence);
- **∘** — present in the shared ADF schema but genuinely untestable here
  (e.g. attachment-gated file media);
- **—** — not available: dropped by the render, rejected by the product's
  ADF endpoint, or stripped/downgraded on save.

**adfast support** (adfast's own handling, independent of product
availability):

- **converted** — has a markdown mapping and round-trips through it;
- **preserved** — survives ADF decode → encode losslessly (typed or as
  `RawNode`/`RawMark`) but is dropped or reduced by the markdown
  projection, with a `raw-node` diagnostic.
- **dropped** — deliberately retired: adfast never produces the kind, and
  a legacy instance decodes to plain text (with a `fontsize-dropped`
  diagnostic) — text preserved, styling lost. `fontSize` is the only such
  kind (no Atlassian product supports the mark).

The product-availability diagnostic uses the render-confirmed
not-available set (see [Product availability](#product-availability-unsupported-in-product)).
`jira.UnsupportedKinds` is `placeholder` (dropped by the render) plus
`multiBodiedExtension` and `extensionFrame` (rejected by the Jira REST
endpoint with INVALID_INPUT) — Jira renders every other kind probed.
`confluence.UnsupportedKinds` is `blockTaskItem` (downgraded to a plain
taskItem). `fontSize` is in neither set: both products reject it, but
adfast retires the mark (never produced), so the check would be moot.

| ADF node                                      | Jira | Confluence | adfast support | Markdown mapping / notes                                                                                                                                                                   |
| --------------------------------------------- | ---- | ---------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| doc                                           | ✓    | ✓          | converted      | document root                                                                                                                                                                              |
| paragraph                                     | ✓    | ✓          | converted      | paragraph                                                                                                                                                                                  |
| text                                          | ✓    | ✓          | converted      | plain text carrying the marks below                                                                                                                                                        |
| heading                                       | ✓    | ✓          | converted      | `#`–`######`                                                                                                                                                                               |
| blockquote                                    | ✓    | ✓          | converted      | `>`                                                                                                                                                                                        |
| rule                                          | ✓    | ✓          | converted      | `---`                                                                                                                                                                                      |
| codeBlock                                     | ✓    | ✓          | converted      | fenced code block; fence grows past embedded backtick runs; language survives                                                                                                              |
| bulletList / orderedList / listItem           | ✓    | ✓          | converted      | `-` / `1.` lists; marker alternation between adjacent lists; `order` start preserved                                                                                                       |
| taskList / taskItem                           | ✓    | ✓          | converted      | `- [ ]` / `- [x]`; `localId` regenerates as empty on encode                                                                                                                                |
| blockTaskItem                                 | ✓    | —          | converted      | `- [ ]` + indented blocks; a single-paragraph item re-encodes as the inline taskItem. Jira renders it first-class; Confluence downgrades it to a plain taskItem                            |
| decisionList / decisionItem                   | ✓    | ✓          | converted      | `::decisions` + following plain bullet list; encodes with state DECIDED; Jira renders decisions first-class (live 2026-07-22)                                                              |
| table / tableRow / tableHeader / tableCell    | ✓    | ✓          | converted      | GFM pipe table; colspan/rowspan via `>`/`^` markers; colwidth attrs via `::colwidths`                                                                                                      |
| panel                                         | ✓    | ✓          | converted      | `:::info` …; unknown panelType degrades to `info`                                                                                                                                          |
| expand / nestedExpand                         | ✓    | ✓          | converted      | `:::expand[Title]` …; encode always emits `expand` (Jira nests it as nestedExpand itself)                                                                                                  |
| mediaSingle / mediaGroup / media              | ✓    | ✓          | converted      | `![alt](path)` or `::media`; plain image only when fully expressible; groups fan out to `group="true"` items                                                                               |
| mediaInline                                   | ∘    | ✓          | converted      | `:media{…}` inline attachment chip. Jira is attachment-gated — not injection-testable with synthetic ids, so left inconclusive                                                             |
| caption                                       | ✓    | ✓          | converted      | image title (`![alt](path "caption")`) when plain text on image-expressible media, else the `:::media` body                                                                                |
| inlineCard                                    | ✓    | ✓          | converted      | `[KEY](url)` link; encodes back to inlineCard when the label equals the resolver-derived key                                                                                               |
| blockCard                                     | ✓    | ✓          | converted      | `::linkCard[…]`; URL-less cards are dropped                                                                                                                                                |
| blockCard + datasource                        | ✓    | ∘          | converted      | `::jql[…]{…}` — only the documented jira/jql shape; richer shapes fall back to `::linkCard`                                                                                                |
| embedCard                                     | ✓    | ✓          | converted      | `::linkEmbed[…]{…}`                                                                                                                                                                        |
| mention                                       | ✓    | ✓          | converted      | `:mention[Name]{#id}`                                                                                                                                                                      |
| emoji                                         | ✓    | ✓          | converted      | with a `text` attr: that text (deliberately lossy — shortName/id degrade to plain text across markdown); without: unicode via the emoji-toolkit shortname table, else `:emoji{shortName…}` |
| status                                        | ✓    | ✓          | converted      | `:status[Text]{color}`                                                                                                                                                                     |
| date                                          | ✓    | ✓          | converted      | `:date[2026-07-15]{timestamp="…"}`; the timestamp attribute is authoritative                                                                                                               |
| hardBreak                                     | ✓    | ✓          | converted      | backslash / trailing-space break                                                                                                                                                           |
| placeholder                                   | —    | ✓          | converted      | `:placeholder[Type something…]`                                                                                                                                                            |
| layoutSection / layoutColumn                  | ✓    | ✓          | converted      | `:::section` containing `:::column{width="…"}` containers. Jira renders a real multi-column layout (live 2026-07-22)                                                                       |
| extension / bodiedExtension / inlineExtension | ✓    | ✓          | converted      | `::extension{…}` / `:::extension{…}` + body / `:extension{…}`. Jira renders them (ak-renderer-extension / inline fallback); Confluence resolves known macros                               |
| multiBodiedExtension / extensionFrame         | —    | ✓          | converted      | `:::extension{…}` whose children are all `:::frame` containers; stage-0 schema. Jira REST rejects them (INVALID_INPUT); Confluence preserves them                                          |
| syncBlock / bodiedSyncBlock                   | ✓    | ✓          | converted      | `::syncBlock{…}` (reference) / `:::syncBlock{…}` + body (source). Jira renders the sync-block widget (live 2026-07-22)                                                                     |

| ADF mark        | Jira | Confluence | adfast support | Markdown mapping / notes                                                                                                                                                                                                                                                                                              |
| --------------- | ---- | ---------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| strong          | ✓    | ✓          | converted      | `**bold**`                                                                                                                                                                                                                                                                                                            |
| em              | ✓    | ✓          | converted      | `_italic_`                                                                                                                                                                                                                                                                                                            |
| strike          | ✓    | ✓          | converted      | `~~strike~~`                                                                                                                                                                                                                                                                                                          |
| code            | ✓    | ✓          | converted      | `` `code` ``; exclusive like ADF (strong/em/strike stripped)                                                                                                                                                                                                                                                          |
| underline       | ✓    | ✓          | converted      | `:u[text]`                                                                                                                                                                                                                                                                                                            |
| link            | ✓    | ✓          | converted      | `[label](url)` incl. titles                                                                                                                                                                                                                                                                                           |
| subsup          | ✓    | ✓          | converted      | `:sub[text]` / `:sup[text]`                                                                                                                                                                                                                                                                                           |
| textColor       | ✓    | ✓          | converted      | `:color[text]{color="#ff5630"}`                                                                                                                                                                                                                                                                                       |
| backgroundColor | ✓    | ✓          | converted      | `:bg[text]{color="#fffae6"}`                                                                                                                                                                                                                                                                                          |
| border          | ✓    | ✓          | converted      | `borderColor`/`borderSize` attributes on the media directive forms                                                                                                                                                                                                                                                    |
| alignment       | ✓    | ✓          | converted      | `:::center` / `:::end` wrapper around the block. Jira renders it first-class (fabric-editor-alignment, live 2026-07-22)                                                                                                                                                                                               |
| indentation     | ✓    | ✓          | converted      | `:::indent{level}` wrapper around the block. Jira renders it first-class (fabric-editor-indentation, live 2026-07-22)                                                                                                                                                                                                 |
| breakout        | ✓    | ✓          | converted      | `:::breakout{mode}` wrapper around the block. Jira renders it first-class (live 2026-07-22)                                                                                                                                                                                                                           |
| annotation      | ✓    | ✓          | converted      | `:annotation[text]{#id annotationType}` — keeps Confluence inline-comment threads anchored across markdown edits; overlapping anchors on one text run degrade to the outermost. Jira renders it (live 2026-07-22)                                                                                                     |
| dataConsumer    | ✓    | ✓          | converted      | `:::dataConsumer{sources="id1,id2"}` wrapper around the block (`sources` is a comma-separated id list). Both products preserve/render the mark (live 2026-07-22)                                                                                                                                                      |
| fragment        | ✓    | ✓          | converted      | `:::fragment{localId name?}` wrapper around the block. Both products preserve/render the mark (live 2026-07-22)                                                                                                                                                                                                       |
| fontSize        | —    | —          | dropped        | **Retired** — no product supports the mark (Jira REST rejects it with INVALID_INPUT; Confluence strips it on save). `:fontSize[text]{size}` still parses but unwraps to plain text on encode, and a legacy `fontSize` ADF mark decodes to bare text; both emit a `fontsize-dropped` diagnostic (text kept, size lost) |

In short: unknown or undocumented ADF content **survives ADF-level
round trips losslessly** and can be reported through diagnostics; only
the markdown projection reduces it. Every kind in the table has a
markdown mapping, so documents also survive markdown-only persistence
(render → store the file → re-parse → push). The mechanics — RawNode/
Extra preservation, diagnostic codes, and the few deliberate edge-case
losses — are documented in [docs/design.md](docs/design.md).

## Extending adfast

The [`extension/`](extension/) package defines the public contract for
custom node kinds. A kind must support **all four pipeline paths** —
capability fragments (render-only or encode-only) are rejected at
registration:

1. **md → ast**: a parse constructor promotes a generic directive node
   (`Registration.Containers`/`Leaves`/`Texts`, keyed by directive name).
2. **ast → md**: the node's `RenderMarkdown` writes its directive form
   through `extension.RenderContext`.
3. **ast → adf**: the node's `EncodeADF` returns its ADF form through
   `extension.EncodeContext`.
4. **adf → ast**: a decode hook recognizes the ADF shape the kind owns
   (`Registration.DecodeBlock`/`DecodeBlockList`/`DecodeInline`).

The known dialect (package [`dialect/`](dialect/)) is implemented on
exactly this contract, so it is both the default registration set and the
reference implementation. A complete custom kind — a fictional
`:youtube[dQw4w9WgXcQ]` inline directive for a (made-up) `youtube` ADF
node:

```go
package youtubenode

import (
    "github.com/pmarschik/adfast/adf"
    "github.com/pmarschik/adfast/ast"
    "github.com/pmarschik/adfast/extension"
)

// YouTube is :youtube[videoId] ⇄ a fictional "youtube" ADF node.
type YouTube struct {
    Children []ast.Node // the video-id label
}

func (*YouTube) Kind() string                    { return "youtube" }
func (n *YouTube) ChildNodes() []ast.Node        { return n.Children } // ast.Parent
func (n *YouTube) SetChildNodes(kids []ast.Node) { n.Children = kids }
func (*YouTube) MarkdownLead() byte              { return ':' } // extension.InlineLead

// ast → md
func (n *YouTube) RenderMarkdown(ctx extension.RenderContext) {
    ctx.WriteTextDirective("youtube", nil, n.Children)
}

// ast → adf
func (n *YouTube) EncodeADF(_ extension.EncodeContext) []adf.Node {
    id := ast.PlainText(n.Children)
    if id == "" {
        return nil // drop, like remark degradation
    }
    // Unknown-to-adfast kinds are built as RawNode (the decoder would
    // produce the same shape for them).
    return []adf.Node{&adf.RawNode{Type: "youtube", Attrs: map[string]any{"videoId": id}}}
}

// Registration bundles the remaining two paths: md → ast and adf → ast.
func Registration() extension.Registration {
    return extension.Registration{
        Kind: "youtube",
        Texts: map[string]func(*ast.TextDirective) extension.Node{
            "youtube": func(d *ast.TextDirective) extension.Node {
                return &YouTube{Children: d.Children}
            },
        },
        DecodeInline: func(n adf.Node, _ extension.DecodeContext) ([]ast.Node, bool) {
            raw, ok := n.(*adf.RawNode)
            if !ok || raw.Type != "youtube" {
                return nil, false
            }
            id := adf.StrAttr(raw.Attrs, "videoId")
            if id == "" {
                return nil, true // owned, but not representable: drop
            }
            return []ast.Node{&YouTube{Children: []ast.Node{&ast.Text{Value: id}}}}, true
        },
    }
}
```

Wire it in — one `adfast.WithExtensions` covers every direction (the
facade forwards to `markdown.WithExtensions` on parse and
`convert.WithExtensions` on encode/decode); the default dialect set stays
active. Register the same bundle on both halves of a composition so the
parse and decode legs never drift apart:

```go
reg := adfast.WithExtensions(youtubenode.Registration())
doc := adfast.ToADF(adfast.FromMarkdown(md, reg), reg)
out := adfast.ToMarkdown(adfast.FromADF(doc, reg), reg)
```

Block kinds additionally embed `ast.BlockSpacing` (blank-line structure)
and implement `extension.ContainerForm` when they render a `:::` container
(so enclosing fences grow around them). A few dialect behaviors deliberately
stay structural in `convert` because they cross node boundaries: the
`::colwidths` ↔ table attachment, the `::decisions` ↔ bullet-list
marking, and the inline mark directives' decode (ADF stores them as
text marks, not nodes) — see the `dialect` package documentation.

## Layout

The module is layered into public subpackages along the pipeline stages;
the root package is a thin facade composing them. Full package notes live
in [docs/design.md](docs/design.md).

| Package                        | Purpose                                                                                                              |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------- |
| `adfast` (root)                | The facade: the four `FromMarkdown`/`FromADF`/`ToADF`/`ToMarkdown` primitives, `Pipeline`, the shared option set     |
| [`adf/`](adf/)                 | Typed ADF document model + JSON codec; lossless `RawNode`/`RawMark`/`Extra` preservation                             |
| [`ast/`](ast/)                 | The pivot Markdown AST (remark-mdast-shaped) both directions share                                                   |
| [`extension/`](extension/)     | Public AST extension contract (`Node`, context interfaces, `Registration`)                                           |
| [`dialect/`](dialect/)         | The known directive dialect as typed AST nodes; wired as the default set                                             |
| [`markdown/`](markdown/)       | Text edge: goldmark parser assembly + remark-compatible renderer                                                     |
| [`convert/`](convert/)         | AST ⇄ ADF transforms (`ToADF`, `FromADF`) and their parameter types                                                  |
| [`assets/`](assets/)           | Pluggable attachment store behind the media resolvers — see [Asset store](#asset-store)                              |
| [`debug/`](debug/)             | Tree dumps of both ASTs; debugging aid only                                                                          |
| [`jira/`](jira/)               | **Separate module**: Jira conventions (`MarkdownOptions`, `RenderOptions`, `EncodeRichText`, `CodeLanguages`)        |
| [`confluence/`](confluence/)   | **Separate module**: Confluence conventions (`MarkdownOptions`, `RenderOptions`, page `SmartLinks`, `CodeLanguages`) |
| [`skill/`](skill/)             | **Separate module**: the dialect as an embeddable agent skill (`Files`, `Install`)                                   |
| [`frontmatter/`](frontmatter/) | **Separate module**: optional YAML frontmatter access (`Parse`, `Render`, `Patch`, `PatchPreserving`, path helpers)  |

The root module is platform-neutral ADF ⇄ Markdown. Platform-specific
addons ship as separate submodules (`jira/`, `confluence/`, the
`skill/` agent-skill bundle, and the optional `frontmatter/` YAML
helpers), so consumers only pull what they use.
Smart-link recognition (bare issue keys, `/browse/` URLs, inline cards)
stays in the root module — Confluence content links to Jira issues
through the same ADF nodes.

## Asset store

The `assets` package is the media seam behind the resolvers. `Store` is
a storage-agnostic interface — nothing in it assumes a filesystem, so an
in-memory or object-storage (S3, …) backend is equally implementable;
scope is the one cross-cutting concern every store honors (a media id is
valid within a product container). `FSStore` is the shipped **default**:
a free-form `assets/` folder next to your markdown files, adding
content-addressed deduplication as an implementation detail (the
interface neither requires nor knows about it). `NewFSStore` keeps
assets next to the documents; `NewFSStoreAt`/`NewFSStoreSplit` separate
physical location from the documents; `assets.Layered` stacks stores.
One call wires a store into each half of a composition:

```go
store, _ := assets.NewFSStore(mdDir)

mdOpts := assets.MarkdownOptions(store)
doc := adfast.ToADF(adfast.FromMarkdown(md, mdOpts...), mdOpts...)

rOpts := assets.RenderOptions(store)
out := adfast.ToMarkdown(adfast.FromADF(doc, rOpts...), rOpts...)
```

Markdown-first assets are supported: reference a local file before any
upload and the encode side reports an `unresolved-asset` diagnostic
instead of silently dropping it. The upload seam is the pluggable
`Uploader` interface — `assets.Sync(ctx, store, uploader)` uploads the
pending worklist in one batch, `assets.PushPipeline(ctx, store, uploader)`
returns an `adfast.Pipeline` that uploads referenced pending assets
automatically right before encoding (via a `WithBeforeEncode` hook on the
pipeline), and `assets.EnsureUploaded` syncs first and returns the wired
markdown options. `assets.RewriteReferences(old, new)` re-paths image
references through the formatter (as a `WithASTTransforms` transform)
after a store-layout change.

Attachments have a product-side container boundary (a Jira media id is
bound to one issue, a Confluence one to one page) —
`assets.ForScope(store, "KEY-123")` binds a view to one container so
every document encodes ids valid for _its_ container while local storage
stays deduplicated.

Store internals (on-disk layout, atomicity, layered/split stores, and
the scoping model) are documented in
[docs/design.md](docs/design.md#asset-store-internals).

## Visitors

Both trees ship exhaustive, type-safe visitors: `ast.Visitor[T]` /
`ast.Visit` and `adf.Visitor[T]` / `adf.Visit` (plus `adf.MarkVisitor[T]`).
Implementing a visitor requires one method per kind, so **adding a node
kind breaks every visitor at compile time** — the enforcement a Go type
switch cannot give. Open-set escapes are explicit: unknown markdown kinds
(extension/dialect nodes) dispatch to `VisitExtension`, unknown ADF
content to `VisitRaw`/`VisitRawMark`.

```go
type linkCollector struct{ urls []string }
// … one method per kind; the compiler tells you which are missing …
func (c *linkCollector) VisitLink(l *ast.Link) struct{} {
    c.urls = append(c.urls, l.URL)
    return struct{}{}
}
```

For ADF-side traversal that does not need per-kind dispatch, `adf.Walk`
iterates every node of a subtree as a Go iterator (`for n := range
adf.Walk(root)`), and `adf.Transform(doc, f)` rewrites a document
copy-on-write: `f` returns (replacement, handled) per node — replace,
delete (empty slice), or prune a subtree from the rewrite by returning
the node itself; the input document is never mutated. The `jira`
transforms (issue-link → inlineCard, bare-key expansion) are built on
`adf.Transform`.

Extension packages chain the exhaustiveness: `dialect.Visitor[T]` /
`dialect.Visit` covers the typed dialect kinds — call it from your
`VisitExtension` and unknown-to-the-dialect kinds continue to its
`VisitOther`, where further extension visitors chain. Extension authors
shipping their own kinds should follow the same escape-and-chain shape.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for the toolchain (mise), hooks,
test invocations, and the fixture/fuzz corpus workflow.
Releases are documented in [docs/RELEASING.md](docs/RELEASING.md).
