// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"path/filepath"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// A demo chooses its GitLab renderer by the configuration it carries: the
// component's default git-cliff one, or git-chglog's .chglog/.
func TestGitLabChangelogInputsFollowTheConsumersConfiguration(t *testing.T) {
	t.Parallel()

	demos := filepath.Join(reporoot.Path(t), "test", "live", "demos")

	for root, creator := range map[string]string{
		"": "git-chglog",
		filepath.Join(demos, "hello-maven", "consumer"):           "git-chglog",
		filepath.Join(demos, "hello-cargo-workspace", "consumer"): "",
	} {
		inputs, err := GitLabChangelogInputs(root)
		if err != nil {
			t.Fatal(err)
		}

		if got, _ := inputs["changelog-creator"].(string); got != creator {
			t.Errorf("%q renders with %q, want %q", root, got, creator)
		}

		if creator == "" && len(inputs) != 0 {
			t.Errorf("%q overrides the component's renderer inputs: %v", root, inputs)
		}
	}
}
