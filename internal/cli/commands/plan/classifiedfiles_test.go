// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package plan_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/cli"
	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/testutil/ghaenv"
	"github.com/stretchr/testify/require"
)

func classifiedCLIArgs(t *testing.T, kind projecttype.Type) []string {
	t.Helper()

	artifact := config.Artifact{Name: "product", ProjectType: kind, WorkingDirectory: ".", SBOMs: "none"}
	selectionFlag := "--go-test-selections-json"
	selection := `{"product":{"packages":["./test/policy"],"gate":"policy"}}`

	if kind == projecttype.Go {
		artifact.Go = &config.GoConfig{BuildMode: config.GoBuildModeArtifactFirst}
	} else {
		artifact.Cargo = &config.CargoConfig{BuildMode: config.CargoBuildModeArtifactFirst}
		selectionFlag = "--cargo-test-selections-json"
		selection = `{"product":{"runs":[{"kind":"package-test","package":"policy","name":"workflow_policy","gate":"policy"}]}}`
	}

	cfg := pipeline.NewConfigPlan(&config.Config{Artifacts: []config.Artifact{artifact}})
	release, err := pipeline.NewReleasePlan(pipeline.ReleasePlanInput{
		ConfigPlan: cfg, RepositoryVisibility: "private", Branch: "main", RefName: "v1.2.3", ReleasePublisher: "github-cli",
		ReleaseRequireAllowlistedSigner: true, ReleaseSignArtifacts: true, ReleaseSBOMs: "none", ChangelogCreator: "git-cliff",
	})
	require.NoError(t, err)
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	releaseJSON, err := json.Marshal(release)
	require.NoError(t, err)

	return []string{"--config-plan-json", string(cfgJSON), "--release-plan-json", string(releaseJSON),
		"--producer-file-only=true", "--release-sboms", "none", selectionFlag, selection}
}

func TestValidateClassifiedFilesCLI(t *testing.T) {
	for _, kind := range []projecttype.Type{projecttype.Go, projecttype.Cargo} {
		t.Run(string(kind), func(t *testing.T) {
			for _, scenario := range []string{
				"valid", "no-declaration", "no-selection", "application-only", "foreign-selection", "sbom-request",
				"missing-config", "missing-release", "missing-sboms", "invalid-config", "invalid-release",
				"missing-build", "missing-publish", "unknown-build-field", "unknown-publish-field", "duplicate-stage-field", "trailing-release",
				"hidden-publish-image", "empty-publish-image", "hidden-build-target", "build-not-running", "empty-build", "stage-conflict", "separate-image-plan",
				"omitted-declaration", "empty-sboms", "null-config", "null-release", "null-selection",
				"environment-only-config", "environment-only-release", "environment-only-sboms",
			} {
				t.Run(scenario, func(t *testing.T) {
					env := ghaenv.Setup(t)
					args := classifiedCLIArgs(t, kind)
					wantDiagnostic := ""

					var release map[string]any
					require.NoError(t, json.Unmarshal([]byte(args[3]), &release))
					stages := release["stages"].(map[string]any)
					build := stages["build"].(map[string]any)
					publish := stages["publish"].(map[string]any)

					switch scenario {
					case "empty-sboms":
						args[6] = ""
						wantDiagnostic = "classified files require explicit release.sboms=none"
					case "null-config":
						args[1] = "null"
						wantDiagnostic = "unsupported config-plan version 0"
					case "null-selection":
						args[8] = "null"
						wantDiagnostic = "validate " + map[projecttype.Type]string{projecttype.Go: "Go", projecttype.Cargo: "Cargo"}[kind] + " test selections: expected one JSON object keyed by selected artifact names"
					case "no-declaration":
						args[4] = "--producer-file-only=false"
					case "no-selection":
						args = args[:7]
					case "application-only":
						args[8] = strings.ReplaceAll(args[8], `"gate":"policy"`, `"gate":"application"`)
					case "foreign-selection":
						args[8] = strings.ReplaceAll(args[8], `"product":`, `"foreign":`)
					case "sbom-request":
						args[6] = "all"
					case "missing-config":
						args[1] = ""
					case "missing-release":
						args[3] = ""
					case "missing-sboms":
						args = append(args[:5], args[7:]...)
					case "invalid-config":
						args[1] = "{"
					case "missing-build":
						delete(stages, "build")
					case "missing-publish":
						delete(stages, "publish")
					case "unknown-build-field":
						build["unknown"] = true
					case "unknown-publish-field":
						publish["unknown"] = true
					case "hidden-publish-image":
						publish["targets"].(map[string]any)["containers"] = map[string]any{"runs": false, "items": []any{map[string]any{"name": "hidden"}}}
					case "empty-publish-image":
						publish["targets"].(map[string]any)["containers"] = map[string]any{"runs": true}
					case "hidden-build-target":
						build["targets"].(map[string]any)["npm"] = map[string]any{"runs": false, "items": []any{map[string]any{"name": "hidden"}}}
					case "build-not-running":
						build["targets"].(map[string]any)[string(kind)].(map[string]any)["runs"] = false
					case "empty-build":
						build["targets"].(map[string]any)[string(kind)].(map[string]any)["items"] = []any{}
					case "stage-conflict":
						build["stage"] = "publish"
					case "separate-image-plan":
						args = append(args, "--release-images-plan-json", "{}")
					}
					// Preserve the intentionally blank release argument in its refusal case.
					if scenario != "missing-release" && scenario != "missing-sboms" {
						body, err := json.Marshal(release)
						require.NoError(t, err)

						args[3] = string(body)
					}

					switch scenario {
					case "omitted-declaration":
						args = append(args[:4], args[5:]...)
						wantDiagnostic = "classified files need an explicit administrator file-only producer declaration"
					case "null-release":
						args[3] = "null"
						wantDiagnostic = "classified files require explicit policy.sboms=none"
					case "environment-only-config":
						env.Setenv("CONFIG_PLAN_JSON", args[1])
						args = args[2:]
						wantDiagnostic = `Required flag "config-plan-json" not set`
					case "environment-only-release":
						env.Setenv("RELEASE_PLAN_JSON", args[3])
						args = append(args[:2], args[4:]...)
						wantDiagnostic = `Required flag "release-plan-json" not set`
					case "environment-only-sboms":
						env.Setenv("RELEASE_SBOMS", args[6])
						args = append(args[:5], args[7:]...)
						wantDiagnostic = `Required flag "release-sboms" not set`
					case "invalid-release":
						args[3] = "{"
					case "duplicate-stage-field":
						args[3] = strings.Replace(args[3], `"stage":"build"`, `"stage":"build","stage":"build"`, 1)
					case "trailing-release":
						args[3] += " {}"
					}

					var stdout, stderr bytes.Buffer

					cmd := cli.New(cli.BuildInfo{})
					cmd.Writer, cmd.ErrWriter = &stdout, &stderr

					err := cmd.Run(t.Context(), append([]string{"forge-ci", "--provider", "local", "--runner", "local", "--json", "plan", "validate-classified-files"}, args...))
					if scenario == "valid" {
						require.NoError(t, err)
						require.Empty(t, stdout.String(), "success must not emit a token or plan")
					} else {
						require.Error(t, err, scenario)

						if wantDiagnostic != "" {
							require.ErrorContains(t, err, wantDiagnostic)
						}

						require.NotContains(t, stdout.String(), `"release-plan-json":`, "no partial or fallback plans")
					}

					for _, path := range []string{env.OutputPath, env.SummaryPath} {
						body, readErr := os.ReadFile(path)
						require.NoError(t, readErr)
						require.Empty(t, body, "validation must not write runner outputs or summaries")
					}
				})
			}
		})
	}
}
