// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import "testing"

// Two cases running at once each read their own inputs, a subtest reads its
// parent's, an input a case leaves out is the environment's, and one it sets
// empty hides the environment's.
func TestInputIsTheCasesOwnAndOtherwiseTheEnvironments(t *testing.T) {
	const builder, signer, archive = "FORGE_CI_LIVE_TEST_BUILDER", "FORGE_CI_LIVE_TEST_SIGNER", "FORGE_CI_LIVE_TEST_ARCHIVE"

	t.Setenv(signer, "from-environment")
	t.Setenv(archive, "left-by-the-caller")

	for _, value := range []string{"buildah", "kaniko"} {
		t.Run(value, func(t *testing.T) {
			SetInputs(t, map[string]string{builder: value, archive: ""})
			t.Parallel()

			t.Run("phase", func(t *testing.T) {
				if got := Input(t, builder); got != value {
					t.Fatalf("builder = %q, want the case's %q", got, value)
				}

				if got := Input(t, signer); got != "from-environment" {
					t.Fatalf("signer = %q, want the environment's", got)
				}

				if got := Input(t, archive); got != "" {
					t.Fatalf("archive = %q, want the case's empty value", got)
				}
			})
		})
	}

	if got := Input(t, builder); got != "" {
		t.Fatalf("builder = %q outside a case, want none", got)
	}
}
