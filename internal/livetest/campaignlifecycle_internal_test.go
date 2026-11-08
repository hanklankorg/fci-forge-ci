// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

const snapshotJobImage = "registry.invalid/java@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// lifecycleCampaignEnv is a campaign with the job images of the Forgejo
// lifecycle and the archives of the GitLab one.
const lifecycleCampaignEnv = "CAMPAIGN_COMMIT=" + campaignHead + "\nJOB_IMAGE_JAVA=" + snapshotJobImage + "\n" +
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA=/c/java.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST=/c/rust.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE=/c/node.tar\n" +
	"FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE=/c/signer.tar\nLINT_IMAGE_ARCHIVE_JAVA=/c/lint-java.tar\nLINT_IMAGE_ARCHIVE_RUST=/c/lint-rust.tar\nLINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT=/c/lint-js.tar\n"

// lifecycleDemos are the demos a lifecycle's admission reads in these tests,
// laid out as the shipped demos are (writeLifecycleDemo): the Maven service
// and the Cargo container have a development route; the npm service's
// declaration is one too; the Go service's declaration extracts; the Go CLI
// declares no container.
var lifecycleDemos = []string{"hello-maven-service", "hello-cargo-container", "hello-npm-service", "hello-go-service", "hello-go-cli"}

// shippedDemoFiles are the files a demo's admission reads for their presence,
// not their content: the lifecycle's root Containerfile and pom.xml, and the
// GitLab release build.
var shippedDemoFiles = []string{"Containerfile", "pom.xml", GitLabConsumerBuildScript}

// writeShippedDemoStubs lays stubs of shippedDemoFiles out for a demo under
// the campaign's demos directory, each where test/live/demos/<demo>/consumer
// ships the file, so a test's demo has what the shipped demo has and not a
// hand-written idea of it.
func writeShippedDemoStubs(t *testing.T, dir, demo string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "demos", demo), 0o750); err != nil {
		t.Fatal(err)
	}

	for _, file := range shippedDemoFiles {
		if _, err := os.Stat(filepath.Join(reporoot.Path(t), "test/live/demos", demo, "consumer", filepath.FromSlash(file))); err == nil {
			writeFile(t, filepath.Join(dir, "demos", demo, filepath.FromSlash(file)), "stub\n")
		}
	}
}

// writeLifecycleDemo lays a demo out under the campaign's demos directory as
// the shipped demo is for a lifecycle's admission: its shipped declarations,
// the stubs of writeShippedDemoStubs, and the linting gate every demo's is.
func writeLifecycleDemo(t *testing.T, dir, demo string) {
	t.Helper()

	root := filepath.Join(dir, "demos", demo)
	writeFile(t, filepath.Join(root, filepath.FromSlash(GitLabGateFile)), lintingGate)
	writeShippedDemoStubs(t, dir, demo)

	for _, declaration := range []string{GitLabDefaultArtifactsConfig, GitLabDedicatedReleaseImagesConfig} {
		if body, err := os.ReadFile(filepath.Join(reporoot.Path(t), "test/live/demos", demo, "consumer", filepath.FromSlash(declaration))); err == nil {
			writeFile(t, filepath.Join(root, filepath.FromSlash(declaration)), string(body))
		}
	}
}

// newLifecycleCampaign is newCampaign with every lifecycle demo laid out.
func newLifecycleCampaign(t *testing.T, cases string) (string, string) {
	t.Helper()

	dir, casesPath := newCampaign(t, lifecycleCampaignEnv, cases)
	for _, demo := range lifecycleDemos {
		writeLifecycleDemo(t, dir, demo)
	}

	return dir, casesPath
}

// planCheckedInLifecycle plans the shipped cases file of a provider's
// lifecycle, with the run its declared settings describe: a public, Buildah
// run without admin enforcement.
func planCheckedInLifecycle(t *testing.T, provider, casesFile string) (string, CampaignPlan) {
	t.Helper()

	dir, cases := newLifecycleCampaign(t, string(reporoot.ReadFile(t, casesFile)))

	run, err := CampaignRunSettings(provider, func(key string) string {
		if key == consumerVisibilityEnv {
			return "public"
		}

		return ""
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCampaignLane(dir, provider, cases, campaignHead, run)
	if err != nil {
		t.Fatal(err)
	}

	return dir, plan
}

func TestCampaignLifecycle_RunsTheMavenServiceDemoOnForgejo(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignForgejo, "test/live/campaigns/lifecycle-forgejo.cases")

	want := []string{
		campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
		"CI_JOB_IMAGE=" + snapshotJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
	}
	if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, want) {
		t.Fatalf("lifecycle recipe %+v, want %v", plan.Cases, want)
	}
}

// On GitLab the lifecycle takes a release's archives and settings: the
// recipe takes the archives apart, as for a release. The lane runs the Maven
// service, artifact-first with its image, and the Cargo container,
// container-first.
func TestCampaignLifecycle_RunsTheServiceDemosOnGitLab(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignGitLab, "test/live/campaigns/lifecycle-gitlab.cases")

	want := [][]string{{
		campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
		"job-archive=/c/java.tar", "build-archive=/c/java.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-java.tar",
		"version=0.2.0", "version-bump=maven", "job-user=65532:65532",
	}, {
		campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-cargo-container"), TrialLifecycle,
		"job-archive=/c/rust.tar", "build-archive=/c/rust.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-rust.tar",
		"version=0.2.0", "version-bump=cargo", "job-user=65532:65532",
	}}

	if len(plan.Cases) != len(want) {
		t.Fatalf("lifecycle recipes %+v, want %v", plan.Cases, want)
	}

	for index, recipe := range want {
		if !slices.Equal(plan.Cases[index].Args, recipe) {
			t.Fatalf("lifecycle recipe %d %+v, want %v", index, plan.Cases[index], recipe)
		}
	}
}

const npmLifecycleLine = "consumer hello-npm-service lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT version=0.2.0 version-bump=npm job-user=65532:65532"

// The npm service's declaration is a service snapshot's and the demo ships
// its GitLab release build, so the planner admits its GitLab lifecycle. The
// lane leaves it out until the npm snapshot layout and the demo's
// Containerfile agree (docs/live-campaigns.md, Lifecycle), which is the
// engine's and the demo's to settle, not an admission rule.
func TestCampaignLifecycle_AdmitsTheNpmServiceOnGitLab(t *testing.T) {
	t.Parallel()

	dir, cases := newLifecycleCampaign(t, npmLifecycleLine+"\n")

	plan, err := PlanCampaignLane(dir, campaignGitLab, cases, campaignHead, CampaignRun{})
	if err != nil || len(plan.Cases) != 1 || plan.Cases[0].Args[2] != filepath.Join(dir, "demos/hello-npm-service") {
		t.Fatalf("npm service lifecycle: %+v %v, want one admitted case", plan, err)
	}
}

// A GitLab lifecycle ends in a release, so a demo without the GitLab release
// build is refused at plan time, as its release would be.
func TestCampaignLifecycle_RefusesAGitLabDemoWithoutARelease(t *testing.T) {
	t.Parallel()

	dir, cases := newLifecycleCampaign(t, npmLifecycleLine+"\n")
	if err := os.Remove(filepath.Join(dir, "demos/hello-npm-service", filepath.FromSlash(GitLabConsumerBuildScript))); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCampaignLane(dir, campaignGitLab, cases, campaignHead, CampaignRun{})
	if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || !strings.Contains(err.Error(), "no "+GitLabConsumerBuildScript) {
		t.Fatalf("a lifecycle without a release build: plan=%+v err=%v", plan, err)
	}
}

func TestCampaignLifecycle_RefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()

	const gitlabArchives = " job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVA"

	for name, test := range map[string]struct {
		provider, line, want string
	}{
		"demo without a container":         {campaignForgejo, "consumer hello-go-cli lifecycle CI_JOB_IMAGE=" + snapshotJobImage, "no root Containerfile"},
		"forgejo demo without a Maven app": {campaignForgejo, "consumer hello-npm-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage, "no root pom.xml"},
		"forgejo demo with extraction":     {campaignForgejo, "consumer hello-cargo-container lifecycle CI_JOB_IMAGE=" + snapshotJobImage, "extraction and OCI provenance are unsupported"},
		"an event the demo lacks":          {campaignForgejo, "consumer hello-maven-service dev-release", "pull-request, release or lifecycle"},
		"an event a demo without a route":  {campaignForgejo, "consumer hello-go-cli dev-release", "pull-request or release:"},
		"no job image":                     {campaignForgejo, "consumer hello-maven-service lifecycle RELEASE_PLATFORMS=linux/amd64", "CI_JOB_IMAGE=<digest reference>"},
		"floating job image":               {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=registry.invalid/java:latest", "CI_JOB_IMAGE=<digest reference>"},
		"workflow":                         {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " workflow=release.yml", "workflow="},
		"interruption":                     {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " interrupt-after=Build", "interrupt-after="},
		"kaniko":                           {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " CI_IMAGE_BUILDER=kaniko CI_IMAGE_BUILDER_JOB_IMAGE=" + snapshotJobImage, "Buildah for now"},
		"gitlab demo without a container":  {campaignGitLab, "consumer hello-go-cli lifecycle" + gitlabArchives, "no root Containerfile"},
		"gitlab without the lint archive":  {campaignGitLab, "consumer hello-maven-service lifecycle job-archive=/a.tar build-archive=/b.tar signer-archive=/s.tar", "nanolinter flavor's archive"},
		"gitlab without the signer":        {campaignGitLab, "consumer hello-maven-service lifecycle job-archive=/a.tar build-archive=/b.tar lint-archive=/l.tar", "signer-archive="},
		"gitlab second case":               {campaignGitLab, "consumer hello-maven-service lifecycle" + gitlabArchives + " scenario=TestGitLabRelease_ConsumerTrial", "no scenario="},
		"gitlab second release":            {campaignGitLab, "consumer hello-maven-service lifecycle" + gitlabArchives + " version=0.2.0 version-bump=maven next-version=0.3.0", "no scenario= or next-version="},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, cases := newLifecycleCampaign(t, test.line+"\n")

			plan, err := PlanCampaignLane(dir, test.provider, cases, campaignHead, CampaignRun{})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("an unrunnable lifecycle: plan=%+v err=%v, want a validation error naming %q", plan, err, test.want)
			}
		})
	}
}

// A demo's development route is read from its declaration, not its name: a
// demo whose declaration the snapshot planner refuses is refused with the
// planner's reason, on both providers.
func TestCampaignLifecycle_RefusesADeclarationTheSnapshotCannotBuild(t *testing.T) {
	t.Parallel()

	dir, cases := newLifecycleCampaign(t, "consumer hello-go-service lifecycle CI_JOB_IMAGE="+snapshotJobImage+"\n")

	for _, provider := range []string{campaignForgejo, campaignGitLab} {
		_, err := PlanCampaignLane(dir, provider, cases, campaignHead, CampaignRun{})
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "not a service snapshot's declaration") ||
			!strings.Contains(err.Error(), "build secrets, extraction and OCI provenance are unsupported") {
			t.Fatalf("%s: %v, want the declaration refused with the planner's reason", provider, err)
		}
	}
}
