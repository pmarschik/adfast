# adfast design notes

Internal design documentation. The README stays consumer-facing, and the
reasoning and the guarantees behind it live here.

## Why the pivot AST (and why not goldmark's)

Both conversion directions meet in one source-independent tree: the
adfast AST in the `ast` package, whose semantics mirror the mdast of
remark. The AST of goldmark cannot be that pivot. Its nodes reference
byte ranges of the parsed source, so a tree synthesized _from ADF_ has
no source to point at, and goldmark ships no markdown renderer. The
adfast AST is built once from the goldmark parse, and that build
normalizes the quirks of the parser. It is also what the
remark-compatible renderer consumes. The `adf` `Doc` and `Node` types
are the AST of the ADF side, and JSON happens only at the very edge. One
shared pivot is what makes round-trip idempotence testable: the encode
path and the decode path cannot drift apart semantically.

## Preservation guarantees and diagnostics

This is the preservation guarantee behind the table. The `adf` document
model is typed, with one Go type per known node kind and mark kind.
`adf.RawNode` and `adf.RawMark` are the lossless escape hatch for an
unknown kind, and an `Extra` map on every typed kind holds the
attributes the typed fields do not model. **JSON → `adf.DecodeDoc` →
JSON keeps unknown node types, unknown marks, and unmodeled attributes
losslessly.** A transform or a diff over ADF therefore never loses them.
The decode can report what it could not type: `adf.DecodeDocOpts` with a
diagnostics sink emits `unknown-node`, `unknown-mark`, and
`unknown-attr`.

The adf → md projection (`ToMarkdown(FromADF(doc))`) is where a
"preserved" kind becomes lossy. With `WithDiagnostics` the decode
reports each `adf.RawNode` it meets as a `raw-node` diagnostic. An
unknown **block** node recurses into its _first_ content child and
renders what it finds there, and the remaining children are dropped. An
unknown **inline** node is dropped. An unknown **mark** is dropped from
the projected text.

`RawNode`, `RawMark`, and `Extra` preservation survives **in-process**
only, along a decode → transform → re-encode path. Across a
markdown-only persistence boundary it is gone: render to markdown, throw
the tree away, re-parse later, re-encode, and anything without a
markdown mapping has disappeared. The "converted" mappings above exist
precisely so that ADF content survives that boundary. The remaining
lossiness across it is deliberate:

- An emoji whose document carries a `text` attribute degrades to that
  text, and the shortName and the id are gone.
- Overlapping annotation marks on one text run keep the outermost anchor
  only.
- An unmodeled `Extra` attribute on a converted kind does not ride
  along.
- The `localId` of a caption, a section, or a column regenerates
  server-side.
- A line break that a producer left inside a text node collapses to a
  single space.

That last one is symmetry rather than lossiness. ADF spells a line break
as `hardBreak`, and the soft break of markdown is a space, which is what
`FromMarkdown` already produces. A text node that carries the newlines
of a soft-wrapped source file therefore renders as one flowing
paragraph, wrapped at the configured print width instead of at the width
of the producer.

## Universal core, product-profile diagnostics

The conversion core is **product-neutral** by design. It must round-trip
any Atlassian ADF document, a Confluence page included, so it never
drops or rewrites a kind because some target product would not render
it. Product availability is enforced one layer out, as an authoring-side
**diagnostic** instead of a change to the conversion.
`convert.WithUnsupportedKinds(product, kinds)` is the generic mechanism.
After `ToADF` produces the document, it walks both the nodes and the
marks and emits one `unsupported-in-product` diagnostic per distinct
kind in the supplied set. The output is byte-identical with and without
the option. The split keeps the core universal and still lets each
product profile speak up.

The _data_ — which kinds — lives with the product modules, not with the
core. `jira.UnsupportedKinds` and `confluence.UnsupportedKinds` are
derived from `docs/adf-availability.json` and wired through the
`MarkdownOptions` of each module. Both sets are scoped to
RENDER-CONFIRMED non-support, where a live product render dropped the
kind or blocked it as unsupported content. Neither set holds
documentation-by-omission. A render probe on 2026-07-22 showed that Jira
renders most docs-omitted kinds first-class (layoutSection, cards,
status, and more), so docs-by-omission would produce false positives.
The confident sets are therefore small. `jira.UnsupportedKinds` is
`placeholder`, which Jira dropped, plus `multiBodiedExtension` and
`extensionFrame`, which the Jira REST endpoint rejects.
`confluence.UnsupportedKinds` is `blockTaskItem`, which Confluence
downgrades to a plain `taskItem`. A new kind needs a live-render
confirmation. The consumer chooses the severity. An encode-side
authoring tool can, for example, promote a Jira
`unsupported-in-product` to a blocking error before it pushes to Jira.

`fontSize` is a special case that appears in NEITHER set. The same probe
found that neither product supports the mark: Jira rejects it, and
Confluence strips it on save. adfast retires the mark entirely instead of
a per-product flag. The mark is never produced — the `:fontSize`
directive parses but drops to plain text on encode, and a legacy
`fontSize` ADF mark decodes to bare text — and a `fontsize-dropped`
diagnostic fires instead. An `unsupported-in-product` check for a kind
that can never be produced would be moot.

## Heading anchors: one markdown surface, a synthetic carrier, two lowerings

A heading anchor is the `## Title {#my-anchor}` surface, which pandoc and
remark-heading-id also use. This construct is the only one where the
markdown surface is universal but the ADF is not. **ADF has no
heading-anchor attribute.** Confluence spells an anchor as the anchor
macro. The macro is an `inlineExtension` with the `extensionKey`
`"anchor"`, and it sits inside the content of the heading. The unnamed
macro parameter is the name that links use as their URL fragment.

A live page gave this shape on 2026-08-21, through a read-only
measurement. The anchor is **not** `heading.attrs.localId`. adf-schema
documents that attribute as an optional UUID for node identity. It
renders to DOM as `data-local-id`, it creates no link target, and live
pages carry real UUIDs in it. Jira has no anchor construct at all.

Three layers therefore divide the work:

- The **root module** owns the markdown surface and a neutral carrier.
  The `{#id}` suffix cannot come from an addon, because `extension/`
  extends the _directive_ forms only. The parse strip and its exact
  render inverse live in `markdown/`, and `ast.Heading.ID` carries the id
  through the pivot AST. On the ADF side the id lands in
  `adf.Heading.Anchor`. This attribute is **synthetic and never-wire**,
  exactly as `ColwidthsHint` and the `tight` list attribute already are.
  It holds the anchor while the document is still product-neutral, and
  something must resolve it before submission.
- The **`confluence` module** lowers and lifts. `LowerAnchors` moves the
  attribute into an anchor-macro `inlineExtension` at the end of the
  heading content. `LiftAnchors` is the inverse. `MarkdownOptions` wires
  the first one as a document transform, and `RenderOptions` wires the
  second one as an ADF transform. A Confluence composition therefore
  never sees the synthetic attribute.
- The **`jira` module** drops. `jira.MarkdownOptions` wires
  `convert.WithoutHeadingAnchors("jira")`, which clears each anchor and
  emits one `heading-anchor-dropped` diagnostic per id. The
  `unsupported-in-product` diagnostic of the previous section leaves the
  output unchanged, but this one _does_ change it. The alternative is a
  document that the product rejects or silently mangles. The option
  itself is product-neutral, because the caller owns the label and the
  caller owns the judgement that the product has no anchors. The core
  therefore holds the diagnostics sink and no product knowledge.

The round trip adds two constraints. The first constraint is the narrow
id grammar. `ast.HeadingIDPattern` is the auto-id charset of pandoc:
alphanumerics, `-`, `_` and `.`, with an alphanumeric first character. A
`:` opens a text directive, and a `*`, a `` ` `` or a `[` opens an
inline span. An id with one of these characters cannot reach the renderer
as plain text. `ast.ValidHeadingID` is the single gate that the parser,
the renderer and the Confluence lift all read, which keeps the three
exact mirrors of each other.

The second constraint is that the lift must **decline** in two cases. It
never guesses. A heading with two anchor macros declines, and a name
outside the grammar declines. Such a heading keeps its macros and decodes
through the `:anchor[name]` directive sugar, which is lossless but less
pretty. The lift drops one parameter instead of a decline. That parameter
is `legacyAnchorId` (`"<PageTitle>-<name>"`), which comes from a page
title that the document does not carry and that Confluence regenerates.

`LiftAnchors` also needed a decode-side seam, and no such seam existed.
`WithDocTransforms` runs after the encode in `ToADF`, and
`WithASTTransforms` runs in the prettier mode of `ToMarkdown` only.
`WithADFTransforms` is the mirror of the first one. It applies caller
transforms to the `adf.Doc` before `FromADF` decodes it. The seam exists
for the product-specific shapes that a per-node decode hook cannot reach.
Such a shape moves content between a node and the attributes of the
parent.

## Rendering compatibility

The markdown renderer is measured against remark-stringify, and against
prettier for the formatter. A fixture corpus generated from those
reference implementations pins the escaping rules, the list marker
alternation, the wrapping, and the character-reference encoding. Format
pinning tests hold the rest. Round-trip idempotence of the md → adf → md
path (`ToMarkdown(FromADF(ToADF(FromMarkdown(md))))`) is enforced by a
continuously grown fuzz corpus. Inputs where the reference pipeline is
itself unstable are excluded as documented skip classes, each with a
probe input and an analysis next to the fuzz target.

### Deliberate divergences from remark-stringify

Parity with remark is the default. The exception is a construct where
the output of remark is itself unstable or lossy and adfast has to
preserve the construct. A named regression test pins each of these:

- **A text directive that stays open gets an empty attribute block.**
  The bare `:name` form ends in a name rune, and the labelled `:name[l]`
  form ends at the position where a `{…}` block starts. Whatever follows
  therefore either fuses into the directive token or, for an emphasis
  marker, cannot flank. remark repairs a non-flankable marker by a
  hex-encode of the rune next to it, and that repair cannot reach into a
  directive name without a rename of the directive. `:name{}` is
  semantically inert for every registered kind, terminates the token,
  and ends the form in punctuation. `*:media!*` therefore renders as
  `:media{}_!_` rather than as the unstable `:media_!_` of remark. The
  formatter needs the block in more places than the renderer does. It
  adds no escape of its own and writes back the source form the parse
  captured, so a `[` or a `_` that the source left bare fuses onto the
  name, where a re-derived escape would have separated it. See
  `markdown.needsPunctTrail`, `flanking_directive_test.go` and
  `format_contract_test.go`.
- **A run-on block inside a list item is blank-separated.** A GFM table
  runs to the first blank line, and a paragraph does too. A table cannot
  interrupt a paragraph, so an attached table donates its header row to
  the paragraph and pushes its own header into the body. A nested list
  that ends in a paragraph leaves the next block on the content column
  of the outer item as a lazy continuation. remark emits no blank line
  in any of these cases and merges the blocks silently: `- - x` plus `y`
  becomes the one paragraph `x y`. adfast forces the blank line when the
  following block is a paragraph or a table, the only two blocks that
  can be absorbed. See `markdown.blockRunsToBlankLine` and
  `list_nesting_test.go`. The one re-pinned entry in
  `testdata/directive_fixtures.json` (`- before … after list`) records
  this against the reference corpus.
- **A backslash in a directive label is escaped where it could start an
  escape sequence.** remark writes label text verbatim, so the alt text
  `\!0` round-trips as `!0`, because the re-parse consumes the
  backslash. A trailing backslash is worse: it escapes the `]` that
  closes the label. Both are escaped. A backslash before a
  non-punctuation byte stays verbatim, which keeps remark parity where
  parity is safe. See `markdown.escapeDirectiveLabel` and
  `flanking_directive_test.go`.
- **A colon in a directive label is escaped whenever it could open a
  nested text directive.** Label content is parsed as inline markdown
  and read back with `ast.PlainText`, which has no text for a directive
  node, so a nested directive does not degrade. Instead the label
  content disappears: `:placeholder[:0:0]` rendered `:placeholder[:0]`,
  which re-parsed to an empty document. The prose escaper protects
  letter-led names only, and only after the first character, for remark
  parity. Inside a label the rule also covers digit-led names, which
  goldmark-directive parses, and the first character of the label. A
  colon that cannot start a name, and a colon that follows another
  colon, still stay verbatim. See `markdown.writeColonEscapePrefix` and
  `flanking_directive_test.go`.
- **Two adjacent code spans are written as one.** Markdown cannot
  separate them. To the parser, the closing fence of the first span and
  the opening fence of the second are one backtick run, and no fence
  length and no padding splits that run. Take two adjacent code-marked
  text nodes that hold "0" each. remark writes six bytes: a one-backtick
  fence around "0", two backticks, "0", and a one-backtick fence. Those
  re-parse as the single span that holds "0", two backticks, "0". Those
  are also the bytes remark writes for that one span, so its output is
  ambiguous as well as lossy. adfast joins the content instead, into one
  span that holds "00". That is the only representable form, and it is
  the one ADF agrees with, because adjacent content under equal marks is
  one run. The adjacency is reachable because the code mark is
  exclusive: an emphasis that wraps nothing but a code span drops in
  normalization and leaves its span beside the neighbor. See
  `markdown.joinAdjacentCodeSpans` and `format_contract_test.go`. The
  re-pinned two-code-node entry in
  `testdata/directive_fixtures.json` records this against the reference
  corpus.
- **A text-directive label that opens on the 4-column indent starts with
  a character reference.** The label of a text directive is parsed as
  block content. A leading whitespace run that reaches four columns is
  therefore an indented code block. Four spaces reach it, and so does a
  tab, which advances to the next stop. Nothing inside is parsed, each
  escape stays literal, and every format escapes the surviving backslash
  again, so `:u[    \*]` grew a backslash per pass. A write of the first
  byte of the run as `&#x20;` or `&#x9;` keeps the label off the indent.
  The reference is one column wide and decodes back to the byte. A leaf
  label and a container label need no repair: they are read back through
  `ast.PlainText` over inline content, which resolves the escape. See
  `markdown.escapeLabelIndent` and `format_contract_test.go`.
- **A whitespace-only code span is written without a pad.** A code span
  whose content begins and ends with a space normally needs a pad space
  at each end, because the parser trims one from each edge. goldmark
  skips that trim when the content is blank, and its blank test counts a
  tab and a carriage return as whitespace, where the rule of CommonMark
  asks only whether the content is all U+0020. So `` ` \t ` `` keeps its
  spaces on re-parse, and the padded form that remark would write grows
  a space per format pass. adfast pads against the parser it
  round-trips against. See `markdown.codeSpanTrims` and
  `format_contract_test.go`.
- **The formatter escapes an '@' that its own output would linkify.**
  The pre-CommonMark parser of prettier has no GFM autolink literals, so
  it leaves every '@' bare, and the formatter follows it. But adfast
  re-parses with the linkify extension of goldmark. Where the source
  kept the address apart, the formatter can write it contiguously: in
  `0@A:u.A` the empty `:u` normalizes away and leaves "0@A" beside ".A".
  The re-parse then turns the run into a link the source never had.
  adfast escapes there, and it mirrors the linkify conditions of
  goldmark rather than the one-character neighborhood of remark. A
  literal starts at a line head or on one of the trigger bytes
  `" *_~("`, never on punctuation, and it needs a dot in its domain. Two
  positions are answered conservatively, where the escape only ever
  keeps plain text plain: a local part that runs out of the text node
  into the markup before it, and an emphasis closer, which is an
  underscore and therefore an address byte to the scan. The equivalent
  fusion of a URL or a `www` literal has no repair, because
  `relinkifyTexts` re-linkifies a decoded text value whatever escapes it
  was written with. An unlinked URL literal is therefore inexpressible
  in the dialect and stays a documented fuzz skip class. See
  `markdown.linkifiesAsEmail`, `convert.joinTextAtoms` and
  `format_contract_test.go`.
- **A re-linked URL literal ends where the parser would end it.** The
  linkify of goldmark skips a literal while it is inside a potential
  link label (`[ http://…`), and an escape can split one out of the
  parse (`http:\//0.a#!`). remark links both, and so does the re-parse
  of the rendered output by adfast, so `relinkifyTexts` links them at
  parse time to keep the round trip stable. That repair has to land on
  the boundary of the parser rather than on the boundary of its regexp.
  goldmark strips a trailing `.`, an unbalanced `)` run and an
  entity-closing `;` from the match, and then a trailing run of
  `?!.,:*_~`, so `http://0.a#!` links only through the `#`. Without the
  trim, the parse claimed a longer link than the render could write
  back, and the format changed the href. See
  `markdown.trimURLLiteralEnd` and `format_contract_test.go`.

The prettier md → md formatter is the composition
`ToMarkdown(FromMarkdown(md, WithPrettierFormat()), WithPrettierFormat())`
— a pure md → ast → md pass with no ADF leg. `FromMarkdown` produces the
faithful parse AST, and the format mode of `ToMarkdown` runs
`convert.Normalize` over it before it renders. `FromMarkdown` is a
_single_ parse for both directions. `ast.Text.Value` is always fully
decoded, because that is the ADF currency, and the literal escapes of
prettier are captured separately on `ast.Text.Raw` as escape provenance,
keyed by `markdown.PreservedEscapes`. The formatter reads that
provenance on the render side, so the escapes survive byte-for-byte
without a second parse mode. `WithPrettierFormat` now has NO parse-side
effect at all: the same `FrontmatterProvider` splits the frontmatter in
both directions, so detection cannot diverge, and the flag is read on
the render call only. Tests enforce its contract instead of structure.
The two obligations are semantic coherence, where a parse of the
formatted output yields the same ADF as a parse of the original, and
idempotence. Both run over the fixture corpus and as the
`FuzzFormatSemanticsPreserved` target, which has its own documented skip
classes.

### A single normalized AST

Canonicalization of the pivot AST lives in one place: `convert.Normalize`,
the shared AST → AST pass in `convert/normalize.go`. `FromMarkdown`, and
the lower-level `markdown.Parse`, returns the _faithful_ parse tree with
no canonicalization. An advanced consumer can therefore still see the
un-normalized pivot AST, which preserves its "remark-faithful,
source-independent" property. The `To*` primitives are what canonicalize
on the way out:

- `ToADF` is a pure structural projection of the tree it is given onto
  the data model of ADF. ADF has no nested inline marks. A projection of
  the nested `strong`, `em`, and `delete` wrappers onto flat per-text
  mark arrays therefore canonicalizes the inline marks as an _inherent
  side effect of the projection_, and there is no separate regroup step
  on the encode side. Blank-line semantics and table span clamping and
  padding fall out of the ADF shape the same way. Two constructs are
  genuinely cross-sibling and the pivot AST has no dedicated node for
  them: `::colwidths` resolving onto the following table, and
  `::decisions` marking the following bullet list. Both are interpreted
  here, structurally.
- `ToADF` then always applies `adf.NormalizeTextNewlines`, the
  spec-level whitespace normalization. Inside a non-code text node, a
  newline run with its surrounding spaces and tabs collapses to one
  space, and so does a space run. The same collapse applies **across the
  junction of two adjacent text nodes with equal marks**, because
  markdown writes those nodes contiguously: the run would exist in the
  ADF only, and a re-parse of the render merges the nodes and shortens
  it. Such a junction is what an inline node that converts to nothing
  leaves behind. An empty link (`x []() y`) leaves one, and so does a
  registered text directive with no content (`:u` in `*0aaa[0 :u ]*`).
  Without the junction rule the md → adf → md round trip is not
  idempotent, so this is a correctness requirement, not a cosmetic one.
- The prettier-format mode of `ToMarkdown` runs `convert.Normalize`
  before it renders. The renderer needs the _nested_ AST while ADF is
  flat, so `Normalize` performs the inverse of the encode flatten. It
  collects the mark set of each atom and regroups the run into the
  canonical `strong`/`em`/`delete` nesting, re-derives the canonical
  payloads of the dialect kinds, and resolves the same
  `::colwidths` and `::decisions` cross-sibling patterns. `Normalize` is
  idempotent.

The flat → nested mark regrouping is the one canonicalization that
genuinely recurred. The ADF decode (`FromADF`) needs it, because ADF
marks are flat, and so does `Normalize`. It now has a **single
implementation**: `groupSpans` and `inferAcrossCode` in
`convert/spanning.go`, parameterized over the per-caller flat-item type
through `spanOps`. Both the decode path and `Normalize` call it, and the
former second copy in the decode path is gone. The two cross-sibling
matchers are likewise single functions that the encode projection and
`Normalize` share: `decisionTargetList` is the following-plain-bullet-list
test, and `resolveColwidthTargets` is the directive-then-table
match-or-orphan loop.

Scope of the claim. `ToADF` is deliberately _not_ routed through
`Normalize`. The identity `ToADF(Normalize(n)) == ToADF(n)` holds for a
parsed tree under invertible options, but not in general. `Normalize`
canonicalizes a smart-link card to its short display label, which is
what reads well in markdown. A `SmartLinks` configuration with
`KeyFromURL` but no `URLForKey` — the common Jira setup — cannot expand
that label back to the URL the ADF card needs, so a route of the encoder
through `Normalize` would corrupt the card URL. The one parallel that
remains is the canonical-payload re-derivation of the dialect kinds. On
the pivot AST, `Normalize` mirrors what the `EncodeADF` methods of the
dialect plus the ADF decode compute. To collapse that into a single
implementation, `Normalize` has to run the actual ADF round trip
(`FromADF(ToADF(n))`). That changes the semantics the round trip is
intentionally lossy about — regenerated `localId`s, foreign-extension
pass-through, and the card label and URL asymmetry above — and it breaks
the byte-level fixture oracle. The mirror is kept instead. The
semantic-coherence test remains the tripwire that guards the
`ToADF`-invariance property.

## Visitor dispatch

The exhaustive visitors (`ast.Visitor`, `adf.Visitor`/`MarkVisitor`,
`dialect.Visitor`) are also how the converters dispatch internally. A
new node kind therefore forces every conversion direction to take a
position at compile time. The in-package `Visit` switches are the single
maintained dispatch points, and their branch count equals the kind count
by design. cyclop is excluded by configuration for exactly those files.

## Package layout

The module is layered into public subpackages along the pipeline stages,
and the root package is a thin facade that composes them. The README
carries the short table, and this is the full version:

- **`adfast`** (root) — the facade: the four pivot-AST primitives
  `FromMarkdown` (parse), `FromADF` (decode), `ToADF` (encode), and
  `ToMarkdown` (render), plus the `Pipeline` and the shared option set.
  Start here.
- **`adf/`** — the typed ADF document model: one Go type per known node
  kind and mark kind, plus the lossless `RawNode`/`RawMark` escape hatch
  and the per-node `Extra` maps, with the JSON codec (decode,
  diagnostics, and a semantically identical re-encode) and the tree
  helpers. For a document-transform author.
- **`ast/`** — the pivot Markdown AST, shaped like remark mdast, that
  both conversion directions share: one typed node kind per remark node,
  pointer-constructed and `Kind()`-tagged. For a custom-pipeline
  builder.
- **`extension/`** — the public AST extension contract: `Node`, the
  three context interfaces, and `Registration`. It covers all four
  pipeline paths, and there are no capability fragments.
- **`dialect/`** — the known directive dialect as typed AST nodes on the
  extension contract: `Panel`, `Expand`, `Media`, `JQL`, `LinkCard`,
  `LinkEmbed`, `Colwidths`, `Mention`, `Status`, `MediaInline`, `Date`,
  `Placeholder`, `Emoji`, the extension family, synced blocks, page
  layouts, and the mark kinds. It is wired as the default set.
- **`markdown/`** — the text edge: the goldmark parser assembly
  (dialect, typed directive nodes) plus the remark-compatible renderer
  (`Parse`, `Render`, `NewParser`).
- **`convert/`** — the AST ⇄ ADF transforms (`ToADF`, `FromADF`), the
  shared `Normalize` canonicalization pass (`normalize.go`, which the
  formatter mode of `ToMarkdown` uses), and their parameter types
  (`SmartLinks`, `MediaAsset`, the resolvers, `Diagnostic`).
- **`assets/`** — the pluggable attachment store behind the media
  resolvers. See [Asset store internals](#asset-store-internals).
- **`debug/`** — human-readable tree dumps of both ASTs (`Dump`,
  `DumpADF`) and a type-tagged JSON encoding of the Markdown AST
  (`MarshalJSON`). It is a debugging aid only, and the compatibility
  guarantees do not cover its output format.
- **`jira/`** (separate module) — the Jira link conventions:
  `MarkdownOptions`, `RenderOptions`, and the issue-key document
  transforms.
- **`frontmatter/`** (separate module) — optional YAML frontmatter
  access. The core is YAML-neutral by design: the `FrontmatterProvider`
  treats the front block as opaque bytes and keeps it verbatim on
  `ast.Frontmatter.Value`, delimiters included. This module is the seam
  for a consumer who wants a `map[string]any` view of that block. It
  offers `Parse`, `Render`, `Patch`, and `Replace`, the `Get`, `Set`,
  and `Remove` dot-path helpers, `KeyOrder`, and a CST-based
  `PatchPreserving` that retains the key order, the comments, and the
  scalar styles. It only turns the raw block into a map and back.
  Boundary detection stays with the provider. A separate module also
  means the root never depends on a YAML implementation.

## Asset store internals

The consumer-facing surface is in the README, and the mechanics live
here.

### On-disk layout

`FSStore` keeps a free-form `assets/` folder next to the markdown files.
Downloaded attachment content is content-addressed under a hidden
`assets/.store/` directory: a friendly name symlinks to the store, and
`index.json` maps the media ids. A rename of a friendly file is safe,
because a lookup falls back to content hashing. A plain file also works
without a symlink, and `Add(mediaID, name, content)`, the
download-direction verb, adopts an existing identical file instead of a
duplicate.

Where the assets folder physically lives is a concern of the store, not
of the caller. `NewFSStore(mdDir)` keeps it next to the documents.
`NewFSStoreAt(assetsParent, docDir)` separates the physical location
from the documents: a repository-root folder shared by nested documents
(pair it with `DiscoverRoot(docDir, ".git")` for anchor-file discovery),
an XDG data directory, or anything else. Reference paths in the markdown
are computed relative to each document (`../../assets/shot.png`), and
every implementation of `Store` is free to choose its own scheme.

### Concurrency and atomicity

Every document next to one `assets/` folder shares the store, and
several store instances over the same folder cooperate. A mutation
reloads and merges the on-disk index and replaces it atomically, so
nothing gets clobbered across subsystems. Across processes the same
holds, and the last write wins per media id there.

### Layered and split stores

Stores compose. `assets.Layered(local, shared)` consults each layer in
order: a read and a per-file operation route to the layer that owns the
file, a download lands in the first layer, and `Pending` is the ordered
union. A document-local folder can therefore sit over a shared
project-root one.

The true store and the nice one can also split.
`NewFSStoreSplit(blobParent, docDir)` keeps the content-addressed blobs
and the index shared under `blobParent`, deduplicated across every
document, while the friendly files, and the reference paths, stay next to
each document. A view that never downloaded an asset materializes its
friendly file from the shared blobs on `Resolve`. An upload deduplicates
by content too: an identical file goes up once, and every duplicate path
resolves to the same media id through the content-addressed `Lookup`.

### Container scoping

Deduplication has a product boundary, and the store models it. Jira
binds an attachment to one issue, and Confluence binds it to one page. A
media id minted for another container renders broken there
([JRACLOUD-92725](https://jira.atlassian.com/browse/JRACLOUD-92725)).
`assets.ForScope(store, "PROJ-123")` binds a view to one such container.
A lookup then returns only the ids of that scope, and a legacy unscoped
id still matches. `Pending` re-lists content that is attached elsewhere
only, and a new association records the scope. The same shared file
pushed from two issues therefore uploads once PER ISSUE, and each
document encodes its own id, while local storage stays fully
deduplicated: one blob, one friendly file, many scoped ids. Encode every
document with the view of ITS container. Batch across documents inside
one scope only.

### Upload flow

`Uploader` is the pluggable media-management seam. Implement it against
the API of your product (Jira attachments, Confluence media, and so on).
The whole pending worklist arrives in ONE `Uploader` call, so an
implementation can fold it into a bulk request. A partial batch keeps
its progress: a success associates, and a failure stays pending for the
next sync.

- `assets.Sync(ctx, store, uploader)` — lazy. Nothing uploads until you
  sync.
- `assets.EnsureUploaded(ctx, store, uploader)` — syncs first and
  returns the wired markdown options, so the encode cannot observe an
  un-uploaded asset. Keep a pure conversion (a diff, a preview) on
  `MarkdownOptions` alone, so that it never touches the network.
- `assets.PushPipeline(ctx, store, uploader)` — an `adfast.Pipeline`
  that wires `MarkdownOptions` plus `assets.SyncOnEncode` as a
  `WithBeforeEncode` hook. The conversion itself triggers the upload,
  and the hook receives the parsed ASTs of every document in the call
  before anything encodes. With `Pipeline.MarkdownToADFAll`, every
  pending asset that any document references goes up in a single batch
  before all of them encode.

An unreferenced file stays pending, so a scratch file in `assets/` is
never uploaded behind your back. `Pipeline.MarkdownToADF` is infallible,
so it downgrades a hook failure to a `before-encode-failed` diagnostic.
`Pipeline.MarkdownToADFAll` returns that failure as an error.

The lower-level verbs remain for a custom flow: `Pending()` lists the
worklist, `Load` reads the bytes, and `Associate(id, path)` binds an
existing file in place.

### Reference rewriting

When the store layout changes — a local folder becomes a shared root, or
a fused store becomes a split one — the markdown rewrites through the
formatter. `assets.RewriteReferences(old, new)` re-paths the image
destinations as an AST transform, wired through `WithASTTransforms` and
run on the tree of the formatter between `convert.Normalize` and the
render, while the formatting pass keeps every other byte:

```go
out := adfast.ToMarkdown(
    adfast.FromMarkdown(md, adfast.WithPrettierFormat(), adfast.WithPrintWidth(width)),
    adfast.WithPrettierFormat(), adfast.WithPrintWidth(width),
    assets.RewriteReferences(oldStore, newStore),
)
```

The canonical conversion needs no facility at all. A render with
`RenderOptions(store)` always emits the current reference paths of the
store.
