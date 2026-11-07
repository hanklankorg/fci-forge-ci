// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package artifact_test

import (
	"strings"
	"testing"

	appartifact "github.com/diggsweden/forge-ci/internal/app/artifact"
	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/testutil/fakeoutputsink"
	"github.com/stretchr/testify/require"
)

func TestExactDownloadPreflightBeforeProvider(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"0", "-42", "9223372036854775808", "+42", "042", " 42", "latest"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			fake := &fakeArtifacts{}
			_, err := appartifact.Download(t.Context(), fake, nil, nil, provider.RunArtifactDownload{
				ID: id, Name: "dist", Repository: "owner/repo", RunID: "9876", ExpectedRunHeadSHA: strings.Repeat("a", 40), Dir: "not-created",
			})
			require.Error(t, err)
			require.Zero(t, fake.downloads)
		})
	}
}

func TestExactDownloadPassesTrustedExpectationAndEmitsObservedFacts(t *testing.T) {
	t.Parallel()

	in := provider.RunArtifactDownload{ID: "42", Name: "dist", Repository: "owner/repo", RunID: "9876", ExpectedRunHeadSHA: strings.Repeat("a", 40), Dir: "out"}
	observation := &provider.RunArtifactObservation{Repository: "owner/repo", RepositoryID: "123", RunID: "9876", RunHeadSHA: in.ExpectedRunHeadSHA, HeadRepositoryID: "123", HeadBranch: "main", ArchiveDigest: "sha256:" + strings.Repeat("b", 64)}
	fake := &fakeArtifacts{info: provider.RunArtifactInfo{Name: "dist", ID: "42", Observation: observation}}
	sink := fakeoutputsink.New(t)
	_, err := appartifact.Download(t.Context(), fake, sink, nil, in)
	require.NoError(t, err)
	require.Equal(t, in, fake.gotDownload)

	for key, expected := range map[string]string{
		"artifact-id": "42", "artifact-name": "dist", "artifact-repository": "owner/repo", "artifact-repository-id": "123",
		"artifact-run-id": "9876", "artifact-run-head-sha": in.ExpectedRunHeadSHA, "artifact-head-repository-id": "123",
		"artifact-head-branch": "main", "artifact-archive-digest": observation.ArchiveDigest,
	} {
		require.Equal(t, expected, sink.Single(key), key)
	}
}

func TestExactDownloadCannotReconstructUnboundBlobStore(t *testing.T) {
	t.Parallel()

	in := appartifact.BlobStoreDownload{ID: "42", Prefix: "blobs"}
	_, err := appartifact.DownloadWithBlobStore(t.Context(), nil, nil, nil, in)
	require.ErrorContains(t, err, "does not support blob-store reconstruction")
}
