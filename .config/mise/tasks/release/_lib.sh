#!/usr/bin/env bash
# Shared helpers for release/* tasks. Source this file; do not execute directly.

bold='\033[1m'; cyan='\033[36m'; green='\033[32m'; yellow='\033[33m'; red='\033[31m'; reset='\033[0m'
info()    { echo -e "  ${cyan}${bold}❯${reset}  $*"; }
success() { echo -e "  ${green}✔${reset}  $*"; }
warn()    { echo -e "  ${yellow}⚠${reset}  $*"; }
die()     { echo -e "  ${red}✖${reset}  $*" >&2; exit 1; }
confirm() {
  [[ -t 0 ]] || return 0  # non-interactive: proceed automatically
  echo -e "  ${yellow}${bold}?${reset}  $1 ${bold}[y/N]${reset} \c"
  read -r answer
  [[ "${answer,,}" == "y" ]]
}

# Sets VCS=jj|git and JJ_HEAD (commit id for jj, empty for git).
detect_vcs() {
  if jj root &>/dev/null; then
    VCS=jj
    JJ_HEAD=$(jj log -r @ --no-graph --template 'commit_id' 2>/dev/null || true)
  elif git rev-parse --git-dir &>/dev/null; then
    VCS=git
    JJ_HEAD=""
  else
    die "not in a git or jj repository"
  fi
}

# Fast-forward the main bookmark onto rev (jj only).
#
# Refuses to move it backwards or sideways: main sitting somewhere that is not
# an ancestor of the release means either the release commit is not on main or
# main has moved on since, and dragging the bookmark would paper over both. The
# tags still need pushing either way, so this warns rather than aborting.
advance_main_bookmark() {
  local rev="$1"
  if ! jj bookmark list main 2>/dev/null | grep -q '^main'; then
    jj bookmark set main -r "${rev}"
  elif jj log -r "main & ::${rev}" --no-graph --template 'commit_id' 2>/dev/null | grep -q .; then
    jj bookmark set main -r "${rev}"
  else
    warn "main is not an ancestor of ${rev} — leaving the bookmark where it is"
    return 0
  fi
  success "main → ${rev}"
}

# Populate MONOREPO_MODULES (module paths) and MODULE_DIRS (relative dirs) from go.work.
# Also sets ROOT_MODULE to the root module path.
# Paths whose directory component matches EXCLUDE_GLOB (default: "example/*") are skipped.
discover_modules() {
  local exclude="${1:-example/*}"
  ROOT_MODULE=$(grep '^module ' go.mod | awk '{print $2}')
  MONOREPO_MODULES=()
  MODULE_DIRS=()

  while IFS= read -r entry; do
    local dir="${entry#./}"
    [[ -z "$dir" ]] && dir="."
    # shellcheck disable=SC2254
    case "$dir" in $exclude) continue ;; esac

    local modfile="$dir/go.mod"
    [[ "$dir" == "." ]] && modfile="go.mod"
    local mod
    mod=$(grep '^module ' "$modfile" 2>/dev/null | awk '{print $2}') || continue
    [[ -z "$mod" ]] && continue

    MONOREPO_MODULES+=("$mod")
    MODULE_DIRS+=("$dir")
  done < <(go work edit -json | grep '"DiskPath"' | sed 's/.*"DiskPath": *"\(.*\)".*/\1/')
}
