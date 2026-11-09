// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

func TestServiceSnapshotDeclarationPreservesBuildSettings(t *testing.T) {
	body := `artifacts:
  - name: app
    project-type: maven
    working-directory: .
    build-type: application
    sboms: none
    config:
      java-version: '25'
      maven-profile: service-checks
      settings-path: development/settings.xml
containers:
  - name: service
    from: [app]
    target: runtime
    platforms: linux/amd64,linux/arm64
    build-args:
      SERVICE_MODE: normal
    scan-severity: HIGH
`

	overlay, plan, err := serviceSnapshotDeclarationOverlay(body)
	if err != nil {
		t.Fatal(err)
	}

	if families := serviceSnapshotFamilies(plan); !slices.Equal(families, []string{snapshotimage.FamilyJava}) {
		t.Fatalf("families %v, want the Maven application's java", families)
	}

	before := serviceTestConfigPlan(t, body)
	after := serviceTestConfigPlan(t, overlay)

	if !reflect.DeepEqual(before.Artifacts, after.Artifacts) {
		t.Fatalf("producer settings changed: %s", overlay)
	}

	before.Containers.All[0].Platforms = "linux/amd64"
	before.Containers.All[0].EnableSLSA = false

	if !reflect.DeepEqual(before.Containers, after.Containers) {
		t.Fatalf("container build settings changed: %s", overlay)
	}
	// The consumer's own yamlfmt, two-space, lints the staged declaration.
	if strings.Contains(overlay, "\n    - ") || !strings.Contains(overlay, "\n  - name: service\n") {
		t.Fatalf("overlay is not two-space indented YAML:\n%s", overlay)
	}
}

// The plan is the planner's: even a declared artifact is not a producer
// unless the image depends on it, and a container-first artifact is built by
// the Containerfile. Container-first consumers must not acquire an eager
// extra build.
func TestServiceSnapshotDeclarationContainerOnlyHasNoProducer(t *testing.T) {
	for _, declaration := range []string{
		"containers: [{name: service}]\n",
		"artifacts: [{name: unused, project-type: maven, build-type: application}]\ncontainers: [{name: service}]\n",
		"artifacts: [{name: service, project-type: cargo, config: {build-mode: container-first}}]\ncontainers: [{name: service, from: [service]}]\n",
	} {
		_, plan, err := serviceSnapshotDeclarationOverlay(declaration)
		if err != nil {
			t.Fatal(err)
		}

		if families := serviceSnapshotFamilies(plan); len(families) != 0 || len(plan.Artifacts) != 0 || len(plan.Builds) != 1 || len(plan.Builds[0].RequiredArtifacts) != 0 {
			t.Fatalf("container-first acquired an artifact producer: %v %+v", families, plan)
		}
	}
}

// The snapshot builds a root producer of any family it has an image for, on
// both forges' routes.
func TestServiceSnapshotDeclarationAdmitsEveryProducerFamily(t *testing.T) {
	for family, artifact := range map[string]string{
		snapshotimage.FamilyJava: "{name: app, project-type: maven, build-type: application}",
		snapshotimage.FamilyNode: "{name: app, project-type: npm}",
		snapshotimage.FamilyGo:   "{name: app, project-type: go, config: {build-mode: artifact-first, main-package: ./cmd/app, binary-name: app}}",
		snapshotimage.FamilyRust: "{name: app, project-type: cargo, config: {build-mode: artifact-first}}",
	} {
		t.Run(family, func(t *testing.T) {
			_, plan, err := serviceSnapshotDeclarationOverlay("artifacts: [" + artifact + "]\ncontainers: [{name: service, from: [app]}]\n")
			if err != nil {
				t.Fatal(err)
			}

			if families := serviceSnapshotFamilies(plan); !slices.Equal(families, []string{family}) {
				t.Fatalf("families %v, want %s", families, family)
			}
		})
	}
}

// A refusal is the planner's where the planner has the rule, and the trial's
// acceptance scope's where the trial has it; neither falls back to another
// route.
func TestServiceSnapshotDeclarationRefusesUnsupportedInputs(t *testing.T) {
	const scope = "the trial judges one container"

	for name, test := range map[string]struct{ body, want string }{
		"missing declaration":      {"", "empty input"},
		"no container":             {"artifacts: [{name: app, project-type: maven}]", scope},
		"unknown dependency":       {"containers: [{name: service, from: [missing]}]", "missing"},
		"unknown setting":          {"containers: [{name: service, bogus: true}]", "bogus"},
		"multiple services":        {"containers: [{name: a}, {name: b}]", scope},
		"no native amd64":          {"containers: [{name: service, platforms: linux/arm64}]", scope},
		"unsupported architecture": {"containers: [{name: service, platforms: 'linux/amd64,linux/arm/v7'}]", "snapshot requires native"},
		"duplicate platform":       {"containers: [{name: service, platforms: 'linux/amd64,linux/amd64'}]", "duplicate"},
		"other context":            {"containers: [{name: service, context: subdir}]", scope},
		"other file":               {"containers: [{name: service, container-file: Otherfile}]", scope},
		"other image":              {"containers: [{name: service, image-name: other/image}]", scope},
		"two producers":            {"artifacts: [{name: a, project-type: maven, build-type: application}, {name: b, project-type: npm}]\ncontainers: [{name: service, from: [a, b]}]", scope},
		"provenance":               {"containers: [{name: service, enable-slsa: true}]", scope},
		"build secret":             {"containers: [{name: service, build-secrets: [TOKEN]}]", "build secrets and OCI provenance are unsupported"},
	} {
		t.Run(name, func(t *testing.T) {
			overlay, _, err := serviceSnapshotDeclarationOverlay(test.body)
			if err == nil || overlay != "" || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unsupported declaration: %q, %v; want a refusal naming %q", overlay, err, test.want)
			}
		})
	}
}

// The demos' own declarations decide which has a development route: the one
// ServiceSnapshotSource names, as the GitLab lifecycle and the campaign
// admission read it. A demo whose declaration the snapshot cannot build has
// none, and the reason is the planner's, not a name's.
func TestServiceSnapshotSourceOfTheDemos(t *testing.T) {
	for demo, want := range map[string]struct {
		source   string
		families []string
		refused  string
	}{
		"hello-maven-service":   {source: GitLabDefaultArtifactsConfig, families: []string{snapshotimage.FamilyJava}},
		"hello-npm-service":     {source: GitLabDefaultArtifactsConfig, families: []string{snapshotimage.FamilyNode}},
		"hello-cargo-container": {source: GitLabDedicatedReleaseImagesConfig, families: []string{}},
		// Two runtime images: the trial judges one container.
		"hello-cargo-workspace": {source: GitLabDedicatedReleaseImagesConfig, refused: "one container"},
		// Container-first with a release's extraction, which a snapshot
		// does not run: no producer.
		"hello-go-service": {source: GitLabDefaultArtifactsConfig, families: []string{}},
		// No container, so no snapshot.
		"hello-go-cli":      {source: GitLabDefaultArtifactsConfig, refused: "one container"},
		"hello-maven":       {source: GitLabDefaultArtifactsConfig, refused: "one container"},
		"hello-npm-package": {source: GitLabDefaultArtifactsConfig, refused: "one container"},
	} {
		t.Run(demo, func(t *testing.T) {
			consumer := filepath.Join(reporoot.Path(t), "test/live/fixtures/consumers", demo, "consumer")

			source, err := ServiceSnapshotSource(consumer)
			if err != nil || source != want.source {
				t.Fatalf("ServiceSnapshotSource() = %q, %v, want %q", source, err, want.source)
			}

			_, plan, err := serviceSnapshotDeclarationOverlay(string(reporoot.ReadFile(t, "test/live/fixtures/consumers/"+demo+"/consumer/"+source)))
			if want.refused != "" {
				if err == nil || !strings.Contains(err.Error(), want.refused) {
					t.Fatalf("%s of %s: %v, want the development route refused for %q", source, demo, err, want.refused)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if families := serviceSnapshotFamilies(plan); !slices.Equal(families, want.families) {
				t.Fatalf("families %v, want %v", families, want.families)
			}
		})
	}
}

// The snapshot event's scope admits what the planner plans, every image in
// the run's one repository: the multi-image demos' declarations, each image
// narrowed to native amd64, with their producers' families. It refuses an
// image of its own repository, declared provenance, an image without amd64
// and a declaration without one.
func TestSnapshotImagesDeclarationPlansEveryImage(t *testing.T) {
	for demo, families := range map[string][]string{
		"hello-cargo-workspace": {},
		"hello-monorepo":        {snapshotimage.FamilyJava, snapshotimage.FamilyNode},
	} {
		t.Run(demo, func(t *testing.T) {
			consumer := filepath.Join(reporoot.Path(t), "test/live/fixtures/consumers", demo, "consumer")

			plan, err := snapshotImagesAdmission(consumer)
			if err != nil {
				t.Fatal(err)
			}

			if len(plan.Images) != 2 || len(plan.Builds) != 2 || slices.ContainsFunc(plan.Images, func(image snapshotimage.Image) bool {
				return !slices.Equal(image.Platforms, []string{"linux/amd64"}) || image.TagSuffix == ""
			}) {
				t.Fatalf("want two suffixed native amd64 images: %+v", plan)
			}

			if got := serviceSnapshotFamilies(plan); !slices.Equal(got, families) {
				t.Fatalf("families %v, want %v", got, families)
			}
		})
	}

	for name, body := range map[string]string{
		"no container":    "artifacts: [{name: app, project-type: maven}]",
		"own repository":  "containers: [{name: a, tag-suffix: -a}, {name: b, tag-suffix: -b, image-name: other/image}]",
		"provenance":      "containers: [{name: a, tag-suffix: -a}, {name: b, tag-suffix: -b, enable-slsa: true}]",
		"no native amd64": "containers: [{name: a, tag-suffix: -a}, {name: b, tag-suffix: -b, platforms: linux/arm64}]",
	} {
		t.Run(name, func(t *testing.T) {
			if overlay, _, err := snapshotDeclarationOverlay(body, snapshotImagesScope); !errors.Is(err, errSnapshotImages) || overlay != "" {
				t.Fatalf("%q, %v; want the snapshot scope's refusal", overlay, err)
			}
		})
	}
}

// ToolchainFamily is the one mapping of project types to the images the
// snapshot's producers build in, on both forges.
func TestToolchainFamilyNamesEveryProducerFamily(t *testing.T) {
	families := map[string]bool{}

	for _, projectType := range []projecttype.Type{projecttype.Go, projecttype.Maven, projecttype.Gradle, projecttype.NPM, projecttype.Cargo} {
		family, err := snapshotimage.ToolchainFamily(projectType)
		if err != nil {
			t.Fatal(err)
		}

		families[family] = true
	}

	if len(families) != 4 {
		t.Fatalf("families %v, want go, java, node and rust", families)
	}

	if _, err := snapshotimage.ToolchainFamily(projecttype.Python); err == nil {
		t.Fatal("a project type without a snapshot producer has a family")
	}
}

func serviceTestConfigPlan(t *testing.T, body string) pipeline.ConfigPlan {
	t.Helper()

	cfg, err := config.Parse([]byte(body))
	if err == nil {
		err = config.Validate(cfg)
	}

	if err == nil {
		err = config.Derive(cfg)
	}

	if err != nil {
		t.Fatal(err)
	}

	return pipeline.NewConfigPlan(cfg)
}
