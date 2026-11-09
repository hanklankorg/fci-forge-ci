// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func cargoHistoricalWorkflow(t *testing.T) string {
	t.Helper()

	body, err := os.ReadFile("testdata/cargo-development-historical.yml")
	if err != nil {
		t.Fatal(err)
	}

	if fmt.Sprintf("%x", sha256.Sum256(body)) != cargoDevelopmentWorkflowSHA256 {
		t.Fatal("historical fixture bytes changed")
	}

	return string(body)
}

func cargoRunnerWorkflow(t *testing.T) string {
	t.Helper()

	body, err := developmentRunnerWorkflow(cargoHistoricalWorkflow(t), "Check: development container", "CI_RUNNER_LABEL_CONTAINER")
	if err != nil {
		t.Fatal(err)
	}

	return body
}

func TestCargoDevelopmentWorkflow(t *testing.T) {
	body := cargoRunnerWorkflow(t)
	for _, enforce := range []bool{false, true} {
		t.Run(strconv.FormatBool(enforce), func(t *testing.T) {
			got, err := cargoDevelopmentWorkflow(body, enforce)
			if err != nil {
				t.Fatal(err)
			}

			var workflow struct {
				Jobs map[string]struct {
					Steps []struct {
						ID  string            `yaml:"id"`
						Run string            `yaml:"run"`
						Env map[string]string `yaml:"env"`
					} `yaml:"steps"`
				} `yaml:"jobs"`
			}
			if err = yaml.Unmarshal([]byte(got), &workflow); err != nil {
				t.Fatal(err)
			}

			steps := workflow.Jobs["development"].Steps
			if len(steps) != 8 || steps[6].ID != "image" || steps[7].Env["DIGEST"] != "${{ steps.image.outputs.digest }}" {
				t.Fatal("step/output contract changed")
			}

			publication := steps[6]
			for _, fragment := range []string{
				fmt.Sprintf("forge-ci --enforce-vulnerabilities=%t container build-push-oci-image", enforce),
				`--tag "dev-${DEVELOPMENT_SHA}"`, `--revision "$DEVELOPMENT_SHA"`,
				`--containerfile Containerfile`, `--context .`, `--builds-json '[{"platform":"linux/amd64"}]'`,
				`--scan-output-template 'dist/trivy-results-runtime-{arch}.json'`,
				`--auth-file "$REGISTRY_AUTH_FILE"`, `--server-url "$DEVELOPMENT_SERVER_URL"`,
				`--repository "$DEVELOPMENT_REPOSITORY"`, `--title hello-cargo-container`,
			} {
				if !strings.Contains(publication.Run, fragment) {
					t.Errorf("missing native argument %s", fragment)
				}
			}

			if publication.Env["REGISTRY_AUTH_FILE"] != "${{ steps.registry-login.outputs.auth-file }}" || strings.Contains(publication.Run, "${{") {
				t.Fatal("unsafe or unbound expression")
			}

			start := strings.Index(got, "        env:\n          DEVELOPMENT_SHA:")
			end := strings.Index(got, "      - name: Export bounded native acceptance artifact")

			if start < 0 || end < start || got[:start]+cargoLegacyPublication+got[end:] != body {
				t.Fatal("changed bytes outside the known publication step")
			}
		})
	}
}

func TestCargoDevelopmentWorkflowRefusesUnknown(t *testing.T) {
	body := cargoRunnerWorkflow(t)
	for _, changed := range []string{
		cargoHistoricalWorkflow(t),
		strings.Replace(body, "build-release-image@", "unknown-action@", 1),
		strings.Replace(body, "role: runtime", "role: base", 1),
		strings.Replace(body, "linux/amd64", "linux/arm64", 1),
		strings.Replace(body, "cargo test --locked", "true", 1),
		strings.Replace(body, "id: image", "id: other", 1),
		strings.Replace(body, "mise-locked: 'true'", "mise-locked: 'false'", 1),
		body + "# changed\n",
	} {
		if got, err := cargoDevelopmentWorkflow(changed, false); err == nil || got != "" {
			t.Fatal("unknown contract accepted")
		}
	}
}

func TestCargoDevelopmentPublicationArgv(t *testing.T) {
	body, err := cargoDevelopmentWorkflow(cargoRunnerWorkflow(t), true)
	if err != nil {
		t.Fatal(err)
	}

	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}

	if err = yaml.Unmarshal([]byte(body), &workflow); err != nil {
		t.Fatal(err)
	}

	// Only an inert argv recorder executes. No CLI, builder, registry or scanner
	// is invoked here. A shell metacharacter in an expression value stays data.
	dir := t.TempDir()
	stageTestWrite(t, dir, "forge-ci", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGV_RECEIPT\"\n", 0755)

	sha := "abcdef;$(exit 99)"
	auth := filepath.Join(dir, "auth file.json")
	cmd := exec.CommandContext(t.Context(), "bash", "-c", workflow.Jobs["development"].Steps[6].Run)

	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "ARGV_RECEIPT="+filepath.Join(dir, "argv"),
		"DEVELOPMENT_SHA="+sha, "REGISTRY_AUTH_FILE="+auth,
		"DEVELOPMENT_SERVER_URL=https://forge.invalid", "DEVELOPMENT_REPOSITORY=Owner/Repo")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("inert publication script failed: %s %v", output, err)
	}

	receipt, err := os.ReadFile(filepath.Join(dir, "argv"))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"--enforce-vulnerabilities=true", "container", "build-push-oci-image",
		"--tag", "dev-" + sha, "--revision", sha, "--containerfile", "Containerfile", "--context", ".",
		"--builds-json", `[{"platform":"linux/amd64"}]`, "--scan-output-template", "dist/trivy-results-runtime-{arch}.json",
		"--title", "hello-cargo-container", "--auth-file", auth,
		"--server-url", "https://forge.invalid", "--repository", "Owner/Repo"}
	if got := strings.Split(strings.TrimSuffix(string(receipt), "\n"), "\n"); !reflect.DeepEqual(got, want) {
		t.Fatalf("native argv differs: got=%q want=%q", got, want)
	}
}

func TestCargoDevelopmentCurrentLayerContract(t *testing.T) {
	layer := sampleLayer()
	layer.admin = testWorkflowAdmin(t)
	layer.Commit = layer.admin.commit
	bind := func(body string) string {
		body = strings.ReplaceAll(body, "https://codefloe.com/itiquette/forge-ci/", layer.actionBase())

		return strings.ReplaceAll(body, "ea604df3f4746d9effe2ddd9d9574de0b0c2ae8c", layer.Commit)
	}

	legacy := cargoRunnerWorkflow(t)
	for _, body := range []string{legacy, strings.Replace(legacy, "build-release-image@", "unknown-action@", 1)} {
		if _, err := layer.renderAdmin(bind(body)); err == nil {
			t.Fatal("missing action validation was bypassed")
		}
	}

	generated, err := cargoDevelopmentWorkflow(legacy, true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = layer.renderAdmin(bind(generated)); err != nil {
		t.Fatalf("generated wrapper is incompatible with committed layer declarations: %v", err)
	}

	// Independently check every remaining action and its input names against the
	// committed tree, not files from the mutable checkout or a synthetic action.
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string         `yaml:"uses"`
				With map[string]any `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err = yaml.Unmarshal([]byte(generated), &workflow); err != nil {
		t.Fatal(err)
	}

	for _, step := range workflow.Jobs["development"].Steps {
		if step.Uses == "" {
			continue
		}

		_, actionPath, _ := strings.Cut(step.Uses, "/.forgejo/")
		path, _, _ := strings.Cut(actionPath, "@")

		declaration, err := layer.admin.read(".forgejo/" + path + "/action.yml")
		if err != nil {
			t.Fatal(err)
		}

		var action struct {
			Inputs map[string]any `yaml:"inputs"`
		}

		if err = yaml.Unmarshal(declaration, &action); err != nil {
			t.Fatal(err)
		}

		for input := range step.With {
			if _, ok := action.Inputs[input]; !ok {
				t.Errorf("undeclared input %s for %s", input, step.Uses)
			}
		}
	}
}

func TestCargoDevelopmentFixturePreservesSource(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, git := stageTestRepo(t, format)
			body := cargoHistoricalWorkflow(t)
			stageTestWrite(t, root, ".forgejo/workflows/development.yml", body, 0755)
			stageTestWrite(t, root, "Containerfile", "FROM unchanged\n", 0644)
			stageTestWrite(t, root, "Cargo.lock", "# unchanged\n", 0644)
			stageTestCommit(t, git, root)

			source, err := openConsumer(t.Context(), git, root, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			runner, err := developmentRunnerFixture(t.Context(), source, "Check: development container", "CI_RUNNER_LABEL_CONTAINER")
			if err != nil {
				t.Fatal(err)
			}

			for _, enforce := range []string{"false", "true"} {
				fixture, err := cargoDevelopmentFixture(t.Context(), runner, func(key string) string {
					if key == "FORGE_CI_LIVE_ADMIN_VULNERABILITIES" {
						return enforce
					}

					return ""
				})
				if err != nil {
					t.Fatal(err)
				}

				got, err := githubWorkflow(t.Context(), fixture, ".forgejo/workflows/development.yml")
				if err != nil || !strings.Contains(got, "--enforce-vulnerabilities="+enforce) {
					t.Fatalf("controller policy not bound: %v", err)
				}

				diff, err := git.run(t.Context(), fixture.stage, "diff-tree", "--no-commit-id", "--name-only", "-r", source.Commit, fixture.Commit)
				if err != nil || strings.TrimSpace(diff) != ".forgejo/workflows/development.yml" {
					t.Fatalf("non-workflow source changed: %s %v", diff, err)
				}

				mode, err := git.run(t.Context(), fixture.stage, "ls-tree", "--format=%(objectmode)", fixture.Commit, "--", ".forgejo/workflows/development.yml")
				if err != nil || strings.TrimSpace(mode) != "100755" {
					t.Fatal("workflow mode changed")
				}
			}

			if _, err = cargoDevelopmentFixture(t.Context(), runner, func(string) string { return "not-a-bool" }); err == nil {
				t.Fatal("invalid controller policy accepted")
			}

			original, err := githubWorkflow(t.Context(), source, ".forgejo/workflows/development.yml")
			if err != nil || original != body {
				t.Fatal("admitted source changed")
			}

			if err = git.requireClean(t.Context(), root, "."); err != nil {
				t.Fatal(err)
			}
		})
	}
}
