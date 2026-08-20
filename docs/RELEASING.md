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
# review the prepared release commit, changelog, go.mod pins, bookmark, and tags
mise run release:rollback  # optional: undo local prepare output before anything is pushed
mise run release:push      # push, update release notes, tidy go.sum
```

With jj, `release:prepare` creates the local release shape to review: it
updates the files, describes `@` as `chore(release): vX.Y.Z`, moves the
`main` bookmark to that commit, and creates the root and submodule tags
on it. If you amend the prepared release, rerun `release:prepare` so the
tags are moved onto the amended commit before pushing. With git, those
local commit/tag steps stay deferred to `release:push`, where backing out
a mistaken release commit is more expensive.

Before anything is pushed, `release:rollback` undoes the local prepare
state. With jj it deletes the prepared tags that still point at `@`,
moves `main` back to `@-` if `main` still points at the prepared release
commit, restores the release files from `@-`, and clears the `@`
description. With git it restores the files that `release:prepare`
changed; there are no local release commit or tags yet.

`release:push` is the remote gate and asks for confirmation first. It
accepts the prepared jj release at `@`, or at `@-` when `@` is an empty
working-copy child, and verifies its bookmark and tags before pushing. If
the release is still at `@`, the task creates an empty child before writing
the post-release `go.sum` changes. In git it commits and tags the
already-reviewed prepare output first. It needs two hardware key
touches — one for the release, one for the `go.sum` commit that follows
it. Tag pushes trigger `.github/workflows/release.yml`, which runs
goreleaser with `--release-notes CHANGELOG.md` against
`.config/goreleaser.yaml`.

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
   consumable only after those two are tagged. Under jj, `release:prepare`
   creates these tags locally for review; under git, `release:push`
   creates them just before the remote push.
3. **Tag the submodules**: `jira/vX.Y.Z`, `confluence/vX.Y.Z`,
   `skill/vX.Y.Z`, `frontmatter/vX.Y.Z`, and — after those —
   `wasm/vX.Y.Z`, all on the same commit. The root tag is pushed first
   (GitHub suppresses workflow events when more than three tags arrive at
   once, so the root tag goes alone to reliably trigger Actions); the
   submodule tags follow in a second push, which carries `jira/` and
   `confluence/` alongside `wasm/`, so `wasm/vX.Y.Z` is never resolvable
   before its requirements.

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
  breaks every build between `prepare` and `push` with "unknown revision
  vX.Y.Z". With jj, those pins are part of the described release commit
  you review before pushing.

## How release:push pushes

The pushes are staged, and the order is load-bearing: the `main`
bookmark first, then the root tag **alone**, then the submodule tags.
With jj, the bookmark and tags already exist locally from
`release:prepare`; `release:push` verifies they still point at the
prepared release commit and then only publishes them. With git,
`release:push` creates the release commit and tags immediately before
the staged push.

`jj git push --all` would also publish unrelated local bookmarks or tags.
The task instead uses `--bookmark main`, then `--tag vX.Y.Z`, then one
push with an explicit `--tag` argument for each prepared submodule tag.
Explicit tag pushes also start tracking newly created remote tags. The
root tag stays in its own push because GitHub suppresses the push event
that `.github/workflows/release.yml` listens for when more than three tags
arrive at once — which is exactly how v0.5.0 and v0.6.0 shipped without
goreleaser artifacts.

Afterwards it waits for the proxy, re-tidies `go.sum`, and pushes that
as `chore: update go.sum after vX.Y.Z` — the tags stay on the release
commit, only the branch moves on.

## Recovery steps

- `mise run release:rollback` — before `release:push`, undo the local
  prepare output. In jj this removes the local prepared tags/bookmark
  position/message as well as the file changes; in git it restores only
  the file changes because commit/tag creation is deferred to push.
- `mise run release:post-tidy` — if the proxy was still catching up when
  `release:push` reached the tidy, run this once the version resolves:
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
