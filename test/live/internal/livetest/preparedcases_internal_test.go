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

// preparedImages is a campaign.env with every image a case can name.
var preparedImages = map[string]string{
	"JOB_IMAGE_GO": "lab/go@sha256:01", "JOB_IMAGE_JAVA": "lab/java@sha256:02",
	"JOB_IMAGE_JAVASCRIPT_TYPESCRIPT": "lab/js@sha256:03", "JOB_IMAGE_RUST": "lab/rust@sha256:04",
	"KANIKO_JOB_IMAGE": "lab/kaniko@sha256:05",
}

// Temporary scaffolding for the migration from the cases files to
// ForgejoPreparedCases: each lane is the consumer rows of its former file,
// row for row, with the same demo, event and settings. It goes with the files.
func TestForgejoPreparedCasesMatchCasesFiles(t *testing.T) {
	t.Parallel()

	for lane, file := range map[string]string{
		LaneLifecycle: "lifecycle-forgejo.cases", LaneSnapshot: "snapshot-forgejo.cases",
		LaneReleaseVariants: "release-variants-forgejo.cases", LanePrivate: "k3s-private-forgejo.cases",
	} {
		t.Run(lane, func(t *testing.T) {
			t.Parallel()

			body, err := os.ReadFile(filepath.Join(reporoot.Path(t), "test/live/campaigns", file))
			if err != nil {
				t.Fatal(err)
			}

			var want []string

			for line := range strings.SplitSeq(string(body), "\n") {
				words := strings.Fields(line)
				if len(words) < 3 || words[0] != caseConsumer {
					continue
				}

				for index, word := range words[3:] {
					if key, name, isRef := strings.Cut(word, "=@"); isRef {
						words[3+index] = key + "=" + preparedImages[name]
					}
				}

				slices.Sort(words[3:])
				want = append(want, strings.Join(words[1:], " "))
			}

			var got []string

			for _, c := range ForgejoPreparedCases {
				if c.Lane != lane {
					continue
				}

				settings, err := c.Settings(preparedImages)
				if err != nil {
					t.Fatal(err)
				}

				slices.Sort(settings)
				got = append(got, strings.Join(append([]string{c.Demo, c.Event}, settings...), " "))
			}

			if len(want) == 0 || !slices.Equal(got, want) {
				t.Fatalf("the lane's cases:\n%s\nthe file's consumer rows:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
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

// Temporary scaffolding, as above, for GitLabPreparedCases: each lane is the
// consumer rows of its former file, with the archives each row named handed
// over as the consumer recipe exported them: a merge request's job archive
// as its consumer image archive. A lifecycle and a snapshot file left the
// builder to the run, so the table has each of their rows once per builder.
func TestGitLabPreparedCasesMatchCasesFiles(t *testing.T) {
	t.Parallel()

	archives := map[string]string{
		"job-archive": gitLabJobArchiveEnv, "build-archive": gitLabBuildArchiveEnv,
		"signer-archive": gitLabSignerArchiveEnv, "lint-archive": gitLabLintArchiveEnv,
	}

	for lane, file := range map[string]string{
		LaneLifecycle: "lifecycle-gitlab.cases", LaneSnapshot: "snapshot-gitlab.cases",
		LanePrivate: "k3s-private-gitlab.cases", LanePrivateAndroid: "k3s-private-android-gitlab.cases",
	} {
		t.Run(lane, func(t *testing.T) {
			t.Parallel()

			body, err := os.ReadFile(filepath.Join(reporoot.Path(t), "test/live/campaigns", file))
			if err != nil {
				t.Fatal(err)
			}

			var want []string

			for line := range strings.SplitSeq(string(body), "\n") {
				words := strings.Fields(line)
				if len(words) < 3 || words[0] != caseConsumer {
					continue
				}

				row := []string{}

				for _, word := range words[3:] {
					key, value, _ := strings.Cut(word, "=")
					if env, isArchive := archives[key]; isArchive {
						key, value = env, gitLabPrepared[strings.TrimPrefix(value, "@")]
					}

					if key == gitLabJobArchiveEnv && words[2] == TrialPullRequest {
						key = gitLabConsumerArchiveEnv
					}

					row = append(row, key+"="+value)
				}

				slices.Sort(row)
				want = append(want, strings.Join(append(words[1:3], row...), " "))
			}

			var got []string

			builders := map[string][]string{}

			for _, c := range GitLabPreparedCases {
				if c.Lane != lane {
					continue
				}

				environment, err := c.GitLabEnvironment(gitLabPrepared)
				if err != nil {
					t.Fatal(err)
				}

				row := c.GitLabSettings()

				for _, env := range []string{gitLabJobArchiveEnv, gitLabBuildArchiveEnv, gitLabSignerArchiveEnv, gitLabLintArchiveEnv, gitLabConsumerArchiveEnv} {
					if environment[env] != "" {
						row = append(row, env+"="+environment[env])
					}
				}

				slices.Sort(row)

				line := strings.Join(append([]string{c.Demo, c.Event}, row...), " ")
				if builders[line] = append(builders[line], c.Builder); len(builders[line]) == 1 {
					got = append(got, line)
				}
			}

			wantBuilders := []string{ImageBuilderKaniko}
			if lane == LaneLifecycle || lane == LaneSnapshot {
				wantBuilders = []string{ImageBuilderBuildah, ImageBuilderKaniko}
			}

			for line, ran := range builders {
				if !slices.Equal(ran, wantBuilders) {
					t.Errorf("%s runs with %v, want %v", line, ran, wantBuilders)
				}
			}

			if len(want) == 0 || !slices.Equal(got, want) {
				t.Fatalf("the lane's cases:\n%s\nthe file's consumer rows:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
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
		}},
		"android merge request": {PreparedCase{Demo: "hello-android", Event: TrialPullRequest, JobImage: flavorKotlin, Runtime: "android"}, map[string]string{
			gitLabConsumerArchiveEnv: "/c/android.tar", gitLabLintArchiveEnv: "/c/lint-kotlin.tar", gitLabJobArchiveEnv: "",
		}},
		"release": {PreparedCase{Demo: "hello-maven", Event: TrialRelease, JobImage: flavorJava, Version: "0.2.0"}, map[string]string{
			gitLabJobArchiveEnv: "/c/java.tar", gitLabBuildArchiveEnv: "/c/java.tar", gitLabSignerArchiveEnv: "/c/signer.tar", gitLabLintArchiveEnv: "/c/lint-java.tar",
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
}

// Temporary scaffolding, as above, for PreparedLanes: each lane is a former
// cases file by name, with the scenarios it named, in order, and cases of
// the table where the file had consumer rows.
func TestPreparedLanesMatchCasesFiles(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join(reporoot.Path(t), "test/live/campaigns", "*.cases"))
	if err != nil || len(files) != len(PreparedLanes) {
		t.Fatalf("%d cases files and %d lanes: %v", len(files), len(PreparedLanes), err)
	}

	for index, file := range files {
		lane := PreparedLanes[index]
		if name := strings.TrimSuffix(filepath.Base(file), ".cases"); lane.Name != name || !strings.HasSuffix(name, "-"+lane.Forge) {
			t.Fatalf("lane %d is %s on %s, and the file %s", index, lane.Name, lane.Forge, name)
		}

		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		var scenarios []string

		consumers := 0

		for line := range strings.SplitSeq(string(body), "\n") {
			switch words := strings.Fields(line); {
			case len(words) >= 2 && words[0] == settingScenario:
				scenarios = append(scenarios, words[1])
			case len(words) >= 3 && words[0] == caseConsumer:
				consumers++
			}
		}

		cases := 0

		for _, c := range PreparedCases(lane.Forge) {
			if lane.Lane != "" && c.Lane == lane.Lane {
				cases++
			}
		}

		if !slices.Equal(lane.Scenarios, scenarios) || (consumers == 0) != (cases == 0) {
			t.Errorf("%s: scenarios %v and %d cases; the file: %v and %d consumer rows", lane.Name, lane.Scenarios, cases, scenarios, consumers)
		}
	}
}
