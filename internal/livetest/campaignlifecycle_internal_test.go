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

// lifecycleRustJobImage is the campaign's nanolinter image of the rust
// flavour, the Cargo container's lifecycle job image.
const lifecycleRustJobImage = "registry.invalid/rust@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// lifecycleCampaignEnv is a campaign with the job images of the Forgejo
// lifecycle and the archives of the GitLab one.
const lifecycleCampaignEnv = "CAMPAIGN_COMMIT=" + campaignHead + "\nJOB_IMAGE_JAVA=" + snapshotJobImage + "\nJOB_IMAGE_RUST=" + lifecycleRustJobImage + "\n" +
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA=/c/java.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST=/c/rust.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE=/c/node.tar\n" +
	"FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE=/c/signer.tar\nLINT_IMAGE_ARCHIVE_JAVA=/c/lint-java.tar\nLINT_IMAGE_ARCHIVE_RUST=/c/lint-rust.tar\nLINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT=/c/lint-js.tar\n"

// lifecycleDemos are the demos a lifecycle's admission reads, as
// writeLifecycleDemo lays them out: their shipped declarations, a
// Containerfile where they have one, and the GitLab release build of those
// that ship one. The Maven service and the Cargo container have a development
// route; the npm service's declaration is one, but here it ships no GitLab
// release build; the Go CLI declares no container.
var lifecycleDemos = map[string]struct {
	declarations      []string
	containerfile, sh bool
}{
	"hello-maven-service":   {[]string{GitLabDefaultArtifactsConfig}, true, true},
	"hello-cargo-container": {[]string{GitLabDefaultArtifactsConfig, GitLabDedicatedReleaseImagesConfig}, true, true},
	"hello-npm-service":     {[]string{GitLabDefaultArtifactsConfig}, true, false},
	"hello-go-cli":          {[]string{GitLabDefaultArtifactsConfig}, false, false},
}

// writeLifecycleDemo lays out one of lifecycleDemos under the campaign's
// demos directory, with the linting gate every demo's is.
func writeLifecycleDemo(t *testing.T, dir, demo string) {
	t.Helper()

	files, known := lifecycleDemos[demo]
	if !known {
		t.Fatalf("no lifecycle demo %s", demo)
	}

	root := filepath.Join(dir, "demos", demo)
	writeFile(t, filepath.Join(root, filepath.FromSlash(GitLabGateFile)), lintingGate)

	for _, declaration := range files.declarations {
		writeFile(t, filepath.Join(root, filepath.FromSlash(declaration)), string(reporoot.ReadFile(t, "test/live/demos/"+demo+"/consumer/"+declaration)))
	}

	if files.containerfile {
		writeFile(t, filepath.Join(root, "Containerfile"), "FROM example.invalid/base\n")
	}

	if files.sh {
		writeFile(t, filepath.Join(root, filepath.FromSlash(GitLabConsumerBuildScript)), "#!/usr/bin/env bash\n")
	}
}

// newLifecycleCampaign is newCampaign with every lifecycle demo laid out.
func newLifecycleCampaign(t *testing.T, cases string) (string, string) {
	t.Helper()

	dir, casesPath := newCampaign(t, lifecycleCampaignEnv, cases)
	for demo := range lifecycleDemos {
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

// On Forgejo the lifecycle runs every demo with a service snapshot, each in
// the nanolinter image of its toolchain flavour: the Maven service,
// artifact-first, whose producer the snapshot builds in that image, and the
// Cargo container, container-first.
func TestCampaignLifecycle_RunsTheServiceDemosOnForgejo(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignForgejo, "test/live/campaigns/lifecycle-forgejo.cases")

	want := [][]string{{
		campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
		"CI_JOB_IMAGE=" + snapshotJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
	}, {
		campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-cargo-container"), TrialLifecycle,
		"CI_JOB_IMAGE=" + lifecycleRustJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
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

// On GitLab the lifecycle takes a release's archives and settings: the
// recipe takes the archives apart, as for a release. It runs every demo with
// a service snapshot and a GitLab release: the Maven service, artifact-first
// with its image, and the Cargo container, container-first.
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

func TestCampaignLifecycle_RefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()

	const gitlabArchives = " job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVA"

	for name, test := range map[string]struct {
		provider, line, want string
	}{
		"demo without a container":         {campaignForgejo, "consumer hello-go-cli lifecycle CI_JOB_IMAGE=" + snapshotJobImage, "no root Containerfile"},
		"no job image":                     {campaignForgejo, "consumer hello-maven-service lifecycle RELEASE_PLATFORMS=linux/amd64", "CI_JOB_IMAGE=<digest reference>"},
		"floating job image":               {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=registry.invalid/java:latest", "CI_JOB_IMAGE=<digest reference>"},
		"workflow":                         {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " workflow=release.yml", "workflow="},
		"interruption":                     {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " interrupt-after=Build", "interrupt-after="},
		"kaniko":                           {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " CI_IMAGE_BUILDER=kaniko CI_IMAGE_BUILDER_JOB_IMAGE=" + snapshotJobImage, "Buildah for now"},
		"gitlab demo without a container":  {campaignGitLab, "consumer hello-go-cli lifecycle" + gitlabArchives, "no root Containerfile"},
		"gitlab demo without a release":    {campaignGitLab, "consumer hello-npm-service lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT version-bump=npm", "no .gitlab/build-release.sh"},
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
	root := filepath.Join(dir, "demos", "hello-go-service")
	writeFile(t, filepath.Join(root, filepath.FromSlash(GitLabDefaultArtifactsConfig)), string(reporoot.ReadFile(t, "test/live/demos/hello-go-service/consumer/"+GitLabDefaultArtifactsConfig)))
	writeFile(t, filepath.Join(root, "Containerfile"), "FROM example.invalid/base\n")

	for _, provider := range []string{campaignForgejo, campaignGitLab} {
		_, err := PlanCampaignLane(dir, provider, cases, campaignHead, CampaignRun{})
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "not a service snapshot's declaration") || !strings.Contains(err.Error(), "extraction") {
			t.Fatalf("%s: %v, want the declaration refused for its extraction", provider, err)
		}
	}
}
