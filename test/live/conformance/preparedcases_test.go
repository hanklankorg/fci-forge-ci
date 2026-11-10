// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/test/live/internal/livetest"
)

// What the lifecycle's service readiness reads: where the published image
// answers, and the fixture a service needs beside it, if any.
const (
	serviceReadinessURLEnv = "FORGE_CI_LIVE_SERVICE_READINESS_URL"
	serviceFixtureEnv      = "FORGE_CI_LIVE_SERVICE_FIXTURE"
)

// forEachConsumerCheckout runs trial on the consumer checkouts the run names.
// A checkout named by FORGE_CI_LIVE_CONSUMER is the only one, with the event
// and settings of the caller's environment. Otherwise the prepared directory
// (livetest.PreparedEnv) gives one per case of the forge's table
// (livetest.PreparedCases) with one of events, each the subtest the case
// names, on the demo's checkout there. The subtest's environment is what
// `just test-live-consumer <forge>` hands a trial: the checkout, the event
// and the case's settings, with the prepared images and the case's builder.
// With neither, the test is skipped with usage.
func forEachConsumerCheckout(t *testing.T, forge provider.ForgeAPI, usage string, trial func(t *testing.T, root string), events ...string) {
	t.Helper()

	if root := os.Getenv(consumerCheckoutEnv); root != "" {
		trial(t, root)

		return
	}

	prepared := os.Getenv(livetest.PreparedEnv)
	if prepared == "" {
		t.Skip("no consumer checkout is named; run `" + usage + "`, or set " + livetest.PreparedEnv + " to run the prepared demos")
	}

	env, err := livetest.ReadPreparedEnvFile(filepath.Join(prepared, livetest.PreparedEnvFile))
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range livetest.PreparedCases(string(forge)) {
		if !slices.Contains(events, c.Event) {
			continue
		}

		t.Run(c.Name(), func(t *testing.T) {
			root := filepath.Join(prepared, "demos", c.Demo)

			settings, environment, err := preparedCaseInputs(forge, c, env)
			if err == nil && forge == provider.ForgeGitLab {
				err = c.RequireGitLabFurtherArchives(root, env)
			}

			if err != nil {
				t.Fatal(err)
			}

			t.Setenv(consumerCheckoutEnv, root)
			t.Setenv(consumerEventEnv, c.Event)
			t.Setenv(consumerSettingsEnv, strings.Join(settings, "\n"))
			t.Setenv(livetest.ImageBuilderEnv(forge), c.Builder)

			for name, value := range environment {
				t.Setenv(name, value)
			}

			for name, value := range c.LaneEnvironment(string(forge)) {
				t.Setenv(name, value)
			}

			trial(t, root)
		})
	}
}

// preparedCaseInputs are the settings and the environment of a case on forge.
func preparedCaseInputs(forge provider.ForgeAPI, c livetest.PreparedCase, env map[string]string) ([]string, map[string]string, error) {
	if forge == provider.ForgeGitLab {
		environment, err := c.GitLabEnvironment(env)

		return c.GitLabSettings(), environment, err
	}

	settings, err := c.Settings(env)
	environment := livetest.PreparedForgejoEnvironment(env)

	// A shipped demo's service answers here and takes no fixture, as
	// `just test-live-consumer forgejo <checkout> lifecycle` sets.
	if c.Event == livetest.TrialLifecycle {
		environment[serviceReadinessURLEnv] = "http://127.0.0.1:8080/health"
		environment[serviceFixtureEnv] = ""
	}

	return settings, environment, err
}
