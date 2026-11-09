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

// On Forgejo the snapshot lane runs the Cargo workspace, both of whose
// images the planner plans, through the harness's development workflow in
// the rust flavour's image and the line's builder; the monorepo has no
// Forgejo workflow to pin that workflow by, so it has no line.
func TestCampaignSnapshot_RunsTheMultiImageDemosOnForgejo(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignForgejo, "test/live/campaigns/snapshot-forgejo.cases", "both", "")

	want := []string{
		campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-cargo-workspace"), TrialSnapshot,
		"CI_JOB_IMAGE=" + lifecycleRustJobImage, "CI_IMAGE_BUILDER=buildah",
	}

	if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, want) {
		t.Fatalf("snapshot recipes %+v, want %v", plan.Cases, want)
	}
}

// On GitLab the snapshot takes the component's job image alone: the Cargo
// workspace, container-first, the monorepo, whose two producers build in
// their families' runtime images, which the campaign prepares from the plan,
// and the Go service, container-first, whose lifecycle needs a GitLab
// release build the demo does not ship.
func TestCampaignSnapshot_RunsTheDevelopmentCellsOnGitLab(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignGitLab, "test/live/campaigns/snapshot-gitlab.cases", "both", "")

	want := [][]string{
		{campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-cargo-workspace"), TrialSnapshot, "job-archive=/c/rust.tar"},
		{campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-monorepo"), TrialSnapshot, "job-archive=/c/java.tar"},
		{campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-go-service"), TrialSnapshot, "job-archive=/c/go.tar"},
	}

	if len(plan.Cases) != len(want) {
		t.Fatalf("snapshot recipes %+v, want %v", plan.Cases, want)
	}

	for index, recipe := range want {
		if !slices.Equal(plan.Cases[index].Args, recipe) {
			t.Fatalf("snapshot recipe %d %+v, want %v", index, plan.Cases[index], recipe)
		}
	}

	root := reporoot.Path(t)

	targets, err := CampaignRuntimeTargets(root, []string{campaignGitLab + "=" + filepath.Join(root, "test/live/campaigns/snapshot-gitlab.cases")}, nil)
	if err != nil || !slices.Equal(targets, []string{"go", "java", "node", "rust"}) {
		t.Fatalf("snapshot runtime targets %q, %v; want the job images' and the monorepo's producer families", targets, err)
	}
}

// Each snapshot cell reruns with one command from the shipped files.
func TestCampaignSnapshot_SelectsOneCell(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		provider, file, selector string
		want                     func(dir string) []string
	}{
		"forgejo cargo workspace": {campaignForgejo, "snapshot-forgejo.cases", "hello-cargo-workspace/snapshot/image-builder=buildah/visibility=public", func(dir string) []string {
			return []string{
				campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-cargo-workspace"), TrialSnapshot,
				"CI_JOB_IMAGE=" + lifecycleRustJobImage, "CI_IMAGE_BUILDER=buildah",
			}
		}},
		"gitlab cargo workspace": {campaignGitLab, "snapshot-gitlab.cases", "hello-cargo-workspace/snapshot/image-builder=buildah/visibility=public", func(dir string) []string {
			return []string{campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-cargo-workspace"), TrialSnapshot, "job-archive=/c/rust.tar"}
		}},
		"gitlab monorepo": {campaignGitLab, "snapshot-gitlab.cases", "hello-monorepo/snapshot/image-builder=buildah/visibility=public", func(dir string) []string {
			return []string{campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-monorepo"), TrialSnapshot, "job-archive=/c/java.tar"}
		}},
		"gitlab go service": {campaignGitLab, "snapshot-gitlab.cases", "hello-go-service/snapshot/image-builder=buildah/visibility=public", func(dir string) []string {
			return []string{campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-go-service"), TrialSnapshot, "job-archive=/c/go.tar"}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, plan := planCheckedInLifecycle(t, test.provider, "test/live/campaigns/"+test.file, "", "", test.selector)
			if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, test.want(dir)) {
				t.Fatalf("selection %q: %+v, want the one case %v", test.selector, plan.Cases, test.want(dir))
			}
		})
	}
}

// A snapshot is refused where it could not judge every planned image, or
// where the lane could not run it, before anything is created.
func TestCampaignSnapshot_RefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		provider, line, want string
	}{
		"demo without a container":        {campaignForgejo, "consumer hello-go-cli snapshot CI_JOB_IMAGE=" + snapshotJobImage, "no development route"},
		"no Forgejo workflow to pin by":   {campaignForgejo, "consumer hello-monorepo snapshot CI_JOB_IMAGE=" + snapshotJobImage, "no " + forgejoReleaseWorkflow},
		"no job image":                    {campaignForgejo, "consumer hello-cargo-workspace snapshot CI_IMAGE_BUILDER=buildah", "CI_JOB_IMAGE=<digest reference>"},
		"workflow":                        {campaignForgejo, "consumer hello-cargo-workspace snapshot CI_JOB_IMAGE=" + snapshotJobImage + " workflow=release.yml", "workflow="},
		"version":                         {campaignForgejo, "consumer hello-cargo-workspace snapshot CI_JOB_IMAGE=" + snapshotJobImage + " version=0.2.0", "version="},
		"gitlab release archives":         {campaignGitLab, "consumer hello-cargo-workspace snapshot job-archive=/a.tar build-archive=/b.tar", "job-archive= alone"},
		"gitlab lint archive":             {campaignGitLab, "consumer hello-cargo-workspace snapshot job-archive=/a.tar lint-archive=/l.tar", "job-archive= alone"},
		"gitlab setting":                  {campaignGitLab, "consumer hello-cargo-workspace snapshot job-archive=/a.tar version=0.2.0", "job-archive= alone"},
		"gitlab demo without a container": {campaignGitLab, "consumer hello-go-cli snapshot job-archive=/a.tar", "no development route"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, cases := newLifecycleCampaign(t, test.line+"\n")

			plan, err := PlanCampaignLane(reporoot.Path(t), dir, test.provider, cases, campaignHead, CampaignRun{})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("an unrunnable snapshot: plan=%+v err=%v, want a validation error naming %q", plan, err, test.want)
			}
		})
	}
}

// A GitLab snapshot's producers build in their families' runtime images, so
// a campaign without one of them refuses the case and names it.
func TestCampaignSnapshot_RefusesAGitLabProducerWithoutItsImage(t *testing.T) {
	t.Parallel()

	env := strings.Replace(lifecycleCampaignEnv, "FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE=/c/node.tar\n", "", 1)
	dir, cases := newCampaign(t, env, "consumer hello-monorepo snapshot job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA\n")
	writeLifecycleDemo(t, dir, "hello-monorepo")

	plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignGitLab, cases, campaignHead, CampaignRun{})
	if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || !strings.Contains(err.Error(), "no "+GitLabSnapshotProducerArchiveEnv("node")) {
		t.Fatalf("a snapshot without its producer's image: plan=%+v err=%v", plan, err)
	}
}

// Admission must use the effective lane builder, not the presence of some
// other builder's archive. No provider resources are needed to discover a
// missing archive, including for a container-first snapshot with no producers.
func TestCampaignSnapshot_RequiresTheSelectedBuilderArchive(t *testing.T) {
	t.Parallel()

	for _, builder := range []string{"", ImageBuilderBuildah, ImageBuilderKaniko} {
		t.Run("builder="+builder, func(t *testing.T) {
			t.Parallel()

			run, err := CampaignRunSettings(campaignGitLab, func(key string) string {
				if key == GitLabImageBuilderEnv {
					return builder
				}

				return ""
			})
			if err != nil {
				t.Fatal(err)
			}

			selected, _ := imageBuilderValue(builder)
			archiveEnv := GitLabSnapshotBuilderArchiveEnv(selected)

			for _, present := range []bool{false, true} {
				env := lifecycleCampaignEnv

				if !present {
					archive := "/c/buildah.tar"
					if selected == ImageBuilderKaniko {
						archive = "/c/kaniko.tar"
					}

					env = strings.ReplaceAll(env, archiveEnv+"="+archive+"\n", "")
				}

				dir, cases := newCampaign(t, env, "consumer hello-cargo-workspace snapshot job-archive=/c/rust.tar\n")
				writeLifecycleDemo(t, dir, "hello-cargo-workspace")

				plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignGitLab, cases, campaignHead, run)
				if present {
					if err != nil || len(plan.Cases) != 1 {
						t.Fatalf("prepared %s snapshot: plan=%+v err=%v", selected, plan, err)
					}
				} else if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || !strings.Contains(err.Error(), archiveEnv) {
					t.Fatalf("missing %s: plan=%+v err=%v", archiveEnv, plan, err)
				}
			}
		})
	}
}
