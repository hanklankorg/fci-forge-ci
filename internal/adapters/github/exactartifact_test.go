// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package github_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/diggsweden/forge-ci/internal/adapters/github"
	domainartifact "github.com/diggsweden/forge-ci/internal/domain/artifact"
	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/testutil/fakegitserver"
	"github.com/stretchr/testify/require"
)

func exactArtifactInput(t *testing.T) provider.RunArtifactDownload {
	t.Helper()

	return provider.RunArtifactDownload{ID: "42", Name: "build-dist", Repository: "owner/repo", RunID: "9876",
		ExpectedRunHeadSHA: strings.Repeat("a", 40), Dir: filepath.Join(t.TempDir(), "not-created", "payload")}
}

func exactArtifactMetadata(body []byte) map[string]any {
	digest := sha256.Sum256(body)

	return map[string]any{
		"id": 42, "name": "build-dist", "expired": false, "digest": "sha256:" + hex.EncodeToString(digest[:]),
		"workflow_run": map[string]any{"id": 9876, "repository_id": 123, "head_repository_id": 123, "head_sha": strings.Repeat("a", 40), "head_branch": "main"},
	}
}

func serveExactArtifact(t *testing.T, metadata map[string]any, archive []byte) (*github.Provider, *fakegitserver.Server) {
	t.Helper()
	srv := fakegitserver.New(t)
	srv.OnGet("/repos/owner/repo", func(_ fakegitserver.Request) fakegitserver.Response {
		return fakegitserver.Response{Status: http.StatusOK, Body: `{"id":123,"full_name":"owner/repo"}`}
	})
	srv.OnGet("/repos/owner/repo/actions/artifacts/42", func(_ fakegitserver.Request) fakegitserver.Response {
		body, err := json.Marshal(metadata)
		require.NoError(t, err)

		return fakegitserver.Response{Status: http.StatusOK, Body: string(body)}
	})
	srv.OnGet("/repos/owner/repo/actions/artifacts/42/zip", func(_ fakegitserver.Request) fakegitserver.Response {
		return fakegitserver.Response{Status: http.StatusFound, Header: http.Header{"Location": {"https://blob.invalid/blob/42?sig=private-signed-query"}}}
	})
	srv.OnGet("/blob/42", func(req fakegitserver.Request) fakegitserver.Response {
		require.Empty(t, req.Header.Get("Authorization"), "API credentials must not reach the presigned blob")

		return fakegitserver.Response{Status: http.StatusOK, Body: string(archive)}
	})

	return &github.Provider{Env: envFunc(map[string]string{"GH_TOKEN": "private-api-token", "GITHUB_RUN_ID": "foreign-ambient-run", "GITHUB_SHA": strings.Repeat("b", 40)}), APIBaseOverride: srv.URL(), HTTPClient: srv.Client()}, srv
}

func TestExactArtifactDownload_ObservesRunHeadNotPreparedCheckout(t *testing.T) {
	for _, digestProvided := range []bool{true, false} {
		t.Run(strconv.FormatBool(digestProvided), func(t *testing.T) {
			in := exactArtifactInput(t)
			archive := buildZip(t, map[string]string{"prepared-sha": strings.Repeat("b", 40), "dist/product.cdx.json": "producer bytes\n"})

			metadata := exactArtifactMetadata(archive)
			if !digestProvided {
				delete(metadata, "digest")
			}

			adapter, srv := serveExactArtifact(t, metadata, archive)
			if !digestProvided {
				srv.OnGet("/blob/42", func(req fakegitserver.Request) fakegitserver.Response {
					require.Empty(t, req.Header.Get("Authorization"))

					return fakegitserver.Response{Status: http.StatusFound, Header: http.Header{"Location": {"https://other-blob.invalid/blob/final?sig=private-signed-query"}}}
				})
				srv.OnGet("/blob/final", func(req fakegitserver.Request) fakegitserver.Response {
					require.Empty(t, req.Header.Get("Authorization"))

					return fakegitserver.Response{Status: http.StatusOK, Body: string(archive)}
				})
			}

			info, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.NoError(t, err)
			require.Equal(t, in.ID, info.ID)
			require.Equal(t, in.Name, info.Name)
			require.Equal(t, in.ExpectedRunHeadSHA, info.Observation.RunHeadSHA)
			require.Equal(t, "123", info.Observation.RepositoryID)
			require.Equal(t, "123", info.Observation.HeadRepositoryID)
			require.Equal(t, "owner/repo", info.Observation.Repository)
			require.Equal(t, "9876", info.Observation.RunID)
			require.Equal(t, "main", info.Observation.HeadBranch)
			require.Equal(t, digestProvided, info.Observation.ArchiveDigest != "")

			body, err := os.ReadFile(filepath.Join(in.Dir, "prepared-sha"))
			require.NoError(t, err)
			require.Equal(t, strings.Repeat("b", 40), string(body), "no assertion that payload checkout equals the request run head")

			expectedRequests := 4
			if !digestProvided {
				expectedRequests++
			}

			require.Len(t, srv.Requests(), expectedRequests, "exact-ID must never list or search by name")

			for _, request := range srv.Requests()[:3] {
				require.Equal(t, "Bearer private-api-token", request.Header.Get("Authorization"))
			}
		})
	}
}

func TestExactArtifactDownload_RejectsForeignOrMissingFactsBeforeArchiveOrStaging(t *testing.T) {
	for _, scenario := range []string{"wrong-id", "wrong-name", "expired", "missing-expiry", "missing-run", "wrong-run", "wrong-repository-id", "foreign-head-repository", "wrong-head", "missing-head", "wrong-repository-name", "missing-artifact", "unsupported-digest"} {
		t.Run(scenario, func(t *testing.T) {
			in := exactArtifactInput(t)
			archive := buildZip(t, map[string]string{"file": "bytes"})
			metadata := exactArtifactMetadata(archive)
			run := metadata["workflow_run"].(map[string]any)

			switch scenario {
			case "wrong-id":
				metadata["id"] = 43
			case "wrong-name":
				metadata["name"] = "other-dist"
			case "expired":
				metadata["expired"] = true
			case "missing-expiry":
				delete(metadata, "expired")
			case "missing-run":
				delete(metadata, "workflow_run")
			case "wrong-run":
				run["id"] = 9877
			case "wrong-repository-id":
				run["repository_id"] = 456
			case "foreign-head-repository":
				run["head_repository_id"] = 456
			case "wrong-head":
				in.ExpectedRunHeadSHA = strings.Repeat("b", 40) // prepared SHA is not run head
			case "missing-head":
				delete(run, "head_sha")
			case "unsupported-digest":
				metadata["digest"] = "md5:untrusted"
			}

			adapter, srv := serveExactArtifact(t, metadata, archive)
			if scenario == "wrong-repository-name" {
				srv.OnGet("/repos/owner/repo", func(_ fakegitserver.Request) fakegitserver.Response {
					return fakegitserver.Response{Status: http.StatusOK, Body: `{"id":123,"full_name":"foreign/repository"}`}
				})
			}

			if scenario == "missing-artifact" {
				srv.OnGet("/repos/owner/repo/actions/artifacts/42", func(_ fakegitserver.Request) fakegitserver.Response {
					return fakegitserver.Response{Status: http.StatusNotFound, Body: `{"message":"not found"}`}
				})
			}

			info, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)
			require.Nil(t, info.Observation)

			for _, request := range srv.Requests() {
				require.NotContains(t, request.Path, "/zip")
				require.NotContains(t, request.Path, "/blob")
			}

			_, err = os.Lstat(filepath.Dir(in.Dir))
			require.True(t, os.IsNotExist(err), "metadata refusal must not create staging or destination ancestors")
		})
	}
}

func TestExactArtifactDownload_InvalidSelectorsHaveNoEffects(t *testing.T) {
	for _, scenario := range []string{"zero", "negative", "overflow", "nonnumeric", "missing-id", "missing-name", "missing-repository", "missing-run", "zero-run", "missing-head", "pattern", "merge"} {
		t.Run(scenario, func(t *testing.T) {
			in := exactArtifactInput(t)

			switch scenario {
			case "zero":
				in.ID = "0"
			case "negative":
				in.ID = "-1"
			case "overflow":
				in.ID = "9223372036854775808"
			case "nonnumeric":
				in.ID = "latest"
			case "missing-id":
				in.ID = ""
			case "missing-name":
				in.Name = ""
			case "missing-repository":
				in.Repository = ""
			case "missing-run":
				in.RunID = ""
			case "zero-run":
				in.RunID = "0"
			case "missing-head":
				in.ExpectedRunHeadSHA = ""
			case "pattern":
				in.Pattern = "*"
			case "merge":
				in.MergeMultiple = true
			}

			adapter, srv := serveExactArtifact(t, nil, nil)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)
			require.Empty(t, srv.Requests())
		})
	}
}

func TestExactArtifactDownload_DoesNotFallBackToAmbiguousName(t *testing.T) {
	in := exactArtifactInput(t)
	archive := buildZip(t, map[string]string{"file": "bytes"})
	adapter, srv := serveExactArtifact(t, exactArtifactMetadata(archive), archive)
	srv.OnGet("/repos/owner/repo/actions/runs/9876/artifacts", func(_ fakegitserver.Request) fakegitserver.Response {
		return fakegitserver.Response{Status: http.StatusOK, Body: `{"total_count":2,"artifacts":[{"id":42,"name":"build-dist"},{"id":43,"name":"build-dist"}]}`}
	})

	_, err := adapter.DownloadRunArtifact(t.Context(), in)
	require.NoError(t, err)

	in.ID, in.ExpectedRunHeadSHA = "", ""
	in.Dir = filepath.Join(t.TempDir(), "legacy")
	_, err = adapter.DownloadRunArtifact(t.Context(), in)
	require.ErrorContains(t, err, "multiple artifacts")
}

func TestExactArtifactDownload_ArchiveRefusalsInstallNothingAndRedactURL(t *testing.T) {
	for _, scenario := range []string{"digest-mismatch", "truncated", "malformed", "oversized", "duplicate-path", "traversal", "blob-error", "credentialed-url", "transport-error"} {
		t.Run(scenario, func(t *testing.T) {
			spool := t.TempDir()
			t.Setenv("RUNNER_TEMP", spool)

			in := exactArtifactInput(t)
			archive := buildZip(t, map[string]string{"file": "bytes"})
			metadata := exactArtifactMetadata(archive)

			switch scenario {
			case "digest-mismatch":
				metadata["digest"] = "sha256:" + strings.Repeat("a", 64)
			case "truncated":
				archive = archive[:len(archive)-8]
				metadata = exactArtifactMetadata(archive)
			case "malformed":
				archive = []byte("not a zip archive")
				metadata = exactArtifactMetadata(archive)
			case "traversal":
				archive = buildZip(t, map[string]string{"../escape": "bytes"})
				metadata = exactArtifactMetadata(archive)
			case "duplicate-path":
				var buffer bytes.Buffer

				writer := zip.NewWriter(&buffer)
				for range 2 {
					_, err := writer.Create("duplicate")
					require.NoError(t, err)
				}

				require.NoError(t, writer.Close())

				archive = buffer.Bytes()
				metadata = exactArtifactMetadata(archive)
			}

			adapter, srv := serveExactArtifact(t, metadata, archive)
			if scenario == "transport-error" {
				base := adapter.HTTPClient.Transport
				adapter.HTTPClient.Transport = exactTestTransport(func(request *http.Request) (*http.Response, error) {
					if strings.HasPrefix(request.URL.Path, "/blob/") {
						require.Empty(t, request.Header.Get("Authorization"))

						return nil, errors.New("synthetic transport failure private-signed-query private-api-token")
					}

					return base.RoundTrip(request)
				})
			}

			if scenario == "oversized" || scenario == "blob-error" {
				srv.OnGet("/blob/42", func(_ fakegitserver.Request) fakegitserver.Response {
					if scenario == "blob-error" {
						return fakegitserver.Response{Status: http.StatusForbidden}
					}

					return fakegitserver.Response{Status: http.StatusOK, Header: http.Header{"Content-Length": {strconv.FormatInt(domainartifact.MaxArchiveBytes+1, 10)}}}
				})
			}

			if scenario == "credentialed-url" {
				srv.OnGet("/repos/owner/repo/actions/artifacts/42/zip", func(_ fakegitserver.Request) fakegitserver.Response {
					return fakegitserver.Response{Status: http.StatusFound, Header: http.Header{"Location": {"https://private-user:private-password@blob.invalid/archive?sig=private-signed-query"}}}
				})
			}

			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.Error(t, err)

			for _, secret := range []string{"private-api-token", "private-signed-query", "private-password"} {
				require.NotContains(t, err.Error(), secret)
			}

			_, err = os.Lstat(in.Dir)
			require.True(t, os.IsNotExist(err))
			entries, err := os.ReadDir(spool)
			require.NoError(t, err)
			require.Empty(t, entries, "failed downloads must clean their ZIP spool")
		})
	}
}

type exactTestTransport func(*http.Request) (*http.Response, error)

func (f exactTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestExactArtifactDownload_RejectsCanonicalZIPAliasesBeforeStaging(t *testing.T) {
	for _, names := range [][]string{{"file", "./file"}, {"dir/file", "dir//file"}, {"dir/file", "dir/./file"}} {
		t.Run(strings.Join(names, "_"), func(t *testing.T) {
			spool := t.TempDir()
			t.Setenv("RUNNER_TEMP", spool)
			in := exactArtifactInput(t)
			archive := buildZip(t, map[string]string{names[0]: "first", names[1]: "replacement"})
			adapter, _ := serveExactArtifact(t, exactArtifactMetadata(archive), archive)
			_, err := adapter.DownloadRunArtifact(t.Context(), in)
			require.ErrorContains(t, err, "duplicate paths")
			_, err = os.Lstat(filepath.Dir(in.Dir))
			require.True(t, os.IsNotExist(err), "alias refusal must precede extraction staging and destination creation")
			entries, err := os.ReadDir(spool)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestExactArtifactDownload_EnforcesSecureCredentialFreeRedirectsEveryHop(t *testing.T) {
	for _, scenario := range []string{"http", "userinfo", "second-http", "second-userinfo", "secure", "bound", "policy-mutation", "policy-refusal"} {
		t.Run(scenario, func(t *testing.T) {
			in := exactArtifactInput(t)
			archive := buildZip(t, map[string]string{"file": "bytes"})
			metadata := exactArtifactMetadata(archive)
			delete(metadata, "digest") // transport safety cannot depend on a ZIP digest
			adapter, srv := serveExactArtifact(t, metadata, archive)
			sharedClient := adapter.HTTPClient
			sharedClient.Timeout = 23 * time.Second
			jar, err := cookiejar.New(nil)
			require.NoError(t, err)
			blobURL, err := url.Parse("https://blob.invalid")
			require.NoError(t, err)
			jar.SetCookies(blobURL, []*http.Cookie{{Name: "ambient", Value: "private-cookie"}})
			sharedClient.Jar = jar
			policyCalls := 0
			sharedClient.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
				policyCalls++

				if scenario == "policy-refusal" {
					return errors.New("private-signed-query caller refuses redirect")
				}

				if scenario == "policy-mutation" {
					next.URL.Scheme = "http"
				}
				// The exact-mode guard must strip even headers a previous policy
				// adds, without changing that policy on the shared client.
				for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Cookie2", "Referer"} {
					next.Header.Set(header, "private-redirect-credential")
				}

				return nil
			}
			nextLocation := "https://other-blob.invalid/blob/final?sig=private-signed-query"

			switch scenario {
			case "http":
				nextLocation = "http://other-blob.invalid/blob/blocked?sig=private-signed-query"
			case "userinfo":
				nextLocation = "https://private-user:private-password@other-blob.invalid/blob/blocked?sig=private-signed-query"
			case "bound", "second-http", "second-userinfo":
				nextLocation = "https://other-blob.invalid/blob/hop1?sig=private-signed-query"
			}

			srv.OnGet("/blob/42", func(req fakegitserver.Request) fakegitserver.Response {
				require.Empty(t, req.Header.Get("Authorization"))
				require.Empty(t, req.Header.Get("Cookie"), "shared cookie jar must not authenticate the initial blob request")

				return fakegitserver.Response{Status: http.StatusFound, Header: http.Header{"Location": {nextLocation}}}
			})

			for _, path := range []string{"/blob/final", "/blob/blocked", "/blob/hop1", "/blob/hop2", "/blob/hop3"} {
				srv.OnGet(path, func(req fakegitserver.Request) fakegitserver.Response {
					for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Cookie2", "Referer"} {
						require.Empty(t, req.Header.Get(header), "credential or signed referrer sent on redirect")
					}

					if req.Path == "/blob/hop1" || req.Path == "/blob/hop2" {
						if scenario == "second-http" || scenario == "second-userinfo" {
							location := "http://other-blob.invalid/blob/blocked?sig=private-signed-query"
							if scenario == "second-userinfo" {
								location = "https://private-user:private-password@other-blob.invalid/blob/blocked?sig=private-signed-query"
							}

							return fakegitserver.Response{Status: http.StatusFound, Header: http.Header{"Location": {location}}}
						}

						next := "/blob/hop2"
						if req.Path == "/blob/hop2" {
							next = "/blob/hop3"
						}

						return fakegitserver.Response{Status: http.StatusFound, Header: http.Header{"Location": {"https://other-blob.invalid" + next}}}
					}

					return fakegitserver.Response{Status: http.StatusOK, Body: string(archive)}
				})
			}

			_, err = adapter.DownloadRunArtifact(t.Context(), in)
			if scenario == "secure" {
				require.NoError(t, err)
				require.Equal(t, 1, policyCalls)
			} else {
				require.Error(t, err)

				for _, secret := range []string{"private-signed-query", "private-password", "private-redirect-credential"} {
					require.NotContains(t, err.Error(), secret)
				}

				_, statErr := os.Lstat(filepath.Dir(in.Dir))
				require.True(t, os.IsNotExist(statErr))
			}

			blobPaths := []string{}

			for _, request := range srv.Requests() {
				if strings.HasPrefix(request.Path, "/blob/") {
					blobPaths = append(blobPaths, request.Path)
				}
			}

			switch scenario {
			case "secure":
				require.Equal(t, []string{"/blob/42", "/blob/final"}, blobPaths)
			case "bound":
				require.Equal(t, []string{"/blob/42", "/blob/hop1", "/blob/hop2"}, blobPaths)
			case "second-http", "second-userinfo":
				require.Equal(t, []string{"/blob/42", "/blob/hop1"}, blobPaths)
			default:
				require.Equal(t, []string{"/blob/42"}, blobPaths, "refused redirect destination must not receive a request")
			}

			require.Same(t, sharedClient, adapter.HTTPClient)
			require.Same(t, jar, sharedClient.Jar)
			require.Equal(t, 23*time.Second, sharedClient.Timeout)
			// Calling the original policy on an HTTP request must still invoke
			// the caller callback: the shared client's policy was not replaced.
			before := policyCalls
			probe, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://probe.invalid", nil)
			require.NoError(t, err)

			_ = sharedClient.CheckRedirect(probe, nil)

			require.Equal(t, before+1, policyCalls)
		})
	}
}
