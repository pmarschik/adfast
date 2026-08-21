# adfast

Markdown ⇄ ADF (Atlassian Document Format) conversion at the AST level.
Read README.md for the dialect and the API. The package documentation is
in doc.go.

## Build & Test Commands

- `mise run setup` — install the dependencies and the hooks
- `mise run check` — run every quality gate (format + lint + typos + test)
- `mise run fmt` — format the code
- `mise run lint` — run the linters
- `mise run test` — run the tests
- `go test -fuzz FuzzRoundTripIdempotent ./...` — grow the round-trip
  corpus

## Conventions

### Commits

Use Conventional Commits. No other form is permitted:
`<type>(<scope>): <description>`.
Types: feat, fix, refactor, build, ci, chore, docs, style, perf, test.
Scopes: `cog.toml` defines them.

### API Stability

This is a public Go library. storysmith-md and future Confluence tooling
build on it. A breaking change reaches every downstream consumer.

- NEVER make a breaking API change before you ask the user
- A breaking change MUST use `feat!:` or `fix!:` (major bump)
- Add instead of change. Deprecate before you remove.

### Behavior invariants

- The md → adf → md round trip
  (`ToMarkdown(FromADF(ToADF(FromMarkdown(md))))`) must stay idempotent.
  After a change to the renderer or the parser, run the fuzzer. Add each
  crasher as a seed.
- The rendering is measured against remark-stringify. Keep the measured
  rules for escaping, wrapping, and alternation documented next to the
  code. A new divergence must be deliberate and documented.
- The fuzz skip classes in adf_fuzz_test.go document two known groups:
  inputs that remark is equally unstable on, and goldmark parser
  divergences. Each class has a probe input. Do not silence a new
  failure without that analysis.

### Change fan-out (update these together)

Many changes ripple across the code, the byte-exact fixtures, and
several docs. Tests do not guard all of the docs. Round-trip tests
protect the tutorial block of `README.md` and
`skill/assets/references/example.md`. The syntax **tables** and the
**prose** in `README.md`, `skill/assets/references/*.md`, and
`docs/design.md` rot without a signal. When you land one of the changes
below, update the whole row:

- **Dialect syntax** (add, rename, or retire a directive or an
  attribute; change the quoting or escaping of a directive surface):
  `dialect/` (the kind and its `dialect.Visitor` case) → `convert/` both
  directions + `convert.Normalize` → re-pin the affected
  `testdata/directive_fixtures.json` entries and every affected
  `testdata/fuzz/` seed (do this deliberately; the ADF payload must stay
  semantically identical, and only the markdown surface changes) →
  `README.md` "Supported Markdown" tables + the tutorial → **all of**
  `skill/assets/references/{syntax.md, adf-coverage.md, example.md}`
  (the skill is the agent-facing mirror of the dialect and rots first) →
  `docs/design.md` if the model changed → the storysmith goldens that
  pin the old surface. Then run the tutorial and skill-example
  round-trip tests.
- **ADF node/mark** (a new or changed kind or coverage): `adf/` (the
  typed node + `adf.Visitor`/`MarkVisitor` + walk + encode/decode) →
  `convert/` both directions + Normalize (the exhaustive visitors give a
  compile error for each missing case; follow them) → the ADF coverage
  matrix in `README.md` **and** in
  `skill/assets/references/adf-coverage.md` → the attribute reference
  (`syntax.md`) → the fixtures. `docs/adf-availability.json` is the
  machine source of truth for the per-product **UnsupportedKinds** sets:
  `jira.UnsupportedKinds` comes from `jira == "no"`, and
  `confluence.UnsupportedKinds` from `confluence == "no"` (empty at
  present). When the coverage matrix or the availability changes,
  regenerate those sets to match. The product UnsupportedKinds sets hold
  RENDER-CONFIRMED non-support only, where a live product render dropped
  or blocked the kind. They never hold docs-by-omission, which keeps the
  `unsupported-in-product` diagnostic free of false positives.
- **Renderer/parser** (escaping, wrapping, list or table formatting):
  keep the remark-stringify and prettier byte pins. Run the round-trip
  and format-semantics fuzzers. Commit each crasher as a seed. Document
  a deliberate new divergence next to the code and in `docs/design.md`.
- **Facade / options / Pipeline**: `doc.go` + the `README.md` quickstart
  and examples + `example_test.go` + the storysmith call sites. The
  godoc of each option must name the primitive that reads it. A breaking
  change uses `feat!:`.
- **New diagnostic code**: add the `convert.Code*` (or `adf.Code*`)
  constant, wire the sink in the primitive that emits it, and list it in
  the `README.md` error-handling section and in
  `skill/assets/references/pitfalls.md`.
- **Store / assets**: keep the `Store` interface storage-agnostic. The
  FSStore specifics (folder, symlink, size caps, dedup) stay on the
  FSStore documentation. Update the README "Asset store" section and the
  asset internals in `docs/design.md`.
- **New submodule/addon**: a `go.work` member + a version-specific
  replace → a `.github/workflows/ci.yml` test step (with `-race`) →
  `docs/RELEASING.md` module tag order and require pins → `README.md`
  Install and Layout → `CHANGELOG.md` → an `example_test.go`. Keep every
  third-party dependency (for example, `yaml.v3`) in the submodule,
  never in the root.

### Multi-module layout

The root module is platform-neutral ADF ⇄ md. Platform-specific addons
ship as submodules (jira/, confluence/) that `go.work` lists. Keep
Jira-only and Confluence-only behavior out of the root module. The
frontmatter/ (YAML) and skill/ modules are optional addons too. The
skill module embeds the agent-facing dialect documentation and MUST
track each dialect change (see Change fan-out).

### Version Control

- Primary VCS: jj (jujutsu), colocated with git
- Run `mise run check` before `jj git push`
- Do not push directly. Prompt the user, because the signature needs a
  hardware key.
