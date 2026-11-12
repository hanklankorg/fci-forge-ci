// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package ci

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// fakeCellsGo stands for `go run …/preflight github-cells`: the cells a test
// gives, or Go's refusal.
const fakeCellsGo = `#!/usr/bin/env bash
if [[ -n "${FAKE_CELLS_REFUSED:-}" ]]; then printf 'github-cells: refused\n' >&2; exit 1; fi
printf '%s\n' "$*${FORGE_CI_LIVE_AFTER:+ (after $FORGE_CI_LIVE_AFTER)}" >>"$FAKE_CALLS"
printf '%s\n' "$FAKE_CELLS"
`

// fakeCellsJust stands for `just test-live-consumer github`: it records the
// engine mode, the visibility and its arguments, and ends as the demo's name
// says once the engine is prepared.
const fakeCellsJust = `#!/usr/bin/env bash
printf '%s|%s|%s\n' "${FORGE_CI_LIVE_ENGINE_MODE:-}" "${FORGE_CI_LIVE_CONSUMER_VISIBILITY:-}" "$*" >>"$FAKE_CALLS"
if [[ "${FORGE_CI_LIVE_ENGINE_MODE:-}" == prepare ]]; then
  [[ -z "${FAKE_PREPARE_FAILS:-}" ]] || exit 1
  printf -- '--- SKIP: TestGitHubLayer_ConsumerTrial (1.00s)\n'
  exit 0
fi
case "$*" in
*hello-fails*) printf -- '--- FAIL: TestGitHubLayer_ConsumerTrial (1.00s)\n'; exit 1 ;;
*hello-skips*) printf -- '--- SKIP: TestGitHubLayer_ConsumerTrial (1.00s)\n' ;;
*) printf -- '--- PASS: TestGitHubLayer_ConsumerLifecycle (1.00s)\n' ;;
esac
`

// medium is the run set and setting the tests run the script with.
var medium = []string{"set=medium", "nanolinter-image=lint"}

// runGitHubCells runs the script on a prepared directory with the fakes and
// args after the organization, and returns its output, its exit status, the
// fakes' calls and cells.txt.
func runGitHubCells(t *testing.T, args []string, env ...string) (string, int, []string, string) {
	t.Helper()

	dir := t.TempDir()
	prepared, calls, evidence := filepath.Join(dir, "prepared"), filepath.Join(dir, "calls"), filepath.Join(dir, "evidence")

	if err := os.MkdirAll(filepath.Join(prepared, "demos"), 0o700); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{"go": fakeCellsGo, "just": fakeCellsJust} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(t.Context(), liveCommandBound)
	defer cancel()

	command := exec.CommandContext(ctx, "bash", append([]string{filepath.Join(reporoot.Path(t), "scripts", "ci", "run-github-cells.sh"), prepared, "4242"}, args...)...)
	command.Env = append(os.Environ(), append(env, "GO="+filepath.Join(dir, "go"), "JUST="+filepath.Join(dir, "just"),
		"FAKE_CALLS="+calls, "FORGE_CI_LIVE_EVIDENCE_ROOT="+evidence)...)

	output, err := command.CombinedOutput()

	status := 0

	if exit, exited := errors.AsType[*exec.ExitError](err); exited {
		status = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}

	recorded, _ := os.ReadFile(calls)
	verdicts, _ := filepath.Glob(filepath.Join(evidence, "*", "cells.txt"))

	var cells []byte
	if len(verdicts) == 1 {
		cells, _ = os.ReadFile(verdicts[0])
	}

	return strings.ReplaceAll(string(output), dir, ""), status, strings.Split(strings.TrimSpace(strings.ReplaceAll(string(recorded), dir, "")), "\n"), strings.ReplaceAll(string(cells), dir, "")
}

// The engine is prepared once, with the first cell and without running it;
// every cell then reuses it, with its own visibility and settings, one after
// another. A cell that fails, or that judged nothing, fails the run after
// the cells behind it ran, and each has its line and its log.
func TestRunGitHubCells_PreparesOnceThenRunsEachCell(t *testing.T) {
	t.Parallel()

	cells := "private\thello-one\tlifecycle\tversion=0.2.0 nanolinter-image=lint\n" +
		"both\thello-fails\tpull-request\tnanolinter-image=lint\n" +
		"both\thello-skips\tpull-request\tnanolinter-image=lint\n" +
		"public\thello-two\trelease\tnanolinter-image=lint"

	// The caller's visibility is not a cell's: "both" runs with none set.
	output, status, calls, verdicts := runGitHubCells(t, medium, "FAKE_CELLS="+cells, "FORGE_CI_LIVE_CONSUMER_VISIBILITY=private")

	const consumer = "--justfile " // then the repository's justfile

	want := []string{
		"run ./test/live/internal/livetest/preflight github-cells medium nanolinter-image=lint",
		"prepare|private|test-live-consumer github /prepared/demos/hello-one lifecycle org=4242 version=0.2.0 nanolinter-image=lint",
		"reuse|private|test-live-consumer github /prepared/demos/hello-one lifecycle org=4242 version=0.2.0 nanolinter-image=lint",
		"reuse||test-live-consumer github /prepared/demos/hello-fails pull-request org=4242 nanolinter-image=lint",
		"reuse||test-live-consumer github /prepared/demos/hello-skips pull-request org=4242 nanolinter-image=lint",
		"reuse|public|test-live-consumer github /prepared/demos/hello-two release org=4242 nanolinter-image=lint",
	}

	if len(calls) != len(want) {
		t.Fatalf("calls\n%s\nwant\n%s\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"), output)
	}

	for i, call := range calls {
		mode, rest, _ := strings.Cut(want[i], "test-live-consumer")
		if !strings.HasPrefix(call, mode) || (i > 0 && !strings.Contains(call, consumer)) || !strings.HasSuffix(call, rest) {
			t.Errorf("call %d was %q, want %q", i, call, want[i])
		}
	}

	if status != 1 {
		t.Errorf("status %d, want 1 for a failed cell\n%s", status, output)
	}

	header, verdicts, _ := strings.Cut(verdicts, "\n")
	if header != "forge_ci_live_set=medium" {
		t.Errorf("cells.txt starts %q, want the run set", header)
	}

	lines := strings.Split(strings.TrimSpace(verdicts), "\n")
	for i, prefix := range []string{
		"FORGE_CI_LIVE_CELL passed hello-one lifecycle private version=0.2.0 nanolinter-image=lint ",
		"FORGE_CI_LIVE_CELL failed hello-fails pull-request both nanolinter-image=lint ",
		"FORGE_CI_LIVE_CELL not-covered hello-skips pull-request both nanolinter-image=lint ",
		"FORGE_CI_LIVE_CELL passed hello-two release public nanolinter-image=lint ",
	} {
		if len(lines) != 4 || !strings.HasPrefix(lines[i], prefix) || !strings.HasSuffix(lines[i], ".log") || !strings.Contains(output, lines[i]) {
			t.Fatalf("cells.txt\n%s\nwant line %d to start %q and be printed\n%s", verdicts, i, prefix, output)
		}
	}

	if output, status, _, _ := runGitHubCells(t, medium, "FAKE_CELLS=public\thello-two\trelease\t"); status != 0 {
		t.Errorf("a run whose cells all passed: status %d\n%s", status, output)
	}
}

// A delta run hands Go its set after, which cells.txt records beside the
// run set; the word is not a setting of the cells.
func TestRunGitHubCells_ADeltaRunRecordsItsSetAfter(t *testing.T) {
	t.Parallel()

	output, status, calls, verdicts := runGitHubCells(t, []string{"set=full", "after=medium", "nanolinter-image=lint"},
		"FAKE_CELLS=public\thello-two\trelease\tnanolinter-image=lint")
	if status != 0 || calls[0] != "run ./test/live/internal/livetest/preflight github-cells full nanolinter-image=lint (after medium)" ||
		!strings.HasPrefix(verdicts, "forge_ci_live_set=full\nforge_ci_live_after=medium\nFORGE_CI_LIVE_CELL passed hello-two ") {
		t.Errorf("status %d, calls %q, cells.txt\n%s\n%s", status, calls, verdicts, output)
	}
}

// What Go refuses, as a release with no credential named, runs nothing; and
// so does a set whose engine could not be prepared.
func TestRunGitHubCells_RunsNothingAfterARefusalOrAFailedPrepare(t *testing.T) {
	t.Parallel()

	if output, status, calls, _ := runGitHubCells(t, medium, "FAKE_CELLS_REFUSED=1"); status != 2 || calls[0] != "" {
		t.Errorf("a refused set: status %d, calls %q\n%s", status, calls, output)
	}

	output, status, calls, verdicts := runGitHubCells(t, medium, "FAKE_CELLS=both\thello-one\tpull-request\t", "FAKE_PREPARE_FAILS=1")
	if status != 1 || len(calls) != 2 || verdicts != "forge_ci_live_set=medium\n" || !strings.Contains(output, "no cell ran") {
		t.Errorf("a failed prepare: status %d, calls %q, cells %q\n%s", status, calls, verdicts, output)
	}
}
