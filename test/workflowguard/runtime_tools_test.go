// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTrustedRuntimeToolOwners(t *testing.T) {
	t.Parallel()
	helper := string(reporoot.ReadFile(t, "scripts/ci/trusted-runtime-tools.sh"))
	require.Contains(t, helper, "export PATH=/usr/local/go/bin:/usr/local/cargo/bin:/usr/local/bin:/usr/bin:/bin\n")
	require.Contains(t, helper, "export RUSTUP_HOME=/usr/local/rustup")
	require.Contains(t, helper, "export CARGO_HOME=${HOME:?}/.cargo")
	require.NotContains(t, helper, "${PATH")
	require.NotContains(t, helper, "GITHUB_WORKSPACE")
	require.NotContains(t, helper, "TRUSTED_TOOL_PATH")

	bootstrap := string(reporoot.ReadFile(t, "scripts/ci/trusted-bootstrap.sh"))
	require.Less(t, strings.Index(bootstrap, "verify_forge_ci || exit $?"), strings.Index(bootstrap, "exec /bin/bash -p"))
	require.Contains(t, bootstrap, `"$checkout/scripts/ci/trusted-runtime-tools.sh" "$HELPERS_ENGINE_VERIFIED" "$@"`)
	require.Contains(t, bootstrap, ".engine/bin/forge-ci-linux-amd64.sha256 scripts/ci/trusted-runtime-tools.sh; do")

	var workflow struct {
		Jobs map[string]struct {
			Steps []canonicalConsumerStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/workflows/canonical-cli-bootstrap.yml"), &workflow))

	prepared := false
	found := false

	for _, step := range workflow.Jobs["canonical-receive"].Steps {
		if strings.Contains(step.Run, `canonical-cli-delivery.sh" prepare`) {
			prepared = true
		}

		if strings.Contains(step.Run, "gh api") {
			require.True(t, prepared, "the actual helper file must be admitted before metadata")
			require.Contains(t, step.Run, `/bin/bash -p "$ENGINE_CHECKOUT/scripts/ci/trusted-runtime-tools.sh" gh api`)
			require.Equal(t, "${{ github.token }}", step.Env["GH_TOKEN"])

			found = true
		}
	}

	require.True(t, found)

	var action struct {
		Runs struct {
			Steps []canonicalConsumerStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	require.NoError(t, yaml.Unmarshal(reporoot.ReadFile(t, ".github/actions/consume-canonical-cli/action.yml"), &action))
	require.Contains(t, action.Runs.Steps[0].Run, "prepare")
	require.Contains(t, action.Runs.Steps[1].Run, `/bin/bash -p "$RUNNER_TEMP/forge-ci-approved-engine/scripts/ci/trusted-runtime-tools.sh" gh api`)

	producer := classifiedLeafStep(t, "producer", "Run administrator-selected")
	require.Contains(t, producer.Run, `/usr/bin/env -i HOME="$RUNNER_TEMP/classified-product-home"`)
	require.Contains(t, producer.Run, `/bin/bash -p "$RUNNER_TEMP/forge-ci-approved-engine/scripts/ci/trusted-runtime-tools.sh" "$script"`)
	require.NotContains(t, producer.Run, "PATH=")
	require.NotContains(t, producer.Run, "RUSTUP_HOME=")
	require.NotContains(t, producer.Run, "CARGO_HOME=")
	require.NotContains(t, producer.Run, "secrets.")
}
