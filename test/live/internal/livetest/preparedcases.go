// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"strings"

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
}

// The lanes of ForgejoPreparedCases.
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
	// the caller selects private repositories.
	LanePrivate = "private"
)

const (
	flavorGo   = "go"
	flavorJava = "java"
	flavorJS   = "javascript-typescript"
	flavorRust = "rust"

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

// Name is the case's subtest name, <lane>/<demo>/<event>/<builder>, one per
// case. `-run` matches it element by element.
func (c PreparedCase) Name() string {
	return strings.Join([]string{c.Lane, c.Demo, c.Event, c.Builder}, "/")
}

// Settings are the trial's settings of the case, one key=value each, with the
// images taken from prepared, the prepared directory's campaign.env. A case
// whose image was not prepared is refused, naming the key that is missing.
func (c PreparedCase) Settings(prepared map[string]string) ([]string, error) {
	jobImage := "JOB_IMAGE_" + strings.ToUpper(strings.ReplaceAll(c.JobImage, "-", "_"))
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
// The administrator inputs (FORGE_CI_LIVE_ADMIN_<CHECK>) are a later slice.
func PreparedForgejoEnvironment(prepared map[string]string) map[string]string {
	environment := map[string]string{}

	for _, key := range []string{signerImageEnv, GitLabSnapshotBuilderArchiveEnv(ImageBuilderBuildah), KanikoJobImageArchiveEnv} {
		environment[key] = prepared[key]
	}

	return environment
}
