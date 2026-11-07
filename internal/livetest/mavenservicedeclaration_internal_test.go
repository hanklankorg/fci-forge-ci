// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/domain/version"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

func TestMavenServiceDevelopmentPreservesDemoArtifactRoute(t *testing.T) {
	root, git := stageTestRepo(t, "sha1")
	recipe := filepath.Join(reporoot.Path(t), "test/live/demos/hello-maven-service/consumer")

	err := fs.WalkDir(os.DirFS(recipe), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		body, err := os.ReadFile(filepath.Join(recipe, path))
		if err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		stageTestWrite(t, root, path, string(body), info.Mode().Perm())

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stageTestCommit(t, git, root)

	source, err := openConsumer(t.Context(), git, root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	fixture, err := mavenServiceDevelopmentFixture(t.Context(), source, MavenServiceDevelopmentConfig{
		RecipeDirectory: recipe, JavaImage: "registry.invalid/java@sha256:" + strings.Repeat("a", 64),
		Branch: "development", ReadinessURL: "http://127.0.0.1:8080/health",
	})
	if err != nil {
		t.Fatal(err)
	}

	changed, err := git.run(t.Context(), fixture.stage, "diff-tree", "--no-commit-id", "--name-only", "-r", source.Commit, fixture.Commit)
	if err != nil {
		t.Fatal(err)
	}

	wantChanged := ".forge-ci/service-development.yml\n.forge-ci/service-fixture.sh\n.forgejo/workflows/development.yml"
	if changed != wantChanged {
		t.Fatalf("original demo bytes or modes changed: %s", changed)
	}

	if err = git.requireClean(t.Context(), root, "."); err != nil {
		t.Fatal(err)
	}

	tag, err := version.ComposeSnapshotContainerTag("0.1.0", "development", fixture.Commit[:7])
	if err != nil {
		t.Fatal(err)
	}

	plan := MavenServiceDevelopmentPlan(t, fixture, snapshotimage.Input{
		SourceSHA: fixture.Commit, SourceBranch: "development", Tag: tag,
		ImageRepository: "registry.invalid/owner/service", SourceDateEpoch: "1", Publish: true,
	})
	if len(plan.Artifacts) != 1 || plan.Artifacts[0].Name != "hello-maven-service" || len(plan.Builds) != 1 ||
		len(plan.Builds[0].RequiredArtifacts) != 1 || plan.Builds[0].RequiredArtifacts[0].Name != "hello-maven-service" ||
		plan.Builds[0].RequiredArtifacts[0].Destination != "target" || plan.Builds[0].Arch != "amd64" || plan.Images[0].Signing != "NONE" {
		t.Fatalf("artifact-first route lost: %+v", plan)
	}

	workflow, err := githubWorkflow(t.Context(), fixture, ".forgejo/workflows/development.yml")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(workflow, "artifact-job-image-java: registry.invalid/java@sha256:") ||
		strings.Contains(workflow, "context-artifact:") || !strings.Contains(workflow, "check:\n    needs: snapshot") {
		t.Fatal("native automatic producer or separate checked-JAR job changed")
	}
}

func TestMavenServiceSnapshotDeclarationPreservesBuildSettings(t *testing.T) {
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

	overlay, err := mavenServiceSnapshotDeclaration(body)
	if err != nil {
		t.Fatal(err)
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

func TestMavenServiceSnapshotDeclarationContainerOnlyHasNoProducer(t *testing.T) {
	// Even a declared Maven artifact is not a producer unless the image depends
	// on it. Container-first consumers must not acquire an eager extra build.
	for _, artifacts := range []string{"", "artifacts: [{name: unused, project-type: maven, build-type: application}]\n"} {
		overlay, err := mavenServiceSnapshotDeclaration(artifacts + "containers: [{name: service}]\n")
		if err != nil {
			t.Fatal(err)
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

func TestMavenServiceSnapshotDeclarationRefusesUnsupportedInputs(t *testing.T) {
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
		"non Maven producer":       "artifacts: [{name: app, project-type: npm}]\ncontainers: [{name: service, from: [app]}]",
		"library producer":         "artifacts: [{name: app, project-type: maven, build-type: library}]\ncontainers: [{name: service, from: [app]}]",
	} {
		t.Run(name, func(t *testing.T) {
			if overlay, err := mavenServiceSnapshotDeclaration(body); err == nil || overlay != "" {
				t.Fatalf("unsupported declaration fell back to another route: %q, %v", overlay, err)
			}
		})
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
