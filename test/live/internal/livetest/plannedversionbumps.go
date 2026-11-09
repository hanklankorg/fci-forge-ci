// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

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

// ForgejoPlannedVersionBumps are the projects a consumer's Forgejo release
// bumps: PlannedVersionBumps of the declaration its release workflow (a file
// name under .forgejo/workflows) passes the layer's prepare-release.yml.
func ForgejoPlannedVersionBumps(consumerRoot, workflow string) ([]pipeline.PlannedArtifact, error) {
	artifactsConfig, err := forgejoCalledArtifactsConfig(filepath.Join(consumerRoot, ".forgejo", "workflows", workflow), "prepare-release.yml")
	if err != nil {
		return nil, err
	}

	return PlannedVersionBumps(consumerRoot, artifactsConfig)
}

// forgejoCalledArtifactsConfig is the artifacts-config input a Forgejo
// workflow passes the layer's called workflow, "" for the default.
func forgejoCalledArtifactsConfig(path, called string) (string, error) {
	body, err := os.ReadFile(path) //nolint:gosec // G304: the workflow of the consumer checkout the caller names
	if err != nil {
		return "", fmt.Errorf("read the release workflow: %w", err)
	}

	var workflow struct {
		Jobs map[string]struct {
			Uses string            `yaml:"uses"`
			With map[string]string `yaml:"with"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	for _, job := range workflow.Jobs {
		if strings.Contains(job.Uses, "/.forgejo/workflows/"+called+"@") {
			return job.With["artifacts-config"], nil
		}
	}

	return "", fmt.Errorf("%s calls no %s: %w", path, called, os.ErrNotExist)
}
