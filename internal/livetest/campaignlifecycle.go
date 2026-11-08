// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// campaignSupportsLifecycle reports whether a demo runs the lifecycle event on
// a provider: whether it has a development route there. A demo without one is
// refused, not run as a shorter flow.
func campaignSupportsLifecycle(provider, checkout string) bool {
	return campaignAdmitLifecycle(provider, checkout) == nil
}

// campaignAdmitLifecycle refuses a lifecycle case whose demo has no
// development route, the lifecycle's development phase: a service snapshot
// of the demo's one container, which the snapshot-images policy builds from
// the declaration the demo's release builds its images from
// (ServiceSnapshotSource), on Forgejo through the harness's service
// development workflow, on GitLab through the component. The route is read
// from the demo's files, not its name, and the planner's refusal is the
// demo's. The case's settings are held to the lifecycle's rules where every
// case's settings are: ParseTrialSettings on Forgejo,
// ParseGitLabLifecycleSettings in the GitLab admission.
func campaignAdmitLifecycle(provider, checkout string) error {
	if !isCampaignProvider(provider) {
		return fmt.Errorf("%s runs on %s and %s: %w", TrialLifecycle, campaignForgejo, campaignGitLab, errs.ErrValidation)
	}

	source, err := ServiceSnapshotSource(checkout)
	if err != nil {
		return err
	}

	return serviceSnapshotAdmission(checkout, source)
}

// errNoDevelopmentRoute is a demo without the root Containerfile or the
// declaration a service snapshot is planned from: the live harness's scope.
var errNoDevelopmentRoute = errors.New("no service snapshot and no development route")

// serviceSnapshotAdmission refuses a demo whose source declaration the
// planner cannot plan as a service snapshot, the development route every
// forge's lifecycle shares; a demo without a root Containerfile or the
// declaration has none.
func serviceSnapshotAdmission(checkout, source string) error {
	if present, err := exists(filepath.Join(checkout, "Containerfile")); err != nil || !present {
		return fmt.Errorf("%s: the demo has no root Containerfile, so %w: %w", TrialLifecycle, errNoDevelopmentRoute, errs.ErrValidation)
	}

	body, err := os.ReadFile(filepath.Join(checkout, source)) //nolint:gosec // G304: a fixed file under the demo checkout the harness itself chose.
	if err != nil {
		return fmt.Errorf("%s: the demo has no %s, so %w: %w", TrialLifecycle, source, errNoDevelopmentRoute, errs.ErrValidation)
	}

	if _, _, err = serviceSnapshotDeclarationOverlay(string(body)); err != nil {
		return fmt.Errorf("%s: %s is not a service snapshot's declaration, so the demo has no development route: %w: %w", TrialLifecycle, source, err, errs.ErrValidation)
	}

	return nil
}
