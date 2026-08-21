# Releasing adfast

adfast is a multi-module repository. It holds the root module
`github.com/pmarschik/adfast` and the nested modules
`github.com/pmarschik/adfast/jira`,
`github.com/pmarschik/adfast/confluence`,
`github.com/pmarschik/adfast/skill`,
`github.com/pmarschik/adfast/frontmatter`, and
`github.com/pmarschik/adfast/wasm`. `go.work` lists all of them. They
release together, from one commit, with matched version numbers.

The release tasks discover the modules from `go.work`. A new module
there is therefore pinned and tagged with no edit to any task.

## TL;DR

```sh
mise run release:prepare   # bump version, regenerate CHANGELOG.md, pin go.mod versions
# review the prepared release commit, changelog, go.mod pins, bookmark, and tags
mise run release:rollback  # optional: undo local prepare output before anything is pushed
mise run release:push      # push, update release notes, tidy go.sum
```

Under jj, `release:prepare` creates the local release shape for you to
review. It updates the files, describes `@` as
`chore(release): vX.Y.Z`, moves the `main` bookmark to that commit, and
creates the root tag and the submodule tags on it. If you amend the
prepared release, run `release:prepare` again, so that the tags move
onto the amended commit before the push. Under git, those local commit
and tag steps stay deferred to `release:push`, where a mistaken release
commit costs more to back out.

`release:rollback` undoes the local prepare state, before anything is
pushed. Under jj it erases the prepared tags that still point at `@`,
moves `main` back to `@-` if `main` still points at the prepared release
commit, restores the release files from `@-`, and clears the description
of `@`. Under git it restores the files that `release:prepare` changed.
No local release commit and no local tags exist yet there.

`release:push` is the remote gate and asks for confirmation first. It
accepts the prepared jj release at `@`, or at `@-` when `@` is an empty
working-copy child, and it makes sure that the bookmark and the tags
still point at that commit before the push. If the release is still at
`@`, the task creates an empty child before it writes the post-release
`go.sum` changes. Under git it commits and tags the already-reviewed
prepare output first. The task needs two hardware key touches: one for
the release, and one for the `go.sum` commit that follows it. A tag push
triggers `.github/workflows/release.yml`, which runs goreleaser with
`--release-notes CHANGELOG.md` against `.config/goreleaser.yaml`.

## Multi-module release order

Go modules in one repository resolve independently, so the order
matters:

1. **Tag the root module first**: `vX.Y.Z`. The root module has no
   intra-repo requirement, so its tag is immediately consumable.
2. **Every submodule go.mod must require that version**: `require
   github.com/pmarschik/adfast vX.Y.Z` in `jira/go.mod`,
   `confluence/go.mod`, `skill/go.mod`, `frontmatter/go.mod`, and
   `wasm/go.mod`. The pin is written at release time. `mise run
   release:prepare` runs `go mod edit -require` with the bumped version
   in every workspace module, and strips each intra-repo `replace`
   directive. `wasm/go.mod` is the one module that requires more than
   the root: it also pins `github.com/pmarschik/adfast/jira vX.Y.Z` and
   `github.com/pmarschik/adfast/confluence vX.Y.Z`, so it becomes
   consumable only after those two are tagged. Under jj,
   `release:prepare` creates these tags locally for review. Under git,
   `release:push` creates them immediately before the remote push.
3. **Tag the submodules**: `jira/vX.Y.Z`, `confluence/vX.Y.Z`,
   `skill/vX.Y.Z`, `frontmatter/vX.Y.Z`, and, after those,
   `wasm/vX.Y.Z`, all on the same commit. The root tag is pushed first,
   and it goes alone to trigger Actions reliably, because GitHub
   suppresses the workflow event when more than three tags arrive at
   once. The submodule tags follow in a second push. That push carries
   `jira/` and `confluence/` next to `wasm/`, so `wasm/vX.Y.Z` is never
   resolvable before its requirements.

## The unpublished-version window

The submodule go.mod files pin the _upcoming_ version, and that version
does not exist on the module proxy until the root tag is pushed. This
has two consequences:

- **`GOWORK=off` resolution of a submodule alone fails until the root
  tag `vX.Y.Z` exists.** With the workspace disabled, a `go build` or a
  `go test` in `jira/`, `confluence/`, `skill/`, `frontmatter/`, or
  `wasm/` tries to download `github.com/pmarschik/adfast@vX.Y.Z` (and,
  for `wasm/`, the `jira/` and `confluence/` tags too) and gets an
  unknown revision. This is normal before a release. Do not "fix" it
  with a `replace` back in a submodule go.mod, because a released module
  file must be replace-free.
- **A local development build resolves through the workspace.** `go.work`
  carries a version-specific replace for every monorepo module that some
  go.mod requires: `replace github.com/pmarschik/adfast vX.Y.Z => .`
  plus, because `wasm/` requires them, `…/confluence vX.Y.Z =>
  ./confluence` and `…/jira vX.Y.Z => ./jira`. The module graph can then
  be computed without a fetch of the not-yet-published versions. The
  `use` list of the workspace is not enough on its own: a load of the
  graph still reads the go.mod of every required version, so a submodule
  that a sibling requires needs a replace of its own.
  `release:prepare` bumps all of them together with the go.mod requires,
  and adds a line for a module that has newly become a sibling
  requirement. A stale or missing pin here breaks every build between
  `prepare` and `push` with "unknown revision vX.Y.Z". Under jj, those
  pins are part of the described release commit that you review before
  the push.

## How release:push pushes

The pushes are staged, and the order is load-bearing: the `main`
bookmark first, then the root tag **alone**, then the submodule tags.
Under jj, the bookmark and the tags already exist locally from
`release:prepare`. `release:push` makes sure that they still point at
the prepared release commit and then only publishes them. Under git,
`release:push` creates the release commit and the tags immediately
before the staged push.

`jj git push --all` would also publish unrelated local bookmarks or
tags. The task uses `--bookmark main` instead, then `--tag vX.Y.Z`, then
one push with an explicit `--tag` argument per prepared submodule tag.
An explicit tag push also starts to track a newly created remote tag.
The root tag stays in its own push, because GitHub suppresses the push
event that `.github/workflows/release.yml` listens for when more than
three tags arrive at once. That is exactly how v0.5.0 and v0.6.0 shipped
without goreleaser artifacts.

Afterwards the task waits for the proxy, re-tidies `go.sum`, and pushes
that as `chore: update go.sum after vX.Y.Z`. The tags stay on the
release commit, and only the branch moves on.

## Recovery steps

- `mise run release:rollback` — undo the local prepare output, before
  `release:push`. Under jj this removes the local prepared tags, the
  bookmark position, and the message, as well as the file changes. Under
  git it restores only the file changes, because the commit and tag
  creation is deferred to the push.
- `mise run release:post-tidy` — if the proxy was still catching up when
  `release:push` reached the tidy, run this task once the version
  resolves. It re-tidies the `go.sum` of every module with `GOWORK=off`,
  because a cross-module checksum can only be computed against the
  published version, and it syncs `go.work.sum`. Commit the result as
  `chore: update go.sum after vX.Y.Z`.

## Versioning

- SemVer, driven by Conventional Commits
  (`git cliff --bumped-version`).
- A breaking API change must use `feat!:` or `fix!:` (major bump). This
  is a public library. Read AGENTS.md for the API stability rules.
- The root version and the submodule versions stay in lockstep: one
  release, six tags (`vX.Y.Z` + `jira/vX.Y.Z` + `confluence/vX.Y.Z` +
  `skill/vX.Y.Z` + `frontmatter/vX.Y.Z` + `wasm/vX.Y.Z`) on the same
  commit.
