// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
//
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"github.com/diggsweden/forge-ci/test/live/internal/livetest"
)

// The consumer the pull request is opened in: one Go module, small enough to
// read here, and a canary file the pull request changes.
const (
	consumerPRCanary = "consumer-pr-canary.txt"

	consumerPRGoMod = "module example.invalid/consumer-pr-canary\n\ngo 1.27.3\n"

	consumerPRMain = `package main

import "fmt"

var version = "development"

func banner() string {
	return "consumer-pr-canary " + version
}

func main() {
	fmt.Println(banner())
}
`

	consumerPRMainTest = `package main

import "testing"

func TestBanner(t *testing.T) {
	if got := banner(); got != "consumer-pr-canary "+version {
		t.Fatalf("banner() = %q", got)
	}
}
`
)

// LAYER-3: a consumer's pull-request workflow builds, tests and passes the
// isolation gate with the layer's actions, and publishes nothing.
//
// The clean case's three jobs assert for themselves: the layer's Go build produces a binary
// stamped with the version the caller gave; the checkout is the pull
// request's head, in this repository, for a pull_request event from the head
// branch into main, and holds the change the pull request makes; and
// check-l3-isolation accepts this repository's own release example, rendered
// as a consumer of the served layer.
//
// The host then asks the forge: the pull request's run is the three jobs,
// each successful on a first attempt on the runner label; the pull request is
// still open and unmerged at the same commits; no branch moved; and the run
// left no tag, release, artifact, deploy key or secret, and no package on the
// account. A pull request is unreviewed code, so "it published nothing" is
// the half of this scenario that matters most.
//
// The two adverse cases change only the PR's Go test source. A real assertion
// failure must reach the binary verification and downstream identity job under
// report-only policy, and block both under enforcing policy. A compiler error
// blocks under either policy. Layer.Render supplies the current administrator
// input from the pinned action contract, not from application source. These
// are PR non-publication checks, not proof of a release publisher being gated.
func TestForgejoLayer_ConsumerPullRequest(t *testing.T) {
	livetest.RegisterLiveScenario(t)

	for _, visibility := range livetest.ConsumerVisibilities(t) {
		t.Run(visibility.Name, func(t *testing.T) {
			format := pairedObjectFormat(visibility)
			t.Run(format, func(t *testing.T) {
				t.Run("clean", func(t *testing.T) { runLayerConsumerPR(t, visibility.Private, format, "") })

				for _, adverse := range consumerPRAdverseTests {
					t.Run(adverse.name, func(t *testing.T) { runLayerConsumerPR(t, visibility.Private, format, adverse.source) })
				}
			})
		})
	}
}

func runLayerConsumerPR(t *testing.T, private bool, objectFormat, adverseTest string) {
	t.Helper()

	const (
		workflow = "layer-consumer-pr"
		head     = "layer-pull-request"
	)

	scenario := newLayerScenario(t, "layer-consumer-pr", private, objectFormat)
	target, repo := scenario.target, scenario.repo
	enforced := livetest.AdminInputs(t, nil)["enforce-application-tests"].(bool)
	blocked := adverseTest != "" && (enforced || adverseTest == consumerPRCompileError)

	example, err := os.ReadFile(filepath.Join(reporoot.Path(t), "examples/go-cli/forgejo/release.yml"))
	if err != nil {
		t.Fatal(err)
	}

	release, err := scenario.layer.RenderReleaseExample(string(example))
	if err != nil {
		t.Fatal(err)
	}

	// One commit: neither workflow starts on a push to a branch, and the pull
	// request should show the one change it makes.
	main := livetest.CommitFiles(t, target, repo, livetest.FileCommit{
		Branch: layerMain, Message: "livetest: the consumer",
		Files: map[string]string{
			"go.mod":                         consumerPRGoMod,
			"main.go":                        consumerPRMain,
			"main_test.go":                   consumerPRMainTest,
			consumerPRCanary:                 "consumer-pr-base\n",
			".forgejo/workflows/release.yml": release,
			".forgejo/workflows/" + workflow + ".yml": scenario.render(t, layerConsumerPRFixture),
		},
	})

	changes := map[string]string{consumerPRCanary: "consumer-pr-feature\n"}
	if adverseTest != "" {
		changes["main_test.go"] = adverseTest
	}

	feature := livetest.CommitFiles(t, target, repo, livetest.FileCommit{
		Branch: layerMain, NewBranch: head, Message: "livetest: the change under review",
		Files: changes,
	})

	packages := livetest.OwnerPackages(t, target)

	opened := livetest.OpenPullRequest(t, target, repo, head, layerMain, "livetest: consumer pull request")
	requirePullRequest(t, opened, head, feature, main)

	run := livetest.WaitForLayerRun(t, target, repo, workflow, "pull_request")

	wantRun, wantBuild, wantDownstream := jobSuccess, jobSuccess, jobSuccess
	if blocked {
		wantRun, wantBuild, wantDownstream = jobFailure, jobFailure, "skipped"
	}

	jobs := scenario.requireStatus(t, run, wantRun)
	requireRunIDs(t, target, repo, run.ID)
	requireJobsOnRunner(t, jobs, run.ID, scenario.label)

	wantJobs := map[string]string{
		"Build consumer pull request":       wantBuild,
		"Test consumer pull request":        wantDownstream,
		"Verify consumer release isolation": jobSuccess,
	}
	if len(jobs) != len(wantJobs) {
		t.Fatalf("got %d jobs, want %d", len(jobs), len(wantJobs))
	}

	for _, job := range jobs {
		if want, ok := wantJobs[job.Name]; !ok || job.Status != want {
			t.Errorf("job %q is %s, want %s", job.Name, job.Status, want)
		}

		delete(wantJobs, job.Name)

		if job.Status != "skipped" && !slices.Equal(job.RunsOn, []string{scenario.label}) {
			t.Errorf("job %q ran on %v, want [%s]", job.Name, job.RunsOn, scenario.label)
		}

		if adverseTest != "" && job.Name == "Build consumer pull request" {
			log := livetest.JobLog(t, target, repo, job)

			marker := "controlled application assertion"
			if adverseTest == consumerPRCompileError {
				marker = "undefined: enforcementCompileError"
			}

			if !strings.Contains(log, marker) {
				t.Errorf("build log lacks the actual Go failure %q", marker)
			}

			if !enforced {
				receipt := "application-tests result=findings count=1 enforcement=false source=flag"
				if adverseTest == consumerPRCompileError {
					receipt = "application-tests result=not-waivable enforcement=false source=flag"
				}

				if !strings.Contains(log, receipt) {
					t.Errorf("build log lacks the engine classification %q", receipt)
				}
			}
		}
	}

	if len(wantJobs) != 0 {
		t.Errorf("missing jobs: %v", wantJobs)
	}

	requirePullRequest(t, livetest.ReadPullRequest(t, target, repo, opened.Number), head, feature, main)

	scenario.requireBranches(t, map[string]string{layerMain: main, head: feature})
	scenario.requireNone(t, "/tags", "/releases", "/keys", "/actions/secrets")
	scenario.requireSamePackages(t, packages)

	if artifacts := livetest.RunArtifacts(t, target, repo, run.ID); len(artifacts) != 0 {
		t.Errorf("the pull request's run left artifacts: %+v", artifacts)
	}
}

// requirePullRequest holds a pull request to open and unmerged, from head at
// its commit into main at its commit.
func requirePullRequest(t *testing.T, pull livetest.PullRequest, head, headCommit, mainCommit string) {
	t.Helper()

	if pull.Number <= 0 || pull.State != "open" || pull.Merged {
		t.Fatalf("the pull request is #%d, %s, merged %t; want an open one", pull.Number, pull.State, pull.Merged)
	}

	if pull.Head.Ref != head || pull.Head.SHA != headCommit || pull.Base.Ref != layerMain || pull.Base.SHA != mainCommit {
		t.Errorf("the pull request is %+v into %+v, want %s@%s into %s@%s", pull.Head, pull.Base, head, headCommit, layerMain, mainCommit)
	}
}
