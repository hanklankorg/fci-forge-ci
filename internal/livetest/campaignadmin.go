// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/enforcement"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// campaignAdminConsumers are the orchestrators of a campaign provider whose
// declared inputs an administrator setting reaches, as globs under the
// checkout: Forgejo's reusable workflows and actions, GitLab's components.
var campaignAdminConsumers = map[string][]string{
	campaignForgejo: {".forgejo/workflows/*.yml", ".forgejo/actions/*/action.yml"},
	campaignGitLab:  {"templates/*.yml"},
}

// campaignAdminSetting is the cases-file declaration of a check's
// administrator setting, admin-<check>.
func campaignAdminSetting(check enforcement.Check) string { return "admin-" + string(check) }

// adminSettingsConsumed returns the administrator settings whose input,
// enforce-<check>, at least one orchestrator of provider under root
// declares, read from the declarations as the administrator wrapper reads
// them. A setting outside it describes a run fact nothing on the lane
// consumes.
func adminSettingsConsumed(root, provider string) (map[string]bool, error) {
	consumed := map[string]bool{}
	files := 0

	read := func(file string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, file)) //nolint:gosec // This checkout's orchestrators, read as data.
	}

	for _, pattern := range campaignAdminConsumers[provider] {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			files++

			file, err := filepath.Rel(root, match)
			if err != nil {
				return nil, err
			}

			declared, err := declaredInputs(read, file)
			if err != nil {
				return nil, err
			}

			for _, check := range enforcement.Checks {
				if gitlabMappingValue(declared, enforcement.Setting(check)) != nil {
					consumed[campaignAdminSetting(check)] = true
				}
			}
		}
	}

	if files == 0 {
		return nil, fmt.Errorf("no %s orchestrator declares inputs under %s: %w", provider, root, errs.ErrValidation)
	}

	return consumed, nil
}

// readLaneCases reads the cases file of a provider's lane, as preparation and
// the lane both read it: its administrator declarations must name inputs an
// orchestrator of the provider under root consumes.
func readLaneCases(root, provider, path string) (campaignCases, error) {
	file, err := readCampaignCases(path)
	if err != nil {
		return campaignCases{}, err
	}

	if err := file.checkAdminSettings(root, provider); err != nil {
		return campaignCases{}, fmt.Errorf("%s: %w", path, err)
	}

	return file, nil
}

// checkAdminSettings refuses a declared administrator setting that no
// orchestrator of the lane's provider under root consumes: the declaration
// would hold the run to a fact nothing on the lane reads. The orchestrators
// are read only when the file declares an administrator setting.
func (file campaignCases) checkAdminSettings(root, provider string) error {
	var consumed map[string]bool

	for _, key := range slices.Sorted(maps.Keys(file.settings)) {
		check, isAdmin := strings.CutPrefix(key, "admin-")
		if !isAdmin {
			continue
		}

		if consumed == nil {
			var err error
			if consumed, err = adminSettingsConsumed(root, provider); err != nil {
				return err
			}
		}

		if !consumed[key] {
			return fmt.Errorf("declared setting %s names %s, an input no %s orchestrator declares, so nothing on this lane consumes it; drop the declaration: %w",
				key, enforcement.Setting(enforcement.Check(check)), provider, errs.ErrValidation)
		}
	}

	return nil
}
