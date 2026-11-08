<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Prepared release / remote request binding

`release validate-tag --expected-parent Q` is the existing opt-in to binding a
prepared release P to an explicitly authorized request commit Q. No new flag,
request schema, Git port or workflow route is introduced. Calls without
`--expected-parent` retain their existing final-tag validation behavior.

In expected-parent mode, the validator now requires:

1. P differs from Q and has exactly one parent, Q (no merge or unchanged source).
2. The local `refs/tags/release-request/<final-tag>^{commit}` equals Q.
3. The exact **unpeeled** request object on the original remote matches the local
   request object. Missing, replaced or unreadable remote refs fail closed.
4. The remote request's **peeled commit** equals Q. An annotated tag's object ID
   must not be compared with the commit SHA; the existing Git adapter's exact-ref
   queries provide those two distinct observations. Lookup failures propagate.

The remote argument is the same original `--repo-url` argument used for final-tag
validation (or `origin`), with the same caller read credential. No expanded URL,
payload field or tag name supplies Q. The fixed request tag name comes from the
native release-request prefix and the validated final version. Raw object equality
also refuses replacing a request annotation while keeping its peeled commit Q.

For the file-only profile, add `--changelog-only`; this permits only
`CHANGELOG.md` to differ. With `--prepare-plan-file`, the release's
prepare-stage plan, it also admits that plan's version bump: each planned
project's changelog and its native version files, checked as a version
substitution to the tag's version, and a non-native project's own version
files. Nothing outside the planned projects' files may change.

The check returns failure before emitting success; it has no private-key,
signer or publication capability. Callers must run it before those capabilities,
then repeat it at the publication boundary. Git observations are not an atomic
lease preventing subsequent remote mutation; the caller must preserve the
separate freshness/authorization checks at each trust boundary.

**This does not authorize a signature.** Existing signed-request and final-tag
signature/allowlist verification must still run with administrator trust material.
Tests intentionally use unsigned annotated/local tags: they demonstrate exact-ref
and parent/change-set semantics, not cryptographic validity. A lightweight request
can satisfy source equality here but cannot bypass its separate signature owner.
The local request object must be the one checked by that authority in the owned
metadata checkout; this validator does not authenticate a consumer-supplied trust
root or defend a concurrently compromised checkout.

Trusted original-job attempt capture, protected workflow ownership, immutable
bootstrap selection and the eventual classified policy/producer/sign/publish
graph remain separate reviewed units. No workflow or production pin changes here.
Integration requires canonical CLI binaries/checksums built from the actual
combined source; earlier-source hashes must not be copied forward.

## Native GitHub late boundaries

The native orchestrator carries the original event `github.sha` as Q to its
final-tag gate and `release-create-github.yml`, separately from prepared P and
assembly outputs. The release leaf requires an immutable `authorized-source-sha`;
it never derives authority from downloaded assembly assertions.

After canonical engine bootstrap, the signing and publishing jobs use separate
engine-created sparse metadata checkouts at Q (public trust files) and P (Git
objects). These steps make no explicit consumer script, build-tool or local-action calls.
These Git operations require trusted runner Git startup configuration and
templates; the checkout adapter does not force `core.hooksPath=/dev/null`, so
this metadata path is not a hooks sandbox.
Immediately before private key setup and immediately before publication, the
native GPG/SSH signature validator verifies the request with Q's trust files and
the configured allowlist requirement. The final signature is verified too (GPG
uses only the configured public release key). The last command rereads the exact
remote request object/peeled Q, exact final object/commit P, and supersession via
`release validate-tag --expected-parent Q`. Only the source-read job token and
public verification material are in scope for these checks.

Before bootstrap/metadata checkout or request/final-tag validation in either
consumer job, a workflow scalar gate requires Q to be exactly 40 or 64 lowercase
hex characters (SHA-1 or SHA-256). Empty, whitespace, short and malformed values
fail explicitly, without falling back to the event, prepared P or assembly
metadata. The orchestrator applies the same gate to the original event SHA
before its initial checkout and final-tag validation. Git checkout and the
existing native validators then establish object/ref equality in the actual
repository. Optional-Q legacy CLI calls retain their existing behavior; the
workflow's required-Q contract does not change that CLI default.

The explicit no-bump plan sets `RELEASE_REQUEST_IS_RELEASE=true` (CLI
`--request-is-release`). This requires P=Q and still checks remote request
identity; it is not an optional parentage bypass for prepared children. This
mode requires Q and rejects prepared-change profiles. Ordinary prepared and
recovery routes retain the single-child rule. An exact-P publication retry
reuses the signed payload and reruns these checks without requiring a new P or
an absent final tag. Existing `--changelog-only`/`--prepare-plan-file` profiles remain
unchanged for callers selecting them; the general native multi-artifact prepare
plan is not silently restricted to the root Go/Cargo file-only profile.

These are fresh observations, not an atomic remote lease or a substitute for
authorization. A revoked/deleted request, retagged annotation (even at the same
Q), invalid/unsigned request, moved final P or superseding final tag refuses
before the corresponding private operation. This change does not activate a
classified route, production pin, or new trust root.
