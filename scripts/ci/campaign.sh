#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
# SPDX-License-Identifier: CC0-1.0

# A live campaign: the inputs of one commit, prepared once, which the cases
# of the Go tables then run on (`just test-live-prepared`).
# docs/live-campaigns.md, Run a campaign, shows the use.
#
#   campaign.sh prepare <forge-lab checkout> <dir> <forgejo|gitlab|lane> ... [job-image=<flavor>:<local image> ...]
#   campaign.sh github <dir> <demo>/<event> [setting ...]
#
# prepare builds and publishes the images of this checkout's HEAD with the
# live recipes: the signer and kaniko job images, the Buildah job image
# archive and the archive of each runtime image the named lanes' cases need
# (preflight campaign-runtime-targets). A forge names every lane of it, a
# lane of livetest.PreparedLanes that one. It publishes each job-image=
# nanolinter image too, and saves it as an archive. It exports every demo as
# a one-commit checkout under <dir>/demos/<name>, and writes
# <dir>/campaign.env: KEY=value lines of image references and archive paths,
# no secrets. The file is never sourced.
#
# github runs one cell: the demo's event on hosted GitHub from a one-commit
# checkout of the built-in demo, inside Forge Lab's hosted session, with
# `just test-live-consumer github` settings (org=<id> and the rest). It
# appends the cell's line to <dir>/github.results and exits as the cell did.
set -euo pipefail
umask 077

script_dir=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd -P)
go_bin=${GO:-go}

usage() {
  printf 'usage: %s prepare <forge-lab checkout> <dir> <forgejo|gitlab|lane> ... [job-image=<flavor>:<local image> ...]\n       %s github <dir> <demo>/<event> [setting ...]\n' "$0" "$0" >&2
  exit 2
}

fail() {
  printf 'campaign: %s\n' "$1" >&2
  exit 2
}

preflight() {
  (cd -- "$repo_root" && "$go_bin" run ./test/live/internal/livetest/preflight "$@")
}

prepare() {
  local forge_lab=${1:-} dir=${2:-}
  [[ $# -ge 2 && "$forge_lab" == /* && -f "$forge_lab/justfile" && "$dir" == /* ]] || usage
  shift 2

  # The flavor and image reach command lines, so they are checked first. A
  # lane is a name the preflight knows or refuses.
  local argument flavor image key
  local -a flavors=() lanes=() resources=()
  local -A job_images=()
  for argument in "$@"; do
    case "$argument" in
    build-*=* | compiler-jobs=*)
      resources+=("$argument")
      continue
      ;;
    esac
    if [[ "$argument" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
      lanes+=("$argument")
      continue
    fi
    [[ "$argument" == job-image=*:* ]] || fail "want forgejo, gitlab, a lane or job-image=<flavor>:<local image>, got $(printf '%q' "$argument")"
    argument=${argument#job-image=}
    flavor=${argument%%:*} image=${argument#*:}
    [[ "$flavor" =~ ^[a-z][a-z0-9-]*$ ]] || fail "a job image flavor is lowercase letters, digits and hyphens, got $(printf '%q' "$flavor")"
    [[ "$image" =~ ^[A-Za-z0-9][A-Za-z0-9._/:@-]*$ ]] || fail "not a local image reference: $(printf '%q' "$image")"
    [[ -z "${job_images[$flavor]:-}" ]] || fail "job image flavor $flavor is named twice"
    flavors+=("$flavor")
    job_images[$flavor]=$image
  done
  ((${#lanes[@]} > 0)) || fail 'name what the campaign prepares for: forgejo, gitlab or a lane'

  # Go resolves once before effects; every child inherits the same request.
  # shellcheck source=scripts/ci/image-build-settings.sh
  source "$script_dir/image-build-settings.sh"
  image_build_settings '' image-build-settings-options "${resources[@]}" || return 2

  # What is prepared is HEAD's, so a difference from it would be published
  # under HEAD's name and tested by nobody.
  if [[ -n "$(git -C "$repo_root" status --porcelain --untracked-files=all)" ]]; then
    printf 'campaign: the checkout has uncommitted changes, and the campaign is HEAD without them; commit or stash them first:\n' >&2
    git -C "$repo_root" status --short --untracked-files=all >&2
    exit 2
  fi
  bash "$repo_root/scripts/ci/engine-binary.sh" check
  # A runtime image archive per target the lanes' cases need, read before
  # anything is built so a lane the preflight refuses, one whose case takes a
  # nanolinter flavor not given among them, costs no build.
  local targets
  targets=$(preflight campaign-runtime-targets "$repo_root" "${lanes[@]}" "${flavors[@]/#/flavor=}") || exit 2
  local commit
  commit=$(git -C "$repo_root" rev-parse --verify HEAD)

  # The directory is the campaign's alone: new, empty or an earlier campaign,
  # whose campaign.env goes first, so a prepare that fails leaves none.
  if [[ -e "$dir" && ! -d "$dir/demos" && -n "$(ls -A -- "$dir")" ]]; then
    fail "$dir is neither empty nor an earlier campaign"
  fi
  install -d -m 0700 -- "$dir"
  chmod 0700 -- "$dir"
  rm -rf -- "$dir/campaign.env" "$dir/demos" "$dir/images"
  install -d -m 0700 -- "$dir/demos" "$dir/images"

  local consumer name
  for consumer in "$repo_root"/test/live/fixtures/consumers/*/consumer; do
    name=${consumer%/consumer}
    name=${name##*/}
    bash "$repo_root/scripts/ci/demo-checkout.sh" "$name" "$dir/demos/$name"
  done

  local -a entries=("CAMPAIGN_COMMIT=$commit")
  local value signer_archive=$dir/images/forge-ci-live-signer.tar kaniko_archive=$dir/images/forge-ci-kaniko-job.tar
  value=$(bash "$repo_root/scripts/ci/publish-live-image.sh" kaniko-job "$forge_lab" "$kaniko_archive")
  entries+=("KANIKO_JOB_IMAGE=$value" "FORGE_CI_LIVE_KANIKO_JOB_IMAGE_ARCHIVE=$kaniko_archive")
  value=$(bash "$repo_root/scripts/ci/publish-live-image.sh" signer "$forge_lab" "$signer_archive")
  entries+=("FORGE_CI_LIVE_SIGNER_IMAGE=$value" "FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE=$signer_archive")
  value=$(just --justfile "$repo_root/justfile" buildah-job-image-archive "$dir/images")
  entries+=("FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE=$value")
  # Each as FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_<TARGET>; the scenarios' go image
  # is also FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE.
  local target
  for target in $targets; do
    [[ "$target" =~ ^[a-z][a-z0-9]*$ ]] || fail "not a runtime target: $(printf '%q' "$target")"
    value=$(just --justfile "$repo_root/justfile" runtime-image-archive "$target" "$dir/images")
    entries+=("FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_${target^^}=$value")
    [[ "$target" != go ]] || entries+=("FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE=$value")
  done
  # Each nanolinter job image as JOB_IMAGE_<FLAVOR>, the reference a Forgejo
  # case's CI_JOB_IMAGE takes, and LINT_IMAGE_ARCHIVE_<FLAVOR>, the archive a
  # GitLab case's gate lints in.
  local lint_archive
  for flavor in "${flavors[@]}"; do
    value=$(just --justfile "$forge_lab/justfile" --working-directory "$forge_lab" \
      publish-image --account forgeci "nanolinter-$flavor" "${job_images[$flavor]}")
    lint_archive=$dir/images/nanolinter-$flavor.tar
    podman save --quiet --format docker-archive --output "$lint_archive" "${job_images[$flavor]}"
    key=${flavor^^}
    key=${key//-/_}
    entries+=("JOB_IMAGE_$key=$value" "LINT_IMAGE_ARCHIVE_$key=$lint_archive")
  done

  # Written beside its place, checked as a run will read it, then moved in.
  local staged
  staged=$(mktemp "$dir/.campaign.env.XXXXXXXX")
  printf '%s\n' "${entries[@]}" >"$staged"
  preflight campaign-env-check "$staged" || {
    rm -f -- "$staged"
    exit 2
  }
  mv -f -- "$staged" "$dir/campaign.env"
  printf 'campaign: prepared %s for %s\n' "$dir/campaign.env" "$commit" >&2
}

# github_cell runs one demo's event on hosted GitHub, the demo's fixture
# workflow there, from a one-commit checkout of the built-in demo: the
# GitHub cell of a matrix. It runs inside Forge Lab's hosted session, as
# `just test-live-consumer github` does, and takes its settings. The cell's
# line in <dir>/github.results is its interval, its text, its exit status
# and the path of its verdict.
github_cell() {
  local dir=${1:-} cell=${2:-} checkout output text started status verdict
  [[ $# -ge 2 ]] || usage
  shift 2
  [[ "$dir" == /* && "$cell" =~ ^[a-z0-9][a-z0-9-]*/[a-z-]+$ ]] ||
    fail "a GitHub cell is <demo>/<event>, got $(printf '%q' "$cell")"
  mkdir -p -- "$dir"
  checkout=$(mktemp -d "${TMPDIR:-/tmp}/forge-ci-campaign-demo.XXXXXXXX")
  output=$(mktemp "${TMPDIR:-/tmp}/forge-ci-campaign.XXXXXXXX")
  trap 'rm -rf -- "$checkout" "$output"' EXIT
  bash "$script_dir/demo-checkout.sh" "${cell%/*}" "$checkout"

  text="consumer $cell $*"
  printf 'campaign: %s\n' "$text" >&2
  started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  # tee ignores an interrupt, so the run's cleanup output still has a reader.
  set +e
  just --justfile "$repo_root/justfile" test-live-consumer github "$checkout" "${cell#*/}" "$@" | tee -i -- "$output"
  status=${PIPESTATUS[0]}
  set -e
  verdict=$(sed -n 's/^Live verdict: //p' "$output" | tail -n 1)
  printf '%s/%s\t%s\texit=%s\tverdict=%s\n' "$started" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$text" "$status" "${verdict:--}" >>"$dir/github.results"
  exit "$status"
}

case "${1:-}" in
prepare)
  shift
  prepare "$@"
  ;;
github)
  shift
  github_cell "$@"
  ;;
*) usage ;;
esac
