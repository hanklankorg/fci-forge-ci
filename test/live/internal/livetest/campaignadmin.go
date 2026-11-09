// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
// would hold the run to a fact nothing on the lane reads.
func (file campaignCases) checkAdminSettings(root, provider string) error {
	declared := []string{}

	for _, key := range slices.Sorted(maps.Keys(file.settings)) {
		if strings.HasPrefix(key, "admin-") {
			declared = append(declared, key)
		}
	}

	return refuseUnconsumedAdmin(root, provider, declared)
}

// enforcedAdminSettings are the administrator settings a run enforces,
// sorted.
func enforcedAdminSettings(settings map[string]string) []string {
	enforced := []string{}

	for _, check := range enforcement.Checks {
		if key := campaignAdminSetting(check); settings[key] == strconv.FormatBool(true) {
			enforced = append(enforced, key)
		}
	}

	return enforced
}

// refuseUnconsumedAdmin refuses an administrator setting, declared by a
// cases file or enforced by the run, that no orchestrator of the provider
// under root consumes: nothing on the lane would read it, and the run's
// record would claim a check that never ran. The orchestrators are read only
// when there is a setting to judge.
func refuseUnconsumedAdmin(root, provider string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	consumed, err := adminSettingsConsumed(root, provider)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if !consumed[key] {
			check, _ := strings.CutPrefix(key, "admin-")

			return fmt.Errorf("setting %s names %s, an input no %s orchestrator declares, so nothing on this lane consumes it; drop it: %w",
				key, enforcement.Setting(enforcement.Check(check)), provider, errs.ErrValidation)
		}
	}

	return nil
}
