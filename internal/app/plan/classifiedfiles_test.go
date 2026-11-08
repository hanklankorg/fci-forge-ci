// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package plan_test

import (
	"encoding/json"
	"strings"
	"testing"

	appplan "github.com/diggsweden/forge-ci/internal/app/plan"
	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/stretchr/testify/require"
)

func classifiedRouteFixture(t *testing.T, kind projecttype.Type) (appplan.ClassifiedFileAdmissionInput, pipeline.ConfigPlan, pipeline.ReleasePlan) {
	t.Helper()

	artifact := config.Artifact{Name: "product", ProjectType: kind, WorkingDirectory: ".", SBOMs: "none"}
	switch kind {
	case projecttype.Go:
		artifact.Go = &config.GoConfig{BuildMode: config.GoBuildModeArtifactFirst}
	case projecttype.Cargo:
		artifact.Cargo = &config.CargoConfig{BuildMode: config.CargoBuildModeArtifactFirst}
	default:
		t.Fatalf("unsupported fixture project type: %s", kind)
	}

	cfg := pipeline.NewConfigPlan(&config.Config{Artifacts: []config.Artifact{artifact}})
	release, err := pipeline.NewReleasePlan(pipeline.ReleasePlanInput{
		ConfigPlan: cfg, RepositoryVisibility: "private", Branch: "main", RefName: "v1.2.3", ReleasePublisher: "github-cli",
		ReleaseRequireAllowlistedSigner: true, ReleaseSignArtifacts: true, ReleaseSBOMs: "none", ChangelogCreator: "git-cliff",
	})
	require.NoError(t, err)

	in := appplan.ClassifiedFileAdmissionInput{ProducerFileOnly: true, ReleaseSBOMs: "none"}
	if kind == projecttype.Go {
		in.GoTestSelectionsJSON = `{"product":{"packages":["./test/policy"],"gate":"policy"}}`
	} else {
		in.CargoTestSelectionsJSON = `{"product":{"runs":[{"kind":"package-test","package":"policy","name":"workflow_policy","gate":"policy"},{"kind":"package-bin","package":"product","name":"product","gate":"application"}]}}`
	}

	encodeClassifiedRoute(t, &in, cfg, release)
	// Model separately transported contracts: mutating a stage in a refusal
	// test must not also mutate the config or another stage through slice aliases.
	release, err = pipeline.DecodeClassifiedFileReleasePlan(in.ReleasePlanJSON)
	require.NoError(t, err)

	return in, cfg, release
}

func encodeClassifiedRoute(t *testing.T, in *appplan.ClassifiedFileAdmissionInput, cfg pipeline.ConfigPlan, release pipeline.ReleasePlan) {
	t.Helper()

	body, err := json.Marshal(cfg)
	require.NoError(t, err)

	in.ConfigPlanJSON = string(body)
	body, err = json.Marshal(release)
	require.NoError(t, err)

	in.ReleasePlanJSON = string(body)
}

func TestClassifiedRouteAdmitsExistingRootPlansWithoutMutation(t *testing.T) {
	t.Parallel()

	for _, kind := range []projecttype.Type{projecttype.Go, projecttype.Cargo} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			in, cfg, release := classifiedRouteFixture(t, kind)
			before := in
			require.NoError(t, appplan.AdmitClassifiedFiles(in))
			require.Equal(t, before, in, "admission must not rewrite caller inputs")
			// Empty and nil inactive target collections both mean no work.
			release.Stages.Publish.Targets.Containers.Items = []pipeline.PlannedContainer{}
			release.Stages.Publish.Targets.ForgePackages.Items = []pipeline.PlannedArtifact{}
			release.Stages.Build.Targets.NPM.Items = []pipeline.PlannedArtifact{}
			encodeClassifiedRoute(t, &in, cfg, release)
			require.NoError(t, appplan.AdmitClassifiedFiles(in))
		})
	}
}

func TestClassifiedRouteRefusesInconsistentAndUnsupportedPlans(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{
		"config-image-flag", "config-hidden-image", "release-image-flag", "publish-image-flag", "publish-hidden-image",
		"wrong-release-version", "wrong-stage-version", "wrong-stage-identity", "prepare-not-running", "prepare-no-items", "prepare-foreign-copy",
		"build-not-running", "build-no-items", "build-foreign-copy", "hidden-other-build", "prepare-profile", "explicit-pathspec", "skipped-prepare",
		"no-signing", "no-allowlist", "no-public-release", "wrong-latest", "engine-sbom-request", "policy-sbom-request", "hidden-sbom-transfer", "missing-config", "missing-release",
	} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			in, cfg, release := classifiedRouteFixture(t, projecttype.Go)

			switch scenario {
			case "config-image-flag":
				cfg.Containers.HasContainers = true
			case "config-hidden-image":
				cfg.Containers.All = []pipeline.PlannedContainer{{Name: "hidden"}}
			case "release-image-flag":
				release.Policy.HasContainers = true
			case "publish-image-flag":
				release.Stages.Publish.Targets.Containers.Runs = true
			case "publish-hidden-image":
				release.Stages.Publish.Targets.Containers.Items = []pipeline.PlannedContainer{{Name: "hidden"}}
			case "wrong-release-version":
				release.Version++
			case "wrong-stage-version":
				release.Stages.Publish.Version++
			case "wrong-stage-identity":
				release.Stages.Prepare.Stage = "publish"
			case "prepare-not-running":
				release.Stages.Prepare.Targets.VersionBump.Runs = false
			case "prepare-no-items":
				release.Stages.Prepare.Targets.VersionBump.Items = nil
			case "prepare-foreign-copy":
				release.Stages.Prepare.Targets.VersionBump.Items[0].WorkingDirectory = "foreign"
			case "build-not-running":
				release.Stages.Build.Targets.Go.Runs = false
			case "build-no-items":
				release.Stages.Build.Targets.Go.Items = nil
			case "build-foreign-copy":
				release.Stages.Build.Targets.Go.Items[0].Name = "foreign"
			case "hidden-other-build":
				release.Stages.Build.Targets.NPM.Items = []pipeline.PlannedArtifact{{Name: "hidden"}}
			case "prepare-profile":
				release.Context.ChangelogCreator = "consumer-hook"
			case "explicit-pathspec":
				release.Stages.Prepare.FilePattern = "*"
			case "skipped-prepare":
				release.Policy.RunVersionBump = false
			case "no-signing":
				release.Policy.SignArtifacts = false
			case "no-allowlist":
				release.Policy.RequireAllowlistedSigner = false
			case "no-public-release":
				release.Policy.CreateRelease = false
			case "wrong-latest":
				release.Policy.MakeLatest = false
			case "engine-sbom-request":
				in.ReleaseSBOMs = "all" // effective policy is still none
			case "policy-sbom-request":
				release.Policy.SBOMs = string(config.SBOMLayerBuild)
			case "hidden-sbom-transfer":
				release.ArtifactTransfers.Items = append(release.ArtifactTransfers.Items, pipeline.ArtifactTransfer{Kind: pipeline.ArtifactTransferBuildSBOM, Name: "hidden", Path: "sboms", Required: true})
			}

			encodeClassifiedRoute(t, &in, cfg, release)

			if scenario == "missing-config" {
				in.ConfigPlanJSON = ""
			}

			if scenario == "missing-release" {
				in.ReleasePlanJSON = ""
			}

			require.Error(t, appplan.AdmitClassifiedFiles(in))
		})
	}
}

func TestClassifiedRouteRefusesEveryPublishTargetEvenWithHiddenItems(t *testing.T) {
	t.Parallel()

	for _, target := range []struct {
		name string
		get  func(*pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact]
	}{
		{"forge-packages", func(p *pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact] {
			return &p.ForgePackages
		}},
		{"maven-central", func(p *pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact] {
			return &p.MavenCentral
		}},
		{"google-play", func(p *pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact] {
			return &p.GooglePlay
		}},
		{"appstore", func(p *pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact] {
			return &p.XcodeIOS
		}},
		{"cargo-container-first", func(p *pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact] {
			return &p.CargoContainerFirst
		}},
		{"go-container-first", func(p *pipeline.ReleasePublishTargets) *pipeline.TargetPlan[pipeline.PlannedArtifact] {
			return &p.GoContainerFirst
		}},
	} {
		for _, hidden := range []bool{false, true} {
			t.Run(target.name+map[bool]string{false: "/running", true: "/hidden-items"}[hidden], func(t *testing.T) {
				t.Parallel()
				in, cfg, release := classifiedRouteFixture(t, projecttype.Go)
				publication := target.get(&release.Stages.Publish.Targets)

				publication.Runs = !hidden
				if hidden {
					publication.Items = []pipeline.PlannedArtifact{{Name: "hidden"}}
				}

				encodeClassifiedRoute(t, &in, cfg, release)
				require.ErrorIs(t, appplan.AdmitClassifiedFiles(in), errs.ErrInvalidConfig)
			})
		}
	}
}

func TestClassifiedRouteRequiresDeclaredScopeAndNoSeparateImagePlans(t *testing.T) {
	t.Parallel()
	in, _, _ := classifiedRouteFixture(t, projecttype.Go)
	in.ProducerFileOnly = false
	require.ErrorIs(t, appplan.AdmitClassifiedFiles(in), errs.ErrUsage)

	in.ProducerFileOnly = true
	for _, images := range []*pipeline.ReleaseImagesPlan{{}, {Images: []pipeline.ReleaseImage{{Name: "base", ImageKind: "base", Flavor: "core"}}}, {Images: []pipeline.ReleaseImage{{Name: "flavor", Flavor: "rust"}}}} {
		in.ImagePlan = images
		require.ErrorContains(t, appplan.AdmitClassifiedFiles(in), "image, base or flavor plans")
	}
}

func TestClassifiedRouteSelectionsCannotWaivePolicyWithApplicationEnforcementFalse(t *testing.T) {
	t.Parallel()
	// There is intentionally no enforce-application-tests input to this pure
	// predicate. Mandatory policy selection cannot become report-only.
	for _, kind := range []projecttype.Type{projecttype.Go, projecttype.Cargo} {
		for _, selection := range []string{"", "{}", "null", `{"foreign":{}}`, `{"product":{"packages":["."],"gate":"application"}}`, `{"product":{"runs":[{"kind":"package-bin","package":"product","name":"product","gate":"application"}]}}`} {
			t.Run(string(kind)+"/"+selection, func(t *testing.T) {
				t.Parallel()

				in, _, _ := classifiedRouteFixture(t, kind)
				if kind == projecttype.Go {
					in.GoTestSelectionsJSON = selection
				} else {
					in.CargoTestSelectionsJSON = selection
				}

				require.Error(t, appplan.AdmitClassifiedFiles(in))
			})
		}
	}
}

func TestClassifiedRouteStrictStageReaderRejectsUnknownAndDuplicateClaims(t *testing.T) {
	t.Parallel()

	in, _, _ := classifiedRouteFixture(t, projecttype.Go)
	for _, raw := range []string{
		strings.Replace(in.ReleasePlanJSON, `"targets":`, `"base_images":{},"targets":`, 1),
		strings.Replace(in.ReleasePlanJSON, `"has_containers":false`, `"has_containers":true,"has_containers":false`, 1),
		strings.Replace(in.ReleasePlanJSON, `"runs":true`, `"runs":false,"runs":true`, 1),
		in.ReleasePlanJSON + " null",
	} {
		bad := in
		bad.ReleasePlanJSON = raw
		require.Error(t, appplan.AdmitClassifiedFiles(bad))
	}
}

func TestClassifiedRouteRejectsConsistentlyPlannedUnsupportedProducerProfiles(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"subdirectory", "container-first", "skip-tests", "other-ecosystem", "multiple-artifacts", "registry", "generated-sboms"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			in, _, _ := classifiedRouteFixture(t, projecttype.Go)
			artifact := config.Artifact{Name: "product", ProjectType: projecttype.Go, WorkingDirectory: ".", SBOMs: "none", Go: &config.GoConfig{BuildMode: config.GoBuildModeArtifactFirst}}

			switch scenario {
			case "subdirectory":
				artifact.WorkingDirectory = "subproject"
			case "container-first":
				artifact.Go.BuildMode = config.GoBuildModeContainerFirst
			case "skip-tests":
				artifact.Go.SkipTests = true
			case "other-ecosystem":
				artifact.ProjectType, artifact.Go = projecttype.Meta, nil
			case "registry":
				artifact.PublishTo = []config.PublishTarget{config.PublishForgePackages}
			case "generated-sboms":
				artifact.SBOMs = string(config.SBOMLayerBuild)
				artifact.EffectiveSBOMs = []config.SBOMLayer{config.SBOMLayerBuild}
			}

			source := config.Config{Artifacts: []config.Artifact{artifact}}
			if scenario == "multiple-artifacts" {
				second := artifact
				second.Name = "second"
				source.Artifacts = append(source.Artifacts, second)
			}

			cfg := pipeline.NewConfigPlan(&source)
			release, err := pipeline.NewReleasePlan(pipeline.ReleasePlanInput{
				ConfigPlan: cfg, RepositoryVisibility: "private", Branch: "main", RefName: "v1.2.3", ReleasePublisher: "github-cli",
				ReleaseRequireAllowlistedSigner: true, ReleaseSignArtifacts: true, ReleaseSBOMs: "none", ChangelogCreator: "git-cliff",
			})
			require.NoError(t, err, "fixture must be a valid native plan, not a broken projection")
			encodeClassifiedRoute(t, &in, cfg, release)
			require.ErrorIs(t, appplan.AdmitClassifiedFiles(in), errs.ErrInvalidConfig)
		})
	}
}
