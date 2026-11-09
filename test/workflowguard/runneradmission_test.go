// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/domain/runners"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// Execute only the plan step's shell against an inert function, not the CLI
// or a provider. Every declared runner input must reach admission unchanged,
// through env (not workflow expression interpolation into shell source).
func TestGitHubPlansPassEveryDeclaredRunnerInput(t *testing.T) {
	t.Parallel()
	known, err := runners.Names(provider.ForgeGitHub)
	require.NoError(t, err)
	for _, tc := range []struct{ file, job, command string }{
		{"release-orchestrator.yml", "parse-config", "release"},
		{"release-snapshot-orchestrator.yml", "setup-snapshot", "snapshot-release"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			path := ".github/workflows/" + tc.file
			var workflow struct {
				Jobs map[string]struct {
					Steps []struct {
						ID  string            `yaml:"id"`
						Run string            `yaml:"run"`
						Env map[string]string `yaml:"env"`
					} `yaml:"steps"`
				} `yaml:"jobs"`
			}
			require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, path), &workflow))
			found := 0
			for _, step := range workflow.Jobs[tc.job].Steps {
				if step.ID != "plan" {
					continue
				}
				found++
				require.NotContains(t, step.Run, "${{")
				cmd := exec.CommandContext(t.Context(), "/bin/bash", "-eu", "-c", "forge-ci() { printf '%s\\0' \"$@\"; };\n"+step.Run)
				cmd.Env = []string{"RELEASE_POLICY=auto"}
				want := map[string]bool{}
				for _, name := range declaredInputs(t, path) {
					if !isRunnerInput(name) {
						continue
					}
					require.Contains(t, known, name)
					envName := ""
					for key, value := range step.Env {
						if value == "${{ inputs['"+name+"'] }}" {
							require.Empty(t, envName, "duplicate input binding for %s", name)
							envName = key
						}
					}
					require.NotEmpty(t, envName, "no plan env binding for %s", name)
					// Commas, whitespace and shell metacharacters remain data.
					value := `["self-hosted", "` + name + `; $(false)"]`
					cmd.Env = append(cmd.Env, envName+"="+value)
					want[name+"="+value] = true
				}
				require.NotEmpty(t, want)
				out, err := cmd.Output()
				require.NoError(t, err)
				args := strings.Split(string(bytes.TrimSuffix(out, []byte{0})), "\x00")
				require.Equal(t, []string{"plan", tc.command}, args[:2])
				for i := 2; i < len(args); i++ {
					if args[i] != "--runner-input" {
						continue
					}
					i++
					require.Less(t, i, len(args))
					require.True(t, want[args[i]], "unexpected or duplicate runner input %s", args[i])
					delete(want, args[i])
				}
				require.Empty(t, want, "runner inputs omitted from admission")
			}
			require.Equal(t, 1, found, "expected exactly one plan step")
		})
	}
}
