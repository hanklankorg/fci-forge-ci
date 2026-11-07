// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package github_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/adapters/forgejo"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/testutil/fakegitserver"
	"github.com/stretchr/testify/require"
)

const boundedCaseBytes = int64(32 << 20)

func boundedArtifactInput(t *testing.T) provider.RunArtifactDownload {
	t.Helper()

	in := exactArtifactInput(t)
	in.ExpectedRepositoryID = "123"
	in.Limits = &provider.RunArtifactLimits{MaxArchiveBytes: boundedCaseBytes, MaxExpandedBytes: boundedCaseBytes, MaxEntries: 128}

	return in
}

func boundedArtifactMetadata(body []byte) map[string]any {
	metadata := exactArtifactMetadata(body)
	metadata["size_in_bytes"] = len(body)

	return metadata
}

func TestBoundedExactArtifactMetadataRefusesBeforeZIP(t *testing.T) {
	for _, scenario := range []string{"oversize", "missing", "zero", "negative", "foreign-created-repository"} {
		t.Run(scenario, func(t *testing.T) {
			in := boundedArtifactInput(t)
			archive := buildZip(t, map[string]string{"dist/file": "bytes"})
			metadata := boundedArtifactMetadata(archive)

			switch scenario {
			case "oversize":
				metadata["size_in_bytes"] = boundedCaseBytes + 1
			case "missing":
				delete(metadata, "size_in_bytes")
			case "zero":
				metadata["size_in_bytes"] = 0
			case "negative":
				metadata["size_in_bytes"] = -1
			case "foreign-created-repository":
				in.ExpectedRepositoryID = "456"
			}

			adapter, server := serveExactArtifact(t, metadata, archive)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)

			for _, request := range server.Requests() {
				require.NotContains(t, request.Path, "/zip")
				require.NotContains(t, request.Path, "/blob")
			}

			_, err = os.Lstat(filepath.Dir(in.Dir))
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestBoundedExactArtifactInvalidOptionsHaveNoEffects(t *testing.T) {
	for _, scenario := range []string{"no-id", "no-created-repository", "bad-created-repository", "zero-compressed", "negative-expanded", "zero-entries", "excess-compressed", "excess-expanded", "excess-entries"} {
		t.Run(scenario, func(t *testing.T) {
			in := boundedArtifactInput(t)

			switch scenario {
			case "no-id":
				in.ID = ""
			case "no-created-repository":
				in.ExpectedRepositoryID = ""
			case "bad-created-repository":
				in.ExpectedRepositoryID = "0123"
			case "zero-compressed":
				in.Limits.MaxArchiveBytes = 0
			case "negative-expanded":
				in.Limits.MaxExpandedBytes = -1
			case "zero-entries":
				in.Limits.MaxEntries = 0
			case "excess-compressed":
				in.Limits.MaxArchiveBytes = 11 << 30
			case "excess-expanded":
				in.Limits.MaxExpandedBytes = 11 << 30
			case "excess-entries":
				in.Limits.MaxEntries = 10001
			}

			adapter, server := serveExactArtifact(t, nil, nil)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)
			require.Empty(t, server.Requests())
		})
	}
}

type boundedZIPEntry struct {
	name, body string
	mode       fs.FileMode
}

func boundedZIP(t *testing.T, entries []boundedZIPEntry) []byte {
	t.Helper()

	var body bytes.Buffer

	writer := zip.NewWriter(&body)

	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		file, err := writer.CreateHeader(header)
		require.NoError(t, err)
		_, err = file.Write([]byte(entry.body))
		require.NoError(t, err)
	}

	require.NoError(t, writer.Close())

	return body.Bytes()
}

func TestBoundedExactArtifactUnsafeZIPDoesNotInstall(t *testing.T) {
	for _, scenario := range []string{"alias", "duplicate", "traversal", "absolute", "windows", "backslash", "symlink", "pipe", "later-bad-entry", "file-parent"} {
		t.Run(scenario, func(t *testing.T) {
			entries := make([]boundedZIPEntry, 1, 2)
			entries[0] = boundedZIPEntry{name: "dist/first", body: "first", mode: 0o600}
			bad := boundedZIPEntry{name: "dist/second", body: "bad", mode: 0o600}

			switch scenario {
			case "alias":
				bad.name = "dist/./alias"
			case "duplicate":
				bad.name = "dist/first"
			case "traversal":
				bad.name = "../escape"
			case "absolute":
				bad.name = "/escape"
			case "windows":
				bad.name = "C:/escape"
			case "backslash":
				bad.name = `dist\escape`
			case "symlink":
				bad.mode = fs.ModeSymlink | 0o777
			case "pipe":
				bad.mode = fs.ModeNamedPipe | 0o600
			case "later-bad-entry":
				bad.name = "dist//alias"
			case "file-parent":
				bad.name = "dist/first/child"
			}

			archive := boundedZIP(t, append(entries, bad))
			adapter, _ := serveExactArtifact(t, boundedArtifactMetadata(archive), archive)
			in := boundedArtifactInput(t)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)
			assertBoundedDestinationEmpty(t, in.Dir)
		})
	}
}

func assertBoundedDestinationEmpty(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if !os.IsNotExist(err) {
		require.NoError(t, err)
		require.Empty(t, entries, "no payload or staging installed on failure")
	}
}

func TestBoundedExactArtifactExpansionAndEntryCaps(t *testing.T) {
	for _, scenario := range []string{"single-bomb", "aggregate", "entry-count", "dishonest-zip-size"} {
		t.Run(scenario, func(t *testing.T) {
			entries := []boundedZIPEntry{{name: "dist/a", body: "small", mode: 0o600}}

			switch scenario {
			case "single-bomb":
				entries[0].body = strings.Repeat("x", int(boundedCaseBytes+1))
			case "aggregate":
				entries[0].body = strings.Repeat("x", 17<<20)
				entries = append(entries, boundedZIPEntry{name: "dist/b", body: entries[0].body, mode: 0o600})
			case "entry-count":
				for index := range 128 {
					entries = append(entries, boundedZIPEntry{name: "dist/" + strings.Repeat("x", index+1), mode: 0o600})
				}
			}

			archive := boundedZIP(t, entries)
			if scenario == "dishonest-zip-size" {
				offset := bytes.Index(archive, []byte{'P', 'K', 1, 2})
				require.GreaterOrEqual(t, offset, 0)
				binary.LittleEndian.PutUint32(archive[offset+24:offset+28], 1)
			}

			adapter, _ := serveExactArtifact(t, boundedArtifactMetadata(archive), archive)
			in := boundedArtifactInput(t)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)
			assertBoundedDestinationEmpty(t, in.Dir)
		})
	}
}

func TestBoundedExactArtifactStreamAndDigestRefusals(t *testing.T) {
	for _, scenario := range []string{"metadata-short", "metadata-long", "truncated", "digest-tamper", "http-oversize"} {
		t.Run(scenario, func(t *testing.T) {
			archive := buildZip(t, map[string]string{"dist/file": "bytes"})
			metadata := boundedArtifactMetadata(archive)

			switch scenario {
			case "metadata-short":
				metadata["size_in_bytes"] = len(archive) - 1
			case "metadata-long":
				metadata["size_in_bytes"] = len(archive) + 1
			case "truncated":
				archive = archive[:len(archive)/2]
				metadata = boundedArtifactMetadata(archive)
			case "digest-tamper":
				archive[len(archive)-1] ^= 1
			}

			adapter, server := serveExactArtifact(t, metadata, archive)
			if scenario == "http-oversize" {
				server.OnGet("/blob/42", func(_ fakegitserver.Request) fakegitserver.Response {
					return fakegitserver.Response{Status: http.StatusOK, Header: http.Header{"Content-Length": {"33554433"}}, Body: "small"}
				})
			}

			in := boundedArtifactInput(t)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)
			assertBoundedDestinationEmpty(t, in.Dir)
		})
	}
}

func TestBoundedExactArtifactCancelAndRetryFresh(t *testing.T) {
	archive := buildZip(t, map[string]string{"dist/file": "original bytes"})
	adapter, server := serveExactArtifact(t, boundedArtifactMetadata(archive), archive)
	in := boundedArtifactInput(t)
	ctx, cancel := context.WithCancel(t.Context())

	server.OnGet("/blob/42", func(_ fakegitserver.Request) fakegitserver.Response {
		cancel()

		return fakegitserver.Response{Status: http.StatusOK, Body: string(archive)}
	})

	_, err := adapter.DownloadRunArtifact(ctx, in)
	require.Error(t, err)
	assertBoundedDestinationEmpty(t, in.Dir)

	server.OnGet("/blob/42", func(_ fakegitserver.Request) fakegitserver.Response {
		return fakegitserver.Response{Status: http.StatusOK, Body: string(archive)}
	})

	info, err := adapter.DownloadRunArtifact(t.Context(), in)
	require.NoError(t, err)
	require.Equal(t, "42", info.ID)
	require.Equal(t, int64(len("original bytes")), info.Bytes)

	installed, err := os.ReadFile(filepath.Join(in.Dir, "dist/file"))
	require.NoError(t, err)
	require.Equal(t, "original bytes", string(installed))
}

func TestBoundedExactArtifactUnsupportedProviderRefusesAllFields(t *testing.T) {
	// A nil provider receiver proves these exact-only fields cannot fall through
	// to legacy resolution, environment credentials or an ignored native budget.
	var adapter *forgejo.Provider

	for _, in := range []provider.RunArtifactDownload{
		{ExpectedRepositoryID: "123"},
		{Limits: &provider.RunArtifactLimits{MaxArchiveBytes: boundedCaseBytes, MaxExpandedBytes: boundedCaseBytes, MaxEntries: 128}},
	} {
		_, err := adapter.DownloadRunArtifact(t.Context(), in)
		require.ErrorIs(t, err, errs.ErrUnsupported)
	}
}
