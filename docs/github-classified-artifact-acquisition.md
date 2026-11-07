<!-- SPDX-FileCopyrightText: The Forge CI Authors -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Classified artifact acquisition: bounded exact-ID port seam

This is a seam-first unit, **not a complete classified acquisition harness**.
The committed native artifact port had no opt-in compressed-byte cap. Per the
task's stop condition, this unit adds that missing capability instead of using
the native 10-GiB download budget temporarily. It introduces no transport, ZIP
parser, credentials, artifact-selection heuristic or launcher activation.

## Opt-in contract

`provider.RunArtifactDownload` accepts optional `RunArtifactLimits`:
`MaxArchiveBytes`, `MaxExpandedBytes` and `MaxEntries`. All must be positive and
no larger than the existing native constants. Bounded mode requires exact ID,
name, repository, run ID, full expected run-head SHA and the independently known
`ExpectedRepositoryID` of the created consumer. Empty ID cannot carry limits or
exact-only expectations. Unsupported Forgejo exact mode also refuses these new
fields, including when supplied without an ID.

The intended classified caller must set both byte limits to **32 MiB** and the
entry limit to **128**, independently for each original producer and signer
artifact. Tests exercise these actual 32-MiB budgets through the native GitHub
reader/extractor, including aggregate expanded overflow and compression bombs.
These per-artifact compressed/expanded budgets are not a global peak-disk cap.

Before requesting an archive URL or creating extraction staging, the existing
scoped GitHub client must observe the expected repository ID/name and exact
artifact ID/name, explicit unexpired state, exact workflow run/repository/head
repository/head Q, and explicit positive `size_in_bytes` within the compressed
case budget. P is **not** the request run head. Any supplied API archive digest
is checked against actual bytes; absent digests retain native optional semantics.

Only the exact artifact ID is downloaded. No fallback to name, latest, pattern
or a newer run occurs. The observed size becomes an exact compressed-stream
bound as well: deceptive short/oversized responses fail, and the sentinel byte
used for overflow detection is never written. Content-Length cannot enlarge
the admitted budget. Temporary ZIP files are removed on failure and success.

The existing HTTPS URL/redirect guard and credential stripping remain in use;
no ambient cookies or API authorization are sent to a redirected blob host.
The same native ZIP reader and descriptor-rooted transactional extractor handle
the payload. Bounded mode preflights entry count, aggregate declared expansion,
canonical relative paths, duplicate paths and regular-file/directory types.
Traversal, absolute/Windows paths, aliases, symlinks and special files fail.
Actual extraction is also byte-bounded while writing, with cancellation checks;
it cannot expand an entire oversized tree before noticing the cap. A late bad
entry, corrupt/truncated ZIP, CRC/size error or cancellation installs no payload.
Native private extraction staging is removed, and a retry creates fresh staging.

## Defaults and integration boundary

With `Limits == nil`, native byte/count defaults remain unchanged. Existing
exact-ID metadata admission still accepts missing size metadata when no case
limit was requested. No public CLI flag or plan schema changes are introduced.
The port/source change needs a fresh canonical build at combined integration;
do not reuse unrelated binary pins or qualification records as its identity.

The port accepts a destination from its caller. It does **not** itself establish
that the destination is a fresh private root outside every checkout. Native
transactional extraction prevents partial payload installation but may leave
an empty destination/ancestor after failure. A classified caller must use the
existing acquisition/temp-root owner, ensure disjoint roots, remove its entire
owned failed acquisition, and register successful cleanup. That wrapper is
deliberately not fabricated before the required attempt/source proof exists.

## Facts the artifact API cannot prove

The API observation is a **readback identity**, not independent producer-job
authentication. It contains no job identity, successful job result, attempt
number or prepared source P. Archive hashes do not supply those missing facts.
The future caller must bind IDs to original producer/signer observations using
the frozen trusted workflow graph plus exact run/attempt/job observations from
the separate attempt owner. Those mandatory caller-proof inputs and their
stale/foreign-attempt refusal remain unimplemented here.

If a trusted upload output exposes the original artifact ID, preserve it. If
only run-scoped listing is available, an exact unique expected name can select
an ID for readback, but that selection is not proof of the producing job. No
such selection is added by this unit, and provider APIs do not expose arbitrary
job step outputs as a substitute. Once selected, pass that exact ID to the port.

Remaining: reviewed original-ID selection/proof, fresh outside-checkout producer
and signer acquisitions, bounded existing-owner evidence retention, combined
oracle/source/attempt integration, key-before-Q staging and final-tag/P proof.
The separate source-serving and attempt worktrees and frozen oracle candidate
are untouched. There is no `githubtrial.go` overlap; integration overlap is in
the native artifact port/adapter files listed by this candidate.

## Offline verification

Focused races use the existing in-memory fake GitHub/HTTP transport and actual
native ZIP reader/extractor. No DNS, provider operation, PAT read, real key,
container, product rebuild or live TestMain is used. Tests cover metadata refusal
before ZIP requests, repository identity, caps, malformed paths/types, duplicates,
declared/actual size mismatch, truncated/digest-tampered archives, cancellation,
cleanup and successful retry. Existing exact-ID redirect/identity tests and
native artifact tests remain compatibility evidence, not hosted acceptance.
