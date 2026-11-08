// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package provider_test

import (
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/stretchr/testify/require"
)

func TestExactArtifactIDsRequireExplicitTrustedFacts(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"valid", "legacy", "missing-name", "missing-repo", "unsafe-repo", "missing-run", "overflow-run", "missing-head", "head-without-id", "short-head", "no-dir"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			in := provider.RunArtifactDownload{ID: "42", Name: "dist", Repository: "owner/repo", RunID: "9876", ExpectedRunHeadSHA: strings.Repeat("a", 40), Dir: "out"}

			switch scenario {
			case "legacy":
				in = provider.RunArtifactDownload{Name: "dist"}
			case "missing-name":
				in.Name = ""
			case "missing-repo":
				in.Repository = ""
			case "unsafe-repo":
				in.Repository = "owner/../repo"
			case "missing-run":
				in.RunID = ""
			case "overflow-run":
				in.RunID = "9223372036854775808"
			case "missing-head":
				in.ExpectedRunHeadSHA = ""
			case "head-without-id":
				in.ID = ""
			case "short-head":
				in.ExpectedRunHeadSHA = "abc123"
			case "no-dir":
				in.Dir = ""
			}

			artifactID, runID, err := provider.ExactArtifactDownloadIDs(in)

			switch scenario {
			case "valid":
				require.NoError(t, err)
				require.EqualValues(t, 42, artifactID)
				require.EqualValues(t, 9876, runID)
			case "legacy":
				require.NoError(t, err)
				require.Zero(t, artifactID)
				require.Zero(t, runID)
			default:
				require.Error(t, err)
			}
		})
	}
}
