// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package syncguard

import (
	"testing"

	"github.com/diggsweden/forge-ci/internal/livetest"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"

	"github.com/stretchr/testify/require"
)

// TestTestingMatricesInSync asserts that the generated sections of
// docs/testing-matrices.md, the E2E variants and the demos, match what the
// tree says now: the shipped demos' declarations as the planner admits them,
// the cases files and the live-evidence overlay. A demo, a cases line or a
// mark changed without the page is a matrix that lies about coverage.
//
// On failure, run `just gen-testing-matrices` to refresh the page.
func TestTestingMatricesInSync(t *testing.T) {
	t.Parallel()

	got := reporoot.ReadFile(t, livetest.TestingMatricesPage)

	want, err := livetest.RenderTestingMatrices(reporoot.Path(t), string(got))
	require.NoError(t, err)
	require.Empty(t, generatedDifference(livetest.TestingMatricesPage, "just gen-testing-matrices", got, []byte(want)))
}
