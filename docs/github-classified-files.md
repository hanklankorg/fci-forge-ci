<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Classified file handoff: first admission unit

**This is a Go use-case unit with explicit CLI routes, not a deployed GitHub
workflow or full release parity.** The existing GitHub native Go/Cargo workflows
are unchanged. No new
artifact schema, test-selection schema, orchestrator or production pin is added.

## Implemented interface

`internal/app/release.ClassifiedFilePayloadDigest(directory)` admits and hashes
an unsigned, closed file-only payload. Its layout is:

```text
payload/
  dist/
    release-files.json
    <exact files in the manifest's assets section>
```

The producer owns the build, packaging, toolchain and names. It can create the
existing `FileManifest` with `release files import-goreleaser` after an unsigned
GoReleaser run, or `release files declare` for another producer. It projects the
manifest and its explicit asset paths into the payload; it does not upload an
arbitrary `dist/**` glob. GoReleaser's internal metadata/build directories stay
outside the payload. The producer is responsible for provisioning GoReleaser,
Cargo/npm or other tooling: this unit installs or pulls nothing.

The admission function checks the existing manifest's classifications and
checksum subjects/digests. It additionally requires closed membership, exact
agreement between an asset and its classified role, no overlapping roles, safe
regular files, bounded metadata/payload sizes and no preseeded signer outputs.
Undeclared files (including hidden files and build metadata) fail admission.

`StageVerifiedClassifiedFilePayload(ctx, source, destination, expectedDigest)`
copies into private staging, repeats admission there, verifies the staged bytes
against the existing stable `.`-prefix dist digest, then installs a new
destination. Existing destinations are never replaced. Failure installs nothing.
Manifest paths remain `dist/...`; source and destination directory names need
not match. The manifest, package/archive bytes, checksum name/content and pinned
SBOM/evidence bytes are not rewritten. There is no SBOM regeneration or SBOM ZIP.

Both functions are key-free and perform no provider calls or consumer execution.
They do not relax or bypass any policy-test selection.

The existing release commands expose this contract only with `--classified-files`:

```sh
forge-ci release dist-digest --classified-files --dist-dir payload
forge-ci release validate-dist --classified-files --dist-dir downloaded-payload \
  --stage-dir fresh-admitted-payload --expected-digest "$EXPECTED_PAYLOAD_DIGEST"
```

The directory is the workspace **containing** `dist/`, not its contents. Consume
requires a non-empty, fresh `--stage-dir`; `--digest-file` can supply the existing
bare lowercase digest recording instead of `--expected-digest`. Classified mode
conflicts with active `--canonical-payload` and any explicit `--manifest-root`,
including an empty one. Generic/canonical modes and their defaults are unchanged.
This CLI route admits unsigned file-only bytes, not image/base claims, signed
payloads or source/artifact authenticity.

## Existing signing owners compose without replacing Cosign

The tests stage a producer payload, invoke the existing independent
`GPGSignPackages` role, invoke `SignArtifacts` for the manifest checksum/SBOM/
evidence bundle roles, and use `WriteReleaseFileManifest` to finalize. The
producer's roles and bytes survive; package `.sig` and artifact `.bundle`
sidecars coexist. Provenance remains signer-owned and is added after unsigned
admission. These tests use inert fake signers and inert provenance: they prove
file-role composition, **not cryptography, authenticity or live acceptance**.

## Authorization boundary still required before workflow wiring

An expected digest is an integrity expectation, not an authenticated source
claim. This unit intentionally accepts no repository/run/job facts and claims
no source authorization. It must not be used alone to justify loading a key or
publishing a release.

The current `provider.RunArtifactDownload` selects name/run/repository, and
`RunArtifactInfo` returns name/ID/size/count. It has no authenticated producer
job, commit or job-result facts, nor an exact artifact-ID download selector.
Wiring a trusted GitHub consumer therefore needs a separately reviewed adapter
boundary, rather than treating producer-supplied JSON as authentication:

1. An independently protected caller owns policy-test selections, producer hook
   selection, immutable engine/runtime pins and permitted release source.
2. Build/check hooks execute only in a key-free job. Mandatory selected policy
   checks and the producer must succeed before it emits handoff outputs.
3. The trusted consumer authenticates the expected repository, source commit,
   exact workflow run/attempt, producer job identity/result and immutable
   artifact ID against provider records and the caller's outputs. It must refuse
   foreign/stale facts before consuming a local payload, not search for a latest
   successful artifact or accept an arbitrary downloaded path as proof.
4. Only after source/artifact/pin admission and this staged payload admission
   may a separately scoped signer load the Cosign and package-GPG materials.
   Trusted jobs must execute no consumer hook, checkout script or build tool.
5. Publishing must consume the finalized native manifest and the exact signed
   handoff, with separately scoped publication credentials; no rediscovery or
   generic GitHub assembly/ZIP/checksum regeneration is appropriate.

No reusable workflow or provider acquisition API is exposed by this slice. The
classified CLI routes do not add authorization or scheduling guarantees. Those
guarantees and their negative tests for foreign source/artifact facts and failed
producer/policy jobs remain pending.

## Explicit image limitation

This unit refuses image/base lifecycle ledgers, publication ledgers and
`premade-sboms/`; it does not silently flatten or omit them. It cannot interpret
a consumer's image declarations because it takes no consumer configuration.
Any future file-only workflow selector must also refuse declared images, bases
or flavors before running a producer—even if the producer omits their ledgers.
Image-aware publication requires the existing native ledger, pinned SBOM,
platform/flavor fan-in and promotion owners to be wired separately.

Consequently this is **not** Gommitlint's complete 81-asset/two-image contract,
not Nanolinter's full GNU/musl/flavor release, and not an upgrade of the accepted
reduced GitHub trial to full parity. Logical product inventory can match once
the same producer is integrated. Identical product bytes still require
identical source/build metadata/toolchains and reproducible settings. Forge
provenance/signature metadata may legitimately differ. Exact pinned SBOM bytes
must survive each handoff; independent cross-forge SBOM regeneration need not
produce identical bytes.
