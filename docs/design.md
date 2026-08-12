# adfast design notes

Internal design documentation — the README stays consumer-facing; the
reasoning and guarantees behind it live here.

## Why the pivot AST (and why not goldmark's)

Both conversion directions meet in one source-independent tree — the
adfast AST (`ast` package), whose semantics mirror remark's mdast.
goldmark's own AST cannot be that pivot: its nodes reference byte ranges
of the parsed source, so a tree synthesized _from ADF_ has no source to
point at, and goldmark ships no markdown renderer. The adfast AST is
built once from the goldmark parse (normalizing parser quirks in the
process) and is what the remark-compatible renderer consumes; the ADF
`Doc`/`Node` types are the ADF-side AST — JSON only happens at the very
edge. Sharing one pivot is what makes round-trip idempotence testable:
the encode and decode paths cannot drift apart semantically.

## Preservation guarantees and diagnostics

The preservation guarantee behind the table: the `adf` document model is
typed — one Go type per known node/mark kind — with
`adf.RawNode`/`adf.RawMark` as the lossless escape hatch for unknown
kinds, and an `Extra` map on every typed kind for attributes the typed
fields do not model. **JSON → `adf.DecodeDoc` → JSON preserves unknown
node types, unknown marks, and unmodeled attributes losslessly** —
transforms and diffing over ADF never lose them. Decoding can report
what it could not type: `adf.DecodeDocOpts` with a diagnostics sink
emits `unknown-node`, `unknown-mark`, and `unknown-attr`. The adf→md
projection (`ToMarkdown(FromADF(doc))`) is where "preserved" kinds get
lossy; with `WithDiagnostics` the decode reports each `adf.RawNode` it
meets as a `raw-node` diagnostic: an unknown **block** node recurses into its
_first_ content child and renders what it finds there (remaining
children are dropped), an unknown **inline** node is dropped, and
unknown **marks** are dropped from the projected text.

Note that `RawNode`/`RawMark`/`Extra` preservation only survives
**in-process** (decode → transform → re-encode): across a markdown-only
persistence boundary — render to markdown, throw the tree away, later
re-parse and re-encode — anything without a markdown mapping is gone.
The "converted" mappings above exist precisely so ADF content survives
that boundary. Deliberate remaining lossiness across it: emojis whose
documents carry a `text` attribute degrade to that text (shortName/id
gone), overlapping annotation marks on one text run keep only the
outermost anchor, unmodeled `Extra` attributes on converted kinds do
not ride along, caption/section/column `localId`s regenerate
server-side, and a line break a producer left inside a text node
collapses to a single space.

That last one is symmetry rather than lossiness: ADF spells a line break
as `hardBreak`, and markdown's soft break is a space, which is what
`FromMarkdown` already produces — so a text node carrying the newlines
of a soft-wrapped source file renders as one flowing paragraph, wrapped
at the configured print width instead of the producer's.

## Universal core, product-profile diagnostics

The conversion core is deliberately **product-neutral**: it must
round-trip any Atlassian ADF document (a Confluence page included), so it
never drops or rewrites a kind just because some target product would not
render it. Product availability is therefore enforced one layer out, as
an authoring-side **diagnostic** rather than a conversion change.
`convert.WithUnsupportedKinds(product, kinds)` is the generic mechanism:
after `ToADF` produces the document it walks both nodes and marks and
emits one `unsupported-in-product` diagnostic per distinct kind in the
supplied set. The output is byte-identical with and without the option —
the split keeps the core universal while letting each product profile
speak up.

The _data_ — which kinds — lives with the product modules, not the core:
`jira.UnsupportedKinds` and `confluence.UnsupportedKinds` are derived
from `docs/adf-availability.json` and wired through each module's
`MarkdownOptions`. Both sets are scoped to RENDER-CONFIRMED non-support
(a live product render dropped or unsupported-blocked the kind), not
documentation-by-omission — a 2026-07-22 render probe showed Jira renders
most docs-omitted kinds first-class (layoutSection, cards, status, …), so
docs-by-omission would false-positive. The confident sets are therefore
small: `jira.UnsupportedKinds` = `placeholder` (Jira dropped it) plus
`multiBodiedExtension`/`extensionFrame` (Jira REST rejects them), and
`confluence.UnsupportedKinds` = `blockTaskItem` (Confluence downgrades it
to a plain `taskItem`). Adding a kind requires a live-render confirmation.
The consumer chooses severity — an encode-side authoring tool can, for
instance, promote a Jira `unsupported-in-product` to a blocking error
before it pushes to Jira.

`fontSize` is a special case that appears in NEITHER set: the same probe
found neither product supports the mark (Jira rejects it, Confluence
strips it on save). Rather than flag it per-product, adfast retires the
mark entirely — it is never produced (the `:fontSize` directive parses but
drops to plain text on encode, and a legacy `fontSize` ADF mark decodes to
bare text), emitting a `fontsize-dropped` diagnostic instead. An
`unsupported-in-product` check for a kind that can never be produced would
be moot.

## Rendering compatibility

The markdown renderer is measured against remark-stringify (and, for
the formatter, prettier): escaping rules, list marker alternation,
wrapping, and character-reference encoding are pinned by a fixture
corpus generated from those reference implementations, plus format
pinning tests. Round-trip idempotence of the md → adf → md path
(`ToMarkdown(FromADF(ToADF(FromMarkdown(md))))`) is enforced by a
continuously grown fuzz corpus; inputs where the reference pipeline is
itself unstable are excluded as documented skip classes (each with a
probe input and analysis next to the fuzz target).

The prettier md → md formatter is the composition
`ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`
— a pure md → ast → md pass with no ADF leg: `FromMarkdown` produces the
faithful parse AST, and `ToMarkdown`'s format mode runs `convert.Normalize`
over it before rendering. `FromMarkdown` is a _single_ parse for both
directions: `ast.Text.Value` is always fully decoded (the ADF currency)
and prettier's literal escapes are captured separately on `ast.Text.Raw`
(escape provenance keyed by `markdown.PreservedEscapes`), which the
formatter reads on the render side so escapes survive byte-for-byte
without a second parse mode. `WithPrettierFormat` now has NO parse-side
effect at all: frontmatter is split by the same `FrontmatterProvider` in
both directions, so detection cannot diverge and the flag is read only on
the render call. Its contract is enforced by tests instead of
structure — semantic coherence (parsing the formatted output yields the
same ADF as parsing the original) and idempotence, over the fixture
corpus and as the `FuzzFormatSemanticsPreserved` target with its own
documented skip classes.

### A single normalized AST

Canonicalization of the pivot AST lives in one place: `convert.Normalize`,
the shared AST → AST pass in `convert/normalize.go`. `FromMarkdown` (and
the lower-level `markdown.Parse`) returns the _faithful_ parse tree — no
canonicalization — so advanced consumers can still see the un-normalized
pivot AST, preserving its "remark-faithful, source-independent" property.
The `To*` primitives are what canonicalize on the way out:

- `ToADF` is a pure structural projection of the tree it is given onto
  ADF's data model. ADF has no nested inline marks, so projecting the
  nested `strong`/`em`/`delete` wrappers onto flat per-text mark arrays
  canonicalizes inline marks as an _inherent side effect of the
  projection_ — there is no separate regroup step on the encode side.
  Blank-line semantics and table span clamping/padding fall out of the
  ADF shape the same way. The two genuinely cross-sibling constructs the
  pivot AST has no dedicated node for — `::colwidths` resolving onto the
  following table, `::decisions` marking the following bullet list — are
  interpreted here structurally.
- `ToMarkdown`'s prettier-format mode runs `convert.Normalize` before
  rendering. Because the renderer needs the _nested_ AST while ADF is
  flat, `Normalize` performs the inverse of the encode flatten: it
  collects each atom's mark set and regroups the run into canonical
  `strong`/`em`/`delete` nesting, re-derives the dialect kinds' canonical
  payloads, and resolves the same `::colwidths`/`::decisions`
  cross-sibling patterns. `Normalize` is idempotent.

The flat → nested mark regrouping is the one canonicalization that
genuinely recurred: the ADF decode (`FromADF`) needs it (ADF marks are
flat) and so does `Normalize`. It now has a **single implementation** —
`groupSpans`/`inferAcrossCode` in `convert/spanning.go`, parameterized
over the per-caller flat-item type through `spanOps` — that both the
decode path and `Normalize` call; the former second copy in the decode
path is gone. The two cross-sibling matchers are likewise single
functions shared by the encode projection and `Normalize`:
`decisionTargetList` (the following-plain-bullet-list test) and
`resolveColwidthTargets` (the directive-then-table match-or-orphan loop).

Scope of the claim. `ToADF` is deliberately _not_ routed through
`Normalize`: the identity `ToADF(Normalize(n)) == ToADF(n)` holds for
parsed trees under invertible options but not in general. `Normalize`
canonicalizes smart-link cards to their short display label (what reads
well in markdown), and a `SmartLinks` config with `KeyFromURL` but no
`URLForKey` (the common Jira setup) cannot expand that label back to the
URL the ADF card needs — so routing the encoder through `Normalize` would
corrupt the card URL. The one parallel that remains is the dialect kinds'
canonical-payload re-derivation: `Normalize` mirrors, on the pivot AST,
what the dialect `EncodeADF` methods plus the ADF decode compute.
Collapsing that to a single implementation would require `Normalize` to
run the actual ADF round trip (`FromADF(ToADF(n))`), which changes the
semantics the round trip is intentionally lossy about (regenerated
`localId`s, foreign-extension pass-through, the card label/URL asymmetry
above) and would break the byte-level fixture oracle; it is kept as a
mirror. The semantic-coherence test remains the tripwire that guards the
`ToADF`-invariance property.

## Visitor dispatch

The exhaustive visitors (`ast.Visitor`, `adf.Visitor`/`MarkVisitor`,
`dialect.Visitor`) are also how the converters dispatch internally, so
adding a node kind forces every conversion direction to take a position
at compile time. The in-package `Visit` switches are the single
maintained dispatch points; their branch count equals the kind count by
design (cyclop excluded by config for exactly those files).

## Package layout

The module is layered into public subpackages along the pipeline stages;
the root package is a thin facade composing them (the README carries the
short table, this is the full version):

- **`adfast`** (root) — the facade: the four pivot-AST primitives
  `FromMarkdown` (parse), `FromADF` (decode), `ToADF` (encode), and
  `ToMarkdown` (render), the `Pipeline`, and the shared option set. Start
  here.
- **`adf/`** — the typed ADF document model: one Go type per known
  node/mark kind plus the lossless `RawNode`/`RawMark` escape hatch and
  per-node `Extra` maps, with the JSON codec (decode + diagnostics,
  semantically identical re-encoding) and tree helpers; for
  document-transform authors.
- **`ast/`** — the pivot Markdown AST (remark-mdast-shaped) both
  conversion directions share: one typed node kind per remark node
  (pointer-constructed, `Kind()`-tagged); for custom-pipeline builders.
- **`extension/`** — the public AST extension contract: `Node`, the
  three context interfaces, and `Registration` (all four pipeline paths,
  no capability fragments).
- **`dialect/`** — the known directive dialect as typed AST nodes on the
  extension contract (`Panel`, `Expand`, `Media`, `JQL`, `LinkCard`,
  `LinkEmbed`, `Colwidths`, `Mention`, `Status`, `MediaInline`, `Date`,
  `Placeholder`, `Emoji`, the extension family, synced blocks, page
  layouts, and the mark kinds); wired as the default set.
- **`markdown/`** — the text edge: goldmark parser assembly (dialect,
  typed directive nodes) plus the remark-compatible renderer (`Parse`,
  `Render`, `NewParser`).
- **`convert/`** — the AST ⇄ ADF transforms (`ToADF`, `FromADF`), the
  shared `Normalize` canonicalization pass (`normalize.go`, used by the
  formatter mode of `ToMarkdown`), and their parameter types
  (`SmartLinks`, `MediaAsset`, resolvers, `Diagnostic`).
- **`assets/`** — the pluggable attachment store behind the media
  resolvers; see [Asset store internals](#asset-store-internals).
- **`debug/`** — human-readable tree dumps of both ASTs (`Dump`,
  `DumpADF`) and a type-tagged JSON encoding of the Markdown AST
  (`MarshalJSON`); debugging aid only, output format not covered by
  compatibility guarantees.
- **`jira/`** (separate module) — the Jira link conventions:
  `MarkdownOptions`, `RenderOptions`, and the issue-key document
  transforms.
- **`frontmatter/`** (separate module) — optional YAML frontmatter
  access. The core is deliberately YAML-neutral: the `FrontmatterProvider`
  treats the front block as opaque bytes and keeps it verbatim on
  `ast.Frontmatter.Value` (delimiters included). This module is the seam
  for consumers who want a `map[string]any` view of that block —
  `Parse`/`Render`/`Patch`/`Replace`, the `Get`/`Set`/`Remove` dot-path
  helpers, `KeyOrder`, and a CST-based `PatchPreserving` that retains
  key order, comments, and scalar styles. It only turns the raw block ⇄
  map; boundary detection stays with the provider. Keeping it a separate
  module means the root never depends on a YAML implementation.

## Asset store internals

The consumer-facing surface is in the README; the mechanics live here.

### On-disk layout

`FSStore` keeps a free-form `assets/` folder next to the markdown files,
with downloaded attachment content content-addressed under a hidden
`assets/.store/` directory: friendly names symlink to the store, and
`index.json` maps media ids. Renaming the friendly files is safe —
lookups fall back to content hashing. Plain files also work without
symlinks, and `Add(mediaID, name, content)` (the download-direction
verb) adopts existing identical files instead of duplicating them.

Where the assets folder physically lives is a store concern, not the
caller's: `NewFSStore(mdDir)` keeps it next to the documents, while
`NewFSStoreAt(assetsParent, docDir)` separates the physical location
from the documents — a repository-root folder shared by nested documents
(pair with `DiscoverRoot(docDir, ".git")` for anchor-file discovery), an
XDG data directory, or anything else. Reference paths in the markdown
are computed relative to each document (`../../assets/shot.png`), and
every implementation of `Store` is free to choose its own scheme.

### Concurrency and atomicity

All documents next to one `assets/` folder share the store, and multiple
store instances over the same folder cooperate: mutations reload-merge
the on-disk index and replace it atomically, so nothing gets clobbered
across subsystems (or processes — last write wins per media id there).

### Layered and split stores

Stores compose: `assets.Layered(local, shared)` consults each layer in
order — reads and per-file operations route to the layer owning the
file, downloads land in the first layer, and `Pending` is the ordered
union — so a document-local folder can sit over a shared project-root
one.

The true store and the nice one can also split:
`NewFSStoreSplit(blobParent, docDir)` keeps the content-addressed blobs
and the index shared under `blobParent` (deduplicated across every
document) while friendly files — and the reference paths — stay next to
each document. A view that never downloaded an asset materializes its
friendly file from the shared blobs on `Resolve`. Uploads deduplicate by
content too: identical files go up once, and every duplicate path
resolves to the same media id through the content-addressed `Lookup`.

### Container scoping

Deduplication has a product boundary, and the store models it: Jira
binds attachments to one issue and Confluence binds them to one page — a
media id minted for another container renders broken there
([JRACLOUD-92725](https://jira.atlassian.com/browse/JRACLOUD-92725)).
`assets.ForScope(store, "PROJ-123")` binds a view to one such container:
lookups only return ids of that scope (legacy unscoped ids still match),
`Pending` re-lists content that is only attached elsewhere, and new
associations record the scope. The same shared file pushed from two
issues therefore uploads once PER ISSUE and each document encodes its
own id, while local storage stays fully deduplicated — one blob, one
friendly file, many scoped ids. Encode every document with the view of
ITS container; batch across documents only within one scope.

### Upload flow

`Uploader` is the pluggable media-management seam — implement it against
your product's API (Jira attachments, Confluence media, …). The whole
pending worklist arrives in ONE `Uploader` call so implementations can
fold it into a bulk request; a partial batch keeps its progress
(successes associate, failures stay pending for the next sync).

- `assets.Sync(ctx, store, uploader)` — lazy: nothing uploads until you
  sync.
- `assets.EnsureUploaded(ctx, store, uploader)` — syncs first and
  returns the wired markdown options, so encoding cannot observe an
  un-uploaded asset. Keep pure conversions (diffs, previews) on
  `MarkdownOptions` alone so they never touch the network.
- `assets.PushPipeline(ctx, store, uploader)` — an `adfast.Pipeline`
  wiring `MarkdownOptions` plus `assets.SyncOnEncode` as a
  `WithBeforeEncode` hook: the conversion itself triggers the upload (the
  hook receives the parsed ASTs of all documents in a call, before
  anything encodes). With `Pipeline.MarkdownToADFAll`, every pending
  asset referenced by any document goes up in a single batch before all
  of them encode.

Unreferenced files stay pending — a scratch file in `assets/` is never
uploaded behind your back. Since `Pipeline.MarkdownToADF` is infallible,
it downgrades a hook failure to a `before-encode-failed` diagnostic;
`Pipeline.MarkdownToADFAll` returns it as an error.

The lower-level verbs remain for custom flows: `Pending()` lists the
worklist, `Load` reads the bytes, and `Associate(id, path)` binds an
existing file in place.

### Reference rewriting

When the store layout changes (local folder → shared root, fused →
split), the markdown rewrites through the formatter —
`assets.RewriteReferences(old, new)` re-paths image destinations (an
AST transform wired via `WithASTTransforms`, run on the formatter's tree
between `convert.Normalize` and rendering) while the formatting pass
keeps every other byte:

```go
out := adfast.ToMarkdown(
    adfast.FromMarkdown(md, adfast.WithPrettierFormat(), adfast.WithPrintWidth(width)),
    adfast.WithPrettierFormat(), adfast.WithPrintWidth(width),
    assets.RewriteReferences(oldStore, newStore),
)
```

The canonical conversion needs no facility at all: rendering with
`RenderOptions(store)` always emits the store's current reference paths.
