# Live cases ledger

**Temporary.** This page exists only while the campaign cases files
(`test/live/campaigns/*.cases`) and the lane planner are being replaced by the
Go tables. It shows that nothing those files ran is lost. Delete it in the
same change that deletes the cases files.

It is written by hand; nothing generates or checks it. The tables it points at
are checked: `TestForgejoPreparedCasesMatchCasesFiles`,
`TestGitLabPreparedCasesMatchCasesFiles` and `TestPreparedLanesMatchCasesFiles`
(`test/live/internal/livetest/preparedcases_internal_test.go`) fail while a
cases file and the tables disagree.

## Where things went

- A `consumer` row is a row of `livetest.ForgejoPreparedCases` or
  `livetest.GitLabPreparedCases` (`test/live/internal/livetest/preparedcases.go`)
  and runs as the subtest `<test>/<lane>/<demo>/<event>/<builder>`, with
  `/<visibility>` below it.
- A bare `scenario` row is a name in the lane's `Scenarios` in
  `livetest.PreparedLanes` (`test/live/internal/livetest/preparedlanes.go`).
- A `scenario` row with operator inputs (`checkout=@NATIVE_CONSUMER`, an
  archive of the operator's own) is listed in `PreparedLanes` for the testing
  matrices and runs through the recipe it always ran through.
- A `setting` row is the caller's environment. Nothing refuses a lane run
  under other settings any more; the planner did.

`<dir>` is the absolute prepared directory of `just campaign-prepare`. One
recipe runs the tables: `just test-live-prepared <dir> <forgejo|gitlab>
<go test -run pattern>`. A whole lane is the pattern cut after the lane, for
example `'^TestForgejoLayer_ConsumerTrial$/^private$'`.

## Environment

A table case takes its images, its builder and its settings from the prepared
directory and its own row. The caller sets only:

| Lane | Caller sets |
| --- | --- |
| k3s-private-forgejo | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` |
| k3s-private-gitlab, k3s-private-android-gitlab | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true` |
| release-variants-forgejo | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public` |
| lifecycle-*, snapshot-* | nothing; both visibilities run |

The nine scenarios of k3s-private-gitlab are not table cases, so nothing
hands them the prepared images. `<scenario images>` in their commands stands
for these, each with the value of the same key in `<dir>/campaign.env`, except
the first, which takes the value of `FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_GO` when
campaign.env has no `FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE`:

```text
FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE=... FORGE_CI_LIVE_SIGNER_IMAGE=... FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE=... FORGE_CI_LIVE_KANIKO_JOB_IMAGE_ARCHIVE=...
```

`just campaign-lane` exported these, and the four administrator inputs as
explicit `false` or `true`, for every case. With the thin recipe an unset
administrator input is the default, false.

## (a) Every row of every cases file

101 rows in 13 files: 32 `setting`, 51 `consumer`, 18 `scenario`. None is
unserved.

| Cases file:line | Row | New owner | Command |
| --- | --- | --- | --- |
| build-capability-gitlab:6 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| build-capability-gitlab:7 | `setting image-builder=buildah` | caller's `FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=buildah` | set by the caller, see Environment below |
| build-capability-gitlab:8 | `scenario TestGitLabRelease_BuildImageCapability build-archive=@BUILD_PROBE_ARCHIVE` | `PreparedLanes` build-capability-gitlab; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=buildah just test-live-gitlab-buildah <BUILD_PROBE_ARCHIVE>` |
| harness-inputs-forgejo:4 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| harness-inputs-forgejo:5 | `setting image-builder=buildah` | caller's `FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=buildah` | set by the caller, see Environment below |
| harness-inputs-forgejo:6 | `scenario TestForgejoLayer_ConsumerTrial checkout=@NATIVE_CONSUMER event=pull-request CI_JOB_IMAGE=@JOB_IMAGE_GO` | `PreparedLanes` harness-inputs-forgejo; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=buildah just test-live-consumer forgejo <NATIVE_CONSUMER> pull-request CI_JOB_IMAGE=<JOB_IMAGE_GO>` |
| harness-inputs-forgejo:7 | `scenario TestForgejoLayer_ConsumerTrial checkout=@NATIVE_CONSUMER event=release version=0.3.0` | `PreparedLanes` harness-inputs-forgejo; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=buildah just test-live-consumer forgejo <NATIVE_CONSUMER> release version=0.3.0` |
| harness-inputs-gitlab:9 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| harness-inputs-gitlab:10 | `setting image-builder=kaniko` | caller's `FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko` | set by the caller, see Environment below |
| harness-inputs-gitlab:11 | `setting admin-lint=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_LINT` unset | set by the caller, see Environment below |
| harness-inputs-gitlab:12 | `setting admin-vulnerabilities=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_VULNERABILITIES` unset | set by the caller, see Environment below |
| harness-inputs-gitlab:13 | `setting admin-application-tests=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS` unset | set by the caller, see Environment below |
| harness-inputs-gitlab:14 | `setting admin-reproducible-builds=true` | caller's `FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true` | set by the caller, see Environment below |
| harness-inputs-gitlab:16 | `scenario TestGitLabRelease_ConsumerBranchProtection checkout=@NATIVE_CONSUMER` | `PreparedLanes` harness-inputs-gitlab; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko just test-live-gitlab-branch-protection <NATIVE_CONSUMER>` |
| harness-inputs-gitlab:17 | `scenario TestGitLabRelease_ConsumerTrial checkout=@NATIVE_CONSUMER job/build/signer/lint-archive=@NATIVE_*` | `PreparedLanes` harness-inputs-gitlab; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko just test-live-consumer gitlab <NATIVE_CONSUMER> release job-archive=<NATIVE_JOB_ARCHIVE> build-archive=<NATIVE_BUILD_ARCHIVE> signer-archive=<NATIVE_SIGNER_ARCHIVE> lint-archive=<NATIVE_LINT_ARCHIVE> scenario=TestGitLabRelease_ConsumerTrial` |
| harness-inputs-gitlab:18 | `scenario TestGitLabRelease_RejectsPreseededProvenance checkout=@NATIVE_CONSUMER job/build/signer/lint-archive=@NATIVE_*` | `PreparedLanes` harness-inputs-gitlab; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko just test-live-consumer gitlab <NATIVE_CONSUMER> release job-archive=<NATIVE_JOB_ARCHIVE> build-archive=<NATIVE_BUILD_ARCHIVE> signer-archive=<NATIVE_SIGNER_ARCHIVE> lint-archive=<NATIVE_LINT_ARCHIVE> scenario=TestGitLabRelease_RejectsPreseededProvenance` |
| harness-inputs-gitlab:19 | `scenario TestGitLabRelease_ConsumerInterruptedRecovery checkout=@RECOVERY_CONSUMER job/build/lint-archive=@RECOVERY_* signer-archive=@NATIVE_SIGNER_ARCHIVE version=0.3.0` | `PreparedLanes` harness-inputs-gitlab; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko just test-live-consumer gitlab <RECOVERY_CONSUMER> release job-archive=<RECOVERY_JOB_ARCHIVE> build-archive=<RECOVERY_BUILD_ARCHIVE> signer-archive=<NATIVE_SIGNER_ARCHIVE> lint-archive=<RECOVERY_LINT_ARCHIVE> version=0.3.0 scenario=TestGitLabRelease_ConsumerInterruptedRecovery` |
| interruption-manual-gitlab:7 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| interruption-manual-gitlab:8 | `scenario TestGitLabWorkflow_InterruptedJob` | `PreparedLanes` interruption-manual-gitlab; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-gitlab-interrupted-job` |
| k3s-private-android-gitlab:7 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| k3s-private-android-gitlab:8 | `setting image-builder=kaniko` | each case's `Builder: kaniko` (parity test asserts it); nothing to set | set by the caller, see Environment below |
| k3s-private-android-gitlab:9 | `setting admin-lint=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_LINT` unset | set by the caller, see Environment below |
| k3s-private-android-gitlab:10 | `setting admin-vulnerabilities=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_VULNERABILITIES` unset | set by the caller, see Environment below |
| k3s-private-android-gitlab:11 | `setting admin-application-tests=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS` unset | set by the caller, see Environment below |
| k3s-private-android-gitlab:12 | `setting admin-reproducible-builds=true` | caller's `FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true` | set by the caller, see Environment below |
| k3s-private-android-gitlab:14 | `consumer hello-android pull-request` | `TestGitLabLayer_ConsumerTrial/private-android/hello-android/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private-android$/^hello-android$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:12 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| k3s-private-forgejo:13 | `setting image-builder=kaniko` | each case's `Builder: kaniko` (parity test asserts it); nothing to set | set by the caller, see Environment below |
| k3s-private-forgejo:14 | `setting admin-lint=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_LINT` unset | set by the caller, see Environment below |
| k3s-private-forgejo:15 | `setting admin-vulnerabilities=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_VULNERABILITIES` unset | set by the caller, see Environment below |
| k3s-private-forgejo:16 | `setting admin-application-tests=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS` unset | set by the caller, see Environment below |
| k3s-private-forgejo:18 | `consumer hello-go-cli pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-go-cli/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-go-cli$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:19 | `consumer hello-go-cli release` | `TestForgejoLayer_ConsumerTrial/private/hello-go-cli/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-go-cli$/^release$/^kaniko$'` |
| k3s-private-forgejo:21 | `consumer hello-go-service pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-go-service/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-go-service$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:22 | `consumer hello-go-service release` | `TestForgejoLayer_ConsumerTrial/private/hello-go-service/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-go-service$/^release$/^kaniko$'` |
| k3s-private-forgejo:24 | `consumer hello-maven-service pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-maven-service/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-maven-service$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:25 | `consumer hello-maven-service release` | `TestForgejoLayer_ConsumerTrial/private/hello-maven-service/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-maven-service$/^release$/^kaniko$'` |
| k3s-private-forgejo:27 | `consumer hello-npm-service pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-npm-service/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-npm-service$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:28 | `consumer hello-npm-service release version=0.2.0` | `TestForgejoLayer_ConsumerTrial/private/hello-npm-service/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-npm-service$/^release$/^kaniko$'` |
| k3s-private-forgejo:30 | `consumer hello-maven pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-maven/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-maven$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:31 | `consumer hello-maven release` | `TestForgejoLayer_ConsumerTrial/private/hello-maven/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-maven$/^release$/^kaniko$'` |
| k3s-private-forgejo:33 | `consumer hello-cargo-container pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-cargo-container/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-cargo-container$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:34 | `consumer hello-cargo-container release version=0.2.0` | `TestForgejoLayer_ConsumerTrial/private/hello-cargo-container/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-cargo-container$/^release$/^kaniko$'` |
| k3s-private-forgejo:36 | `consumer hello-cargo-workspace pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-cargo-workspace/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-cargo-workspace$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:37 | `consumer hello-cargo-workspace release version=0.2.0` | `TestForgejoLayer_ConsumerTrial/private/hello-cargo-workspace/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-cargo-workspace$/^release$/^kaniko$'` |
| k3s-private-forgejo:39 | `consumer hello-npm-package pull-request` | `TestForgejoLayer_ConsumerTrial/private/hello-npm-package/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-npm-package$/^pull-request$/^kaniko$'` |
| k3s-private-forgejo:40 | `consumer hello-npm-package release version=0.2.0` | `TestForgejoLayer_ConsumerTrial/private/hello-npm-package/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^private$/^hello-npm-package$/^release$/^kaniko$'` |
| k3s-private-gitlab:22 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| k3s-private-gitlab:23 | `setting image-builder=kaniko` | each case's `Builder: kaniko` (parity test asserts it); nothing to set; for the scenarios the caller's `FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko` | set by the caller, see Environment below |
| k3s-private-gitlab:24 | `setting admin-lint=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_LINT` unset | set by the caller, see Environment below |
| k3s-private-gitlab:25 | `setting admin-vulnerabilities=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_VULNERABILITIES` unset | set by the caller, see Environment below |
| k3s-private-gitlab:26 | `setting admin-application-tests=false` | the default: caller leaves `FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS` unset | set by the caller, see Environment below |
| k3s-private-gitlab:27 | `setting admin-reproducible-builds=true` | caller's `FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true` | set by the caller, see Environment below |
| k3s-private-gitlab:29 | `scenario TestGitLabRelease_MinimalSignedArtifacts` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabRelease_MinimalSignedArtifacts$'` |
| k3s-private-gitlab:31 | `consumer hello-go-cli pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-go-cli/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-go-cli$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:32 | `consumer hello-maven pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-maven/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-maven$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:33 | `consumer hello-npm-service pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-npm-service/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-npm-service$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:34 | `consumer hello-cargo-container pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-cargo-container/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-cargo-container$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:35 | `consumer hello-go-service pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-go-service/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-go-service$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:36 | `consumer hello-maven-service pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-maven-service/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-maven-service$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:37 | `consumer hello-gradle pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-gradle/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-gradle$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:38 | `consumer hello-monorepo pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-monorepo/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-monorepo$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:39 | `consumer hello-cargo-workspace pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-cargo-workspace/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-cargo-workspace$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:40 | `consumer hello-npm-package pull-request` | `TestGitLabLayer_ConsumerTrial/private/hello-npm-package/pull-request/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerTrial$/^private$/^hello-npm-package$/^pull-request$/^kaniko$'` |
| k3s-private-gitlab:42 | `consumer hello-maven release version=0.2.0` | `TestGitLabRelease_ConsumerTrial/private/hello-maven/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabRelease_ConsumerTrial$/^private$/^hello-maven$/^release$/^kaniko$'` |
| k3s-private-gitlab:43 | `consumer hello-npm-service release version=0.2.0` | `TestGitLabRelease_ConsumerTrial/private/hello-npm-service/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabRelease_ConsumerTrial$/^private$/^hello-npm-service$/^release$/^kaniko$'` |
| k3s-private-gitlab:44 | `consumer hello-maven-service release version=0.2.0` | `TestGitLabRelease_ConsumerTrial/private/hello-maven-service/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabRelease_ConsumerTrial$/^private$/^hello-maven-service$/^release$/^kaniko$'` |
| k3s-private-gitlab:45 | `consumer hello-cargo-container release version=0.2.0` | `TestGitLabRelease_ConsumerTrial/private/hello-cargo-container/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabRelease_ConsumerTrial$/^private$/^hello-cargo-container$/^release$/^kaniko$'` |
| k3s-private-gitlab:46 | `consumer hello-cargo-workspace release version=0.2.0` | `TestGitLabRelease_ConsumerTrial/private/hello-cargo-workspace/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabRelease_ConsumerTrial$/^private$/^hello-cargo-workspace$/^release$/^kaniko$'` |
| k3s-private-gitlab:47 | `consumer hello-npm-package release version=0.2.0` | `TestGitLabRelease_ConsumerTrial/private/hello-npm-package/release/kaniko` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true just test-live-prepared <dir> gitlab '^TestGitLabRelease_ConsumerTrial$/^private$/^hello-npm-package$/^release$/^kaniko$'` |
| k3s-private-gitlab:49 | `scenario TestGitLabCheck_ChecksOutPrivateConsumerWithJobToken` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabCheck_ChecksOutPrivateConsumerWithJobToken$'` |
| k3s-private-gitlab:50 | `scenario TestGitLabCheck_ChecksOutTheConsumerPipelineCommit` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabCheck_ChecksOutTheConsumerPipelineCommit$'` |
| k3s-private-gitlab:51 | `scenario TestGitLabPolicy_ConsumerCannotReplaceTheSigningPipeline` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabPolicy_ConsumerCannotReplaceTheSigningPipeline$'` |
| k3s-private-gitlab:52 | `scenario TestGitLabPolicy_JobTokenCannotWriteThePublicationRegistry` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabPolicy_JobTokenCannotWriteThePublicationRegistry$'` |
| k3s-private-gitlab:53 | `scenario TestGitLabRelease_MinimalBranchProtection` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabRelease_MinimalBranchProtection$'` |
| k3s-private-gitlab:54 | `scenario TestGitLabRelease_ResourceGroupSpansTheChild` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabRelease_ResourceGroupSpansTheChild$'` |
| k3s-private-gitlab:55 | `scenario TestGitLabRelease_PromotionRetryAfterRegistryWrite` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabRelease_PromotionRetryAfterRegistryWrite$'` |
| k3s-private-gitlab:56 | `scenario TestGitLabBootstrap_PublishesAndDownloadsTheRecordedBinary` | `PreparedLanes` k3s-private-gitlab `Scenarios` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS=true FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER=kaniko <scenario images> just test-live-prepared <dir> gitlab '^TestGitLabBootstrap_PublishesAndDownloadsTheRecordedBinary$'` |
| lifecycle-forgejo:32 | `consumer hello-maven-service lifecycle (buildah)` | `TestForgejoLayer_ConsumerLifecycle/lifecycle/hello-maven-service/lifecycle/buildah` | `just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerLifecycle$/^lifecycle$/^hello-maven-service$/^lifecycle$/^buildah$'` |
| lifecycle-forgejo:33 | `consumer hello-maven-service lifecycle (kaniko)` | `TestForgejoLayer_ConsumerLifecycle/lifecycle/hello-maven-service/lifecycle/kaniko` | `just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerLifecycle$/^lifecycle$/^hello-maven-service$/^lifecycle$/^kaniko$'` |
| lifecycle-forgejo:34 | `consumer hello-cargo-container lifecycle` | `TestForgejoLayer_ConsumerLifecycle/lifecycle/hello-cargo-container/lifecycle/buildah` | `just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerLifecycle$/^lifecycle$/^hello-cargo-container$/^lifecycle$/^buildah$'` |
| lifecycle-forgejo:35 | `consumer hello-npm-service lifecycle` | `TestForgejoLayer_ConsumerLifecycle/lifecycle/hello-npm-service/lifecycle/buildah` | `just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerLifecycle$/^lifecycle$/^hello-npm-service$/^lifecycle$/^buildah$'` |
| lifecycle-forgejo:36 | `consumer hello-go-service lifecycle` | `TestForgejoLayer_ConsumerLifecycle/lifecycle/hello-go-service/lifecycle/buildah` | `just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerLifecycle$/^lifecycle$/^hello-go-service$/^lifecycle$/^buildah$'` |
| lifecycle-gitlab:34 | `consumer hello-maven-service lifecycle version=0.2.0 (builder was the caller's)` | `TestGitLabLayer_ConsumerLifecycle/lifecycle/hello-maven-service/lifecycle/buildah`; `TestGitLabLayer_ConsumerLifecycle/lifecycle/hello-maven-service/lifecycle/kaniko` | `just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerLifecycle$/^lifecycle$/^hello-maven-service$/^lifecycle$/^(buildah\|kaniko)$'` |
| lifecycle-gitlab:35 | `consumer hello-cargo-container lifecycle version=0.2.0 (builder was the caller's)` | `TestGitLabLayer_ConsumerLifecycle/lifecycle/hello-cargo-container/lifecycle/buildah`; `TestGitLabLayer_ConsumerLifecycle/lifecycle/hello-cargo-container/lifecycle/kaniko` | `just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerLifecycle$/^lifecycle$/^hello-cargo-container$/^lifecycle$/^(buildah\|kaniko)$'` |
| lifecycle-gitlab:36 | `consumer hello-npm-service lifecycle version=0.2.0 (builder was the caller's)` | `TestGitLabLayer_ConsumerLifecycle/lifecycle/hello-npm-service/lifecycle/buildah`; `TestGitLabLayer_ConsumerLifecycle/lifecycle/hello-npm-service/lifecycle/kaniko` | `just test-live-prepared <dir> gitlab '^TestGitLabLayer_ConsumerLifecycle$/^lifecycle$/^hello-npm-service$/^lifecycle$/^(buildah\|kaniko)$'` |
| release-images-forgejo:5 | `setting visibility=private` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` | set by the caller, see Environment below |
| release-images-forgejo:6 | `setting image-builder=buildah` | caller's `FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=buildah` | set by the caller, see Environment below |
| release-images-forgejo:7 | `scenario TestForgejoLayer_ConsumerTrial checkout=@NATIVE_CONSUMER event=release version=0.3.0 release-images-config=.forge-ci/artifacts.yml RELEASE_PLATFORMS=linux/amd64` | `PreparedLanes` release-images-forgejo; operator recipe | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=private FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=buildah just test-live-consumer forgejo <NATIVE_CONSUMER> release version=0.3.0 release-images-config=.forge-ci/artifacts.yml RELEASE_PLATFORMS=linux/amd64` |
| release-variants-forgejo:35 | `setting visibility=public` | caller's `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public` | set by the caller, see Environment below |
| release-variants-forgejo:36 | `setting image-builder=buildah` | each case's `Builder: buildah` (parity test asserts it); nothing to set | set by the caller, see Environment below |
| release-variants-forgejo:38 | `consumer hello-go-cli release next-version=0.0.2` | `TestForgejoLayer_ConsumerTrial/release-variants/hello-go-cli/release/buildah` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^release-variants$/^hello-go-cli$/^release$/^buildah$'` |
| release-variants-forgejo:39 | `consumer hello-cargo-container release version=0.2.0 next-version=0.2.1` | `TestForgejoLayer_ConsumerTrial/release-variants/hello-cargo-container/release/buildah` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^release-variants$/^hello-cargo-container$/^release$/^buildah$'` |
| release-variants-forgejo:40 | `consumer hello-npm-package release version=0.2.0 next-version=0.2.1` | `TestForgejoLayer_ConsumerTrial/release-variants/hello-npm-package/release/buildah` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^release-variants$/^hello-npm-package$/^release$/^buildah$'` |
| release-variants-forgejo:41 | `consumer hello-maven release next-version=0.0.2` | `TestForgejoLayer_ConsumerTrial/release-variants/hello-maven/release/buildah` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^release-variants$/^hello-maven$/^release$/^buildah$'` |
| release-variants-forgejo:43 | `consumer hello-go-service release release-images-config=.forge-ci/images.yml` | `TestForgejoLayer_ConsumerTrial/release-variants/hello-go-service/release/buildah` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^release-variants$/^hello-go-service$/^release$/^buildah$'` |
| release-variants-forgejo:44 | `consumer hello-cargo-workspace release version=0.2.0 release-images-config=.forge-ci/release-images.yml` | `TestForgejoLayer_ConsumerTrial/release-variants/hello-cargo-workspace/release/buildah` | `FORGE_CI_LIVE_CONSUMER_VISIBILITY=public just test-live-prepared <dir> forgejo '^TestForgejoLayer_ConsumerTrial$/^release-variants$/^hello-cargo-workspace$/^release$/^buildah$'` |
| snapshot-forgejo:29 | `consumer hello-cargo-workspace snapshot` | `TestForgejoLayer_Snapshot/snapshot/hello-cargo-workspace/snapshot/buildah` | `just test-live-prepared <dir> forgejo '^TestForgejoLayer_Snapshot$/^snapshot$/^hello-cargo-workspace$/^snapshot$/^buildah$'` |
| snapshot-gitlab:34 | `consumer hello-cargo-workspace snapshot (builder was the caller's)` | `TestGitLabLayer_Snapshot/snapshot/hello-cargo-workspace/snapshot/buildah`; `TestGitLabLayer_Snapshot/snapshot/hello-cargo-workspace/snapshot/kaniko` | `just test-live-prepared <dir> gitlab '^TestGitLabLayer_Snapshot$/^snapshot$/^hello-cargo-workspace$/^snapshot$/^(buildah\|kaniko)$'` |
| snapshot-gitlab:35 | `consumer hello-monorepo snapshot (builder was the caller's)` | `TestGitLabLayer_Snapshot/snapshot/hello-monorepo/snapshot/buildah`; `TestGitLabLayer_Snapshot/snapshot/hello-monorepo/snapshot/kaniko` | `just test-live-prepared <dir> gitlab '^TestGitLabLayer_Snapshot$/^snapshot$/^hello-monorepo$/^snapshot$/^(buildah\|kaniko)$'` |
| snapshot-gitlab:36 | `consumer hello-go-service snapshot (builder was the caller's)` | `TestGitLabLayer_Snapshot/snapshot/hello-go-service/snapshot/buildah`; `TestGitLabLayer_Snapshot/snapshot/hello-go-service/snapshot/kaniko` | `just test-live-prepared <dir> gitlab '^TestGitLabLayer_Snapshot$/^snapshot$/^hello-go-service$/^snapshot$/^(buildah\|kaniko)$'` |

The three lifecycle-gitlab and three snapshot-gitlab rows named no builder:
the lane ran them with the caller's `FORGE_CI_LIVE_GITLAB_IMAGE_BUILDER`, and
the testing matrices count them under both. Each is two rows of the table,
one a builder, so the 51 `consumer` rows are 57 table rows: 28 on Forgejo and
29 on GitLab.

## (b) Every test of the cases files' parser and planner

"Goes with it" means the test checks only code that is deleted with the cases
files, named in the last column. "Real constraint" means the rule still holds
on the table path, and the last column names the test or the live code that
holds it there.

### `test/live/internal/livetest/campaign*_internal_test.go`

| Test | Verdict | Planner function, or the owner on the table path |
| --- | --- | --- |
| TestPlanCampaignLane_ExpandsAndMapsEveryCaseInOrder | goes with it | `PlanCampaignLane`, `readCampaignCases`, `consumerRecipe` |
| TestPlanCampaignLane_MapsAGitLabConsumerToItsRecipes | goes with it; the mapping itself is a real constraint | `PlanCampaignLane`, `campaignSettings`. Owner: `TestGitLabEnvironmentOfAMergeRequestAndARelease`, `TestGitLabEnvironmentNamesTheMissingArchive` |
| TestPlanCampaignLane_RefusesWhatItCannotRead | goes with it; campaign.env refusals are a real constraint | `readCampaignCases`, `PlanCampaignLane`. Owner of the campaign.env rows: `TestReadCampaignEnvRefusesWhatIsNotData`; of another commit: `TestCheckPreparedSource` |
| TestWriteCampaignPlan_IsNULTerminatedFields | goes with it | `WriteCampaignPlan` |
| TestCampaignRuntimeTargets_ComeFromTheLanesCasesAndTheirDemosGates | stays until `prepare` changes | `CampaignRuntimeTargets`, which `campaign.sh prepare` calls; see below |
| TestAndroidCampaignRuntimeIsExplicitlyOptedInAndRequiresItsLintFlavor | stays until `prepare` changes | `CampaignRuntimeTargets` |
| TestPlanCampaignLane_RefusesAGateImageTheCampaignLacks | real constraint | The live gate refuses at run time: `NotCoveredf` in `ociimage.go` on a job image archive that is not set. A focused run skips there, a full run fails |
| TestPlanCampaignLane_SelectsAScenario | goes with it | `selectCampaignCases`, `parseCampaignSelector`; `go test -run` selects now |
| TestCampaignForgejoSavedNativeInputs | goes with it | `campaignForgejoScenarioArgs` |
| TestCampaignForgejoExplicitConsumerRefusesWholeLane | goes with it | `campaignForgejoScenarioArgs`, `PlanCampaignLane` |
| TestCampaignForgejoNativePRRequiresPreparedFlavor | goes with it | `campaignForgejoScenarioArgs`; an operator's pull request names its job image on the `just test-live-consumer` line |
| TestCampaignDeclarationsRefuseTheWholeLaneOnMismatch | goes with it | `checkSettings`, `CampaignRunSettings`. The table path has no declared lane settings to mismatch |
| TestCampaignDeclarationGrammarIsSharedWithPreparation | goes with it | `readCampaignCases`, `CampaignRuntimeTargets` |
| TestCampaignDeclarationAdmissionIsNotTiedToIndividualSettings | goes with it | `checkSettings` |
| TestCampaignAdminDeclarationsNameConsumedInputsOnly | goes with it; that an input is bound only where an orchestrator declares it is a real constraint | `refuseUnconsumedAdmin`. Owner at run time: the administrator wrappers, `TestForgejoAdminWrapperBindsOnlyInputsDeclaredByItsPinnedEngine`, `TestComponentAdminInputsAddOnlyWhatTheComponentDeclares` (`admininputs_internal_test.go`) |
| TestCampaignConsumerPREnforcementUsesBareScenarioSelector | goes with it | `selectCampaignCases`, `checkSettings` |
| TestCampaignRunSettingsUseTheSameDefaultsAsTheScenarios | goes with it | `CampaignRunSettings`; the scenarios read their own defaults |
| TestCampaignAdmissionChecksTheEffectiveBuilderOfEveryConsumer | goes with it | `PlanCampaignLane`; a table case's builder is its own row's, set for its subtest |
| TestCampaignLifecycle_RunsTheServiceDemosOnForgejo | real constraint | `TestForgejoPreparedCasesMatchCasesFiles` (rows), `TestPreparedCasesRunWhatTheirDemoAdmits` (the demo admits a lifecycle) |
| TestCampaignLifecycle_RunsTheServiceDemosOnGitLab | real constraint | `TestGitLabPreparedCasesMatchCasesFiles` (rows, both builders), `TestPreparedCasesRunWhatTheirDemoAdmits` |
| TestCampaignLifecycle_SelectsOneCell | goes with it | `selectCampaignCases`; `go test -run` selects now |
| TestCampaignLane_RefusesASelectorThatMatchesNoCase | goes with it | `selectCampaignCases`. Note: `go test -run` passes on a pattern that matches nothing |
| TestCampaignLane_NamesTheInheritedSelectorASelectionConflictsWith | goes with it | `selectCampaignCases` |
| TestCampaignLifecycle_RefusesAGitLabDemoWithoutARelease | real constraint | `TestPreparedCasesRunWhatTheirDemoAdmits` (`requireGitLabReleaseBuild` on every GitLab lifecycle and release case) |
| TestCampaignLifecycle_RefusesWhatItCannotRun | real constraint | `TestPreparedCasesRunWhatTheirDemoAdmits` (`matrixLifecycleAdmission`) |
| TestCampaignLifecycle_RefusesADeclarationTheSnapshotCannotBuild | real constraint | `TestPreparedCasesRunWhatTheirDemoAdmits` |
| TestCampaignSnapshot_RunsTheMultiImageDemosOnForgejo | real constraint | `TestForgejoPreparedCasesMatchCasesFiles`, `TestPreparedCasesRunWhatTheirDemoAdmits` |
| TestCampaignSnapshot_RunsTheDevelopmentCellsOnGitLab | real constraint | `TestGitLabPreparedCasesMatchCasesFiles`, `TestPreparedCasesRunWhatTheirDemoAdmits` |
| TestCampaignSnapshot_SelectsOneCell | goes with it | `selectCampaignCases` |
| TestCampaignSnapshot_RefusesWhatItCannotRun | real constraint | `TestPreparedCasesRunWhatTheirDemoAdmits` (`campaignAdmitSnapshot`) |
| TestCampaignSnapshot_RefusesAGitLabProducerWithoutItsImage | real constraint | The live snapshot refuses at run time: `NotCoveredf` in `gitlabsnapshot_test.go` on a producer's archive that is not set |
| TestCampaignSnapshot_RequiresTheSelectedBuilderArchive | real constraint | The live snapshot refuses at run time: `NotCoveredf` in `gitlabsnapshot_test.go` on the builder's archive |
| TestCampaignCannotRelabelPartialTrialsAsProductLifecycle | goes with it | `PlanCampaignLane`, `campaignScenarioRecipe`; a table case's event picks its test (`PreparedCaseTest`) |
| TestCampaignScenariosUseRecipesWithExplicitBoundedInputs | goes with it | `campaignScenarioRecipe`, `campaignGitLabScenarioArgs` |
| TestCampaignScenarioInputsRefuseBeforeAnyCaseCanExecute | goes with it | `campaignGitLabScenarioArgs`; the operator recipes refuse their own missing inputs |
| TestCampaignBuildahCapabilityCannotRunUnderKanikoSelection | goes with it | `campaignScenarioRecipe`; `just test-live-gitlab-buildah` is the operator's call |
| TestCampaignReleaseCases_RequestWhatTheirDemoDeclaresOrBump | real constraint | `TestPreparedReleaseCasesRequestWhatTheirDemoDeclaresOrBump` |

### `scripts/ci/campaign_test.go`

| Test | Verdict | Planner function, or the owner on the table path |
| --- | --- | --- |
| TestImagePreparationResourceRefusalPreservesDestinations | stays | `campaign.sh prepare` and image preparation; not about cases files |
| TestImageArchiveRequiredResourcesRefuseBeforeEffects | stays | image preparation |
| TestImagePreparationInheritedRequiredOnly | stays | image preparation |
| TestCampaignLane_RunsThePlanInOrderAndRecordsEachCase | goes with it | `campaign.sh lane`, `run_case`; `<dir>/<provider>.results` goes too, the verdict of `run-live-tests.sh` remains |
| TestCampaignLane_StopsAtAnInterruptedCase | goes with it | `campaign.sh lane` |
| TestCampaignLane_RunsNothingThePreflightRefuses | goes with it | `campaign.sh lane`, `preflight campaign-plan` |
| TestCampaignPrepare_RefusesAnArgumentItCannotName | stays | `campaign.sh prepare` |
| TestDemoCheckout_IsOneCommitOfTheDemo | stays | `campaign.sh prepare` (demo checkouts) |
| TestCampaignLane_HandsTheSelectorsToThePlan | goes with it | `campaign.sh lane` |
| TestCampaignLane_RunsAGitHubCellFromTheBuiltInDemo | real capability, not from a cases file | `campaign.sh lane <dir> github <demo>/<event>` (`github_cell`). The deletion must keep this entry or move it to a recipe of its own |

47 tests: 24 go with the planner, 3 more go with it but carry a constraint
that has an owner, 12 are real constraints with an owner on the table path, 7
stay with `prepare`, and 1 is the GitHub cell.

## What `campaign.sh prepare` still reads from the cases files

`prepare` is unchanged. It passes each `cases=<provider>=<file>` to
`preflight campaign-runtime-targets` (`CampaignRuntimeTargets`), which reads
from the files:

- the runtime targets to build: every `@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_<TARGET>`
  a row names, the go image when a GitLab file has a `scenario` row, the
  further job images of each demo's gate, and the producer families of each
  snapshot demo; checked against `containers/runtime/images.json`;
- refusals: a `lint-archive=@LINT_IMAGE_ARCHIVE_<FLAVOR>` whose flavor is not
  among the `job-image=<flavor>:<image>` arguments, a lint archive on a demo
  whose gate does not lint, and an administrator setting no orchestrator of
  the forge declares.

The signer image, the kaniko and buildah job images and the demo checkouts do
not depend on the cases files.

To delete the cases files, `prepare` must take the same targets from the
tables: for each GitLab case the target of its flavor or its `Runtime`, the
go target when the lane has scenarios, and the gate and snapshot families of
its demo.
