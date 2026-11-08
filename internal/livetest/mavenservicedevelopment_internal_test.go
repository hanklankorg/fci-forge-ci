// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/domain/version"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"gopkg.in/yaml.v3"
)

// Exercise variations entirely with local Git objects: no Java, Maven, API or
// container execution. The admitted objects and every non-CI blob must survive.
func TestMavenServiceDevelopmentPreservesSource(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, git := stageTestRepo(t, format)
			for path, body := range map[string]string{
				".forge-ci/artifacts.yml": "containers:\n  - name: service\n    container-file: Containerfile\n    context: .\n    platforms: linux/amd64\n",
				"pom.xml":                 "<project><artifactId>different-service</artifactId><version>9.2.0</version></project>",
				"Containerfile":           "FROM example.invalid/java\nCOPY . /src\nRUN cd /src && mvn package\n",
				"src/main/App.java":       "// original business code\n",
				"development/checks.xml":  "<rules/>",
			} {
				stageTestWrite(t, root, path, body, 0644)
			}

			stageTestCommit(t, git, root)

			source, err := openConsumer(t.Context(), git, root, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			admitted := source.Commit

			recipe, err := filepath.Abs("../../test/live/demos/hello-maven-service/consumer")
			if err != nil {
				t.Fatal(err)
			}

			serviceConfig := MavenServiceDevelopmentConfig{RecipeDirectory: recipe, JavaImage: "registry.invalid/java:25@sha256:" + strings.Repeat("a", 64), Branch: "feature/other-service", ReadinessURL: "http://127.0.0.1:9090/ready"}

			fixture, err := mavenServiceDevelopmentFixture(t.Context(), source, serviceConfig)
			if err != nil {
				t.Fatal(err)
			}

			diff, err := git.run(t.Context(), fixture.stage, "diff-tree", "--no-commit-id", "--name-only", "-r", admitted, fixture.Commit)
			if err != nil {
				t.Fatal(err)
			}

			for path := range strings.SplitSeq(diff, "\n") {
				if !strings.HasPrefix(path, ".forge-ci/") && path != ".forgejo/workflows/development.yml" {
					t.Fatalf("changed source blob: %s", path)
				}
			}

			if err = git.requireClean(t.Context(), root, "."); err != nil {
				t.Fatal(err)
			}

			workflow, err := githubWorkflow(t.Context(), fixture, ".forgejo/workflows/development.yml")
			if err != nil {
				t.Fatal(err)
			}

			var parsed map[string]any
			if err = yaml.Unmarshal([]byte(workflow), &parsed); err != nil {
				t.Fatalf("invalid rendered workflow: %v", err)
			}

			var shellJobs struct {
				Jobs map[string]struct {
					Uses    string            `yaml:"uses"`
					Secrets map[string]string `yaml:"secrets"`
					Steps   []struct {
						Run string `yaml:"run"`
					} `yaml:"steps"`
				} `yaml:"jobs"`
			}
			if err = yaml.Unmarshal([]byte(workflow), &shellJobs); err != nil {
				t.Fatal(err)
			}

			// Unsigned is the fixture's policy mode, not proof that a remote
			// registry contains no signatures or attestations.
			if len(shellJobs.Jobs) != 3 || !strings.Contains(shellJobs.Jobs["snapshot"].Uses, "/snapshot-images.yml@") ||
				len(shellJobs.Jobs["check"].Steps) == 0 || len(shellJobs.Jobs["readiness"].Steps) == 0 {
				t.Fatal("snapshot fixture must have only snapshot, check and readiness roles")
			}

			for name, job := range shellJobs.Jobs {
				if name == "snapshot" {
					if len(job.Secrets) != 1 || job.Secrets["SNAPSHOT_REGISTRY_PASSWORD"] != "${{ secrets.DEVELOPMENT_REGISTRY_TOKEN }}" {
						t.Fatal("snapshot must receive only the development registry credential")
					}
				} else if len(job.Secrets) != 0 {
					t.Fatalf("unexpected secret forwarding in %s", name)
				}
			}

			if strings.Count(workflow, "secrets.") != 2 || strings.Count(workflow, "${{ secrets.DEVELOPMENT_REGISTRY_TOKEN }}") != 2 {
				t.Fatal("snapshot fixture references non-development secrets")
			}

			for name, job := range shellJobs.Jobs {
				for _, step := range job.Steps {
					command := exec.CommandContext(t.Context(), "bash", "-n")

					command.Stdin = strings.NewReader(step.Run)
					if out, err := command.CombinedOutput(); err != nil {
						t.Fatalf("invalid %s shell: %v: %s", name, err, out)
					}
				}
			}

			readiness := shellJobs.Jobs["readiness"].Steps
			requireAuthenticatedReadinessPull(t, readiness[len(readiness)-1].Run)

			decl, err := githubWorkflow(t.Context(), fixture, ".forge-ci/service-development.yml")
			if err != nil {
				t.Fatal(err)
			}

			cfg, err := config.Parse([]byte(decl))
			if err != nil {
				t.Fatal(err)
			}

			if err = config.Validate(cfg); err != nil {
				t.Fatal(err)
			}

			if err = config.Derive(cfg); err != nil {
				t.Fatal(err)
			}

			if len(cfg.Containers) != 1 || cfg.Containers[0].EnableSLSA == nil || cfg.Containers[0].EnableSLSAEffective() {
				t.Fatal("snapshot declaration must explicitly disable SLSA")
			}

			tag, err := version.ComposeSnapshotContainerTag("9.2.0", serviceConfig.Branch, fixture.Commit[:7])
			if err != nil {
				t.Fatal(err)
			}

			plan, err := snapshotimage.New(pipeline.NewConfigPlan(cfg), snapshotimage.Input{SourceSHA: fixture.Commit, SourceBranch: serviceConfig.Branch, Tag: tag, ImageRepository: "registry.invalid/owner/service", Publish: true})
			if err != nil || len(plan.Artifacts) != 0 || len(plan.Builds) != 1 || plan.Builds[0].ContainerFile != "Containerfile" {
				t.Fatalf("invalid container-first declaration or unwanted duplicate producer: %+v %v", plan, err)
			}

			for _, alter := range []func(*MavenServiceDevelopmentConfig){
				func(c *MavenServiceDevelopmentConfig) { c.Branch = "main" },
				func(c *MavenServiceDevelopmentConfig) { c.ReadinessURL = "http://external.invalid:9090/ready" },
				func(c *MavenServiceDevelopmentConfig) { c.JavaImage = "maven:latest" },
			} {
				bad := serviceConfig
				alter(&bad)

				if _, err = mavenServiceDevelopmentFixture(t.Context(), source, bad); err == nil {
					t.Fatal("accepted invalid trial infrastructure")
				}
			}
		})
	}
}

// A private consumer's snapshot image need not be anonymously readable, so the
// readiness job logs in to pull it, and the credential is gone before the
// service under test starts.
func requireAuthenticatedReadinessPull(t *testing.T, script string) {
	t.Helper()

	login := strings.Index(script, `forge-ci container login --registry "${ref%@*}"`)
	pull := strings.Index(script, `podman pull --authfile "$auth" "$ref"`)
	drop := strings.LastIndex(script, `rm -f -- "$auth"`)
	start := strings.Index(script, "podman run ")

	if login < 0 || pull < login || drop < pull || start < drop || !strings.Contains(script[login:pull], "unset REGISTRY_PASSWORD") {
		t.Fatalf("readiness must pull with a job-local credential and drop it before the service starts:\n%s", script)
	}
}

// A route whose pipeline the forge's policy owns takes the declaration alone:
// no workflow, no fixture script, and the source otherwise as admitted.
func TestServiceSnapshotDeclarationStagesOnlyTheDeclaration(t *testing.T) {
	root, git := stageTestRepo(t, "sha1")
	for path, body := range map[string]string{
		".forge-ci/artifacts.yml": "containers:\n  - name: service\n    container-file: Containerfile\n    context: .\n    platforms: linux/amd64,linux/arm64\n",
		"pom.xml":                 "<project><artifactId>service</artifactId><version>1.0.0</version></project>",
		"Containerfile":           "FROM example.invalid/java\nCOPY . /src\n",
		"src/main/App.java":       "// original business code\n",
	} {
		stageTestWrite(t, root, path, body, 0644)
	}

	stageTestCommit(t, git, root)

	source, err := openConsumer(t.Context(), git, root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	staged := source.WithServiceSnapshotDeclaration(t, GitLabDefaultArtifactsConfig)

	diff, err := git.run(t.Context(), staged.stage, "diff-tree", "--no-commit-id", "--name-only", "-r", source.Commit, staged.Commit)
	if err != nil {
		t.Fatal(err)
	}

	if diff != ServiceSnapshotDeclarationFile {
		t.Fatalf("the declaration staged %q, want %s alone", diff, ServiceSnapshotDeclarationFile)
	}

	declaration, err := githubWorkflow(t.Context(), staged, ServiceSnapshotDeclarationFile)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := parseServiceDeclaration(declaration)
	if err != nil || len(cfg.Containers) != 1 || cfg.Containers[0].Platforms != "linux/amd64" || cfg.Containers[0].EnableSLSA == nil || *cfg.Containers[0].EnableSLSA {
		t.Fatalf("declaration %+v, %v; want the one container on linux/amd64 without SLSA", cfg, err)
	}

	if err = git.requireClean(t.Context(), root, "."); err != nil {
		t.Fatal(err)
	}
}

// The Maven service development workflow builds the root Maven application
// again in its check job, so a snapshot whose producer is another family's,
// nested, or a library, all of which the snapshot itself builds, is not this
// route's: the rule is the workflow's, and the same declaration is a service
// snapshot on a forge whose policy builds the producer itself.
func TestMavenServiceDevelopmentRefusesOtherProducers(t *testing.T) {
	for name, artifact := range map[string]string{
		"npm producer":            "{name: app, project-type: npm}",
		"nested Maven producer":   "{name: app, project-type: maven, working-directory: web, build-type: application}",
		"Maven library producer":  "{name: app, project-type: maven, build-type: library}",
		"Gradle library producer": "{name: app, project-type: gradle, build-type: library}",
	} {
		t.Run(name, func(t *testing.T) {
			declaration := "artifacts: [" + artifact + "]\ncontainers: [{name: service, from: [app], container-file: Containerfile, context: ., platforms: linux/amd64}]\n"

			root, git := stageTestRepo(t, "sha1")
			for path, body := range map[string]string{
				".forge-ci/artifacts.yml": declaration,
				"pom.xml":                 "<project><artifactId>service</artifactId><version>1.0.0</version></project>",
				"web/pom.xml":             "<project><artifactId>web</artifactId><version>1.0.0</version></project>",
				"package.json":            "{\"name\": \"app\", \"version\": \"1.0.0\"}",
				"Containerfile":           "FROM example.invalid/base\nCOPY . /src\n",
			} {
				stageTestWrite(t, root, path, body, 0644)
			}

			stageTestCommit(t, git, root)

			source, err := openConsumer(t.Context(), git, root, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			recipe, err := filepath.Abs("../../test/live/demos/hello-maven-service/consumer")
			if err != nil {
				t.Fatal(err)
			}

			_, err = mavenServiceDevelopmentFixture(t.Context(), source, MavenServiceDevelopmentConfig{
				RecipeDirectory: recipe, JavaImage: "registry.invalid/java@sha256:" + strings.Repeat("a", 64),
				Branch: "development", ReadinessURL: "http://127.0.0.1:8080/health",
			})
			if !errors.Is(err, errMavenServiceProducer) {
				t.Fatalf("%s: %v, want %v", name, err, errMavenServiceProducer)
			}

			if _, _, err = serviceSnapshotDeclarationOverlay(declaration); err != nil {
				t.Fatalf("%s is not a service snapshot's declaration: %v", name, err)
			}
		})
	}
}

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

	plan := ServiceSnapshotPlan(t.Context(), t, fixture, snapshotimage.Input{
		SourceSHA: fixture.Commit, SourceBranch: "development", Tag: tag,
		ImageRepository: "registry.invalid/owner/service", SourceDateEpoch: "1", Publish: true,
	})
	if len(plan.Artifacts) != 1 || plan.Artifacts[0].Name != "hello-maven-service" || len(plan.Builds) != 1 ||
		len(plan.Builds[0].RequiredArtifacts) != 1 || plan.Builds[0].RequiredArtifacts[0].Name != "hello-maven-service" ||
		plan.Builds[0].RequiredArtifacts[0].Destination != "target" || plan.Builds[0].Arch != "amd64" || plan.Images[0].Signing != "NONE" {
		t.Fatalf("artifact-first route lost: %+v", plan)
	}

	if families := ServiceSnapshotFamilies(t.Context(), t, fixture); !slices.Equal(families, []string{snapshotimage.FamilyJava}) {
		t.Fatalf("families %v, want java", families)
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
