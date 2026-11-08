// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// SeedTestingEvidence drafts TestingMatricesEvidence lines from the runs
// recorded under liveRuns, the live runner's <run>/verdict.json, and, when
// hostedRuns is named, under that root of hosted GitHub trials, one
// <trial>/controller.sh with its controller.exit and finished-at each. For
// every demo consumer it keeps the latest run of each flow on each provider:
// passed as ✅, failed as ❌, anything else as ⏳, with the run's date as the
// note. It reads the recorded facts and makes no judgement: the overlay is
// hand-kept, so the operator reviews the draft before committing it.
func SeedTestingEvidence(liveRuns, hostedRuns string) (string, error) {
	latest := map[string]seededRun{}

	if err := seedLiveRuns(liveRuns, latest); err != nil {
		return "", err
	}

	if hostedRuns != "" {
		if err := seedHostedRuns(hostedRuns, latest); err != nil {
			return "", err
		}
	}

	lines := make([]string, 0, len(latest))

	for key, run := range latest {
		lines = append(lines, key+" "+evidenceMark(run.verdict)+" "+run.date())
	}

	slices.Sort(lines)

	return strings.Join(lines, "\n") + "\n", nil
}

// seededRun is one recorded run: when it started or finished, and its
// verdict.
type seededRun struct{ at, verdict string }

func (run seededRun) date() string {
	return run.at[:min(len(run.at), len("2006-01-02"))]
}

// keepLatest records run under key unless a later run of it is recorded.
func keepLatest(latest map[string]seededRun, key string, run seededRun) {
	if key != "" && latest[key].at <= run.at {
		latest[key] = run
	}
}

func seedLiveRuns(liveRuns string, latest map[string]seededRun) error {
	files, err := filepath.Glob(filepath.Join(liveRuns, "*", "verdict.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		body, err := os.ReadFile(file) //nolint:gosec // G304: the operator's own live-run store, named by the recipe.
		if err != nil {
			return err
		}

		var verdict LiveRunVerdict
		if err := json.Unmarshal(body, &verdict); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}

		keepLatest(latest, evidenceKey(verdict.Run), seededRun{at: verdict.Run.StartedAt, verdict: verdict.Verdict})
	}

	return nil
}

// hostedTrial is the hosted controller's consumer trial: the recipe, the
// GitHub repository ID, the consumer checkout and the event.
var hostedTrial = regexp.MustCompile(`test-live-github-consumer\s+\S+\s+(\S+)\s+(\S+)`)

// seedHostedRuns reads the hosted trials under root. A trial without an exit
// receipt has not finished and is left out; exit 0 is a pass, any other a
// failure.
func seedHostedRuns(root string, latest map[string]seededRun) error {
	scripts, err := filepath.Glob(filepath.Join(root, "*", "controller.sh"))
	if err != nil {
		return err
	}

	for _, script := range scripts {
		dir := filepath.Dir(script)

		exit, err := os.ReadFile(filepath.Join(dir, "controller.exit")) //nolint:gosec // G304: the operator's own trial records, named by the recipe.
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return err
		}

		finished, err := os.ReadFile(filepath.Join(dir, "finished-at")) //nolint:gosec // G304: as above.
		if err != nil {
			return err
		}

		body, err := os.ReadFile(script) //nolint:gosec // G304: as above.
		if err != nil {
			return err
		}

		verdict := liveVerdictFailed
		if strings.TrimSpace(string(exit)) == "0" {
			verdict = verdictPassed
		}

		keepLatest(latest, hostedEvidenceKey(string(body)), seededRun{at: strings.TrimSpace(string(finished)), verdict: verdict})
	}

	return nil
}

// hostedEvidenceKey names the overlay row a hosted controller's trial is
// evidence for: the demo is the consumer checkout's directory name, and a
// dev-release is the development flow.
func hostedEvidenceKey(script string) string {
	match := hostedTrial.FindStringSubmatch(strings.ReplaceAll(script, "\\\n", " "))
	if match == nil {
		return ""
	}

	demo, flow := path.Base(filepath.ToSlash(match[1])), match[2]
	if flow == TrialDevRelease {
		flow = matrixDevelopment
	}

	if !campaignDemo.MatchString(demo) || !slices.Contains([]string{TrialPullRequest, TrialRelease, matrixDevelopment, TrialLifecycle}, flow) {
		return ""
	}

	return matrixGitHub + " " + demo + " " + flow
}

// evidenceKey names the overlay row a run is evidence for, "<provider>
// <demo> <flow>", or nothing for a run of no demo consumer: the demo
// is the consumer path's test/live/demos/<demo>/consumer, the flow the
// recorded event or, where the scenario is the flow, the scenario's.
func evidenceKey(run LiveRunHeader) string {
	_, path, found := strings.Cut(filepath.ToSlash(run.Inputs["forge_ci_live_consumer"]), "/demos/")

	demo := strings.TrimSuffix(path, "/consumer")
	if !found || !campaignDemo.MatchString(demo) || !isCampaignProvider(run.Provider) {
		return ""
	}

	flow := run.Inputs["forge_ci_live_consumer_event"]
	if scenario, known := map[string]string{
		"^TestGitLabLayer_ConsumerTrial$": TrialPullRequest, "^TestGitLabRelease_ConsumerTrial$": TrialRelease,
		"^TestGitLabLayer_ConsumerLifecycle$": TrialLifecycle, "^TestForgejoLayer_ConsumerLifecycle$": TrialLifecycle,
	}[run.Scenario]; known {
		flow = scenario
	}

	if flow == "" {
		return ""
	}

	return run.Provider + " " + demo + " " + flow
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
