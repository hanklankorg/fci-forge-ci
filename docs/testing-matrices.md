# Testing matrices

Three matrices say what Forge CI is proven to do live, on which forge, and
through which consumer. The goal for each is the same: fill every cell that
the product offers, and make an empty cell name its reason (a product gap, a
demo gap, an owner decision or a deliberate exclusion) rather than stay blank.

| Matrix | Rows | Columns | Source of truth | Fills a cell |
|---|---|---|---|---|
| 1 Capabilities | project kind × flow (Maven app, Maven library, npm app/package, Go service/CLI, Cargo app/workspace, Gradle, Android, iOS, monorepo × PR gate, artifact-first snapshot, container-first snapshot, release, full lifecycle); 1b platform capabilities (bundle helper, cache witnesses, classified release, protected-job isolation, Nano chain, lint parity, host delivery) | Forgejo lab · GitLab lab · GitHub hosted | 1: cases files × demo declarations × planner admission with a live-evidence overlay (generated, slice 4); 1b: evidence directories in `testing-parity.md` | one live run on a real consumer whose verdict is read from product-exported contracts |
| 2 E2E scenarios × variants | the whole-flow `lifecycle` event, one row per variant axis value (ecosystem, route, build type, signing, image builder, visibility, architecture, enforcement gates), plus the kept non-lifecycle scenarios | the three forges | cases files × planner admission (generated, slice 4) + evidence | a lifecycle line that ran live, or a refusal whose reason the planner states |
| 3 Demos | folded into matrix 1 (a demo embodies a project kind) | — | — | — |

Principles (owner direction 2026-11-08):

- A whole-flow scenario follows the orchestrators as a consumer calls them;
  the harness requests and observes, it does not sequence.
- Axes are carried by the demo's declaration and consumed by the orchestrator.
  Cases lines carry only what the orchestrator takes as call inputs: forge,
  architecture, signing mode, visibility, image builder, enforcement gates.
  A knob the orchestrator does not consume is the harness simulating the
  product and is not added.
- Expected outcomes are derived by the planner, never written per case.
- A missing axis value is a demo build item (matrix 3), not a harness feature.
- Architecture: amd64 on every forge for now (D-5); arm64 stays cross-compile
  proof, never emulated in a release.
- Single-phase witnesses retire when a lifecycle phase proves the same thing;
  negative/refusal cases, cross-run cache witnesses, the classified route and
  the Lab protected-job qualification stay as their own rows.
- Matrices 1 and 2 are generated from cases files, demo declarations and
  planner admission, with a test that fails when the committed table is stale;
  the live marks are a hand-kept overlay and matrix 1b stays hand-kept, because
  those cells are evidence judgements.

## Snapshot

Matrices 1 and 2 are generated from one source by `just gen-testing-matrices`
(`livetest.RenderTestingMatrices`): the shipped demos' declarations as the
planner and the campaign admission read them, the cases files' lines and
settings, and the live marks of `test/live/campaigns/evidence.txt`, a
hand-kept overlay keyed by forge, demo and flow, since whether a run proved
something is a judgement; `go run ./internal/livetest/preflight
testing-evidence ~/.local/state/forge-ci/live-runs <hosted-runs>` drafts it
for review from the recorded verdicts and from a root of hosted GitHub trial
records, each trial line of a controller or batch script read with its own
exit receipt and finish time. A refusal carries the planner's or the
admission's reason. Matrix 1b is hand-kept. `test/syncguard` fails when the
committed tables are stale. No campaign lane runs on hosted GitHub. There a
line is the fixture workflow a demo ships under `test/live/github/workflows`
for the event the hosted trial settings name, and the lifecycle has one when
the demo ships all three and the hosted settings accept the lifecycle.

Legend: ✅ live-proven · ◐ partial · ⏳ running or queued · ❌ failed, root fix
committed · ⛔ refused by planner admission (reason pinned) · 📝 line exists,
unrun · ⬚ no line · — not applicable.

### 1 Capabilities by project kind × flow (cells: Forgejo / GitLab / GitHub)

Rows are what a consumer is (the demo that embodies it), columns are the
flows the orchestrators offer. "Artifact-first" builds the snapshot image from
the planned artifacts; "container-first" builds from the declared container.

<!-- generated:project-kinds -->

| Demo (project kind) | PR / MR gate | Artifact-first snapshot | Container-first snapshot | Release | Full lifecycle |
|---|---|---|---|---|---|
| hello-android (gradle-android · no container · no image) | ⬚ / 📝 / 📝 | — | — | — / — / — | ⛔ / — / ⛔ |
| hello-cargo-container (rust · container-first · one image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | — | ◐ (fixture consumer proven, run 520; the demo's own lifecycle line awaits a live run) / 📝 lifecycle / admitted | ✅ (2026-11-02) / ✅ (2026-11-02) / 📝 | 📝 / 📝 / ⬚ (admitted, no line) |
| hello-cargo-workspace (rust · container-first · 2 images) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | — | ⛔ / ⛔ / ⛔ | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | ⛔ / ⛔ / ⛔ |
| hello-go-cli (go · no container · no image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | — | — | ✅ (2026-11-02) / — / ✅ (GPG, hosted, run 42490898439 on a copy of the demo, E c-hosted-retry-6b89-20261108T143500Z (matrix 1b, C)) | ⛔ / — / ⛔ |
| hello-go-service (go · container-first · one image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | — | 📝 lifecycle / — / admitted | ✅ (2026-11-02) / — / 📝 | 📝 / — / ⬚ (admitted, no line) |
| hello-gradle (java · no container · no image) | ⬚ / ✅ (2026-11-02) / 📝 | — | — | — / — / ◐ (2026-11-04, L github-gradle-c3b23772: passed, 0 Cosign bundles verified (5 GPG signatures)) | ⛔ / — / ⛔ |
| hello-ios (xcode-ios · no container · no image) | ⬚ / ⬚ / 📝 | — | — | — / — / — | ⛔ / — / ⛔ |
| hello-maven (java · no container · no image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | — | — | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | ⛔ / ⛔ / ⛔ |
| hello-maven-service (java · artifact-first · one image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | ✅ (A run 517, the retired published snapshot) / ✅ (lifecycle phase 3, E gitlab-lifecycle-campaign-22ddcfd9) / 📝 lifecycle | — | ✅ (2026-11-02) / ✅ (2026-11-02) / 📝 | ✅ (E forgejo-lifecycle-campaign-3c178f94) / ◐ (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) / 📝 |
| hello-monorepo (java, node · artifact-first · nested artifacts (monorepo)) | ⬚ / ✅ (2026-11-02) / ⬚ | ⛔ / — / ⛔ | — | — / — / — | ⛔ / — / ⛔ |
| hello-npm-package (node · no container · no image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | — | — | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | ⛔ / ⛔ / ⛔ |
| hello-npm-service (node · artifact-first · one image) | ✅ (2026-11-02) / ✅ (2026-11-02) / ✅ (2026-11-04) | 📝 lifecycle / 📝 lifecycle / admitted | — | ✅ (2026-11-02) / ✅ (2026-11-02) / 📝 | 📝 / 📝 / ⬚ (admitted, no line) |

Outside the live harness's scope, the trial's acceptance and not a product rule:

- hello-cargo-workspace, forgejo: .forge-ci/release-images.yml is not a service snapshot's declaration, so the demo has no development route: the trial judges one container of the root Containerfile and context, planned for amd64, without image name, tag suffix or declared provenance, from at most one artifact
- hello-cargo-workspace, gitlab: .forge-ci/release-images.yml is not a service snapshot's declaration, so the demo has no development route: the trial judges one container of the root Containerfile and context, planned for amd64, without image name, tag suffix or declared provenance, from at most one artifact
- hello-cargo-workspace, github: .forge-ci/release-images.yml is not a service snapshot's declaration, so the demo has no development route: the trial judges one container of the root Containerfile and context, planned for amd64, without image name, tag suffix or declared provenance, from at most one artifact
- hello-monorepo, forgejo: the demo has no root Containerfile, so no service snapshot and no development route
- hello-monorepo, github: the demo has no root Containerfile, so no service snapshot and no development route

<!-- /generated:project-kinds -->

PR/MR gate and release marks for the lab forges come from the private k3s
pairs of 2026-11-02, seeded from their verdicts; the lifecycle and snapshot
marks are the owner's judgements of the evidence roots in `testing-parity.md`.

### 1b Platform capabilities (not per project kind)

| Capability | Forgejo | GitLab | GitHub |
|---|---|---|---|
| Shared bundle helper, default + GPG (C) | ✅ | ✅ | ✅ |
| Focused cache witnesses R / G | ✅ | ✅ | — |
| Classified 11-asset release | — | — | ◐ attempts 1–2; attempt 3 ❌ fixed `b4590d69`, needs new S |
| Protected-job owner (BASE isolation, Lab) | ❌ rounds 1–4, fixes `8ebcea13`, round 5 prepared | same run | — |
| Nano / root → child chain | ⬚ after protected-job | ⬚ | ⬚ |
| Full Gommitlint 81-file | ✅ | ✅ | ◐ 8-file canary |
| Canonical host delivery (option B) | — | — | ⏳ lane 3b |

### 2 E2E scenarios × variants (lifecycle unless stated)

<!-- generated:e2e-variants -->

| Axis | Variant | Forgejo | GitLab | GitHub |
|---|---|---|---|---|
| ecosystem | go | 📝 hello-go-service | — hello-go-cli | ⬚ hello-go-service (admitted, no line) |
|  | gradle-android | ⛔ hello-android | — hello-android | ⛔ hello-android |
|  | java | ✅ hello-maven-service (E forgejo-lifecycle-campaign-3c178f94) | ◐ hello-maven-service (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | 📝 hello-maven-service |
|  | node | 📝 hello-npm-service | 📝 hello-npm-service | ⬚ hello-npm-service (admitted, no line) |
|  | rust | 📝 hello-cargo-container | 📝 hello-cargo-container | ⬚ hello-cargo-container (admitted, no line) |
|  | xcode-ios | ⛔ hello-ios | — hello-ios | ⛔ hello-ios |
| route | artifact-first | ✅ hello-maven-service (E forgejo-lifecycle-campaign-3c178f94) | ◐ hello-maven-service (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | 📝 hello-maven-service |
|  | container-first | 📝 hello-cargo-container | 📝 hello-cargo-container | ⬚ hello-cargo-container (admitted, no line) |
|  | no container | ⛔ hello-android | ⛔ hello-maven | ⛔ hello-android |
| images | 2 images | ⛔ hello-cargo-workspace | ⛔ hello-cargo-workspace | ⛔ hello-cargo-workspace |
|  | nested artifacts (monorepo) | ⛔ hello-monorepo | — hello-monorepo | ⛔ hello-monorepo |
|  | no image | ⛔ hello-android | ⛔ hello-maven | ⛔ hello-android |
|  | one image | ✅ hello-maven-service (E forgejo-lifecycle-campaign-3c178f94) | ◐ hello-maven-service (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | 📝 hello-maven-service |
| signing | GPG declaration | ⛔ hello-go-cli | — hello-go-cli | ⛔ hello-go-cli |
|  | default | 📝 hello-cargo-container | 📝 hello-cargo-container | ⬚ hello-cargo-container (admitted, no line) |
|  | sigstore | ✅ hello-maven-service (E forgejo-lifecycle-campaign-3c178f94) | ◐ hello-maven-service (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | 📝 hello-maven-service |
| image builder | buildah | ✅ lifecycle-forgejo (E forgejo-lifecycle-campaign-3c178f94) | ◐ lifecycle-gitlab (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | ⬚ no lane |
|  | kaniko | 📝 lifecycle-forgejo | 📝 lifecycle-gitlab | ⬚ no lane |
| visibility | public | ✅ lifecycle-forgejo (E forgejo-lifecycle-campaign-3c178f94) | ◐ lifecycle-gitlab (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | 📝 hello-maven-service |
|  | private | 📝 lifecycle-forgejo | 📝 lifecycle-gitlab | 📝 hello-maven-service |
| enforcement | admin lint | 📝 lifecycle-forgejo | 📝 lifecycle-gitlab | ⬚ no lane |
|  | admin vulnerabilities | 📝 lifecycle-forgejo | 📝 lifecycle-gitlab | ⬚ no lane |
|  | admin application-tests | 📝 lifecycle-forgejo | 📝 lifecycle-gitlab | ⬚ no lane |
|  | admin reproducible-builds | 📝 lifecycle-forgejo | 📝 lifecycle-gitlab | ⬚ no lane |
|  | none enforced | ✅ lifecycle-forgejo (E forgejo-lifecycle-campaign-3c178f94) | ◐ lifecycle-gitlab (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | ⬚ no lane |
| architecture | amd64 | ✅ hello-maven-service (E forgejo-lifecycle-campaign-3c178f94) | ◐ hello-maven-service (phases 1–4 run 4, E gitlab-lifecycle-campaign-ec2380ef; phase 5 oracle fixed 4651b638, run 5 failed phase 1 (E gitlab-lifecycle-campaign-4651b638)) | 📝 hello-maven-service |
|  | arm64 | — D-5 | — D-5 | — D-5 |

Kept scenario lines, by lane:

| Lane | Scenario | State |
|---|---|---|
| build-capability-gitlab | `TestGitLabRelease_BuildImageCapability` | 📝 |
| harness-inputs-gitlab | `TestGitLabRelease_ConsumerBranchProtection` | 📝 |
| harness-inputs-gitlab | `TestGitLabRelease_ConsumerTrial` | 📝 |
| harness-inputs-gitlab | `TestGitLabRelease_RejectsPreseededProvenance` | 📝 |
| harness-inputs-gitlab | `TestGitLabRelease_ConsumerInterruptedRecovery` | 📝 |
| interruption-manual-gitlab | `TestGitLabWorkflow_InterruptedJob` | 📝 |
| k3s-private-gitlab | `TestGitLabRelease_MinimalSignedArtifacts` | ✅ (2026-11-07, V 20261107T062007Z) |
| k3s-private-gitlab | `TestGitLabCheck_ChecksOutPrivateConsumerWithJobToken` | 📝 |
| k3s-private-gitlab | `TestGitLabCheck_ChecksOutTheConsumerPipelineCommit` | 📝 |
| k3s-private-gitlab | `TestGitLabPolicy_ConsumerCannotReplaceTheSigningPipeline` | ✅ (2026-11-02, V 20261102T130536Z) |
| k3s-private-gitlab | `TestGitLabPolicy_JobTokenCannotWriteThePublicationRegistry` | ✅ (2026-11-02, V 20261102T130642Z) |
| k3s-private-gitlab | `TestGitLabRelease_MinimalBranchProtection` | ✅ (2026-11-02, V 20261102T130732Z) |
| k3s-private-gitlab | `TestGitLabRelease_ResourceGroupSpansTheChild` | ✅ (2026-11-02, V 20261102T130753Z) |
| k3s-private-gitlab | `TestGitLabRelease_PromotionRetryAfterRegistryWrite` | ✅ (2026-11-02, V 20261102T130918Z) |
| k3s-private-gitlab | `TestGitLabBootstrap_PublishesAndDownloadsTheRecordedBinary` | 📝 |

<!-- /generated:e2e-variants -->

### 3 Demos

Folded into matrix 1: a demo is the embodiment of a project kind, so the
per-demo view is matrix 1's row set. The generator keeps one source for both.
