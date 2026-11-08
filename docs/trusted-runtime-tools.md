<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Fixed tools for an independently selected canonical CLI

This narrow software change is based on actual committed
`76609471f3d902e968e3b2bc01e93cd43627846a`. It fixes runtime qualification gap 1:
the former strict `/usr/bin:/bin` hid the approved runtime's gh, Cosign,
git-cliff, Go and Cargo. It does not qualify an image, provide cold modules,
select production pins or activate a release.

`scripts/ci/trusted-runtime-tools.sh` is the single fixed tool-profile owner.
Its first argument names one of three finite profiles; the admitted workflow
YAML states it as a literal (`tool-profile` on the consume action, or the
literal in a producer step), and an unknown name is refused:

```text
runtime         PATH=/usr/local/go/bin:/usr/local/cargo/bin:/usr/local/bin:/usr/bin:/bin
                RUSTUP_HOME=/usr/local/rustup
                CARGO_HOME=$HOME/.cargo
host            PATH=/usr/bin:/bin
host-toolchain  PATH=/usr/bin:/bin:/usr/local/bin
```

`runtime` is the approved runtime Containerfile's fixed tool directories, for
every receiver whose job container is the admitted image. `host` is a hosted
runner's root-owned system tools (bash, git, gh, jq, gpg, python3 in
`/usr/bin`), for the plain-runner jobs that hold signing keys or the release
token: version-bump, the GPG signer and the publisher. `host-toolchain` adds
the hosted runner's documented product-toolchain directory and is used by
exactly one job, `prepare-version-files`, whose CLI runs the product's own
version tools (`mvn`, `npm`) and holds no key. Those tools are unqualified by
design: on the hosted image `mvn` resolves into a world-writable Maven tree
and `/usr/local/bin` is world-writable. This is an explicitly stated
trusted-platform premise, not a verified property; the privileged
`version-bump` job re-verifies that job's output by exact artifact ID and
digest before any private effect, so a tampered tool changes a file the next
job can refuse, never a key operation. No profile reads a caller PATH or
tool-directory input, and no workspace, current directory, home bin,
job-install directory or source-tree/Mise shim is searched. The image-owned
directories must be immutable/trusted in the qualified runtime; a profile does
not authenticate an arbitrary image or a writable tool directory. There is no
SDK CLI identity comparison and no installer/download fallback.

The existing runtime Containerfile installs Go at `/usr/local/go`, Cargo's
executables at `/usr/local/cargo/bin`, and Rust toolchains at `/usr/local/rustup`.
Its build-only CARGO_HOME argument is **not** a runtime requirement to store job
registry state in `/usr/local/cargo`: job state belongs in its own HOME/.cargo.
The fixed RUSTUP_HOME locates the installed image toolchain even when a key-free
producer starts with a new private HOME. Cargo toolchain/target availability and
permissions still require qualification; this script installs nothing.

## Admission before tool use; CLI identity is separate

Bootstrap identity, origin, committed-file/raw-content checks and CLI hashing
still run with system `/usr/bin:/bin`. The new helper is itself included in the
existing committed regular-file/raw-hash admission set. Only after the original
CLI's strict hash check does bootstrap execute the admitted helper with the
**absolute verified CLI** as its command. The helper rejects bare `forge-ci`;
neither it nor bootstrap discovers a CLI on image PATH. An older image CLI can
remain installed without being selected. Every canonical launcher call repeats
the existing verification boundary. Protected callers must continue using that
launcher/absolute verified entrypoint, not run an image CLI directly.

Both native metadata callsites execute `gh api` through the same helper after
delivery `prepare` admitted its bytes, before any CLI use. Original native token,
host/repository/run/request-head expectations and strict download checks are
unchanged. The canonical workflow has only this small metadata-callsite change;
other owners' builder/image/module-preparation work is not copied here.

The classified product step invokes the same admitted helper under `env -i`,
with only its new private HOME and existing P/tag/epoch/output data. It does not
inherit host/toolchain homes, caches, registry credentials, checkout/publication
tokens or key variables. Its executable remains the administrator-selected
absolute prepared-source path. Product hooks remain key-free; the helper neither
authorizes that product nor makes it a trusted signing program.

## Tests and limits

The original nineteen bootstrap security scenarios remain; new cases reject
modified/hidden/missing/linked tool helpers. The real strict helper and unchanged
hash owner execute committed inert CLI fixtures while hostile caller PATH and
FORGE_CI_BIN/PATH/PLAN values cannot select old code. Tool fixtures are stored in
an owned image-shaped temporary tree. Only the inert payload/mock exec boundary
translates the actual fixed PATH into that tree; production scripts are not
rewritten, and there is no test-only environment override in production. This
tests cold resolution without writing host `/usr/local`, a container or real SDK.
Metadata owner/order guards and the actual cleared producer step exercise the
same helper, including fixed Cargo homes and absence of sentinel credentials.

Privileged Bash entrypoints are **not global environment sanitizers**. Ordinary
descendant Bash, a trusted startup environment, authenticated fresh checkout and
object storage, and no concurrent writers remain prerequisites. This is not
hostile-runner isolation, tool binary qualification or live signing acceptance.

Cold selected-engine module preparation and a separately qualified canonical
builder image remain independently owned follow-ups. No compiler/product build,
network, provider, key, container, image pull or production pin change is part of
this unit. These shell/YAML/unembedded-doc/test changes do not alter canonical
Go inputs; ordinary generated run-by/docs sync checks must still pass. If a later
combined integration changes embedded inputs, its owner must rebuild the actual
canonical checksum records rather than reuse another source's bytes.
