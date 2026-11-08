// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// lifecycleJobImage is the variable a lifecycle's development phase takes the
// Java job image from, the image its service snapshot builds with: the same
// image the consumer's own workflows run in.
const lifecycleJobImage = "CI_JOB_IMAGE"

// campaignSupportsLifecycle reports whether a demo runs the lifecycle event on
// a provider: the demos of the published snapshot, because the lifecycle's
// development phase is that demo's service development route. A demo without
// a development route is refused, not run as a shorter flow.
func campaignSupportsLifecycle(provider, project string) bool {
	return campaignSupportsPublishedSnapshot(provider, project)
}

// campaignAdmitLifecycle refuses a lifecycle case the lane could not run: a
// demo or provider without the development route, or a Java job image that is
// not pinned by digest, which the service fixture would refuse after the
// repositories exist.
func campaignAdmitLifecycle(provider, project string, settings []string) error {
	if !campaignSupportsLifecycle(provider, project) {
		return fmt.Errorf("%s runs the Forgejo hello-maven-service demo only, whose service development is the lifecycle's development phase: %w", TrialLifecycle, errs.ErrValidation)
	}

	for _, setting := range settings {
		if image, found := strings.CutPrefix(setting, lifecycleJobImage+"="); found && container.ValidDigestReference(image) {
			return nil
		}
	}

	return fmt.Errorf("%s requires %s=<digest reference>, the Java job image its development phase builds the service with: %w", TrialLifecycle, lifecycleJobImage, errs.ErrValidation)
}
