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

type transitivePrepareWorkflow struct {
	On struct {
		Call struct {
			Inputs map[string]struct {
				Default any `yaml:"default"`
			} `yaml:"inputs"`
			Outputs map[string]struct {
				Value string `yaml:"value"`
			} `yaml:"outputs"`
		} `yaml:"workflow_call"`
	} `yaml:"on"`
	Jobs map[string]struct {
		Outputs map[string]string `yaml:"outputs"`
		Steps   []struct {
			Name string            `yaml:"name"`
			If   string            `yaml:"if"`
			Run  string            `yaml:"run"`
			Env  map[string]string `yaml:"env"`
			With map[string]any    `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func readTransitivePrepare(t *testing.T) transitivePrepareWorkflow {
	t.Helper()

	var workflow transitivePrepareWorkflow
	require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/workflows/release-prepare-stage.yml"), &workflow))

	return workflow
}

func TestTransitivePreparationOutputsAndExactSelection(t *testing.T) {
	t.Parallel()
	workflow := readTransitivePrepare(t)
	require.Equal(t, false, workflow.On.Call.Inputs["require-current-attempt"].Default)

	for prefix, job := range map[string]string{"full-changelog": "generate-full-changelog", "minimal-changelog": "generate-minimal-changelog", "version-files": "prepare-version-files"} {
		for _, suffix := range []string{"binding-run-id", "binding-run-attempt"} {
			require.Equal(t, "${{ jobs."+job+".outputs."+suffix+" }}", workflow.On.Call.Outputs[prefix+"-"+suffix].Value)
		}
	}

	require.Equal(t, "${{ steps.upload.outputs.artifact-id }}", workflow.Jobs["prepare-version-files"].Outputs["artifact-id"])

	for _, jobName := range []string{"prepare-version-files", "version-bump"} {
		job := workflow.Jobs[jobName]
		require.Contains(t, job.Steps[0].If, "inputs['require-current-attempt']")
		require.Contains(t, job.Steps[0].Run, "CURRENT_RUN_ATTEMPT")

		exact := 0

		privateAt, lastCheck := len(job.Steps), -1
		for index, step := range job.Steps {
			for _, value := range step.Env {
				if strings.Contains(value, "secrets.") && index < privateAt {
					privateAt = index
				}
			}

			if strings.Contains(step.Name, "Download exact current") {
				exact++

				require.Contains(t, step.If, "inputs['require-current-attempt']")

				for _, flag := range []string{"--artifact-id", "--name", "--repository", "--run-id", "--expected-run-head-sha"} {
					require.Contains(t, step.Run, flag)
				}

				require.Equal(t, "${{ github.repository }}", step.Env["REPOSITORY"])
				require.Equal(t, "${{ github.run_id }}", step.Env["RUN_ID"])
				require.Equal(t, "${{ github.sha }}", step.Env["REQUEST_SHA"])
				require.Contains(t, step.Env["ARTIFACT_ID"], "needs.")
				require.Contains(t, step.Run, "test ! -e")

				lastCheck = index
			}

			if step.Name == "Apply complete version-bump plan" || step.Name == "Read Minimal Changelog" {
				lastCheck = index
			}

			if strings.Contains(step.Run, "artifact download --name") {
				require.Contains(t, step.If, "!inputs['require-current-attempt']")
			}
		}

		if jobName == "version-bump" {
			require.Equal(t, 3, exact)
			require.Less(t, lastCheck, privateAt)
		} else {
			require.Equal(t, 1, exact)
		}
	}
}

func validTransitivePrepareEnv() map[string]string {
	env := map[string]string{"CURRENT_RUN_ID": "123", "CURRENT_RUN_ATTEMPT": "2", "CONTEXT_SOURCE_SHA": strings.Repeat("a", 40), "AUTHORIZED_SOURCE_SHA": strings.Repeat("a", 40), "EXISTING_RELEASE_SHA": ""}
	for prefix, role := range map[string]string{"FULL": "full-changelog", "MINIMAL": "minimal-changelog", "PATCH": "prepared-version-bumps"} {
		env[prefix+"_RESULT"], env[prefix+"_RUN_ID"], env[prefix+"_RUN_ATTEMPT"] = "success", "123", "2"
		env[prefix+"_ARTIFACT_ID"], env[prefix+"_ARTIFACT_NAME"] = "456", role+"-123-2"
	}

	return env
}

func TestTransitivePreparationGateBeforeAnyInertPrivateEffect(t *testing.T) {
	t.Parallel()
	workflow := readTransitivePrepare(t)

	for _, job := range []string{"prepare-version-files", "version-bump"} {
		prefixes := []string{"FULL"}
		if job == "version-bump" {
			prefixes = append(prefixes, "MINIMAL", "PATCH")
		}

		cases := map[string][]string{"": {""}, "AUTHORIZED_SOURCE_SHA": {strings.Repeat("b", 40)}, "CURRENT_RUN_ATTEMPT": {""}, "CURRENT_RUN_ID": {""}}
		if job == "version-bump" {
			cases["EXISTING_RELEASE_SHA"] = []string{strings.Repeat("a", 40)}
		}

		for _, prefix := range prefixes {
			for suffix, values := range map[string][]string{"RESULT": {"skipped", "failure", "cancelled", ""}, "RUN_ID": {"122", ""}, "RUN_ATTEMPT": {"1", ""}, "ARTIFACT_ID": {"", "0", "$(touch forbidden)"}, "ARTIFACT_NAME": {"foreign-123-2", ""}} {
				cases[prefix+"_"+suffix] = values
			}
		}

		for field, values := range cases {
			for _, value := range values {
				t.Run(job+"/"+field+"/"+value, func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()

					env := validTransitivePrepareEnv()
					if field != "" {
						env[field] = value
					}

					cmd := exec.CommandContext(t.Context(), testBash(t), "-c", workflow.Jobs[job].Steps[0].Run+"\nprintf private-effect > effect\n")

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
					}

					require.NoFileExists(t, filepath.Join(dir, "forbidden"))
				})
			}
		}
	}
}

func TestTransitiveExactDownloadFailureCannotReachInertPrivateEffect(t *testing.T) {
	t.Parallel()

	workflow := readTransitivePrepare(t)
	for _, step := range workflow.Jobs["version-bump"].Steps {
		if !strings.Contains(step.Name, "Download exact current") {
			continue
		}

		t.Run(step.Name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "forge-ci"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > call\nexit 1\n"), 0o700))
			cmd := exec.CommandContext(t.Context(), testBash(t), "-c", step.Run+"\nprintf private-effect > effect\n")
			cmd.Dir = dir
			cmd.Env = []string{"PATH=" + dir, "RUNNER_TEMP=" + dir, "ARTIFACT_ID=456", "ARTIFACT_NAME=current-123-2", "REPOSITORY=fixture/repo", "RUN_ID=123", "REQUEST_SHA=" + strings.Repeat("a", 40)}
			_, err := cmd.CombinedOutput()
			require.Error(t, err)
			require.FileExists(t, filepath.Join(dir, "call"))
			require.NoFileExists(t, filepath.Join(dir, "effect"))
		})
	}
}
