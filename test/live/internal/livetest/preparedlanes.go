// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

// PreparedLane is one purpose's run on a forge, as the testing matrices list
// it: the cases of its lane in the forge's table (PreparedCases), and the
// live tests it runs beside them, by name. What a lane runs with is the
// caller's environment: FORGE_CI_LIVE_CONSUMER_VISIBILITY and
// FORGE_CI_LIVE_ADMIN_<CHECK>; the builder is each case's own.
type PreparedLane struct {
	// Name is <purpose>-<forge>; Forge is forgejo or gitlab.
	Name, Forge string
	// Lane is the lane of the forge's table whose cases it runs, or none.
	Lane string
	// Scenarios are the live tests the lane runs beside its cases.
	Scenarios []string
}

// PreparedLanes are the lanes, by name. `just test-live-prepared <dir>
// <forge> <pattern>` runs a lane's cases and scenarios from a prepared
// directory. The scenarios of build-capability-gitlab, harness-inputs-*,
// interruption-manual-gitlab and release-images-forgejo take an operator's
// own checkout or archive, or the operator's hand, and run through a recipe
// of their own instead: `just test-live-consumer <forge> <checkout> <event>
// [settings]`, test-live-gitlab-branch-protection, test-live-gitlab-buildah
// or test-live-gitlab-interrupted-job.
var PreparedLanes = []PreparedLane{
	{Name: "build-capability-gitlab", Forge: campaignGitLab, Scenarios: []string{"TestGitLabRelease_BuildImageCapability"}},
	// A reviewed native checkout's pull request, then its release.
	{Name: "harness-inputs-forgejo", Forge: campaignForgejo, Scenarios: []string{"TestForgejoLayer_ConsumerTrial", "TestForgejoLayer_ConsumerTrial"}},
	{Name: "harness-inputs-gitlab", Forge: campaignGitLab, Scenarios: []string{
		"TestGitLabRelease_ConsumerBranchProtection", gitlabConsumerTrialTest, gitlabPreseededTest, gitlabRecoveryTest,
	}},
	{Name: "interruption-manual-gitlab", Forge: campaignGitLab, Scenarios: []string{"TestGitLabWorkflow_InterruptedJob"}},
	{Name: "k3s-private-android-gitlab", Forge: campaignGitLab, Lane: LanePrivateAndroid},
	{Name: "k3s-private-forgejo", Forge: campaignForgejo, Lane: LanePrivate},
	// The minimal signed release and the narrower policy scenarios run in
	// the go runtime image, beside the demos' gates and releases.
	{Name: "k3s-private-gitlab", Forge: campaignGitLab, Lane: LanePrivate, Scenarios: []string{
		"TestGitLabRelease_MinimalSignedArtifacts",
		"TestGitLabCheck_ChecksOutPrivateConsumerWithJobToken",
		"TestGitLabCheck_ChecksOutTheConsumerPipelineCommit",
		"TestGitLabPolicy_ConsumerCannotReplaceTheSigningPipeline",
		"TestGitLabPolicy_JobTokenCannotWriteThePublicationRegistry",
		"TestGitLabRelease_MinimalBranchProtection",
		"TestGitLabRelease_ResourceGroupSpansTheChild",
		"TestGitLabRelease_PromotionRetryAfterRegistryWrite",
		"TestGitLabBootstrap_PublishesAndDownloadsTheRecordedBinary",
	}},
	{Name: "lifecycle-forgejo", Forge: campaignForgejo, Lane: LaneLifecycle},
	{Name: "lifecycle-gitlab", Forge: campaignGitLab, Lane: LaneLifecycle},
	{Name: "release-images-forgejo", Forge: campaignForgejo, Scenarios: []string{"TestForgejoLayer_ConsumerTrial"}},
	{Name: "release-variants-forgejo", Forge: campaignForgejo, Lane: LaneReleaseVariants},
	{Name: "snapshot-forgejo", Forge: campaignForgejo, Lane: LaneSnapshot},
	{Name: "snapshot-gitlab", Forge: campaignGitLab, Lane: LaneSnapshot},
}
