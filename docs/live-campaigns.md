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

## Preflight a run

A preflight makes every admission check a run makes and stops before the run's
first effect. It writes nothing to the lab, so it takes no live lock and may run
beside another run. It takes the same arguments as the run:

```bash
just test-live-preflight                                   # as just test-live
just test-live-preflight '^TestGitLabLayer_ConsumerTrial$' # as just test-live <scenario>
FORGE_CI_LIVE_PREFLIGHT=1 just test-live-consumer gitlab /absolute/consumer pull-request
FORGE_CI_LIVE_PREFLIGHT=1 just campaign-lane ~/campaigns/k3s gitlab test/live/campaigns/k3s-private-gitlab.cases
```

`FORGE_CI_LIVE_PREFLIGHT=1` reaches the launcher, `scripts/ci/run-live-tests.sh`,
which every recipe above ends in. The launcher reads the real lab contract the
run would use and checks it, its CA, the host tools and the build as for the
run. Each scenario then runs up to its admission boundary: everything it
checks before its first effect, such as the runner capabilities the contract
declares, the contract's credential for that forge, and the consumer checkout
and gate it was given. Runner inputs are not judged here: the engine refuses a
plan whose platform has no runner when the run plans it. A campaign lane first
plans the cases file, as for the run, and then preflights each case.

A scenario declares its boundary with `livetest.RegisterAdmittedScenario(t)` in
place of `RegisterLiveScenario`, and calls `livetest.Admitted(t)` before its
first lab write. A preflight run stops the scenario at `Admitted` and reports it
admitted; one with a subtest per visibility is admitted only when every
subtest was. A scenario that declares no boundary does not run, and the
verdict is `unadmitted`, because a preflight that skipped its admission cannot
vouch for the run. A test that reaches the lab contract without having
registered, itself or a parent, fails. `TestAdmittedScenariosEndTheirAdmission` holds every registration to a
boundary. As a backstop, a preflight run blanks the credential the scenario is
handed and refuses the lab client. Today only the GitLab consumer trial declares
a boundary; adopting `Admitted` in another scenario is what makes it
preflightable. The verdict is `admitted` (exit 0), `unadmitted`, `skipped`
(nothing ran) or `failed` (an admission check refused); `verdict.json` beside the
log lists the admitted scenarios.

The preflight judges only the credentials of the endpoints the run uses: both
forges' for an unscoped run, and one forge's under `FORGE_CI_LIVE_PROVIDER`. An
unused endpoint's credential is not judged and is not mentioned. A used one is
refused when it is stale. A credential without an expiry is stale 24 hours after
it was created. One with an expiry is stale when less than five minutes of it
remain, or when the expiry is more than 31 days away. A stale credential stops
the preflight, and the refusal names the safe step. Other sessions may hold the
shared account's contract, and emitting it again in place revokes their tokens.
Instead, mint a contract of your own beside it in Forge Lab with
`LAB_TARGETS_FILE=/absolute/path/own.json just targets --account forgeci gitlab forgejo`,
then point `LAB_TARGETS_FILE` at that path here. An unscoped run is also told it
can select the other forge with `FORGE_CI_LIVE_PROVIDER`.

## Run a campaign

### Required preparation resources (source-only admission milestone)

Existing no-budget callers retain their engine behavior. To request a mandatory
budget use `campaign-prepare ... build-budget=required [build-cpus=N]
[build-memory-mib=N] [compiler-jobs=N]`. Duplicate/unknown resource words refuse.
Direct positional recipes are `test-live-image KIND LAB [default|required]
[CPUS] [MEMORY_MIB] [JOBS]`, `buildah-job-image-archive DIR [MODE] [CPUS]
[MEMORY_MIB] [JOBS]`, and `runtime-image-archive TARGET DIR [MODE] [CPUS]
[MEMORY_MIB] [JOBS]`. Do not pass `cpus=2` as a positional value.

The required tuple defaults to 2 CPUs / 4096 MiB / jobs equal to CPUs. Empty
positional fields keep defaults; explicit campaign values cannot be empty.
Only canonical positive decimals without leading zeros/signs/whitespace are
accepted. Memory is at least 4 MiB and at most 8796093022207 MiB; CPUs at most
92233720368547 (signed 64-bit quota with period 100000); compiler jobs at most
2147483647 and not greater than CPUs. Values with `default` refuse.

**Required means native limits must be requested, not enforcement verified.**
After validation, the campaign exports the resolved tuple to its child build
owners. Publishers and archive builders apply Podman CPU period/quota, memory,
memory-plus-swap total equal to memory (no additional swap), and stage jobs=1.
Native command failure stops that operation; there is no unbounded fallback or
caller `enforced=true` input. No engine query/probe is implicit here. Rootless,
remote and VM enforcement remain unmeasured. A cache hit cannot claim a RUN was
newly executed under a budget.

Required host builds receive compiler jobs as GOMAXPROCS/explicit `-p` and memory
as GOMEMLIMIT. Canonical builds continue to exclude ambient GOFLAGS.
GOMEMLIMIT is a per-process soft GC target, not a hard aggregate memory cap.
Reserve a separate bounded host-tool slot before preflight compilation, plus
current host/non-RUN/disk headroom; native RUN flags do not contain all helpers.
Run one native build at a time. Explicit legacy default calls remain unbudgeted,
not silently qualified. Never retry without the budget.

Omitted direct recipe fields inherit the publisher's corresponding
`FORGE_CI_IMAGE_BUILD_{BUDGET,CPUS,MEMORY_MIB,COMPILER_JOBS}` environment inputs.
If both sources specify a nonempty field they must agree, otherwise Go admission
refuses a conflict. In particular an explicit `default` cannot silently clear
inherited `required`, and omission cannot clear inherited numeric requirements.
The same Go resolver applies to campaign option words and direct recipes; shell
does not merge or rewrite resource policy. To deliberately choose the legacy
unbudgeted route, remove inherited requirements explicitly before invocation.

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
just campaign-lane ~/campaigns/k3s forgejo test/live/campaigns/k3s-private-forgejo.cases &
FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true \
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
and for the GitLab lane explicitly enables reproducible builds, as its declaration
requires. An administrator setting is declared, or enforced by the run, only
where an orchestrator of the lane's forge declares its input,
`enforce-<check>`, read from this checkout's `.forgejo/workflows` and
`.forgejo/actions` or `templates/`: no Forgejo orchestrator takes
`enforce-reproducible-builds`, so the Forgejo lanes declare none. A
declaration nothing on the lane consumes is refused at preparation and before
the lane runs, and so is a run that enforces such a check, which its record
would otherwise claim. Each controller variable accepts exactly
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
The lane exports the four `FORGE_CI_LIVE_ADMIN_*` inputs it was admitted with
to every case, `false` where none was set, so each run header and verdict
records them on both forges. The embedded engine default stays report-only; no lab-only policy source
edit or checksum rebuild is needed to choose the run's settings.

A cases file then holds one case a line: `consumer <demo>
<pull-request|release|lifecycle> [key=value ...]` or `scenario <TestName>
[key=value ...]`. Explicit scenario inputs
are supported by the native consumer recipes described below. A value `@NAME` is campaign.env's
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

### Rerun one cell

A selector after the cases file reruns the matrix cell it names and nothing
more: `<demo>[/<event>][/<setting>=<value>...]`, the consumer cases of the
demo and event whose run has those axis values, in the words of the testing
overlay (`test/live/campaigns/evidence.txt`): `image-builder=buildah|kaniko`,
the case's own builder, `admin-<check>=true|false`, the lane's input, and
`visibility=public|private`, which narrows a run of both visibilities to
that one and exports it to the case. `<TestName>` is a scenario case. Every
case is still planned first, so the whole file's typos are caught, and a
selector that names no case, a word other than the event without `=`, or
selectors of two visibilities, are refused. A selected case that runs with
another value, such as a visibility or GitLab builder the run inherits from
the caller, is refused naming that value:

```bash
just campaign-lane ~/campaigns/lc gitlab test/live/campaigns/lifecycle-gitlab.cases hello-maven-service/lifecycle/image-builder=buildah/visibility=public
just campaign-lane ~/campaigns/lc forgejo test/live/campaigns/lifecycle-forgejo.cases hello-maven-service/lifecycle/image-builder=buildah/visibility=private
just campaign-lane ~/campaigns/sn gitlab test/live/campaigns/snapshot-gitlab.cases hello-monorepo/snapshot/image-builder=buildah/visibility=public
```

A hosted GitHub cell is the demo's fixture workflow for the event, run by
the same recipe with `github` and `<demo>/<event>` in place of the cases
file, from a one-commit checkout of the built-in demo, inside Forge Lab's
hosted session; the settings are `just test-live-consumer github`'s, and
the line goes to `<dir>/github.results`:

```bash
just campaign-lane ~/campaigns/hosted github hello-maven-service/lifecycle org=<id>
just campaign-lane ~/campaigns/hosted github hello-go-cli/release org=<id> fixtures=test/live/github-gpg/workflows workflow=release-gpg.yml
```

### Lifecycle

`test/live/campaigns/lifecycle-forgejo.cases` and
`test/live/campaigns/lifecycle-gitlab.cases` run the linked flow of one
consumer in one scratch repository, the `lifecycle` event, for each demo with
a development route on the provider:

```text
consumer hello-maven-service lifecycle CI_JOB_IMAGE=@JOB_IMAGE_JAVA CI_IMAGE_BUILDER=buildah RELEASE_PLATFORMS=linux/amd64
consumer hello-maven-service lifecycle CI_JOB_IMAGE=@JOB_IMAGE_JAVA CI_IMAGE_BUILDER=kaniko CI_IMAGE_BUILDER_JOB_IMAGE=@KANIKO_JOB_IMAGE RELEASE_PLATFORMS=linux/amd64
consumer hello-cargo-container lifecycle CI_JOB_IMAGE=@JOB_IMAGE_RUST CI_IMAGE_BUILDER=buildah RELEASE_PLATFORMS=linux/amd64
consumer hello-npm-service lifecycle CI_JOB_IMAGE=@JOB_IMAGE_JAVASCRIPT_TYPESCRIPT CI_IMAGE_BUILDER=buildah RELEASE_PLATFORMS=linux/amd64
consumer hello-maven-service lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVA version=0.2.0 job-user=65532:65532
consumer hello-cargo-container lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_RUST version=0.2.0 job-user=65532:65532
consumer hello-npm-service lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT version=0.2.0 job-user=65532:65532
```

Neither file declares a visibility, a builder or an administrator setting.
The visibility is the run's selector: `FORGE_CI_LIVE_CONSUMER_VISIBILITY`
unset or `both` runs each line public and private. So are the
administrator inputs: `FORGE_CI_LIVE_ADMIN_<CHECK>=true` enforces that check
on every line, unset enforces none, and the run header records them. The
builder on GitLab is the lane's selector, `FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER`
(unset or `buildah`, or `kaniko`), under which every GitLab line runs; on Forgejo it is the consumer's
`CI_IMAGE_BUILDER` variable, so the Forgejo file carries one line per builder,
and `FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER` stays unset (set, it refuses the other
builder's line). On Forgejo both builders' image builds, and the service's
readiness, run on the contract's container/readiness runner
(`CI_RUNNER_LABEL_CONTAINER`); on GitLab kaniko builds on the ordinary runner.

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
3. **Development.** `development.yml`, the harness's service development
   workflow (`internal/livetest/testdata/service-development.yml.tmpl`), is
   dispatched on the merged `main` and snapshots `development`: the
   snapshot-images workflow over the staged declaration, then a readiness job
   that runs the published digest with Podman. The acceptance is the frozen
   plan, one amd64 image receipt, digest and tag with no alias, and the
   readiness evidence. Only the development registry credential is set for it.
4. **Release.** The deploy key and the release secrets are set now, a signed
   `release-request/v<version>` is pushed at the merged `main`, and the release
   is judged as a release trial's: one signed release commit on `main`, signed
   tag, a release whose Cosign bundles verify against the trial's key.
5. **Final.** The repository holds exactly those three runs, two tags, one
   release and the bot's deploy key; `development` still points at the change.

On Forgejo the event takes the release's settings but `workflow=`,
`next-version=` and `interrupt-after=`; `CI_JOB_IMAGE`, pinned by digest, is a
nanolinter image of the demo's toolchain flavour, the `job-image=<flavour>:`
the campaign publishes: the demo's lint and build jobs run in it, and the
snapshot builds the demo's producers in it, through the repository variable
the snapshot-images workflow reads for each producer family,
`SNAPSHOT_ARTIFACT_JOB_IMAGE_<FAMILY>` (none for a container-first service). A
plain runtime image fails the lint job on the forge, "nanolinter is not on
PATH". It selects its builder as the other events do, with the consumer's
`CI_IMAGE_BUILDER`, and runs in the consumer trial's layer repository
(`fci-consumer-trial-layer`, beside its own `fci-consumer-lifecycle`): a run
admits a kaniko job image only for the repository its source label names, and
the campaign's is labelled for that one, so one prepared image serves every
event; under kaniko, `CI_IMAGE_BUILDER_JOB_IMAGE` pinned by digest is required
before anything is created, as the run admits it by digest and label in the
development phase. It needs what the published snapshot and a Forgejo release
need: the contract's container/readiness runner, whichever builder builds the
image, `FORGE_CI_LIVE_SIGNER_IMAGE`, ssh-keygen and gpg.

On GitLab (`TestGitLabLayer_ConsumerLifecycle`) the same five phases run in
one project under one pinned policy file, whose includes serve the three
pipelines: the demo's gate for merge-request pipelines, the `snapshot-images`
component for the push pipelines of `main`, the `release` component for the
request tag. The merge request is merged through the API, told the head
commit; the push that merge makes to `main` is the development phase, a
snapshot of `development` whose child pipeline builds the producer the
declaration names in the build image, the ecosystem's toolchain image (none
for a container-first service, whose Containerfile builds it), builds the
image with the selected builder and publishes it to the project's registry;
the acceptance is the frozen plan, one amd64 receipt, digest and tag with no
alias. The component has no readiness or checked-JAR job, so none is
claimed. The release is judged as the GitLab release trial's,
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

On hosted GitHub (`TestGitHubLayer_ConsumerLifecycle`) the same five phases
run in one disposable repository of the test organization, against the engine
the checkout pushed there, through the three orchestrators the demo's GitHub
fixtures call: `pullrequest.yml` for the pull request, `release-dev.yml`, the
demo's call of the snapshot orchestrator, dispatched on the merged `main` with
`source-branch: development` for the development phase, and `release.yml`
for the release. The pull request is merged through the API, told the head
commit, and the merge is read back as on the lab forges: `main` at it and
`development` still at the change, through the API, then, in the working
copy fetched to `main`, its parents the base and the change and its tree the
change's. The development
phase snapshots the service snapshot's declaration the trial stages into
the pushed tree, `.forge-ci/service-development.yml`, overlaid unsigned and
native amd64 as on the lab forges (`livetest.ServiceSnapshotOverlay`). Its
acceptance is the development build's (a green run, its artifacts
delivered, `main`, tags and releases unchanged) and the same snapshot oracle
the lab forges use (`livetest.RequireSnapshotImages`): the frozen plan from
the run's `snapshot-images-inputs` artifact equal to the planner's, the
receipts from its `snapshot-images` artifact collected against it, and each
image read from the repository's container package on GHCR, which is
removed with the repository. The release is judged as
the GitHub release trial's. The snapshot publishes with the workflow's own
token; the release secrets are set at the release phase and not before. It
runs outside the campaign route, inside Forge Lab's hosted session:
`just test-live-consumer github <checkout> lifecycle org=<org-id>`
(docs/testing.md, Trying a consumer on GitHub). On GitHub the event is
admitted for the demos that ship all three fixtures and a service
snapshot's declaration: `hello-maven-service`, `hello-cargo-container`,
`hello-go-service` and `hello-npm-service`.

**Development routes.** On Forgejo and GitLab the event is admitted for a
demo with a development route on the provider, read from the demo's own
files, not its name; a demo without one is refused rather than run as a
shorter flow. The route is a *service snapshot*: the snapshot-images
policy, the reusable workflow on Forgejo and the component on GitLab, over
the demo's declaration. What the snapshot builds is the planner's rule
(`snapshotimage.New`): the harness plans the declaration, overlaid for the
trial as native linux/amd64 and unsigned into
`.forge-ci/service-development.yml` (`livetest.ServiceSnapshotDeclarationFile`),
and keeps no copy of the planner's rules. The trial's own scope is one
container of the root Containerfile and context with native amd64 declared,
without image name, tag suffix or declared provenance, from at most one
artifact, since that is the one image it judges. On both forges the
declaration is the one the demo's release builds its images from,
`.forge-ci/release-images.yml` when the demo has one (its `artifacts.yml` then
serves another forge's flow in a shape the snapshot refuses too), else
`.forge-ci/artifacts.yml` (`livetest.ServiceSnapshotSource`); on GitLab the
demo must also ship the GitLab release build, `.gitlab/build-release.sh`. On
Forgejo the harness writes the consumer-side development workflow, one for
every family (`internal/livetest/testdata/service-development.yml.tmpl`): the
snapshot-images workflow builds the producers in the image the repository's
`SNAPSHOT_ARTIFACT_JOB_IMAGE_<FAMILY>` variable names, the lifecycle's
`CI_JOB_IMAGE`, then the image; a readiness job runs the published digest.
The campaign refuses a case before anything is created
(`campaignAdmitLifecycle`), with the planner's reason. The generated matrix in
[Testing matrices](testing-matrices.md) lists every demo's state; the reasons
behind it:

| Demo | Forgejo | GitLab | Why |
|---|---|---|---|
| hello-maven-service | lifecycle | lifecycle | artifact-first Maven application and its runtime image |
| hello-cargo-container | lifecycle | lifecycle | container-first from `release-images.yml`, no producer |
| hello-npm-service | lifecycle | lifecycle | artifact-first: `artifacts.yml` is a service snapshot's (one npm producer), with the snapshot-npm-layout merge's layout, and the demo ships its GitLab release build |
| hello-cargo-workspace | — | — | two runtime images; the lifecycle judges one container (the snapshot event judges both, see [Snapshot](#snapshot)) |
| hello-go-service | lifecycle | — | container-first from `artifacts.yml`, no producer; its `extract.binary` is the release's, which the snapshot does not run; no GitLab release build, as no example demo ships one (the snapshot event runs its development phase on GitLab, see [Snapshot](#snapshot)) |
| hello-go-cli, hello-maven, hello-npm-package | — | — | no container, so no snapshot; hello-maven's library development route publishes a SNAPSHOT package, another shape |
| hello-gradle, hello-monorepo, hello-android, hello-ios | — | — | no Forgejo flow or no container; the example demos ship no GitLab release build (the snapshot event judges hello-monorepo's two images on GitLab, see [Snapshot](#snapshot)) |

amd64 runners only for now. Standalone
`test-live-consumer <forge> <checkout> lifecycle …` runs the same test
outside a campaign; the GitLab lifecycle reads the checkout's snapshot
source itself. The standalone pull-request and release trials, and GitHub's
snapshot trial, remain the single-phase entrypoints; the lifecycle calls
their extracted checks. On Forgejo its development phase is the one
published development snapshot trial (the standalone
`published-unsigned-snapshot` event retired into it). A GPG-signed lifecycle
has no demo yet: `hello-go-cli` has the GPG
declaration (`artifacts-gpg.yml`) but no development route, and the Forgejo
demos' release workflows take the default declaration, so selecting another
would be a workflow of the demo's, not a setting.

### Snapshot

`test/live/campaigns/snapshot-forgejo.cases` and
`test/live/campaigns/snapshot-gitlab.cases` run the `snapshot` event: the
lifecycle's development phase alone, over every image the planner plans from
the demo's declaration. It is the route of every development cell no
lifecycle line runs: the multi-image demos' until their full lifecycle
exists, and `hello-go-service`'s on GitLab, which has no GitLab release
build. It releases nothing and does not couple to a release's image
fan-out:

```text
consumer hello-cargo-workspace snapshot CI_JOB_IMAGE=@JOB_IMAGE_RUST CI_IMAGE_BUILDER=buildah
consumer hello-cargo-workspace snapshot job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST
consumer hello-monorepo snapshot job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA
consumer hello-go-service snapshot job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_GO
```

Each cell reruns with one command:

```bash
just campaign-lane <dir> forgejo test/live/campaigns/snapshot-forgejo.cases hello-cargo-workspace/snapshot/image-builder=buildah/visibility=public
just campaign-lane <dir> gitlab test/live/campaigns/snapshot-gitlab.cases hello-cargo-workspace/snapshot/image-builder=buildah/visibility=public
just campaign-lane <dir> gitlab test/live/campaigns/snapshot-gitlab.cases hello-monorepo/snapshot/image-builder=buildah/visibility=public
just campaign-lane <dir> gitlab test/live/campaigns/snapshot-gitlab.cases hello-go-service/snapshot/image-builder=buildah/visibility=public
```

The declaration is the lifecycle's, `livetest.ServiceSnapshotSource`, overlaid
as native linux/amd64 and unsigned into `.forge-ci/service-development.yml`.
The planner decides the images; a case names none. The trial's scope is every
planned image under the run's one image repository, so no container may name
an image of its own, each image must declare native amd64, and a declared
request for provenance is refused rather than dropped
(`snapshotImagesAdmitted`). The acceptance is the frozen plan, equal to the
plan the planner makes of the staged declaration for the selected source,
every build native amd64, and one receipt per planned image as the product's
own collection requires (`snapshotimage.Collect`), each digest the registry
serves under its receipt's tag, with no tag but the planned images' and
their `-amd64` architecture tags. This snapshot-only event probes no
readiness, even for a single image: the image count does not declare a
runnable service. The lifecycle event retains its service readiness check.
Snapshot-only events need neither a readiness URL nor a service fixture;
the Forgejo fixture does not read the operator's service fixture for this event.
The branches stay where the trial pushed them, and the repository
holds no tag and no release.

On Forgejo (`TestForgejoLayer_Snapshot`) the consumer's main and a
change on `development` are pushed with Actions off; the harness's
`development.yml` is the only workflow planned, and the only run: it is
dispatched on `main` and snapshots `development`. The receipts are the
snapshot-images workflow's own `snapshot-images` artifact. The settings are
the lifecycle's `CI_JOB_IMAGE` and `CI_IMAGE_BUILDER` (with
`CI_IMAGE_BUILDER_JOB_IMAGE` under kaniko), and none of the release's. The
harness pins its workflow as the demo's `.forgejo/workflows/release.yml` pins
the layer, so a demo without one has no Forgejo route: `hello-monorepo`,
whose example ships no Forgejo workflows, is refused on Forgejo, and adding
them is the example's work.

On GitLab (`TestGitLabLayer_Snapshot`) one pinned policy includes the
`snapshot-images` component alone, admitted only for the pipeline the
harness requests on `main` through the API; the pushes of `main` and
`development` match no rule. The frozen plan is the parent's `Snapshot:
plan` job's, the receipts the child's `Snapshot: collect` job's. The
component probes no readiness, so none is claimed. The event takes
`job-archive=` alone, the component's job image; each producer builds in
the runtime image of its toolchain family,
`FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_<FAMILY>`, which `campaign-prepare`
prepares from the plan (none for the container-first Cargo workspace,
`java` and `node` for the monorepo), and the images with the lane's
builder. `job-archive=` is required even without producers and is not the
builder archive: Buildah requires `FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE`,
kaniko requires `FORGE_CI_LIVE_KANIKO_JOB_IMAGE_ARCHIVE`. Campaign admission
and the standalone trial refuse a missing selected-builder archive before
creating mutable resources. The development registry credential is a protected
variable scoped to the publisher's environment, on a protected `main`. No release build,
signer or lint archive is asked for.

The generated [Testing matrices](testing-matrices.md) show these lines in the
snapshot columns, `📝 snapshot`; a run's verdict is evidence for the
`development` flow of the overlay. amd64 runners only for now. Standalone
`test-live-consumer <forge> <checkout> snapshot …` runs the same test
outside a campaign.

### Explicit-input Forgejo consumer trials

`test/live/campaigns/harness-inputs-forgejo.cases` illustrates the existing native
`TestForgejoLayer_ConsumerTrial` with `checkout=@NATIVE_CONSUMER`. Add an absolute,
clean, reviewed Git root as `NATIVE_CONSUMER` in `campaign.env` and keep it frozen.
The example PR also requires `JOB_IMAGE_GO`, prepared with
`job-image=go:<reviewed local image>`; choose an image satisfying the consumer's
actual tool contract. The case declarations require private visibility and
Buildah. These are expectations, not environment overrides.

Explicit inputs select `test-live-consumer forgejo <checkout> <event>`, defaulting
to release. `event=pull-request` selects the native PR route. Only those two
events are admitted here: development snapshots have a distinct scenario and
must not be relabelled as releases. Bare scenario selection is unchanged.
Missing references, duplicate checkout/event selectors, unknown settings and
other providers refuse the whole plan before the first case executes. All
normal source, target, acknowledgement, resource, locking and cleanup gates remain.
The consumer's own workflows and packaging run without a replacement build.

The release oracle verifies offered Cosign bundles. For authenticated
`checksums.txt` or `*_checksums.txt` it also verifies every uniquely named
published subject's SHA-256, refusing missing or substituted payloads. This is
not proof of intended inventory completeness, embedded package signatures,
local-registry image signature/attestation readback or provenance semantics.
An image snapshot cannot establish archive/package snapshot coverage.
Consumer-specific bindings, rollout plans and operational coverage maps belong
outside this repository.

#### Local release-image output readback

`test/live/campaigns/release-images-forgejo.cases` opts a native image release into
`release-images-config=<repository-relative declaration>`. The setting is
release-only, mutually exclusive with external `image=` verification, and does
not change the consumer build or declaration. It requires the accepted local
registry and the standard engine `release-images` run artifact; a missing,
duplicate, expired or empty receipt is a failure, not an image-free success.
An explicit external-registry session is refused before the release starts.

After verifying the prepared Git commit/tag, the oracle reads that commit's
regular declaration blob and resolves it through the existing engine planner.
It admits the complete collected ledger against this independently expected
output set and repository before any registry reads. The ledger is not a trust
root: every final tag must serve its exact top-level digest, and the existing
release-image verifier then authenticates that digest's Cosign signature,
CycloneDX attestation and SLSA release source, prepared commit, tag and caller
workflow. Re-attestation eligibility is never accepted as verified output.
The lab's no-public-transparency policy and existing scoped auth/CA transport
apply unchanged.

This bounded route supports declarative release images, not custom resolved/base
plans, moving-tag policies or custom artifact namespaces. It checks planned
image presence and final digest/evidence readback, not full architecture/payload
inventory, package signatures, arbitrary provenance extensions or equality with
an independently authenticated pre-signing build digest. The internal ledger
alone cannot prove that last property. Releases without this explicit readback
setting retain their existing, narrower oracle; workflow success must not be
reported as independent image verification.

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
