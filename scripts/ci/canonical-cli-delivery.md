<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Protected canonical CLI delivery (initial linux/amd64 unit)

`canonical-cli-bootstrap.yml` is a reusable key-free producer/receiver unit,
not a public release route or integration with the HOLD workflow. It reuses
the existing canonical recipe, not a competing build implementation.
`build-cli.yml` defaults are unchanged; it additionally exposes its original
canonical upload ID and producer run/attempt. Its existing two-architecture,
fixed-name artifacts are NOT interchangeable with this current-attempt unit.
The separate entry point is needed because the legacy reusable workflow has
an OIDC-write job: a conditional legacy job would not establish a read-only
reusable-workflow permission contract. No signing permission is requested here.

## Administrator inputs and deployment dependencies

Call the reusable workflow at an administrator-approved **immutable full pin**.
There is deliberately no invented published coordinate here. Supply:

- `engine-repository`: approved owner/repository on `github.server_url`.
- `engine-commit`: full lowercase 40-character GitHub engine commit.
- `runtime-image`: approved native linux/amd64 image by `@sha256:...`.

These inputs must come from the protected administrator graph, never a PR,
consumer plan, consumer action, or consumer-controlled repository variable.
Format checks are **not** administrator authorization or image attestation.
The workflow image input controls both jobs; this does not prove the host ran
the image honestly. The fixed hosted runner/native Actions service is trusted.

The selected engine commit must contain this delivery glue AND the separately
security-reviewed `scripts/ci/trusted-bootstrap.sh` integration. That helper is
queued separately and is **not copied/reimplemented here**. At base f983f81f it
is absent: selecting that old base fails closed, it is not a usable deployed
coordinate. The eventual integrated full commit and workflow pin require fresh
approval. Neither this candidate nor the Java/amd64 SDK 368 qualification
qualifies the required Go/tooling image. Qualification must establish:

- native linux/amd64, `/bin/bash`, trusted `/usr/bin:/bin` tools including Git,
  `jq`, `gh`, `sha256sum`, `stat`, `realpath`, and native pinned Node Actions;
- `/usr/local/go/bin/go` at the exact Go release in the approved engine's
  `go.mod`, with its modules already available to the canonical recipe offline;
- compatibility with the already pinned checkout/upload/download action versions.

No Go/tool installer, `go install`, SDK binary selection or latest-ref fallback
is added. Checkout uses only the read-only job token and persists no credentials.
Engine source must be accessible to that token or public; this does not invent
a cross-repository private PAT facility. There is no OIDC/signing permission,
private environment, registry login, or publishing credential in either job.
The native artifact service uses its own job-scoped authenticated transport.

## Actual graph and binding

1. `canonical-admit` validates explicit inputs before starting the selected
   image. No legacy build/signing job is called. No consumer checkout occurs.
2. `canonical-build` checks out only the explicit engine commit, without LFS or
   submodules. The existing `engine-binary.sh build amd64` recipe performs the
   pinned-toolchain/offline-module build, then its existing `verify` checks the
   result against **that checkout's** `.engine/bin` record. No CLI runs yet.
3. The original build step records native `github.run_id`/`run_attempt` and
   actual locally checked approved engine identity. Its artifact name is
   `forge-ci-canonical-linux-amd64-RUN-ATTEMPT-FULL_ENGINE_COMMIT`. The fixed
   upload step uploads exactly one canonical file with overwrite disabled.
   Its original artifact ID is forwarded directly through the job output.
4. `canonical-receive`, a separate job with `needs: canonical-build`, checks out
   the approved engine again. It compares ORIGINAL producer run/attempt and
   engine identity to the current approved context; it computes the expected
   name independently. A fresh empty deterministic destination is required.
5. Before downloading, the approved image's `gh api` reads **metadata only**
   using the current read-only Actions token. The admission glue checks native
   artifact ID/name, run ID, repository ID, run source SHA, expiration and
   compressed size (at most 128 MiB). It does not download a ZIP or add a
   transport client/manifest schema. No arbitrary delivery URL is accepted.
6. The existing fixed `actions/download-artifact` pin downloads ONLY
   `needs.canonical-build.outputs.artifact-id` in the current run/repository.
   There is no name, latest, wildcard, cross-run or alternate credential input.
   `merge-multiple: true` gives the single ID's content directly in the fresh
   directory; it does not authorize multiple IDs (admission requires one integer).
7. After native extraction, metadata/context are checked again, the inventory
   must be exactly `forge-ci-linux-amd64`, a non-symlink regular file, and the
   extracted file must be nonempty and at most 128 MiB. No sidecars, nested
   payloads or wildcard executables are accepted. The original engine hash
   verifier checks bytes BEFORE calling the bootstrap helper. Only then is
   executable mode restored (native artifacts do not preserve it).
8. The queued helper repeats origin/full-commit/committed-verifier/checksum
   checks and strict binary verification, then executes that absolute file
   with the fixed inert `--version` command. No hostile SDK/PATH probe occurs.

The native pinned Actions implementation remains the archive extraction trust
boundary. This unit preserves that boundary; metadata limits compressed bytes
and inventory checks limit accepted extracted bytes, but these are **not** an
independent pre-extraction decompression sandbox. No extra ZIP parser or
unverified archive client is introduced. Only the protected canonical producer
may supply the original upload ID. No hosted extraction/tooling qualification
has been run in this offline candidate.

Source identity checks are local assertions under the fresh trusted checkout
prerequisite, not authentication of arbitrary Git databases/HEAD strings.
Protect local configuration/object storage and delivery from concurrent writers
throughout verification/exec. Plumbing uses no status/diff/filter/submodule
inspection, with paging/replacement objects/lazy fetching disabled. Whole-tree
cleanliness belongs to the trusted checkout caller, as in the reviewed helper.
The approved script/checkout action is the entry trust boundary; checking a
script's own source would not independently authenticate an untrusted entry.

## Reruns and later privileged jobs

Current-attempt-only is deliberate. Rerunning only the receiver with old
producer outputs fails; rerun the **whole chain**, not a wrapper that replaces
the original attempt label. There is no re-upload of the verified binary or
new identity assigned by the receiver. The output artifact ID is still the
original producer's ID; a path/verification result from this job is not a
trust token for another runner.

Any later trusted leaf must repeat the original-output context and native
metadata admission, exact-ID native download into an empty directory, inventory,
and checkout/hash/helper verification on its own runner BEFORE any CLI command,
consumer checkout/action or private environment. The `prepare`/`admit`/`receive`
step helper can be reused for that purpose with the original producer outputs.
Do not replace its original run/attempt with values from the receiver/wrapper.
This unit deliberately ends with a verified version probe: request/attempt
authorization, approved signing/release image qualification, consumer staging,
and all full signing/publishing ordering remain separate work.

## Focused offline tests

Run the self-contained `canonicaldelivery_test.go` file with
`FORGE_CI_REVIEWED_BOOTSTRAP` naming the frozen reviewed helper until that queued
dependency is integrated. Tests execute the unchanged reviewed helper and real
strict verifier via a test-only invocation-marker wrapper. Canonical build glue
is exercised through the real recipe with an inert compiler fixture, not a
claim that a real CLI build or image has been qualified. Test selection skips
with an explicit dependency reason if neither integrated nor supplied helper
is available. Static ordering checks cover the actual reusable workflow.
