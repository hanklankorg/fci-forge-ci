#!/bin/bash
# SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
# SPDX-License-Identifier: CC0-1.0

# Protected canonical workflow glue. Transport is the pinned native
# Actions upload/download pair, never a forge-ci command or curl fallback.
set -euo pipefail
export PATH=/usr/bin:/bin
unset FORGE_CI_BIN FORGE_CI_PATH FORGE_CI_BINARY_REF FORGE_CI_PLAN
fail() {
	printf 'canonical-cli-delivery: %s\n' "$1" >&2
	exit 69
}
[[ $# == 1 ]] || fail 'expected build, prepare, admit or receive'
mode=$1
[[ "$mode" == build || "$mode" == prepare || "$mode" == admit || "$mode" == receive ]] || fail 'unknown operation'
[[ "${ENGINE_REPOSITORY:-}" =~ ^[A-Za-z0-9_-][A-Za-z0-9_.-]*/[A-Za-z0-9_-][A-Za-z0-9_.-]*$ ]] || fail 'explicit approved engine repository required'
[[ "${ENGINE_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || fail 'full approved engine commit required'
[[ "${ENGINE_ORIGIN:-}" == "${GITHUB_SERVER_URL:?}/$ENGINE_REPOSITORY" && "$GITHUB_SERVER_URL" =~ ^https://[^/?#@[:space:]]+$ ]] || fail 'engine origin must be on the approved GitHub server'
[[ "${RUNTIME_IMAGE:-}" =~ ^[a-z0-9][a-z0-9./:_-]*@sha256:[0-9a-f]{64}$ ]] || fail 'approved image digest required'
for value in "${GITHUB_RUN_ID:-}" "${GITHUB_RUN_ATTEMPT:-}" "${GITHUB_REPOSITORY_ID:-}"; do
	[[ "$value" =~ ^[1-9][0-9]*$ ]] || fail 'native current-run context is missing'
done
[[ "${GITHUB_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || fail 'native run source commit missing'
: "${DELIVERY_DIRECTORY:=${RUNNER_TEMP:?}/forge-ci-canonical-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT}"
: "${ARTIFACT_METADATA:=$RUNNER_TEMP/forge-ci-canonical-metadata.json}"
[[ "${ENGINE_CHECKOUT:-}" == /* && -d "$ENGINE_CHECKOUT" && "$(realpath -e -- "$ENGINE_CHECKOUT")" == "$ENGINE_CHECKOUT" ]] || fail 'canonical engine checkout path required'
[[ "${DELIVERY_DIRECTORY:-}" == /* && "${RUNNER_TEMP:-}" == /* && "$DELIVERY_DIRECTORY" == "$RUNNER_TEMP/forge-ci-canonical-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT" ]] || fail 'unexpected delivery directory'
artifact_name=forge-ci-canonical-linux-amd64-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT-$ENGINE_COMMIT

# Fresh authenticated checkout/local object storage and protected no-writer
# runner are prerequisites, not facts authenticated by these local assertions.
engine_git() {
	env -i PATH=/usr/bin:/bin HOME=/nonexistent GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_NO_LAZY_FETCH=1 \
		git --no-pager --no-replace-objects -C "$ENGINE_CHECKOUT" "$@"
}
[[ "$(engine_git rev-parse --show-toplevel)" == "$ENGINE_CHECKOUT" && "$(engine_git rev-parse --verify HEAD)" == "$ENGINE_COMMIT" ]] || fail 'wrong engine checkout'
origin=$(engine_git config --local --no-includes --get remote.origin.url) || fail 'missing engine origin'
[[ "${origin%.git}" == "$ENGINE_ORIGIN" ]] || fail 'wrong engine origin'
files=(scripts/ci/engine-binary.sh .engine/bin/forge-ci-linux-amd64.sha256)
if [[ "$mode" != build ]]; then
	files+=(scripts/ci/trusted-bootstrap.sh .engine/bin/lib-forge-ci.sh)
fi
for relative in "${files[@]}"; do
	file=$ENGINE_CHECKOUT/$relative
	[[ -f "$file" && ! -L "$file" && "$(realpath -e -- "$file")" == "$file" ]] || fail "missing/unsafe engine file: $relative (queued trusted-bootstrap integration is required)"
	tree_entry=$(engine_git ls-tree "$ENGINE_COMMIT" -- "$relative")
	[[ "$tree_entry" == '100644 blob '* || "$tree_entry" == '100755 blob '* ]] || fail "not a committed regular file: $relative"
	[[ "$(engine_git hash-object --no-filters -- "$file")" == "$(engine_git rev-parse "$ENGINE_COMMIT:$relative")" ]] || fail "modified engine file: $relative"
done

if [[ "$mode" == build ]]; then
	[[ "${GO:-}" == /* && -x "$GO" ]] || fail 'approved absolute Go compiler required'
	[[ ! -e "$DELIVERY_DIRECTORY" && ! -L "$DELIVERY_DIRECTORY" ]] || fail 'delivery directory already exists'
	mkdir -m0700 -- "$DELIVERY_DIRECTORY"
	/bin/bash "$ENGINE_CHECKOUT/scripts/ci/engine-binary.sh" build amd64 "$DELIVERY_DIRECTORY/forge-ci-linux-amd64"
	/bin/bash "$ENGINE_CHECKOUT/scripts/ci/engine-binary.sh" verify amd64 "$DELIVERY_DIRECTORY/forge-ci-linux-amd64"
	# Capture ORIGINAL producer identity here, never in a later wrapper job.
	{
		printf 'artifact-name=%s\nrun-id=%s\nrun-attempt=%s\n' "$artifact_name" "$GITHUB_RUN_ID" "$GITHUB_RUN_ATTEMPT"
		printf 'engine-repository=%s\nengine-commit=%s\n' "$ENGINE_REPOSITORY" "$ENGINE_COMMIT"
	} >>"${GITHUB_OUTPUT:?}"
	exit 0
fi

[[ "${PRODUCER_ARTIFACT_ID:-}" =~ ^[1-9][0-9]*$ ]] || fail 'one original upload artifact ID required'
[[ "${PRODUCER_RUN_ID:-}" == "$GITHUB_RUN_ID" && "${PRODUCER_RUN_ATTEMPT:-}" == "$GITHUB_RUN_ATTEMPT" ]] || fail 'stale producer: rerun the whole chain in the current attempt'
[[ "${PRODUCER_ENGINE_REPOSITORY:-}" == "$ENGINE_REPOSITORY" && "${PRODUCER_ENGINE_COMMIT:-}" == "$ENGINE_COMMIT" ]] || fail 'producer engine differs from approval'
[[ "${PRODUCER_ARTIFACT_NAME:-}" == "$artifact_name" ]] || fail 'producer name is not the independently computed current-attempt name'
if [[ "$mode" == prepare ]]; then
	[[ ! -e "$DELIVERY_DIRECTORY" && ! -L "$DELIVERY_DIRECTORY" ]] || fail 'delivery directory already exists'
	mkdir -m0700 -- "$DELIVERY_DIRECTORY"
	exit 0
fi

[[ "${ARTIFACT_METADATA:-}" == "$RUNNER_TEMP/forge-ci-canonical-metadata.json" && -f "$ARTIFACT_METADATA" && ! -L "$ARTIFACT_METADATA" ]] || fail 'native artifact metadata missing'
[[ "$(stat -c %s -- "$ARTIFACT_METADATA")" -le 65536 ]] || fail 'oversized metadata'
# Native GitHub metadata only; no new manifest/trust schema or transport client.
jq -e --arg id "$PRODUCER_ARTIFACT_ID" --arg name "$artifact_name" \
	--arg run "$GITHUB_RUN_ID" --arg repo "$GITHUB_REPOSITORY_ID" --arg sha "$GITHUB_SHA" '
	type == "object" and (.id | tostring) == $id and .name == $name and .expired == false
	and (.workflow_run.id | tostring) == $run and (.workflow_run.repository_id | tostring) == $repo
	and .workflow_run.head_sha == $sha
	and (.size_in_bytes | type == "number" and . > 0 and . <= 134217728 and floor == .)
' "$ARTIFACT_METADATA" >/dev/null || fail 'foreign/stale/oversized artifact metadata'
[[ -d "$DELIVERY_DIRECTORY" && ! -L "$DELIVERY_DIRECTORY" && "$(realpath -e -- "$DELIVERY_DIRECTORY")" == "$DELIVERY_DIRECTORY" ]] || fail 'unsafe delivery directory'
shopt -s nullglob dotglob
entries=("$DELIVERY_DIRECTORY"/*)
if [[ "$mode" == admit ]]; then
	[[ ${#entries[@]} == 0 ]] || fail 'download destination is not empty'
	exit 0
fi
binary=$DELIVERY_DIRECTORY/forge-ci-linux-amd64
[[ ${#entries[@]} == 1 && "${entries[0]}" == "$binary" && -f "$binary" && ! -L "$binary" ]] || fail 'delivery requires exactly one canonical regular amd64 file, no sidecars'
[[ "$(stat -c %s -- "$binary")" -gt 0 && "$(stat -c %s -- "$binary")" -le 134217728 ]] || fail 'delivery exceeds canonical binary bounds'
# No helper/CLI process before canonical hash admission. Native Actions may
# restore mode 0644; set executable only after this first byte verification.
/bin/bash "$ENGINE_CHECKOUT/scripts/ci/engine-binary.sh" verify amd64 "$binary"
chmod 0755 -- "$binary"
exec /bin/bash "$ENGINE_CHECKOUT/scripts/ci/trusted-bootstrap.sh" \
	"$ENGINE_CHECKOUT" "$ENGINE_ORIGIN" "$ENGINE_COMMIT" "$binary" -- --version
