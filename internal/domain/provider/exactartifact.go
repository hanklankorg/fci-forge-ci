// SPDX-FileCopyrightText: The Forge CI Authors
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

// ExactArtifactDownloadIDs validates the opt-in binding before any provider
// call. Empty ID preserves legacy semantics, but cannot carry exact-only facts.
// Expectations must come from the trusted caller, never the downloaded payload.
func ExactArtifactDownloadIDs(in RunArtifactDownload) (int64, int64, error) {
	if in.ID == "" {
		if in.ExpectedRunHeadSHA != "" {
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

func exactArtifactID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, fmt.Errorf("artifact-id and run-id must be canonical positive int64 decimal IDs: %w", errs.ErrUsage)
	}

	return parsed, nil
}
