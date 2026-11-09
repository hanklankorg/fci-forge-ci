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

func TestCampaignForgejoSavedNativeInputs(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(reporoot.Path(t), "test/live/campaigns/harness-inputs-forgejo.cases"))
	if err != nil {
		t.Fatal(err)
	}

	consumer := t.TempDir()
	image := "registry.example.invalid/job@sha256:" + strings.Repeat("a", 64)
	dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nNATIVE_CONSUMER="+consumer+"\nJOB_IMAGE_GO="+image+"\n", string(body))
	run := CampaignRun{Settings: map[string]string{"visibility": "private", "image-builder": "buildah"}, Select: []string{"TestForgejoLayer_ConsumerTrial"}}

	plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignForgejo, cases, campaignHead, run)
	if err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{campaignConsumerRecipe, campaignForgejo, consumer, TrialPullRequest, "CI_JOB_IMAGE=" + image},
		{campaignConsumerRecipe, campaignForgejo, consumer, TrialRelease, "version=0.3.0"},
	}
	if len(plan.Cases) != len(want) {
		t.Fatalf("cases: %+v", plan.Cases)
	}

	for i, args := range want {
		if !slices.Equal(plan.Cases[i].Args, args) {
			t.Fatalf("case %d: got %v, want %v", i, plan.Cases[i].Args, args)
		}
	}
}

func TestCampaignForgejoExplicitConsumerRefusesWholeLane(t *testing.T) {
	t.Parallel()

	consumer := t.TempDir()
	for name, extra := range map[string]string{
		"missing checkout":     "version=0.94.0",
		"unknown reference":    "checkout=@MISSING",
		"relative checkout":    "checkout=relative",
		"absent checkout":      "checkout=" + filepath.Join(consumer, "missing"),
		"duplicate checkout":   "checkout=@CONSUMER checkout=@CONSUMER",
		"unknown setting":      "checkout=@CONSUMER build-archive=/archive",
		"scenario override":    "checkout=@CONSUMER scenario=TestAnything",
		"unsupported event":    "checkout=@CONSUMER event=lifecycle",
		"snapshot is distinct": "checkout=@CONSUMER event=snapshot",
		"duplicate event":      "checkout=@CONSUMER event=release event=pull-request",
		"invalid version":      "checkout=@CONSUMER version=not-semver",
		"builder conflict":     "checkout=@CONSUMER CI_IMAGE_BUILDER=kaniko",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nCONSUMER="+consumer+"\n",
				"scenario TestFirst\nscenario TestForgejoLayer_ConsumerTrial "+extra+"\n")
			run := CampaignRun{BuilderSelector: ImageBuilderBuildah, Select: []string{"TestFirst"}}

			plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignForgejo, cases, campaignHead, run)
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
				t.Fatalf("invalid case left executable plan: %+v, %v", plan, err)
			}
		})
	}

	for _, extra := range []string{"", " checkout=@CONSUMER"} {
		dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nCONSUMER="+consumer+"\n",
			"scenario TestForgejoLayer_ConsumerTrial"+extra+"\n")
		if plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignGitLab, cases, campaignHead, CampaignRun{}); !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
			t.Fatalf("Forgejo scenario entered GitLab: %+v, %v", plan, err)
		}
	}

	args, err := campaignScenarioRecipe(campaignForgejo, nil, strings.Fields("scenario TestForgejoLayer_ConsumerTrial"))
	if err != nil || !slices.Equal(args, []string{"test-live", "^TestForgejoLayer_ConsumerTrial$"}) {
		t.Fatalf("bare suite selector changed: %v, %v", args, err)
	}
}

func TestCampaignForgejoNativePRRequiresPreparedFlavor(t *testing.T) {
	t.Parallel()

	if err := campaignCaseTargets(reporoot.Path(t), campaignForgejo, strings.Fields("scenario TestForgejoLayer_ConsumerTrial checkout=@NATIVE_CONSUMER event=pull-request CI_JOB_IMAGE=@JOB_IMAGE_GO"), nil, map[string]bool{}); !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("missing reviewed Go flavor admitted at preparation: %v", err)
	}
}
