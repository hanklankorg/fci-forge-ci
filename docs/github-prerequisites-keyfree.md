<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Validation-only release prerequisites

`validate-release-prerequisites.yml` never imports a private GPG key, resolves a
private-key/passphrase file, enables Git signing or cleans up a private keyring.
This holds even when the plan selects GPG for Git objects or release artifacts,
and when request signature/allowlist validation refuses the request.

Public request verification remains enabled. The validator receives the public
verification key and existing signer/allowlist policy; it may use public-key
verification mechanisms. Maven Central's signing-key availability check and the
summary receive only booleans computed by Actions, not private key values.

The reusable workflow retains its optional `RELEASE_GPG_PRIVATE_KEY` and
`RELEASE_GPG_PASSPHRASE` declarations for caller compatibility and presence
reporting. Neither value is mapped into a job process. This does not remove other
existing prerequisite credentials, such as the release-token access probe.

Actual preparation and artifact/package signing still own their independent
private-key setup, fingerprint checks and cleanup; those jobs are unchanged.
This fixes the native prerequisite ordering defect without activating a new
release route or claiming producer, inventory or hosted-release acceptance.

Tests combine a structural guard over the actual workflow, its native commands
with an inert CLI/private-effect counters, and actual prerequisite use-case
refusals with a public verification stub. They use no real private key or GPG
process and make no cryptographic-acceptance claim. No CLI flags or command
behavior change. Removing these workflow calls does change the generated,
embedded `internal/cli/runby.json` help ownership table: it is refreshed with
`cmd/gen-runby`, not a hand-maintained claim that prerequisites still imports
keys. Integration therefore requires legitimate canonical binary/checksum builds
from the actual combined source for both architectures; no pins or checksum
records are refreshed by this candidate.
