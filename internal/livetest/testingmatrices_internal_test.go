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
// Maven service is the one demo with a lifecycle on both forges, the Cargo
// container has a GitLab line, the npm service is admitted on GitLab without
// a line, a GitHub line is the fixture workflow a demo ships, and a refusal
// carries the planner's or the admission's reason.
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
		"| ⛔ / admitted / admitted |",
		"| hello-ios (xcode-ios · no container · no image) | ⬚ / ⬚ / 📝 |",
		"- hello-monorepo, github: the demo has no root Containerfile",
		"| hello-go-cli (go · no container · no image) |",
		"- hello-go-service, gitlab: .forge-ci/artifacts.yml is not a service snapshot's declaration",
		"|  | container-first | ⛔ hello-cargo-container | 📝 hello-cargo-container | ⬚ hello-cargo-container (admitted, no line) |",
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

// The seed keeps each flow's latest run whatever the record order, reads a
// hosted controller's trial through its line continuations, and leaves an
// unfinished trial and a run of no demo out.
func TestSeedTestingEvidenceKeepsTheLatestRunOfEachFlow(t *testing.T) {
	t.Parallel()

	live, hosted := t.TempDir(), t.TempDir()
	verdict := func(run, started, result string) {
		writeFile(t, filepath.Join(live, run, "verdict.json"), `{"verdict":"`+result+`","run":{"provider":"gitlab","started_at":"`+started+
			`","scenario":"^TestGitLabRelease_ConsumerTrial$","inputs":{"forge_ci_live_consumer":"/c/demos/hello-maven/consumer"}}}`)
	}
	verdict("b-older", "2026-11-01T10:00:00Z", "failed")
	verdict("a-newer", "2026-11-02T10:00:00Z", "passed")
	verdict("c-no-demo", "2026-11-03T10:00:00Z", "passed")
	writeFile(t, filepath.Join(live, "c-no-demo", "verdict.json"), `{"verdict":"passed","run":{"provider":"gitlab","started_at":"2026-11-03T10:00:00Z","inputs":{"forge_ci_live_consumer":"/c/elsewhere"}}}`)

	trial := func(name, event, exit string) {
		writeFile(t, filepath.Join(hosted, name, "controller.sh"), "just test-live-github-consumer 1 \\\n  /v/rustfix/hello-cargo-workspace \\\n  "+event+" RELEASE_PLATFORMS=linux/amd64 &\n")

		if exit != "" {
			writeFile(t, filepath.Join(hosted, name, "controller.exit"), exit+"\n")
			writeFile(t, filepath.Join(hosted, name, "finished-at"), "2026-11-04T07:00:00+00:00\n")
		}
	}
	trial("pr", TrialPullRequest, "0")
	trial("dev", TrialDevRelease, "1")
	trial("release", TrialRelease, "")

	seed, err := SeedTestingEvidence(live, hosted)
	if err != nil {
		t.Fatal(err)
	}

	if want := "github hello-cargo-workspace development ❌ 2026-11-04\ngithub hello-cargo-workspace pull-request ✅ 2026-11-04\ngitlab hello-maven release ✅ 2026-11-02\n"; seed != want {
		t.Fatalf("seed:\n%s\nwant:\n%s", seed, want)
	}
}
