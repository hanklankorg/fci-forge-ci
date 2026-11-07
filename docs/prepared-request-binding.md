<!-- SPDX-FileCopyrightText: The Forge CI Authors -->
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

For the initial file-only root Go profile, add `--changelog-only`; this permits
only `CHANGELOG.md` to differ. For root Cargo, also pass `--version-bump cargo`;
the existing version-substitution validator checks root Cargo version files and
the changelog, not arbitrary source/config changes. Cargo files below subdirectories
are refused in this bounded profile. Existing Maven/npm bump handling is unchanged.

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
