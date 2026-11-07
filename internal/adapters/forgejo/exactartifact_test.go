// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package forgejo_test

import (
	"testing"

	"github.com/diggsweden/forge-ci/internal/adapters/forgejo"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/stretchr/testify/require"
)

func TestExactArtifactIDRefusedBeforeForgejoCredentialsOrEffects(t *testing.T) {
	t.Parallel()

	adapter := &forgejo.Provider{}
	_, err := adapter.DownloadRunArtifact(t.Context(), provider.RunArtifactDownload{ID: "42"})
	require.ErrorIs(t, err, errs.ErrUnsupported)
}
