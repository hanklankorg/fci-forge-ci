// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package pipeline

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/domain/version"
)

// DecodeClassifiedFileReleasePlan reads the existing release contract, including
// its embedded stages. Use the same strict reader and duplicate-member admission
// as the existing selected-test preflight, not a second projection from JSON.
func DecodeClassifiedFileReleasePlan(raw string) (ReleasePlan, error) {
	var plan ReleasePlan
	if err := DecodeStagePlan(raw, "classified release-plan-json", &plan); err != nil {
		return plan, err
	}

	if err := checkConfigPlanMembers(json.RawMessage(raw), reflect.TypeFor[ReleasePlan](), "release-plan"); err != nil {
		return plan, err
	}

	return plan, nil
}

// ValidateClassifiedFilePlans is a side-effect-free eligibility predicate over
// current plan types, not a workflow plan constructor or source authorization.
// generatedSBOMs is the original caller release.sboms input: effective policy
// alone cannot reveal a generation request whose intersection resolved to none.
// No FileManifest is read or modified; none here never suppresses producer SBOMs.
func ValidateClassifiedFilePlans(cfg ConfigPlan, plan ReleasePlan, generatedSBOMs string) (PlannedArtifact, error) {
	if err := ValidateConfigPlan(cfg); err != nil {
		return PlannedArtifact{}, err
	}

	if cfg.Containers.HasContainers || len(cfg.Containers.All) != 0 || plan.Policy.HasContainers {
		return PlannedArtifact{}, fmt.Errorf("classified files refuse container declarations and release image policy: %w", errs.ErrInvalidConfig)
	}

	artifact, err := classifiedRootArtifact(cfg)
	if err != nil {
		return PlannedArtifact{}, err
	}

	for name, value := range map[string]string{"release.sboms": generatedSBOMs, "pipeline_sboms": cfg.PipelineSBOMs, "policy.sboms": plan.Policy.SBOMs} {
		if value != SBOMsNone {
			return PlannedArtifact{}, fmt.Errorf("classified files require explicit %s=none for engine-generated layers; producer manifest SBOM roles remain intact: %w", name, errs.ErrInvalidConfig)
		}
	}

	if len(artifact.EffectiveSBOMs) != 0 || plan.Policy.SBOMConflict != nil {
		return PlannedArtifact{}, fmt.Errorf("classified files refuse generated-layer SBOM requests or conflicts: %w", errs.ErrInvalidConfig)
	}

	if err := validateClassifiedReleaseContext(plan); err != nil {
		return PlannedArtifact{}, err
	}

	if err := validateClassifiedStages(cfg, plan); err != nil {
		return PlannedArtifact{}, err
	}

	return artifact, nil
}

func classifiedRootArtifact(cfg ConfigPlan) (PlannedArtifact, error) {
	if len(cfg.Artifacts.All) != 1 {
		return PlannedArtifact{}, fmt.Errorf("classified files support exactly one root Go or Cargo artifact: %w", errs.ErrInvalidConfig)
	}

	artifact := cfg.Artifacts.All[0]
	if artifact.WorkingDirectory != "." || len(artifact.PublishTo) != 0 || artifact.BuildType != "" {
		return PlannedArtifact{}, fmt.Errorf("classified artifact must be a root project with no registry publication or library build-type: %w", errs.ErrInvalidConfig)
	}

	if artifact.Maven != nil || artifact.NPM != nil || artifact.Gradle != nil || artifact.GradleAndroid != nil || artifact.XcodeIOS != nil || artifact.Python != nil {
		return PlannedArtifact{}, fmt.Errorf("classified artifact has unsupported ecosystem configuration: %w", errs.ErrInvalidConfig)
	}

	if err := validateClassifiedNativeConfig(artifact); err != nil {
		return PlannedArtifact{}, err
	}

	return artifact, nil
}

func validateClassifiedNativeConfig(artifact PlannedArtifact) error {
	switch artifact.ProjectType {
	case projecttype.Go:
		if artifact.Go == nil || artifact.Cargo != nil || artifact.GoBuildMode != config.GoBuildModeArtifactFirst || artifact.Go.SkipTests {
			return fmt.Errorf("classified Go requires artifact-first configuration and mandatory tests: %w", errs.ErrInvalidConfig)
		}
	case projecttype.Cargo:
		if artifact.Cargo == nil || artifact.Go != nil || artifact.CargoBuildMode != config.CargoBuildModeArtifactFirst || artifact.Cargo.SkipTests {
			return fmt.Errorf("classified Cargo requires artifact-first configuration and mandatory tests: %w", errs.ErrInvalidConfig)
		}
	default:
		return fmt.Errorf("classified files support only root Go or Cargo: %w", errs.ErrInvalidConfig)
	}

	return nil
}

func validateClassifiedReleaseContext(plan ReleasePlan) error {
	if plan.Version != ReleasePlanVersion {
		return fmt.Errorf("unsupported classified release plan version: %w", errs.ErrInvalidConfig)
	}

	if plan.Context.Branch == "" || plan.Context.Branch != strings.TrimSpace(plan.Context.Branch) || plan.Context.FilePattern != "" || plan.Context.ChangelogCreator != "git-cliff" || plan.Context.ReleasePublisher != "github-cli" {
		return fmt.Errorf("classified files require automatic root-project prepare paths, git-cliff and github-cli release publication: %w", errs.ErrInvalidConfig)
	}

	parsed, err := version.ValidateReleaseTag(plan.Context.RefName, plan.Context.ReleasePolicy)
	if err != nil {
		return err
	}

	policy := plan.Policy
	if !policy.RunVersionBump || !policy.SignArtifacts || !policy.RequireAllowlistedSigner || !policy.CreateRelease || policy.Prerelease != (parsed.Prerelease != "") || policy.MakeLatest != (parsed.Prerelease == "") {
		return fmt.Errorf("classified release requires signed allowlisted preparation and coherent release policy: %w", errs.ErrInvalidConfig)
	}

	return nil
}

func validateClassifiedStages(cfg ConfigPlan, plan ReleasePlan) error {
	const buildStageName = "build"

	prepare, build, publish := plan.Stages.Prepare, plan.Stages.Build, plan.Stages.Publish
	for _, stage := range []struct {
		version   int
		got, want string
	}{
		{prepare.Version, prepare.Stage, "prepare"},
		{build.Version, build.Stage, buildStageName},
		{publish.Version, publish.Stage, "publish"},
	} {
		if stage.version != ReleasePlanVersion || stage.got != stage.want {
			return fmt.Errorf("invalid classified release stage version or identity: %w", errs.ErrInvalidConfig)
		}
	}

	expectedPrepare := newReleasePrepareStagePlan(cfg, plan.Policy, plan.Context.FilePattern)
	if prepare.FilePattern != expectedPrepare.FilePattern || !sameClassifiedTarget(prepare.Targets.VersionBump, expectedPrepare.Targets.VersionBump) {
		return fmt.Errorf("classified prepare target disagrees with the root artifact: %w", errs.ErrInvalidConfig)
	}

	expectedBuild := newReleaseBuildStagePlan(cfg)
	for _, pair := range []struct{ got, want TargetPlan[PlannedArtifact] }{
		{build.Targets.Go, expectedBuild.Targets.Go}, {build.Targets.Cargo, expectedBuild.Targets.Cargo},
		{build.Targets.Maven, expectedBuild.Targets.Maven}, {build.Targets.NPM, expectedBuild.Targets.NPM},
		{build.Targets.Gradle, expectedBuild.Targets.Gradle}, {build.Targets.GradleAndroid, expectedBuild.Targets.GradleAndroid}, {build.Targets.XcodeIOS, expectedBuild.Targets.XcodeIOS},
	} {
		if !sameClassifiedTarget(pair.got, pair.want) {
			return fmt.Errorf("classified build target disagrees with root artifact: %w", errs.ErrInvalidConfig)
		}
	}

	if publish.Targets.Containers.Runs || len(publish.Targets.Containers.Items) != 0 {
		return fmt.Errorf("classified files refuse image publication targets: %w", errs.ErrInvalidConfig)
	}

	for _, target := range []TargetPlan[PlannedArtifact]{publish.Targets.ForgePackages, publish.Targets.MavenCentral, publish.Targets.GooglePlay, publish.Targets.XcodeIOS, publish.Targets.GoContainerFirst, publish.Targets.CargoContainerFirst} {
		if target.Runs || len(target.Items) != 0 {
			return fmt.Errorf("classified files refuse registry publication and container-first SBOM targets: %w", errs.ErrInvalidConfig)
		}
	}

	expectedTransfers := newReleaseArtifactTransferPlan(cfg, plan.Policy)
	if plan.ArtifactTransfers.Version != ArtifactTransferPlanVersion || !slices.Equal(plan.ArtifactTransfers.Items, expectedTransfers.Items) {
		return fmt.Errorf("classified legacy transfer plan disagrees with file-only policy: %w", errs.ErrInvalidConfig)
	}

	return nil
}

func sameClassifiedTarget[T any](got, want TargetPlan[T]) bool {
	if got.Runs != want.Runs || len(got.Items) != len(want.Items) {
		return false
	}

	return len(got.Items) == 0 || reflect.DeepEqual(got.Items, want.Items)
}
