<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Executable classified release prerequisite: transitive preparation binding

Base: `ef4719e2ec493263cab874b6481413311e5f7513`.
This is a concrete preparation prerequisite, **not the requested executable
classified policy/producer/sign/publish leaf**. No public release-route selector,
dummy producer/signer, or refusal-only workflow is added. The old HOLD candidate
and the independently reviewed canonical-delivery candidate remain untouched.

## Actual gap and executable fix

The integrated attempt foundation captures `version-bump` but explicitly does
not bind its transitive changelog and native-patch producers. A new version-bump
attempt could otherwise consume old successful jobs and advertise only its fresh
attempt. This must not be represented as whole-chain freshness.

The existing `generate-changelog.yml` now exposes original-job run/attempt and
the fixed upload step's artifact ID/name. `prepare-version-files` captures the
same original facts for its patch upload. `release-prepare-stage.yml` forwards
each original transitive run/attempt directly, never from its summary or receiver.

An internal, default-false `require-current-attempt` input on the existing prepare
workflow enables real enforcement, not terminal refusal:

- Full/minimal changelog and patch names include the current run/attempt.
- Before native version-file preparation, require the full changelog producer's
  exact success, original run/attempt, nonempty positive artifact ID and expected
  attempt-specific name. Require authorized Q to equal receiving `github.sha`.
- Before version-bump checkout, require the same facts for **all three** producing
  jobs. Missing/skipped/cancelled/failed/stale/foreign outputs fail. This bounded
  mode refuses existing-final-tag recovery instead of relabeling old work fresh.
- Each strict download uses the original upload ID plus an independently
  computed name and receiving repository/run/request Q with the existing exact-ID
  CLI. No name-only fallback runs in strict mode. Download destinations are fresh
  scratch children, not the consumer checkout; full changelog projection checks
  its expected regular-file leaf before copying it into the working tree.
- All downloads, the existing pure patch admission and changelog checks precede
  GPG/SSH private signing setup. Only then do the unchanged native commit/tag
  owners run. Legacy mode retains fixed names/name downloads, defaults and signing
  mechanisms; its key setup also moves after input validation, without a new role.

This gate binds original workflow outputs under the protected GitHub control
plane. It is not hostile-runner attestation. Exact artifact metadata does not
prove tests, job identity or attempt; those originate in the fixed job graph.
The upstream resolver's execution is not claimed fresh by these three captures;
approved engine identity/bootstrap remain separate requirements.

## Remaining executable-route composition

The parent must deliberately enable this prepare input when the complete safe
classified route is implemented. It is not enabled implicitly here. The future
leaf must inspect direct-needs results and original parse/prerequisite/prepare/
tag bindings, plus the forwarded original transitive preparation bindings. It
must capture actual policy/producer/signer jobs too, and refuse skipped or stale
paths in summary. A fresh summary or wrapper cannot mint freshness.

Canonical delivery remains an independently reviewed integration dependency.
Use the corrected contract: the verified artifact-ID step output appears only
after receive/download/hash/helper verification succeeds, then flows through
receiver-job and workflow outputs. Its value is still the **original producer
ID**, with original producer run/attempt labels, not a receiver relabel. No
downstream authorization may rely on output presence alone: require direct-needs
success and original context, and repeat receiver verification on each trusted
runner before consumer checkout or private environments. The seven-file delivery
candidate is not copied, edited or presumed integrated here.

Still absent here: the executable root Go/Cargo selected-test policy and unsigned
consumer producer; file-only/original-SBOM plan checks before producer and again
before keys; P-child-Q bounded/fresh remote/final-tag checks; closed native
manifest handoff; purpose-scoped independent package GPG/Cosign/provenance and
native finalizer; signed exact-ID staging and reconcile publication. No packaging
names, pinned SBOM/checksum bytes or roles are changed by this prerequisite.

The protected administrator owns workflow/engine refs, qualified runtime and
role trust; those cannot be established by consumer inputs or SDK PATH. No
production approval, pin, credential, provider run, image qualification or live
release identity claim is supplied. Actual combined-source semantic review and
canonical CLI/runtime verification remain required at integration; no old binary
hashes are copied forward. This workflow-only prerequisite changes no Go CLI
source or canonical build recipe.

## Offline evidence for this prerequisite only

With `GOMAXPROCS=2`, `-p 1`, existing build cache and owned disk-backed TMP:
full workflow guards, docs and sync guards passed; focused transitive/original-job
capture races passed; Actionlint with ShellCheck passed on both changed workflows;
configured actual-base delta Go lint (including the new test file) reported zero
issues. Inert counters cover missing/stale/foreign/skipped/failed/cancelled source
facts and failed exact download, with no private-effect marker on refusal. A valid
gate reaches its inert marker: this is an operational opt-in, not refuse-only glue.
These tests neither execute GitHub Actions nor validate hosted rerun semantics,
canonical delivery, real signing, providers, image qualification or publication.
