<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Protected classified file release leaf

Combined review base: `f539911ee63fbe5709e7d477863fc833722d8cbf`.
Original conditional source review base: `45a0c289f2230e1e44efbffbfdfd8b5efbca95ac`.
This unit implements policy → unsigned product producer → independent package
GPG/Cosign/provenance → native finalization → exact signed publication. It is not
administrator deployment approval, live acceptance or image/flavor parity.

## Integration dependencies (not bootstrap blockers)

The canonical consumption, delivery, transitive preparation, file-plan, classified
CLI, manifest-GPG and prepared/request units are committed ancestors. They are
used directly, without overlays. Bootstrap is not missing.

Three separately reviewed native root fixes are committed ancestors of this
combined candidate, not copied from mutable candidates:

- Root1 (`b943ff34`) removed unnecessary private GPG import and cleanup before
  request authorization. Its prerequisite workflow and dedicated guards remain
  unchanged; the classified route calls that key-free prerequisite owner.
- `forge-ci-producer-acquisition-root`: isolate native legacy acquisition from
  source fixtures. This leaf does not use legacy assembly/downloads: both its
  acquisitions already use new runner-temp directories, separate from metadata
  checkout and private staging. Root2 (`f539911e`) now protects the native-default
  branch too; its complete native release workflow remains unchanged here.
- `forge-ci-late-request-binding`: repeat configured signed-request/final-source
  authorization at native signing/publication boundaries, with native no-bump
  and recovery compatibility. This leaf deliberately supports only P != Q,
  repeats the existing bounded expected-parent validator and **both** local
  request/final-tag signature+allowlist validators before private roles and again
  before publication. Prepared/request equality is not signature authorization.
  Root3 (`029156ae`) supplies the original-Q and late native authority contract.
  Its implementation and dedicated guards remain unchanged. No no-bump/recovery
  policy is invented in this leaf.

The only production overlap is `release-orchestrator.yml`: an opt-in route, strict
upstream freshness predicates, canonical propagation, native route exclusions and
summary dispatch. No prerequisite, preparation, acquisition or source-validator
implementation from those other owners is modified here.
The unconditional original-Q guard remains the first parse-config step; route
admission follows it before canonical consumption. Native comparison tests now
compare directly to the combined base, without historical rootfix exceptions.
The classified leaf workflow changes only the policy command spelling: its
Go/Cargo case branches invoke literal `build go test` and `build cargo test`
commands with the same arguments, so the repository CLI contract check can
validate both paths. This reconciliation requires its own combined semantic GO.

## Protected inputs and native compatibility

The existing orchestrator accepts `release-file-mode: native | classified-files`,
default `native`. Unknown values cannot publish. Classified mode automatically
enables canonical consumption in all upstream receivers and current-attempt
transitive preparation. It runs instead of, never alongside, native build,
registry publication, promotions and legacy release creation. Summary refuses a
failed/skipped classified route instead of treating skipped native jobs as a
successful release. Native CLI bodies, defaults, environments and secret mappings
remain unchanged (the summary result has an explicit mode-dependent source).

Use only an independently protected administrator entry point pinned to the
reviewed full workflow/engine commit, with explicitly qualified digest runtime
tools for the chosen ecosystem and package/signing roles. No production tuple is
provided or invented. A consumer-owned call with identical strings is not an
independent administrator policy. New producer/trust inputs are not arbitrary
dispatch artifact IDs or a producer-claimed source identity.

Classified mode requires the existing original `release.sboms: none` generation
control, no generated SBOM layers, signed/allowlisted automatic root preparation,
git-cliff, one artifact-first root Go or Cargo project, no registry/image/base/
flavor work and complete administrator test selections containing mandatory
policy tests. Application assertions may remain report-only; policy tests never
become optional. Draft creation and unsupported profiles are refused. The actual
`plan validate-classified-files` owner runs before producer code and in trusted
consumers; it is not replaced with an ad-hoc YAML projection.

## Product-owned unsigned contract

`classified-producer-path` selects an executable in prepared checkout P by a
relative path such as `.forge-ci/produce-classified-files.sh`; absolute paths
and `.`/`..` segments are refused before anything runs. It runs
with a cleared environment, the [fixed approved runtime tool profile](trusted-runtime-tools.md), private temporary HOME,
prepared SHA/tag/epoch and `FORGE_CI_CLASSIFIED_OUTPUT`. It receives no private
keys, publication token, checkout token or artifact upload runtime variables.
Its administrator must declare `classified-producer-file-only: true` and keep
its packaging toolchain file-only; no parser can infer hidden OCI behavior in an
opaque script. Provisioning/product hooks run only in key-free jobs.

The script writes the existing closed workspace contract to that output:
`dist/release-files.json` and exactly its declared unsigned files. Use the native
producer/import/declare commands, and project only those declared files. Original
SBOM bytes/roles, package/archive names and checksum filename/content are retained.
No `release assemble`, regenerated SBOM ZIP, checksum regeneration or loose
`dist/**` discovery enters the trusted path. `classified-release-notes-path` must
name declared evidence, not a hidden extra file or source-tree fixture.

Only the fixed protected upload's actual ID and the producer's verified digest
are emitted through original job outputs. Sign consumes those `needs` values,
and publish consumes the equivalent signer outputs. Each independently computes
the attempt-specific artifact name. Exact acquisition checks repository, run and
request head Q (not prepared P), then privately stages and verifies bytes before
keys/tokens. The API does not attest job identity/attempt/success: the protected
upload/needs graph supplies those facts, including policy and all three original
preparation producers. Failed-job-only reruns reusing earlier attempts fail shut.

## Independent roles and capability scope

Trusted jobs execute no consumer hooks, local product actions, configuration
sourcing or cache restoration. Source checkout is metadata only and separate
from acquisition/staging. Every CLI/consumer checkout follows canonical raw-byte
verification from a fresh full-commit engine checkout.

The manifest-backed native package-GPG command signs all declared distro packages,
including nested ones, with the administrator fingerprint. Only that step receives
GPG private material. A separate mandatory key-backed Cosign role first binds its
private key to the existing public-key/digest trust root, signs native checksums,
SBOMs/evidence and prepared-source provenance, and honors explicit transparency
policy. GPG does not substitute for Cosign. The native finalizer adds sidecars and
provenance without reclassifying or changing producer bytes. Publisher receives
only exact signed bytes and verifies native manifest/checksums after generic
private digest staging. Only the native reconciliation publication command gets
the explicit `RELEASE_TOKEN`; no signing key reaches publication. Job tokens stay
contents/actions-read; there are no new write/OIDC grants or registry logins.

## Bounded evidence and trust limits

Workflow guards execute real graph/refusal blocks and inspect capability order.
The new workflow changes the generated embedded CLI run-by index, so that index
and both source-build checksum records must be regenerated from this actual tree.
These are legitimate offline engine-recipe builds, not production workflow/runtime
pin selection; ARM64 is cross-built and byte-verified, not executed on this host.
The CLI test executes actual selected Go/Cargo policy tests (both success and
mandatory refusal with application enforcement false), real file-only CLI plans,
the actual product step, classified handoff, manifest-GPG, Cosign binding/signing,
provenance, finalizer, signed staging and reconciliation dry-run. Only external
GPG/Cosign mechanisms use inert executable fixtures and non-key bytes. This is
not real cryptography, provider mutation, production key qualification or hosted
control-plane acceptance. Existing native Git/source/exact-artifact contract
tests are separate evidence, not an assertion that fake shell counters implement
those validators.

Privileged Bash entrypoints are not global environment sanitizers: ordinary
descendant Bash, trusted startup environment, authenticated fresh checkout/object
storage and no concurrent writers remain prerequisites. A hostile producer that
compromises its entire key-free runner can lie about in-job outputs; this design
does not add a job attestation or claim hostile-runner/reproducibility proof.
Remote freshness is checked at the boundary, not an atomic authorization lease
over subsequent signing/publication. Full image/flavor inventories, administrator
deployment, actual-source runtime/tool qualification, independent review and live
acceptance remain external gates. B/C still require A's actual verdict.
