// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package github

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoundedArtifactCopyNeverWritesSentinel(t *testing.T) {
	for _, body := range []string{"abcd", "abcde", "abcdef"} {
		var destination bytes.Buffer

		written, err := copyArtifactBytes(t.Context(), &destination, strings.NewReader(body), 5, true)
		if len(body) <= 5 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}

		require.Equal(t, min(len(body), 5), destination.Len())
		require.Equal(t, int64(destination.Len()), written)
	}
}

func TestBoundedArtifactCopyCancelledBeforeWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var destination bytes.Buffer

	_, err := copyArtifactBytes(ctx, &destination, strings.NewReader("bytes"), 5, true)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, destination.Len())
}
