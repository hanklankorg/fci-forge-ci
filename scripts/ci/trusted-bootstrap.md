<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Key-free trusted leaf bootstrap

`trusted-bootstrap.sh` is a narrow exec helper for an administrator-owned leaf,
not a public workflow route. Use an administrator-pinned copy of the helper,
not a local consumer action or consumer-selected script. Launch `/bin/bash`
with no consumer-controlled startup environment (`BASH_ENV`, exported functions,
loader variables, etc.). The approved runner's `/usr/bin:/bin` is trusted; SDK
and consumer PATH entries are not used. No image/toolchain qualification is
implied. In particular SDK 368's Java/amd64 qualification does not qualify Go,
Cargo, cosign, or a general release image.

The protected caller supplies **all four** positional inputs from its approved
graph: a separate frozen engine checkout, its HTTPS origin, its full commit,
and an absolute delivered executable file. A repository URL is an assertion
checked against local Git metadata, not proof that an administrator approved
that repository or authentication of an arbitrary supplied Git database. The
caller must freshly obtain the checkout through its trusted checkout action,
including trusted local configuration and complete local object storage. HEAD
and origin strings alone cannot establish that provenance. Keep the checkout,
Git metadata/object storage and delivery read-only to other processes for the
entire verification/exec interval; there is no TOCTOU protection against writers.
The helper checks the origin/commit and
committed helper/checksum contents **before sourcing** the engine verifier.
It uses the engine checkout's `.engine/bin` checksum, never a consumer digest.
The existing strict verifier checks bytes before **any** CLI invocation,
including `--version`, and the helper execs only that absolute file.

Only consumed verifier/checksum files are compared with committed blobs, using
raw `hash-object --no-filters` hashes and regular-file modes from `ls-tree`.
`rev-parse` reads identity/object names; `config --local --no-includes --get`
reads the literal origin. These are non-worktree-inspecting plumbing reads,
with paging, replacement objects and lazy object fetching disabled. They do
not run checkout hooks, clean/textconv filters, fsmonitor, external diff tools,
or submodule inspection. Whole-checkout cleanliness is a fresh trusted checkout
**caller prerequisite**, not a claim checked by this helper. In particular,
it deliberately does not invoke `status` or `diff`, nor maintain a purportedly
complete executable-config denylist. Unrelated worktree changes are not inspected.

## Integration dependency (not provided here)

Before calling this helper, the protected graph must provide the canonical
Linux delivery binary of the approved engine checkout for the host's
architecture (amd64 or arm64; the helper refuses any other). Existing
`build-cli.yml` produces `engine-cli/forge-ci-linux-amd64` using
`scripts/ci/engine-binary.sh build` and `verify`. Alternatively a separate
key-free job can run those same commands with the approved, pinned Go toolchain
and cached modules. Do not use `go install`, an SDK's embedded CLI, a stamped
release tarball or a latest-ref fallback.

For GitHub, use the existing pinned native artifact actions for delivery, **not
the forge-ci CLI downloader** (which is not bootstrapped yet). The repository
pins upload-artifact `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`
and download-artifact `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c`.
The integration must expose and bind the producer's immutable artifact ID and
download that exact ID from the same protected graph/run with the fixed action;
existing name-based downloads alone are not that integration. Preserve the
native action's archive safety/size controls. This helper accepts an already
delivered file and neither extracts archives nor implements transport. Native
artifact download may restore mode 0644: the protected delivery step must set
the expected regular file executable before calling the helper. No CLI runs
to do so. The byte check still applies. Missing delivery fails closed, with no
network fallback. The Forgejo/GitLab generic-package locator is **not** a
GitHub locator and is not used here.

Required leaf order (caller-owned orchestration, deliberately not added here):

1. On a clean key-free job, checkout the approved engine by full commit with
   the pinned checkout action, separate from consumer work; no credentials
   persisted and no consumer checkout/code/startup environment yet.
2. Deliver the canonical CLI through the approved exact-ID graph above (or
   build key-free by the canonical recipe). Keep delivery outside the engine
   checkout. Set its executable mode using trusted runner tools.
3. Invoke `/bin/bash /ADMIN_PINNED_COPY/scripts/ci/trusted-bootstrap.sh
   /ABSOLUTE_ENGINE_CHECKOUT https://APPROVED_HOST/OWNER/ENGINE FULL_COMMIT
   /ABSOLUTE_DELIVERY/forge-ci-linux-<arch> -- --version` as separate shell argv.
   These are placeholders, not published production coordinates.
4. Only after success may later stages checkout consumer data or enter private
   environments. Use the same helper for later CLI invocations with explicit
   administrator arguments; do not fall back to `forge-ci` on PATH. Rejection
   of a bad checkout/delivery occurs before the CLI process is started.

`FORGE_CI_BIN`, `FORGE_CI_PATH`, `FORGE_CI_BINARY_REF`, and `FORGE_CI_PLAN` are
cleared in the helper process. `SIGNER_STRICT` is forced during verification;
the path comes only from the explicit delivery input. Other administrator
inputs are retained, not blanket-cleared; callers must authorize CLI flags,
plans, environment and secret exposure separately. This helper is not a
consumer-code sandbox, release authorization, or request/attempt binding.
There is no production image/ref approval, live validation, or orchestrator
completion claimed by these files.
