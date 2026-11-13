#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
# SPDX-License-Identifier: CC0-1.0

# Runs the hosted GitHub cells of a run set, one after another, on the demos
# of a prepared directory: the engine is prepared once, then each cell runs
# `just test-live-consumer github` with the prepared engine reused.
#
#   run-github-cells.sh <prepared-dir> <org-id> <set> [after=<set>] [setting ...]
#
# after=small or after=medium makes it a delta run: the cells and
# visibilities of the set that the set after did not run.
#
# Go owns the cells and their settings (livetest.GitHubCells) and refuses,
# before anything runs, a setting a cell does not take, a lint flavor with no
# nanolinter-image=<flavor>:<reference>, and a release with no
# FORGE_CI_LIVE_GITHUB_RELEASE_TOKEN_FILE. This script reads no credential.
# It starts no hosted session: run it inside Forge Lab's guarded one, as a
# single trial is. A cell is not run again.
#
# The run directory is one of the live launcher's (FORGE_CI_LIVE_EVIDENCE_ROOT,
# default $XDG_STATE_HOME/forge-ci/live-runs): a log for the engine and for
# each cell, and cells.txt with the run set, the set after of a delta run,
# and each cell's line as it is printed.
set -euo pipefail
umask 077

script_dir=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd -P)
go_bin=${GO:-go}
just_bin=${JUST:-just}

if [[ $# -lt 3 || "$1" != /* || ! -d "$1/demos" || -z "$2" ]]; then
  printf 'usage: %s <absolute prepared directory> <organization id> <set=small|medium|full> [after=small|medium] [setting ...]\n' "$0" >&2
  exit 2
fi
prepared=$1 org=$2 set=${3#set=} after=
shift 3
if [[ "${1:-}" == after=* ]]; then
  after=${1#after=}
  shift
fi

# go run gives any refusal status 1; a set or a setting Go refuses is a usage error.
cells=$(cd -- "$repo_root" && FORGE_CI_LIVE_AFTER="$after" "$go_bin" run ./test/live/internal/livetest/preflight github-cells "$set" "$@") || exit 2
[[ -n "$cells" ]] || {
  printf 'live: the %s set%s has no hosted GitHub cell\n' "$set" "${after:+ after $after}" >&2
  exit 2
}

log_root=${FORGE_CI_LIVE_EVIDENCE_ROOT:-${XDG_STATE_HOME:-$HOME/.local/state}/forge-ci/live-runs}
[[ "$log_root" == /* ]] || {
  printf 'FORGE_CI_LIVE_EVIDENCE_ROOT must be absolute, got %q\n' "$log_root" >&2
  exit 2
}
run_dir="$log_root/$(date -u +%Y%m%dT%H%M%SZ)-$$"
install -d -m 0700 -- "$run_dir"
{
  printf 'forge_ci_live_set=%s\n' "${set:-full}"
  [[ -z "$after" ]] || printf 'forge_ci_live_after=%s\n' "$after"
} >"$run_dir/cells.txt"

# trial <engine mode> <visibility> <log> <demo> <event> [setting ...] runs one
# call of the consumer recipe into its log. The visibility is the cell's:
# "both" runs with FORGE_CI_LIVE_CONSUMER_VISIBILITY unset, whatever the
# caller's environment has.
trial() {
  local mode=$1 visibility=$2 log=$3 demo=$4 event=$5
  shift 5
  (
    export FORGE_CI_LIVE_ENGINE_MODE="$mode"
    unset FORGE_CI_LIVE_CONSUMER_VISIBILITY
    [[ "$visibility" == both ]] || export FORGE_CI_LIVE_CONSUMER_VISIBILITY="$visibility"
    exec "$just_bin" --justfile "$repo_root/justfile" test-live-consumer github "$prepared/demos/$demo" "$event" "org=$org" "$@"
  ) </dev/null >"$log" 2>&1
}

# An interrupt reaches the running trial too, which cleans up after itself;
# no further cell starts.
interrupted=0
trap 'interrupted=1' INT TERM

# The engine is prepared once, with the first cell's consumer, which does
# not run: every cell then checks that engine and builds nothing.
IFS=$'\t' read -r visibility demo event words <<<"${cells%%$'\n'*}"
read -ra settings <<<"$words"
printf 'live: preparing the engine; log %s\n' "$run_dir/engine-prepare.log"
if ! trial prepare "$visibility" "$run_dir/engine-prepare.log" "$demo" "$event" "${settings[@]}" || [[ "$interrupted" -ne 0 ]]; then
  printf 'live: the engine was not prepared; no cell ran\n' >&2
  exit 1
fi

# A cell passed when its trial exited clean and its test passed. A trial
# that exits clean with its test skipped, as every visibility not covered,
# ran nothing to judge.
status=0 number=0
while IFS=$'\t' read -r visibility demo event words; do
  [[ "$interrupted" -eq 0 ]] || break
  read -ra settings <<<"$words"
  number=$((number + 1))
  log=$(printf '%s/%02d-%s-%s.log' "$run_dir" "$number" "$demo" "$event")
  verdict=failed
  if trial reuse "$visibility" "$log" "$demo" "$event" "${settings[@]}"; then
    verdict=not-covered
    if grep -q '^--- PASS: TestGitHubLayer_Consumer' -- "$log"; then verdict=passed; fi
  fi
  [[ "$verdict" == passed ]] || status=1
  printf 'FORGE_CI_LIVE_CELL %s %s %s %s %s %s\n' "$verdict" "$demo" "$event" "$visibility" "${words:--}" "$log" | tee -a "$run_dir/cells.txt"
done <<<"$cells"

if [[ "$interrupted" -ne 0 ]]; then
  printf 'live: interrupted after %d cell(s)\n' "$number" >&2
  exit 130
fi
printf 'Live cells: %s\n' "$run_dir/cells.txt"
exit "$status"
