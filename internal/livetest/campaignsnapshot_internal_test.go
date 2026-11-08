// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

const snapshotJobImage = "registry.invalid/java@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestCampaignPublishedSnapshotRoutesToNativeTrial(t *testing.T) {
	t.Parallel()

	for _, image := range []string{snapshotJobImage, "@JOB_IMAGE_JAVA"} {
		t.Run(image, func(t *testing.T) {
			t.Parallel()

			dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nJOB_IMAGE_JAVA="+snapshotJobImage+"\n",
				"consumer hello-maven-service "+campaignPublishedSnapshot+" job-image="+image+"\n")
			writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")

			plan, err := PlanCampaignLane(dir, campaignForgejo, cases, campaignHead, CampaignRun{})
			if err != nil {
				t.Fatal(err)
			}

			want := []string{"test-live-forgejo-snapshot", filepath.Join(dir, "demos/hello-maven-service"), snapshotJobImage}
			if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, want) {
				t.Fatalf("wrong native snapshot route: %+v, want %v", plan, want)
			}
		})
	}
}

func TestCampaignPublishedSnapshotRefusesWholePlan(t *testing.T) {
	t.Parallel()

	for name, tail := range map[string]string{
		"missing":       "",
		"floating":      " job-image=registry.invalid/java:latest",
		"short digest":  " job-image=registry.invalid/java@sha256:aa",
		"malformed":     " job-image=not-an-image",
		"duplicate":     " job-image=" + snapshotJobImage + " job-image=" + snapshotJobImage,
		"unknown ref":   " job-image=@MISSING",
		"missing java":  " job-image=@JOB_IMAGE_JAVA",
		"other flavor":  " job-image=@JOB_IMAGE_GO",
		"wrong key":     " image=" + snapshotJobImage,
		"extra setting": " job-image=" + snapshotJobImage + " workflow=release.yml",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// A valid snapshot first must not escape a later admission failure.
			first := "consumer hello-maven-service " + campaignPublishedSnapshot + " job-image=" + snapshotJobImage + "\n"
			dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nJOB_IMAGE_GO="+snapshotJobImage+"\n",
				first+"consumer hello-maven-service "+campaignPublishedSnapshot+tail+"\n")
			writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")

			plan, err := PlanCampaignLane(dir, campaignForgejo, cases, campaignHead, CampaignRun{})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
				t.Fatalf("invalid snapshot left executable cases: plan=%+v err=%v", plan, err)
			}
		})
	}
}

func TestCampaignPublishedSnapshotSupportedRoadAndDiagnostics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		provider, project, event string
		advertise                bool
		diagnostic               string
	}{
		{campaignForgejo, "hello-maven-service", "unknown", true, "a consumer event is pull-request or release, published-unsigned-snapshot or lifecycle"},
		{campaignForgejo, "hello-maven-service", "", true, "want consumer <demo> <pull-request or release, published-unsigned-snapshot or lifecycle> [key=value ...]"},
		{campaignForgejo, "hello-go-cli", "unknown", false, "a consumer event is pull-request or release"},
		{campaignGitLab, "hello-maven-service", "unknown", false, "a consumer event is pull-request or release"},
		{campaignForgejo, "hello-go-cli", campaignPublishedSnapshot, false, "published unsigned snapshot supports only the Forgejo hello-maven-service native trial"},
		{campaignGitLab, "hello-maven-service", campaignPublishedSnapshot, false, "published unsigned snapshot supports only the Forgejo hello-maven-service native trial"},
		{"github", "hello-maven-service", campaignPublishedSnapshot, false, `the provider is forgejo or gitlab, got "github"`},
	} {
		t.Run(tc.provider+"/"+tc.project+"/"+tc.event, func(t *testing.T) {
			t.Parallel()

			line := "consumer " + tc.project + " " + tc.event
			if tc.event == campaignPublishedSnapshot {
				line += " job-image=" + snapshotJobImage
			}

			dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", "scenario TestFirst\n"+line+"\n")
			writeFile(t, filepath.Join(dir, "demos", tc.project, "README.md"), "native fixture\n")

			plan, err := PlanCampaignLane(dir, tc.provider, cases, campaignHead, CampaignRun{})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
				t.Fatalf("unsupported road/event admitted: plan=%+v err=%v", plan, err)
			}

			if !strings.HasSuffix(err.Error(), tc.diagnostic+": "+errs.ErrValidation.Error()) {
				t.Fatalf("wrong refusal boundary: got %v, want diagnostic %q", err, tc.diagnostic)
			}

			if tc.event != campaignPublishedSnapshot && strings.Contains(err.Error(), campaignPublishedSnapshot) != tc.advertise {
				t.Fatalf("diagnostic advertises the wrong supported events: %v", err)
			}
		})
	}
}

func TestCampaignPublishedSnapshotBuilderDeclarations(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		selector, declared string
		valid              bool
	}{
		{"", ImageBuilderBuildah, true},
		{ImageBuilderBuildah, ImageBuilderBuildah, true},
		{ImageBuilderKaniko, ImageBuilderKaniko, true},
		{"", ImageBuilderKaniko, false},
		{ImageBuilderKaniko, ImageBuilderBuildah, false},
		{ImageBuilderBuildah, ImageBuilderKaniko, false},
		{"invalid", ImageBuilderBuildah, false},
	} {
		t.Run(tc.selector+"/"+tc.declared, func(t *testing.T) {
			t.Parallel()

			run, err := CampaignRunSettings(campaignForgejo, func(key string) string {
				if key == ForgejoImageBuilderEnv {
					return tc.selector
				}

				return ""
			})
			if err != nil {
				if tc.valid || tc.selector != "invalid" {
					t.Fatal(err)
				}

				return
			}

			facts, err := run.caseSettings([]string{"test-live-forgejo-snapshot", "/consumer", snapshotJobImage})
			if err != nil || facts["image-builder"] != run.Settings["image-builder"] {
				t.Fatalf("planner differs from native SelectedImageBuilder: %v %v", facts, err)
			}

			dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n",
				"setting image-builder="+tc.declared+"\nconsumer hello-maven-service "+campaignPublishedSnapshot+" job-image="+snapshotJobImage+"\n")
			writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")

			plan, err := PlanCampaignLane(dir, campaignForgejo, cases, campaignHead, run)
			if tc.valid {
				if err != nil || len(plan.Cases) != 1 {
					t.Fatalf("matching builder refused: %+v %v", plan, err)
				}
			} else if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
				t.Fatalf("declaration overrode native builder: %+v %v", plan, err)
			}
		})
	}
}

func TestCampaignPublishedSnapshotResolvesNativeBuilder(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ selector, want string }{
		{"", ImageBuilderBuildah},
		{ImageBuilderBuildah, ImageBuilderBuildah},
		{ImageBuilderKaniko, ImageBuilderKaniko},
		{"invalid", ""},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			t.Parallel()

			// Deliberately stale facts catch a recipe falling through unchanged
			// instead of resolving the selector that the native driver reads.
			run := CampaignRun{BuilderSelector: tc.selector, Settings: map[string]string{
				"image-builder": "stale", "admin-application-tests": "true",
			}}

			facts, err := run.caseSettings([]string{"test-live-forgejo-snapshot", "/consumer", snapshotJobImage})
			if tc.want == "" {
				if !errors.Is(err, errs.ErrValidation) {
					t.Fatalf("invalid selector accepted: %v", err)
				}

				return
			}

			if err != nil || facts["image-builder"] != tc.want || facts["admin-application-tests"] != "true" || run.Settings["image-builder"] != "stale" {
				t.Fatalf("native builder resolution changed/ignored run facts: %v %v", facts, err)
			}
		})
	}
}

func TestCampaignPublishedSnapshotCheckedInProfile(t *testing.T) {
	t.Parallel()

	profile := string(reporoot.ReadFile(t, "test/live/campaigns/published-snapshot-forgejo.cases"))
	dir, cases := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\nJOB_IMAGE_JAVA="+snapshotJobImage+"\n", profile)
	writeFile(t, filepath.Join(dir, "demos/hello-maven-service/README.md"), "native fixture\n")

	run, err := CampaignRunSettings(campaignForgejo, func(key string) string {
		if key == consumerVisibilityEnv {
			return "public"
		}

		return ""
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCampaignLane(dir, campaignForgejo, cases, campaignHead, run)
	if err != nil || len(plan.Cases) != 1 || plan.Cases[0].Args[0] != "test-live-forgejo-snapshot" {
		t.Fatalf("checked-in snapshot profile: %+v %v", plan, err)
	}

	for _, flavors := range [][]string{{"java"}, nil} {
		_, err := CampaignRuntimeTargets(reporoot.Path(t), []string{campaignForgejo + "=" + cases}, flavors)
		if (err == nil) != (len(flavors) == 1) {
			t.Fatalf("profile image preparation flavors=%v: %v", flavors, err)
		}
	}
}

// Only the copied recipe runs. Its launcher is replaced by an inert capture;
// the real launcher, TestMain, providers, images and credentials are never used.
func TestCampaignPublishedSnapshotRecipeOverridesTrialSelectors(t *testing.T) {
	t.Parallel()

	just, err := exec.LookPath("just")
	if err != nil {
		t.Fatal("pinned just is required for the inert recipe contract")
	}

	root := t.TempDir()
	file := writeFile(t, filepath.Join(root, "justfile"), string(reporoot.ReadFile(t, "justfile")))
	writeFile(t, filepath.Join(root, "scripts/ci/run-live-tests.sh"), snapshotRecipeCapture)

	checkout := "/reviewed path/consumer;not-a-command"
	command := exec.CommandContext(t.Context(), just, "--justfile", file, "test-live-forgejo-snapshot", checkout, snapshotJobImage)

	command.Env = append(os.Environ(),
		"FORGE_CI_LIVE_CONSUMER=/stale", "FORGE_CI_LIVE_CONSUMER_EVENT=release",
		"FORGE_CI_LIVE_CONSUMER_SETTINGS=workflow=release.yml\nCI_IMAGE_BUILDER=kaniko",
		"FORGE_CI_LIVE_MAVEN_APPLICATION_IMAGE=stale", "FORGE_CI_LIVE_SERVICE_BRANCH=main",
		"FORGE_CI_LIVE_SERVICE_READINESS_URL=http://outside.invalid/", "FORGE_CI_LIVE_SERVICE_FIXTURE=/stale",
		"FORGE_CI_LIVE_DEMO=keep-me", "FORGE_CI_LIVE_SIGNING_KEY_DIR=/stale",
		"FORGE_CI_LIVE_SNAPSHOT_JOB_IMAGE=stale", "FORGE_CI_LIVE_SNAPSHOT_DEPENDENCY_TOKEN_FILE=/stale",
		"FORGE_CI_LIVE_PROVIDER=gitlab", "FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER=kaniko",
		"FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS=true", "FORGE_CI_LIVE_CONSUMER_VISIBILITY=public")

	body, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("inert snapshot recipe: %v\n%s", err, body)
	}

	want := []string{"focused", "^TestForgejoLayer_ConsumerDevelopment$", checkout, snapshotJobImage,
		"forgejo", campaignPublishedSnapshot, "", "development", "http://127.0.0.1:8080/health",
		"", "", "", "", "", "kaniko", "true", "public"}
	if got := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n"); !slices.Equal(got, want) {
		t.Fatalf("inherited selectors changed the snapshot or its intentional settings:\ngot %q\nwant %q", got, want)
	}
}

const snapshotRecipeCapture = `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" "$FORGE_CI_LIVE_CONSUMER" "$FORGE_CI_LIVE_MAVEN_APPLICATION_IMAGE" \
  "$FORGE_CI_LIVE_PROVIDER" "$FORGE_CI_LIVE_CONSUMER_EVENT" "$FORGE_CI_LIVE_CONSUMER_SETTINGS" \
  "$FORGE_CI_LIVE_SERVICE_BRANCH" "$FORGE_CI_LIVE_SERVICE_READINESS_URL" \
  "$FORGE_CI_LIVE_SERVICE_FIXTURE" "$FORGE_CI_LIVE_DEMO" "$FORGE_CI_LIVE_SIGNING_KEY_DIR" \
  "$FORGE_CI_LIVE_SNAPSHOT_JOB_IMAGE" "$FORGE_CI_LIVE_SNAPSHOT_DEPENDENCY_TOKEN_FILE" \
  "$FORGE_CI_LIVE_FORGEJO_IMAGE_BUILDER" "$FORGE_CI_LIVE_ADMIN_APPLICATION_TESTS" \
  "$FORGE_CI_LIVE_CONSUMER_VISIBILITY"
`
