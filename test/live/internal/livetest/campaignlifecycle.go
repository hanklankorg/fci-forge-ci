// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
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

// campaignSupportsSnapshot reports whether a demo runs the snapshot event on
// a provider: whether it has the development route for every image the
// planner plans from its declaration there.
func campaignSupportsSnapshot(provider, checkout string) bool {
	return campaignAdmitSnapshot(provider, checkout) == nil
}

// campaignAdmitSnapshot refuses a snapshot case whose demo has no
// development route over every planned image: the declaration its release
// builds its images from (ServiceSnapshotSource), which the planner must plan
// in the snapshot's scope (snapshotImagesScope), on Forgejo through the
// harness's service development workflow, which is pinned as the demo's
// Forgejo release workflow pins the layer, on GitLab through the component,
// which needs nothing of the demo's but the declaration. The snapshot
// releases nothing, so no release build is asked for.
func campaignAdmitSnapshot(provider, checkout string) error {
	if !isCampaignProvider(provider) {
		return fmt.Errorf("%s runs on %s and %s: %w", TrialSnapshot, campaignForgejo, campaignGitLab, errs.ErrValidation)
	}

	if _, err := snapshotImagesAdmission(checkout); err != nil {
		return err
	}

	if provider != campaignForgejo {
		return nil
	}

	if present, err := exists(filepath.Join(checkout, filepath.FromSlash(forgejoReleaseWorkflow))); err != nil || !present {
		return fmt.Errorf("%s: the demo has no %s to pin the harness's development workflow by, so %w: %w", TrialSnapshot, forgejoReleaseWorkflow, errNoDevelopmentRoute, errs.ErrValidation)
	}

	return nil
}

// snapshotImagesAdmission plans the demo's source declaration in the
// snapshot's scope and returns the plan; the planner's or the scope's
// refusal is the demo's.
func snapshotImagesAdmission(checkout string) (snapshotimage.Plan, error) {
	source, err := ServiceSnapshotSource(checkout)
	if err != nil {
		return snapshotimage.Plan{}, err
	}

	body, err := os.ReadFile(filepath.Join(checkout, source)) //nolint:gosec // G304: a fixed file under the demo checkout the harness itself chose.
	if err != nil {
		return snapshotimage.Plan{}, fmt.Errorf("%s: the demo has no %s, so %w: %w", TrialSnapshot, source, errNoDevelopmentRoute, errs.ErrValidation)
	}

	_, plan, err := snapshotDeclarationOverlay(string(body), snapshotImagesScope)
	if err != nil {
		return snapshotimage.Plan{}, fmt.Errorf("%s: %s is not a snapshot's declaration, so the demo has no development route: %w: %w", TrialSnapshot, source, err, errs.ErrValidation)
	}

	return plan, nil
}
