<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Live-scenario refactor: source and acceptance handoff

2026-11-09. This is a migration disposition and acceptance handoff, **not a
second scenario registry**. Declarations, existing campaign cases, product
planners and the [testing matrices](testing-matrices.md) remain authoritative.
Source checks cannot create live evidence or discharge required native proof.

## Source boundary and phase status

The baseline is commit `afd06f71f395bf101ef57de8d31926f56c0c05e7`, tree
`74ec3b04bb08ff290aad208cdd07fe7c162f81f9`. It supplies the existing campaign
selectors and lifecycles; it is not the older root checkout or a feature-lane
successor. Structural source boundaries, in order:

| Boundary | Tree | Meaning |
|---|---|---|
| Consumer fixtures | `56c9185a3f69fbead7660a9dabee92ae8cc0fd4a` | Atomic consumer-root move and exporter regression |
| Native entrypoints | `bc7a155b8a1f00276596e106daa6d97906648c53` | Both native scenario packages moved intact |
| Support kit | `b4911093f1447565bf3201c8e1fcee8148e36d85` | Internal ownership, callers and transferred assertions |
| Reviewed discovery corrections | `825c5655ad0ac7643046f4f5a7565536253c3811` | Same support tree plus only `justfile` comments, `docs/testing.md` and `docs/live-campaigns.md` |

The subsequent integrated boundary `da6706b54f43a93c1deb0276715a166997183119`
retains the reviewed discovery/help and Phase 1 mapping corrections. This
handoff integrates the independently GO-reviewed fixture slice
`5b5618c9d7d9e2cc580394c0bd5b7e8e9836bb0a` (reviewed against native-entrypoint
boundary `bc7a155b8a1f00276596e106daa6d97906648c53`) onto those newer support
owners, not by replacing them with older files. The integrated source is now
frozen in signed/DCO local commit
`af9da105f9f86ef8a0d604e2a7f40e569e13610a`, tree
`258a56e06d2ff0f1e3c989112eb6a75b8d0f0a00`. Independent combined review,
the full ordinary untagged race suite, maintained canonical binary refresh/check
(no checksum delta), and integration/smoke/live-tag compile/vet passed on that
boundary. Signature verification and clean source freeze were recorded by the
parent session; these are source checks, not native acceptance. No push or tag
is implied. Earlier staged/uncommitted identities above are historical boundaries.

The Phase 6 reconciliation below compares the original baseline with **signed
`af9da105` only**. The parent subsequently reported independent Phase 5 helper
integration review GO and pure checks PASS. That combination is not audited
here, has no final tree identity in this handoff, and is not signed qualification
input.
These documentation updates likewise do not change the frozen native source.

- **Phase 1: COMPLETE for mapping/disposition, independently reviewed.** The
  recursive inventory, exact first-slice/Maven coverage baseline,
  every original matrix row and all audited requirement IDs have a current
  scenario/assertion/input owner or an explicit gap, refusal, exclusion or
  unresolved evidence decision in the appendices. Missing native proof and
  historical conflicts are recorded decisions, not silently excluded rows.
- **Phase 2: SOURCE READY; NATIVE QUALIFICATION FAILED, not accepted.** Existing
  selection, exporter, admission, native orchestration and assertions now have coherent
  source owners. No parallel pilot driver or case parser was added. The full
  done criterion still requires passing native proof on this source. The approved
  attempt passed PR/merge but failed development before Maven build; blocked
  downstream phases and release were NOT RUN. No retry was performed.
- **Phase 3:** source/discovery mapping ready on all three roads; **native
  acceptance remains outstanding and provider restrictions still apply**.
  Commands and prerequisites are not provider passes or launch authorization.
- **Phase 4: SOURCE INTEGRATED; combined independent review GO.** All
  40 fixture members retain suffixes/modes, eight have reviewed path/depth edits,
  and the generic-CLI support import retains its new internal owner. Guard roots,
  references and licensing are merged with the support relocation. This is not
  native acceptance or a claim that all phases 2–4 are complete.
- **Phase 5: helper integration independently reviewed GO outside the signed freeze.** Previously
  reviewed isolated helper tree `e783a04f407033eceba302ad2b3e15fcd6285a2e`
  was adapted to current support owners in a separate worktree. The parent reports
  integration review GO and pure checks PASS; **final tree not audited here**.
  Primary `af9da105` is unchanged. Helper code is not natively qualified; source
  review does not complete image-graph/specialized native acceptance.
- **Phase 6: signed-source inventory/retirement reconciled locally; residual
  guard disposition, final combined-source identity/accounting and acceptance
  remain open.** No
  further unique code, scenario or case retirement is justified. Neither this
  audit nor source readiness declares full phase completion or native PASS.

## Exhaustive recursive group dispositions

Counts are original tracked files, not scenarios or assertions. Each `/**` row
applies to every descendant, including hidden files, lockfiles, tests, embedded
data, commands and modes; unchanged production owners are outside these moves.

| Original recursive group | Current owner / planned destination | Disposition |
|---|---|---|
| `test/live/demos/**` (253) | `test/live/fixtures/consumers/**` | MOVE complete: same relative suffix and executable modes; initial move byte-identical, subsequent README support-path references updated; consumer payloads unchanged, no old-root fallback |
| `internal/livetest/conformance/**` (84) | `test/live/conformance/**` | MOVE; retain every native, CLI/service, negative, recovery and offline fixture assertion; consumer-path references adjusted |
| `internal/livetest/githubconsumer/**` (6) | `test/live/githubconsumer/**` | MOVE; retain hosted authority, fixture profiles and session/cleanup distinctions |
| All other `internal/livetest/**` (255) | `test/live/internal/livetest/**` | MOVE/REFACTOR rooted readers and imports; retain commands, rawref, probes, testdata and kit-local tagged scenarios, not production code |
| `test/live/consumer-fixtures/generic-cli/**` (17) | `test/live/fixtures/adapters/generic-cli/**` | MOVE integrated; KEEP distinct actual-source adapters, self-rooting/checksource callers and relocated support import |
| `test/live/consumer-fixtures/maven-library/**` (1) | `test/live/fixtures/oracles/maven-library/**` | MOVE integrated; KEEP byte-identical Java downstream oracle, not an application fixture |
| `test/live/cases/maven-issuer-service/**` (1) | `test/live/fixtures/services/maven-issuer-service/**` | MOVE integrated; KEEP byte-identical disposable issuer runtime setup |
| `test/live/cases/npm-rest-service/**` (3) | `test/live/fixtures/services/npm-rest-service/**` | MOVE integrated; KEEP installed-package oracle, operator inputs and failure history; reference edits only |
| `test/live/cases/rust-development/**` (18) | `test/live/fixtures/development/rust-development/**` | MOVE integrated; KEEP four-role image-byte/source receipts; README path/depth edits only |
| `test/live/campaigns/**` (12 `.cases` + `evidence.txt`) | Same owner | KEEP all selections and historical evidence; no new grammar, scheduling retirement or evidence regeneration |
| `.github/**`, `.forgejo/**`, `templates/**`, production `internal/**`, `cmd/**` | Same production owners | KEEP native authority and behavior, including planners/adapters; no production implementation move |
| `scripts/ci/**`, guards, configuration, licensing and docs referencing those groups | Same owners with current paths | REFACTOR references only; launcher supervision, signals, scopes and cleanup retained |

The five consolidated fixture groups total **40** files, now at the destinations
above with no old-root fallback. Embedded probes/templates remain beside their
Go owner rather than being copied into a
second fixture catalogue. Shipped consumers remain all twelve: Android, Cargo
container/workspace, Go CLI/service, Gradle, iOS, Maven/plain and service,
monorepo, npm package/service. All receive KEEP behavior disposition.

Two assertions crossed the new Go internal boundary without being dropped:
`TestRunLiveTests_HeaderInputsAreTheVerdictsInputs` moved from
`scripts/ci/livescript_test.go` into the kit's `liveinputs_internal_test.go`;
`TestTestingMatricesInSync` moved from `test/syncguard/testingmatrices_sync_test.go`
into the kit's `testingmatrices_internal_test.go`. Launcher tests now exercise
the maintained preflight CLI's **`classify-run` only**, replacing the obsolete
`TestLiveClassifierProcessFixture` subprocess shortcut. Both classifier parent
assertions and package-separation cases remain. This is not retirement of a
native witness. Ordinary untagged checks still own both transferred assertions.

## Signed-source retirement and inventory reconciliation

This audit uses immutable baseline `afd06f71` → signed `af9da105`, not a pending
combined tree. All **638 original mapped members** (253 consumer, 90 native,
255 support, 40 other fixture files) survive at the recursive destinations
above with suffixes and Git modes preserved. All 14 embedded live-suite payloads
and every mapped Go build/embed directive retain identity. Repository build
constraints remain 91 `live`, 66 `linux`, 36 `integration`, eight `unix`, six
`!linux`, four `smoke` and one `!unix`; no tagged scenario became ordinary proof.

Static column-zero Test declarations change **5806 → 5807**, excluding 18
`TestMain` and 19 fuzz declarations in both trees (5843 → 5844 total records).
The two exporter regressions are additions; the classifier subprocess shortcut
is the one intentional removal, for the real-CLI-backed reason above. Both
transferred assertions remain ordinary tests. Conformance retains 114 Tests,
hosted 15, and support grows 493 → 495 through those two transfers. Counts are
source declarations across all tags/platforms, not executed tests or assertions.

All **51 original suffix-aware HAR/PAR IDs and 89 occurrences at their original
owners** survive. Added explanatory mappings increase whole-tree mentions, not
requirements or proof. Appendices A–C continue to map the existing matrix's
12 project kinds × five flows (three provider cells each), 27 axis/value rows,
seven platform families, 18 kept scenario rows and 51 requirement IDs. They are
not another authoritative registry. All 12 campaign files (93 noncomment input
lines, including settings) are byte/mode-identical. All 56 evidence data rows
are unchanged; only their maintained command comment moved. Both generated
matrix blocks retain exact bytes; the existing generator and
`TestTestingMatricesInSync` remain their sole synchronization mechanism.

**Retirement decision: retain every unique witness.** Standalone PR, snapshot
and release preserve event/input-specific admission and output checks;
multi-image and library/package roads cannot be replaced by one-service
lifecycle success. Refusal/security, interrupted recovery, cold/partial/warm
cache, private-read/private-image and protected-job witnesses preserve distinct
authority, prior state, readback and cleanup contracts. Only the obsolete test
transport shortcut above was replaced; no native witness or campaign row was
removed. No permanent dual driver, compatibility fixture root, settings parser
or scenario catalogue was introduced. Further retirement requires reviewed
assertion/input equivalence and applicable replacement proof, not similar names.

Old active roots are absent. Whole tracked-tree searches include hidden tool
configuration, imports, scripts, maintained commands and compound literals.
Remaining apparent old-root references are explained, not fallback discovery:

- The recursive disposition table intentionally names historical source paths.
- `internal/livetest` in the live README is relative to `test/live`, not the
  removed repository-root package.
- The successor narrows lint globs to `**/test/live/internal/livetest/**` and its
  `rawref` child, preserving current kit layering and raw-oracle independence
  without exempting a hypothetical reintroduced root-internal live kit.
- `TestReleaseEvidenceRejectsCheckoutDestinations` constructs a disposable
  `internal`/`livetest` directory under a fake Git root to test ancestor-based
  checkout refusal. It neither reads nor discovers the removed source root.
- The successor removes the dormant root-internal `livetest` test-layer exemption
  from `test/archguard/layers_test.go`. Its existing classifier and run-context
  table cases now require utility-layer enforcement for that old name. They
  remain regressions against a reintroduced exemption, not a supported legacy
  source owner or a migrated current-package guard witness.
- Exported campaign `<prepared-dir>/demos` and consumers' own `test/live/github`
  or GPG workflow profiles are maintained runtime interfaces, not legacy source
  roots. Historical evidence coordinates and authenticated regression fixtures
  remain historical; they must not be modernized into new acceptance claims.

No active old-root reader/fallback or source-preservation loss was found at
`af9da105`; the successor resolves its dormant guard-category exception as above.
This immutable-source conclusion excludes the concurrent helper integration. The
final combined tree must receive its own identity/accounting and retain its
reported review/affected checks; it must not replace `af9da105` as the source
identity for the failed native attempt.

## Maven application pilot and no-loss owners

The pilot is exactly **`hello-maven-service`: PR → native merge → unsigned
development image snapshot → stable release**, in one native consumer history.
Its declaration is
[`artifacts.yml`](../test/live/fixtures/consumers/hello-maven-service/consumer/.forge-ci/artifacts.yml).
It is an artifact-first Maven application JAR plus one runtime image. Bare
`hello-maven` is also an application, not the actual Maven library adapter: it
has no container/service development route and is refused for this lifecycle.
Its standalone PR/file-release witnesses stay. No Maven Central or Maven
SNAPSHOT package publication is implied by the service image snapshot.

| Contract | Existing source/assertion owner; preserved boundary |
|---|---|
| Exported bytes, modes, one clean commit, nonempty-destination refusal | `scripts/ci/demo-checkout.sh`; `demo_checkout_test.go` invokes the real exporter for all twelve consumers |
| One saved selection, no alternate pilot cases | `campaign.go`, `campaignlifecycle.go`, `campaignsettings.go` and their tests in `test/live/internal/livetest`; existing lifecycle case files |
| Admitted source and declaration, private-index overlays and rendering | Kit `consumer*`, `servicesnapshot*`, `servicedevelopment*`, `gitlabconsumer*`, `githubstage*`; product pipeline/snapshot planners remain owners of expected outputs |
| Forgejo PR and merge | `test/live/conformance/consumerlifecycle_test.go`: PR/run/source checks; no release/tag side effects; `MirrorRepo.RequireMergeOf` checks parents/tree |
| Snapshot payload/source/digest | Shared kit `snapshotoracle.go`, `RequireSnapshotImages`; conformance `servicedevelopment_test.go`; producer checks and frozen plan/receipts/registry readbacks |
| Release trust and bytes | Conformance `layerconsumertrial_test.go` shared `requestRelease`; request/commit/tag signing, offered bundle subjects and authenticated checksum subjects; `releaseimageoutputs_test.go` is the separate opt-in complete declared-image oracle, not inferred from green publication |
| Native release differences | `gitlabconsumerlifecycle_test.go` + `gitlabrelease_test.go`; hosted `consumertrial_test.go` + `lifecycle_test.go`; retain archives/policy/child pipelines versus hosted credentials/SDK and native assets |
| Security sequencing | Forgejo release deploy key/secrets introduced only after development; GitLab protected scoped publisher/release variables with early bot key for protected main; hosted release secrets delayed to release. PR builder setup is not evidence of an image build |
| Final inventory | Forgejo branches/three runs/two tags/one release/bot deploy key; GitLab parent/child-aware pipeline inventory; hosted run/tag/release counts. Do not normalize their different oracles |
| Cleanup and interruption | Kit `scratch*`, `layer.go`, `context*`, launcher signal tests; hosted `main_test.go`, kit `githubdevelopment.go` exact snapshot-package registration/identity checks before dispatch. Keep bounded cleanup, evidence-before-deletion and errors |

Forgejo lifecycle additionally asserts service readiness against the published
digest. GitLab and hosted lifecycle do **not** assert application HTTP readiness.
Standalone snapshot-only roads assert planned image receipts/source/digests, not
readiness even for one image. Teardown success and final pre-teardown inventory
are not independent per-resource absence receipts. Stable hosted release packages
can remain for the authorized organization sweep; snapshot package cleanup and
kept-demo administration have different authority.

Existing exporter, selection/admission, source-preservation and render regressions
cover the moved pilot paths; no uncovered source/no-loss path was identified that
justifies duplicating those tests merely to tick a phase. Native proof remains
the missing layer, not a missing second driver.

## Three maintained native roads

Use the [consumer command chooser](testing.md#trying-a-consumer),
[lifecycle contracts](live-campaigns.md#lifecycle),
[snapshot contracts](live-campaigns.md#snapshot) and
[single-cell selection](live-campaigns.md#rerun-one-cell).
These links own settings/defaults; this page does not copy a settings table.
For standalone trials, use an absolute clean independent consumer Git root,
exported with `bash scripts/ci/demo-checkout.sh hello-maven-service <empty-dir>`;
do not pass the nested recipe in the engine Git checkout. Campaign preparation
exports its own consumer checkouts.

The following commands describe the maintained roads, **not executed by this
audit**. Only the bounded Forgejo cell has separate launch-scope approval;
the approved attempt failed and does not establish native acceptance:

| Road | Maintained Maven-service lifecycle command | Prerequisites, owner and evidence distinction |
|---|---|---|
| Forgejo | `just campaign-lane <prepared-dir> forgejo test/live/campaigns/lifecycle-forgejo.cases hello-maven-service/lifecycle/image-builder=buildah/visibility=public` | Java digest-pinned nanolinter job image, signer and container/readiness runner; conformance `TestForgejoLayer_ConsumerLifecycle`; provider-scoped launcher log plus classifier `verdict.json` |
| GitLab | `just campaign-lane <prepared-dir> gitlab test/live/campaigns/lifecycle-gitlab.cases hello-maven-service/lifecycle/image-builder=buildah/visibility=public` | Prepared Java job/build, signer/lint and selected-builder archives; pinned policy and guarded release runner profile; `TestGitLabLayer_ConsumerLifecycle`; native child-pipeline evidence, no readiness; live hold remains |
| Hosted GitHub | `just campaign-lane <evidence-dir> github hello-maven-service/lifecycle org=<authorized-id>` | One guarded hosted session, frozen prepared SDK/images reused, separate private release PAT file and all three fixture workflows; `TestGitHubLayer_ConsumerLifecycle`; retained per-cell output/exit/run IDs/cleanup, **not** a lab classifier receipt |

For Forgejo keep the global builder selector unset: whole-file admission sees
both Buildah and Kaniko before selecting a cell. GitLab lane builder must agree
with the selector (unset defaults Buildah). A selector does not bypass whole-file
source/settings/archive admission. Both existing lifecycle files also contain
other consumers; selecting one cell does not qualify those consumers.

Single-phase requests remain maintained, not retired: local `pull-request`,
`snapshot`, `release` dispatch through `just test-live-consumer <forge> <checkout>`
with the documented native inputs; hosted uses `pull-request`, **`dev-release`**,
`release`, never literal `snapshot`. The checked-in snapshot campaign files have
no Maven-service row: use its standalone snapshot command or the lifecycle, not
an invented selector. GitLab lifecycle does not accept old `version-bump=maven`.
Hosted `image=<declared-name>` selects the optional release-image oracle; do not
claim image verification merely from release success.

All roads retain [scope/lock and credential admission](live-campaigns.md#execution-scopes-and-locks),
[hosted prepare/reuse invariants](validation-invariants.md#1-freeze-the-engine-before-parallel-consumers)
and [shared-resource coordination](live-campaigns.md#shared-dependencies).
No GitLab hold, hosted organization restriction, protected-job or classified lane
is resumed. No cross-provider proof substitution is permitted. Native scope is
Linux/amd64; arm64 cross-build assets are not native proof, and the iOS macOS
gate exception does not broaden release scope.

## Requirement families and explicit gaps

This is a pointer map to the existing requirements, not new IDs or coverage marks.
The [project-kind and variant rows](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github),
[platform families](testing-matrices.md#1b-platform-capabilities-not-per-project-kind)
and [dated parity evidence](testing-parity.md) retain their original status.

| Existing family | Retained source and gap/acceptance boundary |
|---|---|
| Project kinds, routes, flows and axes (matrices 1/2) | All twelve consumer declarations, existing cases/admission and native entrypoints above; no dedicated Go/Rust library downstream-publication fixture/capability, npm workspace or Maven reactor row. Actual Maven library adapter/oracle is distinct from plain Maven app |
| PAR-ART, PAR-RUN, PAR-CHK, PAR-PKG | Conformance artifact/inrunner/checkout/forgepackages/mavenpublications tests; native byte handoff, output consumption and registry resolution are not rendered YAML or tarball-release proof |
| PAR-SIGN and signing profiles | Conformance sign/keyless/basesign, release bundle checks and hosted signing/profile/boundary tests; key/GPG/keyless/KMS/distro/attestation methods remain distinct, no-Rekor lab policy retained; optional readbacks are not universally asserted |
| PAR-REG, PAR-REL | Registry/ledger/rollback/release/assetretention plus release-image owners; preserve candidate journal, prior-tag restoration, exact asset subjects/inventory and no-write negatives, not just lifecycle success |
| PAR-CAP, PAR-TOK, PAR-UX | Capability/degradation/token/error-shape/JSON/weburl/dry-run witnesses; preserve unavailable versus unsupported versus refused and actual no-effect assertions |
| HAR-01–11 | Existing CLI black-box harness stays isolated; HAR-05 forbids real endpoints there. Mocks/source checks cannot become live marks |
| Enforcement, protected jobs, private access and recovery | Administrator wrapper tests, GitLab policy/serialization/preseed/interruption/promotionretry, privateaccess/privateimage; retain effective-job authority and recovery checkpoints. Inert policy canary is not protected-runner qualification |
| Cache/base/flavor/image graphs (matrix 1b) | `layerimageplan_test.go` cold/partial/warm siblings and final companion; `gitlabbases_test.go` signed cold/warm reuse; separate base/final namespaces. Full Nano signed root→parent→child/fan-in remains open; runtime job images are not application bases |
| Full/classified release, CLI/runtime/host delivery | Existing full versus reduced Gommitlint, classified eleven-asset and canonical-host evidence remain separately scoped/held; local crypto/checksum/source proof does not authenticate hosted delivery |
| Mobile, meta/files and package snapshots | Preserve Android/iOS restrictions, exact Rust development receipts, installed npm and timestamped Maven oracles; signed mobile releases and provider-specific library/actual-source support remain gaps. Generic selected-file delivery is separate unintegrated feature work |

The row-level appendices below supply the migration mapping, not replacement
live judgements. In particular preserve historical partial/failed GitLab lifecycle attempts,
Gradle's zero-Cosign caveat, classified/protected-job failures, missing full Nano
graph and full-versus-reduced inventories. Dated parity prose and newer matrix
marks can refer to different milestones; resolve their evidence coordinates at
the existing owners rather than promoting or erasing either here. R/G cache
source/run linkage and independent cleanup-absence receipts remain explicit
evidence limitations. No current-tree full lifecycle PASS is established by this page.

## Bounded native acceptance: failed qualification, no retry

The parent recorded user approval for required preparation and **one Forgejo
public Buildah Linux/amd64 Maven-service lifecycle cell**, the exact first
command above, using signed `af9da105` in the named scratch repositories
`forgeci/fci-consumer-lifecycle` and `forgeci/fci-consumer-trial-layer`.
It includes PR, native merge, artifact-first snapshot/readiness, stable release
and owned cleanup. It is not approval for the whole cases file, private/Kaniko
variants, other consumers or held provider lanes. Serializing this bounded first
acceptance is deliberate scope control, not a recommendation to serialize future
independent ready providers.

The parent reports that the approved single qualification on frozen `af9da105`
completed **FAILED**, exit 1 after 90.27 seconds, with **no retry**:

- PR run **545 PASS** and native PR merge **PASS**.
- Development run **546 FAILED** in producer job **7539**, before Maven build:
  Java runtime discovery's ten-second deadline killed the process. This is the
  confirmed failure boundary, not a diagnosis of why discovery exceeded it.
- Downstream context/build/publish/readiness were blocked, not dispatched:
  **NOT RUN**. Stable release **NOT RUN**. This is not a Phase 2 pass.
- Exact retained verdict (external evidence, not regenerated here):
  `/home/jos/development/.opencode-itiquette-evidence/forgejo-maven-refactor-20261109/native-runs/20261109T193041Z-599727/verdict.json`.
- Parent read-only status at **20:32:39**: both providers idle, zero active and
  zero retained jobs. Network unchanged at 228 leases / 25 free / eight attached /
  220 historical leaked; no observed persistent lease growth. This does not
  provide independent repository/package absence proof.

These results are parent-supplied, not a native run or evidence modification by
this audit. Historical matrix/evidence marks are unchanged and are not this
source's proof. A separate parent-owned read-only diagnostic investigates the
Java failure; no producer fix, source edit, retry or broader scope is authorized
here. GitLab/GitHub and protected/classified campaign holds remain in force.
The capacity/no-network-reset/no-automatic-retry and no-public-log conditions
remain explicit for any separately approved continuation.

Retained acceptance obligations for a separately authorized continuation:

1. Stay within that approved road, visibility, builder, source and exact mutable
   coordinates/cleanup authority. Additional roads or broader effects require
   separate approval; approval is not a successful preparation or native result.
2. Preserve the independently reviewed, signed clean `af9da105` engine and its
   canonical checksum binding. Retain preparation's exact consumer source,
   declaration/settings hashes and image digest bindings. The separately reviewed helper
   and these new docs are not qualification inputs; do not reuse historical
   passes as this source's proof.
3. Current Forge Lab instructions, authorized fresh target/CA/acknowledgements,
   free supported provider lock, Linux/amd64 container/readiness runner, verified
   Java nanolinter and signer images, signing tools and supported key-backed
   no-public-log profile. No HOME/XDG/contract/lock workaround is acceptable.
4. Approved preparation/evidence directories, exact cleanup authority and retained
   failure evidence; inventory the consumer/layer repositories, packages, keys
   and any shared image/runner resources before effects. Never sweep kept demos
   or reusable prerequisite images as disposable trial resources.
5. Observed host/runner CPU, RAM, disk, concurrency and a bounded preparation/run
   budget with headroom, including cleanup. The parent recorded a 25-free-address/
   220-leaked-lease warning; separate address-budget analysis and current capacity
   admission remain required for further effects. The post-run unchanged network
   observation above is not future capacity admission. This audit neither
   refreshes capacity nor authorizes network repair or broad cleanup.
6. Retain source identities, native run/attempt and merge/request/tag identities,
   planned output/receipt/digest/signature/readiness readbacks, exact exit/verdict
   and cleanup results (with absence-proof limitations explicit).

No providers, default/live preflight, mutable preparation, evidence regeneration,
commits, pushes or tags are performed by this documentation audit. The parent's
separately approved source freeze/preparation and failed attempt do not establish
acceptance. Phase 2's required native proof and Phase 3's road-specific acceptance cannot honestly be
closed until those separately approved results exist.

## Appendix A: every project-kind and flow row

This is a baseline-to-current migration crosswalk, not runnable configuration.
Each row label links its authoritative [matrix 1](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github)
row; evidence is the existing overlay keyed by **provider / that row's demo /
flow**, [E](../test/live/campaigns/evidence.txt), with its exact dated notes:
PR uses `pull-request`, either snapshot column uses `development`, release uses
`release`, and the linked whole flow uses `lifecycle`. Missing E keys stay
unproven; neither the snapshot column name nor `snapshot` is a new evidence key.
No marks are copied or rejudged here. Cells below are **Forgejo / GitLab /
GitHub**, including all five flow columns even where inapplicable.

Source abbreviations (only typographical shortcuts in this appendix):

| Source | Exact maintained entrypoint, selection and assertion owner |
|---|---|
| FP / FR | [`layerconsumertrial_test.go`](../test/live/conformance/layerconsumertrial_test.go), `TestForgejoLayer_ConsumerTrial`; [`k3s-private-forgejo.cases`](../test/live/campaigns/k3s-private-forgejo.cases), selector `<row-demo>/pull-request` or `<row-demo>/release`; consumer gate/run checks versus shared `requestRelease`/signature and authenticated checksum-subject checks |
| LP / LR | [`gitlabconsumertrial_test.go`](../test/live/conformance/gitlabconsumertrial_test.go), `TestGitLabLayer_ConsumerTrial`, versus [`gitlabrelease_test.go`](../test/live/conformance/gitlabrelease_test.go), `TestGitLabRelease_ConsumerTrial`; [`k3s-private-gitlab.cases`](../test/live/campaigns/k3s-private-gitlab.cases), corresponding `<row-demo>/pull-request` or `/release`; native policy/runtime gate versus signed release/package/image checks |
| GP / GD / GR | [`githubconsumer/consumertrial_test.go`](../test/live/githubconsumer/consumertrial_test.go), `TestGitHubLayer_ConsumerTrial`; `just test-live-consumer github <exported-row-checkout> pull-request`, `dev-release` or `release org=<id>`; each row's `consumer/test/live/github/workflows/{pullrequest,release-dev,release}.yml`. `judge`, development artifact/source checks, `CheckRelease`; `checkImages` is opt-in, not universal |
| FC / LC / GC | [`consumerlifecycle_test.go`](../test/live/conformance/consumerlifecycle_test.go) `TestForgejoLayer_ConsumerLifecycle`; [`gitlabconsumerlifecycle_test.go`](../test/live/conformance/gitlabconsumerlifecycle_test.go) `TestGitLabLayer_ConsumerLifecycle`; [`githubconsumer/lifecycle_test.go`](../test/live/githubconsumer/lifecycle_test.go) `TestGitHubLayer_ConsumerLifecycle`. Lab selection: `<row-demo>/lifecycle` in the linked lifecycle cases below; hosted command as above with `lifecycle`. Preserve `requireMerge`, `requireServiceDevelopment`, `requireGitLabServiceSnapshot`, `requireGitLabLifecyclePipelines`, `judgeSnapshot`, `requireInventory` and the provider-specific release checks |
| FS / LS | [`forgejosnapshot_test.go`](../test/live/conformance/forgejosnapshot_test.go) `TestForgejoLayer_Snapshot`; [`gitlabsnapshot_test.go`](../test/live/conformance/gitlabsnapshot_test.go) `TestGitLabLayer_Snapshot`; `<row-demo>/snapshot` in [`snapshot-forgejo.cases`](../test/live/campaigns/snapshot-forgejo.cases) / [`snapshot-gitlab.cases`](../test/live/campaigns/snapshot-gitlab.cases); shared `RequireSnapshotImages`, native source/plan/receipt/registry checks, no readiness |

`FP:24` means the actual line in FP's linked cases file, not a new selector.
FC/LC line numbers refer to [`lifecycle-forgejo.cases`](../test/live/campaigns/lifecycle-forgejo.cases)
and [`lifecycle-gitlab.cases`](../test/live/campaigns/lifecycle-gitlab.cases).
`NA` means no applicable declared image route or served native release, **not a
claim that the product can never support it**. `gap` means no saved/fixture road;
`ref` is the named harness admission refusal. Every source entry is KEEP at its
current moved owner; no absent cell authorizes retirement of another flow.

| Matrix row / inherited E key | PR / MR | Artifact-first snapshot | Container-first snapshot | Release | Full lifecycle |
|---|---|---|---|---|---|
| [hello-android](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | F gap: no workflow; LP in [Android cases](../test/live/campaigns/k3s-private-android-gitlab.cases):14; GP | NA / NA / NA: no image | NA / NA / NA: no image | NA / NA / NA: no served release | ref no container / NA no release build / ref no service route |
| [hello-cargo-container](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:33 / LP:34 / GP | NA / NA / NA: container-first | FC:34 / LC:35 / GC (also GD); E's old fixture run is not this demo's current proof | FR:34 / LR:45 / GR | FC:34 / LC:35 / GC |
| [hello-cargo-workspace](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:36 / LP:39 / GP | NA / NA / NA: container-first | FS:29 / LS:34 / ref two-image service scope, no GD fixture | FR:37 / LR:46 / GR | ref / ref / ref: two images outside one-service scope |
| [hello-go-cli](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:18 / LP:31 / GP | NA / NA / NA: no image | NA / NA / NA: no image | FR:19 / NA no release build / GR; inherited GPG profile uses `fixtures=test/live/github-gpg workflow=release-gpg.yml`, not default-profile equivalence | ref no container / NA no release build / ref no service route |
| [hello-go-service](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:21 / LP:35 / GP | NA / NA / NA: container-first | FC:36 / LS:36 / GC (also GD) | FR:22 / NA no release build / GR | FC:36 / NA no release build / GC |
| [hello-gradle](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | F gap: no workflow / LP:37 / GP | NA / NA / NA: no image | NA / NA / NA: no image | NA no workflow / NA no release build / GR; retain E's zero-Cosign/five-GPG partial limitation | ref no container / NA no release build / ref no service route |
| [hello-ios](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | F gap: no workflow / L gap: no gate / GP macOS simulator/archive only | NA / NA / NA: no image | NA / NA / NA: no image | NA / NA / NA: no served release | ref no container / NA no release build / ref no service route |
| [hello-maven](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:30 / LP:32 / GP | NA / NA / NA: no image | NA / NA / NA: no image | FR:31 / LR:42 / GR | ref / ref / ref: no container, not the actual library adapter |
| [hello-maven-service](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:24 / LP:36 / GP | FC:32–33 / LC:34 / GC (also GD); E's retired A snapshot remains historical | NA / NA / NA: artifact-first | FR:25 / LR:44 / GR | FC:32–33 / LC:34 / GC; inherited GitLab partial/failed attempts remain distinct |
| [hello-monorepo](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | F gap: no workflow / LP:38 / G gap: no fixture | ref no Forgejo release pin / LS:35 / ref no root-container route or GD fixture | NA / NA / NA: artifact-first | NA / NA / NA: no served release | ref no root container / NA no release build / ref no root container |
| [hello-npm-package](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:39 / LP:40 / GP | NA / NA / NA: no image | NA / NA / NA: no image | FR:40 / LR:47 / GR; signed tarball is not guaranteed npm registry publication | ref / ref / ref: no container/service route |
| [hello-npm-service](testing-matrices.md#1-capabilities-by-project-kind--flow-cells-forgejo--gitlab--github) | FP:27 / LP:33 / GP | FC:35 / LC:36 / GC (also GD) | NA / NA / NA: artifact-first | FR:28 / LR:43 / GR | FC:35 / LC:36 / GC |

Inputs remain in each referenced case and consumer declaration under
[`fixtures/consumers`](../test/live/fixtures/consumers/README.md), not a second
settings table. Native fixture availability was checked in each consumer tree.
Refusal/NA interpretation is owned by [`campaignlifecycle.go`](../test/live/internal/livetest/campaignlifecycle.go)
(`serviceSnapshotAdmission`, `campaignAdmitSnapshot`) and
[`testingmatrices.go`](../test/live/internal/livetest/testingmatrices.go)
(`lifecycleApplies`, `lifecycleState`, `developmentCell`, `releaseCell`).
`TestCampaignLifecycle_RefusesWhatItCannotRun`,
`TestCampaignSnapshot_RefusesWhatItCannotRun` and `TestTestingMatricesInSync`
retain executable source checks; they do not make these native runs pass.
Missing dedicated Go/Rust library downstream publication, npm workspace and
Maven reactor rows remain explicit fixture/capability decisions, not aliases of
these application rows. The actual Maven-library stable/development witnesses
remain [`TestForgejoMavenLibraryConsumer_Stable`](../test/live/conformance/mavenlibraryconsumer_test.go)
and [`TestForgejoMavenLibraryConsumer_Development`](../test/live/conformance/mavenlibrarydevelopment_test.go):
signed stable POM/JAR/classifier bytes versus exact-SHA fresh timestamped
registry resolution and owned package cleanup, not these demo application rows.

## Appendix B: every variant, platform and kept scenario row

Each variant label links [matrix 2](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated);
its existing E record is qualified by actual run axes through `axisMark` and
`axisValueCovers`, never by fixture presence. Matrix 1's best-flow mark is not
proof of every axis value. KEEP all rows and their declaration/input owners;
all live evidence is inherited, not newly observed.

| Matrix 2 axis / exact variant | Source/input and assertion disposition; explicit limit |
|---|---|
| ecosystem / [go](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Go-service declaration → FC/GC and shared snapshot oracle; LC gap: no GitLab release build; CLI release is not service or library proof |
| ecosystem / [gradle-android](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Android declaration/MR/GP; no container lifecycle or signed release; preserve unsigned APK/AAB assertions |
| ecosystem / [java](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Maven-service declaration → FC/LC/GC merge/snapshot/release oracles; Maven-library/Gradle publication not inferred |
| ecosystem / [node](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | npm-service declaration → FC/LC/GC; installed registry/package consumer is separate, not ordinary image lifecycle proof |
| ecosystem / [rust](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Cargo-container declaration → FC/LC/GC; two-image workspace and four-role development receipts stay separate |
| ecosystem / [xcode-ios](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | iOS fixture GP only: macOS ARM64 unsigned gate/archive; no service lifecycle or signed release |
| route / [artifact-first](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Maven/npm service declarations; `ServiceSnapshotDeclaration` + `RequireSnapshotImages` bind producer/plan/source/receipts; FC readiness is additional |
| route / [container-first](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Cargo/Go service declarations; same oracle but no invented producer JAR/package job; native builds, not emulation |
| route / [no container](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | `serviceSnapshotAdmission` refuses root-Containerfile absence; standalone PR/file release kept, no fake snapshot |
| images / [2 images](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Cargo-workspace declaration → FS/LS exact two-image plan/receipts/suffix tags; one-service lifecycle refused on all roads |
| images / [nested artifacts (monorepo)](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Monorepo declaration → LS nested Java/Node producers and two images; F/G root-container/workflow gap, no LC release build |
| images / [no image](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Declaration/admission refusal for service lifecycle; retain file/package/mobile-specific checks, not an image oracle |
| images / [one image](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | FC/LC/GC service admission + shared exact plan/receipt oracle; standalone one-image snapshot still has no readiness |
| signing / [GPG declaration](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Go-CLI `artifacts-gpg.yml`, hosted exact GPG fixture profile; profile/boundary tests and `CheckRelease`; no service lifecycle demo |
| signing / [default](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Declaration/native release default, e.g. Cargo; shared bundle checks retain native method; no default→GPG/keyless/KMS equivalence |
| signing / [sigstore](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Maven/npm/Go service native release and signature-subject checks; actual key/keyless/transparency admission remains native, not a generic setting copied here |
| image builder / [buildah](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Forgejo case variable versus GitLab lane builder/archive; `ForgejoConsumerImageBuilder`, `TestCampaignAdmissionChecksTheEffectiveBuilderOfEveryConsumer`, FS/LS/FC/LC receipts; hosted has no campaign-builder lane |
| image builder / [kaniko](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Same native owners plus digest/source-labelled Kaniko job image and archive admission; preserve Forgejo whole-file mixed-builder rule; hosted has no equivalent selector |
| visibility / [public](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | `ConsumerVisibilities`, existing `/visibility=public` selector; native repository/publication/readback authority, not private access proof |
| visibility / [private](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Same selector/native owners, `CheckPrivateReleaseAssets` and applicable authenticated image readbacks; private job-image pull is a distinct witness |
| enforcement / [admin lint](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | `adminInputEnvironment`, `adminSettingsConsumed`, `ComponentAdminInputs`; generated wrappers/pinned policy consume declared lint enforcement; `TestConsumerPlanBindsAdminInputsOnlyInItsGeneratedWrappers`; no hosted campaign lane |
| enforcement / [admin vulnerabilities](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Same authority owner, independent vulnerability input; `TestAdminWrapperRejectsConsumerValuesAndMergedInputAuthority`; real consuming-job refusal still requires native proof; no hosted lane |
| enforcement / [admin application-tests](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Same authority owner, independent application-test input; `TestGitLabAdminPolicyKeepsEnforcementOutOfConsumerRendering`; no consumer override or hosted lane |
| enforcement / [admin reproducible-builds](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Same consumed-input validation; GitLab templates declare it, Forgejo does not. Generated Forgejo lane listing is **not** executable enforcement support: declaring/enabling unconsumed input refuses; explicit source/generator representation limitation, no hosted lane |
| enforcement / [none enforced](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | `readAdminInputs` report-only defaults and wrapper literal tests; checks still run, not an enforcement witness; hosted has no campaign lane |
| architecture / [amd64](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | Declaration overlay/`RunnerArch`, FC/LC/GC and FS/LS native platform assertions; exact runner/receipt evidence required |
| architecture / [arm64](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated) | D-5 exclusion on all native release roads; cross-compile assets/OCI fixtures do not execute ARM; iOS gate exception is not generalized |

Authoritative source links for the input/assertion references above:
[`campaignsettings.go`](../test/live/internal/livetest/campaignsettings.go),
[`campaignadmin.go`](../test/live/internal/livetest/campaignadmin.go),
[`admininputs.go`](../test/live/internal/livetest/admininputs.go),
[`admininputs_internal_test.go`](../test/live/internal/livetest/admininputs_internal_test.go),
[`campaignsettings_internal_test.go`](../test/live/internal/livetest/campaignsettings_internal_test.go),
[`servicesnapshot.go`](../test/live/internal/livetest/servicesnapshot.go),
[`snapshotoracle.go`](../test/live/internal/livetest/snapshotoracle.go).
Effective settings/defaults stay in these owners and the existing road docs.

| Matrix 1b family (all seven) | Source/assertion/input disposition and inherited evidence boundary |
|---|---|
| [Shared bundle helper, default + GPG (C)](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP FR/LR bundle-subject verification and hosted `CheckRelease`, `signing_test.go`/`signing_boundary_test.go` profiles. E's Go-CLI hosted GPG coordinate and [E7](testing-parity.md#e7-b-source-integration-and-c-hold) refer to scoped milestones; matrix green versus held/source-only prose is an unresolved evidence-profile decision, not permission to drop a witness |
| [Focused cache R / G](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP [`TestForgejoLayer_ResolvedImagePlan`](../test/live/conformance/layerimageplan_test.go): cold/partial/warm sibling identities, actual job hits/misses and final inventory/BOM; [`TestGitLabRelease_BaseImagesColdAndWarm`](../test/live/conformance/gitlabbases_test.go): two releases, signed immutable-base verified reuse. Exact R/G letter→run linkage remains an evidence gap; neither is full Nano DAG proof |
| [Classified 11-asset release](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP exact eleven-name package/crypto and refusal material in [E4](testing-parity.md#e4-local-source517-qualification) / [E5](testing-parity.md#e5-offline-acquisition-seam) and [acquisition contract](github-classified-artifact-acquisition.md). LOCAL/OFFLINE proof, hosted original producer/signer/current-attempt authority still missing/held; no equivalent F/L native witness asserted |
| [Protected-job owner / BASE isolation](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP external Forge Lab qualification owner and failed rounds/prepared round 5. GitLab `TestGitLabPolicy_ConsumerCannotReplaceTheSigningPipeline` is only the pinned-policy/inert-key canary, **not** crypto or privileged-runner containment; replacement native proof is an explicit external acceptance gap |
| [Nano / root → child chain](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP product base-graph owners and [`TestRootBaseCanonicalHandoffUsesVerifiedEvidenceRoot`](../test/gitlabbehavior/rootbasehandoff_test.go); offline root handoff is not native signed root→parent→child/fan-in. Full graph/flavor/cold-warm native fixture/evidence remains a gap after protected-job gate |
| [Full Gommitlint 81-file](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP FR/LR actual-source release input profiles and exact inventory/26-bundle readbacks in [E1](testing-parity.md#e1-forgejo-full-release) / [E2](testing-parity.md#e2-gitlab-full-release); hosted [E3](testing-parity.md#e3-github-reduced-release) eight-file/four-bundle profile is not replacement proof; no cross-forge byte reproducibility inferred |
| [Canonical host delivery, option B](testing-matrices.md#1b-platform-capabilities-not-per-project-kind) | KEEP existing canonical CLI/bootstrap/runtime owners and [E7](testing-parity.md#e7-b-source-integration-and-c-hold). Matrix queued/running lane is historical; no current-source hosted authenticated-delivery result/entrypoint is established by this source-only audit. CLI checksum/local PASSMATCH is not that acceptance; pending provider proof is an explicit gap |

All kept non-lifecycle matrix-2 rows also receive KEEP, **not** lifecycle
equivalence. Each test below is in `test/live/conformance`; the linked cases
file is the exact input owner. The [kept-row table](testing-matrices.md#2-e2e-scenarios--variants-lifecycle-unless-stated)
and E's `scenario` keys retain evidence; an unrun row is not promoted here.

| Existing case row | Current test source and unique assertion/input boundary |
|---|---|
| [build-capability-gitlab](../test/live/campaigns/build-capability-gitlab.cases) | `gitlabrelease_test.go`: `TestGitLabRelease_BuildImageCapability`; privileged Buildah RUN archive, not normal Kaniko capability |
| [harness-inputs-forgejo](../test/live/campaigns/harness-inputs-forgejo.cases), PR row | `layerconsumertrial_test.go`: `TestForgejoLayer_ConsumerTrial`; explicit checkout/event/flavor input admission and native PR, not release |
| Same file, release row | Same test, explicit checkout/release request/version; authenticated checksum subjects; not the PR's assertion set |
| [harness-inputs-gitlab](../test/live/campaigns/harness-inputs-gitlab.cases), branch protection | `gitlabrelease_test.go`: `TestGitLabRelease_ConsumerBranchProtection`; explicit checkout and protected-branch refusal |
| Same file, native release | `gitlabrelease_test.go`: `TestGitLabRelease_ConsumerTrial`; exact checkout/archives and native signed release |
| Same file, preseed negative | `gitlabunsignedhandoff_test.go`: `TestGitLabRelease_RejectsPreseededProvenance`; refuse unsigned/preseeded handoff |
| Same file, interrupted recovery | `gitlabpromotionretry_test.go`: `TestGitLabRelease_ConsumerInterruptedRecovery`; original child/request/candidate, delayed signing, retry convergence |
| [interruption-manual-gitlab](../test/live/campaigns/interruption-manual-gitlab.cases) | `gitlabinterruption_test.go`: `TestGitLabWorkflow_InterruptedJob`; external interruption/130 and cleanup, not normal success |
| [k3s-private-gitlab](../test/live/campaigns/k3s-private-gitlab.cases):29 | `gitlabrelease_test.go`: `TestGitLabRelease_MinimalSignedArtifacts`; minimal signed profile, not full consumer inventory |
| Same file:49 | `gitlabcheck_test.go`: `TestGitLabCheck_ChecksOutPrivateConsumerWithJobToken`; native private-read authority |
| Same file:50 | `gitlabcheck_test.go`: `TestGitLabCheck_ChecksOutTheConsumerPipelineCommit`; exact pipeline commit, not branch-head clone |
| Same file:51 | `gitlabpolicy_test.go`: `TestGitLabPolicy_ConsumerCannotReplaceTheSigningPipeline`; pinned policy survives hostile consumer, inert key/sentinel preserved |
| Same file:52 | `gitlabpolicy_test.go`: `TestGitLabPolicy_JobTokenCannotWriteThePublicationRegistry`; negative publication-project write plus producer-project positive control |
| Same file:53 | `gitlabrelease_test.go`: `TestGitLabRelease_MinimalBranchProtection`; minimal owner protection/refusal |
| Same file:54 | `gitlabserialization_test.go`: `TestGitLabRelease_ResourceGroupSpansTheChild`; second bridge queues while first child holds resource |
| Same file:55 | `gitlabpromotionretry_test.go`: `TestGitLabRelease_PromotionRetryAfterRegistryWrite`; original failed-write journal and immutable digest converge |
| Same file:56 | `gitlabbootstrap_test.go`: `TestGitLabBootstrap_PublishesAndDownloadsTheRecordedBinary`; exact bootstrap binary delivery; E explicitly withholds its failed latest verdict |
| [release-images-forgejo](../test/live/campaigns/release-images-forgejo.cases) | FR `TestForgejoLayer_ConsumerTrial` + `releaseimageoutputs_test.go`; release-only opt-in `release-images-config`, verified prepared-commit declaration, unique unexpired receipts and exact image/signature readbacks, not generic base/final namespaces |

## Appendix C: each original HAR/PAR requirement ID

All **50 exact IDs counted by the cross-slice audit** are individually mapped
below. Source verification additionally found the existing **HAR-03b** row in
[`cli-black-box.md`](cli-black-box.md#test-harness-requirements): the audit's
HAR-numeric-only expression omitted that suffix. It is included, not silently
excluded (51 source tokens with suffix-aware counting). The audit's historical
50-ID/88-occurrence preservation result remains a statement of its method,
not a claim that the suffixed requirement did not exist.

Every PAR source filename below links the current owner; each named function
was checked in source, including its assertions, not inferred from its ID.
`S` = source/offline contract; `CLI/service` = host CLI/adapter talking to real
lab services, often synthetic context; `Native` = actual provider jobs. A
registered live test is not itself a pass. Retain capability-selected provider
limits and [proof/evidence rules](testing-parity.md#reading-the-proof); these
PAR primitives have no new per-ID E overlay rows, so absent exact retained
run/profile linkage remains an **evidence gap**, not a fresh live judgement.

| ID | Current source function/assertion and input/evidence disposition |
|---|---|
| HAR-01 | S: [`smoke_test.go`](../cmd/forge-ci/smoke_test.go) `TestMain`, `buildBinary`, `runBinary`; fresh external binary, bounded child and cleanup, not live service proof |
| HAR-02 | S: [`isolatedenv_test.go`](../internal/testutil/isolatedenv/isolatedenv_test.go) `TestIsolate_ScrubsAndPinsProcessEnv`; HOME/XDG/GPG/tool state below temp root; smoke `cleanupSmokeRoot` and `TestCleanupSmokeRoot_ReportsAndFailsOnARemovalError` preserve cleanup failures |
| HAR-03 | S: same file `TestIsolate_UnsetsGitRoutingAndRestoresCallerEnvironment`, `TestIsolate_AHostTemplateHookDoesNotReachAScratchRepo`; explicit Git/GPG/SSH isolation, not an OS sandbox |
| HAR-03b (additional existing suffix) | S: same file `TestIsolate_ScrubsAmbientCIEnv`, `TestIsolate_ScrubsEveryFamilyAndRestoresAfterTheScope`; clears ambient forge/token families before explicit fixtures |
| HAR-04 | S: same file `TestIsolate_ScrubsAndPinsProcessEnv`; actual implementation pins `LC_ALL=C`, `NO_COLOR=1`. HAR prose's C.UTF-8 example is not an enforced alternate locale guarantee |
| HAR-05 | S: [`smoke_test.go`](../cmd/forge-ci/smoke_test.go) `runBinary` refusing proxy plus isolated mock/fixture design under the linked HAR policy; real forge/publish/identity/tlog endpoints forbidden. Public package build-input downloads are the documented exception; no claim of OS-level network containment |
| HAR-06 | S: [`smoke_test.go`](../cmd/forge-ci/smoke_test.go) `TestBinary_ValidateAuthRegistry_SecretIsUsedButNeverWritten` with disposable input; [`credential_guard_test.go`](../internal/testutil/isolatedenv/credential_guard_test.go) `TestIsolate_CredentialSourcesAreClearedAndRestored`; no host keyring/real token proof |
| HAR-07 | S: smoke `runBinary` independently returns stdout/stderr/exit; `TestBinary_Doctor_JSONReportIsConsistent` checks machine output, not native result interpretation |
| HAR-08 | S: [`smoke_cli_contract_test.go`](../cmd/forge-ci/smoke_cli_contract_test.go) `TestBinary_FlagFormEquivalence` compares both streams/status for string `--log-level` forms; exhaustive integer/string-flag coverage is not established by this one witness: explicit remaining coverage gap |
| HAR-09 | S: same file `TestBinary_InvalidGlobalEnum_ExitsUsageNamingAcceptedValues` and explicit `--provider` artifact/SARIF cases; runtime fixtures are explicit, not ambient CI; complete native-provider execution is outside this isolated tier |
| HAR-10 | S: smoke `TestBinary_ArtifactUpload_NoSelectorExitsUsage` and refused merge's output absence checks in `smoke_cli_contract_test.go`, plus `requireSharedWorkDirEmpty`; these are partial no-write controls. Full every-scenario tree + mock-call before/after snapshot coverage is an explicit audit gap, not inferred from cleanup |
| HAR-11 | Explicit scheduled-hardening gap: HAR source says n/a, no complete performance input-size/wall-clock/machine-class regression scenario identified; do not substitute ordinary test timing |
| PAR-ART-1 | CLI/service: [`artifact_test.go`](../test/live/conformance/artifact_test.go) `TestArtifact_WithoutAStore_RefusesAndNamesTheGap`; unsupported forge/store refusal names platform, EX_CONFIG not retryable outage |
| PAR-ART-2 | CLI/service: same file `TestArtifact_WithAStore_FailsOnTheRuntimeNotTheCapability`; store-capable forge outside job fails for runtime, not unsupported store |
| PAR-ART-3 | Native: same file `TestInRunner_ArtifactRoundTripsThroughTheStore`; actual runner upload/download byte identity on capable roads |
| PAR-ART-4 | Native: same file `TestInRunner_GitLabNativeArtifactsReachTheNextJob`, `TestInRunner_GitLabReleaseHandoffRejectsTamperedBytes`; distinct-job binary handoff and tamper refusal, GitLab-specific |
| PAR-CAP-1 | S/untagged: [`capability_test.go`](../test/live/conformance/capability_test.go) `TestCapabilityMatrix_MatchesTheAdapters`, `TestParseCapabilityTable_ReadsOnlyTheScopedUniqueMatrix`; every model field/provider cell matches [provider matrix](providers.md#capability-matrix), not a live capability measurement |
| PAR-CAP-2 | CLI/service: [`degradation_test.go`](../test/live/conformance/degradation_test.go) `TestDegradation_SARIFUpload_NoticesAndSucceeds`; absent code scanning notices/succeeds; fallback delivery and UI ingestion are not asserted by this witness; smoke offline counterpart stays S |
| PAR-CAP-3 | CLI/service: same file `TestProvenance_GeneratesEquivalentlyOnEveryForge`; exact in-toto/SLSA subject/hash from supplied checksum, no quiet degradation; generation is not hosted authenticated producer authority |
| PAR-CHK-1 | CLI/service: [`checkout_test.go`](../test/live/conformance/checkout_test.go) `TestCheckout_ProducesAWorkingTreeAtTheRequestedRef`; README/HEAD exact requested ref and later-only file absence, not native job-token checkout proof |
| PAR-PKG-1 | Native: [`forgepackages_test.go`](../test/live/conformance/forgepackages_test.go) `TestInRunner_ForgePackages_NPMPublishReachesTheRegistry`; absent→present exact version, installed package API/version/dist-tag assertions using native package authority; not signed-tarball equivalence |
| PAR-PKG-2 | Native: same file `TestInRunner_ForgePackages_MavenDeployReachesTheRegistry`; actual Maven deployment absent→present exact package version; not blanket downstream timestamped/classifier resolution proof (separate Maven-library oracle) |
| PAR-REG-1 | CLI/service: [`registry_test.go`](../test/live/conformance/registry_test.go) `TestRegistry_SyntheticArtifacts_RoundTripByDigest`; immutable synthetic digest and absent-tag control, not native injected auth |
| PAR-REG-2 | CLI/service: [`ledger_test.go`](../test/live/conformance/ledger_test.go) `TestLedger_RecordVerifyPromote_PreservesTheDigest`; record/verify/promote and mismatch refusal retain served manifest identity |
| PAR-REG-3 | CLI/service: same file `TestLedger_Cleanup_RemovesTheCandidateAndKeepsTheRelease`; candidate absent while release/aliases sharing manifest survive with same digest; not ordinary scratch teardown |
| PAR-REG-4 | Umbrella only: [`rollback_test.go`](../test/live/conformance/rollback_test.go) defines undo-promotion family; exact executable branches are 4a/4b below, not a third independent pass |
| PAR-REG-4a | CLI/service: same file `TestRollback_FromJournal_RemovesAReleaseTagThePromotionCreated`; journal permits deletion only of newly created immutable release tag |
| PAR-REG-4b | CLI/service: same file `TestRollback_FromJournal_RestoresAMovingTagToItsPreviousImage`; restores prior moving-tag digest, not deletion |
| PAR-REG-5 | Native: [`registry_test.go`](../test/live/conformance/registry_test.go) `TestInRunner_ForgeInjectedRegistryCredentialAuthenticates`; actual runner-injected credential finds/authenticates registry, not controller PAT success |
| PAR-REL-1 | CLI/service: [`release_test.go`](../test/live/conformance/release_test.go) `TestRelease_CreateWithAssets_IsEquivalentAcrossForges`; API metadata and exact served asset bytes after create |
| PAR-REL-2 | CLI/service: same file `TestRelease_UploadAssetToExistingRelease_ServesTheSameBytes`; separate upload API size/hash readback, not create-with-assets shortcut |
| PAR-REL-3 | CLI/service: same file `TestRelease_ReReleasingATag_ReplacesRatherThanAccumulates`; exact replacement names/body/bytes, not accumulation |
| PAR-REL-4 | CLI/service: same file `TestRelease_PublishReconcile_RefusesPublishedChanges`; public CLI refusal preserves completed release names/state/bytes for both strategies |
| PAR-REL-5 | CLI/service: same file `TestRelease_LargeAsset_ArrivesIntact`; large uploaded payload length/hash readback; no inference from small assets |
| PAR-REL-6 | CLI/service: same file `TestRelease_Policy_IsWhatTheForgeRecords`; stable-only prerelease refusal/no effects, actual draft/prerelease flags (GitLab note instead), retry byte preservation and changed-content refusal; release-file retention is a separate `assetretention_test.go` witness |
| PAR-RUN-1 | Native: [`inrunner_test.go`](../test/live/conformance/inrunner_test.go) `TestInRunner_DetectsItsOwnRuntime`; real job runtime identity, not synthetic host environment |
| PAR-RUN-2 | Native: same file `TestInRunner_ProductDetectsItsRunner`; product binary reports actual runner, separate from fixture detecting environment |
| PAR-RUN-3 | Native: same file `TestInRunner_NoGitHubAnnotationsOnOtherForges`; runner-appropriate annotation dialect, not workflow syntax alone |
| PAR-RUN-4 | Native: same file `TestInRunner_StepSummaryReachesAReader`; summary actually reaches provider reader, not merely local file creation |
| PAR-RUN-5 | Native: same file `TestInRunner_StepOutputsReachTheRunnersOutputFile`; later step/job consumes actual output, including GitLab translated dotenv names; no default-value false pass |
| PAR-SIGN-1 | CLI/service: [`sign_test.go`](../test/live/conformance/sign_test.go) `TestSign_LedgerImages_ProducesAVerifiableSignature`; registry digest/key, signature + attestation subjects/identity negatives, absent tlog entry; synthetic run context is not native authority |
| PAR-SIGN-2 | Native/capability-selected: [`keyless_test.go`](../test/live/conformance/keyless_test.go) `TestInRunner_KeylessSigningAgainstTheLabCA`; native OIDC and lab CA, no public Rekor; missing OIDC/CA support is not pass or key-backed equivalence |
| PAR-SIGN-3 | CLI/service: [`basesign_test.go`](../test/live/conformance/basesign_test.go) `TestSign_BaseImages_AreAcceptedByTheBaseImageVerifier`; authenticated promote verifies pinned source/workflow/flavor/input/SBOM lineage and rejects wrong workflow, absent tlog; anonymous validate-cache miss is a different operation |
| PAR-TOK-1 | CLI/service: [`token_test.go`](../test/live/conformance/token_test.go) `TestToken_RunCredential_Validates`; owned repository's actual run credential validation |
| PAR-TOK-2 | CLI/service: same file `TestToken_RejectedCredential_IsPermissionDeniedNotUnavailable`; deliberately refused credential classification and non-outage exit mapping |
| PAR-TOK-3 | CLI/service: same file `TestToken_RejectedCredential_FailsTheCommandWithAReason`; public CLI reason and non-retryable exit, distinct from adapter classification; kit `cli.go` preserves exact expected-failure path |
| PAR-TOK-4 | CLI/service: same file `TestToken_BotPermissions_ReflectRealAccess`; actual owned repository access/permissions, not token syntax admission |
| PAR-UX-1 | CLI/service: [`weburl_test.go`](../test/live/conformance/weburl_test.go) `TestWebURLs_ResolveOnEveryForge`; real linked pages resolve and nonexistent-resource control prevents generic-200 false proof |
| PAR-UX-2 | CLI/service: [`dryrun_test.go`](../test/live/conformance/dryrun_test.go) `TestDryRun_DestructiveLedgerVerbs_MutateNothing`; promote/cleanup/rollback before/after registry state equal, wet positive control changes it |
| PAR-UX-3 | CLI/service: [`jsonschema_test.go`](../test/live/conformance/jsonschema_test.go) `TestJSON_DoctorReport_HasTheSameShapeOnEveryForge`; actual forge identity and required JSON paths/shapes; not runner isolation qualification |
| PAR-UX-4 | CLI/service: [`errorshape_test.go`](../test/live/conformance/errorshape_test.go) `TestErrors_SameFailureClass_ExitsTheSameOnEveryForge`; controlled failures give same actionable status/reason and no credential/internal-crash leaks |

Phase 1 closes **accounting**, not all of these gaps: all original rows/IDs now
have a disposition with a current source or a named missing/unresolved owner.
No source-only/CLI-service result is relabelled native; optional image oracles
remain optional. Full-row live acceptance, historical evidence reconciliation,
native capability additions and any witness retirement remain later gates.
