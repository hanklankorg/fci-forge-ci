// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// lifecycleCampaignEnv is a campaign with the Java job image of a Forgejo
// lifecycle and the archives of a GitLab one.
const lifecycleCampaignEnv = "CAMPAIGN_COMMIT=" + campaignHead + "\nJOB_IMAGE_JAVA=" + snapshotJobImage + "\n" +
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA=/c/java.tar\nFORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE=/c/signer.tar\nLINT_IMAGE_ARCHIVE_JAVA=/c/lint-java.tar\n"

// newLifecycleCampaign is newCampaign with the Maven service demo, whose
// GitLab gate lints.
func newLifecycleCampaign(t *testing.T, cases string) (string, string) {
	t.Helper()

	dir, casesPath := newCampaign(t, lifecycleCampaignEnv, cases)
	writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")
	writeFile(t, filepath.Join(dir, "demos/hello-maven-service", filepath.FromSlash(GitLabGateFile)), lintingGate)

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
// recipe takes the archives apart, as for a release.
func TestCampaignLifecycle_RunsTheMavenServiceDemoOnGitLab(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignGitLab, "test/live/campaigns/lifecycle-gitlab.cases")

	want := []string{
		campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
		"job-archive=/c/java.tar", "build-archive=/c/java.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-java.tar",
		"version=0.2.0", "version-bump=maven", "job-user=65532:65532",
	}
	if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, want) {
		t.Fatalf("lifecycle recipe %+v, want %v", plan.Cases, want)
	}
}

func TestCampaignLifecycle_RefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()

	const gitlabArchives = " job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVA"

	for name, test := range map[string]struct {
		provider, line, want string
	}{
		"demo without a development route": {campaignForgejo, "consumer hello-go-cli lifecycle CI_JOB_IMAGE=" + snapshotJobImage, "hello-maven-service demo only"},
		"no job image":                     {campaignForgejo, "consumer hello-maven-service lifecycle RELEASE_PLATFORMS=linux/amd64", "CI_JOB_IMAGE=<digest reference>"},
		"floating job image":               {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=registry.invalid/java:latest", "CI_JOB_IMAGE=<digest reference>"},
		"workflow":                         {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " workflow=release.yml", "workflow="},
		"interruption":                     {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " interrupt-after=Build", "interrupt-after="},
		"kaniko":                           {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " CI_IMAGE_BUILDER=kaniko CI_IMAGE_BUILDER_JOB_IMAGE=" + snapshotJobImage, "Buildah for now"},
		"gitlab demo without a route":      {campaignGitLab, "consumer hello-go-cli lifecycle" + gitlabArchives, "hello-maven-service demo only"},
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
