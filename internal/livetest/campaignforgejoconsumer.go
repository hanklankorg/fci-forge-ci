// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// Keep the existing native consumer scenario, but give saved campaigns the same
// explicit-checkout route as GitLab. Packaging remains consumer-owned.
func campaignForgejoScenarioArgs(provider string, env map[string]string, words []string) ([]string, bool, error) {
	if len(words) < 2 || words[1] != "TestForgejoLayer_ConsumerTrial" {
		return nil, false, nil
	}

	if provider != campaignForgejo {
		return nil, true, fmt.Errorf("this scenario requires a Forgejo lane: %w", errs.ErrValidation)
	}

	// Preserve bare suite selection. Explicit inputs default to a release.
	if len(words) == 2 {
		return nil, false, nil
	}

	archives, settings, err := campaignSettings(provider, env, words[2:])
	if err != nil {
		return nil, true, err
	}

	checkout, remaining, err := campaignScenarioCheckout(settings)
	if err != nil {
		return nil, true, err
	}

	event := TrialRelease

	var trialWords []string

	eventNamed := false

	for _, word := range remaining {
		key, value, _ := strings.Cut(word, "=")
		if key != "event" {
			trialWords = append(trialWords, word)

			continue
		}

		if eventNamed || value != TrialPullRequest && value != TrialRelease {
			return nil, true, fmt.Errorf("event= must name pull-request or release once: %w", errs.ErrValidation)
		}

		event, eventNamed = value, true
	}

	if _, err = ParseTrialSettings(event, trialWords); err != nil {
		return nil, true, err
	}

	args, err := consumerRecipe(provider, event, checkout, archives, trialWords)

	return args, true, err
}
