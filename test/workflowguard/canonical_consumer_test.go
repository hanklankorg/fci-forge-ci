// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"fmt"
	"maps"
	"os/exec"
	"path/filepath"
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
	require.Equal(t, map[string]string{"GH_TOKEN": "${{ github.token }}", "TOOL_PROFILE": "${{ inputs.tool-profile }}"}, steps[1].Env)
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

	// Container callees keep the opt-in: their image carries a CLI, and the
	// classified route is what requires the canonical one there.
	optIn := map[string]bool{"validate-release-prerequisites.yml": true, "generate-changelog.yml": true}

	for file, jobs := range map[string][]string{
		"release-orchestrator.yml":           {"parse-config", "validate-release-tag", "release-summary"},
		"validate-release-prerequisites.yml": {"validate"},
		"release-prepare-stage.yml":          {"prepare-version-files", "version-bump", "summarize-prepare-stage"},
		"generate-changelog.yml":             {"generate-changelog"},
		"release-create-github.yml":          {"sign-release-gpg", "create-release"},
		"publish-container.yml":              {"build-arch"},
		"snapshot-images.yml":                {"build"},
		"release-snapshot-orchestrator.yml":  {"setup-snapshot", "snapshot-release-summary"},
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

			if file == "release-orchestrator.yml" && name == "parse-config" {
				offset = 2

				require.Equal(t, "Admit protected release route before consumer checkout", job.Steps[1].Name)
				require.NotContains(t, job.Steps[1].Run, "forge-ci")
			}

			if file == "publish-container.yml" {
				offset = 1

				require.Equal(t, "Harden GitHub runner", job.Steps[0].Name)
			}

			if file == "release-create-github.yml" {
				offset = 2

				require.Equal(t, "Require original immutable request source", job.Steps[0].Name)
				require.Empty(t, job.Steps[0].Uses)
				require.Equal(t, "Harden GitHub runner", job.Steps[1].Name)
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
				if optIn[file] {
					require.Equal(t, "${{ inputs['require-canonical-bootstrap'] }}", step.If)
				} else {
					require.Empty(t, step.If, "%s/%s consumes the canonical CLI unconditionally", file, name)
				}

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

				require.NotEqual(t, "Install forge-ci binary", step.Name, "no host installer beside a canonical receiver")
				require.NotContains(t, step.Run, "install-forge-ci.sh")
			}
		}
	}

	root := readCanonicalConsumerWorkflow(t, "release-orchestrator.yml")
	require.Equal(t, "${{ needs.canonical-cli.outputs.artifact-id }}", root.Jobs["parse-config"].Steps[2].Env["PRODUCER_ARTIFACT_ID"])

	for _, caller := range []string{"validate-prerequisites", "execute-prepare-stage", "execute-publish-stage", "create-release"} {
		require.Contains(t, root.Jobs[caller].Needs, "canonical-cli")
		require.Equal(t, "${{ needs.canonical-cli.result }}", root.Jobs[caller].With["canonical-producer-result"])
	}

	for _, input := range []string{"runtime-image", "canonical-builder-image", "forge-ci-binary-ref"} {
		spec, _ := root.On.Call.Inputs[input].(map[string]any)
		require.Equal(t, true, spec["required"], "%s is an administrator pin the route admits, not a resolved default", input)
	}

	// The snapshot route admits the same pins and forwards the delivery to
	// the image workflow, whose host build legs receive it.
	snapshot := readCanonicalConsumerWorkflow(t, "release-snapshot-orchestrator.yml")
	require.Contains(t, snapshot.Jobs["execute-snapshot-images"].Needs, "canonical-cli")
	require.Equal(t, "${{ needs.canonical-cli.result }}", snapshot.Jobs["execute-snapshot-images"].With["canonical-producer-result"])
	require.Equal(t, "${{ needs.canonical-cli.outputs.artifact-id }}", snapshot.Jobs["execute-snapshot-images"].With["canonical-artifact-id"])

	for _, input := range []string{"runtime-image", "canonical-builder-image", "forge-ci-binary-ref"} {
		spec, _ := snapshot.On.Call.Inputs[input].(map[string]any)
		require.Equal(t, true, spec["required"], "snapshot route: %s is an administrator pin", input)
	}
}

func TestReleaseAcquisitionSeparatesSourceFromProducerInventory(t *testing.T) {
	t.Parallel()
	workflow := readCanonicalConsumerWorkflow(t, "release-create-github.yml")
	steps := workflow.Jobs["assemble-release"].Steps

	indices := map[string]int{}
	for index, step := range steps {
		indices[step.Name] = index
		require.NotEqual(t, "forge-ci release attachments plan", step.Run, "no source-resident implicit binary attachments")
	}

	acquire := steps[indices["Download planned release artifacts"]]
	require.Equal(t, "acquisition", acquire.ID)
	require.Equal(t, "forge-ci release download-artifacts --isolated", acquire.Run)

	for _, name := range []string{"Generate CISA-layer SBOMs", "Assemble release files"} {
		require.Greater(t, indices[name], indices[acquire.Name])
		require.Equal(t, "${{ steps.acquisition.outputs.acquisition-dir }}", steps[indices[name]].Env["ACQUISITION_DIR"])
	}

	require.Equal(t, "${{ inputs['attach-artifacts'] }}", steps[indices["Assemble release files"]].Env["ATTACH_ARTIFACTS"])
	// Changelog and git metadata remain source-owned, never producer-inventory hints.
	require.Equal(t, "forge-ci artifact download --name release-changelog --dir .", steps[indices["Download Release Changelog"]].Run)
	require.Contains(t, steps[indices["Set deterministic release epoch"]].Run, "git rev-parse")
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
		"PRODUCER_ARTIFACT_ID": "456", "PRODUCER_ARTIFACT_NAME": "forge-ci-canonical-linux-123-2-" + commit,
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
