# Contributing to adfast

## Toolchain

[mise](https://mise.jdx.dev) manages the tooling.
`.config/mise/config.toml` pins the tool versions:

```sh
mise install      # install the pinned toolchain
mise run setup    # download the Go deps and install the git hooks
```

[hk](https://hk.jdx.dev) manages the git hooks through `.config/hk.pkl`.
The pre-commit hook formats and lints the changed files. The commit-msg
hook checks the message against Conventional Commits.

## Everyday tasks

```sh
mise run check          # all quality gates: format + lint + typos + test
mise run test           # go test across every workspace module
mise run fmt            # golangci-lint --fix + dprint
mise run lint           # golangci-lint + dprint check
mise run check-changed  # hk check on the changed files only
mise run tidy           # go mod tidy across the workspace modules
```

Run `mise run check` before you push. The repository is a Go workspace
(`go.work`) with six modules: the root, `jira/`, `confluence/`,
`skill/`, `frontmatter/`, and `wasm/`. Each task iterates over all of
them.

## Testing

- `go test ./...` from the root covers the workspace. CI also runs
  `jira/`, `confluence/`, `skill/`, `frontmatter/`, and `wasm/` on their
  own.
- After a change to the renderer or the parser, run the round-trip
  fuzzer too:

  ```sh
  go test -fuzz FuzzRoundTripIdempotent .
  ```

  Each new crasher becomes a corpus seed in `testdata/fuzz/`. The skip
  classes in `adf_fuzz_test.go` document the inputs that remark is
  equally unstable on. Each class has a probe input and an analysis. Do
  not silence a new failure without that analysis.
- The rendering rules are measured against reference implementations:
  remark-stringify, and prettier for the formatter. A fixture corpus
  generated from them (`testdata/directive_fixtures.json`) pins the
  escaping, the wrapping, the list marker alternation, and the
  character-reference encoding. A new divergence must be deliberate and
  documented next to the code.
- The README tutorial is executable. `TestReadmeTutorialRoundTrips`
  extracts the fenced example and asserts that it round-trips. When you
  edit either side, keep that test green.

## Commits

Use Conventional Commits. No other form is permitted:
`<type>(<scope>): <description>`, with the types
`feat|fix|refactor|build|ci|chore|docs|style|perf|test` and the scopes
from `cog.toml`. A breaking change uses `feat!:` or `fix!:`. This is a
public library. Read AGENTS.md for the API stability rules.

## Releases

Read [docs/RELEASING.md](docs/RELEASING.md).
