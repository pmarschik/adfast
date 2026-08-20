# Releasing adfast

adfast is a multi-module repository: the root module
`github.com/pmarschik/adfast` and the nested modules
`github.com/pmarschik/adfast/jira`,
`github.com/pmarschik/adfast/confluence`,
`github.com/pmarschik/adfast/skill`,
`github.com/pmarschik/adfast/frontmatter`, and
`github.com/pmarschik/adfast/wasm` (listed in `go.work`). All
release together, from one commit, with matched version numbers.

The release tasks discover modules from `go.work`, so adding a module
there is enough for it to be pinned and tagged — no task edits needed.

## TL;DR

```sh
mise run release:prepare   # bump version, regenerate CHANGELOG.md, pin go.mod versions
# review CHANGELOG.md and the go.mod changes
mise run release:tag       # commit, tag root + submodules — local only
mise run release:push      # push branch, then tags in order, update release notes
mise run release:post-tidy # once the proxy serves it: re-tidy go.sum
```

Nothing before `release:push` touches the remote, so a bad `prepare` or
`tag` is undone locally. Pushing requires a hardware key touch.
Tag pushes trigger `.github/workflows/release.yml`, which runs goreleaser
with `--release-notes CHANGELOG.md` against `.config/goreleaser.yaml`.

## Multi-module release order

Go modules in one repository resolve independently, so the order matters:

1. **Tag the root module first**: `vX.Y.Z`. The root module has no
   intra-repo requirements, so its tag is immediately consumable.
2. **Every submodule go.mod must require that version**: `require
   github.com/pmarschik/adfast vX.Y.Z` in `jira/go.mod`,
   `confluence/go.mod`, `skill/go.mod`, `frontmatter/go.mod`, and
   `wasm/go.mod`. The pin is written at release time — `mise run
   release:prepare` runs `go mod edit -require` with the bumped version
   (and strips any intra-repo `replace` directives) in every workspace
   module. `wasm/go.mod` is the one module that requires more than the
   root: it also pins `github.com/pmarschik/adfast/jira vX.Y.Z` and
   `github.com/pmarschik/adfast/confluence vX.Y.Z`, so it must be
   consumable only after those two are tagged.
3. **Tag the submodules**: `jira/vX.Y.Z`, `confluence/vX.Y.Z`,
   `skill/vX.Y.Z`, `frontmatter/vX.Y.Z`, and — after those —
   `wasm/vX.Y.Z`, all on the same commit. `release:tag` creates the tags;
   `release:push` pushes the root tag first (GitHub suppresses workflow
   events when more than three tags arrive at once, so the root tag goes
   alone to reliably trigger Actions; the submodule tags follow in a
   second push, which carries `jira/` and `confluence/` alongside
   `wasm/`, so `wasm/vX.Y.Z` is never resolvable before its
   requirements).

## The unpublished-version window

The submodule go.mod files pin the _upcoming_ version, which does not
exist on the module proxy until the root tag is pushed. Two
consequences:

- **`GOWORK=off` resolution of a submodule alone is expected to fail
  until the root tag `vX.Y.Z` exists** — `go build`/`go test` in
  `jira/`, `confluence/`, `skill/`, `frontmatter/`, or `wasm/` with the
  workspace disabled tries to download
  `github.com/pmarschik/adfast@vX.Y.Z` (and, for `wasm/`, the `jira/`
  and `confluence/` tags too) and gets an unknown revision. This is
  normal before a release; do not
  "fix" it by re-adding a `replace` to a submodule go.mod (released
  module files must be replace-free).
- **Local development builds resolve through the workspace.** `go.work`
  carries a version-specific replace for every monorepo module some
  go.mod requires — `replace github.com/pmarschik/adfast vX.Y.Z => .`
  plus, because `wasm/` requires them, `…/confluence vX.Y.Z =>
  ./confluence` and `…/jira vX.Y.Z => ./jira` — so that the module graph
  can be computed without fetching the not-yet-published versions. The
  workspace `use` list is not enough on its own: loading the graph still
  reads the go.mod of every required version, so a submodule required by
  a sibling needs its own replace. `release:prepare` bumps all of them
  together with the go.mod requires (and adds a line for a module that
  has newly become a sibling requirement); a stale or missing pin here
  breaks every build between `prepare` and `tag` with "unknown revision
  vX.Y.Z".

## After tagging

`release:tag` stops at the local tags. Two steps finish the release:

- `mise run release:push` — advance the `main` bookmark, push the branch,
  then the root tag alone, then the submodule tags, and create/update the
  GitHub release from `CHANGELOG.md`. The staging is the whole point of
  the task: `jj git push --all` pushes bookmarks **and** tags (jj has
  pushed tags since 0.36), so pushing everything at once sends six tags
  in one go and GitHub suppresses the push event that
  `.github/workflows/release.yml` listens for — which is exactly how
  v0.5.0 and v0.6.0 shipped without goreleaser artifacts. Hence
  `--bookmark main`, then `--tag vX.Y.Z`, then `--all`.
- `mise run release:post-tidy` — after the proxy serves the new version,
  re-tidy every module's `go.sum` with `GOWORK=off` (cross-module
  checksums can only be computed against the published version) and sync
  `go.work.sum`. Commit the result as
  `chore: update go.sum after vX.Y.Z`.

## Versioning

- SemVer, driven by Conventional Commits (`git cliff --bumped-version`).
- Breaking API changes must use `feat!:`/`fix!:` (major bump); this is a
  public library — see AGENTS.md for the API stability rules.
- Root and submodule versions stay in lockstep: one release, six tags
  (`vX.Y.Z` + `jira/vX.Y.Z` + `confluence/vX.Y.Z` + `skill/vX.Y.Z` +
  `frontmatter/vX.Y.Z` + `wasm/vX.Y.Z`) on the same commit.
