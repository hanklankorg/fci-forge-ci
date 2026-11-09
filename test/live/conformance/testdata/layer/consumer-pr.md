<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Consumer PR application-test enforcement

`TestForgejoLayer_ConsumerPullRequest` retains the clean case and adds
`application-test-findings` and `application-test-compile-error`, under the
existing visibility/object-format subtests. Both adverse cases commit real Go
test source, not a shell exit or a forged report. The offline fixture test runs
those sources through Go and the engine's Go JSON classifier.

The pinned `build-go` action always runs tests. Its existing administrator
renderer supplies `enforce-application-tests` from
`FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS`. Run the same scenario with `false` and
`true` in separate authorized runs; do not override the value in consumer YAML.
For a reviewed cases file containing the existing syntax
`scenario TestForgejoLayer_ConsumerPullRequest`, use the bare campaign selector
`TestForgejoLayer_ConsumerPullRequest` in both runs. Scenario selectors do not
accept administrator-axis suffixes. Set
`FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS=false` or `true` in the existing lane
environment; an optional `setting admin-application-tests=false` or `true`
declaration in that run's reviewed cases file must match the lane's input.
Declarations validate the input; they do not choose it. No new case-setting
language is needed. Direct focused runs can select
`^TestForgejoLayer_ConsumerPullRequest$/public$/sha1$/application-test-findings$`
or `application-test-compile-error`; the `clean` subtest is the positive control.

Expected outcomes:

| Source | Report-only | Enforcing |
|---|---|---|
| clean | build and downstream identity job succeed | same |
| assertion failure | recognized finding, binary verification and downstream job succeed | build fails, downstream job skipped |
| compiler error | not-waivable, build fails, downstream job skipped | build fails, downstream job skipped |

Every case retains the release-isolation job, exact first-attempt/job/runner
checks, unchanged PR and branch identities, and absence of tags, releases,
artifacts, deploy keys, secrets and new owner packages. A successful report-only
build also executes the existing version-stamped binary oracle. The native
assertions are maintained source, **not execution evidence** until separately
run with the frozen layer and its checksum-verified engine.

This slice does not test a release publication job's refusal or adverse lint,
vulnerability or reproducibility findings. GitLab/GitHub native work remains
held; Forgejo reproducible-build enforcement remains unconsumed/refused. No
provider or release-wide enforcement coverage is implied by these PR cases.
