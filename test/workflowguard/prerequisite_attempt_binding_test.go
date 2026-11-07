// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReleasePrerequisiteBindingCapturedAtOriginalJobs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file, producer string
		reusable       bool
	}{
		{"release-orchestrator.yml", "parse-config", false},
		{"release-orchestrator.yml", "validate-release-tag", false},
		{"validate-release-prerequisites.yml", "validate", true},
		{"release-prepare-stage.yml", "version-bump", true},
	} {
		t.Run(tc.producer, func(t *testing.T) {
			var workflow struct {
				On struct {
					Call struct {
						Outputs map[string]struct{ Value string } `yaml:"outputs"`
					} `yaml:"workflow_call"`
				} `yaml:"on"`
				Jobs map[string]struct {
					Outputs map[string]string `yaml:"outputs"`
				} `yaml:"jobs"`
			}
			require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/workflows/"+tc.file), &workflow))
			for output, context := range map[string]string{
				"binding-run-id": "run_id", "binding-run-attempt": "run_attempt",
			} {
				require.Equal(t, "${{ github."+context+" }}", workflow.Jobs[tc.producer].Outputs[output])
				if tc.reusable {
					require.Equal(t, "${{ jobs."+tc.producer+".outputs."+output+" }}", workflow.On.Call.Outputs[output].Value)
				}
			}
			// A reusable call or summary must not manufacture a new label.
			for _, wrapper := range []string{"validate-prerequisites", "execute-prepare-stage", "summarize-prepare-stage"} {
				require.NotContains(t, workflow.Jobs[wrapper].Outputs, "binding-run-id")
				require.NotContains(t, workflow.Jobs[wrapper].Outputs, "binding-run-attempt")
			}
		})
	}
}

func TestPrerequisiteBindingReceiverRefusesBeforeInertEffect(t *testing.T) {
	t.Parallel()

	var template struct {
		Jobs map[string]struct {
			Needs []string `yaml:"needs"`
			Steps []struct {
				Env map[string]string `yaml:"env"`
				Run string            `yaml:"run"`
				If  string            `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	body, err := os.ReadFile("testdata/prerequisite-attempt-binding.yml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(body, &template))
	receiver := template.Jobs["receiver"]
	require.Equal(t, []string{"prerequisite"}, receiver.Needs)
	require.Len(t, receiver.Steps, 2)
	gate, effect := receiver.Steps[0], receiver.Steps[1]
	require.Equal(t, map[string]string{
		"EXPECTED_RUN": "${{ github.run_id }}", "EXPECTED_ATTEMPT": "${{ github.run_attempt }}",
		"PREREQUISITE_RESULT": "${{ needs.prerequisite.result }}",
		"CAPTURED_RUN":        "${{ needs.prerequisite.outputs.binding-run-id }}",
		"CAPTURED_ATTEMPT":    "${{ needs.prerequisite.outputs.binding-run-attempt }}",
	}, gate.Env)
	require.Empty(t, effect.Env) // No actual credentials, actions, providers or CLI.
	for _, step := range receiver.Steps {
		require.Empty(t, step.If) // Implicit success(), no always() bypass.
		require.NotContains(t, step.Run, "${{")
	}

	for _, tc := range []struct {
		name, result, run, attempt, expectedRun, expectedAttempt string
		accepted                                                 bool
	}{
		{"current", "success", "123", "2", "123", "2", true},
		{"old-run", "success", "122", "2", "123", "2", false},
		{"failed-only-rerun-retains-successful-job", "success", "123", "1", "123", "2", false},
		{"missing-run", "success", "", "2", "123", "2", false},
		{"missing-attempt", "success", "123", "", "123", "2", false},
		{"both-missing", "success", "", "", "", "", false},
		{"failed", "failure", "123", "2", "123", "2", false},
		{"cancelled", "cancelled", "123", "2", "123", "2", false},
		{"skipped", "skipped", "123", "2", "123", "2", false},
		{"missing-result", "", "123", "2", "123", "2", false},
		{"literal-not-shell", "success", "$(touch forbidden)", "2", "123", "2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			effectFile := filepath.Join(dir, "effect")
			env := []string{
				"EXPECTED_RUN=" + tc.expectedRun, "EXPECTED_ATTEMPT=" + tc.expectedAttempt,
				"PREREQUISITE_RESULT=" + tc.result, "CAPTURED_RUN=" + tc.run,
				"CAPTURED_ATTEMPT=" + tc.attempt, "EFFECT_FILE=" + effectFile,
			}
			// Execute only the inert template's shell builtins. The second step
			// models Actions' default success condition; it cannot run on refusal.
			for _, step := range receiver.Steps {
				cmd := exec.Command("/bin/sh", "-c", step.Run)
				cmd.Dir, cmd.Env = dir, env
				if err := cmd.Run(); err != nil {
					require.False(t, tc.accepted)
					break
				}
			}
			data, err := os.ReadFile(effectFile)
			if tc.accepted {
				require.NoError(t, err)
				require.Equal(t, "admitted", strings.TrimSpace(string(data)))
			} else {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			require.NoFileExists(t, filepath.Join(dir, "forbidden"))
		})
	}
}
