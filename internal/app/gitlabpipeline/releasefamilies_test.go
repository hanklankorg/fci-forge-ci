// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package gitlabpipeline_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlabpipeline "github.com/diggsweden/forge-ci/internal/app/gitlabpipeline"
	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
)

// monorepoRelease is the plan of a Maven backend, an npm frontend and a Go
// tool, each bumped and built.
func monorepoRelease() (pipeline.ReleasePrepareStagePlan, pipeline.ReleaseBuildStagePlan) {
	backend := pipeline.PlannedArtifact{Name: "backend", ProjectType: projecttype.Maven, WorkingDirectory: "backend", BuildType: config.BuildTypeApplication}
	frontend := pipeline.PlannedArtifact{Name: "frontend", ProjectType: projecttype.NPM, WorkingDirectory: "frontend"}
	tool := pipeline.PlannedArtifact{Name: "tool", ProjectType: projecttype.Go, WorkingDirectory: "tool"}

	prepare := pipeline.ReleasePrepareStagePlan{Version: 1, Stage: "prepare", Targets: pipeline.ReleasePrepareTargets{
		VersionBump: runningTarget(backend, frontend, tool),
	}}
	build := pipeline.ReleaseBuildStagePlan{Version: 1, Stage: "build", Targets: pipeline.ReleaseBuildTargets{
		Maven: runningTarget(backend), NPM: runningTarget(frontend), Go: runningTarget(tool),
	}}

	return prepare, build
}

// The plan names the jobs: a version job for each family whose projects a
// native tool bumps (Go has none), a build job for each family it builds, and
// each job only extends its hidden job in the pinned template.
func TestPlanReleaseFamilies_NamesThePlannedFamiliesJobs(t *testing.T) {
	t.Parallel()

	prepare, build := monorepoRelease()

	families, err := gitlabpipeline.PlanReleaseFamilies(prepare, build)
	require.NoError(t, err)
	assert.Equal(t, gitlabpipeline.ReleaseFamilies{Version: []string{"java", "node"}, Build: []string{"java", "node", "go"}}, families)
	assert.Equal(t, map[string]any{
		"Prepare: version java": map[string]any{"extends": ".forge-ci-release-version-java"},
		"Prepare: version node": map[string]any{"extends": ".forge-ci-release-version-node"},
		"Build: java":           map[string]any{"extends": ".forge-ci-release-build-java"},
		"Build: node":           map[string]any{"extends": ".forge-ci-release-build-node"},
		"Build: go":             map[string]any{"extends": ".forge-ci-release-build-go"},
	}, families.Jobs())
	assert.Equal(t, []string{"Prepare: version java", "Prepare: version node", "Build: java", "Build: node", "Build: go"}, families.JobNames())

	// A plan that does not bump runs no version job.
	prepare.Targets.VersionBump.Runs = false
	families, err = gitlabpipeline.PlanReleaseFamilies(prepare, build)
	require.NoError(t, err)
	assert.Empty(t, families.Version)
}

// GitLab refuses an empty configuration file, so a release without a family
// still writes one hidden key.
func TestReleaseFamilies_JobsOfNoFamilyIsOneHiddenKey(t *testing.T) {
	t.Parallel()

	prepare, build := pipeline.UndeclaredReleaseStagePlans()

	families, err := gitlabpipeline.PlanReleaseFamilies(prepare, build)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{".forge-ci-release-families": map[string]any{}}, families.Jobs())
}

func TestPlanReleaseFamilies_RefusesWhatNoJobCanRun(t *testing.T) {
	t.Parallel()

	prepare, build := monorepoRelease()

	images := testRuntimeImages
	images.Node = ""

	families, err := gitlabpipeline.PlanReleaseFamilies(prepare, build)
	require.NoError(t, err)
	require.NoError(t, families.RequireImages(testRuntimeImages))
	err = families.RequireImages(images)
	require.ErrorIs(t, err, errs.ErrUsage)
	require.ErrorContains(t, err, "runtime-image-node")

	// The bump alone needs the family's image too.
	build.Targets.NPM = pipeline.TargetPlan[pipeline.PlannedArtifact]{}
	families, err = gitlabpipeline.PlanReleaseFamilies(prepare, build)
	require.NoError(t, err)
	require.ErrorContains(t, families.RequireImages(images), "runtime-image-node")

	build.Targets.XcodeIOS = runningTarget(pipeline.PlannedArtifact{Name: "app", ProjectType: projecttype.XcodeIOS, WorkingDirectory: "ios"})
	_, err = gitlabpipeline.PlanReleaseFamilies(prepare, build)
	require.ErrorIs(t, err, errs.ErrUnsupported)
}

func TestReleaseFamilyArtifacts_ListsTheFamilyPlannedArtifacts(t *testing.T) {
	t.Parallel()

	_, build := monorepoRelease()
	build.Targets.Gradle = runningTarget(pipeline.PlannedArtifact{Name: "service", ProjectType: projecttype.Gradle, WorkingDirectory: "service"})

	names, err := gitlabpipeline.ReleaseFamilyArtifacts(build, "java")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"backend", "service"}, names)

	names, err = gitlabpipeline.ReleaseFamilyArtifacts(build, "rust")
	require.NoError(t, err)
	assert.Empty(t, names)

	_, err = gitlabpipeline.ReleaseFamilyArtifacts(build, "python")
	require.ErrorIs(t, err, errs.ErrUsage)
}
