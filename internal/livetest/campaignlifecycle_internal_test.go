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

// lifecycleKanikoJobImage is the campaign's kaniko job image, labelled for
// the consumer trial's layer repository, as the lifecycle's layer is.
const lifecycleKanikoJobImage = "forgejo.compose.forgelab:8443/forgeci/forge-ci-live-kaniko-job@sha256:1111111111111111111111111111111111111111111111111111111111111111"

// lifecycleRustJobImage is the campaign's nanolinter image of the rust
// flavour, the Cargo container's lifecycle job image.
const lifecycleRustJobImage = "registry.invalid/rust@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// lifecycleNodeJobImage is the campaign's nanolinter image of the
// javascript-typescript flavour, the npm service's lifecycle job image.
const lifecycleNodeJobImage = "registry.invalid/node@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

// lifecycleGoJobImage is the campaign's nanolinter image of the go flavour,
// the Go service's lifecycle job image.
const lifecycleGoJobImage = "registry.invalid/go@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

// lifecycleCampaignEnv is a campaign with the Java, Rust, Node, Go and kaniko
// job images of the Forgejo lifecycle and the archives of the GitLab one.
const lifecycleCampaignEnv = "CAMPAIGN_COMMIT=" + campaignHead + "\nJOB_IMAGE_JAVA=" + snapshotJobImage + "\nJOB_IMAGE_RUST=" + lifecycleRustJobImage +
	"\nJOB_IMAGE_JAVASCRIPT_TYPESCRIPT=" + lifecycleNodeJobImage + "\nJOB_IMAGE_GO=" + lifecycleGoJobImage +
	"\nKANIKO_JOB_IMAGE=" + lifecycleKanikoJobImage + "\n" +
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA=/c/java.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST=/c/rust.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE=/c/node.tar\nFORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_GO=/c/go.tar\n" +
	"FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE=/c/signer.tar\nLINT_IMAGE_ARCHIVE_JAVA=/c/lint-java.tar\nLINT_IMAGE_ARCHIVE_RUST=/c/lint-rust.tar\nLINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT=/c/lint-js.tar\n" +
	"FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE=/c/buildah.tar\nFORGE_CI_LIVE_KANIKO_JOB_IMAGE_ARCHIVE=/c/kaniko.tar\n"

// lifecycleDemos are the demos a lifecycle's admission reads in these tests,
// laid out as the shipped demos are (writeLifecycleDemo): the Maven service,
// the Cargo container and the Go service have a development route, the Go
// service on Forgejo alone, since it ships no GitLab release build; the npm
// service's declaration is one too; the Cargo workspace declares two images,
// more than the lifecycle judges and what the snapshot event judges; the
// monorepo two images of two producers, without a root Containerfile or a
// Forgejo workflow; the Go CLI declares no container.
var lifecycleDemos = []string{"hello-maven-service", "hello-cargo-container", "hello-npm-service", "hello-go-service", "hello-cargo-workspace", "hello-monorepo", "hello-go-cli"}

// shippedDemoFiles are the files a demo's admission reads for their presence,
// not their content: the lifecycle's root Containerfile, the GitLab release
// build, and the Forgejo release workflow a snapshot's workflow is pinned by.
var shippedDemoFiles = []string{"Containerfile", GitLabConsumerBuildScript, forgejoReleaseWorkflow}

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
// lifecycle, with the run its declared settings describe: the visibility and
// the image builder selected as the file's header says, without admin
// enforcement, narrowed to the cases the selectors name.
func planCheckedInLifecycle(t *testing.T, provider, casesFile, visibility, builder string, selectors ...string) (string, CampaignPlan) {
	t.Helper()

	dir, cases := newLifecycleCampaign(t, string(reporoot.ReadFile(t, casesFile)))

	run, err := CampaignRunSettings(provider, func(key string) string {
		switch key {
		case consumerVisibilityEnv:
			return visibility
		case GitLabImageBuilderEnv, ForgejoImageBuilderEnv:
			return builder
		}

		return ""
	})
	if err != nil {
		t.Fatal(err)
	}

	run.Select = selectors

	plan, err := PlanCampaignLane(reporoot.Path(t), dir, provider, cases, campaignHead, run)
	if err != nil {
		t.Fatal(err)
	}

	return dir, plan
}

// On Forgejo the lifecycle runs every demo with a service snapshot, each in
// the nanolinter image of its toolchain flavour: the Maven service,
// artifact-first, whose producer the snapshot builds in that image, under
// both builders, the Cargo container and the Go service, container-first,
// and the npm service, artifact-first. The shipped file
// declares neither visibility nor builder: the visibility is the run's
// selector, both by default, and the builder each line's CI_IMAGE_BUILDER,
// Buildah or the campaign's kaniko job image, labelled for the layer
// repository the lifecycle runs from, with the Forgejo selector left unset.
func TestCampaignLifecycle_RunsTheServiceDemosOnForgejo(t *testing.T) {
	t.Parallel()

	dir, plan := planCheckedInLifecycle(t, campaignForgejo, "test/live/campaigns/lifecycle-forgejo.cases", "both", "")

	recipe := []string{campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle, "CI_JOB_IMAGE=" + snapshotJobImage}

	want := [][]string{
		append(slices.Clone(recipe), "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64"),
		append(slices.Clone(recipe), "CI_IMAGE_BUILDER=kaniko", "CI_IMAGE_BUILDER_JOB_IMAGE="+lifecycleKanikoJobImage, "RELEASE_PLATFORMS=linux/amd64"),
		{
			campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-cargo-container"), TrialLifecycle,
			"CI_JOB_IMAGE=" + lifecycleRustJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
		},
		{
			campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-npm-service"), TrialLifecycle,
			"CI_JOB_IMAGE=" + lifecycleNodeJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
		},
		{
			campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-go-service"), TrialLifecycle,
			"CI_JOB_IMAGE=" + lifecycleGoJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
		},
	}

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
// recipe takes the archives apart, as for a release. The lane runs the Maven
// and npm services, artifact-first with their images, and the Cargo
// container, container-first. The builder is the lane's selector, so the one file plans
// under either builder, and the visibility the run's.
func TestCampaignLifecycle_RunsTheServiceDemosOnGitLab(t *testing.T) {
	t.Parallel()

	for _, builder := range []string{"", ImageBuilderKaniko} {
		t.Run("builder="+builder, func(t *testing.T) {
			t.Parallel()

			dir, plan := planCheckedInLifecycle(t, campaignGitLab, "test/live/campaigns/lifecycle-gitlab.cases", "both", builder)

			want := [][]string{{
				campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
				"job-archive=/c/java.tar", "build-archive=/c/java.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-java.tar",
				"version=0.2.0", "version-bump=maven", "job-user=65532:65532",
			}, {
				campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-cargo-container"), TrialLifecycle,
				"job-archive=/c/rust.tar", "build-archive=/c/rust.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-rust.tar",
				"version=0.2.0", "version-bump=cargo", "job-user=65532:65532",
			}, {
				campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-npm-service"), TrialLifecycle,
				"job-archive=/c/node.tar", "build-archive=/c/node.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-js.tar",
				"version=0.2.0", "version-bump=npm", "job-user=65532:65532",
			}}

			if len(plan.Cases) != len(want) {
				t.Fatalf("lifecycle recipes %+v, want %v", plan.Cases, want)
			}

			for index, recipe := range want {
				if !slices.Equal(plan.Cases[index].Args, recipe) {
					t.Fatalf("lifecycle recipe %d %+v, want %v", index, plan.Cases[index], recipe)
				}
			}
		})
	}
}

// A selector reruns one matrix cell from the shipped lifecycle files, in the
// overlay's axis words, from a run of both visibilities: each of these
// selections plans exactly one case and exports the one visibility it runs.
// They are the held reruns' commands (docs/live-campaigns.md, Rerun one
// cell).
func TestCampaignLifecycle_SelectsOneCell(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		provider, file, selector, visibility string
		want                                 func(dir string) []string
	}{
		"gitlab maven service, buildah, public": {campaignGitLab, "lifecycle-gitlab.cases", "hello-maven-service/lifecycle/image-builder=buildah/visibility=public", visibilityPublic, func(dir string) []string {
			return []string{
				campaignConsumerRecipe, campaignGitLab, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
				"job-archive=/c/java.tar", "build-archive=/c/java.tar", "signer-archive=/c/signer.tar", "lint-archive=/c/lint-java.tar",
				"version=0.2.0", "version-bump=maven", "job-user=65532:65532",
			}
		}},
		"forgejo cargo container-first, buildah, public": {campaignForgejo, "lifecycle-forgejo.cases", "hello-cargo-container/lifecycle/image-builder=buildah/visibility=public", visibilityPublic, func(dir string) []string {
			return []string{
				campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-cargo-container"), TrialLifecycle,
				"CI_JOB_IMAGE=" + lifecycleRustJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
			}
		}},
		"forgejo maven service, kaniko, public": {campaignForgejo, "lifecycle-forgejo.cases", "hello-maven-service/lifecycle/image-builder=kaniko/visibility=public", visibilityPublic, func(dir string) []string {
			return []string{
				campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
				"CI_JOB_IMAGE=" + snapshotJobImage, "CI_IMAGE_BUILDER=kaniko", "CI_IMAGE_BUILDER_JOB_IMAGE=" + lifecycleKanikoJobImage, "RELEASE_PLATFORMS=linux/amd64",
			}
		}},
		"forgejo maven service, buildah, private": {campaignForgejo, "lifecycle-forgejo.cases", "hello-maven-service/lifecycle/image-builder=buildah/visibility=private", visibilityPrivate, func(dir string) []string {
			return []string{
				campaignConsumerRecipe, campaignForgejo, filepath.Join(dir, "demos/hello-maven-service"), TrialLifecycle,
				"CI_JOB_IMAGE=" + snapshotJobImage, "CI_IMAGE_BUILDER=buildah", "RELEASE_PLATFORMS=linux/amd64",
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, plan := planCheckedInLifecycle(t, test.provider, "test/live/campaigns/"+test.file, "", "", test.selector)
			if len(plan.Cases) != 1 || !slices.Equal(plan.Cases[0].Args, test.want(dir)) {
				t.Fatalf("selection %q: %+v, want the one case %v", test.selector, plan.Cases, test.want(dir))
			}

			if exported := plan.Env[len(plan.Env)-1]; exported != consumerVisibilityEnv+"="+test.visibility {
				t.Fatalf("selection %q exports %q last, want the one visibility %s", test.selector, exported, test.visibility)
			}
		})
	}
}

// A selector that names no case, an axis value outside the overlay's, or
// two visibilities is refused, so a typo never runs nothing in silence; the
// other cases are still planned before the selection.
func TestCampaignLane_RefusesASelectorThatMatchesNoCase(t *testing.T) {
	t.Parallel()

	for _, selectors := range [][]string{
		{"hello-maven-service/release"}, {"hello-mavn"}, {"TestNoSuchScenario"},
		{"hello-cargo-container/lifecycle/image-builder=kaniko"}, {"hello-maven-service/lifecycle/image-builder=docker"},
		{"hello-maven-service/lifecycle/visibility=both"}, {"hello-maven-service/lifecycle/visibility=public", "hello-cargo-container/lifecycle/visibility=private"},
		{"hello-maven-service/lifecycle/kaniko"}, {"hello-maven-service/lifecycle/visibility=public/typo"}, {"hello-maven-service//visibility=public"},
		{"TestNoSuchScenario/visibility=public"},
	} {
		dir, cases := newLifecycleCampaign(t, string(reporoot.ReadFile(t, "test/live/campaigns/lifecycle-forgejo.cases")))

		run := CampaignRun{Settings: map[string]string{"visibility": "both", "image-builder": ImageBuilderBuildah}, Select: selectors}
		if plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignForgejo, cases, campaignHead, run); !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 {
			t.Fatalf("selectors %q: %+v %v, want a refusal", selectors, plan, err)
		}
	}
}

// A selector whose case runs with another value than it names says which,
// rather than that nothing matched: here the caller's visibility and the
// GitLab lane's builder, both inherited from the run.
func TestCampaignLane_NamesTheInheritedSelectorASelectionConflictsWith(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		provider, file, visibility, builder, selector, want string
	}{
		"inherited visibility": {campaignForgejo, "lifecycle-forgejo.cases", visibilityPrivate, "", "hello-maven-service/lifecycle/image-builder=buildah/visibility=public", "runs with visibility=private"},
		"inherited builder":    {campaignGitLab, "lifecycle-gitlab.cases", "", ImageBuilderKaniko, "hello-maven-service/lifecycle/image-builder=buildah/visibility=public", "runs with image-builder=kaniko"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, cases := newLifecycleCampaign(t, string(reporoot.ReadFile(t, "test/live/campaigns/"+test.file)))

			run, err := CampaignRunSettings(test.provider, func(key string) string {
				switch key {
				case consumerVisibilityEnv:
					return test.visibility
				case GitLabImageBuilderEnv:
					return test.builder
				}

				return ""
			})
			if err != nil {
				t.Fatal(err)
			}

			run.Select = []string{test.selector}
			if _, err := PlanCampaignLane(reporoot.Path(t), dir, test.provider, cases, campaignHead, run); !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("selector %q under %s: %v, want a refusal naming %q", test.selector, name, err, test.want)
			}
		})
	}
}

// npmLifecycleLine is the shipped GitLab lifecycle line of the npm service.
const npmLifecycleLine = "consumer hello-npm-service lifecycle job-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE build-archive=@FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE signer-archive=@FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE lint-archive=@LINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT version=0.2.0 version-bump=npm job-user=65532:65532"

// A GitLab lifecycle ends in a release, so a demo without the GitLab release
// build is refused at plan time, as its release would be.
func TestCampaignLifecycle_RefusesAGitLabDemoWithoutARelease(t *testing.T) {
	t.Parallel()

	dir, cases := newLifecycleCampaign(t, npmLifecycleLine+"\n")
	if err := os.Remove(filepath.Join(dir, "demos/hello-npm-service", filepath.FromSlash(GitLabConsumerBuildScript))); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCampaignLane(reporoot.Path(t), dir, campaignGitLab, cases, campaignHead, CampaignRun{})
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
		"demo without a container":        {campaignForgejo, "consumer hello-go-cli lifecycle CI_JOB_IMAGE=" + snapshotJobImage, "no root Containerfile"},
		"an event the demo lacks":         {campaignForgejo, "consumer hello-maven-service dev-release", "pull-request, release, lifecycle or snapshot"},
		"an event a demo without a route": {campaignForgejo, "consumer hello-go-cli dev-release", "pull-request or release:"},
		"no job image":                    {campaignForgejo, "consumer hello-maven-service lifecycle RELEASE_PLATFORMS=linux/amd64", "CI_JOB_IMAGE=<digest reference>"},
		"floating job image":              {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=registry.invalid/java:latest", "CI_JOB_IMAGE=<digest reference>"},
		"workflow":                        {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " workflow=release.yml", "workflow="},
		"interruption":                    {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " interrupt-after=Build", "interrupt-after="},
		"kaniko without its job image":    {campaignForgejo, "consumer hello-maven-service lifecycle CI_JOB_IMAGE=" + snapshotJobImage + " CI_IMAGE_BUILDER=kaniko", "CI_IMAGE_BUILDER_JOB_IMAGE=<digest reference>"},
		"gitlab demo without a container": {campaignGitLab, "consumer hello-go-cli lifecycle" + gitlabArchives, "no root Containerfile"},
		"gitlab without the lint archive": {campaignGitLab, "consumer hello-maven-service lifecycle job-archive=/a.tar build-archive=/b.tar signer-archive=/s.tar", "nanolinter flavor's archive"},
		"gitlab without the signer":       {campaignGitLab, "consumer hello-maven-service lifecycle job-archive=/a.tar build-archive=/b.tar lint-archive=/l.tar", "signer-archive="},
		"gitlab second case":              {campaignGitLab, "consumer hello-maven-service lifecycle" + gitlabArchives + " scenario=TestGitLabRelease_ConsumerTrial", "no scenario="},
		"gitlab second release":           {campaignGitLab, "consumer hello-maven-service lifecycle" + gitlabArchives + " version=0.2.0 version-bump=maven next-version=0.3.0", "no scenario= or next-version="},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, cases := newLifecycleCampaign(t, test.line+"\n")

			plan, err := PlanCampaignLane(reporoot.Path(t), dir, test.provider, cases, campaignHead, CampaignRun{})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("an unrunnable lifecycle: plan=%+v err=%v, want a validation error naming %q", plan, err, test.want)
			}
		})
	}
}

// A demo's development route is read from its declaration, not its name: a
// demo whose declaration the service snapshot cannot take is refused with
// the reason, on both providers.
func TestCampaignLifecycle_RefusesADeclarationTheSnapshotCannotBuild(t *testing.T) {
	t.Parallel()

	dir, cases := newLifecycleCampaign(t, "consumer hello-cargo-workspace lifecycle CI_JOB_IMAGE="+lifecycleRustJobImage+"\n")

	for _, provider := range []string{campaignForgejo, campaignGitLab} {
		_, err := PlanCampaignLane(reporoot.Path(t), dir, provider, cases, campaignHead, CampaignRun{})
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "not a service snapshot's declaration") ||
			!strings.Contains(err.Error(), "the trial judges one container") {
			t.Fatalf("%s: %v, want the declaration refused with the trial's reason", provider, err)
		}
	}
}
