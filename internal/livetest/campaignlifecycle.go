// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// campaignLifecycleDemo is the one demo with a development route, the
// lifecycle's development phase: on Forgejo its own service development
// workflow, on GitLab the policy's snapshot-images component.
const campaignLifecycleDemo = "hello-maven-service"

// campaignSupportsLifecycle reports whether a demo runs the lifecycle event on
// a provider. A demo without a development route is refused, not run as a
// shorter flow.
func campaignSupportsLifecycle(provider, project string) bool {
	return isCampaignProvider(provider) && project == campaignLifecycleDemo
}

// campaignAdmitLifecycle refuses a lifecycle case outside the one demo with a
// development route. The case's settings are held to the lifecycle's rules
// where every case's settings are: ParseTrialSettings on Forgejo,
// ParseGitLabLifecycleSettings in the GitLab admission.
func campaignAdmitLifecycle(provider, project string) error {
	if !campaignSupportsLifecycle(provider, project) {
		return fmt.Errorf("%s runs the %s demo only, whose service development is the lifecycle's development phase: %w", TrialLifecycle, campaignLifecycleDemo, errs.ErrValidation)
	}

	return nil
}
