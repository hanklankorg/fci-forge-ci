// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func classifiedLeafStep(t *testing.T, job, prefix string) canonicalConsumerStep {
	t.Helper()

	for _, step := range readCanonicalConsumerWorkflow(t, "release-classified-files.yml").Jobs[job].Steps {
		if strings.HasPrefix(step.Name, prefix) {
			return step
		}
	}

	t.Fatalf("missing %s/%s", job, prefix)

	return canonicalConsumerStep{}
}

// Compare the full predicate with the actual committed native one. With mode
// native each added restriction is an identity; classified never runs a legacy
// publisher and never waives a successful/fresh upstream prerequisite.
func assertClassifiedRootCondition(t *testing.T, job, before, after string) {
	t.Helper()

	expected := before

	switch job {
	case "canonical-cli":
		expected = "${{ inputs['require-canonical-bootstrap'] || inputs['release-file-mode'] == 'classified-files' }}"
	case "parse-config", "validate-prerequisites":
		expected = strings.Replace(before, "!inputs['require-canonical-bootstrap'] ||", "(!inputs['require-canonical-bootstrap'] && inputs['release-file-mode'] == 'native') ||", 1)
		if job == "parse-config" {
			expected = strings.Replace(expected, "inputs['release-file-mode'] == 'native'", "inputs['release-file-mode'] != 'classified-files'", 1)
		}

		if job == "validate-prerequisites" {
			expected = strings.TrimSuffix(expected, " }}") + " && (inputs['release-file-mode'] != 'classified-files' || (needs.parse-config.outputs.binding-run-id == github.run_id && needs.parse-config.outputs.binding-run-attempt == github.run_attempt)) }}"
		}
	case "execute-prepare-stage":
		expected = strings.TrimSuffix(before, " }}") + " && (inputs['release-file-mode'] == 'native' || (inputs['release-file-mode'] == 'classified-files' && needs.parse-config.result == 'success' && needs.validate-prerequisites.result == 'success' && needs.canonical-cli.result == 'success' && needs.parse-config.outputs.binding-run-id == github.run_id && needs.parse-config.outputs.binding-run-attempt == github.run_attempt && needs.validate-prerequisites.outputs.binding-run-id == github.run_id && needs.validate-prerequisites.outputs.binding-run-attempt == github.run_attempt)) }}"
	case "execute-build-stage", "execute-publish-stage", "promote-dev", "promote-staging", "promote-release":
		expected = strings.Replace(before, "!cancelled()", "!cancelled() && inputs['release-file-mode'] == 'native'", 1)
	case "create-release":
		expected = strings.Replace(before, "${{ ", "${{ inputs['release-file-mode'] == 'native' && ", 1)
	default:
		t.Fatalf("unapproved native condition delta in %s", job)
	}

	require.Equal(t, expected, after, job)
}

func TestClassifiedLeafGraphAndCapabilities(t *testing.T) {
	t.Parallel()
	workflow := readCanonicalConsumerWorkflow(t, "release-classified-files.yml")
	root := readCanonicalConsumerWorkflow(t, "release-orchestrator.yml")
	require.Equal(t, "native", root.On.Call.Inputs["release-file-mode"].(map[string]any)["default"])

	for _, name := range []string{"execute-build-stage", "execute-publish-stage", "promote-dev", "promote-staging", "promote-release", "create-release"} {
		require.Contains(t, root.Jobs[name].If, "inputs['release-file-mode'] == 'native'")
	}

	require.Contains(t, root.Jobs["canonical-cli"].If, "inputs['release-file-mode'] == 'classified-files'")

	for _, name := range []string{"validate-prerequisites", "execute-prepare-stage"} {
		require.Equal(t, "${{ inputs['require-canonical-bootstrap'] || inputs['release-file-mode'] == 'classified-files' }}", root.Jobs[name].With["require-canonical-bootstrap"])
		require.Contains(t, root.Jobs[name].If, "needs.parse-config.outputs.binding-run-attempt == github.run_attempt")
	}

	require.Equal(t, "${{ inputs['release-file-mode'] == 'classified-files' }}", root.Jobs["execute-prepare-stage"].With["require-current-attempt"])
	require.Contains(t, root.Jobs["release-summary"].Needs, "classified-release")
	require.Contains(t, root.Jobs["classified-release"].Needs, "validate-prerequisites")
	require.Equal(t, "${{ toJson(needs) }}", root.Jobs["classified-release"].With["upstream-json"])

	for job, needs := range map[string][]string{"policy": nil, "producer": {"policy"}, "sign": {"policy", "producer"}, "publish": {"policy", "producer", "sign"}} {
		j := workflow.Jobs[job]
		require.Equal(t, needs, j.Needs)
		require.Equal(t, map[string]string{"contents": "read", "actions": "read"}, j.Permissions)
		require.Empty(t, j.Env)
		require.Contains(t, j.Steps[0].Run, "LOCAL_NEEDS")
		require.Contains(t, j.Steps[1].Run, "CANONICAL_PRODUCER_RESULT")
		require.Equal(t, "${{ inputs['engine-commit'] }}", j.Steps[2].With["ref"])
		require.Equal(t, "./_forge_ci_bootstrap/.github/actions/consume-canonical-cli", j.Steps[3].Uses)

		for index, step := range j.Steps {
			if index < 4 {
				require.NotContains(t, step.Run, "forge-ci ")
			}

			require.Empty(t, step.If, "no optional signing/policy bypasses")

			for _, value := range step.Env {
				if job == "policy" || job == "producer" {
					require.NotContains(t, value, "PRIVATE_KEY")
					require.NotContains(t, value, "secrets.RELEASE_TOKEN")
				}

				if strings.Contains(value, "PRIVATE_KEY") || strings.Contains(value, "secrets.RELEASE_TOKEN") {
					require.Greater(t, index, 8, "no private environment before admission")
				}
			}

			if job == "sign" || job == "publish" {
				require.NotContains(t, step.Run, "source ")
				require.NotContains(t, step.Run, "release assemble")
				require.NotContains(t, step.Run, "sbom generate")
				require.NotContains(t, step.Uses, "cache")

				if step.Uses != "" {
					require.True(t, index == 2 || index == 3 || step.ID == "upload-signed")
				}
			}
		}
	}

	for _, job := range []string{"sign", "publish"} {
		step := classifiedLeafStep(t, job, "Acquire exact")
		require.Contains(t, step.Run, "--artifact-id \"$ARTIFACT_ID\"")
		require.Contains(t, step.Run, "--expected-run-head-sha \"$GITHUB_SHA\"")
		require.NotContains(t, step.Run, "--expected-run-head-sha \"$PREPARED_SHA\"")
		require.Contains(t, step.Run, "--stage-dir \"$RUNNER_TEMP/classified-")
		require.NotContains(t, step.Run, "classified-source")
	}

	require.Contains(t, classifiedLeafStep(t, "sign", "Independently GPG").Run, "--manifest dist/release-files.json")
	cosign := classifiedLeafStep(t, "sign", "Bind Cosign")
	require.Contains(t, cosign.Run, "validate signing-key")
	require.Contains(t, cosign.Run, "release sign --method kms")
	require.Contains(t, cosign.Run, "--source-sha \"$PREPARED_SHA\"")
	require.Contains(t, classifiedLeafStep(t, "publish", "Publish only").Env["GITHUB_TOKEN"], "secrets.RELEASE_TOKEN")
}

func classifiedLeafGraph() map[string]any {
	graph := map[string]any{}

	for _, name := range []string{"parse-config", "validate-prerequisites", "execute-prepare-stage", "validate-release-tag", "policy", "producer", "sign"} {
		outputs := map[string]string{"binding-run-id": "123", "binding-run-attempt": "2"}
		for _, prefix := range []string{"full-changelog", "minimal-changelog", "version-files"} {
			outputs[prefix+"-binding-run-id"] = "123"
			outputs[prefix+"-binding-run-attempt"] = "2"
		}

		outputs["release-sha"], outputs["release-tag"], outputs["source-date-epoch"] = strings.Repeat("b", 40), "v1.2.3", "1234"
		graph[name] = map[string]any{"result": "success", "outputs": outputs}
	}

	return graph
}

func TestClassifiedLeafGraphRefusesBeforePrivateEffects(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"parse-config", "validate-prerequisites", "execute-prepare-stage", "validate-release-tag", "policy", "producer", "sign"} {
		for _, failure := range []string{"failure", "cancelled", "skipped", "stale", "missing"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				t.Parallel()

				graph := classifiedLeafGraph()

				job := graph[name].(map[string]any)
				if failure == "stale" || failure == "missing" {
					job["outputs"].(map[string]string)["binding-run-attempt"] = map[string]string{"stale": "1", "missing": ""}[failure]
				} else {
					job["result"] = failure
				}

				raw, err := json.Marshal(graph)
				require.NoError(t, err)
				dir := t.TempDir()
				cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", classifiedLeafStep(t, "publish", "Admit original").Run+"\nprintf forbidden > private-effect\n")
				cmd.Dir = dir
				cmd.Env = []string{"PATH=/usr/bin:/bin", "UPSTREAM=" + string(raw), "LOCAL_NEEDS=" + string(raw), "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=2", "GITHUB_SHA=" + strings.Repeat("a", 40), "GITHUB_EVENT_NAME=push", "GITHUB_REF=refs/tags/release-request/v1.2.3", "GITHUB_ENV=" + filepath.Join(dir, "env")}
				_, err = cmd.CombinedOutput()
				require.Error(t, err)
				require.NoFileExists(t, filepath.Join(dir, "private-effect"))
			})
		}
	}
}

func TestClassifiedLeafAcceptsOnlyOriginalTransitivePreparation(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"valid", "full-changelog", "minimal-changelog", "version-files", "no-bump"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			graph := classifiedLeafGraph()

			outputs := graph["execute-prepare-stage"].(map[string]any)["outputs"].(map[string]string)
			if scenario == "no-bump" {
				outputs["release-sha"] = strings.Repeat("a", 40)
			} else if scenario != "valid" {
				outputs[scenario+"-binding-run-attempt"] = "1"
			}

			raw, err := json.Marshal(graph)
			require.NoError(t, err)
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", classifiedLeafStep(t, "policy", "Admit original").Run+"\nprintf admitted > effect\n")
			cmd.Dir = dir
			cmd.Env = []string{"PATH=/usr/bin:/bin", "UPSTREAM=" + string(raw), "LOCAL_NEEDS={}", "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=2", "GITHUB_SHA=" + strings.Repeat("a", 40), "GITHUB_EVENT_NAME=push", "GITHUB_REF=refs/tags/release-request/v1.2.3", "GITHUB_ENV=" + filepath.Join(dir, "env")}

			out, runErr := cmd.CombinedOutput()
			if scenario == "valid" {
				require.NoError(t, runErr, string(out))
				require.FileExists(t, filepath.Join(dir, "effect"))
			} else {
				require.Error(t, runErr)
				require.NoFileExists(t, filepath.Join(dir, "effect"))
			}
		})
	}
}

func TestClassifiedLeafRootRouteAndSummaryRefusals(t *testing.T) {
	t.Parallel()

	root := readCanonicalConsumerWorkflow(t, "release-orchestrator.yml")
	steps := root.Jobs["parse-config"].Steps
	require.Equal(t, "Require original immutable request source", steps[0].Name)
	require.Empty(t, steps[0].If)
	require.Equal(t, "Admit protected release route before consumer checkout", steps[1].Name)

	for _, step := range steps[1:2] {
		for _, mode := range []string{"native", "classified-files", "", "foreign"} {
			cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", step.Run)
			cmd.Env = []string{"RELEASE_FILE_MODE=" + mode}
			err := cmd.Run()
			require.Equal(t, mode == "native" || mode == "classified-files", err == nil)
		}
	}

	for _, step := range root.Jobs["release-summary"].Steps {
		if !strings.HasPrefix(step.Name, "Refuse incomplete classified") {
			continue
		}

		for _, result := range []string{"success", "failure", "cancelled", "skipped", ""} {
			cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", step.Run)
			cmd.Env = []string{"CLASSIFIED_RESULT=" + result}
			err := cmd.Run()
			require.Equal(t, result == "success", err == nil)
		}
	}
}

func TestClassifiedLeafSourceAndAcquisitionRefusalsBeforePrivateEffects(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"request-signature", "final-tag", "source", "artifact", "digest", "path", "plan"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, "classified-source"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "forge-ci"), []byte(`#!/bin/bash
set -eu
case "$FAIL_STAGE:$*" in
  plan:*validate-classified-files*|source:*release\ validate-tag*|final-tag:*signature\ --tag\ v*|request-signature:*signature\ --tag\ release-request/*|artifact:*artifact\ download*|digest:*validate-dist*|path:*validate-dist*) printf 'refused %s\n' "$FAIL_STAGE" >&2; exit 1 ;;
esac
printf '%s\n' "$*" >>"$RUNNER_TEMP/calls"
`), 0o700))
			body := classifiedLeafStep(t, "sign", "Admit trusted").Run + "\n" + classifiedLeafStep(t, "sign", "Revalidate request").Run + "\n" + classifiedLeafStep(t, "sign", "Acquire exact").Run + "\nprintf forbidden > \"$RUNNER_TEMP/private-effect\"\n"
			cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", body)
			cmd.Dir = dir
			cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin", "RUNNER_TEMP=" + dir, "FAIL_STAGE=" + stage, "PRODUCER_FILE_ONLY=true", "RELEASE_SBOMS=none", "GO_SELECTIONS={}", "CARGO_SELECTIONS={}", `CONFIG_PLAN={"artifacts":{"all":[{"project_type":"go"}]}}`, `RELEASE_PLAN={"context":{"ref_name":"v1.2.3","release_policy":"stable"},"policy":{"create_draft_release":false}}`, "TAG=v1.2.3", "PREPARED_SHA=" + strings.Repeat("b", 40), "GITHUB_SHA=" + strings.Repeat("a", 40), "GITHUB_SERVER_URL=https://approved.invalid", "GITHUB_REPOSITORY=owner/product", "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=2", "ARTIFACT_ID=456", "PAYLOAD_DIGEST=" + strings.Repeat("c", 64)}
			out, err := cmd.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(out), "refused "+stage)
			require.NoFileExists(t, filepath.Join(dir, "private-effect"))
		})
	}
}

// The producer path guard is the first command of the key-free producer
// step. The consumers' convention is `.forge-ci/<script>`, so a leading dot
// is a legal first character; traversal and absolute paths stay refused.
func TestClassifiedProducerPathGuard(t *testing.T) {
	t.Parallel()
	guard, _, _ := strings.Cut(classifiedLeafStep(t, "producer", "Run administrator-selected").Run, "\n")
	require.Contains(t, guard, `"$PRODUCER_PATH" =~`)

	for path, admitted := range map[string]bool{
		".forge-ci/produce-classified-files.sh": true,
		"forge-ci/produce.sh":                   true,
		"produce":                               true,
		"./produce.sh":                          false,
		"../produce.sh":                         false,
		"a/./b.sh":                              false,
		"a/../b.sh":                             false,
		"/usr/bin/env":                          false,
		"-flag":                                 false,
		"a b.sh":                                false,
		"":                                      false,
	} {
		cmd := exec.CommandContext(t.Context(), testBash(t), "-peu", "-o", "pipefail", "-c", guard)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "PRODUCER_PATH=" + path}

		err := cmd.Run()
		if admitted {
			require.NoError(t, err, path)
		} else {
			require.Error(t, err, path)
		}
	}
}
