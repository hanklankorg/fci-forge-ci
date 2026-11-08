<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Opt-in canonical CLI consumption in existing release prerequisites

Base: `4943724be6b9ac5088a517c35cd87b40b5665b27`. This is a bootstrap ordering
patch, **not an executable classified-release leaf or production activation**.
`require-canonical-bootstrap` defaults to false. Existing native commands,
defaults and secret channels are retained. The intentional job-specific read
permission changes below also apply to the native branch: Actions permissions
are static, not conditional on the opt-in boolean.

## Original delivery provenance and local verification

The existing orchestrator's opt-in calls the integrated
`canonical-cli-bootstrap.yml` before plan/config. Its verified artifact ID must
be emitted only after successful receive, and still name the original canonical
build upload. Original build run/attempt, name and engine identity are forwarded
unchanged. They are never replaced with a receiver/summary's current context.
Parse and the subsequent protected workflow calls depend directly on that
producer workflow. Internal consumers receive its result and original outputs,
not dispatch-supplied artifact IDs or arbitrary download URLs.

Each actual receiver checks success, original current run/attempt, explicit full
engine commit/repository, independently computed artifact name and positive ID.
This happens before an engine checkout could default to the consumer repository.
The approved engine is fetched by the existing pinned checkout action, with no
persisted credentials, LFS or submodules. Only then does the workflow invoke
`./_forge_ci_bootstrap/.github/actions/consume-canonical-cli`: this is code from
that **separate, freshly pinned ENGINE checkout**, not a local consumer action.
Consumer checkout has not occurred yet.
Canonical metadata is step-scoped until the opt-in gate validates it; only the
validated nonsecret fields are then exported for subsequent bootstrap steps.
The native job environment, like its command bodies and secret mappings, stays
unchanged when opt-in is false.

The composite contains the fixed native download action and one shared script;
four workflows do not maintain competing transport/verifier implementations.
`canonical-consumer.sh` delegates context, native metadata, inventory and raw-byte
verification to the existing delivery helper, then the existing trusted bootstrap.
Every runner reads fresh artifact metadata with its own job token and downloads
only the original ID. Before verification there is no CLI invocation, including
`--version`, and no private signing environment. The metadata step uses only
`github.token`, never RELEASE_TOKEN, a PAT or a new credential channel.

The authenticated engine checkout is retained in a fresh fixed child under
RUNNER_TEMP (leaving the composite's source in place until it completes) before consumer
checkout can replace the workspace. After successful receive, a private launcher
directory is prepended through GITHUB_PATH. Its `forge-ci` entry invokes the
existing trusted helper with fixed, shell-quoted absolute engine/binary paths on
**every call**; it never selects the SDK's embedded CLI. Privileged Bash mode
ignores startup functions/BASH_ENV for this launcher and the bootstrap action.
These privileged entry points are **not a global environment sanitizer**: the
integrated dependency starts ordinary Bash descendants. A trusted startup
environment, authenticated fresh checkout/object storage and no concurrent writer
remain caller prerequisites. The hostile-SDK PATH tests do not prove isolation
from arbitrary loader variables, exported startup state or a compromised runner.
Bad metadata, missing delivery, wrong hash or unavailable helper exposes neither
launcher nor local completion output and has no installer/latest/PATH fallback.
The two legacy source-install steps in prepare run only when opt-in is false.
Always-run CLI cleanup/summary steps require local verification when opted in.

## Actual covered jobs and permission caps

All newly added permissions are `actions: read`; no workflow-global permission,
write permission, OIDC permission or secret mapping is expanded.

| Workflow | Actual receiver / calling cap | Change |
| --- | --- | --- |
| release-orchestrator | canonical-cli | New key-free canonical call: contents read, actions read |
| release-orchestrator | parse-config, validate-release-tag, release-summary | Add actions read for their own fresh metadata/download |
| release-orchestrator | validate-prerequisites call | Add contents read/actions read explicitly, matching old inherited contents read plus new receiver read cap |
| release-orchestrator | execute-prepare-stage call | Existing actions read/contents write/packages read unchanged; forward original delivery fields |
| validate-release-prerequisites | validate | Add actions read; contents read and private step channels unchanged |
| generate-changelog | generate-changelog | Explicit contents read/actions read replaces inherited contents read |
| release-prepare-stage | prepare-version-files, version-bump | Existing actions read retained; both bootstrap before consumer checkout |
| release-prepare-stage | summarize-prepare-stage | Add actions read; independently bootstrap before reporting CLI |
| release-prepare-stage | generate-full-changelog, generate-minimal-changelog calls | Add actions read to propagate callee cap |
| release-create-github | generate-changelog call | Permission-only actions read cap propagation |
| version-bump | both changelog calls | Permission-only actions read cap propagation |

`self-release:release` already grants actions read to the orchestrator and needs
no change. Existing orchestrator callers already need actions read for native
artifact jobs. External standalone changelog callers must also grant the newly
required read cap, even with opt-in off. The metadata operation is read-only;
some existing job tokens (notably native version-bump) already have other
permissions. This patch does not claim those tokens themselves are read-only.

The top-level opt-in uses existing protected `forge-ci-binary-ref` (must be a full
commit) and runtime inputs (must be qualified digests), not a new tuple model.
For Cargo prerequisites an explicit Rust image may differ; otherwise opt-in uses
the explicit base runtime, never a resolver's mutable Rust tag. Plain preparation
runs on the existing trusted hosted runner, not a newly claimed container. Role
tool availability/qualification remains an administrator prerequisite. No image
must embed this CLI or share its version: delivered canonical bytes are checked
independently. An old Java SDK qualification does not qualify every other tool.

## Dependencies, limitations and review scope

Canonical delivery and transitive preparation are committed in this base. This
candidate consumes those actual files; no dependency is copied from a frozen
candidate or supplied through an overlay. The approved engine commit must contain
the corrected delivery workflow/helper, trusted helper and this consumer glue.
A missing runtime dependency still fails closed. Review readiness of this unit
is not deployment approval: the sole integrator must review the actual merged
source before choosing the approved engine/workflow refs.

The protected caller owns the actual immutable workflow/engine selection and
input wiring. A caller-controlled `canonical-producer-result: success` is not
administrator authorization. Local Git identity checks assume a fresh trusted
checkout and no concurrent writers; they do not authenticate an arbitrary Git
database or protect a hostile runner. Native Actions remain the extraction trust
boundary. GITHUB_PATH integration assumes that protected pre-consumer job state
and qualified runner/image startup environment, not a sandbox for later consumer
execution. Bootstrap after a consumer producer must happen on a fresh runner.

This patch does not wire a classified producer/signer/publisher, change native
packaging roles, or certify every downstream native release workflow. The merged
transitive-preparation gates, exact-ID transfers and original outputs are retained.
The two default-false options remain independent; canonical bootstrap does not
silently enable or waive current-attempt preparation. P/Q request authorization,
policy execution, manifest admission and
purpose-specific private role checks remain their existing owners' obligations.
No new production refs, hashes, approvals, credentials, provider runs or live
acceptance evidence are supplied. Later canonical delivery must verify the actual
combined source's record; no previous-source hash is copied as new evidence.

## Validation scope

Focused graph, original-lineage, native-command/secret/default comparison and
inert preflight-effect tests run offline. The launcher test uses the unchanged
integrated trusted helper, strict verifier, delivery helper and raw-hash owner
with inert bytes. External reviewed-helper overrides are not accepted by the new
consumer tests. No inert bytes are a real canonical CLI
or cryptographic acceptance proof. Missing/stale/foreign metadata, unavailable
dependency and bad/missing bytes expose no CLI, launcher or completion; hostile
SDK PATH is not selected and later launcher calls repeat hash verification.

The ordinary workflow input-contract guard, full workflow guards, Actionlint and
consumer tests use actual source without skips, dependency overrides or overlays.
The native comparison is pinned to the actual base above, not the old d7 source.
All four combinations of current-attempt/canonical opt-in are tested with inert
gate effects, including independently stale preparation and canonical producers.
Docs/sync checks, focused races, ShellCheck/shfmt, configured actual-base delta Go
lint and tracked/untracked diff checks are separate bounded evidence. No canonical
build is needed here: no Go CLI source, embedded docs or build recipe changes.

## Semantic reconciliation versus the scoped-GO d7 candidate

The original `forge-ci-classified-release-leaf` remains frozen. Its source review
GO (`ses_e49094d7cffehmLm3CelNGxSL5`) is not integration approval for this unit.
Only its owned delta was three-way applied to this committed base. Four conflict
regions were resolved by retaining both owners, not by copying old whole files:

| Region | Reconciliation |
| --- | --- |
| generate-changelog producer fields | Keep the merged original run/attempt/upload-ID/name outputs AND the reviewed read-only receiver permissions |
| prepare input declarations | Keep `require-current-attempt` and add the independent default-false canonical option/fields |
| prepare-version-files entry | Keep its original scalar current-attempt gate first, then canonical context/checkout/receive, then consumer checkout; no CLI or private environment precedes verification |
| version-bump entry | Keep its original three-producer scalar gate first, then the same canonical boundary; retain merged exact-ID transfers, patch/changelog checks, and later GPG/SSH setup |

Compared with the reviewed d7 delta, the only production adaptation is preserving
those already merged fields/gates and their order, plus updating an obsolete
queued-dependency comment. Shared action and launcher behavior are unchanged;
shfmt normalizes whitespace in the launcher's argument-condition parentheses.
Tests now locate the canonical boundary after the two known scalar gates, compare
native behavior with 4943724b, require the integrated dependency, and exercise the
independent mode combinations. Documentation removes obsolete dependency blockers
and records these reconciliation/trust limits. No permission expansion beyond the
previously approved job-specific actions-read map is introduced.
