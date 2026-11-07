// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"fmt"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCanonicalConsumerIntegratedWorkflowContract(t *testing.T) {
	t.Parallel()

	documents := map[string][]byte{}

	for _, entry := range reporoot.ReadDir(t, ".github/workflows") {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yml") {
			documents[entry.Name()] = reporoot.ReadFile(t, ".github/workflows/"+entry.Name())
		}
	}

	require.Contains(t, documents, "canonical-cli-bootstrap.yml", "the actual integrated dependency is mandatory")

	failures, err := internalContractViolations(documents)
	require.NoError(t, err)
	require.Empty(t, failures)
}

type canonicalConsumerStep struct {
	Name string            `yaml:"name"`
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env"`
	With map[string]any    `yaml:"with"`
}

func TestCanonicalConsumerActionOrder(t *testing.T) {
	t.Parallel()

	var action struct {
		Outputs map[string]struct {
			Value string `yaml:"value"`
		} `yaml:"outputs"`
		Runs struct {
			Steps []canonicalConsumerStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/actions/consume-canonical-cli/action.yml"), &action))
	steps := action.Runs.Steps
	require.Len(t, steps, 5)
	require.Contains(t, steps[0].Run, " prepare")
	require.Contains(t, steps[1].Run, "gh api")
	require.Equal(t, map[string]string{"GH_TOKEN": "${{ github.token }}"}, steps[1].Env)
	require.Contains(t, steps[2].Run, " admit")
	require.Equal(t, "actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c", steps[3].Uses)
	require.Equal(t, "${{ env.PRODUCER_ARTIFACT_ID }}", steps[3].With["artifact-ids"])
	require.NotContains(t, steps[3].With, "name")
	require.NotContains(t, steps[3].With, "github-token")
	require.Contains(t, steps[4].Run, " receive")
	require.Equal(t, "${{ steps.receive.outputs.verified }}", action.Outputs["verified"].Value)

	for _, step := range steps {
		require.Empty(t, step.If)
	}
}

type canonicalConsumerWorkflow struct {
	Permissions map[string]string `yaml:"permissions"`
	On          struct {
		Call struct {
			Inputs  map[string]any `yaml:"inputs"`
			Secrets map[string]any `yaml:"secrets"`
		} `yaml:"workflow_call"`
	} `yaml:"on"`
	Jobs map[string]struct {
		If          string                  `yaml:"if"`
		Needs       []string                `yaml:"needs"`
		Permissions map[string]string       `yaml:"permissions"`
		Secrets     map[string]string       `yaml:"secrets"`
		Env         map[string]string       `yaml:"env"`
		With        map[string]any          `yaml:"with"`
		Steps       []canonicalConsumerStep `yaml:"steps"`
	} `yaml:"jobs"`
}

func readCanonicalConsumerWorkflow(t *testing.T, file string) canonicalConsumerWorkflow {
	t.Helper()

	var workflow canonicalConsumerWorkflow
	require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/workflows/"+file), &workflow))

	return workflow
}

func TestCanonicalConsumerWorkflowGraph(t *testing.T) {
	t.Parallel()

	for file, jobs := range map[string][]string{
		"release-orchestrator.yml":           {"parse-config", "validate-release-tag", "release-summary"},
		"validate-release-prerequisites.yml": {"validate"},
		"release-prepare-stage.yml":          {"prepare-version-files", "version-bump", "summarize-prepare-stage"},
		"generate-changelog.yml":             {"generate-changelog"},
	} {
		workflow := readCanonicalConsumerWorkflow(t, file)
		for _, name := range jobs {
			job := workflow.Jobs[name]
			require.Equal(t, "read", job.Permissions["actions"], file+name)
			require.GreaterOrEqual(t, len(job.Steps), 4)

			offset := 0
			if file == "release-orchestrator.yml" && (name == "parse-config" || name == "validate-release-tag") {
				offset = 1

				require.Equal(t, "Require original immutable request source", job.Steps[0].Name)
				require.Empty(t, job.Steps[0].Uses)
			}

			if file == "release-prepare-stage.yml" && name != "summarize-prepare-stage" {
				offset = 1

				before := job.Steps[0]
				if name == "prepare-version-files" {
					require.Equal(t, "Require current full changelog producer before preparing version files", before.Name)
				} else {
					require.Equal(t, "Require original current-attempt preparation producers before private effects", before.Name)
				}

				require.Empty(t, before.Uses)
				require.NotContains(t, before.Run, "forge-ci")

				for _, value := range before.Env {
					require.NotContains(t, value, "secrets.")
				}
			}

			gate, checkout, receive := job.Steps[offset], job.Steps[offset+1], job.Steps[offset+2]
			require.Contains(t, gate.Run, "CANONICAL_PRODUCER_RESULT")
			require.Equal(t, "${{ env.ENGINE_COMMIT }}", checkout.With["ref"])
			require.Equal(t, "${{ env.ENGINE_REPOSITORY }}", checkout.With["repository"])
			require.Equal(t, false, checkout.With["persist-credentials"])
			require.Equal(t, "./_forge_ci_bootstrap/.github/actions/consume-canonical-cli", receive.Uses)
			require.Equal(t, "canonical-bootstrap", receive.ID)

			for _, step := range job.Steps[offset : offset+3] {
				require.Contains(t, step.If, "inputs['require-canonical-bootstrap']")
				require.NotContains(t, step.Run, "forge-ci ")

				for _, value := range step.Env {
					require.NotContains(t, value, "secrets.")
				}
			}

			for _, value := range job.Env {
				require.NotContains(t, value, "secrets.")
			}

			for _, step := range job.Steps[offset+3:] {
				if strings.Contains(step.If, "always()") && strings.Contains(step.Run, "forge-ci") {
					require.Contains(t, step.If, "steps.canonical-bootstrap.outputs.verified == 'true'")
				}

				if step.Name == "Install forge-ci binary" || step.Name == "Checkout forge-ci bootstrap installers" {
					require.Equal(t, "${{ !inputs['require-canonical-bootstrap'] }}", step.If)
				}
			}
		}
	}

	root := readCanonicalConsumerWorkflow(t, "release-orchestrator.yml")
	require.Equal(t, "${{ needs.canonical-cli.outputs.artifact-id }}", root.Jobs["parse-config"].Steps[1].Env["PRODUCER_ARTIFACT_ID"])

	for _, caller := range []string{"validate-prerequisites", "execute-prepare-stage"} {
		require.Contains(t, root.Jobs[caller].Needs, "canonical-cli")
		require.Equal(t, "${{ needs.canonical-cli.result }}", root.Jobs[caller].With["canonical-producer-result"])
	}
}

func TestCanonicalConsumerNativeCommandsSecretsAndReadCaps(t *testing.T) {
	t.Parallel()

	allowed := map[string][]string{
		"release-orchestrator.yml":           {"parse-config", "validate-prerequisites", "validate-release-tag", "release-summary"},
		"validate-release-prerequisites.yml": {"validate"}, "generate-changelog.yml": {"generate-changelog"},
		"release-prepare-stage.yml": {"generate-full-changelog", "generate-minimal-changelog", "summarize-prepare-stage"},
		"release-create-github.yml": {"generate-changelog"}, "version-bump.yml": {"generate-full-changelog", "generate-minimal-changelog"},
	}
	for file, readJobs := range allowed {
		current := readCanonicalConsumerWorkflow(t, file)
		cmd := exec.CommandContext(t.Context(), "git", "show", "4943724be6b9ac5088a517c35cd87b40b5665b27:.github/workflows/"+file)
		cmd.Dir = reporoot.Path(t)
		body, err := cmd.Output()
		require.NoError(t, err)

		var original canonicalConsumerWorkflow
		require.NoError(t, yaml.Unmarshal(body, &original))
		applyLateRequestBindingDelta(file, &original)
		require.Equal(t, original.Permissions, current.Permissions, "no workflow-global expansion")
		require.Equal(t, original.On.Call.Secrets, current.On.Call.Secrets)

		for key, value := range original.On.Call.Inputs {
			require.Equal(t, value, current.On.Call.Inputs[key], file+key)
		}

		for name, before := range original.Jobs {
			after := current.Jobs[name]
			require.Equal(t, before.Env, after.Env, "native job environment is unchanged")

			if file == "release-orchestrator.yml" && (name == "parse-config" || name == "validate-prerequisites") {
				require.Contains(t, after.If, "!inputs['require-canonical-bootstrap'] || needs.canonical-cli.result == 'success'")
			} else {
				require.Equal(t, before.If, after.If, file+name)
			}

			require.Equal(t, before.Secrets, after.Secrets, file+name)

			oldPermissions, newPermissions := before.Permissions, after.Permissions
			if len(oldPermissions) == 0 {
				oldPermissions = original.Permissions
			}

			if len(newPermissions) == 0 {
				newPermissions = current.Permissions
			}

			for key, value := range newPermissions {
				if key == "actions" && value == "read" && oldPermissions[key] != "read" {
					require.Contains(t, readJobs, name)

					continue
				}

				require.Equal(t, oldPermissions[key], value, file+name+key)
			}

			for key, value := range oldPermissions {
				require.Equal(t, value, newPermissions[key])
			}

			for _, step := range before.Steps {
				// Intentional security correction after the bootstrap baseline:
				// validation must not import or clean up a private signing key.
				// All other native commands and secret declarations stay pinned.
				if file == "validate-release-prerequisites.yml" && name == "validate" &&
					(step.Run == "forge-ci release gpg import" || step.Run == "forge-ci release gpg cleanup") {
					for _, candidate := range after.Steps {
						require.NotEqual(t, step.Run, candidate.Run)
					}

					continue
				}

				found := false

				for _, candidate := range after.Steps {
					if candidate.Name == step.Name {
						if candidate.If != step.If {
							if step.Name == "Install forge-ci binary" || step.Name == "Checkout forge-ci bootstrap installers" {
								require.Empty(t, step.If)
								require.Equal(t, "${{ !inputs['require-canonical-bootstrap'] }}", candidate.If)
							} else {
								require.Contains(t, step.If, "always()")
								require.Contains(t, candidate.If, "!inputs['require-canonical-bootstrap'] || steps.canonical-bootstrap.outputs.verified == 'true'")
							}
						}

						require.Equal(t, step.Run, candidate.Run)
						require.Equal(t, step.Env, candidate.Env)
						require.Equal(t, step.With, candidate.With)

						found = true

						break
					}
				}

				require.True(t, found, file+name+step.Name)
			}
		}
	}
}

// Keep the bootstrap transplant's historical comparison, with only the native
// late-authorization delta admitted. Dedicated late-boundary guards assert the
// replacement checks, their original-Q binding and ordering before private use.
func applyLateRequestBindingDelta(file string, original *canonicalConsumerWorkflow) {
	switch file {
	case "release-orchestrator.yml":
		original.Jobs["create-release"].Secrets["RELEASE_GPG_PUBLIC_KEY"] = "${{ secrets.RELEASE_GPG_PUBLIC_KEY }}"

		for _, step := range original.Jobs["validate-release-tag"].Steps {
			if step.Name == "Validate remote final tag object and commit" {
				step.Env["RELEASE_REQUEST_SHA"] = "${{ github.sha }}"
				step.Env["RELEASE_REQUEST_IS_RELEASE"] = "${{ !fromJson(needs.parse-config.outputs['prepare-stage-plan-json']).targets.version_bump.runs }}"
			}
		}
	case "release-create-github.yml":
		original.On.Call.Secrets["RELEASE_GPG_PUBLIC_KEY"] = map[string]any{
			"description": "Public verification material only, for final-tag signature checks", "required": false,
		}
		publisher := original.Jobs["create-release"]
		publisher.Steps = slices.DeleteFunc(publisher.Steps, func(step canonicalConsumerStep) bool {
			return step.Name == "Checkout exact release source" || step.Name == "Revalidate release tag before publication"
		})
		original.Jobs["create-release"] = publisher
	}
}

func TestCanonicalConsumerContextRefusesBeforeInertEffect(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"release-orchestrator.yml", "validate-release-prerequisites.yml", "release-prepare-stage.yml", "generate-changelog.yml"} {
		workflow := readCanonicalConsumerWorkflow(t, file)

		var gate string

		for _, job := range workflow.Jobs {
			if len(job.Steps) > 0 && job.Steps[0].Name == "Require original canonical success and approved identity" {
				gate = job.Steps[0].Run

				break
			}
		}

		require.NotEmpty(t, gate)

		for field, values := range map[string][]string{"": {""}, "CANONICAL_PRODUCER_RESULT": {"", "failure", "skipped", "cancelled"}, "PRODUCER_RUN_ID": {"", "122"}, "PRODUCER_RUN_ATTEMPT": {"", "1"}, "PRODUCER_ARTIFACT_ID": {"", "0", "$(touch forbidden)"}, "PRODUCER_ARTIFACT_NAME": {"foreign"}, "PRODUCER_ENGINE_COMMIT": {strings.Repeat("b", 40)}, "ENGINE_COMMIT": {"short"}, "ENGINE_REPOSITORY": {""}, "RUNTIME_IMAGE": {"sdk:latest"}} {
			for _, value := range values {
				t.Run(file+"/"+field+"/"+value, func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()

					env := canonicalConsumerContextEnv(dir)
					if field != "" {
						env[field] = value
					}

					cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", gate+"\nprintf private > effect\n")

					cmd.Dir = dir
					for key, value := range env {
						cmd.Env = append(cmd.Env, key+"="+value)
					}

					out, err := cmd.CombinedOutput()
					if field == "" {
						require.NoError(t, err, string(out))
						require.FileExists(t, filepath.Join(dir, "effect"))
					} else {
						require.Error(t, err)
						require.NoFileExists(t, filepath.Join(dir, "effect"))
						require.NoFileExists(t, filepath.Join(dir, "environment"))
					}

					require.NoFileExists(t, filepath.Join(dir, "forbidden"))
				})
			}
		}
	}
}

func canonicalConsumerContextEnv(dir string) map[string]string {
	commit := strings.Repeat("a", 40)

	return map[string]string{
		"GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "2", "GITHUB_SERVER_URL": "https://approved.invalid",
		"ENGINE_ORIGIN": "https://approved.invalid/approved/engine", "GITHUB_ENV": filepath.Join(dir, "environment"),
		"CANONICAL_PRODUCER_RESULT": "success", "PRODUCER_RUN_ID": "123", "PRODUCER_RUN_ATTEMPT": "2",
		"PRODUCER_ARTIFACT_ID": "456", "PRODUCER_ARTIFACT_NAME": "forge-ci-canonical-linux-amd64-123-2-" + commit,
		"ENGINE_REPOSITORY": "approved/engine", "PRODUCER_ENGINE_REPOSITORY": "approved/engine",
		"ENGINE_COMMIT": commit, "PRODUCER_ENGINE_COMMIT": commit,
		"RUNTIME_IMAGE": "registry.invalid/tools@sha256:" + strings.Repeat("c", 64),
	}
}

func TestCanonicalConsumerTransitiveModeComposition(t *testing.T) {
	t.Parallel()
	steps := readCanonicalConsumerWorkflow(t, "release-prepare-stage.yml").Jobs["version-bump"].Steps
	require.Equal(t, "Require original current-attempt preparation producers before private effects", steps[0].Name)
	require.Equal(t, "Require original canonical success and approved identity", steps[1].Name)

	for _, current := range []bool{false, true} {
		for _, canonical := range []bool{false, true} {
			for _, scenario := range []string{"current", "stale-preparation", "stale-canonical"} {
				t.Run(fmt.Sprintf("current=%t/canonical=%t/%s", current, canonical, scenario), func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()

					env := validTransitivePrepareEnv()
					maps.Copy(env, canonicalConsumerContextEnv(dir))

					if scenario == "stale-preparation" {
						env["FULL_RUN_ATTEMPT"] = "1"
					}

					if scenario == "stale-canonical" {
						env["PRODUCER_RUN_ATTEMPT"] = "1"
					}

					body := ""
					if current {
						body += steps[0].Run + "\n"
					}

					if canonical {
						body += steps[1].Run + "\n"
					}

					cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", body+"printf admitted > effect\n")

					cmd.Dir = dir
					for key, value := range env {
						cmd.Env = append(cmd.Env, key+"="+value)
					}

					out, err := cmd.CombinedOutput()

					refused := current && scenario == "stale-preparation" || canonical && scenario == "stale-canonical"
					if refused {
						require.Error(t, err)
						require.NoFileExists(t, filepath.Join(dir, "effect"))
						require.NoFileExists(t, filepath.Join(dir, "environment"))
					} else {
						require.NoError(t, err, string(out))
						require.FileExists(t, filepath.Join(dir, "effect"))

						if !canonical {
							require.NoFileExists(t, filepath.Join(dir, "environment"))
						}
					}
				})
			}
		}
	}
}
