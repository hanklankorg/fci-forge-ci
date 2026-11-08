// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package github

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	gogithub "github.com/google/go-github/v76/github"

	"github.com/diggsweden/forge-ci/internal/adapters/httpretry"
	domainartifact "github.com/diggsweden/forge-ci/internal/domain/artifact"
	domaincontainer "github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/provider"
)

const (
	artifactArchiveScheme            = "https"
	exactArtifactAuthorizationHeader = "Authorization"
)

// downloadExactArtifact uses repository and artifact metadata, not a name list.
// All expectations hold before resolving a blob URL or creating local staging.
func (p *Provider) downloadExactArtifact(ctx context.Context, in provider.RunArtifactDownload, artifactID, runID int64) (provider.RunArtifactInfo, error) {
	owner, repo, err := resolveOwnerRepo(p, in.Repository)
	if err != nil {
		return provider.RunArtifactInfo{}, err
	}

	client, err := p.releaseClient(ctx)
	if err != nil {
		return provider.RunArtifactInfo{}, httpretry.SafeError(err)
	}

	repository, response, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return provider.RunArtifactInfo{}, fmt.Errorf("observe artifact repository: %w", httpretry.SafeError(classifyArtifactResponse(response, err)))
	}

	if repository.GetID() <= 0 || !strings.EqualFold(repository.GetFullName(), in.Repository) {
		return provider.RunArtifactInfo{}, fmt.Errorf("artifact repository does not match expected repository: %w", errs.ErrValidation)
	}

	artifact, response, err := client.Actions.GetArtifact(ctx, owner, repo, artifactID)
	if err != nil {
		return provider.RunArtifactInfo{}, fmt.Errorf("observe exact artifact: %w", httpretry.SafeError(classifyArtifactResponse(response, err)))
	}

	observation, err := bindExactArtifact(in, repository, artifact, artifactID, runID)
	if err != nil {
		return provider.RunArtifactInfo{}, err
	}

	limits := exactArchiveLimits(in, artifact)

	location, response, err := client.Actions.DownloadArtifact(ctx, owner, repo, artifactID, maxArtifactRedirects)
	if err != nil {
		return provider.RunArtifactInfo{}, fmt.Errorf("resolve exact artifact archive: %w", httpretry.SafeError(classifyArtifactResponse(response, err)))
	}

	if err := validateExactArtifactURL(location); err != nil {
		return provider.RunArtifactInfo{}, err
	}

	written, count, err := downloadAndExtractZipChecked(ctx, exactArtifactDownloadClient(p.httpClient()), location.String(), in.Dir,
		limits.expanded, limits.entries, observation.ArchiveDigest, true, limits)
	if err != nil {
		return provider.RunArtifactInfo{}, err
	}

	return provider.RunArtifactInfo{Name: artifact.GetName(), ID: in.ID, Bytes: written, FileCount: count, Observation: observation}, nil
}

func bindExactArtifact(in provider.RunArtifactDownload, repository *gogithub.Repository, artifact *gogithub.Artifact, artifactID, runID int64) (*provider.RunArtifactObservation, error) {
	if artifact == nil || artifact.GetID() != artifactID || artifact.GetName() != in.Name || artifact.Expired == nil || artifact.GetExpired() {
		return nil, fmt.Errorf("exact artifact ID/name/expiry does not match the expected live artifact: %w", errs.ErrValidation)
	}

	if err := validateExactRepositoryAndSize(in, repository, artifact); err != nil {
		return nil, err
	}

	run := artifact.WorkflowRun
	if run == nil || run.GetID() != runID || run.GetRepositoryID() != repository.GetID() || run.GetHeadRepositoryID() != repository.GetID() || run.GetHeadSHA() != in.ExpectedRunHeadSHA {
		return nil, fmt.Errorf("artifact workflow run/repository/head does not match trusted expectations: %w", errs.ErrValidation)
	}

	digest := artifact.GetDigest()
	if digest != "" && (!strings.HasPrefix(digest, "sha256:") || !domaincontainer.ValidSHA256Hex(strings.TrimPrefix(digest, "sha256:"))) {
		return nil, fmt.Errorf("artifact API returned an unsupported archive digest: %w", errs.ErrValidation)
	}

	return &provider.RunArtifactObservation{
		Repository: repository.GetFullName(), RepositoryID: strconv.FormatInt(repository.GetID(), 10),
		RunID: strconv.FormatInt(run.GetID(), 10), RunHeadSHA: run.GetHeadSHA(),
		HeadRepositoryID: strconv.FormatInt(run.GetHeadRepositoryID(), 10), HeadBranch: run.GetHeadBranch(), ArchiveDigest: digest,
	}, nil
}

func validateExactRepositoryAndSize(in provider.RunArtifactDownload, repository *gogithub.Repository, artifact *gogithub.Artifact) error {
	if in.ExpectedRepositoryID != "" && strconv.FormatInt(repository.GetID(), 10) != in.ExpectedRepositoryID {
		return fmt.Errorf("artifact repository ID differs from trusted created repository: %w", errs.ErrValidation)
	}

	if in.Limits != nil && (artifact.SizeInBytes == nil || artifact.GetSizeInBytes() <= 0 || artifact.GetSizeInBytes() > in.Limits.MaxArchiveBytes) {
		return fmt.Errorf("exact artifact metadata size is missing or exceeds case archive budget: %w", errs.ErrValidation)
	}

	return nil
}

func validateExactArtifactZip(reader *zip.ReadCloser, exact bool, limits artifactArchiveLimits) error {
	if !exact {
		return nil
	}

	if len(reader.File) > limits.entries {
		return fmt.Errorf("exact artifact ZIP exceeds entry budget: %w", errs.ErrValidation)
	}

	seen := make(map[string]bool, len(reader.File))

	if limits.expanded < 0 {
		return fmt.Errorf("invalid expanded ZIP budget: %w", errs.ErrValidation)
	}

	remaining := uint64(limits.expanded)

	for _, entry := range reader.File {
		name := strings.TrimSuffix(entry.Name, "/")

		if limits.bounded {
			if name == "." || path.Clean(name) != name || strings.ContainsAny(name, ":\x00") || (!entry.Mode().IsRegular() && !entry.FileInfo().IsDir()) {
				return fmt.Errorf("bounded artifact ZIP requires canonical regular paths: %w", errs.ErrValidation)
			}

			if entry.UncompressedSize64 > remaining {
				return fmt.Errorf("bounded artifact ZIP exceeds expanded budget: %w", errs.ErrValidation)
			}

			remaining -= entry.UncompressedSize64
		}

		destination, err := domainartifact.SafeJoin(".", name)
		if err != nil {
			return err
		}

		if seen[destination] {
			return fmt.Errorf("exact artifact ZIP contains duplicate paths: %w", errs.ErrValidation)
		}

		seen[destination] = true
	}

	return nil
}

func validateExactArtifactURL(location *url.URL) error {
	if location == nil || location.Scheme != artifactArchiveScheme || location.Host == "" || location.User != nil {
		return fmt.Errorf("exact artifact archive requires a credential-free HTTPS URL: %w", errs.ErrValidation)
	}

	return nil
}

// Keep the transport (including retries), timeout and caller redirect policy,
// without modifying the shared client. SameAuthority is intentionally not used:
// presigned blobs may legitimately redirect between HTTPS storage authorities.
func exactArtifactDownloadClient(client *http.Client) *http.Client {
	guarded := *client
	previous := client.CheckRedirect
	// Artifact blobs require no ambient cookies, including cookies a shared jar
	// would otherwise inject after CheckRedirect has returned.
	guarded.Jar = nil
	guarded.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= maxArtifactRedirects {
			return fmt.Errorf("exact artifact archive redirect limit reached: %w", errs.ErrDependencyUnavailable)
		}

		if err := validateExactArtifactURL(next.URL); err != nil {
			return err
		}

		if previous != nil {
			if err := previous(next, via); err != nil {
				return err
			}
		}

		// A caller policy can modify the next request. Recheck after composition
		// and strip credentials and the signed source URL before the hop is sent.
		if err := validateExactArtifactURL(next.URL); err != nil {
			return err
		}

		for _, header := range []string{exactArtifactAuthorizationHeader, "Proxy-Authorization", "Cookie", "Cookie2", "Referer"} {
			next.Header.Del(header)
		}

		return nil
	}

	return &guarded
}

func verifyArtifactArchiveDigest(actual, expected string) error {
	if expected != "" && "sha256:"+actual != expected {
		return fmt.Errorf("artifact archive digest mismatch: %w", errs.ErrValidation)
	}

	return nil
}
