// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/domain/version"
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

	overlay, cfg, err := serviceSnapshotDeclarationOverlay(body)
	if err != nil {
		t.Fatal(err)
	}

	if families := serviceSnapshotFamilies(cfg); !slices.Equal(families, []string{snapshotimage.FamilyJava}) {
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
}

func TestServiceSnapshotDeclarationContainerOnlyHasNoProducer(t *testing.T) {
	// Even a declared artifact is not a producer unless the image depends on
	// it, and a container-first artifact is built by the Containerfile.
	// Container-first consumers must not acquire an eager extra build.
	for _, declaration := range []string{
		"containers: [{name: service}]\n",
		"artifacts: [{name: unused, project-type: maven, build-type: application}]\ncontainers: [{name: service}]\n",
		"artifacts: [{name: service, project-type: cargo, config: {build-mode: container-first}}]\ncontainers: [{name: service, from: [service]}]\n",
	} {
		overlay, cfg, err := serviceSnapshotDeclarationOverlay(declaration)
		if err != nil {
			t.Fatal(err)
		}

		if families := serviceSnapshotFamilies(cfg); len(families) != 0 {
			t.Fatalf("container-first declaration has producer families %v", families)
		}

		tag, err := version.ComposeSnapshotContainerTag("1.0.0", "development", "aaaaaaa")
		if err != nil {
			t.Fatal(err)
		}

		plan, err := snapshotimage.New(serviceTestConfigPlan(t, overlay), snapshotimage.Input{
			SourceSHA: strings.Repeat("a", 40), SourceBranch: "development", Tag: tag,
			ImageRepository: "registry.invalid/owner/service", SourceDateEpoch: "1", Publish: true,
		})
		if err != nil || len(plan.Artifacts) != 0 || len(plan.Builds) != 1 || len(plan.Builds[0].RequiredArtifacts) != 0 {
			t.Fatalf("container-first acquired an artifact producer: %+v %v", plan, err)
		}
	}
}

// The snapshot builds a root producer of any family it has an image for; the
// Maven service development workflow's Maven-only rule is the fixture's
// (TestMavenServiceDevelopmentRefusesOtherProducers).
func TestServiceSnapshotDeclarationAdmitsEveryProducerFamily(t *testing.T) {
	for family, artifact := range map[string]string{
		snapshotimage.FamilyJava: "{name: app, project-type: maven, build-type: application}",
		snapshotimage.FamilyNode: "{name: app, project-type: npm}",
		snapshotimage.FamilyGo:   "{name: app, project-type: go, config: {build-mode: artifact-first, main-package: ./cmd/app, binary-name: app}}",
		snapshotimage.FamilyRust: "{name: app, project-type: cargo, config: {build-mode: artifact-first}}",
	} {
		t.Run(family, func(t *testing.T) {
			_, cfg, err := serviceSnapshotDeclarationOverlay("artifacts: [" + artifact + "]\ncontainers: [{name: service, from: [app]}]\n")
			if err != nil {
				t.Fatal(err)
			}

			if families := serviceSnapshotFamilies(cfg); !slices.Equal(families, []string{family}) {
				t.Fatalf("families %v, want %s", families, family)
			}
		})
	}
}

func TestServiceSnapshotDeclarationRefusesUnsupportedInputs(t *testing.T) {
	for name, body := range map[string]string{
		"missing declaration":      "",
		"no container":             "artifacts: [{name: app, project-type: maven}]",
		"unknown dependency":       "containers: [{name: service, from: [missing]}]",
		"unknown setting":          "containers: [{name: service, bogus: true}]",
		"multiple services":        "containers: [{name: a}, {name: b}]",
		"no native amd64":          "containers: [{name: service, platforms: linux/arm64}]",
		"unsupported architecture": "containers: [{name: service, platforms: 'linux/amd64,linux/arm/v7'}]",
		"other context":            "containers: [{name: service, context: subdir}]",
		"other file":               "containers: [{name: service, container-file: Otherfile}]",
		"other image":              "containers: [{name: service, image-name: other/image}]",
		"provenance":               "containers: [{name: service, enable-slsa: true}]",
		"build secret":             "containers: [{name: service, build-secrets: [TOKEN]}]",
		"extraction":               "containers: [{name: service, extract: {binary: {target: export, names: [app]}}}]",
		"two producers":            "artifacts: [{name: a, project-type: maven, build-type: application}, {name: b, project-type: npm}]\ncontainers: [{name: service, from: [a, b]}]",
		"nested producer":          "artifacts: [{name: app, project-type: npm, working-directory: web}]\ncontainers: [{name: service, from: [app]}]",
		"library producer":         "artifacts: [{name: app, project-type: maven, build-type: library}]\ncontainers: [{name: service, from: [app]}]",
		"gradle library producer":  "artifacts: [{name: app, project-type: gradle, build-type: library}]\ncontainers: [{name: service, from: [app]}]",
	} {
		t.Run(name, func(t *testing.T) {
			if overlay, _, err := serviceSnapshotDeclarationOverlay(body); err == nil || overlay != "" {
				t.Fatalf("unsupported declaration fell back to another route: %q, %v", overlay, err)
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
		refused  bool
	}{
		"hello-maven-service":   {source: GitLabDefaultArtifactsConfig, families: []string{snapshotimage.FamilyJava}},
		"hello-npm-service":     {source: GitLabDefaultArtifactsConfig, families: []string{snapshotimage.FamilyNode}},
		"hello-cargo-container": {source: GitLabDedicatedReleaseImagesConfig, families: []string{}},
		// Two runtime images: a service snapshot is of one container.
		"hello-cargo-workspace": {source: GitLabDedicatedReleaseImagesConfig, refused: true},
		// Extraction, which the snapshot planner refuses.
		"hello-go-service": {source: GitLabDefaultArtifactsConfig, refused: true},
		// No container, so no snapshot.
		"hello-go-cli":      {source: GitLabDefaultArtifactsConfig, refused: true},
		"hello-maven":       {source: GitLabDefaultArtifactsConfig, refused: true},
		"hello-npm-package": {source: GitLabDefaultArtifactsConfig, refused: true},
	} {
		t.Run(demo, func(t *testing.T) {
			consumer := filepath.Join(reporoot.Path(t), "test/live/demos", demo, "consumer")

			source, err := ServiceSnapshotSource(consumer)
			if err != nil || source != want.source {
				t.Fatalf("ServiceSnapshotSource() = %q, %v, want %q", source, err, want.source)
			}

			_, cfg, err := serviceSnapshotDeclarationOverlay(string(reporoot.ReadFile(t, "test/live/demos/"+demo+"/consumer/"+source)))
			if want.refused {
				if err == nil {
					t.Fatalf("%s of %s is a service snapshot's declaration; the demo's development route is not refused", source, demo)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if families := serviceSnapshotFamilies(cfg); !slices.Equal(families, want.families) {
				t.Fatalf("families %v, want %v", families, want.families)
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
