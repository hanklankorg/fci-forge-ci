<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Live test ownership

- `internal/livetest` (relative to this directory): test-only support kit, raw
  reference oracle, embedded probes, support fixtures and remaining kit-local
  tagged scenarios. Its harness
  commands are `preflight`, `demoadmin`, `credentialproxy` and `probejson`.
- `conformance`: shared provider scenarios and their layer fixtures.
- `githubconsumer`: hosted GitHub consumer scenarios.
- `fixtures/consumers`: consumer recipe projects.
- `fixtures/adapters/generic-cli`: actual-source adapters and their offline tests.
- `fixtures/oracles/maven-library`: downstream Java publication oracle.
- `fixtures/services`: disposable service setup, installed-package oracle and inputs.
- `fixtures/development/rust-development`: four-role development receipt fixture.
- `campaigns`: campaign declarations and the hand-maintained evidence overlay.

Ordinary `go test ./...` runs untagged support and fixture tests; the `live` tag
gates scenarios. To compile tagged owners without executing their TestMain or
scenarios, use `go test -tags=live -exec=/bin/true ./test/live/internal/livetest/... ./test/live/conformance ./test/live/githubconsumer ./test/live/fixtures/adapters/generic-cli/...`
from the repository root. This is compilation evidence, not live proof.

Live execution stays behind the guarded launcher in `scripts/ci`; see
[testing](../../docs/testing.md) and [campaign safety](../../docs/live-campaigns.md).
Production planners, adapters and CLI remain under `internal` and `cmd/forge-ci`.
