// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// NewPackageDevelopmentFixture binds the package job to the normal runner in
// the private consumer index. The admitted checkout and all other bytes stay put.
func NewPackageDevelopmentFixture(tb TB, source Consumer) Consumer {
	tb.Helper()

	return newDevelopmentRunnerFixture(tb, source, "Check: development package", "CI_RUNNER_LABEL")
}

// NewCargoDevelopmentFixture binds the sole Cargo build/publish job to the
// container runner and migrates the exact historical publication step to the
// current native CLI in the private index. The admitted checkout stays unchanged.
func NewCargoDevelopmentFixture(tb TB, source Consumer) Consumer {
	tb.Helper()

	runner := newDevelopmentRunnerFixture(tb, source, "Check: development container", "CI_RUNNER_LABEL_CONTAINER")

	ctx, cancel := context.WithTimeout(Context(tb), timeoutLayerPush)
	defer cancel()

	fixture, err := cargoDevelopmentFixture(ctx, runner, os.Getenv)
	if err != nil {
		tb.Fatalf("Cargo development publication fixture: %v", err)
	}

	tb.Logf("Cargo development publication fixture: admitted=%s runner-staged=%s generated=%s", source.Commit, runner.Commit, fixture.Commit)

	return fixture
}

func newDevelopmentRunnerFixture(tb TB, source Consumer, jobName, runnerVariable string) Consumer {
	tb.Helper()

	ctx, cancel := context.WithTimeout(Context(tb), timeoutLayerPush)
	defer cancel()

	fixture, err := developmentRunnerFixture(ctx, source, jobName, runnerVariable)
	if err != nil {
		tb.Fatalf("development runner fixture: %v", err)
	}

	tb.Logf("development runner fixture: admitted=%s staged=%s job=%s runner-variable=%s", source.Commit, fixture.Commit, jobName, runnerVariable)

	return fixture
}

func developmentRunnerFixture(ctx context.Context, source Consumer, jobName, runnerVariable string) (Consumer, error) {
	const path = ".forgejo/workflows/development.yml"

	body, err := githubWorkflow(ctx, source, path)
	if err != nil {
		return source, err
	}

	rendered, err := developmentRunnerWorkflow(body, jobName, runnerVariable)
	if err != nil || rendered == body {
		return source, err
	}

	return stageDevelopmentWorkflow(ctx, source, rendered)
}

func stageDevelopmentWorkflow(ctx context.Context, source Consumer, rendered string) (Consumer, error) {
	const path = ".forgejo/workflows/development.yml"

	mode, err := source.git.run(ctx, source.stage, "ls-tree", "--format=%(objectmode)", source.Commit, "--", path)
	if err != nil {
		return source, err
	}

	mode = strings.TrimSpace(mode)
	if mode != "100644" && mode != "100755" {
		return source, fmt.Errorf("development workflow is not a regular tracked file: %w", errs.ErrValidation)
	}

	if _, err = source.git.run(ctx, source.stage, "read-tree", source.Commit); err != nil {
		return source, err
	}

	if err = source.stageFile(ctx, mode, path, rendered); err != nil {
		return source, err
	}

	source.Commit, err = source.commit(ctx, consumerBaseMessage)

	return source, err
}

// Only the known single-job development contract may acquire a runner binding.
// Keep every other byte (including shell, images, credentials and policy) intact.
func developmentRunnerWorkflow(body, jobName, runnerVariable string) (string, error) {
	var workflow struct {
		Jobs map[string]struct {
			Name   string    `yaml:"name"`
			RunsOn yaml.Node `yaml:"runs-on"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal([]byte(body), &workflow); err != nil {
		return "", fmt.Errorf("development workflow YAML: %w", err)
	}

	if len(workflow.Jobs) != 1 {
		return "", fmt.Errorf("development runner fixture requires one job: %w", errs.ErrValidation)
	}

	for _, job := range workflow.Jobs {
		if job.Name != jobName || job.RunsOn.Kind != yaml.ScalarNode || job.RunsOn.Style != 0 {
			return "", fmt.Errorf("development runner fixture requires the declared job and plain runner expression: %w", errs.ErrValidation)
		}

		want := "${{ fromJSON(vars." + runnerVariable + ") }}"
		if job.RunsOn.Value == want {
			return body, nil
		}

		const legacy = "${{ vars.CI_RUNNER_LABEL }}"
		if job.RunsOn.Value != legacy {
			return "", fmt.Errorf("development runner fixture refuses a different runner binding: %w", errs.ErrValidation)
		}

		lines := strings.SplitAfter(body, "\n")

		line, column := job.RunsOn.Line-1, job.RunsOn.Column-1
		if line < 0 || line >= len(lines) || column < 0 || column > len(lines[line]) || !strings.HasPrefix(lines[line][column:], legacy) {
			return "", fmt.Errorf("development runner expression location is invalid: %w", errs.ErrValidation)
		}

		lines[line] = lines[line][:column] + want + lines[line][column+len(legacy):]

		return strings.Join(lines, ""), nil
	}

	return "", fmt.Errorf("development runner job is missing: %w", errs.ErrValidation)
}
