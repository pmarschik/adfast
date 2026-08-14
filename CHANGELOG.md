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
  [v0.5.0]: https://github.com/pmarschik/adfast/releases/tag/v0.5.0
  [v0.4.0]: https://github.com/pmarschik/adfast/releases/tag/v0.4.0
  [v0.3.0]: https://github.com/pmarschik/adfast/releases/tag/v0.3.0
  [v0.2.0]: https://github.com/pmarschik/adfast/releases/tag/v0.2.0
  [v0.1.0]: https://github.com/pmarschik/adfast/releases/tag/v0.1.0
