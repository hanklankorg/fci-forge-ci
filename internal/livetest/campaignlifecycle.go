// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
)

// campaignSupportsLifecycle reports whether a demo runs the lifecycle event on
// a provider: whether it has a development route there. A demo without one is
// refused, not run as a shorter flow.
func campaignSupportsLifecycle(provider, checkout string) bool {
	return campaignAdmitLifecycle(provider, checkout) == nil
}

// campaignAdmitLifecycle refuses a lifecycle case whose demo has no
// development route on the provider, the lifecycle's development phase: a
// service snapshot of the demo's one container, which the snapshot-images
// policy builds from the demo's own declaration. On GitLab the component
// takes the declaration the demo's release builds its images from; on
// Forgejo the route is the Maven service development workflow, so the demo
// is a root Maven application. The case's settings are held to the
// lifecycle's rules where every case's settings are: ParseTrialSettings on
// Forgejo, ParseGitLabLifecycleSettings in the GitLab admission.
func campaignAdmitLifecycle(provider, checkout string) error {
	if !isCampaignProvider(provider) {
		return fmt.Errorf("%s runs on %s and %s: %w", TrialLifecycle, campaignForgejo, campaignGitLab, errs.ErrValidation)
	}

	source := GitLabDefaultArtifactsConfig

	if provider == campaignGitLab {
		var err error
		if source, err = ServiceSnapshotSource(checkout); err != nil {
			return err
		}
	}

	plan, err := serviceSnapshotAdmission(checkout, source)
	if err != nil || provider != campaignForgejo {
		return err
	}

	if present, err := exists(filepath.Join(checkout, "pom.xml")); err != nil || !present {
		return fmt.Errorf("%s on %s runs the Maven service development workflow, the one Forgejo development route, and the demo has no root pom.xml: %w", TrialLifecycle, campaignForgejo, errs.ErrValidation)
	}

	if err := mavenServiceProducers(plan); err != nil {
		return fmt.Errorf("%s on %s: %w: %w", TrialLifecycle, campaignForgejo, err, errs.ErrValidation)
	}

	return nil
}

// serviceSnapshotAdmission plans the demo's service snapshot from its source
// declaration, the development route every forge's lifecycle shares; a demo
// without a root Containerfile or the declaration has none.
func serviceSnapshotAdmission(checkout, source string) (snapshotimage.Plan, error) {
	if present, err := exists(filepath.Join(checkout, "Containerfile")); err != nil || !present {
		return snapshotimage.Plan{}, fmt.Errorf("%s: the demo has no root Containerfile, so no service snapshot and no development route: %w", TrialLifecycle, errs.ErrValidation)
	}

	body, err := os.ReadFile(filepath.Join(checkout, source)) //nolint:gosec // G304: a fixed file under the demo checkout the harness itself chose.
	if err != nil {
		return snapshotimage.Plan{}, fmt.Errorf("%s: the demo has no %s, so no service snapshot and no development route: %w", TrialLifecycle, source, errs.ErrValidation)
	}

	_, plan, err := serviceSnapshotDeclarationOverlay(string(body))
	if err != nil {
		return snapshotimage.Plan{}, fmt.Errorf("%s: %s is not a service snapshot's declaration, so the demo has no development route: %w: %w", TrialLifecycle, source, err, errs.ErrValidation)
	}

	return plan, nil
}
