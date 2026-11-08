// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package provider

import (
	"fmt"
	"strconv"

	"github.com/diggsweden/forge-ci/internal/domain/artifact"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/git"
	"github.com/diggsweden/forge-ci/internal/domain/repository"
)

// RunArtifactLimits narrows native budgets for one exact artifact. It does not
// authenticate a producer job/attempt or establish a global peak disk budget.
type RunArtifactLimits struct {
	MaxArchiveBytes  int64
	MaxExpandedBytes int64
	MaxEntries       int
}

func (limits RunArtifactLimits) validate() error {
	if limits.MaxArchiveBytes <= 0 || limits.MaxArchiveBytes > artifact.MaxArchiveBytes ||
		limits.MaxExpandedBytes <= 0 || limits.MaxExpandedBytes > artifact.MaxTotalBytes ||
		limits.MaxEntries <= 0 || limits.MaxEntries > artifact.MaxFileCount {
		return fmt.Errorf("exact artifact limits must be positive and not exceed native budgets: %w", errs.ErrUsage)
	}

	return nil
}

// ExactArtifactDownloadIDs validates the opt-in binding before any provider
// call. Empty ID preserves legacy semantics, but cannot carry exact-only facts.
// Expectations must come from the trusted caller, never the downloaded payload.
func ExactArtifactDownloadIDs(in RunArtifactDownload) (int64, int64, error) {
	if in.ID == "" {
		if in.RequiresExactBinding() {
			return 0, 0, fmt.Errorf("expected run head requires artifact-id: %w", errs.ErrUsage)
		}

		return 0, 0, nil
	}

	if in.Pattern != "" || in.MergeMultiple || in.Dir == "" || in.Repository == "" || in.RunID == "" {
		return 0, 0, fmt.Errorf("exact artifact download needs name, repository, run-id, expected-run-head-sha and dir; pattern/merge are not supported: %w", errs.ErrUsage)
	}

	if err := artifact.ValidateName(in.Name); err != nil {
		return 0, 0, err
	}

	if !git.ValidCommitSHA(in.ExpectedRunHeadSHA) {
		return 0, 0, fmt.Errorf("expected-run-head-sha must be a full commit SHA: %w", errs.ErrUsage)
	}

	if err := validateExactArtifactBounds(in); err != nil {
		return 0, 0, err
	}

	artifactID, err := exactArtifactID(in.ID)
	if err != nil {
		return 0, 0, err
	}

	runID, err := exactArtifactID(in.RunID)
	if err != nil {
		return 0, 0, err
	}

	if _, err := repository.ParseOwnerName(in.Repository); err != nil {
		return 0, 0, fmt.Errorf("exact artifact repository must be owner/repo: %w", errs.ErrUsage)
	}

	return artifactID, runID, nil
}

// RequiresExactBinding lets unsupported providers refuse every opt-in exact
// field before effects, including a malformed request with no artifact ID.
func (in RunArtifactDownload) RequiresExactBinding() bool {
	return in.ID != "" || in.ExpectedRunHeadSHA != "" || in.ExpectedRepositoryID != "" || in.Limits != nil
}

func validateExactArtifactBounds(in RunArtifactDownload) error {
	if in.Limits != nil {
		if err := in.Limits.validate(); err != nil {
			return err
		}

		if in.ExpectedRepositoryID == "" {
			return fmt.Errorf("bounded exact artifact requires the trusted repository ID: %w", errs.ErrUsage)
		}
	}

	if in.ExpectedRepositoryID != "" {
		if _, err := exactArtifactID(in.ExpectedRepositoryID); err != nil {
			return err
		}
	}

	return nil
}

func exactArtifactID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, fmt.Errorf("artifact-id and run-id must be canonical positive int64 decimal IDs: %w", errs.ErrUsage)
	}

	return parsed, nil
}
