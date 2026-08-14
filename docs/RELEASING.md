# Releasing adfast

adfast is a multi-module repository: the root module
`github.com/pmarschik/adfast` and the nested modules
`github.com/pmarschik/adfast/jira`,
`github.com/pmarschik/adfast/confluence`,
`github.com/pmarschik/adfast/skill`, and
`github.com/pmarschik/adfast/frontmatter` (listed in `go.work`). All
release together, from one commit, with matched version numbers.

## TL;DR

```sh
mise run release:prepare   # bump version, regenerate CHANGELOG.md, pin go.mod versions
# review CHANGELOG.md and the go.mod changes
mise run release:tag       # commit, tag root + submodules, push, tidy go.sum
```

Pushing requires a hardware key touch; `release:tag` prompts for it.
Tag pushes trigger `.github/workflows/release.yml`, which runs goreleaser
with `--release-notes CHANGELOG.md` against `.config/goreleaser.yaml`.

## Multi-module release order

Go modules in one repository resolve independently, so the order matters:

1. **Tag the root module first**: `vX.Y.Z`. The root module has no
   intra-repo requirements, so its tag is immediately consumable.
2. **Every submodule go.mod must require that version**: `require
   github.com/pmarschik/adfast vX.Y.Z` in `jira/go.mod`,
   `confluence/go.mod`, `skill/go.mod`, and `frontmatter/go.mod`. The
   pin is written at release time — `mise run release:prepare` runs `go
   mod edit -require` with the bumped version (and strips any intra-repo
   `replace` directives) in every workspace module.
3. **Tag the submodules**: `jira/vX.Y.Z`, `confluence/vX.Y.Z`,
   `skill/vX.Y.Z`, and `frontmatter/vX.Y.Z`, all on the same commit. The
   `release:tag` task creates the tags and pushes the root tag first
   (GitHub suppresses workflow events when more than three tags arrive
   at once, so the root tag goes alone to reliably trigger Actions; the
   submodule tags follow in a second push).

## The unpublished-version window

The submodule go.mod files pin the _upcoming_ version, which does not
exist on the module proxy until the root tag is pushed. Two
consequences:

- **`GOWORK=off` resolution of a submodule alone is expected to fail
  until the root tag `vX.Y.Z` exists** — `go build`/`go test` in
  `jira/`, `confluence/`, `skill/`, or `frontmatter/` with the workspace
  disabled tries to download `github.com/pmarschik/adfast@vX.Y.Z` and
  gets an unknown revision. This is normal before a release; do not
  "fix" it by re-adding a `replace` to a submodule go.mod (released
  module files must be replace-free).
- **Local development builds resolve through the workspace.** `go.work`
  carries a version-specific replace
  (`replace github.com/pmarschik/adfast vX.Y.Z => .`) so that the module
  graph can be computed without fetching the not-yet-published version.
  `release:prepare` bumps it together with the submodule go.mod requires;
  a stale pin here breaks every build between `prepare` and `tag` with
  "unknown revision vX.Y.Z".

## After tagging

`release:tag` finishes the job automatically; the pieces are also
available as manual recovery steps:

- `mise run release:push` — push branch + tags, create/update the GitHub
  release from `CHANGELOG.md` (needed only if `release:tag` was
  interrupted).
- `mise run release:post-tidy` — after the proxy serves the new version,
  re-tidy every module's `go.sum` with `GOWORK=off` (cross-module
  checksums can only be computed against the published version) and sync
  `go.work.sum`. Commit the result as
  `chore: update go.sum after vX.Y.Z`.

## Versioning

- SemVer, driven by Conventional Commits (`git cliff --bumped-version`).
- Breaking API changes must use `feat!:`/`fix!:` (major bump); this is a
  public library — see AGENTS.md for the API stability rules.
- Root and submodule versions stay in lockstep: one release, five tags
  (`vX.Y.Z` + `jira/vX.Y.Z` + `confluence/vX.Y.Z` + `skill/vX.Y.Z` +
  `frontmatter/vX.Y.Z`) on the same commit.
