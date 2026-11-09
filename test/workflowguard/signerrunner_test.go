// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// signingSecrets are the secrets that sign: release, package and app signing
// keys and their passphrases.
var signingSecrets = []string{
	"RELEASE_GPG_PRIVATE_KEY", "RELEASE_GPG_PASSPHRASE",
	"RELEASE_COSIGN_PRIVATE_KEY", "RELEASE_COSIGN_PASSPHRASE",
	"RELEASE_SSH_SIGNING_KEY",
	"ANDROID_KEYSTORE", "ANDROID_KEYSTORE_PASSWORD", "ANDROID_KEY_ALIAS", "ANDROID_KEY_PASSWORD",
	"IOS_SIGNING_CERTIFICATE_BASE64", "IOS_SIGNING_CERTIFICATE_PASSPHRASE", "PROVISIONING_PROFILE_BASE64",
}

// The GitHub signer role is a hosted literal: a job of a reusable workflow
// that receives a signing secret, or a GitHub OIDC token to sign keylessly with,
// names its runner as a literal label, never an input or any other
// expression, so no caller chooses the runner that holds the key.
func TestGitHubSigningJobsRunOnALiteralRunner(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(reporoot.Path(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	signing := 0

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		var workflow struct {
			On   map[string]yaml.Node `yaml:"on"`
			Jobs map[string]yaml.Node `yaml:"jobs"`
		}

		require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/workflows/"+entry.Name()), &workflow), entry.Name())

		// Only a reusable workflow has a caller who could choose its runner.
		if _, reusable := workflow.On["workflow_call"]; !reusable {
			continue
		}

		for name, node := range workflow.Jobs {
			var job struct {
				RunsOn      yaml.Node         `yaml:"runs-on"`
				Uses        string            `yaml:"uses"`
				Permissions map[string]string `yaml:"permissions"`
			}

			require.NoError(t, node.Decode(&job), "%s/%s", entry.Name(), name)

			// A reusable call has no runner; the callee's jobs are checked.
			if job.Uses != "" {
				continue
			}

			text, err := yaml.Marshal(&node)
			require.NoError(t, err)

			if !holdsSigningMaterial(string(text), job.Permissions) {
				continue
			}

			signing++

			require.Equal(t, yaml.ScalarNode, job.RunsOn.Kind, "%s/%s holds signing material; its runs-on must be one literal label", entry.Name(), name)
			require.NotContains(t, job.RunsOn.Value, "${{", "%s/%s holds signing material, so a caller must not choose its runner", entry.Name(), name)
		}
	}

	require.Positive(t, signing, "no signing job found: the guard checks nothing")
}

func holdsSigningMaterial(text string, permissions map[string]string) bool {
	if permissions["id-token"] == "write" {
		return true
	}

	for _, secret := range signingSecrets {
		if strings.Contains(text, "secrets."+secret+" ") || strings.Contains(text, "secrets."+secret+"}") ||
			strings.Contains(text, "secrets['"+secret+"']") {
			return true
		}
	}

	return false
}
