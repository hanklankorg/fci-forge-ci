// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
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
	"time"

	"github.com/diggsweden/forge-ci/internal/domain/enforcement"
)

// SeedTestingEvidence drafts TestingMatricesEvidence lines from the runs
// recorded under liveRuns, the live runner's <run>/verdict.json, and, when
// hostedRuns is named, under that root of hosted GitHub trials, one
// <trial>/controller.sh with its controller.exit and finished-at each. For
// every demo consumer it keeps the latest run of each flow on each provider:
// passed as ✅, failed as ❌, anything else as ⏳, with the run's date as the
// note; a lifecycle run is kept for each set of the axis values its verdict
// header records, which lead the note (evidenceAxes). It reads the recorded
// facts and makes no judgement: the overlay is hand-kept, so the operator
// reviews the draft before committing it.
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
		// The axis values follow the flow in the key and the mark in the line.
		words := strings.Fields(key)
		lines = append(lines, strings.Join(slices.Concat(words[:3], []string{evidenceMark(run.verdict)}, words[3:], []string{run.date()}), " "))
	}

	slices.Sort(lines)

	return strings.Join(lines, "\n") + "\n", nil
}

// seededRun is one recorded run: when it started or finished, and its
// verdict.
type seededRun struct {
	at      time.Time
	verdict string
}

func (run seededRun) date() string {
	return run.at.Format(time.DateOnly)
}

// newSeededRun reads a run's recorded RFC 3339 time.
func newSeededRun(at, verdict, record string) (seededRun, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(at))
	if err != nil {
		return seededRun{}, fmt.Errorf("%s: %w", record, err)
	}

	return seededRun{at: parsed, verdict: verdict}, nil
}

// keepLatest records run under key unless a later run of it is recorded.
func keepLatest(latest map[string]seededRun, key string, run seededRun) {
	if key != "" && !latest[key].at.After(run.at) {
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

		key := evidenceKey(verdict.Run)
		if key == "" {
			continue
		}

		if strings.HasSuffix(key, " "+TrialLifecycle) {
			key = strings.TrimSpace(key + " " + evidenceAxes(verdict.Run))
		}

		run, err := newSeededRun(verdict.Run.StartedAt, verdict.Verdict, file)
		if err != nil {
			return err
		}

		keepLatest(latest, key, run)
	}

	return nil
}

// hostedTrial is a hosted consumer trial in a controller or batch script:
// the recipe, the GitHub repository ID, the consumer checkout and the event.
var hostedTrial = regexp.MustCompile(`test-live-github-consumer\s+\S+\s+(\S+)\s+(\S+)`)

// hostedReceipt is a file the script writes into its evidence directory: a
// case's exit receipt or its finish time.
var hostedReceipt = regexp.MustCompile(`"\$evidence/([A-Za-z0-9._-]*(?:\.exit|finished-at))"`)

// seedHostedRuns reads the hosted trials under root, every trial line of
// every <trial>/*.sh. A trial's exit receipt is the first one its script
// writes after it, as a batch writes each case's, or else the last one
// before it, as a controller's exit trap does; its finish time is the first
// one written after that receipt. A trial whose receipt is missing has not
// finished and is left out; exit 0 is a pass, any other a failure.
func seedHostedRuns(root string, latest map[string]seededRun) error {
	scripts, err := filepath.Glob(filepath.Join(root, "*", "*.sh"))
	if err != nil {
		return err
	}

	for _, script := range scripts {
		body, err := os.ReadFile(script) //nolint:gosec // G304: the operator's own trial records, named by the recipe.
		if err != nil {
			return err
		}

		text := strings.ReplaceAll(string(body), "\\\n", " ")
		for _, trial := range hostedTrial.FindAllStringSubmatchIndex(text, -1) {
			run, finished, err := hostedTrialRun(filepath.Dir(script), text, trial[1])
			if err != nil {
				return err
			}

			if finished {
				keepLatest(latest, hostedEvidenceKey(text, text[trial[2]:trial[3]], text[trial[4]:trial[5]]), run)
			}
		}
	}

	return nil
}

// hostedTrialRun reads the receipts of the trial whose line ends at end.
func hostedTrialRun(dir, script string, end int) (seededRun, bool, error) {
	receipts := hostedReceipt.FindAllStringSubmatchIndex(script, -1)
	exit, finish := -1, ""

	for i, receipt := range receipts {
		if strings.HasSuffix(script[receipt[2]:receipt[3]], ".exit") && (exit < 0 || receipts[exit][0] < end) {
			exit = i
		}
	}

	if exit < 0 {
		return seededRun{}, false, nil
	}

	for _, receipt := range receipts[exit+1:] {
		if name := script[receipt[2]:receipt[3]]; strings.HasSuffix(name, "finished-at") {
			finish = name

			break
		}
	}

	code, err := os.ReadFile(filepath.Join(dir, script[receipts[exit][2]:receipts[exit][3]])) //nolint:gosec // G304: a receipt the trial's own script names.
	if errors.Is(err, fs.ErrNotExist) || finish == "" {
		return seededRun{}, false, nil
	}

	if err != nil {
		return seededRun{}, false, err
	}

	finished, err := os.ReadFile(filepath.Join(dir, finish)) //nolint:gosec // G304: as above.
	if err != nil {
		return seededRun{}, false, err
	}

	verdict := liveVerdictFailed
	if strings.TrimSpace(string(code)) == "0" {
		verdict = verdictPassed
	}

	run, err := newSeededRun(string(finished), verdict, filepath.Join(dir, finish))

	return run, err == nil, err
}

// hostedEvidenceKey names the overlay row a hosted trial is evidence for: the
// demo is the consumer checkout's directory name, read through the script's
// own assignment where the line names a variable, and a dev-release is the
// development flow.
func hostedEvidenceKey(script, consumer, flow string) string {
	consumer = strings.Trim(consumer, `"'`)
	if name, isVariable := strings.CutPrefix(consumer, "$"); isVariable {
		assignment := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(strings.Trim(name, "{}")) + `=(\S+)$`).FindStringSubmatch(script)
		if assignment == nil {
			return ""
		}

		consumer = strings.Trim(assignment[1], `"'`)
	}

	demo := path.Base(filepath.ToSlash(consumer))

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
		// A snapshot is the development route alone, the matrices' snapshot flow.
		"^TestGitLabLayer_ServiceSnapshot$": matrixDevelopment, "^TestForgejoLayer_ServiceSnapshot$": matrixDevelopment,
	}[run.Scenario]; known {
		flow = scenario
	}

	if flow == "" {
		return ""
	}

	return run.Provider + " " + demo + " " + flow
}

// evidenceAxes are the axis values a run's verdict header records, as the
// overlay qualifies a mark with them: the visibility (both where none was
// selected), the image builder, the Forgejo case's own CI_IMAGE_BUILDER or
// the lane's selector, and each administrator input. An axis the header does
// not record is left out, so the matrices claim no value for it.
func evidenceAxes(run LiveRunHeader) string {
	input := func(env string) string { return run.Inputs[strings.ToLower(env)] }

	axes := []string{settingVisibility + "=" + cmp.Or(input(consumerVisibilityEnv), visibilityBoth)}

	variables := map[string]string{}

	for line := range strings.SplitSeq(input("FORGE_CI_LIVE_CONSUMER_SETTINGS"), "\n") {
		if key, value, isVariable := strings.Cut(line, "="); isVariable {
			variables[key] = value
		}
	}

	builder, err := "", error(nil)

	switch run.Provider {
	case campaignForgejo:
		builder, err = ForgejoConsumerImageBuilder(variables, input(ForgejoImageBuilderEnv))
	case campaignGitLab:
		builder, _ = imageBuilderValue(input(GitLabImageBuilderEnv))
	}

	if err == nil && builder != "" {
		axes = append(axes, settingImageBuilder+"="+builder)
	}

	for _, check := range enforcement.Checks {
		if value := input(adminInputEnvironment(check)); value != "" {
			axes = append(axes, campaignAdminSetting(check)+"="+value)
		}
	}

	return strings.Join(axes, " ")
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
