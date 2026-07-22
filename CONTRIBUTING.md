# Contributing to adfast

## Toolchain

Tooling is managed with [mise](https://mise.jdx.dev) — tool versions are
pinned in `.config/mise/config.toml`:

```sh
mise install      # install the pinned toolchain
mise run setup    # download Go deps and install the git hooks
```

The git hooks are managed by [hk](https://hk.jdx.dev)
(`.config/hk.pkl`): pre-commit runs format/lint on changed files, and
commit messages are checked against Conventional Commits.

## Everyday tasks

```sh
mise run check          # all quality gates: format + lint + typos + test
mise run test           # go test across both workspace modules
mise run fmt            # golangci-lint --fix + dprint
mise run lint           # golangci-lint + dprint check
mise run check-changed  # hk check on changed files only
mise run tidy           # go mod tidy across workspace modules
```

Run `mise run check` before pushing. The repo is a Go workspace
(`go.work`) with four modules — the root, `jira/`, `confluence/`, and
`skill/` — and the tasks iterate over all of them.

## Testing

- `go test ./...` from the root covers the workspace; CI additionally
  runs each submodule (`jira/`, `confluence/`, `skill/`) on its own.
- Renderer or parser changes should also run the round-trip fuzzer:

  ```sh
  go test -fuzz FuzzRoundTripIdempotent .
  ```

  New crashers become corpus seeds (`testdata/fuzz/`). Known
  remark-equally-unstable inputs are documented as skip classes in
  `adf_fuzz_test.go` — each has a probe input and analysis; do not
  silence new failures without that analysis.
- The rendering rules are measured against reference implementations
  (remark-stringify, and prettier for the formatter): a fixture corpus
  generated from them (`testdata/directive_fixtures.json`) pins
  escaping, wrapping, list marker alternation, and character-reference
  encoding. New divergences must be deliberate and documented next to
  the code.
- The README tutorial is executable: `TestReadmeTutorialRoundTrips`
  extracts the fenced example and asserts it round-trips, so keep it
  green when editing either side.

## Commits

Conventional Commits, strictly: `<type>(<scope>): <description>` with
types `feat|fix|refactor|build|ci|chore|docs|style|perf|test` and scopes
from `cog.toml`. Breaking changes use `feat!:`/`fix!:` — this is a
public library; see AGENTS.md for the API stability rules.

## Releases

See [docs/RELEASING.md](docs/RELEASING.md).
