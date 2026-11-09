// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package conformance_test

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/domain/enforcement"
)

// Shared with the live owner so ordinary offline tests check the actual graph.
//
//go:embed testdata/layer/consumer-pr.yml
var layerConsumerPRFixture string

const consumerPRAssertionFailure = `package main

import "testing"

func TestControlledApplicationFinding(t *testing.T) {
	t.Fatal("controlled application assertion")
}
`

// Valid Go syntax, but no test can run: this is an operational compiler error,
// not an assertion finding eligible for report-only continuation.
const consumerPRCompileError = `package main

import "testing"

func TestControlledApplicationFinding(t *testing.T) {
	enforcementCompileError(t)
}
`

var consumerPRAdverseTests = []struct{ name, source string }{
	{"application-test-findings", consumerPRAssertionFailure},
	{"application-test-compile-error", consumerPRCompileError},
}

func TestConsumerPRFixtureGatesTheDownstreamJob(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Needs []string `yaml:"needs"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(layerConsumerPRFixture), &workflow); err != nil {
		t.Fatal(err)
	}

	if needs := workflow.Jobs["consumer-test"].Needs; len(needs) != 1 || needs[0] != "consumer-build" {
		t.Fatalf("downstream identity job must depend on the actual build action, got %v", needs)
	}
}

// Execute the same tiny test sources that the maintained PR owner commits.
// This is an offline classifier witness, not native action/job execution.
func TestConsumerPRAdverseSourcesAreFindingsAndOperationalError(t *testing.T) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range consumerPRAdverseTests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range map[string]string{
				"go.mod":       "module example.invalid/enforcement-fixture\n\ngo 1.27.3\n",
				"main_test.go": test.source,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, goBinary, "test", "-json", "-count=1", "./...")
			cmd.Dir = dir

			var stdout, stderr bytes.Buffer

			cmd.Stdout, cmd.Stderr = &stdout, &stderr

			var exit *exec.ExitError

			if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("fixture must fail through Go, not a shell exit: %v\n%s\n%s", err, &stdout, &stderr)
			}

			output := enforcement.NewTestOutput(enforcement.GoJSON)
			_, _ = output.Stdout().Write(stdout.Bytes())
			_, _ = output.Stderr().Write(stderr.Bytes())

			want, marker := 1, "controlled application assertion"
			if test.source == consumerPRCompileError {
				want, marker = 0, "undefined: enforcementCompileError"
			}

			if got := output.FailedTests(); got != want || !strings.Contains(stdout.String()+stderr.String(), marker) {
				t.Fatalf("classified %d assertion failures, want %d and %q\n%s\n%s", got, want, marker, &stdout, &stderr)
			}
		})
	}
}
