// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/enforcement"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// PreparedCase is one consumer trial of a shipped demo that a run takes from
// the prepared directory (PreparedEnv): the demo under <prepared>/demos, the
// trial's event, and the settings that vary between cases.
type PreparedCase struct {
	// Lane groups the cases one purpose runs together; it opens the case's
	// name, so `-run` selects a lane.
	Lane, Demo, Event string
	// JobImage is the flavor of the nanolinter image the demo's jobs run in,
	// CI_JOB_IMAGE, as prepared with job-image=<flavor>:<image>.
	JobImage string
	// Builder is the consumer's CI_IMAGE_BUILDER; kaniko builds in the
	// prepared kaniko job image.
	Builder string
	// Version, NextVersion and ReleaseImagesConfig are the trial's settings of
	// those names, each left out when empty.
	Version, NextVersion, ReleaseImagesConfig string
	// Platforms is RELEASE_PLATFORMS, left out when empty.
	Platforms string
	// Runtime is the runtime image target a GitLab case's jobs run in where
	// it is not its flavor's own (gitLabRuntimeTarget): hello-android's.
	Runtime string
}

// The lanes of the tables; PreparedLanes names each as the matrices do.
const (
	// LaneLifecycle: each demo with a development route through pull request,
	// merge, service snapshot and signed release, with either builder.
	LaneLifecycle = "lifecycle"
	// LaneSnapshot: the snapshot of each multi-image demo.
	LaneSnapshot = "snapshot"
	// LaneReleaseVariants: two releases in a row (NextVersion) and release
	// images read back (ReleaseImagesConfig), with Buildah.
	LaneReleaseVariants = "release-variants"
	// LanePrivate: each demo's pull request, then its release, with kaniko;
	// private repositories. On GitLab each demo's merge-request gate, then
	// the release of each demo that ships a GitLab release build.
	LanePrivate = "private"
	// LanePrivateAndroid: the Android merge-request gate, apart from
	// LanePrivate because its Android runtime image is prepared only on
	// request.
	LanePrivateAndroid = "private-android"
)

const (
	flavorGo     = "go"
	flavorJava   = "java"
	flavorJS     = "javascript-typescript"
	flavorKotlin = "kotlin"
	flavorRust   = "rust"

	nativePlatform = "linux/amd64"

	preparedKanikoJobImage = "KANIKO_JOB_IMAGE"
)

// ForgejoPreparedCases are the Forgejo consumer trials of the shipped demos.
// TestForgejoLayer_ConsumerTrial runs the pull-request and release cases,
// TestForgejoLayer_ConsumerLifecycle the lifecycles and
// TestForgejoLayer_Snapshot the snapshots, each as the subtest Name names.
var ForgejoPreparedCases = []PreparedCase{
	{Lane: LaneLifecycle, Demo: "hello-maven-service", Event: TrialLifecycle, JobImage: flavorJava, Builder: ImageBuilderBuildah, Platforms: nativePlatform},
	{Lane: LaneLifecycle, Demo: "hello-maven-service", Event: TrialLifecycle, JobImage: flavorJava, Builder: ImageBuilderKaniko, Platforms: nativePlatform},
	{Lane: LaneLifecycle, Demo: "hello-cargo-container", Event: TrialLifecycle, JobImage: flavorRust, Builder: ImageBuilderBuildah, Platforms: nativePlatform},
	{Lane: LaneLifecycle, Demo: "hello-npm-service", Event: TrialLifecycle, JobImage: flavorJS, Builder: ImageBuilderBuildah, Platforms: nativePlatform},
	{Lane: LaneLifecycle, Demo: "hello-go-service", Event: TrialLifecycle, JobImage: flavorGo, Builder: ImageBuilderBuildah, Platforms: nativePlatform},

	{Lane: LaneSnapshot, Demo: "hello-cargo-workspace", Event: TrialSnapshot, JobImage: flavorRust, Builder: ImageBuilderBuildah},

	{Lane: LaneReleaseVariants, Demo: "hello-go-cli", Event: TrialRelease, JobImage: flavorGo, Builder: ImageBuilderBuildah, NextVersion: "0.0.2", Platforms: nativePlatform},
	{Lane: LaneReleaseVariants, Demo: "hello-cargo-container", Event: TrialRelease, JobImage: flavorRust, Builder: ImageBuilderBuildah, Version: "0.2.0", NextVersion: "0.2.1", Platforms: nativePlatform},
	{Lane: LaneReleaseVariants, Demo: "hello-npm-package", Event: TrialRelease, JobImage: flavorJS, Builder: ImageBuilderBuildah, Version: "0.2.0", NextVersion: "0.2.1", Platforms: nativePlatform},
	{Lane: LaneReleaseVariants, Demo: "hello-maven", Event: TrialRelease, JobImage: flavorJava, Builder: ImageBuilderBuildah, NextVersion: "0.0.2", Platforms: nativePlatform},
	{Lane: LaneReleaseVariants, Demo: "hello-go-service", Event: TrialRelease, JobImage: flavorGo, Builder: ImageBuilderBuildah, ReleaseImagesConfig: ".forge-ci/images.yml", Platforms: nativePlatform},
	{Lane: LaneReleaseVariants, Demo: "hello-cargo-workspace", Event: TrialRelease, JobImage: flavorRust, Builder: ImageBuilderBuildah, Version: "0.2.0", ReleaseImagesConfig: ".forge-ci/release-images.yml", Platforms: nativePlatform},

	{Lane: LanePrivate, Demo: "hello-go-cli", Event: TrialPullRequest, JobImage: flavorGo, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-go-cli", Event: TrialRelease, JobImage: flavorGo, Builder: ImageBuilderKaniko, Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-go-service", Event: TrialPullRequest, JobImage: flavorGo, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-go-service", Event: TrialRelease, JobImage: flavorGo, Builder: ImageBuilderKaniko, Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-maven-service", Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-maven-service", Event: TrialRelease, JobImage: flavorJava, Builder: ImageBuilderKaniko, Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-npm-service", Event: TrialPullRequest, JobImage: flavorJS, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-npm-service", Event: TrialRelease, JobImage: flavorJS, Builder: ImageBuilderKaniko, Version: "0.2.0", Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-maven", Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-maven", Event: TrialRelease, JobImage: flavorJava, Builder: ImageBuilderKaniko, Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-cargo-container", Event: TrialPullRequest, JobImage: flavorRust, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-cargo-container", Event: TrialRelease, JobImage: flavorRust, Builder: ImageBuilderKaniko, Version: "0.2.0", Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-cargo-workspace", Event: TrialPullRequest, JobImage: flavorRust, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-cargo-workspace", Event: TrialRelease, JobImage: flavorRust, Builder: ImageBuilderKaniko, Version: "0.2.0", Platforms: nativePlatform},
	{Lane: LanePrivate, Demo: "hello-npm-package", Event: TrialPullRequest, JobImage: flavorJS, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-npm-package", Event: TrialRelease, JobImage: flavorJS, Builder: ImageBuilderKaniko, Version: "0.2.0", Platforms: nativePlatform},
}

// GitLabPreparedCases are the GitLab consumer trials of the shipped demos.
// TestGitLabLayer_ConsumerTrial runs the pull-request cases,
// TestGitLabRelease_ConsumerTrial the releases,
// TestGitLabLayer_ConsumerLifecycle the lifecycles and
// TestGitLabLayer_Snapshot the snapshots, each as the subtest Name names. A
// lifecycle and a release request Version and run their job-image jobs as the
// runtime images' nonroot user. A lifecycle and a snapshot run with either
// builder, a case each.
var GitLabPreparedCases = []PreparedCase{
	{Lane: LaneLifecycle, Demo: "hello-maven-service", Event: TrialLifecycle, JobImage: flavorJava, Builder: ImageBuilderBuildah, Version: "0.2.0"},
	{Lane: LaneLifecycle, Demo: "hello-maven-service", Event: TrialLifecycle, JobImage: flavorJava, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LaneLifecycle, Demo: "hello-cargo-container", Event: TrialLifecycle, JobImage: flavorRust, Builder: ImageBuilderBuildah, Version: "0.2.0"},
	{Lane: LaneLifecycle, Demo: "hello-cargo-container", Event: TrialLifecycle, JobImage: flavorRust, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LaneLifecycle, Demo: "hello-npm-service", Event: TrialLifecycle, JobImage: flavorJS, Builder: ImageBuilderBuildah, Version: "0.2.0"},
	{Lane: LaneLifecycle, Demo: "hello-npm-service", Event: TrialLifecycle, JobImage: flavorJS, Builder: ImageBuilderKaniko, Version: "0.2.0"},

	{Lane: LaneSnapshot, Demo: "hello-cargo-workspace", Event: TrialSnapshot, JobImage: flavorRust, Builder: ImageBuilderBuildah},
	{Lane: LaneSnapshot, Demo: "hello-cargo-workspace", Event: TrialSnapshot, JobImage: flavorRust, Builder: ImageBuilderKaniko},
	{Lane: LaneSnapshot, Demo: "hello-monorepo", Event: TrialSnapshot, JobImage: flavorJava, Builder: ImageBuilderBuildah},
	{Lane: LaneSnapshot, Demo: "hello-monorepo", Event: TrialSnapshot, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LaneSnapshot, Demo: "hello-go-service", Event: TrialSnapshot, JobImage: flavorGo, Builder: ImageBuilderBuildah},
	{Lane: LaneSnapshot, Demo: "hello-go-service", Event: TrialSnapshot, JobImage: flavorGo, Builder: ImageBuilderKaniko},

	{Lane: LanePrivate, Demo: "hello-go-cli", Event: TrialPullRequest, JobImage: flavorGo, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-maven", Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-npm-service", Event: TrialPullRequest, JobImage: flavorJS, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-cargo-container", Event: TrialPullRequest, JobImage: flavorRust, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-go-service", Event: TrialPullRequest, JobImage: flavorGo, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-maven-service", Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-gradle", Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-monorepo", Event: TrialPullRequest, JobImage: flavorJava, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-cargo-workspace", Event: TrialPullRequest, JobImage: flavorRust, Builder: ImageBuilderKaniko},
	{Lane: LanePrivate, Demo: "hello-npm-package", Event: TrialPullRequest, JobImage: flavorJS, Builder: ImageBuilderKaniko},

	{Lane: LanePrivate, Demo: "hello-maven", Event: TrialRelease, JobImage: flavorJava, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LanePrivate, Demo: "hello-npm-service", Event: TrialRelease, JobImage: flavorJS, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LanePrivate, Demo: "hello-maven-service", Event: TrialRelease, JobImage: flavorJava, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LanePrivate, Demo: "hello-cargo-container", Event: TrialRelease, JobImage: flavorRust, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LanePrivate, Demo: "hello-cargo-workspace", Event: TrialRelease, JobImage: flavorRust, Builder: ImageBuilderKaniko, Version: "0.2.0"},
	{Lane: LanePrivate, Demo: "hello-npm-package", Event: TrialRelease, JobImage: flavorJS, Builder: ImageBuilderKaniko, Version: "0.2.0"},

	{Lane: LanePrivateAndroid, Demo: "hello-android", Event: TrialPullRequest, JobImage: flavorKotlin, Builder: ImageBuilderKaniko, Runtime: "android"},
}

// PreparedCases are the cases of a lab forge's table, none for another.
func PreparedCases(forge string) []PreparedCase {
	switch forge {
	case campaignForgejo:
		return ForgejoPreparedCases
	case campaignGitLab:
		return GitLabPreparedCases
	default:
		return nil
	}
}

// preparedCaseTests name the live test that runs a forge's cases of an event.
var preparedCaseTests = map[string]map[string]string{
	campaignForgejo: {
		TrialPullRequest: "TestForgejoLayer_ConsumerTrial", TrialRelease: "TestForgejoLayer_ConsumerTrial",
		TrialLifecycle: "TestForgejoLayer_ConsumerLifecycle", TrialSnapshot: "TestForgejoLayer_Snapshot",
	},
	campaignGitLab: {
		TrialPullRequest: "TestGitLabLayer_ConsumerTrial", TrialRelease: gitlabConsumerTrialTest,
		TrialLifecycle: "TestGitLabLayer_ConsumerLifecycle", TrialSnapshot: "TestGitLabLayer_Snapshot",
	},
}

// PreparedCaseTest names the live test that runs a forge's cases of event,
// each as the subtest <test>/<case name>.
func PreparedCaseTest(forge, event string) string {
	return preparedCaseTests[forge][event]
}

// Name is the case's subtest name, <lane>/<demo>/<event>/<builder>, one per
// case. `-run` matches it element by element.
func (c PreparedCase) Name() string {
	return strings.Join([]string{c.Lane, c.Demo, c.Event, c.Builder}, "/")
}

// LaneEnvironment is what a private lane fixes for its cases on forge, so a
// run cannot leave it out: private repositories alone, and on GitLab the
// administrator's reproducible-build check turned on. Another lane fixes
// nothing; its visibility and administrator inputs are the caller's.
func (c PreparedCase) LaneEnvironment(forge string) map[string]string {
	if c.Lane != LanePrivate && c.Lane != LanePrivateAndroid {
		return nil
	}

	environment := map[string]string{consumerVisibilityEnv: visibilityPrivate}
	if forge == campaignGitLab {
		environment[adminInputEnvironment(enforcement.ReproducibleBuilds)] = "true"
	}

	return environment
}

// Settings are the trial's settings of the case, one key=value each, with the
// images taken from prepared, the prepared directory's campaign.env. A case
// whose image was not prepared is refused, naming the key that is missing.
func (c PreparedCase) Settings(prepared map[string]string) ([]string, error) {
	jobImage := campaignJobImagePrefix + campaignFlavorKey(c.JobImage)
	if prepared[jobImage] == "" {
		return nil, c.notPrepared(jobImage)
	}

	settings := []string{"CI_JOB_IMAGE=" + prepared[jobImage], "CI_IMAGE_BUILDER=" + c.Builder}

	if c.Builder == ImageBuilderKaniko {
		if prepared[preparedKanikoJobImage] == "" {
			return nil, c.notPrepared(preparedKanikoJobImage)
		}

		settings = append(settings, "CI_IMAGE_BUILDER_JOB_IMAGE="+prepared[preparedKanikoJobImage])
	}

	if c.Version != "" {
		settings = append(settings, settingVersion+"="+c.Version)
	}

	if c.NextVersion != "" {
		settings = append(settings, settingNextVersion+"="+c.NextVersion)
	}

	if c.ReleaseImagesConfig != "" {
		settings = append(settings, settingReleaseImagesConfig+"="+c.ReleaseImagesConfig)
	}

	if c.Platforms != "" {
		settings = append(settings, "RELEASE_PLATFORMS="+c.Platforms)
	}

	return settings, nil
}

func (c PreparedCase) notPrepared(key string) error {
	return fmt.Errorf("%s: %s has no %s; prepare its image: %w", c.Name(), CampaignEnvFile, key, errs.ErrValidation)
}

// PreparedForgejoEnvironment is what a Forgejo consumer trial reads from its
// environment and the prepared directory's campaign.env holds: the signer
// image and the builders' saved job images. One that was not prepared is
// empty, so a value left in the caller's environment is never taken for it.
// The administrator inputs (FORGE_CI_LIVE_ADMIN_<CHECK>) are the caller's.
func PreparedForgejoEnvironment(prepared map[string]string) map[string]string {
	environment := map[string]string{}

	for _, key := range []string{signerImageEnv, GitLabSnapshotBuilderArchiveEnv(ImageBuilderBuildah), KanikoJobImageArchiveEnv} {
		environment[key] = prepared[key]
	}

	return environment
}

// The archives a GitLab consumer trial reads from its environment, as
// `just test-live-consumer gitlab` names them from job-archive=,
// build-archive=, signer-archive= and lint-archive=; a merge request's job
// archive is its consumer image archive.
const (
	gitLabJobArchiveEnv      = "FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE"
	gitLabBuildArchiveEnv    = "FORGE_CI_LIVE_BUILD_IMAGE_ARCHIVE"
	gitLabSignerArchiveEnv   = "FORGE_CI_LIVE_SIGNER_IMAGE_ARCHIVE"
	gitLabLintArchiveEnv     = "FORGE_CI_LIVE_LINT_IMAGE_ARCHIVE"
	gitLabConsumerArchiveEnv = "FORGE_CI_LIVE_CONSUMER_IMAGE_ARCHIVE"

	// The user a release's and a lifecycle's job images run as.
	gitLabReleaseJobUser = "65532:65532"
)

// gitLabRuntimeTarget is the runtime image target a flavor's demos build and
// check in on GitLab, the <TARGET> of GitLabJobImageArchiveEnv in capitals.
var gitLabRuntimeTarget = map[string]string{flavorGo: "go", flavorJava: "java", flavorJS: "node", flavorRust: "rust"}

// GitLabSettings are the trial's settings of the case on GitLab, one
// key=value each: a release's and a lifecycle's version and job user, and
// none for a merge request or a snapshot, which take their images alone. A
// release names no scenario, so it is the consumer release trial
// (GitLabConsumerReleaseScenario).
func (c PreparedCase) GitLabSettings() []string {
	if c.Event != TrialLifecycle && c.Event != TrialRelease {
		return nil
	}

	return []string{settingVersion + "=" + c.Version, "job-user=" + gitLabReleaseJobUser}
}

// GitLabEnvironment is what a GitLab consumer trial of the case reads from
// its environment and prepared, the prepared directory's campaign.env, holds.
// The job image is the runtime archive of the case's flavor, or of Runtime.
// A snapshot takes it alone, and builds in the saved job image of its
// builder. A merge request takes it as its consumer image and lints in the
// flavor's nanolinter archive. A release and a lifecycle take it as job and
// build image, sign with the prepared signer and lint as the merge request
// does. A missing archive is refused, naming its key; one the event does not
// take is empty, as is any that was not prepared: a value left in the
// caller's environment is never taken for one.
func (c PreparedCase) GitLabEnvironment(prepared map[string]string) (map[string]string, error) {
	environment := map[string]string{gitLabJobArchiveEnv: "", gitLabBuildArchiveEnv: "", gitLabSignerArchiveEnv: "", gitLabLintArchiveEnv: ""}

	for _, key := range []string{signerImageEnv, GitLabSnapshotBuilderArchiveEnv(ImageBuilderBuildah), KanikoJobImageArchiveEnv} {
		environment[key] = prepared[key]
	}

	// A gate's further job image takes its runtime archive by name.
	for key, value := range prepared {
		if strings.HasPrefix(key, GitLabJobImageArchiveEnv) {
			environment[key] = value
		}
	}

	job := GitLabJobImageArchiveEnv + strings.ToUpper(cmp.Or(c.Runtime, gitLabRuntimeTarget[c.JobImage]))
	if prepared[job] == "" {
		return nil, c.notPrepared(job)
	}

	if c.Event == TrialSnapshot {
		if builder := GitLabSnapshotBuilderArchiveEnv(c.Builder); prepared[builder] == "" {
			return nil, c.notPrepared(builder)
		}

		environment[gitLabJobArchiveEnv] = prepared[job]

		return environment, nil
	}

	lint := campaignLintArchivePrefix + campaignFlavorKey(c.JobImage)
	if prepared[lint] == "" {
		return nil, c.notPrepared(lint)
	}

	environment[gitLabLintArchiveEnv] = prepared[lint]

	if c.Event == TrialPullRequest {
		environment[gitLabConsumerArchiveEnv] = prepared[job]

		return environment, nil
	}

	if prepared[gitLabSignerArchiveEnv] == "" {
		return nil, c.notPrepared(gitLabSignerArchiveEnv)
	}

	environment[gitLabJobArchiveEnv] = prepared[job]
	environment[gitLabBuildArchiveEnv] = prepared[job]
	environment[gitLabSignerArchiveEnv] = prepared[gitLabSignerArchiveEnv]

	return environment, nil
}
