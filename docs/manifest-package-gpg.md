<!-- SPDX-FileCopyrightText: The Forge CI Authors -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Manifest-backed package GPG selection

The existing package signer has an explicit opt-in:

```sh
forge-ci release gpg sign-packages --dir dist \
  --manifest dist/release-files.json --fingerprint "$PACKAGE_GPG_FINGERPRINT"
```

The existing private-key/passphrase channels remain unchanged. The fingerprint
is independently administrator-reviewed input, not a manifest assertion. Without
`--manifest`, legacy top-level `*.deb`, `*.rpm`, `*.apk` selection and stale-sidecar
cleanup remain unchanged; nested files are not newly selected by default.

With `--manifest`, the existing strict bounded `FileManifest` reader is shared
with classified admission. All manifest sections, regular-file paths, duplicate
paths, publication names and the complete expected checksum subject set/digests
are checked using existing release validators before key reads, imports or GPG
processes. The CLI preflights before resolving secret files or creating a GPG
home; the application rechecks before reading its key material. No discovery or
fallback occurs when an explicitly named manifest is missing or invalid.

Selection is exactly the manifest's `assets` entries ending in `.deb`, `.rpm` or
`.apk`, in declared order. Paths are canonical relative paths under `--dir`;
traversal, path aliases, symlinks and nonregular files are refused. Nested names
are preserved and signatures are written alongside them as `<exact-path>.sig`.
Unlisted packages and other assets are never signed by this role. The manifest,
checksums and producer SBOM bytes are not rewritten.

This opt-in requires an unsigned workspace. Existing signer provenance and
declared signatures are refused using native unsigned validation; all existing
`.sig`/`.bundle` files, even unlisted sidecars, are refused before keys. Symlinks
or nonregular files anywhere under `--dir` are also refused. Unlisted ordinary
files are ignored, not admitted as part of a closed payload. Failed preflight
does not delete stale outputs or partially sign earlier valid entries. A retry
after partial signing needs a fresh unsigned workspace. Legacy behavior is not
changed by these stricter opt-in checks.

Run package GPG before the separate Cosign/provenance steps. Successful package
signing still requires fresh nonempty regular sidecars from the backend. The
existing finalizer collects the declared packages' sidecars; it remains the
owner of publication inventory. This change neither substitutes GPG for Cosign
nor changes their independent roles or native finalization.

This is selection/preflight, not manifest self-authorization, source approval,
closed classified-payload admission or protection against concurrent hostile
workspace mutation. Use the separately admitted private workspace; do not run
consumer code concurrently with trusted signing. No workflow is activated here.
Tests use inert signers, not real keys or proof of cryptographic validity.
Future integration must build canonical binaries/checksums from the actual
combined source; no production pins are refreshed by this candidate.
