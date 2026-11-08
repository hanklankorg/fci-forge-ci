// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

func TestCampaignLifecycle_RunsTheMavenServiceDemoOnForgejo(t *testing.T) {
	t.Parallel()

	dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nJOB_IMAGE_JAVA="+snapshotJobImage+"\n",
		"consumer hello-maven-service lifecycle CI_JOB_IMAGE=@JOB_IMAGE_JAVA CI_IMAGE_BUILDER=buildah RELEASE_PLATFORMS=linux/amd64 version=0.2.0\n")
	writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")

	plan, err := PlanCampaignLane(dir, campaignForgejo, cases, campaignHead, CampaignRun{})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
		"CI_JOB_IMAGE=" + snapshotJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64", "version=0.2.0",
	}
	if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, want) {
		t.Fatalf("lifecycle recipe %+v, want %v", plan.Cases, want)
	}
}

func TestCampaignLifecycle_RefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		provider string
		line     string
	}{
		"demo without a development route": {campaignForgejo, "consumer hello-go-cli lifecycle CI_JOB_IMAGE=" + snapshotJobImage},
		"gitlab":                           {campaignGitLab, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage},
		"no job image":                     {campaignForgejo, "consumer hello-maven-service lifecycle RELEASE_PLATFORMS=linux/amd64"},
		"floating job image":               {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=registry.invalid/java:latest"},
		"workflow":                         {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " workflow=release.yml"},
		"interruption":                     {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " interrupt-after=Build"},
		"kaniko":                           {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " CI_IMAGE_BUILDER=kaniko CI_IMAGE_BUILDER_JOB_IMAGE=" + snapshotJobImage},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", test.line+"\n")
			writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")

			plan, err := PlanCampaignLane(dir, test.provider, cases, campaignHead, CampaignRun{})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
				t.Fatalf("an unrunnable lifecycle left executable cases: plan=%+v err=%v", plan, err)
			}
		})
	}
}
