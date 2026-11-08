<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Classified file route: plan eligibility unit

This is an ordinary, side-effect-free Go admission unit, **not an activated
workflow, an authorization receipt or a new plan/manifest format**. Existing
native planning, CLI defaults and workflows are unchanged. The explicit
`plan validate-classified-files` CLI exposes this application gate separately
from plan generation.

`internal/app/plan.AdmitClassifiedFiles(ClassifiedFileAdmissionInput)` takes the
existing config-plan JSON and whole release-plan JSON (with its existing prepare,
build, publish and transfer contracts), original caller `ReleaseSBOMs`, the
existing administrator Go/Cargo selection maps, an explicit `ProducerFileOnly`
declaration and an optional existing typed `ReleaseImagesPlan`. It returns only
an error or success: no normalized plan, partial outputs, admission token or
mutable state. It reads no files, calls no provider and runs no product code.

## Checks

- Use existing `DecodeConfigPlan` and `ValidateConfigPlan` for the redundant
  artifact projections, resolved execution fields and container gates. The
  release reader uses existing `DecodeStagePlan` plus the existing consumed-member
  duplicate walk, as selected-test preflight does. Unknown/trailing release-stage
  input and duplicate consumed members are refused, not projected away.
- Reject config containers, release image policy, publish image targets and all
  other registry/container-first publish targets. Both `runs` and `items` are
  checked: false does not hide a nonempty item list; true without items is not
  empty work. Nil and empty inactive lists are equivalent.
- Require exactly one root (`working_directory: .`) artifact-first Go or Cargo
  artifact, with the matching native config, no skip-tests, registry destination,
  library build type or conflicting ecosystem config.
- Require the supported automatic root preparation profile: git-cliff, no custom
  file pattern, version bump enabled, signed/allowlisted release policy and a
  coherent release tag/prerelease/latest classification. Stage identities,
  versions and prepare/build copies must match the actual root config; unexpected
  build targets and transfer requests are refused. This does not authenticate the
  eventual prepared commit or execute preparation.
- Require explicit `ReleaseSBOMs: none`, config pipeline layers `none` and release
  policy layers `none`, with no effective generated layers or SBOM conflict. The
  original release-level request is required separately: an effective `none`
  cannot reveal an original `all` request whose intersection was empty. These
  are **engine generation controls**, not the product's native SBOM inventory.
- Validate the complete administrator selection maps using existing
  `ReleaseTestSelections`. Omitted/blank maps are not a default on this route.
  Go needs a policy-gated selection; Cargo needs an explicit finite selection
  containing a policy executable. There is no application-enforcement waiver
  argument: report-only application assertions cannot remove mandatory policy
  selection. Execution/report freshness and actual coverage remain the native
  test owners' responsibility.
- Require the caller's explicit file-only producer declaration. Any separately
  supplied `ReleaseImagesPlan`, including an empty one, is refused; its actual
  image/base/flavor fields are not fabricated in `ConfigPlan`.

The pure domain predicate is
`pipeline.ValidateClassifiedFilePlans(cfg, release, originalReleaseSBOMs)`; the
application entry point also owns the producer declaration, separate-image-plan
refusal and complete mandatory selection checks. Calling only the domain helper
is not equivalent to complete route admission.

## Boundaries and remaining work

`DecodeConfigPlan` intentionally retains its existing unknown-extension behavior.
This unit does not invent base/flavor fields in that type or claim to close its
wire schema. The config source must already have passed its supported schema
owner; unmodeled producer profiles/other image-plan inputs must be refused by the
caller, not silently dropped before invoking this function. A boolean
declaration cannot inspect hidden OCI work in opaque consumer scripts or prove
administrator ownership. The explicit typed image-plan input provides a refusal
boundary when that existing plan is actually supplied.

No `FileManifest` is read or changed. In particular, `none` here cannot remove a
declared producer SBOM, rewrite pinned SBOM bytes, regenerate checksums, alter
asset names or waive the closed classified payload's roles. The current native
build/transfer plan is checked for eligibility/consistency, **not executed** or
claimed as the classified producer's inventory. Product packaging remains
consumer-owned.

Workflow activation, independently protected caller ownership, immutable engine
and runtime binding, request/prepared-checkout/tag authorization, successful
policy/producer scheduling, artifact ID/run/attempt binding, private payload
staging, package-GPG path coverage, signing and publication remain separate
reviewed units. No private-key or registry-login capability is present here.
Tests use native plan constructors, in-process admission and the real CLI command
tree; they are not live-provider, cryptographic or release acceptance evidence.

## Opt-in CLI validation

`forge-ci plan validate-classified-files` consumes explicit JSON strings through
`--config-plan-json` and `--release-plan-json`. These are the existing serialized
values emitted by `config parse-artifacts` and `plan release`, not their outer
output envelopes. The whole release plan supplies the required prepare, build,
publish and transfer contracts; separate stage flags would duplicate that input.
The existing strict release-stage readers reject unknown fields, consumed-member
duplicates and trailing input. Missing stages, contradictory targets and invalid
plans fail closed through the application gate.

The caller must supply `--producer-file-only`, the original `--release-sboms none`
request and the applicable complete `--go-test-selections-json` or
`--cargo-test-selections-json` map. These inputs have no environment defaults on
this command. No application-test enforcement waiver is exposed. Separate image,
base and flavor plans have no supported CLI input: unknown flags are refused,
not silently projected away. The caller must not use this route for such work.

For already generated, trusted root Go plans, for example:

```sh
forge-ci --provider local --runner local plan validate-classified-files \
  --config-plan-json "$config_plan" --release-plan-json "$release_plan" \
  --producer-file-only --release-sboms none \
  --go-test-selections-json '{"product":{"packages":["./test/policy"],"gate":"policy"}}'
```

Success returns exit zero and emits no output, plan or authorization receipt.
Failure returns an error without partial runner outputs, summary writes or a
fallback to native plan generation. The command invokes `AdmitClassifiedFiles`,
not just its domain predicate. A flag does not establish administrator ownership
or prove producer execution. Generic plan generation commands are unchanged.

Workflow integration must explicitly choose this gate before producer scheduling
and before trusted consumption; native defaults must not select it implicitly.
Because the gate is now CLI-reachable, integration requires fresh legitimate
canonical binary/checksum builds from the actual combined source. No old pins,
copied build records or production references are supplied here.
