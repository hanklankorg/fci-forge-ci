# Resource-aware asynchronous live campaigns

## Plan before launch

Build a short lane table with provider/road, case, frozen candidate, prerequisites,
mutable repository/package coordinates, expected peak RAM/CPU/disk, and evidence
destination. Read Forge Lab's current `AGENTS.md` and documented recipes before
operating its providers or runners.

An unpublished consumer draft may use exactly `PIN_FORGE_CI_SHA` in every layer
reference. The consumer-trial planner replaces that marker, including policy-test
literals, with the frozen lab layer commit before serving any workflow. Mixed
draft/commit pins, arbitrary markers and branches are refused; full SHA-1 and
SHA-256 layer commits are supported. This is lab rendering, not a relaxation of
production immutable-pin checks. Retain reviewed public signer/key literals for
PR-only trials, whose isolation gate reads but never executes them. Release
trials replace those literals with the lab's verified signer and key facts;
unresolved trust placeholders are not valid PR isolation inputs.
Both source checkouts must still be clean and frozen before a live trial; do not
bypass source, target, acknowledgement or cleanup admission to run a dirty draft.

Inspect host available memory, CPU/load, free disk, active jobs and runner
concurrency/limits. Forge Lab's read-only doctor/status interfaces and host
metrics provide the observations; configured runner limits and previous measured
peaks provide the budgets. Leave explicit headroom for the forges, compiler work
and cleanup. Unknown memory cost is a reason for a conservative budget, not for
assuming every CPU can host another Maven build.

Choose ready cases that fit the remaining budget. Independent Forgejo, GitLab
and hosted GitHub work should normally overlap. Hosted jobs consume remote
runner capacity, while their local orchestration, compilation and image
preparation still consume local resources. Reassess after each completion or
resource failure and start another compatible case instead of waiting behind an
unrelated provider.

## Execution scopes and locks

Focused local suites support `FORGE_CI_LIVE_PROVIDER=forgejo` or `gitlab`.
Selection is checked by Go preflight, runtime provider selection and runner
architecture selection. A single-provider run cannot claim the full profile.
The retained report names its execution provider and preserves the complete
contract identity/provenance. Read each lane's outcome from the `verdict.json`
beside its log; an `infrastructure` verdict is the lab's failure, not the
product's: fix the lab and run the case again rather than counting it either way.

The launcher takes the existing `live.lock` shared and its provider lock
exclusive. Different providers can overlap; the same provider remains exclusive
because fixtures use fixed repository names. Unscoped/all-provider and older
launchers take `live.lock` exclusively, so mixed old/new runs still conflict
safely. Locks remain held through test cleanup and are not inherited by suites
or loggers. Never unlink a lock file or split HOME/XDG to create concurrency.

For GitHub, use the guarded Forge Lab hosted session recipe and its own
organization lock; shared SDK preparation precedes parallel read-only reuse by
independent consumers.

Hosted GitHub consumer **release** trials require
`FORGE_CI_LIVE_GITHUB_RELEASE_TOKEN_FILE` to name an owner-private regular file
containing a fine-grained PAT. Only its path enters the environment; the token
is read natively with a 4096-byte bound and uploaded only as the consumer's
`RELEASE_TOKEN`. There is deliberately no fallback to the hosted session's
management/Packages token. Missing or invalid input refuses before hosted
resources are created. Pull-request, development-release and engine-only
`FORGE_CI_LIVE_ENGINE_MODE=prepare` runs do not require this file. The guarded
session and SDK/GHCR publication keep their existing credential unchanged.
Token kind admission does not prove repository selection or write permissions;
the ordinary release workflow and its content-write assertions remain authoritative.

## Run a campaign

Prepare once, from a clean checkout whose engine checksum is fresh, naming
each lane the campaign will run as `<provider>=<cases file>`. It publishes the
kaniko job, signer and named nanolinter job images with Forge Lab, saves the
signer image, the Buildah and kaniko job images
(`FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE`, `FORGE_CI_LIVE_KANIKO_JOB_IMAGE_ARCHIVE`)
and the runtime images as archives, exports every demo as a one-commit
checkout and writes `<dir>/campaign.env` (references and paths, no secrets,
and `CAMPAIGN_COMMIT`). The runtime images are those the named cases need,
each as `FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_<TARGET>`: every one a case names as
`@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_<TARGET>`, and on GitLab the scenarios' go
image, also `FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE`, and every
`$CI_JOB_IMAGE_<TARGET>` the gate of a named demo checks in
(`go run ./internal/livetest/preflight campaign-runtime-targets . gitlab=<cases file> flavor=<flavor> ...`
lists them). Each nanolinter job image is published as `JOB_IMAGE_<FLAVOR>`
and saved as `LINT_IMAGE_ARCHIVE_<FLAVOR>`, the flavor in capitals with `-` as
`_`; a case naming a flavor the campaign is not given is refused before
anything is built:

```bash
just campaign-prepare ~/src/forge-lab ~/campaigns/k3s \
  forgejo=test/live/campaigns/k3s-private-forgejo.cases \
  gitlab=test/live/campaigns/k3s-private-gitlab.cases \
  job-image=go:<local image> job-image=java:<local image> \
  job-image=javascript-typescript:<local image> job-image=rust:<local image>
```

Then run one lane a provider, side by side, against fresh forgeci targets
(minted with `LAB_ROAD=k3s` in Forge Lab for that road). `LAB_TARGETS_FILE`,
`FORGE_CI_LIVE_CONSUMER_VISIBILITY` and the GitLab image builder,
`FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=buildah|kaniko`, stay the caller's; the
builder is the whole GitLab lane's, kaniko for the k3s cases. The image
archives are the campaign's or the case's own: the lane clears a caller's
`FORGE_CI_LIVE_LINT_IMAGE_ARCHIVE` and any image variable campaign.env does not
set.

```bash
export FORGE_CI_LIVE_CONSUMER_VISIBILITY=private
export FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=kaniko
export FORGE_CI_LIVE_ADMIN_LINT=false
export FORGE_CI_LIVE_ADMIN_VULNERABILITIES=false
export FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS=false
export FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true
just campaign-lane ~/campaigns/k3s forgejo test/live/campaigns/k3s-private-forgejo.cases &
FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko \
  just campaign-lane ~/campaigns/k3s gitlab test/live/campaigns/k3s-private-gitlab.cases &
wait
```

A cases file can declare expected run settings before its first case:

```text
setting visibility=private
setting image-builder=kaniko
setting admin-lint=false
setting admin-vulnerabilities=false
setting admin-application-tests=false
setting admin-reproducible-builds=true
```

The lane checks these declarations against the caller's effective settings before
running any case. Declarations do not override the environment. Missing visibility
means `both`, and an unset provider-specific image-builder selector means `buildah`,
as in the scenarios. Each Forgejo consumer case also resolves its own
`CI_IMAGE_BUILDER` (default `buildah`); it must match a declared builder and any
explicit Forgejo run selector. A conflicting later case refuses the entire plan,
before earlier cases execute or the conflicting trial creates repositories.
Unknown declarations, mismatches, duplicate declarations and
settings after a case are refused. Preparation validates the declaration syntax
before building images. The checked-in private k3s lanes require private visibility
and Kaniko; set the corresponding Forgejo or GitLab image-builder selector.
The four administrator settings are independent and default to `false`. For these
lanes the controller sets lint, vulnerabilities and application tests to `false`,
explicitly enabling reproducible builds as their declarations require. Each controller variable accepts exactly
`true` or `false`; invalid values refuse preflight. Checks still run when their
findings are nonblocking.

The harness supplies literals only in the wrappers and independent GitLab policy
projects it owns. Consumer source checkouts and repository variables receive no
enforcement settings. GitLab MR gates are snapshotted into a separately pinned
policy project, including the MR gate used by a release trial. Development trials
preserve their root push workflow and jobs in that policy project, with its local
gate include bound to the captured administrator gate. Forgejo and GitHub wrappers
receive only the settings declared by the called engine workflow/action at its
exact pinned Git commit, including when local uncommitted files differ.
The run header and verdict record the controller's four `FORGE_CI_LIVE_ADMIN_*`
inputs. The embedded engine default stays report-only; no lab-only policy source
edit or checksum rebuild is needed to choose the run's settings.

A cases file then holds one case a line: `consumer <demo> <pull-request|release>
[key=value ...]` or `scenario <TestName> [key=value ...]`. Explicit scenario inputs
are supported by the GitLab recipes described below. A value `@NAME` is campaign.env's
`NAME`; a GitLab consumer names its archives with `job-archive=` (and for a
release `build-archive=` and `signer-archive=`), for example
`job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA`, which a campaign prepared
with that cases file has, and a release runs its `job-image` jobs as the
runtime images' `nonroot` with `job-user=65532:65532`. A GitLab consumer whose
gate includes the nanolinter component names its flavor's archive,
`lint-archive=@LINT_IMAGE_ARCHIVE_JAVA` for example, for that case alone.
Every runtime image archive and the Buildah and kaniko job images reach every
case, so a gate's further job images and a release's image jobs find theirs. The live preflight reads both
files as data and plans the lane (`internal/livetest/campaign.go`); it refuses
a checkout other than `CAMPAIGN_COMMIT` and any case it cannot read before the
first runs, among them a case whose archive, or a further job image of its
demo's gate, the campaign was not prepared with, and a GitLab consumer whose
gate lints without `lint-archive=`. The lane then runs past failures and fails when any case did. Read the outcome from
`<dir>/<provider>.results`, one tab-separated line a case: interval, case,
exit status and the `verdict.json` to read, as above.

### Published development snapshot (Forgejo Maven service only)

`test/live/campaigns/published-snapshot-forgejo.cases` adds one standalone event:

```text
consumer hello-maven-service published-unsigned-snapshot job-image=@JOB_IMAGE_JAVA
```

Prepare with `forgejo=<that cases file>` and `job-image=java:<reviewed local image>`.
The Java job image must support the existing native Maven build/check recipe.
The consumer's declaration selects the existing route: artifact-first dependencies
are built and gathered by the native snapshot producer before the original
Containerfile wraps them; a declared container-only service builds from source
inside its original Containerfile. The fixture preserves that choice. A literal
full SHA-256 digest reference is also accepted. No other case settings are accepted.
The checked-in profile requires public visibility and Buildah. Declarations are
expectations, not overrides: an explicit conflicting builder refuses the whole
plan. The native driver uses the Forgejo run selector (default Buildah), not an
inherited consumer `CI_IMAGE_BUILDER` setting. A separately declared Kaniko run
still requires its existing reviewed archive. Both builders require the contract's
container/readiness runner, because readiness runs the published digest with Podman.
Budget that runner and the builder before launch; this profile claims native Linux
amd64 only, not ARM images or a multi-platform product matrix.

Fresh scratch-repository bindings keep `CI_IMAGE_BUILDER=buildah` explicit and omit
its empty optional `CI_IMAGE_BUILDER_JOB_IMAGE`: Forgejo requires nonempty Actions
variable values, and Buildah uses the privileged runner's default image. This is
not an update/unset operation on an existing repository. A nonempty image remains
unchanged; Kaniko still requires its qualified digest image and reviewed archive.
Conflicting bindings and other empty required values refuse the complete merge.

The recipe calls the existing guarded launcher and
`TestForgejoLayer_ConsumerDevelopment` / `runForgejoMavenServiceDevelopment`.
It fixes branch `development`, readiness `/health`, an empty optional runtime
fixture and disposable repository mode. It overrides stale consumer event/settings,
ephemeral-snapshot image/dependency selectors and reused signing-key selection;
administrator settings, builder selection, targets, acknowledgements, resource
admission, locks, evidence retention and cleanup authority remain the existing ones.
Only the development registry credential is provisioned by the native trial.

Acceptance binds the frozen snapshot plan and image receipt to source, architecture,
registry digest/tag and readiness, rejects stable image aliases, and checks unchanged
policy main plus absence of Git tags/releases. This is **unsigned snapshot mode;
signature/attestation absence and scan readbacks are not independently verified**.
It is not the ephemeral `ConsumerSnapshotTrial`, a linked PR/snapshot/release run,
or full Gommi/Nano inventory acceptance. GitLab and hosted GitHub do not support this
campaign event and refuse admission; hosted GitHub keeps its separate guarded road.

Forge CI owns these CI consumer scenarios. Forge Lab owns its CLI/infrastructure
tests and provides the existing contract/SDK guards; its destructive live TestMain
is not a validation entry point for this profile. No scenario is duplicated there.

### Lifecycle (Maven service only)

`test/live/campaigns/lifecycle-forgejo.cases` and
`test/live/campaigns/lifecycle-gitlab.cases` run the linked flow of one
consumer in one scratch repository, the `lifecycle` event:

```text
consumer hello-maven-service lifecycle CI_JOB_IMAGE=@JOB_IMAGE_JAVA CI_IMAGE_BUILDER=buildah RELEASE_PLATFORMS=linux/amd64
consumer hello-maven-service lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVA version=0.2.0 version-bump=maven job-user=65532:65532
```

The same reader, `key=value` words and `@NAME` expansion as the other consumer
events; only the event word is new. On Forgejo the phases, each ending the
lifecycle when it fails, with their acceptance owners unchanged:

1. **Pull request.** The consumer is rendered once, with the release's public
   facts and the release requester's public allowed-signers staged in the
   private index before the base commit, so every commit of the flow carries
   the trust. Base to `main`, the change to `development`, a pull request
   judged by the consumer's own `pullrequest.yml`. No tag, release or other run.
2. **Merge.** The pull request is merged natively with a merge commit, the forge
   told which head commit is meant. A mirror then shows `main` at a commit whose
   parents are the base and the change and whose tree is the change's.
3. **Development.** `development.yml` is dispatched on the merged `main` and
   snapshots `development`; the acceptance is the published snapshot's (frozen
   plan, one amd64 image receipt and readiness, digest and tag, checked JARs).
   Only the development registry credential is set for it.
4. **Release.** The deploy key and the release secrets are set now, a signed
   `release-request/v<version>` is pushed at the merged `main`, and the release
   is judged as a release trial's: one signed release commit on `main`, signed
   tag, a release whose Cosign bundles verify against the trial's key.
5. **Final.** The repository holds exactly those three runs, two tags, one
   release and the bot's deploy key; `development` still points at the change.

On Forgejo the event takes the release's settings but `workflow=`,
`next-version=` and `interrupt-after=`; `CI_JOB_IMAGE`, pinned by digest, is
also the Java image the snapshot builds the service with. It builds with
Buildah for now: the campaign's kaniko job image is labelled for the consumer
trial's layer repository, which a lifecycle's layer is not, so
`CI_IMAGE_BUILDER=kaniko` is refused before anything is created. It needs what
the published snapshot and a Forgejo release need: the contract's
container/readiness runner, `FORGE_CI_LIVE_SIGNER_IMAGE`, ssh-keygen and gpg.

On GitLab (`TestGitLabLayer_ConsumerLifecycle`) the same five phases run in
one project under one pinned policy file, whose includes serve the three
pipelines: the demo's gate for merge-request pipelines, the `snapshot-images`
component for the push pipelines of `main`, the `release` component for the
request tag. The merge request is merged through the API, told the head
commit; the push that merge makes to `main` is the development phase, a
snapshot of `development` whose child pipeline builds the JAR in the build
image, builds the image with the selected builder and publishes it to the
project's registry; the acceptance is the frozen plan, one amd64 receipt,
digest and tag with no alias. The component has no readiness or checked-JAR
job, so none is claimed. The release is judged as the GitLab release trial's,
packages and images included. The event takes the release's archives and
settings but `scenario=` and `next-version=`. The credentials are protected
variables scoped to their jobs' environments: the merge request's pipeline, on
an unprotected branch, gets none; the snapshot's publisher the development
registry credential alone, set before the flow; the release's jobs the
release's, set at the release phase and not before, as on Forgejo. The bot's
deploy key alone exists from the start, because `main` is protected to it
before its first write. The project's pipeline configuration is set after the
base and the change are pushed, so the merge's push is the first pipeline on
`main`. GitLab resolves a policy only as it creates a pipeline, never ahead of
one: its project lint reads the project's own repository, not a policy in
another project, so a refused include or component input is read from the
pipeline's `yaml_errors`, as for every GitLab trial. The include rules'
variables are the documented ones, `CI_PIPELINE_SOURCE` and
`CI_COMMIT_REF_NAME`. The GitLab `snapshot-images` component has no standalone
trial: the lifecycle's development phase is its first conformance exercise,
so a failure there may be the component's rather than the flow's.

The event is admitted for the `hello-maven-service` demo alone, the one demo
with a development route; a demo without one is refused rather than run as a
shorter flow, and hosted GitHub refuses the event. amd64 runners only for now.
Standalone `test-live-consumer <forgejo|gitlab> <checkout> lifecycle …` runs
the same test outside a campaign. The standalone pull-request, release and
snapshot trials remain the single-phase entrypoints; the lifecycle calls their
extracted checks.

### Explicit-input GitLab regressions

The normal consumer release recipe also selects these exact scenarios with
`scenario=<name>`; the controller validates the selector before launch:

| Scenario | Recipe/input |
|---|---|
| `TestGitLabRelease_ConsumerTrial` | `test-live-consumer gitlab <checkout> release`; default scenario |
| `TestGitLabRelease_ConsumerBranchProtection` | `test-live-gitlab-branch-protection <checkout>`; no job images |
| `TestGitLabRelease_BuildImageCapability` | `test-live-gitlab-buildah <build archive>`; specifically Buildah and a privileged runner |
| `TestGitLabRelease_RejectsPreseededProvenance` | Consumer release recipe with `scenario=TestGitLabRelease_RejectsPreseededProvenance`; the harness adds the inert provenance fixture in its private index |
| `TestGitLabRelease_ConsumerInterruptedRecovery` | Consumer release recipe with `scenario=TestGitLabRelease_ConsumerInterruptedRecovery version=0.3.0`; reviewed single-image policy-owned release consumer required |
| `TestGitLabWorkflow_InterruptedJob` | `test-live-gitlab-interrupted-job`; manual observed-signal proof |

For provenance rejection, use an ordinary reviewed native-release consumer. Its
original build runs first, then the controller's wrapper inserts inert unsigned
provenance into `dist`. The source checkout remains clean. The test must observe
the specific handoff refusal, blocked signer/publication and empty release APIs.

Recovery requires a reviewed consumer using `release-images: true`, producing one
release image in the source project's root image repository, with the unsuffixed
version tag (no image flavor suffix). Its build produces files/image context only;
it must not write the registry or implement a pause. The old build-running checkpoint
is unreachable: policy image builds and `Images: publish` need successful
`Build: release files`. Retained consumers that publish inside the build are not
compatible and must not be reused as this case's fixture.

The harness generates a test-owned component from the exact captured engine parent
and pins it in the independent policy project. It preserves the engine SHA,
parent mirror/lock/forwarding policy and production child include, then adds a
separately pinned child extension. `Fixture: candidate checkpoint` runs after
successful `Images: publish`, without a credential environment or downloaded
artifacts. Signing keeps its original image-publication artifact dependency and
also waits for this checkpoint. Production templates have no fixture-variable
hook. The checkpoint prints `fci-live-candidate-checkpoint-ready` and holds while
its launch-time `FIXTURE_PAUSE_AFTER_CANDIDATE` value is true; expiry fails closed.

The controller observes the owned running checkpoint, reads the exact successful
publication job's immutable `_release-images/dist/release-images.json` receipt,
and checks the source `staging-v<version>` tag against its repository and digest.
It cancels the original child before signing, proves no release/new final image
and unchanged prior `latest`/`v0.0.0`, clears the project variable, and retries that
same child. Successful preparation, file-build and image jobs must retain their
original job IDs; the prepared Git tag/commit and original receipt stay unchanged.
The versioned final publication must use the original candidate digest. The
ordinary release-image plan declares no moving tag, so `latest` keeps its initial
presence and digest (including remaining absent); uncertain registry reads fail.
Only a receipt explicitly declaring `latest` may advance it to that same candidate
digest. Other moving destinations are outside this fixture's scope and refused.
`v0.0.0` remains unchanged. Normal signed-release verification still follows.
One explicit nonzero version is required; `next-version` is not accepted for it.
Local inert regressions cover wrapper/DAG ordering, fail-closed checkpoint behavior
and receipt identity; only a later explicit-input live run can prove GitLab's
cancellation/retry semantics and registry/signing outcome.

`test/live/campaigns/harness-inputs-gitlab.cases` supplies the four consumer case
lines. After preparing the normal images, add these reviewed references/absolute
paths as literal entries in `campaign.env`:

- `NATIVE_CONSUMER`, `NATIVE_JOB_ARCHIVE`, `NATIVE_BUILD_ARCHIVE`, `NATIVE_SIGNER_ARCHIVE`, `NATIVE_LINT_ARCHIVE`;
- `RECOVERY_CONSUMER`, `RECOVERY_JOB_ARCHIVE`, `RECOVERY_BUILD_ARCHIVE`, `RECOVERY_LINT_ARCHIVE`.

The archive toolchains must fit their consumer. Checkouts must be clean reviewed
Git roots and kept frozen through the lane. Scenario input records use
`checkout=@NAME` and explicit archive words; missing references, duplicate
checkout/selector settings, invalid scenario names and incomplete inputs refuse
the whole plan. These explicit-input consumers do not infer additional lint or
runtime images from the demo catalog; provide every image their gate requires.
The example case lines include `lint-archive=` for a gate that lints. Remove that
word for a consumer whose gate does not lint; a mismatch is refused at admission.

Run `build-capability-gitlab.cases` separately with
`FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=buildah` and `BUILD_PROBE_ARCHIVE` in
`campaign.env`. It requires a privileged runner and a reviewed Buildah RUN-capable
image. A Kaniko lane is refused before any case executes; changing only the
archive cannot change the selected builder.

For interruption, run the dedicated recipe separately, or use the one-case
`interruption-manual-gitlab.cases`. Observe `fci-live-interruption-ready` in the
owned GitLab job trace, then interrupt the launcher with Ctrl-C. Its expected
outcome is exit 130, an `interrupted` verdict and normal exact-resource cleanup
receipts. The campaign lane stops on interruption; this is not an automated PASS
and must not be counted as normal successful scenario coverage. If the fixture
finishes its sleep normally, it fails rather than claiming cancellation proof.

### Optional Android lane

The default k3s cases do not prepare the large Android SDK/runtime image.
To opt in, add both of these arguments to the same `campaign-prepare` command:

```text
gitlab=test/live/campaigns/k3s-private-android-gitlab.cases
job-image=kotlin:localhost/nanolinter-kotlin:latest
```

Supply a reviewed local Kotlin-flavour nanolinter image under that name first.
The additional cases file makes prepare build the linux/amd64 Android runtime
archive; missing Kotlin lint input is refused before any image build. Keep the
ordinary cases-file arguments too when preparing the combined campaign. Then
run the optional file with `campaign-lane <dir> gitlab
test/live/campaigns/k3s-private-android-gitlab.cases`, with the same private
visibility and selected builder as the ordinary lane. It runs the real Android
demo MR gate without production signing/store credentials. Debug APKs use the
normal Android debug signing; release APK/AAB outputs remain unsigned.
Budget additional SDK image/archive and Gradle-cache disk space before preparing
or starting it; the base campaign's headroom estimate does not include Android.

## Shared dependencies

`TestRunnerLending_LocalLifecycle` uses the accepted Forgejo endpoint on either
road. Its lent worker is a local rootless Podman container, so the host needs
rootless Podman and the cached pinned runner image even when Forgejo runs on k3s.
The worker shares the host network, maps the accepted lab hostname to loopback,
and verifies TLS with that endpoint's captured CA. Unaccepted, foreign-account
and non-lab targets refuse before provisioning. The job, idle and cancellation
cases keep their exact runner/container cleanup receipts on both roads.

Provider separation alone does not prove independence. Target rotation, runner
reconciliation, package coordinates, local image builds and backing services can
be shared. In particular, the Compose runner's shared network and a `docker`
service alias can collide across jobs. Serialize that resource or choose cases
that do not require it; keep other ready lanes moving.

Record intentional serialization and its dependency/capacity reason. Keep source
snapshots immutable during runs and preserve actual statuses, findings and
cleanup receipts. Result classification is implemented in Go; a package-level
PASS with only skipped scenarios is not a successful live lane.
