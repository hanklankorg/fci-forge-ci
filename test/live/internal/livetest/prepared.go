// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// PreparedEnv names the directory `just campaign-prepare` wrote: the images
// and archives built once from one commit, which a run then takes by digest.
const PreparedEnv = "FORGE_CI_LIVE_PREPARED"

// CheckPreparedSource refuses a run that would test prepared images against
// other source than they were built from. head is the checkout's commit and
// changes its `git status --porcelain`: the prepared commit must be head and
// the tree must hold nothing else, or the run would carry that commit's name
// over different files.
func CheckPreparedSource(dir, head, changes string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%s must be absolute, got %q: %w", PreparedEnv, dir, errs.ErrValidation)
	}

	env, err := ReadCampaignEnv(filepath.Join(dir, CampaignEnvFile))
	if err != nil {
		return err
	}

	if env[campaignCommitKey] != head {
		return fmt.Errorf("%s was prepared for %s and this checkout is %s; check that commit out or prepare again: %w",
			dir, env[campaignCommitKey], head, errs.ErrValidation)
	}

	if changed := strings.TrimSpace(changes); changed != "" {
		first, _, _ := strings.Cut(changed, "\n")

		return fmt.Errorf("%s was prepared for %s and this checkout has changes on top of it (%s); commit or remove them, then prepare again: %w",
			dir, head, strings.TrimSpace(first), errs.ErrValidation)
	}

	return nil
}
