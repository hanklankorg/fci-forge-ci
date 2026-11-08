// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// This event requires registry publication in unsigned snapshot mode. Neither
// an ephemeral development artifact nor a successful PR is a substitute.
const campaignPublishedSnapshot = "published-unsigned-snapshot"

func campaignSupportsPublishedSnapshot(provider, project string) bool {
	return provider == campaignForgejo && project == "hello-maven-service"
}

func campaignSnapshotRecipe(dir, provider string, env map[string]string, words []string) ([]string, error) {
	if !campaignSupportsPublishedSnapshot(provider, words[1]) {
		return nil, fmt.Errorf("published unsigned snapshot supports only the Forgejo hello-maven-service native trial: %w", errs.ErrValidation)
	}

	if len(words) != 4 {
		return nil, fmt.Errorf("published unsigned snapshot takes job-image=<digest reference> alone: %w", errs.ErrValidation)
	}

	if _, ref, found := strings.Cut(words[3], "=@"); found && ref != "JOB_IMAGE_JAVA" {
		return nil, fmt.Errorf("published unsigned snapshot accepts only @JOB_IMAGE_JAVA or a literal digest reference: %w", errs.ErrValidation)
	}

	key, image, err := campaignSettingValue(env, words[3])
	if err != nil {
		return nil, err
	}

	if key != "job-image" || !container.ValidDigestReference(image) {
		return nil, fmt.Errorf("published unsigned snapshot requires job-image=<digest reference>: %w", errs.ErrValidation)
	}

	return []string{"test-live-forgejo-snapshot", filepath.Join(dir, "demos", words[1]), image}, nil
}
