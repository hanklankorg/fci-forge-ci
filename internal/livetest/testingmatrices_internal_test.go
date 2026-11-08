// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// The overlay is hand-kept, so a malformed line is refused with its number
// rather than silently dropped from the matrices.
func TestReadMatrixEvidenceRefusesMalformedLines(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"short":    "forgejo hello-maven-service lifecycle\n",
		"provider": "bitbucket hello-maven-service lifecycle ✅\n",
		"twice":    "forgejo hello-maven-service lifecycle ✅\nforgejo hello-maven-service lifecycle ◐ again\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, filepath.Join(t.TempDir(), "evidence.txt"), "# comment\n\n"+body)
			if _, err := readMatrixEvidence(path); err == nil || !strings.Contains(err.Error(), "evidence.txt:") {
				t.Fatalf("malformed overlay: %v, want a refusal naming the line", err)
			}
		})
	}
}

// The matrices are read from the tree, so the shipped tree decides them: the
// service demos have lifecycle lines on both lab forges, the npm service is
// admitted on GitHub without a fixture for its development build, a GitHub
// line is the fixture workflow a demo ships, and a refusal carries the
// planner's or the admission's reason.
func TestRenderTestingMatricesReadsTheShippedTree(t *testing.T) {
	t.Parallel()

	page, err := RenderTestingMatrices(reporoot.Path(t), "<!-- generated:project-kinds -->\n<!-- /generated:project-kinds -->\n<!-- generated:e2e-variants -->\n<!-- /generated:e2e-variants -->\n")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"| hello-maven-service (java · artifact-first · one image) |",
		"| hello-cargo-container (rust · container-first · one image) |",
		"| hello-npm-service (node · artifact-first · one image) |",
		"| 📝 lifecycle / 📝 lifecycle / admitted |",
		"| hello-ios (xcode-ios · no container · no image) | ⬚ / ⬚ / 📝 |",
		"- hello-monorepo, github: the demo has no root Containerfile",
		"| hello-go-cli (go · no container · no image) |",
		"- hello-go-service, gitlab: .forge-ci/artifacts.yml is not a service snapshot's declaration",
		"|  | container-first | 📝 hello-cargo-container | 📝 hello-cargo-container | ⬚ hello-cargo-container (admitted, no line) |",
		"| architecture | amd64 |",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("matrices lack %q:\n%s", want, page)
		}
	}

	if _, err := RenderTestingMatrices(reporoot.Path(t), "no markers"); err == nil {
		t.Fatal("a page without the generated markers was rendered")
	}
}

// The seed keeps each flow's latest run whatever the record order. It reads a
// hosted controller's trial, whose exit trap writes the receipt above the
// trial line, and each case of a batch, whose receipt follows its own line,
// through line continuations and a consumer named by a variable. An
// unfinished trial and a run of no demo are left out.
func TestSeedTestingEvidenceKeepsTheLatestRunOfEachFlow(t *testing.T) {
	t.Parallel()

	live, hosted := t.TempDir(), t.TempDir()
	verdict := func(run, started, result, consumer string) {
		writeFile(t, filepath.Join(live, run, "verdict.json"), `{"verdict":"`+result+`","run":{"provider":"gitlab","started_at":"`+started+
			`","scenario":"^TestGitLabRelease_ConsumerTrial$","inputs":{"forge_ci_live_consumer":"`+consumer+`"}}}`)
	}
	verdict("b-older", "2026-11-01T10:00:00Z", "failed", "/c/demos/hello-maven/consumer")
	verdict("a-newer", "2026-11-02T10:00:00+01:00", "passed", "/c/demos/hello-maven/consumer")
	verdict("c-no-demo", "2026-11-03T10:00:00Z", "passed", "/c/elsewhere")

	receipt := func(trial, name, body string) { writeFile(t, filepath.Join(hosted, trial, name), body+"\n") }

	receipt("controller", "controller.sh", `finish() {
    printf '%s\n' "$?" > "$evidence/controller.exit"
    date --iso-8601=seconds > "$evidence/finished-at"
}
just test-live-github-consumer 1 \
  /v/rustfix/hello-cargo-workspace \
  release version=0.2.0 &`)
	receipt("controller", "controller.exit", "0")
	receipt("controller", "finished-at", "2026-11-04T07:00:00+00:00")

	receipt("batch", "batch.sh", `consumer=/v/demos/hello-go-cli
just test-live-github-consumer 1 "$consumer" \
  pull-request > "$evidence/cli.log"
printf '%s\n' "$?" > "$evidence/cli.exit"
date --iso-8601=seconds > "$evidence/cli.finished-at"
just test-live-github-consumer 1 /v/demos/hello-go-service \
  dev-release > "$evidence/service.log"
printf '%s\n' "$?" > "$evidence/service.exit"
date --iso-8601=seconds > "$evidence/service.finished-at"
just test-live-github-consumer 1 /v/demos/hello-maven pull-request
printf '%s\n' "$?" > "$evidence/unfinished.exit"
date --iso-8601=seconds > "$evidence/unfinished.finished-at"`)
	receipt("batch", "cli.exit", "0")
	receipt("batch", "cli.finished-at", "2026-11-04T08:00:00+00:00")
	receipt("batch", "service.exit", "1")
	receipt("batch", "service.finished-at", "2026-11-04T09:00:00+00:00")

	seed, err := SeedTestingEvidence(live, hosted)
	if err != nil {
		t.Fatal(err)
	}

	if want := `github hello-cargo-workspace release ✅ 2026-11-04
github hello-go-cli pull-request ✅ 2026-11-04
github hello-go-service development ❌ 2026-11-04
gitlab hello-maven release ✅ 2026-11-02
`; seed != want {
		t.Fatalf("seed:\n%s\nwant:\n%s", seed, want)
	}
}

// An overlay mark no cell would show is refused, not dropped.
func TestReadMatricesRefusesAnUnknownSubject(t *testing.T) {
	t.Parallel()

	data := matrices{demos: []matrixDemo{{name: "hello-maven"}}, evidence: matrixEvidence{"github/hello-mavn/release": {mark: "✅"}}}
	if err := data.knownSubjects(); err == nil || !strings.Contains(err.Error(), "hello-mavn is no shipped demo") {
		t.Fatalf("unknown subject: %v, want a refusal naming it", err)
	}
}
