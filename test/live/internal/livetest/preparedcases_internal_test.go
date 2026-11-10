// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// preparedImages is a campaign.env with every image a Forgejo case can name.
var preparedImages = map[string]string{
	"JOB_IMAGE_GO":                    "registry.invalid/go@sha256:" + strings.Repeat("1", 64),
	"JOB_IMAGE_JAVA":                  "registry.invalid/java@sha256:" + strings.Repeat("2", 64),
	"JOB_IMAGE_JAVASCRIPT_TYPESCRIPT": "registry.invalid/js@sha256:" + strings.Repeat("3", 64),
	"JOB_IMAGE_RUST":                  "registry.invalid/rust@sha256:" + strings.Repeat("4", 64),
	"KANIKO_JOB_IMAGE":                "registry.invalid/kaniko@sha256:" + strings.Repeat("5", 64),
}

// A Forgejo case's settings are its flavor's prepared nanolinter image, its
// builder, with kaniko the prepared kaniko job image, and what the case
// names, each left out when empty. An image that was not prepared is refused
// by its key.
func TestForgejoSettingsOfACase(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		c    PreparedCase
		want []string
	}{
		"buildah snapshot": {PreparedCase{Event: TrialSnapshot, JobImage: flavorRust, Builder: ImageBuilderBuildah}, []string{
			"CI_JOB_IMAGE=" + preparedImages["JOB_IMAGE_RUST"], "CI_IMAGE_BUILDER=buildah",
		}},
		"kaniko lifecycle": {PreparedCase{Event: TrialLifecycle, JobImage: flavorJava, Builder: ImageBuilderKaniko, Platforms: nativePlatform}, []string{
			"CI_JOB_IMAGE=" + preparedImages["JOB_IMAGE_JAVA"], "CI_IMAGE_BUILDER=kaniko",
			"CI_IMAGE_BUILDER_JOB_IMAGE=" + preparedImages["KANIKO_JOB_IMAGE"], "RELEASE_PLATFORMS=linux/amd64",
		}},
		"gate with a front end": {PreparedCase{Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderBuildah, NodeJobImage: flavorJS}, []string{
			"CI_JOB_IMAGE=" + preparedImages["JOB_IMAGE_JAVA"], "CI_IMAGE_BUILDER=buildah",
			"CI_JOB_IMAGE_NODE=" + preparedImages["JOB_IMAGE_JAVASCRIPT_TYPESCRIPT"],
		}},
		"release with every setting": {PreparedCase{
			Event: TrialRelease, JobImage: flavorJS, Builder: ImageBuilderBuildah,
			Version: "0.2.0", NextVersion: "0.2.1", ReleaseImagesConfig: ".forge-ci/images.yml", Platforms: nativePlatform,
		}, []string{
			"CI_JOB_IMAGE=" + preparedImages["JOB_IMAGE_JAVASCRIPT_TYPESCRIPT"], "CI_IMAGE_BUILDER=buildah",
			"version=0.2.0", "next-version=0.2.1", "release-images-config=.forge-ci/images.yml", "RELEASE_PLATFORMS=linux/amd64",
		}},
	} {
		if got, err := test.c.Settings(preparedImages); err != nil || !slices.Equal(got, test.want) {
			t.Errorf("%s: settings %q, %v; want %q", name, got, err, test.want)
		}
	}

	kaniko := PreparedCase{Event: TrialPullRequest, JobImage: flavorGo, Builder: ImageBuilderKaniko}

	for _, missing := range []string{"JOB_IMAGE_GO", "KANIKO_JOB_IMAGE"} {
		prepared := maps.Clone(preparedImages)
		delete(prepared, missing)

		if _, err := kaniko.Settings(prepared); !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "has no "+missing+";") {
			t.Errorf("without %s: %v", missing, err)
		}
	}
}

// Every case's settings are ones its trial takes, so a case the trial would
// refuse costs no run to learn: a job image by digest, kaniko with its job
// image, and on GitLab a release's and a lifecycle's version and job user.
func TestPreparedCasesHaveSettingsTheirTrialTakes(t *testing.T) {
	t.Parallel()

	for _, c := range ForgejoPreparedCases {
		settings, err := c.Settings(preparedImages)
		if err == nil {
			_, err = ParseTrialSettings(c.Event, settings)
		}

		if err != nil {
			t.Errorf("forgejo/%s: %v", c.Name(), err)
		}
	}

	for _, c := range GitLabPreparedCases {
		var err error

		switch settings := strings.Join(c.GitLabSettings(), "\n"); c.Event {
		case TrialRelease:
			_, err = ParseGitLabReleaseSettings(settings)
		case TrialLifecycle:
			_, err = ParseGitLabLifecycleSettings(settings)
		default:
			if settings != "" {
				t.Errorf("gitlab/%s takes its images alone, and has the settings %q", c.Name(), settings)
			}
		}

		if err != nil {
			t.Errorf("gitlab/%s: %v", c.Name(), err)
		}
	}
}

// gitLabPrepared is a campaign.env with every archive a GitLab case can name.
var gitLabPrepared = map[string]string{
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_GO": "/c/go.tar", "FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA": "/c/java.tar",
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE": "/c/node.tar", "FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_RUST": "/c/rust.tar",
	"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_ANDROID": "/c/android.tar", "FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE": "/c/signer.tar",
	"FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE": "/c/buildah.tar", "LINT_IMAGE_ARCHIVE_GO": "/c/lint-go.tar",
	"LINT_IMAGE_ARCHIVE_JAVA": "/c/lint-java.tar", "LINT_IMAGE_ARCHIVE_RUST": "/c/lint-rust.tar",
	"LINT_IMAGE_ARCHIVE_JAVASCRIPT_TYPESCRIPT": "/c/lint-js.tar", "LINT_IMAGE_ARCHIVE_KOTLIN": "/c/lint-kotlin.tar",
}

// A lifecycle without its signer or lint archive is refused by name, and no
// archive is taken from anywhere but the prepared directory.
func TestGitLabEnvironmentNamesTheMissingArchive(t *testing.T) {
	t.Parallel()

	lifecycle := gitLabCaseOf(t, TrialLifecycle)

	for _, missing := range []string{"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA", "FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE", "LINT_IMAGE_ARCHIVE_JAVA"} {
		prepared := maps.Clone(gitLabPrepared)
		delete(prepared, missing)

		_, err := lifecycle.GitLabEnvironment(prepared)
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "has no "+missing+";") {
			t.Errorf("without %s: %v", missing, err)
		}
	}

	snapshot, err := gitLabCaseOf(t, TrialSnapshot).GitLabEnvironment(gitLabPrepared)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{gitLabBuildArchiveEnv, gitLabSignerArchiveEnv, gitLabLintArchiveEnv, KanikoJobImageArchiveEnv} {
		if value, cleared := snapshot[key]; !cleared || value != "" {
			t.Errorf("a snapshot's %s is %q, cleared=%t", key, value, cleared)
		}
	}
}

// A snapshot takes the runtime archive of its flavor as its job image, and
// builds in the saved job image of its own builder: without that archive the
// case is refused by name, whatever the other builder's is.
func TestGitLabEnvironmentOfASnapshot(t *testing.T) {
	t.Parallel()

	both := maps.Clone(gitLabPrepared)
	both[KanikoJobImageArchiveEnv] = "/c/kaniko.tar"

	for _, builder := range []string{ImageBuilderBuildah, ImageBuilderKaniko} {
		snapshot := PreparedCase{Demo: "hello-cargo-workspace", Event: TrialSnapshot, JobImage: flavorRust, Builder: builder}
		archive := GitLabSnapshotBuilderArchiveEnv(builder)

		got, err := snapshot.GitLabEnvironment(both)
		if err != nil || got[gitLabJobArchiveEnv] != "/c/rust.tar" || got[archive] != both[archive] {
			t.Errorf("a %s snapshot: %v, %v", builder, got, err)
		}

		prepared := maps.Clone(both)
		delete(prepared, archive)

		if _, err := snapshot.GitLabEnvironment(prepared); !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "has no "+archive+";") {
			t.Errorf("a %s snapshot without %s: %v", builder, archive, err)
		}
	}
}

// gitLabCaseOf is the first GitLab case of the event.
func gitLabCaseOf(t *testing.T, event string) PreparedCase {
	t.Helper()

	index := slices.IndexFunc(GitLabPreparedCases, func(c PreparedCase) bool { return c.Event == event })
	if index < 0 {
		t.Fatalf("no GitLab case of %s", event)
	}

	return GitLabPreparedCases[index]
}

// A merge request takes its job archive as the consumer image, which leaves
// the release's job archive empty, and its flavor's lint archive; a release
// builds in its job image and signs. The Android gate runs in the android
// runtime image and lints in the kotlin flavor's.
func TestGitLabEnvironmentOfAMergeRequestAndARelease(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		c    PreparedCase
		want map[string]string
	}{
		"merge request": {PreparedCase{Demo: "hello-maven", Event: TrialPullRequest, JobImage: flavorJava}, map[string]string{
			gitLabConsumerArchiveEnv: "/c/java.tar", gitLabLintArchiveEnv: "/c/lint-java.tar",
			gitLabJobArchiveEnv: "", gitLabBuildArchiveEnv: "", gitLabSignerArchiveEnv: "",
			// Every runtime archive, for a gate's further job images, and the
			// builders' job images.
			"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_NODE": "/c/node.tar", "FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA": "/c/java.tar",
			"FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE": "/c/buildah.tar",
		}},
		"android merge request": {PreparedCase{Demo: "hello-android", Event: TrialPullRequest, JobImage: flavorKotlin, Runtime: "android"}, map[string]string{
			gitLabConsumerArchiveEnv: "/c/android.tar", gitLabLintArchiveEnv: "/c/lint-kotlin.tar", gitLabJobArchiveEnv: "",
		}},
		"release": {PreparedCase{Demo: "hello-maven", Event: TrialRelease, JobImage: flavorJava, Version: "0.2.0"}, map[string]string{
			gitLabJobArchiveEnv: "/c/java.tar", gitLabBuildArchiveEnv: "/c/java.tar", gitLabSignerArchiveEnv: "/c/signer.tar", gitLabLintArchiveEnv: "/c/lint-java.tar",
		}},
		"lifecycle": {PreparedCase{Demo: "hello-npm-service", Event: TrialLifecycle, JobImage: flavorJS, Version: "0.2.0"}, map[string]string{
			gitLabJobArchiveEnv: "/c/node.tar", gitLabBuildArchiveEnv: "/c/node.tar", gitLabSignerArchiveEnv: "/c/signer.tar", gitLabLintArchiveEnv: "/c/lint-js.tar",
		}},
	} {
		got, err := test.c.GitLabEnvironment(gitLabPrepared)
		if err != nil {
			t.Fatal(err)
		}

		for key, want := range test.want {
			if value, set := got[key]; !set || value != want {
				t.Errorf("%s: %s is %q, set=%t; want %q", name, key, value, set, want)
			}
		}

		if _, set := got[gitLabConsumerArchiveEnv]; set && test.c.Event != TrialPullRequest {
			t.Errorf("%s takes a consumer image archive", name)
		}
	}

	release := PreparedCase{Event: TrialRelease, JobImage: flavorJava, Version: "0.2.0"}
	if got := release.GitLabSettings(); !slices.Equal(got, []string{"version=0.2.0", "job-user=65532:65532"}) {
		t.Errorf("a release's settings are %q", got)
	}

	if got := (PreparedCase{Event: TrialLifecycle, JobImage: flavorJava, Version: "0.2.0"}).GitLabSettings(); !slices.Equal(got, []string{"version=0.2.0", "job-user=65532:65532"}) {
		t.Errorf("a lifecycle's settings are %q", got)
	}

	if got := (PreparedCase{Event: TrialPullRequest, JobImage: flavorJava}).GitLabSettings(); got != nil {
		t.Errorf("a merge request's settings are %q", got)
	}

	for _, missing := range []string{"FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE_JAVA", "LINT_IMAGE_ARCHIVE_JAVA"} {
		prepared := maps.Clone(gitLabPrepared)
		delete(prepared, missing)

		_, err := PreparedCase{Event: TrialPullRequest, JobImage: flavorJava}.GitLabEnvironment(prepared)
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "has no "+missing+";") {
			t.Errorf("a merge request without %s: %v", missing, err)
		}
	}
}

// A private lane's cases cannot be run public, and on GitLab they keep the
// reproducible-build check the lane always ran with.
func TestPrivateLaneFixesItsVisibilityAndReproducibleBuilds(t *testing.T) {
	t.Parallel()

	private := PreparedCase{Lane: LanePrivate}

	if got := private.LaneEnvironment(campaignForgejo); !maps.Equal(got, map[string]string{"FORGE_CI_LIVE_CONSUMER_VISIBILITY": "private"}) {
		t.Errorf("a private Forgejo case fixes %v", got)
	}

	if got := private.LaneEnvironment(campaignGitLab); !maps.Equal(got, map[string]string{
		"FORGE_CI_LIVE_CONSUMER_VISIBILITY": "private", "FORGE_CI_LIVE_ADMIN_REPRODUCIBLE_BUILDS": "true",
	}) {
		t.Errorf("a private GitLab case fixes %v", got)
	}

	if got := (PreparedCase{Lane: LaneLifecycle}).LaneEnvironment(campaignGitLab); got != nil {
		t.Errorf("a lifecycle case fixes %v", got)
	}

	// The check is turned on only where a component takes its input: turned
	// on where none does, the run's record would claim a check that never ran.
	release, err := os.ReadFile(filepath.Join(reporoot.Path(t), "templates", "release.yml"))
	if err != nil || !strings.Contains(string(release), "enforce-reproducible-builds:") {
		t.Errorf("GitLab's release component no longer takes enforce-reproducible-builds: %v", err)
	}
}
