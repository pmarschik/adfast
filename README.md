# adfast

adfast converts Markdown to and from
[ADF](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)
at the **AST level**. ADF (Atlassian Document Format) is the JSON
document model behind Jira Cloud and Confluence Cloud. The output is
round-trip-stable and remark-compatible. A typed ADF document model
keeps everything that adfast does not understand, and it keeps it
losslessly.

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

adfast needs Go 1.27 or later. [`jira/`](jira/),
[`confluence/`](confluence/), [`skill/`](skill/), and
[`frontmatter/`](frontmatter/) are **separate Go modules**. A
product-specific addon ships as a submodule, so a consumer pulls only
what it uses.

[`wasm/`](wasm/) is a separate module too, but it is a **build artifact
rather than a library**. It compiles the conversion and the directive
dialect to WebAssembly for a JavaScript consumer, such as an editor
integration that must locate and convert directives without a second
parser in TypeScript. Build it instead of a `go get`:

```sh
mise run wasm:build   # writes wasm/adfast.wasm; ship it with wasm/wasm_exec.js
```

Read [`wasm/README.md`](wasm/README.md) for the JS surface, and for the
offsets contract. That contract is the one thing a consumer must not get
wrong.

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

md → adf is `ToADF(FromMarkdown(md))`, and adf → md is
`ToMarkdown(FromADF(doc))`. The four primitives meet at the pivot AST
(`ast.Node`), and each one reads the subset of the shared `adfast.Option`
that it needs. The reverse edge into `FromADF` is `adf.DecodeDoc`. It
turns any JSON-decoded ADF value into the typed `adf.Doc`. Runnable
examples live in [`example_test.go`](example_test.go) and on
[pkg.go.dev](https://pkg.go.dev/github.com/pmarschik/adfast#pkg-examples).

## Key properties

- **Dialect**: CommonMark and GFM, plus
  [remark-directive](https://github.com/remarkjs/remark-directive)-style
  directives through
  [goldmark-directive](https://github.com/pmarschik/goldmark-directive).
  The directives carry the ADF features that have no native syntax. Read
  [Supported Markdown](#supported-markdown).
- **remark-compatible rendering**: the escaping, the list marker
  alternation, the prose wrapping, and the character-reference encoding
  are measured against remark-stringify. They byte-match it on the
  covered corpus.
- **Round-trip stable**: the md → adf → md round trip
  (`ToMarkdown(FromADF(ToADF(FromMarkdown(md))))`) is idempotent. A
  continuously grown fuzz corpus (`FuzzRoundTripIdempotent`) enforces
  this.
- **Formatter**: the prettier md → md formatter is the composition
  `ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`.
  It is a pure md → ast → md pass with prettier-compatible output, and it
  never routes through ADF. Tests enforce its semantic coherence with the
  ADF conversion.
- **Extensible**: a custom node kind plugs into all four pipeline paths
  through one public contract. Read
  [Extending adfast](#extending-adfast).

## How it works

Both conversion directions pivot through one source-independent tree, the
adfast AST. Its semantics mirror the mdast of remark.

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

The pivot is built once from the goldmark parse, and that build normalizes
the quirks of the parser. It is also what the remark-compatible renderer
consumes. The ADF `Doc` and `Node` types are the AST of the ADF side, and
JSON happens only at the very edge.

The facade is FOUR primitives with the pivot AST (`ast.Node`) as the
explicit currency. Each one is named by its non-AST end, and `From*` and
`To*` are inverses at the AST boundary. One shared `adfast.Option` type
serves them all, and each primitive reads the subset that it needs:

| Primitive                            | Shape                  | Role                       |
| ------------------------------------ | ---------------------- | -------------------------- |
| `adfast.FromMarkdown(md, ...Option)` | md → `ast.Node`        | parse (faithful pivot AST) |
| `adfast.ToADF(n, ...Option)`         | `ast.Node` → `adf.Doc` | encode                     |
| `adfast.FromADF(doc, ...Option)`     | `adf.Doc` → `ast.Node` | decode                     |
| `adfast.ToMarkdown(n, ...Option)`    | `ast.Node` → md        | render                     |

The common conversions are compositions. Pass the same options to both
halves, because each primitive ignores what it does not read:

| Conversion | Composition                                                                |
| ---------- | -------------------------------------------------------------------------- |
| md → adf   | `ToADF(FromMarkdown(md))`                                                  |
| adf → md   | `ToMarkdown(FromADF(doc))`                                                 |
| md → md    | `ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())` |

`FromMarkdown` parses to the faithful pivot AST and stops. It produces no
ADF and no canonicalization, so it is the currency that the `To*`
primitives consume. The subpackage functions
`markdown.Parse`/`markdown.Render` and `convert.ToADF`/`convert.FromADF`
sit one layer down under the same shapes. The `To*` primitives normalize
on the way out. `ToADF` encodes through the canonicalizing projection onto
the data model of ADF, and the prettier-format mode of `ToMarkdown`
(`WithPrettierFormat`) runs the shared `convert.Normalize` pass before it
renders.

The prettier md → md formatter is therefore the composition
`ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`.
For a custom width, add `WithPrintWidth(w)` to both calls. `FromMarkdown`
is a single faithful parse in both directions. Text values are fully
decoded, because that is the ADF currency, and the literal escapes of
prettier ride separately on `ast.Text.Raw` as escape provenance. The
format therefore re-emits them byte-for-byte without a change to the
semantic value. Escaping is a render-only concern, and
`WithPrettierFormat` now has NO parse-side effect at all. Both directions
share one `FrontmatterProvider`, so frontmatter detection cannot diverge,
and the flag is read on the render call only. The formatter is a pure
md → ast → md pass. It parses to the pivot AST, applies the
`convert.Normalize` canonicalization, and renders back with the text
rules of prettier. That canonicalization degrades an unknown directive,
resolves `::colwidths` and `::decisions`, canonicalizes the inline marks,
and re-derives the canonical payload of media. Nothing routes through
ADF, so the formatter never loses a construct that ADF cannot model.
Frontmatter, raw HTML, and inline images pass straight through.
`WithASTTransforms` is its content-rewrite seam. Two test obligations
keep the format and the conversion from a drift apart, in place of a
structural guarantee (read `format_contract_test.go`). The first is
semantic coherence: a format followed by a parse produces the same ADF as
a parse of the original. The second is idempotence. Both run over the
fixture corpus, and both are fuzzed continuously
(`FuzzFormatSemanticsPreserved`).

Canonical `ToADF(FromMarkdown(md))` output is wire-safe unless
`WithPreserveListTightness` is enabled, or the source carries a `{#id}`
heading anchor, or a table whose delimiter row carries alignment colons.
Each of these three writes a synthetic attribute, and ADF has no wire form
for it. Run `adf.IsWireSafe` as the guard
before you submit a document of uncertain origin, and use
`adf.StripSynthetic` as the matching cleanup.

For a heading anchor, the product bundles are the better answer.
`confluence.MarkdownOptions` lowers the anchor to the anchor macro of
Confluence. `jira.MarkdownOptions` drops the anchor and reports a
`heading-anchor-dropped` diagnostic. For table alignment, both bundles
install `adf.LowerTableAlign`, which gives every alignable block of an
aligned column the ADF alignment mark. `adf.StripSynthetic` clears the
attribute for a document that no bundle touches.

Media and attachment resolution is pluggable through `WithMediaAssets`,
`WithAssetIDResolver`, and `WithImageDimsResolver`. If the collection of
downloaded files is large, or an entry costs something to produce,
`WithMediaAssetResolver` answers the same question one media id at a
time. The conversion then asks only about the media that it meets.

ADF records no ordered-list marker style, so `FromADF` renders the
reference form: the start number repeated on every item, which matches
remark-stringify with `incrementListMarker` off. Add
`WithIncrementListMarkers` where people write and read the Markdown. The
items renumber `1. 2. 3.`, and a list that a document already spelled
that way survives the round trip unchanged.

Automatic link handling makes **no assumptions about the host product**.
`WithSmartLinks(convert.SmartLinks{KeyFromURL, URLForKey})` teaches the
conversion a URL scheme once for both directions. A link whose text
equals the derived key encodes as an inlineCard, a bare
`::linkCard[KEY]` label expands, and a card renders back to the short
key. `WithLinkResolver(convert.LinkResolver{Encode, Decode})` rewrites
the destination of an ordinary labelled link at the ADF boundary.
`Encode` maps a Markdown href to its product-facing form, and `Decode`
restores the stable Markdown href. A resolver miss keeps the original
destination, and cards and media are unaffected.

`WithFileCards(convert.FileCards{Card, Link})` publishes a labelled link
as the inline file card the host editor writes for an attached file.
`Card` answers with the media id of the attachment, and with the
collection it hangs off. `Link` reads a card back as the link it stands
for, so the round trip returns the Markdown unchanged. The card resolver
sees the href that `LinkResolver.Encode` produced, and `Link` gives its
href to `LinkResolver.Decode`. One link becomes one card, however many
nodes its label was split across. A card holds no label, so a label of
`Link` stands in, and then the alt text of the card, and then the last
segment of the href. A resolver miss keeps the link, and a card the
resolver does not know stays a `::media` directive.

`WithDocTransforms`
hooks document-level rewrites on the encode side. `WithADFTransforms` is
the decode-side mirror of this option. Both exist for the
product-specific shapes that a per-node hook cannot reach. Such a shape
moves content between a node and the attributes of the parent, as
`confluence.LowerAnchors` and `confluence.LiftAnchors` do, or between a
table and the blocks of its cells, as `adf.LowerTableAlign` and
`adf.LiftTableAlign` do.
The [`jira/`](jira/) submodule bundles the
Jira conventions. `jira.MarkdownOptions` and `jira.RenderOptions` each
return a `[]adfast.Option` slice. Pass the encode-side bundle to both
halves of the md → adf composition, and the decode-side bundle to both
halves of the adf → md one:

```go
mdOpts := jira.MarkdownOptions(baseURL, jira.ExpandAuto)
doc := adfast.ToADF(adfast.FromMarkdown(md, mdOpts...), mdOpts...)

rOpts := jira.RenderOptions()
out := adfast.ToMarkdown(adfast.FromADF(doc, rOpts...), rOpts...)
```

The typed `jira.ExpandMode` constants select the bare-key expansion:
`ExpandAuto`, `ExpandAll`, and `ExpandExplicit`. The submodule also ships
`jira.EncodeRichText` with the typed `jira.RichTextFormat` constants
`RichTextADF` and `RichTextText`, where `InferRichTextFormat` matches
whatever an existing field holds. `jira.CodeLanguages` is the code-block
language set of the Jira Cloud editor, for the `WithCodeLanguages` check.

The [`confluence/`](confluence/) submodule bundles the Confluence
conventions the same way. `confluence.MarkdownOptions(baseURL)` and
`confluence.RenderOptions()` wire smart links for the page URLs of
Confluence Cloud: `…/wiki/spaces/KEY/pages/123456789/Title` ⇄ the stable
`KEY/123456789` key. The mutable title slug is deliberately not part of
the key. The submodule also ships `confluence.CodeLanguages`, the
language set of the code block macro of Confluence Cloud, which is a much
smaller set than the editor list of Jira.

`confluence.RepairReadBack(doc, storage)` repairs what a page read
loses. Confluence converts a page to ADF from its own storage format,
and that conversion drops the `code` mark on link text and the title
slug of an internal page link. The storage body of the same page version
holds both, so the repair reads it as the oracle. Call it on the document
that the read returned, before `FromADF`. A comparison between the local
document and the page it published then reports no difference that nobody
made. `docs/design.md` holds the measurements.

Some pages carry a `legacy-content` extension: Confluence's own wrapper
for a blockquote or a table nested inside a list item, a shape ADF's
content model forbids. The page still renders — the wrapper carries the
original storage HTML — but reading it back as ADF used to hand you the
wrapper itself, a screenful of escaped HTML and JSON through the generic
`::extension` directive. `confluence.RenderOptions()` installs
`confluence.ExpandLegacyContent`, which replaces the wrapper with the
ADF content its `nestedContent` parameter carries, in the position the
wrapper held. It is also available standalone
(`adfast.WithADFTransforms(confluence.ExpandLegacyContent)`) for a
caller composing options by hand. `docs/design.md` holds the details.

Both bundles also install `confluence.Macros()`, named directives for the
core Confluence macros. The generic `::extension{key type parameters}`
directive can express each one, but the `parameters` attribute carries a
JSON blob that nobody wants to hand-write. The macros that people use
therefore get a directive of their own:

| Directive                     | Macro key         | Notes                                         |
| ----------------------------- | ----------------- | --------------------------------------------- |
| `::toc{maxLevel="3"}`         | `toc`             | Table of contents                             |
| `::children{sort="title"}`    | `children`        | Child pages                                   |
| `::pagetree{root="Notes"}`    | `pagetree`        | Page tree                                     |
| `:::excerpt{name="…"}` + body | `excerpt`         | Excerpt definition (the bodied form)          |
| `::excerptInclude[Page]`      | `excerpt-include` | Insert excerpt — the label is the target page |
| `::includePage[Page]`         | `include`         | Include page — the label is the target page   |

Macro parameters ride as directive attributes, and the unnamed parameter
of the macro is the `[label]`. Every name registers in all three
directive positions (`::name`, `:name`, `:::name`). The same macro key
genuinely appears as a block, an inline, and a bodied node in live pages,
and the ADF node type decides the form on the way back. A container that
holds nothing, such as `:::toc` with an empty body, encodes as the
bodiless macro: ADF gives `bodiedExtension` a required body, and
Confluence answers an empty one by dropping the macro.

Everything that Confluence derives is left out of the markdown. `macroId`
is server-generated, so a macro written without one comes back with one
filled in. `schemaVersion` and `title` are constant per macro key.
Therefore the encode synthesizes all three and the decode drops them, and
a plain table of contents is no more than `::toc`. A value that
_diverges_ from the per-key default survives as an explicit
`schemaVersion=` or `title=` attribute instead of a silent rewrite.
`layout="default"` is dropped, because it is the default.

The sugar claims only what it can carry exactly. Four cases decline and
degrade through the generic `::extension` path with the `parameters` JSON
intact: an unsugared macro key, a non-string parameter, an unexpected
metadata field, and a parameter named like one of the reserved attributes
(`layout`, `localId`, `schemaVersion`, `title`). The sugar was measured
against 182 macro instances in 150 live pages. Every one round-tripped
through the sugar, and none degraded.

The [`skill/`](skill/) submodule ships the markdown dialect as an
**agent skill**. It is an embedded bundle of `SKILL.md` plus
`references/`, and it holds the complete syntax, the ADF coverage, a
format-stable worked example, and the pitfalls. It teaches an AI coding
agent to read and write adfast-flavored markdown. A host serves it
through `skill.Files()`, an `fs.FS`, or materializes it with
`skill.Install(dir)` into its own agent-skills directory, such as
`.claude/skills/`.

Leading document metadata is pluggable through
`WithFrontmatterProvider`. The default handles YAML `---` frontmatter.
Supply your own provider for another convention, for example the
`<!-- Space: X -->` HTML-comment headers of the Confluence sync tools.
The same provider drives BOTH directions, md → adf and the formatter, so
detection cannot diverge between them. A found block never reaches the
parser, and the style-preserving formatter re-emits it verbatim. A
provider can also report a block as _malformed_, which means that the
block opens the convention but does not close validly. The bytes are then
kept as body and a `malformed-frontmatter` diagnostic fires.

The core stays YAML-neutral. The front block is opaque bytes, kept
verbatim on `ast.Frontmatter.Value`, delimiters included. A consumer who
wants _structured_ access to a YAML block opts into the
[`frontmatter/`](frontmatter/) submodule. It turns the raw block into a
`map[string]any` and back, and it does not couple the core to a YAML
implementation. It offers `frontmatter.Parse` and `Render`,
`frontmatter.Patch` (a merge under a caller-supplied top-level key order,
where a nil value erases the key), `Replace`, the nested dot-path helpers
`Get`, `Set`, and `Remove`, and `KeyOrder`, which reads the authored order
back out. It never re-implements boundary detection. That work stays with
the `FrontmatterProvider`, and `frontmatter.ParseNode` bridges straight
from an `ast.Frontmatter` node. For a hand-authored block where
formatting matters, `frontmatter.PatchPreserving` edits the changed keys
on the YAML CST only. It keeps the original key order, the comments, and
the scalar styles of everything that it does not touch.

### Reusable pipelines

The two halves of a composition drift apart easily. An extension that is
registered on the parse call only parses, but it never decodes back. A
`Pipeline` registers the cross-cutting options once for BOTH directions
and shows the composed one-shot conveniences, so every call reuses the
same options. A `Pipeline` is immutable and safe for concurrent use:

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

There is one shared option type, so every cross-cutting option goes
through `WithPipelineOptions`. There are no direction-specific pipeline
constructors. `pipe.MarkdownToADFAll(mds)` is the batched variant. It
parses every document, runs the `WithBeforeEncode(hooks…)` hooks over the
whole set of parsed ASTs, and then encodes each one. Cross-document work,
such as a single batched asset upload, therefore happens before anything
encodes. `pipe.ADFBytesToMarkdown(v)` decodes raw ADF JSON, or any
decoded value, first. The free primitives stay as sugar for a one-off
call.

## When not to use adfast

- **ADF → HTML rendering** — adfast targets markdown, not display HTML.
  Use the frontend tooling of Atlassian to render ADF for viewing.
- **Jira Data Center and Server** — those APIs speak wiki markup, not
  ADF. adfast covers Cloud ADF only.
- **Full API clients** — adfast converts documents, and it does not talk
  to the Atlassian APIs. Pair it with a client library such as
  go-atlassian or go-jira for the transport.

## Error handling

The four primitives never return an error and never panic. `FromMarkdown`,
`FromADF`, `ToADF`, and `ToMarkdown` always produce a result. A lossy or
recovered situation flows through a diagnostics sink instead:

- an orphan `::colwidths` or `::decisions` that is dropped
  (`colwidths-orphan`, `decisions-orphan`),
- a table span marker whose merge cannot apply (`span-marker-invalid`),
- a code-block language outside a configured `WithCodeLanguages` set
  (`unsupported-code-language`),
- a node or a mark that the target product does not render
  (`unsupported-in-product`, described below),
- a recovered parser panic (`parse-recovered`),
- an unknown ADF node that reaches the markdown projection (`raw-node`),
- a retired `:fontSize` that is dropped to plain text
  (`fontsize-dropped`),
- a heading anchor dropped because the target product has no anchor
  construct (`heading-anchor-dropped`, from `WithoutHeadingAnchors`),
- an inline `![alt](https://…)` rewritten as a link because ADF has no
  inline image for an external URL (`inline-image-degraded`),
- a GFM footnote flattened to a superscript and a list at the end of the
  document, because ADF has no footnote (`footnote-flattened`),
- a blockquote, a table or another block inside a list item, which ADF's
  `listItem` content model cannot carry (`list-item-content`, described
  below).

One `WithDiagnostics(func(convert.Diagnostic))` wires the sink into
whichever primitive emits: parse notices on `FromMarkdown`, encode
notices on `ToADF`, and decode notices on `FromADF`. Pass it to whichever
primitives a composition runs. Without a sink, every diagnostic is
silently dropped. `Pipeline.MarkdownToADFAll` is the errable batch
variant. A failure of a `BeforeEncode` hook, for example a batched asset
upload, aborts the call and returns the error.

### Product availability (`unsupported-in-product`)

The core conversion is universal, and it round-trips a Confluence
document faithfully. Product availability is therefore enforced as an
authoring-side **diagnostic**, not as a change to the conversion.
`WithUnsupportedKinds(product, kinds)` declares the ADF node kinds and
mark kinds that a target product does not render. After `ToADF` produces
the document, it walks both the nodes and the marks and emits one
`unsupported-in-product` diagnostic per distinct offending kind, for
example `placeholder is not available in jira`. No node is dropped and no
node is altered, and the output is byte-identical with and without the
option. The **consumer decides the severity**, and it can treat the
diagnostic as a blocking error before a Jira-targeted push.

The product sets are scoped to **render-confirmed non-support**. A full
live probe on 2026-07-22 showed each such kind dropped, shown as an
unsupported-content block, rejected by the ADF endpoint of the product,
or stripped or downgraded on save. The sets are not
documentation-by-omission, which proved unreliable. The Jira docs are
non-exhaustive, the Jira REST accepts most of the shared schema, and Jira
renders most omitted kinds first-class, layoutSection, cards, status, the
extension family, syncBlock, and the alignment, indentation, breakout,
annotation, fragment, and dataConsumer marks among them. Therefore
`jira.UnsupportedKinds` is `placeholder`, which the render drops, plus
`multiBodiedExtension` and `extensionFrame`, which the Jira REST endpoint
rejects with INVALID_INPUT. `confluence.UnsupportedKinds` is
`blockTaskItem`, which Confluence downgrades to a plain taskItem. Both
sets are wired through `jira.MarkdownOptions` and
`confluence.MarkdownOptions`. `fontSize` is in neither set, although both
products reject it. adfast **retires** the mark and never produces one,
described in the `fontSize` note below, so an `unsupported-in-product`
check for it would be moot. A new kind needs a live-probe confirmation,
not a missing docs page. The evidence and the full availability data live
in `docs/adf-coverage.md` and `docs/adf-availability.json`.

### List item content (`list-item-content`)

ADF gives `listItem` the content model `(paragraph | bulletList |
orderedList | taskList | mediaSingle | codeBlock | unsupportedBlock |
extension)+` (the pinned schema oracle, `docs/adf-coverage.md`), a
single flat repeatable alternation, so a blockquote, a table, a heading,
a rule, a panel or a mediaGroup inside a list item is not representable,
however sensible the markdown that produced it. `extension` and
`unsupportedBlock` ARE in the alternation, so those two never raise this
diagnostic. `ToADF` does not restructure the author's document to fit
the model — lifting the block out of the item changes what the document
says, and re-nesting it changes the structure the author chose — so it
encodes exactly as written and emits one `list-item-content` diagnostic
per distinct offending kind instead. A live probe (2026-08-26) showed
Confluence accepting such a push, rendering the page correctly, and
rewriting the offending subtree on save into a bodiless
`com.atlassian.confluence.migration` / `legacy-content` extension;
`confluence.ExpandLegacyContent` reads that wrapper back. The pinned
model is one flat alternation with no first-position restriction to
enforce — older ADF schema revisions did restrict what could open a
listItem, but this repo's pin postdates that rule, so a list item whose
first block is a nested list (`- - x`) is not a violation at all, and
adfast does not report it.

## Supported Markdown

The base dialect is **CommonMark and GFM**. It gives pipe tables, padded
to column width, with the cell merging of
[remark-extended-table](https://github.com/wataru-chocola/remark-extended-table).
A cell that holds `>` only merges into the cell to its right, and a cell
that holds `^` only extends the cell above. Literal `>` and `^` cell
content is escaped. GFM also gives task lists (`- [ ]` and `- [x]`),
strikethrough, autolink literals, and footnotes (`[^1]` and
`[^1]: note`). On top of that come four things.
The first is decision lists, where a `::decisions` leaf directive marks
the plain bullet list that follows it, exactly like `::colwidths` marks
the table that follows. The second is YAML frontmatter, which is
pluggable through `WithFrontmatterProvider`. The third is heading anchors,
`## Title {#my-anchor}`, the pandoc spelling. The fourth is the directive
dialect below: `:name[label]{attrs}` inline, `::name[label]{attrs}` as a
block leaf, and `:::name … :::` as a container. Everything below
round-trips losslessly through ADF.

### Container directives (block elements)

| Markdown                               | ADF                          | Notes                                                                                                                                                                                                                   |
| -------------------------------------- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `:::info` … `:::`                      | panel                        | Also `note`, `warning`, `success`, `error`                                                                                                                                                                              |
| `:::expand[Title]` … `:::`             | expand                       | Title is optional; nests inside panels as nestedExpand                                                                                                                                                                  |
| `:::media[alt]{…}` … `:::`             | mediaSingle + caption        | The `::media` attrs on the fence line, the caption as the body; a plain-text caption on image-expressible media uses the image title instead: `![alt](path "caption")`                                                  |
| `:::extension{…}` … `:::`              | bodiedExtension              | Same attrs as `::extension`; when every child is a `:::frame` container (extensionFrame) it encodes as multiBodiedExtension (a frameless one carries the bare `multi`); an empty body encodes as the bodiless extension |
| `:::syncBlock{resourceId localId}` …   | bodiedSyncBlock              | The source body of a synced block                                                                                                                                                                                       |
| `:::section` + `:::column{width="…"}`  | layoutSection / layoutColumn | Page layouts; `columnRuleStyle`/`localId` on the section, `width`/`valign`/`localId` on each column                                                                                                                     |
| `:::center` / `:::end` … `:::`         | alignment mark               | Block mark on each wrapped paragraph/heading                                                                                                                                                                            |
| `:::indent{2}` … `:::`                 | indentation mark             | The bare value is the level (1–6)                                                                                                                                                                                       |
| `:::breakout{wide}` … `:::`            | breakout mark                | Modes `wide`/`full-width`; optional `width="1200"`                                                                                                                                                                      |
| `:::dataConsumer{sources="id1,id2"}` … | dataConsumer mark            | `sources` is a comma-separated list of source ids (opaque strings; parsed by splitting on commas and trimming)                                                                                                          |
| `:::fragment{localId="…" name?}` …     | fragment mark                | Stable references to tables/extensions                                                                                                                                                                                  |

A nested container grows the outer fence (`::::`), like remark. The
mark-wrapper containers (`:::center` and `:::end`, `:::indent`,
`:::breakout`, `:::dataConsumer`, `:::fragment`) put the ADF **block
mark** on every block that they wrap. The wrappers compose by nesting,
and the ADF mark array maps inside-out onto that nesting, with the first
mark innermost, so a round trip keeps the mark order. A single-valued
directive takes the **bare-value attribute form** (`{2}`, `{wide}`,
`{small}`), where exactly one attribute with an empty value is the value
of the directive. When both forms are present, a named `level=`, `mode=`,
or `size=` attribute wins. An arbitrary-JSON payload, such as
`parameters` on an extension, uses a **canonical JSON attribute
encoding**: the output of `json.Marshal`, with sorted keys and no
insignificant whitespace. That JSON holds a `"`, so the attribute is
**single-quoted** and stays readable and lossless, as in
`parameters='{"station":"rooftop"}'`. When the JSON value itself holds a
`'`, a single quote would not be lossless. The attribute then falls back
to double quotes and writes every `"` as `&quot;`. This is
remark-compatible, because remark decodes a character reference in an
attribute value. The `sources` attribute of `dataConsumer` is a plain
comma-separated list of source ids, and it is not JSON.

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

A media directive also carries the `borderColor` and `borderSize`
attributes, for the ADF border mark on the media node.

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

A mark directive nests with regular emphasis, as in
`:color[**bold red**]{color="#ff5630"}`. An inline mark directive wraps
per text run, in a fixed nesting order from outside to inside:
`:annotation`, `:color`, `:bg`, `:u`, and `:sub` or `:sup`. (`:fontSize`
is retired. It parses, but it drops to plain text.) A directive label
cannot nest brackets, so overlapping annotation marks on one text run
degrade to the outermost anchor.

Every known directive parses into a **typed AST node** in the package
[`dialect/`](dialect/), and that node implements the public extension
contract. An unknown directive name keeps the generic directive kinds and
degrades exactly like remark: a container dissolves into its content, an
unknown leaf drops, and an unknown text directive flattens to text.

### Related conventions (no directive needed)

- **Attachments as images** — wire in a media-asset store, with
  `WithMediaAssets` or `WithMediaAssetResolver`, plus
  `WithAssetIDResolver` and `WithImageDimsResolver`. File media whose
  local copy carries every ADF property then renders as a plain
  `![alt](assets/shot.png)`, and it maps back to its media id on encode.
  Anything richer keeps the `::media` directive: a PDF, resized media, or
  a non-default layout.
- **Inline images** — an image inside a paragraph, a table cell, or a
  list item has three fates, because ADF's inline media covers only one
  of them. A path the asset store maps to a media id becomes a
  `mediaInline` chip, the faithful form, and reads back as the same
  `![alt](path)` when the store is wired on the render side too. An
  absolute `http(s)` URL has no faithful form at all, because
  `mediaInline` addresses an uploaded attachment by id and has no
  external variant, unlike block media. It therefore degrades to the
  link it can still be — the alt text is the label, the image URL the
  href — with an `inline-image-degraded` diagnostic. Any other path is
  an asset not in the store yet, so it drops with an `unresolved-asset`
  diagnostic that an upload flow can act on.
- **Footnotes** — GFM footnotes, `a[^1]` with `[^1]: the note`. The
  label rules are micromark's: no whitespace inside it, not even escaped
  (`[^a b]:` is a link reference definition), an escaped `\[` allowed
  where a raw one is not, and a reference with no definition in the same
  document stays literal text. The md → md route keeps both ends where
  the source put them, so the formatter never moves, sorts, or deletes a
  definition. ADF has no footnote of any kind, so the ADF route
  **flattens**: each reference becomes its number as superscript text,
  and every definition collects at the end of the document behind a
  `rule`, as one `orderedList` whose item numbers are those numbers. The
  numbering is definition order, the order of that list. A reference
  carries no link to its definition, because ADF has no anchor to link
  to. This is the one construct that does not come back: `adf → md`
  returns the flattened form, and each flattened footnote reports a
  `footnote-flattened` diagnostic.
- **Heading anchors** — `## Title {#my-anchor}` gives the heading an
  explicit anchor id. This is the spelling of pandoc and of
  remark-heading-id. The id must match `[0-9A-Za-z][0-9A-Za-z._-]*`, and
  a space must separate it from the heading text. Any other form stays
  literal text, and an escaped brace (`## Title \{#lit}`) always stays
  literal. ADF has no platform-neutral anchor, so the id rides as a
  synthetic never-wire attribute (`adf.Heading.Anchor`) that the addon of
  the host product resolves. `confluence.MarkdownOptions` lowers it to
  the anchor macro of Confluence, `confluence.RenderOptions` lifts it
  back, and `jira.MarkdownOptions` drops it with a
  `heading-anchor-dropped` diagnostic.
- **Issue links** — a link whose text equals the resolver-derived key,
  for example `[ABC-123](https://…/browse/ABC-123)`, becomes an
  inlineCard.
- **Image titles as captions** — `![alt](path "caption")` maps the title
  to a mediaSingle caption child. A richer caption, with formatting or a
  hard break, uses the `:::media` container form.
- **Tables** — GFM pipe tables. A header row is synthesized when the ADF
  table has none. Column alignment (`|:--|--:|:-:|`) survives the ADF
  route: ADF tables have no alignment attribute of any kind, so the
  per-column list rides as a synthetic never-wire attribute
  (`adf.Table.Align`), and the render places the colons and the cell
  padding exactly where remark-stringify does. Both product bundles
  lower the attribute onto the ADF alignment mark of each cell block
  (`adf.LowerTableAlign`), and read it back (`adf.LiftTableAlign`).
  The mark spells only center and end, so a left-aligned column comes
  back unaligned, which is what it renders as.

## A complete example

One document that exercises most of the dialect, usable as a template. It
round-trips through `FromMarkdown` → `ToMarkdown` unchanged, and a test
extracts this block and asserts that:

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
so this clause starts on its own line. New keepers sign the [rota](#rota) where a :placeholder[your name here…] marks an empty template field to fill in later — that link points at a heading's explicit anchor id, written as a `{#rota}` suffix on the heading itself.

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

## Inspection rota {#rota}

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

The tables below hold every node type and mark type in the ADF schema of
Atlassian
([`@atlaskit/adf-schema`](https://unpkg.com/browse/@atlaskit/adf-schema/dist/json-schema/v1/),
full and stage-0, cross-checked against the
[ADF reference](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)).
Each row states whether the kind can occur in the documents of that
product, and how adfast treats it.

> **Fully cited matrix:** every row below lives in
> [docs/adf-coverage.md](docs/adf-coverage.md), with the upstream
> schema-definition link of each kind, pinned to a mirror commit SHA, and
> the exact Jira and Confluence evidence behind its marker. The
> machine-readable form is
> [docs/adf-availability.json](docs/adf-availability.json). Both columns
> were confirmed empirically on 2026-07-22 against a live Cloud site.
> Every node and mark was written to a Jira issue and to a Confluence
> page, and the product-rendered DOM was inspected. The Confluence page
> was also read back, to see what survives the save. Read the "Empirical
> validation" section of the cited matrix.

**Per-product marker** — whether the kind can occur in the documents of
that product. As of 2026-07-22 the markers reflect live render and
round-trip evidence, not documentation alone:

- **✓** — available. The product renders it first-class, or renders it
  degraded but present (Jira), or keeps it on save (Confluence).
- **∘** — present in the shared ADF schema, but genuinely untestable
  here, for example attachment-gated file media.
- **—** — not available. The render drops it, the ADF endpoint of the
  product rejects it, or the save strips or downgrades it.

**adfast support** — the handling of adfast itself, independent of
product availability:

- **converted** — the kind has a markdown mapping and round-trips through
  it.
- **preserved** — the kind survives an ADF decode → encode losslessly,
  typed or as a `RawNode` or `RawMark`, but the markdown projection drops
  or reduces it, with a `raw-node` diagnostic.
- **dropped** — deliberately retired. adfast never produces the kind, and
  a legacy instance decodes to plain text with a `fontsize-dropped`
  diagnostic. The text is kept and the styling is lost. `fontSize` is the
  only such kind, because no Atlassian product supports the mark.

The product-availability diagnostic uses the render-confirmed
not-available set. Read
[Product availability](#product-availability-unsupported-in-product).
`jira.UnsupportedKinds` is `placeholder`, which the render drops, plus
`multiBodiedExtension` and `extensionFrame`, which the Jira REST endpoint
rejects with INVALID_INPUT. Jira renders every other kind that the probe
covered. `confluence.UnsupportedKinds` is `blockTaskItem`, which
Confluence downgrades to a plain taskItem. `fontSize` is in neither set.
Both products reject it, but adfast retires the mark and never produces
one, so the check would be moot.

| ADF node                                      | Jira | Confluence | adfast support | Markdown mapping / notes                                                                                                                                                                                                                                                    |
| --------------------------------------------- | ---- | ---------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| doc                                           | ✓    | ✓          | converted      | document root                                                                                                                                                                                                                                                               |
| paragraph                                     | ✓    | ✓          | converted      | paragraph                                                                                                                                                                                                                                                                   |
| text                                          | ✓    | ✓          | converted      | plain text carrying the marks below                                                                                                                                                                                                                                         |
| heading                                       | ✓    | ✓          | converted      | `#`–`######`, with a trailing `{#id}` as the explicit anchor id (see Related conventions)                                                                                                                                                                                   |
| blockquote                                    | ✓    | ✓          | converted      | `>`                                                                                                                                                                                                                                                                         |
| rule                                          | ✓    | ✓          | converted      | `---`                                                                                                                                                                                                                                                                       |
| codeBlock                                     | ✓    | ✓          | converted      | fenced code block; fence grows past embedded backtick runs; language survives                                                                                                                                                                                               |
| bulletList / orderedList / listItem           | ✓    | ✓          | converted      | `-` / `1.` lists; marker alternation between adjacent lists; `order` start preserved                                                                                                                                                                                        |
| taskList / taskItem                           | ✓    | ✓          | converted      | `- [ ]` / `- [x]`; `localId` regenerates as empty on encode                                                                                                                                                                                                                 |
| blockTaskItem                                 | ✓    | —          | converted      | `- [ ]` + indented blocks; a single-paragraph item re-encodes as the inline taskItem. Jira renders it first-class; Confluence downgrades it to a plain taskItem                                                                                                             |
| decisionList / decisionItem                   | ✓    | ✓          | converted      | `::decisions` + following plain bullet list; encodes with state DECIDED; Jira renders decisions first-class (live 2026-07-22)                                                                                                                                               |
| table / tableRow / tableHeader / tableCell    | ✓    | ✓          | converted      | GFM pipe table; colspan/rowspan via `>`/`^` markers; colwidth attrs via `::colwidths`; column alignment rides the synthetic never-wire `align` attribute (ADF has none), which the product bundles lower onto the alignment mark of each cell block                         |
| panel                                         | ✓    | ✓          | converted      | `:::info` …; unknown panelType degrades to `info`                                                                                                                                                                                                                           |
| expand / nestedExpand                         | ✓    | ✓          | converted      | `:::expand[Title]` …; encode always emits `expand` (Jira nests it as nestedExpand itself)                                                                                                                                                                                   |
| mediaSingle / mediaGroup / media              | ✓    | ✓          | converted      | `![alt](path)` or `::media`; plain image only when fully expressible; groups fan out to `group="true"` items                                                                                                                                                                |
| mediaInline                                   | ∘    | ✓          | converted      | `:media{…}` inline attachment chip, or an inline `![alt](path)` the asset store maps to a media id; an inline image with an external URL has no ADF form and degrades to a link. Jira is attachment-gated — not injection-testable with synthetic ids, so left inconclusive |
| caption                                       | ✓    | ✓          | converted      | image title (`![alt](path "caption")`) when plain text on image-expressible media, else the `:::media` body                                                                                                                                                                 |
| inlineCard                                    | ✓    | ✓          | converted      | `[KEY](url)` link; encodes back to inlineCard when the label equals the resolver-derived key                                                                                                                                                                                |
| blockCard                                     | ✓    | ✓          | converted      | `::linkCard[…]`; URL-less cards are dropped                                                                                                                                                                                                                                 |
| blockCard + datasource                        | ✓    | ∘          | converted      | `::jql[…]{…}` — only the documented jira/jql shape; richer shapes fall back to `::linkCard`                                                                                                                                                                                 |
| embedCard                                     | ✓    | ✓          | converted      | `::linkEmbed[…]{…}`                                                                                                                                                                                                                                                         |
| mention                                       | ✓    | ✓          | converted      | `:mention[Name]{#id}`                                                                                                                                                                                                                                                       |
| emoji                                         | ✓    | ✓          | converted      | with a `text` attr: that text (deliberately lossy — shortName/id degrade to plain text across markdown); without: unicode via the emoji-toolkit shortname table, else `:emoji{shortName…}`                                                                                  |
| status                                        | ✓    | ✓          | converted      | `:status[Text]{color}`                                                                                                                                                                                                                                                      |
| date                                          | ✓    | ✓          | converted      | `:date[2026-07-15]{timestamp="…"}`; the timestamp attribute is authoritative                                                                                                                                                                                                |
| hardBreak                                     | ✓    | ✓          | converted      | backslash / trailing-space break. A newline in the text node beside the break is the producer's wrap — Confluence writes one after every `<br/>` — and drops, rather than folding to a space markdown could only write as `&#x20;`                                          |
| placeholder                                   | —    | ✓          | converted      | `:placeholder[Type something…]`                                                                                                                                                                                                                                             |
| layoutSection / layoutColumn                  | ✓    | ✓          | converted      | `:::section` containing `:::column{width="…"}` containers. Jira renders a real multi-column layout (live 2026-07-22)                                                                                                                                                        |
| extension / bodiedExtension / inlineExtension | ✓    | ✓          | converted      | `::extension{…}` / `:::extension{…}` + body / `:extension{…}`. Jira renders them (ak-renderer-extension / inline fallback); Confluence resolves known macros, and `confluence.Macros()` sugars the common ones (`::toc`, `:::excerpt`, `::includePage[Page]`, …)            |
| multiBodiedExtension / extensionFrame         | —    | ✓          | converted      | `:::extension{…}` whose children are all `:::frame` containers; stage-0 schema. Jira REST rejects them (INVALID_INPUT); Confluence preserves them                                                                                                                           |
| syncBlock / bodiedSyncBlock                   | ✓    | ✓          | converted      | `::syncBlock{…}` (reference) / `:::syncBlock{…}` + body (source). Jira renders the sync-block widget (live 2026-07-22)                                                                                                                                                      |

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

In short: unknown or undocumented ADF content **survives an ADF-level
round trip losslessly**, and diagnostics can report it. Only the markdown
projection reduces it. Every kind in the table has a markdown mapping, so
a document also survives markdown-only persistence: render, store the
file, re-parse, and push. The mechanics are documented in
[docs/design.md](docs/design.md): the RawNode and Extra preservation, the
diagnostic codes, and the few deliberate edge-case losses.

## Extending adfast

The [`extension/`](extension/) package defines the public contract for a
custom node kind. A kind must support **all four pipeline paths**. A
capability fragment, render-only or encode-only, is rejected at
registration:

1. **md → ast**: a parse constructor promotes a generic directive node
   (`Registration.Containers`/`Leaves`/`Texts`, keyed by directive name).
2. **ast → md**: the node's `RenderMarkdown` writes its directive form
   through `extension.RenderContext`.
3. **ast → adf**: the node's `EncodeADF` returns its ADF form through
   `extension.EncodeContext`.
4. **adf → ast**: a decode hook recognizes the ADF shape the kind owns
   (`Registration.DecodeBlock`/`DecodeBlockList`/`DecodeInline`).

The known dialect, in the package [`dialect/`](dialect/), is implemented
on exactly this contract. It is therefore both the default registration
set and the reference implementation. Here is a complete custom kind: a
fictional `:youtube[dQw4w9WgXcQ]` inline directive for an invented
`youtube` ADF node.

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

Wire it in. One `adfast.WithExtensions` covers every direction, because
the facade forwards to `markdown.WithExtensions` on the parse and to
`convert.WithExtensions` on the encode and the decode. The default
dialect set stays active. Register the same bundle on both halves of a
composition, so that the parse leg and the decode leg never drift apart:

```go
reg := adfast.WithExtensions(youtubenode.Registration())
doc := adfast.ToADF(adfast.FromMarkdown(md, reg), reg)
out := adfast.ToMarkdown(adfast.FromADF(doc, reg), reg)
```

A block kind also embeds `ast.BlockSpacing`, the blank-line structure.
When it renders a `:::` container, it implements
`extension.ContainerForm`, so that the enclosing fences grow around it.
Three dialect behaviors deliberately stay structural in `convert`,
because they cross node boundaries: the `::colwidths` ↔ table attachment,
the `::decisions` ↔ bullet-list marking, and the decode of the inline
mark directives, which ADF stores as text marks instead of nodes. Read
the documentation of the `dialect` package.

## Layout

The module is layered into public subpackages along the pipeline stages,
and the root package is a thin facade that composes them. The full
package notes live in [docs/design.md](docs/design.md).

| Package                        | Purpose                                                                                                                                        |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `adfast` (root)                | The facade: the four `FromMarkdown`/`FromADF`/`ToADF`/`ToMarkdown` primitives, `Pipeline`, the shared option set                               |
| [`adf/`](adf/)                 | Typed ADF document model + JSON codec; lossless `RawNode`/`RawMark`/`Extra` preservation                                                       |
| [`ast/`](ast/)                 | The pivot Markdown AST (remark-mdast-shaped) both directions share                                                                             |
| [`extension/`](extension/)     | Public AST extension contract (`Node`, context interfaces, `Registration`)                                                                     |
| [`dialect/`](dialect/)         | The known directive dialect as typed AST nodes; wired as the default set                                                                       |
| [`markdown/`](markdown/)       | Text edge: goldmark parser assembly + remark-compatible renderer                                                                               |
| [`convert/`](convert/)         | AST ⇄ ADF transforms (`ToADF`, `FromADF`) and their parameter types                                                                            |
| [`assets/`](assets/)           | Pluggable attachment store behind the media resolvers — see [Asset store](#asset-store)                                                        |
| [`debug/`](debug/)             | Tree dumps of both ASTs; debugging aid only                                                                                                    |
| [`jira/`](jira/)               | **Separate module**: Jira conventions (`MarkdownOptions`, `RenderOptions`, `EncodeRichText`, `CodeLanguages`)                                  |
| [`confluence/`](confluence/)   | **Separate module**: Confluence conventions (`MarkdownOptions`, `RenderOptions`, page `SmartLinks`, `CodeLanguages`, macro sugar via `Macros`) |
| [`skill/`](skill/)             | **Separate module**: the dialect as an embeddable agent skill (`Files`, `Install`)                                                             |
| [`frontmatter/`](frontmatter/) | **Separate module**: optional YAML frontmatter access (`Parse`, `Render`, `Patch`, `PatchPreserving`, path helpers)                            |
| [`wasm/`](wasm/)               | **Separate module**: a js/wasm build exposing conversion and directive span scanning to JavaScript — a build artifact, not an importable API   |

The root module is platform-neutral ADF ⇄ Markdown. A platform-specific
addon ships as a separate submodule, so a consumer pulls only what it
uses: `jira/`, `confluence/`, the `skill/` agent-skill bundle, and the
optional `frontmatter/` YAML helpers. Smart-link recognition stays in the
root module, for bare issue keys, `/browse/` URLs, and inline cards.
Confluence content links to a Jira issue through the same ADF nodes.

`wasm/` is a submodule for the same reason, but it is a different kind of
thing: no Go code imports it. It compiles to WebAssembly and it is
consumed from JavaScript. It is therefore versioned and tagged with the
rest, but delivered as a `.wasm` file instead of a `go get`.

## Asset store

The `assets` package is the media seam behind the resolvers. `Store` is a
storage-agnostic interface. Nothing in it assumes a filesystem, so an
in-memory backend or an object-storage backend, on S3 or elsewhere, is
equally implementable. Scope is the one cross-cutting concern that every
store honors, because a media id is valid inside a product container
only. `FSStore` is the shipped **default**: a free-form `assets/` folder
next to your markdown files. It adds content-addressed deduplication as
an implementation detail, and the interface neither requires that nor
knows about it. `NewFSStore` keeps the assets next to the documents.
`NewFSStoreAt` and `NewFSStoreSplit` separate the physical location from
the documents, and `assets.Layered` stacks stores. One call wires a store
into each half of a composition:

```go
store, _ := assets.NewFSStore(mdDir)

mdOpts := assets.MarkdownOptions(store)
doc := adfast.ToADF(adfast.FromMarkdown(md, mdOpts...), mdOpts...)

rOpts := assets.RenderOptions(store)
out := adfast.ToMarkdown(adfast.FromADF(doc, rOpts...), rOpts...)
```

Markdown-first assets are supported. Reference a local file before any
upload, and the encode side reports an `unresolved-asset` diagnostic
instead of a silent drop. The upload seam is the pluggable `Uploader`
interface. `assets.Sync(ctx, store, uploader)` uploads the pending
worklist in one batch. `assets.PushPipeline(ctx, store, uploader)`
returns an `adfast.Pipeline` that uploads the referenced pending assets
automatically, immediately before the encode, through a
`WithBeforeEncode` hook on the pipeline. `assets.EnsureUploaded` syncs
first and returns the wired markdown options.
`assets.RewriteReferences(old, new)` re-paths the image references
through the formatter, as a `WithASTTransforms` transform, after a change
to the store layout.

An attachment has a product-side container boundary. A Jira media id is
bound to one issue, and a Confluence one to one page.
`assets.ForScope(store, "KEY-123")` binds a view to one container, so
every document encodes ids that are valid for _its_ container, while
local storage stays deduplicated.

The store internals are documented in
[docs/design.md](docs/design.md#asset-store-internals): the on-disk
layout, the atomicity, the layered and split stores, and the scoping
model.

## Visitors

Both trees ship exhaustive, type-safe visitors: `ast.Visitor[T]` with
`ast.Visit`, and `adf.Visitor[T]` with `adf.Visit`, plus
`adf.MarkVisitor[T]`. A visitor needs one method per kind, so **a new
node kind breaks every visitor at compile time**. That is the enforcement
a Go type switch cannot give. The open-set escapes are explicit. An
unknown markdown kind, which is an extension node or a dialect node,
dispatches to `VisitExtension`. Unknown ADF content dispatches to
`VisitRaw` or `VisitRawMark`.

```go
type linkCollector struct{ urls []string }
// … one method per kind; the compiler tells you which are missing …
func (c *linkCollector) VisitLink(l *ast.Link) struct{} {
    c.urls = append(c.urls, l.URL)
    return struct{}{}
}
```

For an ADF-side traversal that needs no per-kind dispatch, `adf.Walk`
iterates every node of a subtree as a Go iterator: `for n := range
adf.Walk(root)`. `adf.Transform(doc, f)` rewrites a document
copy-on-write. Per node, `f` returns a replacement and a handled flag,
which gives three actions: replace the node, erase it with an empty
slice, or prune a subtree from the rewrite by a return of the node
itself. The input document is never mutated. The `jira` transforms, the
issue-link → inlineCard rewrite and the bare-key expansion, are built on
`adf.Transform`.

An extension package chains the exhaustiveness. `dialect.Visitor[T]` with
`dialect.Visit` covers the typed dialect kinds. Call it from your
`VisitExtension`, and a kind that the dialect does not know continues to
its `VisitOther`, where a further extension visitor chains. An extension
author who ships custom kinds must follow the same escape-and-chain
shape.

## Development

Read [CONTRIBUTING.md](CONTRIBUTING.md) for the mise toolchain, the
hooks, the test commands, and the workflow of the fixture corpus and the
fuzz corpus. Releases are documented in
[docs/RELEASING.md](docs/RELEASING.md).
