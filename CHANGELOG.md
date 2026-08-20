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
  [v0.6.2]: https://github.com/pmarschik/adfast/releases/tag/v0.6.2
  [v0.6.1]: https://github.com/pmarschik/adfast/releases/tag/v0.6.1
  [v0.6.0]: https://github.com/pmarschik/adfast/releases/tag/v0.6.0
  [v0.5.0]: https://github.com/pmarschik/adfast/releases/tag/v0.5.0
  [v0.4.0]: https://github.com/pmarschik/adfast/releases/tag/v0.4.0
  [v0.3.0]: https://github.com/pmarschik/adfast/releases/tag/v0.3.0
  [v0.2.0]: https://github.com/pmarschik/adfast/releases/tag/v0.2.0
  [v0.1.0]: https://github.com/pmarschik/adfast/releases/tag/v0.1.0
