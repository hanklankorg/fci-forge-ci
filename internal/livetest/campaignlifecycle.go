// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// campaignAdmitLifecycle refuses a lifecycle case outside the demos of the
// published snapshot on Forgejo, because the lifecycle's development phase is
// that demo's service development route. A demo without a development route
// is refused, not run as a shorter flow. The case's settings are held to the
// lifecycle's rules by ParseTrialSettings, as every Forgejo consumer case's.
func campaignAdmitLifecycle(provider, project string) error {
	if !campaignSupportsPublishedSnapshot(provider, project) {
		return fmt.Errorf("%s runs the Forgejo hello-maven-service demo only, whose service development is the lifecycle's development phase: %w", TrialLifecycle, errs.ErrValidation)
	}

	return nil
}
