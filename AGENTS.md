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
- `go test -run xxx -fuzz FuzzRoundTripIdempotent -fuzztime=90s .` — grow
  the round-trip corpus. The package argument is `.`, not `./...`; see
  "Running the fuzzers" below for why neither `./...` nor `./markdown/`
  works.

**Format and check through the tasks, not through the tools.** `mise run
fmt` formats everything the repo owns, `mise run lint` and `mise run check`
verify it, and both already hand every tool this repo's own config. There is
no reason to invoke `dprint` by hand, and a hand-typed one is how the repo
got measured against a stranger's config: see the `dprint` bullet below.

The bullets below matter only when you are debugging a tool's report. A
`gofmt` or a `dprint` taken from `PATH` measures something other than this
repo and reports files that are already correct:

- `mise exec -- gofmt -l .`, not `gofmt -l .`. The `gofmt` on `PATH` is
  whichever Go the shell happens to have, while `.config/mise/config.toml`
  pins `go = "1.27.0"`. The alignment of a long map literal moved between
  Go releases, so an older `gofmt` reports the generated
  `dialect/emoji_map.go`. The generator already writes what its own
  toolchain's `gofmt` writes, and `golangci-lint` agrees because that
  binary is built with the pinned Go.
- The standalone `gofumpt` binary carries the same skew: 0.9.2 is built
  with go1.25.3 and so rewrites `dialect/emoji_map.go` to the older
  alignment. Read `mise exec -- gofumpt -l .` with that in mind, and if
  the hooks ever rewrite that file, regenerate it with
  `go run ./internal/genemoji -output dialect/emoji_map.go` rather than
  keeping the rewrite.
- `dprint` cannot discover `.config/dprint.json`: it looks only for
  `dprint.json(c)` or `.dprint.json(c)` walking up from the cwd. A bare
  `dprint check` therefore used to load the home config of whoever ran it and
  report this repo against a plugin set it never chose.
  `DPRINT_CONFIG_DISCOVERY = "false"` in `.config/mise/config.toml` now makes
  that run fail with exit 11 instead of answering wrongly. If you see it,
  you called dprint directly — run `mise run fmt` or `mise run lint`.

## Conventions

### Commits

Use Conventional Commits. No other form is permitted:
`<type>(<scope>): <description>`.
Types: feat, fix, refactor, build, ci, chore, docs, style, perf, test.
Scopes: `cog.toml` defines them.

Never cite an issue tracker ID in a commit message or a source comment.
This repository is public; the trackers that drive the work on it are
not, so a bare tracker slug — a project prefix and four random
characters — is an opaque token to every reader outside one machine. Write the reason instead — the measurement,
the failing input, the name of the test that pins it. Referring to
storysmith-md as a consumer by name is fine; citing its issue IDs is
not.

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

#### Running the fuzzers, and the two ways it goes wrong

Both of these have cost real time, and both fail quietly rather than
loudly.

- **`FuzzRoundTripIdempotent` lives in the ROOT package**
  (`adf_fuzz_test.go`), not in `./markdown/`, and the package argument has
  to name it exactly. `./...` refuses outright — `cannot use -fuzz flag
  with multiple packages` — and the natural next guess, `./markdown/`,
  is the dangerous one: it prints `no fuzz tests to fuzz`, then `PASS`, and
  exits 0. So "I ran the fuzzer" can mean nothing was measured. Measured
  2026-09-09 on the same tree: `-fuzztime=5s .` reproduces the known
  `:www.0` crasher in 1.1s, while `./markdown/` passes in 0.4s. Run it as
  `go test -run xxx -fuzz FuzzRoundTripIdempotent -fuzztime=90s .`, and run
  `FuzzFormatSemanticsPreserved` too — it catches what the round-trip leg
  misses.
- **`testdata/fuzz` is a TRACKED corpus of ~225 files.** Clearing fuzzer
  output with `rm -rf testdata/fuzz` deletes it. Delete only the files the
  run just wrote, and check `git status --untracked-files=all` before
  committing. A commit carrying a corpus deletion is a serious error.
- A crasher you hit is not automatically yours. Before concluding anything,
  replay the minimized seed on the base commit and compare the output byte
  for byte — a `-fuzztime` budget under ~90s also means whole families
  never surface, so a clean short run is weak evidence.
- **A round-trip run that dies in seconds has told you nothing about your
  change.** While a shallow crasher is open, the fuzzer reaches it long
  before it reaches anything new, and every further second of budget is
  spent re-failing it. Measured 2026-09-09: `FuzzRoundTripIdempotent` dies
  on `:www.0` at 0.58s, and with a fresh `GOCACHE` — which discards the
  2371-entry generated corpus — it explores 253,278 execs in 6.4s and then
  finds the same input on its own. So clearing the cache is not the fix,
  and the leg is not a gate until that input is fixed. Read the elapsed
  time before reading the verdict, and if the run ended on a crasher that
  was already open, say that the leg was blocked rather than that it
  passed.

### Which prettier is the reference

There is exactly one authoritative prettier, and it is the copy inside
the frozen TS reference that storysmith-md ships:

```text
~/.local/share/volta/tools/image/packages/@ixopay/storysmith-md/lib/node_modules/@ixopay/storysmith-md/node_modules/prettier
```

It is authoritative because it is the version that reference pins
(3.8.1 today), and matching that reference byte for byte is the whole
point of the prettier mode. A prettier taken from any other checkout
formats a document the product never formats. Drive it with the flags
the parity pins use — `--parser markdown --prose-wrap always
--print-width 80 --embedded-language-formatting off` — and with
`--no-config`, so no surrounding checkout's `.prettierrc` reaches it.
Formatting is a pure local pass, so running it needs no network and no
credentials.

A newer prettier is not a second opinion about the same rules; it is a
different formatter. Prettier replaced its markdown parser after 3.8, so
3.9.6 — which sits in the developer-hub checkout on this machine and is
easy to reach for by accident — disagrees with the authority on
emphasis parsing and nesting, on `*` and `_` escaping, on setext
headings, on a table inside a tight list item, and on single-tilde
strikethrough. Every one of those is a surface adfast renders, so a rule
measured on the wrong install lands as a parity regression rather than a
parity fix.

One consequence is worth stating outright, because it decides whether a
divergence is a defect at all. The authority's markdown parser predates
CommonMark and reads `_` exactly as it reads `*`, so it makes emphasis
out of runs that CommonMark leaves as literal text: it formats
`foo__bar__baz` to `foo**bar**baz`, `a_b_` to `a*b*`, and `***a***` to
`**_a_**`. adfast parses with goldmark, which is CommonMark, and so
agrees with the NEWER prettier at parse time on every one of those. The
authority's answer there is not a target adfast is missing, it is a
shape adfast cannot reach from markdown source at all, and rewriting the
renderer to chase it would mean inventing emphasis the parse never saw.
What remains reachable on those inputs is the escaping decision for the
literal `_`, and adfast already matches the authority on it —
`_bar_baz` renders as `\_bar_baz`, `a _ b` as `a \_ b`. Under the newer
prettier all four of those read as over-escaping. They are not.

Because the two sit side by side, a measurement quoted in a comment, a
test, or a commit message must name the version it came from. A note
that says only "measured against prettier" cannot be re-checked, and the
next reader cannot tell whether it describes the product's formatter or
somebody's node_modules. Several comments under `markdown/` cite 3.9.6.
Those cases were re-measured against 3.8.1 and the two agree on all of
them, so the comments stand; the citation is the part to read with care.

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
