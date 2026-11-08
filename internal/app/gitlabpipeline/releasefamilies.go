// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package gitlabpipeline

import (
	"fmt"
	"slices"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
)

// ReleaseFamilies are the GitLab release child's plan-driven jobs. The child
// pipeline is templates/release-jobs.yml, which defines every job body, as
// a hidden job per family, beside this generated file, which only names the
// jobs the plan runs: one "Prepare: version <family>" per toolchain family
// whose projects the plan bumps with their native tool, and one
// "Build: <family>" per family the plan builds. The file carries no policy
// value and no consumer string, so what a job may do stays in the pinned
// template.
type ReleaseFamilies struct {
	// Version are the families whose native tools prepare the version bump.
	Version []string
	// Build are the families whose build jobs run the release build commands.
	Build []string
}

// releaseFamilyOrder is the order families are listed and their jobs named.
var releaseFamilyOrder = []string{familyJava, familyNode, familyGo, familyRust, familyAndroid}

// nativeBumpFamilies maps the project types with a native version tool to
// the family whose runtime image carries it. The other types' version files
// are written as text by the engine in the release commit's job.
var nativeBumpFamilies = map[projecttype.Type]string{
	projecttype.Maven: familyJava,
	projecttype.NPM:   familyNode,
	projecttype.Cargo: familyRust,
}

// NativeBumpProjectTypes are the project types whose native version tool a
// family's version job runs, in a stable order.
func NativeBumpProjectTypes(family string) []projecttype.Type {
	var kinds []projecttype.Type

	for _, kind := range []projecttype.Type{projecttype.Maven, projecttype.NPM, projecttype.Cargo} {
		if nativeBumpFamilies[kind] == family {
			kinds = append(kinds, kind)
		}
	}

	return kinds
}

// PlanReleaseFamilies selects the release child's families from the plan and
// refuses one the policy gives no runtime image, or a target no GitLab
// runner builds.
func PlanReleaseFamilies(prepare pipeline.ReleasePrepareStagePlan, build pipeline.ReleaseBuildStagePlan, images RuntimeImages) (ReleaseFamilies, error) {
	if targetPlanned(build.Targets.XcodeIOS) {
		return ReleaseFamilies{}, fmt.Errorf("the GitLab release has no macOS runner for the planned xcode-ios build: %w", errs.ErrUnsupported)
	}

	version, buildSet := map[string]bool{}, map[string]bool{}

	if prepare.Targets.VersionBump.Runs {
		for _, item := range prepare.Targets.VersionBump.Items {
			if family, native := nativeBumpFamilies[item.ProjectType]; native {
				version[family] = true
			}
		}
	}

	imageOf := map[string]string{}

	for _, target := range buildFamilies(build.Targets, images, "") {
		if err := validateTargetContract(target.component, target.target); err != nil {
			return ReleaseFamilies{}, err
		}

		imageOf[target.family] = target.image
		if target.target.Runs {
			buildSet[target.family] = true
		}
	}

	out := ReleaseFamilies{}

	for _, family := range releaseFamilyOrder {
		if (version[family] || buildSet[family]) && strings.TrimSpace(imageOf[family]) == "" {
			return ReleaseFamilies{}, fmt.Errorf("the plan has %s projects, which need the runtime-image-%s input: %w", family, family, errs.ErrUsage)
		}

		if version[family] {
			out.Version = append(out.Version, family)
		}

		if buildSet[family] {
			out.Build = append(out.Build, family)
		}
	}

	return out, nil
}

// Jobs renders the generated part of the release child: each job extends its
// family's hidden job of templates/release-jobs.yml. Without a family the file
// holds one hidden key, since GitLab refuses an empty configuration file.
func (f ReleaseFamilies) Jobs() map[string]any {
	jobs := map[string]any{}

	for _, family := range f.Version {
		jobs["Prepare: version "+family] = map[string]any{keyExtends: ".forge-ci-release-version-" + family}
	}

	for _, family := range f.Build {
		jobs["Build: "+family] = map[string]any{keyExtends: ".forge-ci-release-build-" + family}
	}

	if len(jobs) == 0 {
		jobs[".forge-ci-release-families"] = map[string]any{}
	}

	return jobs
}

// ReleaseFamilyArtifacts are the names of the artifacts a family's build job
// builds, in plan order.
func ReleaseFamilyArtifacts(build pipeline.ReleaseBuildStagePlan, family string) ([]string, error) {
	if !slices.Contains(releaseFamilyOrder, family) {
		return nil, fmt.Errorf("unknown release build family %q: %w", family, errs.ErrUsage)
	}

	var names []string

	for _, target := range buildFamilies(build.Targets, RuntimeImages{}, "") {
		if target.family != family || !target.target.Runs {
			continue
		}

		for _, item := range target.target.Items {
			names = append(names, item.Name)
		}
	}

	return names, nil
}
