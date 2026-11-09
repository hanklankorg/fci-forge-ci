// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"strings"
	"testing"
)

func developmentRunnerTestWorkflow(job string) string {
	return "# preserved comment: ${{ vars.CI_RUNNER_LABEL }}\nname: Development\non:\n  push:\n    branches: [development]\npermissions:\n  contents: read\njobs:\n  development:\n    name: \"" + job + "\"\n    runs-on: ${{ vars.CI_RUNNER_LABEL }} # preserve comment\n    env:\n      SAMPLE: ${{ vars.CI_RUNNER_LABEL }}\n    steps:\n      - run: cargo test --locked\n"
}

func TestDevelopmentRunnerWorkflow(t *testing.T) {
	for _, test := range []struct {
		name, job, variable string
	}{
		{"package normal", "Check: development package", "CI_RUNNER_LABEL"},
		{"Cargo container", "Check: development container", "CI_RUNNER_LABEL_CONTAINER"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := developmentRunnerTestWorkflow(test.job)
			want := strings.Replace(body, "runs-on: ${{ vars.CI_RUNNER_LABEL }}", "runs-on: ${{ fromJSON(vars."+test.variable+") }}", 1)

			got, err := developmentRunnerWorkflow(body, test.job, test.variable)
			if err != nil || got != want {
				t.Fatalf("runner-only transformation differs: %v\n%s", err, got)
			}

			second, err := developmentRunnerWorkflow(got, test.job, test.variable)
			if err != nil || second != got {
				t.Fatalf("already bound workflow must remain byte-identical: %v", err)
			}
		})
	}
}

func TestDevelopmentRunnerWorkflowRefusesDifferentContracts(t *testing.T) {
	const job = "Check: development container"

	body := developmentRunnerTestWorkflow(job)
	for _, test := range []struct{ name, body string }{
		{"invalid YAML", "jobs: ["},
		{"no jobs", "name: Development\n"},
		{"additional job", body + "  extra:\n    name: other\n    runs-on: original\n"},
		{"wrong job", strings.Replace(body, job, "other job", 1)},
		{"explicit runner", strings.Replace(body, "runs-on: ${{ vars.CI_RUNNER_LABEL }}", "runs-on: private-pool", 1)},
		{"different expression", strings.Replace(body, "runs-on: ${{ vars.CI_RUNNER_LABEL }}", "runs-on: ${{ vars.OTHER }}", 1)},
		{"sequence runner", strings.Replace(body, "runs-on: ${{ vars.CI_RUNNER_LABEL }}", "runs-on: [private-pool]", 1)},
		{"quoted runner", strings.Replace(body, "runs-on: ${{ vars.CI_RUNNER_LABEL }}", "runs-on: '${{ vars.CI_RUNNER_LABEL }}'", 1)},
		{"missing runner", strings.Replace(body, "    runs-on: ${{ vars.CI_RUNNER_LABEL }} # preserve comment\n", "", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := developmentRunnerWorkflow(test.body, job, "CI_RUNNER_LABEL_CONTAINER")
			if err == nil || got != "" {
				t.Fatalf("different contract must refuse without partial workflow: %q %v", got, err)
			}
		})
	}
}

func TestDevelopmentRunnerFixturePreservesOriginalSource(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, test := range []struct{ job, variable string }{
			{"Check: development package", "CI_RUNNER_LABEL"},
			{"Check: development container", "CI_RUNNER_LABEL_CONTAINER"},
		} {
			t.Run(format+"/"+test.job, func(t *testing.T) {
				root, git := stageTestRepo(t, format)
				body := developmentRunnerTestWorkflow(test.job)
				stageTestWrite(t, root, ".forgejo/workflows/development.yml", body, 0755)
				stageTestWrite(t, root, "Containerfile", "FROM original.invalid@sha256:unchanged\n", 0644)
				stageTestWrite(t, root, "Cargo.lock", "# original lock\n", 0644)
				stageTestWrite(t, root, "package.json", "{\"version\":\"1.2.3\"}\n", 0644)
				stageTestCommit(t, git, root)

				source, err := openConsumer(t.Context(), git, root, t.TempDir())
				if err != nil {
					t.Fatal(err)
				}

				fixture, err := developmentRunnerFixture(t.Context(), source, test.job, test.variable)
				if err != nil {
					t.Fatal(err)
				}

				bound, err := developmentRunnerFixture(t.Context(), fixture, test.job, test.variable)
				if err != nil || bound.Commit != fixture.Commit {
					t.Fatalf("already bound fixture must retain its commit: %v", err)
				}

				diff, err := git.run(t.Context(), fixture.stage, "diff-tree", "--no-commit-id", "--name-only", "-r", source.Commit, fixture.Commit)
				if err != nil || strings.TrimSpace(diff) != ".forgejo/workflows/development.yml" {
					t.Fatalf("changed non-workflow source: %q %v", diff, err)
				}

				mode, err := git.run(t.Context(), fixture.stage, "ls-tree", "--format=%(objectmode)", fixture.Commit, "--", ".forgejo/workflows/development.yml")
				if err != nil || strings.TrimSpace(mode) != "100755" {
					t.Fatalf("workflow mode changed: %q %v", mode, err)
				}

				if err = git.requireClean(t.Context(), root, "."); err != nil {
					t.Fatal(err)
				}

				original, err := git.run(t.Context(), source.stage, "show", source.Commit+":.forgejo/workflows/development.yml")
				if err != nil || original != strings.TrimSpace(body) {
					t.Fatalf("admitted workflow changed: %v", err)
				}
			})
		}
	}
}
