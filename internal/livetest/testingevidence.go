// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SeedTestingEvidence drafts TestingMatricesEvidence lines from the verdicts
// the live runner recorded under liveRuns (<run>/verdict.json): for every
// demo consumer, the latest run of each flow on each provider, passed as ✅,
// failed as ❌, anything else as ⏳, with the run's date as the note. It reads the recorded facts and makes no judgement: the overlay is
// hand-kept, so the operator reviews the draft before committing it.
func SeedTestingEvidence(liveRuns string) (string, error) {
	files, err := filepath.Glob(filepath.Join(liveRuns, "*", "verdict.json"))
	if err != nil {
		return "", err
	}

	latest := map[string]LiveRunVerdict{}

	for _, file := range files {
		body, err := os.ReadFile(file) //nolint:gosec // G304: the operator's own live-run store, named by the recipe.
		if err != nil {
			return "", err
		}

		var verdict LiveRunVerdict
		if err := json.Unmarshal(body, &verdict); err != nil {
			return "", fmt.Errorf("%s: %w", file, err)
		}

		key, flow := evidenceKey(verdict.Run)
		if key == "" || latest[key].Run.StartedAt > verdict.Run.StartedAt {
			continue
		}

		latest[key+" "+flow] = verdict
		delete(latest, key)
	}

	lines := []string{}

	for key, verdict := range latest {
		lines = append(lines, key+" "+evidenceMark(verdict.Verdict)+" "+verdict.Run.StartedAt[:len("2006-01-02")])
	}

	slices.Sort(lines)

	return strings.Join(lines, "\n") + "\n", nil
}

// evidenceKey names the overlay row a run is evidence for, "<provider>
// <demo>" and its flow, or nothing for a run of no demo consumer: the demo
// is the consumer path's test/live/demos/<demo>/consumer, the flow the
// recorded event or, where the scenario is the flow, the scenario's.
func evidenceKey(run LiveRunHeader) (string, string) {
	_, path, found := strings.Cut(filepath.ToSlash(run.Inputs["forge_ci_live_consumer"]), "/demos/")

	demo := strings.TrimSuffix(path, "/consumer")
	if !found || !campaignDemo.MatchString(demo) || !isCampaignProvider(run.Provider) {
		return "", ""
	}

	flow := run.Inputs["forge_ci_live_consumer_event"]
	if scenario, known := map[string]string{
		"^TestGitLabLayer_ConsumerTrial$": TrialPullRequest, "^TestGitLabRelease_ConsumerTrial$": TrialRelease,
		"^TestGitLabLayer_ConsumerLifecycle$": TrialLifecycle, "^TestForgejoLayer_ConsumerLifecycle$": TrialLifecycle,
	}[run.Scenario]; known {
		flow = scenario
	}

	if flow == "" {
		return "", ""
	}

	return run.Provider + " " + demo, flow
}

func evidenceMark(verdict string) string {
	switch verdict {
	case verdictPassed:
		return "✅"
	case liveVerdictFailed:
		return "❌"
	default:
		return "⏳"
	}
}
