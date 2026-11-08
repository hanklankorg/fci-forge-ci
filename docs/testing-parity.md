<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Testing parity and proof

Last reviewed: **2026-11-08 UTC**. Documentation base: `3c5be102`;
current bounded-acquisition runtime proof is pinned to `51781c82`, not this
later documentation base. This is a retained-evidence snapshot, not a release
readiness declaration, exhaustive suite audit or production adoption claim.
See [testing conventions](testing.md) for tiers and
[live campaigns](live-campaigns.md) for launch/authorization rules.

## Reading the proof

- **✅ Live-proven**: the named real-forge case passed; only the stated scope.
- **🧪 Offline-tested**: provider-free tests passed; not live acceptance.
- **◐ Partial proof**: a bounded qualification or foundation exists, but the
  complete requested path has not passed.
- **⏳ Pending/unproven**: no qualifying pass. A recorded failure stays a failure.

Every result also needs a scope: **HOSTED** means actual hosted GitHub jobs;
**FORGE-LAB** means real Forgejo/GitLab jobs on local lab infrastructure;
**LOCAL qualification** means local runtime/tool/crypto execution without a
forge; **OFFLINE** means provider-free checks. These scopes are not interchangeable.
In particular, a local real-signature pass is not a hosted release pass, and a
fake-transport or unit pass must never be promoted to ✅ Live-proven.

## Real-forge release cases

Only observed edges are green here. Different consumers, commits and profiles
must not be collapsed into a single all-features provider tick.

| Case / edge | Forgejo | GitLab | GitHub |
| --- | --- | --- | --- |
| Full Gommitlint release inventory and Cosign readback | ✅ Live-proven · FORGE-LAB: 81 names, 26 bundles [E1](#e1-forgejo-full-release) | ✅ Live-proven · FORGE-LAB: 81 names, 26 bundles [E2](#e2-gitlab-full-release) | ⏳ Pending/unproven for this full profile; reduced case below is different |
| Package GPG, final images and negative verification in that GitLab release | ⏳ Not qualified by this row | ✅ Live-proven · FORGE-LAB: 9 GPG sidecars, 2 images/attestations; changed content/signature and wrong key refused [E2](#e2-gitlab-full-release) | ⏳ Not qualified by this row |
| Reduced Gommitlint release | n/a — GitHub-only reduced profile; Forgejo is covered by the full campaign row | n/a — GitHub-only reduced profile; GitLab is covered by the full campaign row | ✅ Live-proven · HOSTED: 8 assets, 4 Cosign bundles, **0 GPG**; 745.19 s [E3](#e3-github-reduced-release) |
| Classified first-profile release: exact 11 public assets, real package and signatures, original producer/signer authority through publication | ⏳ Pending/unproven | ⏳ Pending/unproven | ◐ Partial proof · LOCAL/OFFLINE only; **HOSTED unproven** [E4](#e4-local-source517-qualification) |

The two full releases have matching **name inventories**, not cross-forge
byte identity: engine/consumer commits and produced source revisions differ.
Publishing cross-architecture packages also does not prove native execution on
every packaged OS/architecture. No full Nanolinter DAG, full cold/warm fan-in,
or signed root → parent → child lifecycle is closed by these releases or an old
focused image canary.

## Local qualification and offline boundaries

| Work / scope | Status and actual boundary |
| --- | --- |
| Canonical source517 CLI, LOCAL qualification | ◐ Partial proof: cold build PASSMATCH in 89.174 s; native amd64 binary matches source517's committed SHA-256 [E4](#e4-local-source517-qualification). Not hosted source/request authentication. |
| Exact 11-name classified package/crypto composition, LOCAL qualification | ◐ Partial proof: fresh real producer, nested Debian package, 1 GPG sidecar, 4 real Cosign bundles, changed-subject refusal and extracted binary execution passed with source517. Independent audit checked 72 sealed files and the exact inventory [E4](#e4-local-source517-qualification). Fixture source context is a reviewed uncommitted snapshot, not hosted authority. |
| Bounded exact-ID artifact acquisition seam, OFFLINE | 🧪 Offline-tested: native fake-transport/ZIP tests cover identity, byte/entry limits, malformed archives, cancellation and retry [E5](#e5-offline-acquisition-seam). Caller proof, original producer/signer ID authority and hosted integration remain open. |
| Source anchor, key-before-Q staging, attempt/release oracle and acquisition wrappers | ◐ Partial proof: separate source-review foundations are not an integrated hosted harness. SDK read-only capability does not itself supply an available public tuple, client permission, reviewed original-ID authority adapter or scoped conditional writer. No hosted READY claim. |
| Protected base root/parent/child and CLI contracts | ◐ Partial proof: source contracts are not full live lifecycle or full cold/warm fan-in/DAG acceptance. Full Nanolinter coverage remains open. |

The first-profile eleven names comprise an archive, Debian package and its GPG
sidecar, SBOM, notes, checksums and provenance, with four Cosign bundles. Native
control manifests are not extra public assets. No local proof above establishes
hosted current-attempt/rerun binding, final tag/source authority, publication,
production trust, Cargo, ARM runtime or full 81-file/image parity.

## Ordered E2E work

| Lane | Snapshot outcome / gate |
| --- | --- |
| A: artifact-first snapshot image/service | ✅ Live-proven · FORGE-LAB: public Linux/amd64 Buildah unsigned snapshot, run **517**, controller `063a49d9` / demo `3d5108ad`; native case PASS in 290 s [E6](#e6-e2e-a-native-pass-and-retained-failures). Earlier failures remain failed; the previous `image=true` cause is unresolved. |
| B: development acceptance | ◐ Reviewed source integrated locally at `3c5be102`; representative Forgejo generic/Cargo native proofs are still pending. A's PASS satisfies the prerequisite, not B acceptance [E7](#e7-b-source-integration-and-c-hold). |
| C: release-bundle checks | ◐ Source-only reviewed candidate on `34894e69`, **HOLD / unmerged** as a separate milestone. No C live acceptance [E7](#e7-b-source-integration-and-c-hold). |
| D: linked flow | ◐ Source only: `TestForgejoLayer_ConsumerLifecycle` and `test/live/campaigns/lifecycle-forgejo.cases` (pull request → native merge → service development snapshot → signed release, Forgejo `hello-maven-service`, amd64 only; see docs/live-campaigns.md, Lifecycle). No live pass yet; ordered A → B → C → D gates remain. Whole-file deletion: **NONE**. |

A's latest native cleanup completed without a reported error; no standalone
per-object absence proof was emitted. Its acceptance is the actual run-517 native
verdict, not cleanup or package PASS lines with no matching tests. The older
failed attempts are not reinterpreted as passes or as established causal proof.

## Evidence coordinates and source anchors

Coordinates below are relative to the **configured retained campaign evidence
root**, not repository files or public CI links. Public canonical URLs are not
available for this snapshot. Retention can expire; missing evidence means
downgrade/requalify, not infer success. Do not commit campaign archives, logs,
snapshots, credentials or signer keys alongside this page.

### E1: Forgejo full release

Engine `be6d5a81`, Gommitlint `6ff3549f`;
`forgejo-be6d5a81-6ff3549f.log`, `TestForgejoLayer_ConsumerTrial/public`,
run 515, PASS 683.65 s. Retained inventory:
`forgejo-be6d5a81-6ff3549f/release-3e574f3a32b83f99abcce94c-inventory.json`.
The log records 81 files and 26 verified Cosign bundles.

### E2: GitLab full release

Engine `e53ea2e9`, Gommitlint `92da8403`;
`gitlab-e53ea2e9-92da8403-profile-v2.log`,
`TestGitLabRelease_ConsumerTrial/public`, parent/child pipelines **927/928**,
PASS 1268.72 s. Retained inventory:
`gitlab-e53ea2e9-92da8403-profile-v2/release-b9a2eeef2cada90628f79790-inventory.json`.
The log records the bundle/GPG/image verifications and three passing negative
subcases. Do not substitute `gitlab-e53ea2e9-92da8403-rebuild.log`: that earlier
campaign ended **failed**, so its name is not a reproducibility pass. No
20-file byte-rebuild claim is made without a separately verified receipt.

### E3: GitHub reduced release

SDK/engine `36841f99`, Gommitlint `8ea1fbef`;
`github-release-36841f99-gommitlint-8ea1fbef.log`,
`TestGitHubLayer_ConsumerTrial/public` (whole case 745.19 s). Retained inventory:
`github-release-36841f99-gommitlint-8ea1fbef/release-190a9aa5a4447508e734de01-inventory.json`.
Eight assets/four bundles are not the later classified eleven-name profile.

### E4: Local source517 qualification

Source `51781c8249196a9ffeab5ee6246507cc94e2cf2c`; committed amd64 record in
`.engine/bin/forge-ci-linux-amd64.sha256`:
`db6cf2266dad6251ad1f9f962aba75401d5bd8266305bebe867dd5dd80b08398`.
Read that record at **source517**, not at a later HEAD.
Under `classified-release-runtime-qualification-51781c82/`:

- Cold proof: `context-restore-20261108-20261107T234618Z/qualification-20261108-20261107T234919Z/cold/result.json`.
- Crypto proof: `local-crypto-20261108T002545Z/{review.md,result.json}`;
  workload/launcher exit 0, terminal `PASS_LOCAL_REAL_PRODUCER_GPG_COSIGN`.
- Independent scope correction: `local-crypto-20261108T002545Z-audit-addendum.md`;
  72-file seal SHA-256
  `adf5e781411334518da9d125cdfa3479663d06259e31a6e9444cfc4a6bcf7a91`.

Earlier blocked attempts remain blocked. The audit records one unavailable
metric sample with cause unknown; sampled resource observations are not exact
whole-job peaks. No operator-privacy assurance is inferred from these records.

### E5: Offline acquisition seam

Merged at `51781c82` (implementation `1735f194`), retained
`artifact-cap-integrated-339e6879-m5w5f26g/committed-races-summary.json`
and `committed-races.jsonl` beside it (exit 0, no skips/failures). Source owners:
`internal/domain/provider/exactartifact.go` and
`internal/adapters/github/{exactartifact.go,artifactlimits.go}`; see
[bounded acquisition and its offline verification](github-classified-artifact-acquisition.md).
The caller's classified budgets are 32 MiB compressed/expanded and 128 entries
per artifact, not a whole-job disk cap. Classified leaf/module work at
`76609471`/`8340ef85` is earlier source progress, not hosted acceptance.

### E6: E2E A native pass and retained failures

The accepted public Forgejo case is `TestForgejoLayer_ConsumerDevelopment/public`,
native verdict **passed**, exit **0**, run **517**, in **290 s**. Controller
`063a49d9f01f93dc7f143579ae6b043a4e69da4a`, demo
`3d5108ada3f18792e092d04569721691d0f07974`; native Linux/amd64, Buildah.
Retained coordinates under `e2e-a-live-063a49d9-20261108T015701Z/`:

- `native-runs/20261108T021422Z-1303682/verdict.json` — actual native acceptance.
- `result-summary.json` — source identities, run/jobs and proof boundaries.
- `publication-receipt-from-log.json` — the native collector's logged image
  digest `sha256:ab87227bd1655998fed0586840f4f1bd1b9d0177c783a482ffeeb60b1de7e7f6`.

The producer selected the approved Java image
`ghcr.io/hanklankorg/fc-forge-ci/forge-ci-runtime-java-25@sha256:77654a74eb9a71e062935ac94b333b4d14b4d4dcee59fa6c9d2cc6193a4bcc0d`.
The native case accepted six producer tests, six independent check tests, one
checked JAR, the plan/JAR/receipt/image-config/readiness oracles and unchanged
demo-source modes. Those bodies were checked in memory, not separately persisted
by the existing hooks; actual served workflow bytes were not separately retained.
Cleanup reported no error, not an independent per-object absence receipt.
Crypto-absence, independent scan readback, checked-JAR/image byte equality,
linked flow, cross-forge parity and production approval are not established.

Historical failures remain unchanged. In particular, the old HTTP-422 attempt:

`e2e-snapshot-image-368/live-f40838cf-20261107T174001Z/result.md` and
`native-runs/20261107T174459Z-2178192/{live-run.log,verdict.json}` beneath it.
Actual selected case: `TestForgejoLayer_ConsumerDevelopment/public`, failed
7.85 s, native verdict `failed`, exit 1. No per-job/readiness receipt exists.

Run 516 later reached a producer on v13.2.0 but attempted `image=true`; its root
cause is still unknown. The old selector also returned correct strings in the
cached v12 interpreter. Run 517's PASS does not prove that selector hardening
explained that failure, and neither failed record is overwritten.

### E7: B source integration and C hold

B's four reviewed development-acceptance files are locally integrated in signed
merge `3c5be102bafd8cbbdb7f7753bec003dd28cec9f5`: pure run/artifact predicates,
their complete negative tables and the existing Forgejo generic/container
callers. Current-base helper races, docs/sync, unchanged native snapshot/Maven
regressions and configured lint passed. The live conformance suite was compiled
only; representative generic/Cargo B proofs have **not** been executed here.

The latest reviewed nine-file preparation was based on `34894e69`; its five C
release-bundle/code/docs paths remain unmerged. A's actual PASS removes the A
prerequisite block but does not authorize C integration or establish B/C proof.
The original source-only evidence also remains historical:

`e2e-bc-current-6190579e-20261107/{review.md,test-summary.json,checks.json}`
records offline helper/race, compile-only conformance, lint and docs/sync checks.
That compiled live suite was **not executed**. No source-review GO or compilation
is promoted to live acceptance; C and D remain separate gated milestones.

## Keeping the snapshot honest

When adding a feature or changing a qualified case's outcome, update the row,
scope, source/consumer commits and retained evidence coordinates together.
Record failures, skipped stages and exclusions explicitly. Never turn a unit
pass, successful compilation, old canary, stale green run or source-review GO
into live acceptance. Counts are observations, not a generated coverage score.

This maintainer page and `testing.md` are outside the explicit 27-page CLI embed
list in `docs/docs.go`; this documentation-only change needs no canonical CLI
checksum refresh. Recheck that boundary if the embed list or reachable source
changes. Keep scenario catalogs, schemas and launch contracts in their existing
owners rather than duplicating them here.
