# adfast

Markdown ⇄ ADF (Atlassian Document Format) conversion at the AST level.
See README.md for the dialect and API; docs in doc.go.

## Build & Test Commands

- `mise run setup` — install dependencies and hooks
- `mise run check` — run all quality gates (format + lint + typos + test)
- `mise run fmt` — format code
- `mise run lint` — run linters
- `mise run test` — run tests
- `go test -fuzz FuzzRoundTripIdempotent ./...` — grow the round-trip corpus

## Conventions

### Commits

Use Conventional Commits strictly: `<type>(<scope>): <description>`.
Types: feat, fix, refactor, build, ci, chore, docs, style, perf, test.
Scopes: defined in `cog.toml`.

### API Stability

This is a public Go library (storysmith-md and future Confluence tooling
build on it). Breaking changes affect downstream consumers.

- NEVER introduce breaking API changes without asking the user first
- Breaking changes MUST use `feat!:`/`fix!:` (major bump)
- Prefer adding over changing; deprecate before removing

### Behavior invariants

- The md → adf → md round trip
  (`ToMarkdown(FromADF(ToADF(FromMarkdown(md))))`) must stay idempotent:
  run the fuzzer after renderer/parser changes and add crashers as seeds.
- Rendering is measured against remark-stringify; keep the measured
  escaping/wrapping/alternation rules documented next to the code. New
  divergences must be deliberate and documented.
- The fuzz skip classes in adf_fuzz_test.go document known
  remark-equally-unstable inputs and goldmark parser divergences — each
  has a probe input; do not silence new failures without that analysis.

### Change fan-out (update these together)

Many changes ripple across code, byte-exact fixtures, and several docs.
The docs are NOT all test-guarded — round-trip tests protect
`README.md`'s tutorial block and `skill/assets/references/example.md`,
but the syntax **tables** and **prose** in `README.md`,
`skill/assets/references/*.md`, and `docs/design.md` rot silently. When
you land one of these, update the whole row:

- **Dialect syntax** (add/rename/retire a directive or attribute; change
  quoting/escaping of a directive surface): `dialect/` (the kind + its
  `dialect.Visitor` case) → `convert/` both directions + `convert.Normalize`
  → re-pin the affected `testdata/directive_fixtures.json` entries and any
  `testdata/fuzz/` seeds (deliberately; the ADF payload must stay
  semantically identical — only the markdown surface changes) →
  `README.md` "Supported Markdown" tables + the tutorial → **all of**
  `skill/assets/references/{syntax.md, adf-coverage.md, example.md}` (the
  skill is the agent-facing mirror of the dialect and rots first) →
  `docs/design.md` if the model changed → storysmith goldens if any pin
  the old surface. Run the tutorial + skill-example round-trip tests.
- **ADF node/mark** (new/changed kind or coverage): `adf/` (typed node +
  `adf.Visitor`/`MarkVisitor` + walk + encode/decode) → `convert/` both
  directions + Normalize (the exhaustive visitors give compile errors
  for missing cases — follow them) → the ADF coverage matrix in
  `README.md` **and** `skill/assets/references/adf-coverage.md` → the
  attribute reference (`syntax.md`) → fixtures. `docs/adf-availability.json`
  is the machine source of truth for the per-product **UnsupportedKinds**
  sets (`jira.UnsupportedKinds` from `jira == "no"`,
  `confluence.UnsupportedKinds` from `confluence == "no"` — currently
  empty): when the coverage matrix/availability changes, regenerate those
  sets to match — but the product UnsupportedKinds sets are RENDER-CONFIRMED non-support only (a live product render dropped/blocked the kind), NOT docs-by-omission, to keep the `unsupported-in-product` diagnostic free of false positives.
- **Renderer/parser** (escaping, wrapping, list/table formatting): keep
  the remark-stringify / prettier byte pins; run the round-trip and
  format-semantics fuzzers, commit any crasher as a seed, and document a
  deliberate new divergence next to the code + in `docs/design.md`.
- **Facade / options / Pipeline**: `doc.go` + `README.md` quickstart &
  examples + `example_test.go` + storysmith call sites; each option's
  godoc must state which primitive reads it; breaking → `feat!:`.
- **New diagnostic code**: add the `convert.Code*` (or `adf.Code*`)
  constant, wire the sink in the emitting primitive, and list it in
  `README.md` error-handling + `skill/assets/references/pitfalls.md`.
- **Store / assets**: keep the `Store` interface storage-agnostic
  (FSStore specifics — folder, symlink, size caps, dedup — stay on
  FSStore's own docs); update the README "Asset store" section +
  `docs/design.md` asset internals.
- **New submodule/addon**: `go.work` member + version-specific replace →
  `.github/workflows/ci.yml` test step (with `-race`) → `docs/RELEASING.md`
  module tag order + require pins → `README.md` Install + Layout →
  `CHANGELOG.md` → an `example_test.go`. Keep third-party deps (e.g.
  `yaml.v3`) in the submodule, never the root.

### Multi-module layout

The root module is platform-neutral ADF ⇄ md. Platform-specific addons
ship as submodules (jira/, confluence/) listed in go.work; keep
Jira/Confluence-only behavior out of the root module. The frontmatter/
(YAML) and skill/ modules are likewise optional addons; the skill module
embeds the agent-facing dialect docs and MUST track dialect changes
(see Change fan-out).

### Version Control

- Primary VCS: jj (jujutsu), colocated with git
- Run `mise run check` before `jj git push`
- Do not push directly — prompt the user (hardware key signing)
