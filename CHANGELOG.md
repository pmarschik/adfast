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
  [v0.4.0]: https://github.com/pmarschik/adfast/releases/tag/v0.4.0
  [v0.3.0]: https://github.com/pmarschik/adfast/releases/tag/v0.3.0
  [v0.2.0]: https://github.com/pmarschik/adfast/releases/tag/v0.2.0
  [v0.1.0]: https://github.com/pmarschik/adfast/releases/tag/v0.1.0
