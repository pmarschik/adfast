## [v0.8.0] - 2026-10-30

### 🚀 Features

- _(core)_ Carry GFM table column alignment through the ADF route
- _(core)_ Parse GFM footnotes and flatten them for ADF
- _(confluence)_ Repair what Confluence's ADF page read loses
- _(adf)_ Carry table column alignment on the ADF alignment mark
- _(assets)_ [**breaking**] Key index.json by content hash and carry embedder metadata
- _(assets)_ Read the store back as an inventory
- _(assets)_ Let a caller take content back out
- _(adf)_ Report a node ADF's listItem content model forbids
- _(convert)_ Canonicalize fence languages to Atlassian's own spelling
- _(core)_ PlainTextOf, a directive-aware plain-text projection
- _(markdown)_ Expose a source-anchored span surface and the code-block view
- _(markdown)_ Expose heading source spans
- _(markdown)_ Expose image source spans
- _(markdown)_ Expose directive source spans, with per-attribute spans
- _(markdown)_ Expose inline code spans
- _(markdown)_ Add a list-item view to Source
- _(render)_ Add an option to write a significant space as itself
- _(markdown)_ Add a text-pattern view to Source
- _(markdown)_ Add a link-destination view to Source
- _(adf)_ Model the link mark's title attribute
- _(markdown)_ Locate the parts of a link reference definition
- _(markdown)_ Count the constructs a source view could not locate
- _(markdown)_ Report the spans of raw and inline HTML
- _(markdown)_ Report where each autolink is written
- _(assets)_ Narrow the upload batch with PendingRefs
- _(core)_ Expose the frontmatter split without a conversion
- _(convert)_ Export the diagnostic-code inventory as Codes()
- _(assets)_ Resolve media read-only for a report
- _(markdown)_ Report a link's and an image's title span
- _(markdown)_ Autolink bare email addresses in prose
- _(directives)_ Add a rawNode directive family for unknown ADF nodes
- _(convert)_ Preserve unknown ADF nodes losslessly via the rawNode directive

### 🐛 Bug Fixes

- _(convert)_ Stop dropping inline images silently
- _(jira)_ Submit a wire-safe rich-text document
- _(convert)_ Drop the whitespace Confluence stores next to a hardBreak
- _(assets)_ Upload an asset a document references with ./
- _(confluence)_ Publish an empty macro container as the bodiless macro
- _(convert)_ Keep the emphasis a code span opened under
- _(confluence)_ Read a legacy-content extension as the document it wraps
- _(AST)_ Give an unregistered text directive its literal text back
- _(confluence)_ Accept the code languages Cloud ADF renders
- _(format)_ Escape an ampersand that opens a character reference
- _(render)_ Separate a list item that the renderer had to break open
- _(render)_ A GFM table may follow a paragraph in a tight list item
- _(convert)_ Stop the code-span mark carry from crossing into unmarked text
- _(render)_ Keep directive-shaped prose intact in prettier mode
- _(markdown)_ Resolve an image whose tail continues on a quoted line
- _(render)_ Escape a bracket that would close a text-directive label
- _(render)_ Keep the attribute block of a container directive
- _(render)_ Drop the directive terminator the colon escape already writes
- _(render)_ Keep an id the {#id} shortcut cannot spell
- _(render)_ Drop a directive attribute a line ending cannot spell
- _(render)_ Drop a directive attribute key the block cannot spell
- _(render)_ Close a text directive whose attributes all dropped
- _(format)_ Keep generic directives in the md to md formatter
- _(markdown)_ Peel a leading byte order mark before the parse
- _(markdown)_ Keep link reference definitions and the reference form
- _(render)_ Escape a title so it survives the next parse
- _(convert)_ Keep the destination of a link that wraps an image
- _(convert)_ Read a linked image's destination back from ADF
- _(render)_ Refuse a directive name the dialect cannot spell
- _(convert)_ Keep the label of an image no asset store can place
- _(markdown)_ Measure a label closer past a nested written form
- _(render)_ Escape both brackets in a text directive's label
- _(render)_ Escape a markdown mark in a leaf or container label
- _(markdown)_ End a directive label at the closer the parser ends it at
- _(convert)_ Carry a link title across both legs
- _(markdown)_ Resolve a reference label that crosses a line
- _(convert)_ Honor WithPreserveLocalImages on the format leg
- _(directives)_ Keep a jql query when the datasource card cannot be built
- _(directives)_ Keep a smart-link label when the card cannot be built
- _(convert)_ Resolve path-addressed media on both projection legs
- _(convert)_ Report a media directive path the asset store cannot place
- _(convert)_ Carry a link title onto the media node's link mark
- _(convert)_ Keep an inline-comment anchor on a media node
- _(markdown)_ Keep a directive label's trailing whitespace
- _(AST)_ Pair a link reference on the parser's own fold
- _(directives)_ Degrade a ::jql query to flat text
- _(markdown)_ Report links and images written in a text directive's label
- _(markdown)_ Put the mailto: scheme on an email autolink's href
- _(convert)_ Stop the lax mark inference at an explicit line break
- _(markdown)_ Locate a definition whose label or title carries padding
- _(markdown)_ Reject a definition title read from a line CommonMark excludes
- _(AST)_ Keep a padded container's phantom spaces out of a definition's value
- _(convert)_ Stop the footnote diagnostic claiming a superscript nobody wrote
- _(convert)_ Stop the image spellings inventing a case for a store media id
- _(convert)_ Report a media directive that names no source at all
- _(directives)_ Resolve the path an inline media chip names
- _(convert)_ Keep a media pin an author spelled beside a path
- _(assets)_ Upload the file a media directive names by path
- _(markdown)_ Recognize an uppercase scheme and a dotless host in a bare URL
- _(assets)_ Follow the file in every spelling of a picture
- _(convert)_ Stop the external-media degrade from deleting a path
- _(markdown)_ Link a bare URL whose host opens on a dot
- _(markdown)_ Stop a non-ASCII letter truncating a bare URL's host
- _(markdown)_ Escape a colon between p-or-s and a slash in remark mode
- _(markdown)_ Link a URL literal whose host starts non-ASCII
- _(markdown)_ Escape a dot after a w in remark mode
- _(directives)_ Complete the required mediaSingle layout on a media directive
- _(markdown)_ Carry an author's escape through the format leg
- _(markdown)_ Stop escaping an ordered-list marker inside a table cell
- _(assets)_ Repair a friendly asset file on a path-keyed read
- _(markdown)_ Keep prettier's blank content line in an empty fence
- _(markdown)_ Link a scheme-less www host with no dot of its own
- _(convert)_ Stop the unplaceable-image message from claiming the store lacks the file
- _(markdown)_ Report a scheme-less www literal as a raw autolink span
- _(markdown)_ Keep a fence's trailing blank lines in the code value
- _(convert)_ Stop the format leg from deleting a fenced body's trailing blank lines
- _(markdown)_ Keep a block table cell's text that lives in a Value field
- _(markdown)_ Keep raw HTML in a table cell from breaking its row
- _(markdown)_ Fold the line ends of every value a table cell recovers
- _(convert)_ Stop a table cell's non-paragraph blocks from being deleted
- _(markdown)_ Linkify a bare URL that follows a backslash escape
- _(directives)_ Read a bare inline directive name as prose
- _(format)_ Keep the marks around an inline extension node
- _(format)_ Keep a directive whose payload the kind cannot read
- _(assets)_ Confine a report-mode blob read to the assets folder's top level
- _(convert)_ Keep the directive attributes the format leg cannot read
- _(convert)_ Report a link whose label cannot carry the mark
- _(markdown)_ Write an intraword emphasis with the delimiter that has one
- _(parser)_ Refuse an autolink host with an underscore in its last two segments
- _(parser)_ Read a "www." literal's prefix case-insensitively
- _(parser)_ Open a bare URL literal after any ASCII punctuation byte
- _(convert)_ Give the payload-less media chip a spelling that reads back
- _(convert)_ Stop a table cell from deleting an attribute-only leaf node
- _(markdown)_ Thread a mark's own closing marker to its last child
- _(markdown)_ Do not let a URL literal swallow an emphasis delimiter
- _(markdown)_ Keep a bare www literal in its source spelling when formatting
- _(confluence)_ Join a multi-block task item before Confluence runs it together
- _(markdown)_ Keep the checkbox on a task marker that heads indented blocks
- _(markdown)_ Match GFM's bare-URL scheme set and TLD class
- _(markdown)_ Keep the paragraph when a textless chip is a wrapper's sole child
- _(render)_ Keep the row a hard break ended and the text a definition lost in a table cell
- _(render)_ Hold the one-line, escaped-pipe invariant where a cell's bytes are finished
- _(markdown)_ Carry an autolink's form on the node instead of sniffing the byte before it
- _(markdown)_ Keep a hyphen in the TLD inside the bare-URL literal
- _(render)_ Ask a bare link what it writes before choosing an emphasis delimiter
- _(markdown)_ Read back the trailing blank lines goldmark deletes from an indented code block
- _(render)_ Escape the scheme colon for either case of the last scheme letter
- _(convert)_ Keep a bare autolink's label in sync with its href on decode
- _(convert)_ Render a pulled autolink bare, matching the format leg
- _(convert)_ Merge a blockTaskItem degrade's split text nodes before render
- _(convert)_ Rewrite a bare autolink's label when decode resolves its href
- _(convert)_ Merge adjacent same-mark text nodes at ADF encode
- _(markdown)_ Link a bare URL that follows an unescaped backslash
- _(markdown)_ Stop escaping a backtick the source already left bare
- _(markdown)_ Link a hyphen-led TLD after the bare www. prefix
- _(markdown)_ Scope the email-literal rescue away from escape-induced directives
- _(render)_ Keep the angle form where a bare autolink would not re-parse
- _(release)_ Keep pre-1.0 breaking changes on a minor bump

### 💼 Other

- _(release)_ Source the release tasks from the shared devkit repo
- Wire the repo to devkit's shared Go tooling
- _(build)_ Add the markdown scope the source-view commits already use
- _(deps)_ Bump goldmark-directive to v0.3.2
- Make the formatting gates measure this repo
- _(build)_ Raise the dprint pin so .config/ reaches the formatter
- Pin gofumpt to 0.12.0, the first release built against the pinned Go
- _(build)_ Make every dprint route name the config, and fail the ones that do not

### 🚜 Refactor

- _(build)_ Pass devkit's golangci thresholds with no overrides
- _(markdown)_ [**breaking**] Retire WrapProse for the renderer's print width
- _(convert)_ Build the media leaf through one constructor
- _(assets)_ Export the reference-spelling rule

### 📚 Documentation

- _(docs)_ Forbid issue tracker IDs in messages and comments
- _(wasm)_ Record why scanSpans reports no attribute offsets
- _(convert)_ Record the unplaceable-image re-pin where re-pins live
- _(markdown)_ Say why Links omits a definition, correctly
- _(markdown)_ Warn that the bare Parse/Render pair flattens frontmatter
- _(markdown)_ Drop the bare URL from TextMatches' use cases
- _(markdown)_ Warn that a text directive's span can cover less
- _(markdown)_ Say which bytes a label parse actually trims
- _(markdown)_ Say why a definition run keeps the block separator
- _(convert)_ Stop the reference and two doc comments understating media links
- _(core)_ Correct what WithPreserveLocalImages and WithNoWrap claim
- _(convert)_ Say where ToADF is not invariant under Normalize
- _(directives)_ Say that a canonical directive spells a schema default
- _(docs)_ State the tracker-ID rule without an example that breaks it
- _(format)_ Name the one prettier the parity measurements may use
- _(format)_ Say why the authority's underscore emphasis is unreachable
- _(docs)_ State format-leg totality at the attribute level
- _(confluence)_ Say that a downgraded blockTaskItem joins its blocks with nothing
- _(directives)_ Say that the sourceless chip now has a spelling, and what gives it one
- _(confluence)_ Say why legacy-content expansion matches only the bodiless form
- _(markdown)_ Mark the tight-item table row as a deliberate divergence
- Record the join confluence.MarkdownOptions adds to the blockTaskItem downgrade
- Retire the ftp advice the GFM scheme set made stale
- _(docs)_ Correct the fuzz command and name the corpus it can delete
- _(markdown)_ Correct colonURLParser's reference measurement and host gate
- _(confluence)_ Correct SeparateBlockTaskItems doc against live measurement
- _(skill)_ Document the raw-node-corrupted diagnostic

### 🎨 Styling

- _(assets)_ Tidy three lint findings in the asset store
- _(assets)_ Clear the shadow, naming, and unused-parameter findings
- _(markdown)_ Satisfy the quality gate on the new render-leg tests

### 🧪 Testing

- _(convert)_ Measure the doubled media projection against itself
- _(markdown)_ Fuzz the directive view against the parser it mirrors
- _(markdown)_ Assert the guarded parse's recovery ladder with an injected panic
- _(markdown)_ Pin the definition resolver's verification predicate itself
- _(convert)_ Pin raw HTML on both legs and record the escape as correct
- _(format)_ Drop the unreachable layout-less mediaSingle fuzz waiver
- _(format)_ Spell unparsable the way the typos gate does
- _(format)_ Record that the backtick's spelling is re-derived, not preserved
- _(render)_ Pin the underscore-before-autolink escape to the tree, not the next byte
- _(markdown)_ Pin that no character-reference byte is a linkify boundary
- _(core)_ Repair the escaped-URL skip classes against the bytes the renderer writes
- _(core)_ Pin that the escape-adjacent skips do not reach a schemed literal
- _(core)_ Record the bare-link form each render leg owes its own reference
- _(core)_ Inventory the uppercase scheme colon as a live failure, not a skip
- _(core)_ Skip a www host whose second dot a one-byte lookahead misses
- _(core)_ Recognize a dotless host in the fused-underscore skip class
- _(convert)_ Pin the lossless rawNode round trip and its corruption diagnostics
- _(markdown)_ Pin the punycoded www/URL-literal href across the ADF leg
- _(markdown)_ Pin the dangling-label trailing-underscore split (3p7g row 5)

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.7.0
- _(docs)_ State the measured fact instead of citing a tracker item
- _(build)_ Bump pinned dev tool versions

## [v0.7.0] - 2026-08-21

### 🚀 Features

- _(convert)_ Add ordinary link resolver
- _(core)_ Support custom heading anchor ids with {#my-anchor}
- _(convert)_ Publish a labelled link as the host editor's file card

### 📚 Documentation

- Rewrite prose in Simplified Technical English

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.6.2
- _(build)_ Add the convert scope to cog.toml

## [v0.6.2] - 2026-08-20

### 🐛 Bug Fixes

- _(release)_ Select root tag for goreleaser

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.6.1

## [v0.6.1] - 2026-08-20

### 🐛 Bug Fixes

- _(release)_ Push the tags in stages again, from release:push alone

### 💼 Other

- _(release)_ Split prepare and push by vcs

### 🚜 Refactor

- _(release)_ Fold release:tag into release:push

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.6.0

## [v0.6.0] - 2026-08-20

### 🚀 Features

- _(wasm)_ Add the js/wasm module exposing adfast to JavaScript
- _(wasm)_ Export catalog() so consumers can bind directive names

### 🐛 Bug Fixes

- _(core)_ Keep the md round trip idempotent across eighteen re-parse hazards
- _(release)_ Pin every unpublished monorepo module in go.work

### 💼 Other

- _(release)_ Allow the deps commit scope
- _(deps)_ Bump goldmark-directive to v0.3.0
- _(deps)_ Bump goldmark-directive to v0.3.1 and drop its workaround
- _(release)_ Pin the v0.6.0 monorepo module versions

### 📚 Documentation

- _(wasm)_ Document the module, its offsets contract and its release order

### 🧪 Testing

- _(wasm)_ Add the Node smoke test and the build tasks

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.5.0
- _(wasm)_ Test, build and smoke-test the wasm module

## [v0.5.0] - 2026-08-14

### 🚀 Features

- _(adf)_ Add WithIncrementListMarkers for adf->md ordered lists
- _(adf)_ Add WithMediaAssetResolver for lazy media lookup
- _(confluence)_ Add named directives for the core macros

### 🐛 Bug Fixes

- _(adf)_ Flow a text node's own line breaks on adf->md
- _(format)_ Drop the default media type from inline media
- _(render)_ Drop the blank paragraphs Markdown cannot write
- _(release)_ Resolve the release version from the tag being released

### 🚜 Refactor

- _(directives)_ Split media attribute writing out of mediaLeafNode
- _(adf)_ Split media attribute writing out of the format normalizer

### 📚 Documentation

- _(skill)_ Show media directives in their canonical form

### 🎨 Styling

- _(directives)_ Flip negated media default checks

### 🧪 Testing

- Check the errors the lint gate flags

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.4.0

## [v0.4.0] - 2026-07-29

### 🚀 Features

- _(directives)_ Add WithPreserveLocalImages to keep unresolved local image refs

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.3.0

## [v0.3.0] - 2026-07-28

### 🚀 Features

- _(directives)_ Omit default media attrs (type=file, layout=align-start)
- _(directives)_ Resolve media id from the asset store, omit it when local
- _(directives)_ Omit re-derivable media dims + empty collection
- _(assets)_ Measure WebP/TIFF/BMP dimensions via golang.org/x/image
- _(directives)_ Drop no-op display width from media directives

### 🐛 Bug Fixes

- _(format)_ Apply store-aware media slimming in format mode

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.2.0

## [v0.2.0] - 2026-07-28

### 🚀 Features

- _(assets)_ WithStoreDir option to place blobs in a custom dir
- _(assets)_ SyncMarkdown to upload assets referenced by one md string

### 💼 Other

- Add assets commit scope to cog.toml

### ⚙️ Miscellaneous Tasks

- Update go.sum after v0.1.0

## [v0.1.0] - 2026-07-23

### 🚀 Features

- Initial implementation — markdown ⇄ Atlassian Document Format at the AST level
  [v0.8.0]: https://github.com/pmarschik/adfast/releases/tag/v0.8.0
  [v0.7.0]: https://github.com/pmarschik/adfast/releases/tag/v0.7.0
  [v0.6.2]: https://github.com/pmarschik/adfast/releases/tag/v0.6.2
  [v0.6.1]: https://github.com/pmarschik/adfast/releases/tag/v0.6.1
  [v0.6.0]: https://github.com/pmarschik/adfast/releases/tag/v0.6.0
  [v0.5.0]: https://github.com/pmarschik/adfast/releases/tag/v0.5.0
  [v0.4.0]: https://github.com/pmarschik/adfast/releases/tag/v0.4.0
  [v0.3.0]: https://github.com/pmarschik/adfast/releases/tag/v0.3.0
  [v0.2.0]: https://github.com/pmarschik/adfast/releases/tag/v0.2.0
  [v0.1.0]: https://github.com/pmarschik/adfast/releases/tag/v0.1.0
