// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"path/filepath"

	appconfig "github.com/diggsweden/forge-ci/internal/app/config"
	"github.com/diggsweden/forge-ci/internal/domain/output"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
)

// PlannedVersionBumps are the projects a consumer's GitLab or Forgejo release
// bumps: the release plan of the declaration its artifacts-config names
// (artifactsConfig, empty for the default, the same file on both forges), as
// the release's own plan job reads it. Without a declaration nothing is
// bumped.
func PlannedVersionBumps(consumerRoot, artifactsConfig string) ([]pipeline.PlannedArtifact, error) {
	if consumerRoot == "" {
		return nil, nil
	}

	plans, err := appconfig.LoadReleaseStagePlans(appconfig.EmitConfigPlanInput{
		Path: filepath.Join(consumerRoot, cmp.Or(artifactsConfig, GitLabDefaultArtifactsConfig)),
	}, output.Annotator{}, artifactsConfig == "")
	if err != nil {
		return nil, err
	}

	if !plans.Prepare.Targets.VersionBump.Runs {
		return nil, nil
	}

	return plans.Prepare.Targets.VersionBump.Items, nil
}
