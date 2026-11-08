// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPublishContainerValidatesNamespaceForSelectedRegistry(t *testing.T) {
	t.Parallel()

	env := workflowStepEnv(t, "publish-container.yml", "prep", "Validate Image Namespace")
	require.Equal(t, "${{ inputs.registry }}", env["CONTAINER_REGISTRY"])
	require.Equal(t, "${{ inputs.registry }}", env["ENFORCE_NAMESPACE_ON"])
}

func workflowStepEnv(t *testing.T, workflowName, jobName, stepName string) map[string]string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(reporoot.Path(t), ".github", "workflows", workflowName))
	require.NoError(t, err)

	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(body, &workflow))

	job, ok := workflow.Jobs[jobName]
	require.Truef(t, ok, "%s has no job %q", workflowName, jobName)

	for _, step := range job.Steps {
		if step.Name == stepName {
			return step.Env
		}
	}

	require.FailNowf(t, "workflow step not found", "%s job %q has no step %q", workflowName, jobName, stepName)

	return nil
}
