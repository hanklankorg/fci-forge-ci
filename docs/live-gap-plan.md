<!--
SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
SPDX-License-Identifier: CC0-1.0
-->

# Live coverage gap plan

What the live demo tables (`test/live/internal/livetest/preparedcases.go`) and
the hosted GitHub trial do not cover yet, by ecosystem, shape and capability,
and when each gap is closed. The generated state is in
[testing-matrices.md](testing-matrices.md); this file is the plan and is
removed when its items are done and reviewed.

A gap is closed by a table row or a scenario with a native run, not by a
fixture that only renders.

## One take per shape

An example, its demo and its live case are one thing seen three ways, not
three takes to keep alike:

1. The **example** under `examples/` is what an adopter copies.
2. The **demo** under `test/live/fixtures/consumers/` is that example filled
   in: its own source, the example's CI files, kept equal by
   `TestExampleDemosAreTheirExamples`.
3. The **case** is one table row that runs the demo.

So a new shape starts as an example, never as a hand-written demo, and a gap
is not closed by a second demo of a shape that has one. `hello-go-cli`,
`hello-go-service`, `hello-gradle` and `hello-monorepo` follow this already.
Item 26 brings the others in line.

## Part 1: closed with the live refactor

These need no new demo and no product change: the fixture exists, or the case
is written and only awaits its run.

| # | Gap | Closed by |
|---|---|---|
| 1 | Gradle library on Forgejo: no gate, no release | Rows `private/hello-gradle` pull-request and release |
| 2 | Monorepo gate on Forgejo | Row `private/hello-monorepo` pull-request, with the Node job image beside the Java one |
| 3 | Gradle release on GitLab | Row `private/hello-gradle` release |
| 4 | Snapshots written but never run: npm, Go and Cargo services | One whole-table run per lab forge from a prepared directory |
| 5 | GitLab table never run natively | Same run |
| 6 | Hosted pull request, public and private | `hello-go-cli` trial in the test organization |
| 7 | Hosted no-build release | `hello-meta` release trial |
| 8 | Docs say Forgejo has no Gradle consumer | `docs/forgejo.md`, `docs/providers.md` |

## Part 2: after the refactor, with Must, Should and Nice

Ordered by what a consumer is most likely to be.

### New shapes

| # | Gap | What it needs |
|---|---|---|
| 9 | Gradle service: no Gradle demo has a runtime image | A demo; artifact-first snapshot and lifecycle rows |
| 10 | Cargo without a container: no plain CLI, no library crate | A demo each, or one workspace with both |
| 11 | Go library: a module with no binary | A demo; its release is the tag alone, as `hello-meta` |
| 12 | Maven or Gradle application: an executable jar without an image | A demo, or a variant of `hello-maven` |
| 13 | Multi-module builds: Maven reactor, npm workspace | A demo each |
| 14 | Monorepo release and lifecycle on the lab forges | Release workflows for the example, then rows |
| 15 | No-build release on Forgejo and GitLab | `hello-meta` workflows for both, then rows |
| 16 | Mobile releases, and Android on Forgejo | A disposable signing setup; a runtime image with the Android SDK |

### Flows

| # | Gap | What it needs |
|---|---|---|
| 17 | Lifecycle for libraries and CLIs: gate and release are proven apart, never linked | A lifecycle without the service phase |
| 18 | Go releases on GitLab | Release scripts for the Go examples |
| 19 | Hosted lifecycles and the unrun hosted releases | Trials in the test organization |

### Parameters and capabilities

| # | Gap | What it needs |
|---|---|---|
| 20 | Forgejo lifecycle with kaniko runs for one demo of four | Three rows |
| 21 | Private dependencies beyond npm: Maven, Gradle, Cargo, Go modules | A private-registry variant per ecosystem |
| 22 | Signing: Forgejo GPG release files; Cosign key and SSH have examples, no rows | The deferred Forgejo GPG work; one row per mode |
| 23 | Release options on the lab forges: no SBOMs, no changelog, draft, prerelease, retention | Variant rows; the first two exist on GitHub |
| 24 | Failure paths outside GitLab: interrupted release, promotion retry | Scenarios for Forgejo and GitHub |
| 25 | More than `linux/amd64` | An approved native arm64 scope |

### Coherence

| # | Gap | What it needs |
|---|---|---|
| 26 | Hand-written demos beside an example of the same shape: the Maven, npm, Cargo and Android demos against `maven-app`, `npm-app`, `cargo-app` and `android-app` | Per demo: make it the example filled in, or record what the example lacks and add that to the example |

## Not gaps

- iOS on Forgejo and GitLab: neither lab has a macOS runner.
- Private lifecycles and snapshots: a run that leaves
  `FORGE_CI_LIVE_CONSUMER_VISIBILITY` unset runs both visibilities.
