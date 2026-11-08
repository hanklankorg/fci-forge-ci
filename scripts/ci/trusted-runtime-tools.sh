#!/bin/bash
# SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
# SPDX-License-Identifier: CC0-1.0

# Run only from the admitted immutable engine. These are the approved runtime
# Containerfile's fixed tool directories, never caller PATH or workspace entries.
# This is tool placement, not CLI selection or a global environment sanitizer.
set -euo pipefail
export PATH=/usr/local/go/bin:/usr/local/cargo/bin:/usr/local/bin:/usr/bin:/bin
export RUSTUP_HOME=/usr/local/rustup
# The image's cargo executables/toolchain are shared read-only inputs. Registry
# state belongs to this job's HOME (a fresh private HOME for the product step).
export CARGO_HOME=${HOME:?}/.cargo
[[ $# -gt 0 && "$1" != forge-ci ]] || {
	printf 'trusted-runtime-tools: explicit command required; forge-ci must be verified and absolute\n' >&2
	exit 69
}
exec "$@"
