// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	appconfig "github.com/diggsweden/forge-ci/internal/app/config"
	gitlabpipeline "github.com/diggsweden/forge-ci/internal/app/gitlabpipeline"
	"github.com/diggsweden/forge-ci/internal/cli/deps"
	"github.com/diggsweden/forge-ci/internal/cliio"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

const (
	defaultGitLabArtifactsConfig = ".forge-ci/artifacts.yml"
	flagFile                     = "file"
	flagFamily                   = "family"
	flagPrepareOutput            = "prepare-plan-output"
	flagArtifactsOutput          = "artifacts-output"
)

// gitlabReleasePipelineCmd plans the GitLab release child from the
// declaration: the generated jobs, the version-bump plan the release commit
// applies and checks, and a build family's artifacts.
func gitlabReleasePipelineCmd() *cli.Command {
	return &cli.Command{
		Name:  "gitlab-release-pipeline",
		Usage: "plan the GitLab release child pipeline's version and build jobs from artifacts.yml",
		Description: `Plans the release's version bump and builds from the declaration, as
   'plan release' does, and writes what the GitLab release jobs read:

   --output               the generated part of the release child pipeline:
                          one "Prepare: version <family>" job per toolchain
                          family whose projects the plan bumps with their
                          native tool, one "Build: <family>" job per family it
                          builds. Each extends its hidden job in
                          templates/release-jobs.yml and carries nothing else.
                          A planned family needs its --runtime-image-<family>.
   --prepare-plan-output  the prepare-stage plan JSON, for 'version bump-plan'
                          and 'release validate-tag --prepare-plan-file'.
   --artifacts-output     with --family, the names of the artifacts that
                          family's build job builds, one a line.

   Without --file and without .forge-ci/artifacts.yml the release has no
   planned bump and no family; a file named by --file must exist.

EXAMPLE:
   forge-ci plan gitlab-release-pipeline \
     --runtime-image-java "registry.example.com/team/forge-ci-runtime-java-25@sha256:<digest>" \
     --output release-families.yml`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: flagFile, Usage: "artifacts.yml of the release (default: .forge-ci/artifacts.yml, which may be absent)"},
			&cli.StringFlag{Name: "runtime-image-java", Usage: "runtime image of the java family (Maven, Gradle)"},
			&cli.StringFlag{Name: "runtime-image-node", Usage: "runtime image of the node family (npm)"},
			&cli.StringFlag{Name: "runtime-image-go", Usage: "runtime image of the go family"},
			&cli.StringFlag{Name: "runtime-image-rust", Usage: "runtime image of the rust family (Cargo)"},
			&cli.StringFlag{Name: "runtime-image-android", Usage: "runtime image of the android family (Gradle Android)"},
			&cli.StringFlag{Name: flagOutput, Usage: "file to write the generated release jobs to"},
			&cli.StringFlag{Name: flagPrepareOutput, Usage: "file to write the prepare-stage plan JSON to"},
			&cli.StringFlag{Name: flagFamily, Usage: "build family whose artifacts --artifacts-output lists: java, node, go, rust or android"},
			&cli.StringFlag{Name: flagArtifactsOutput, Usage: "file to write --family's artifact names to, one a line"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.String(flagOutput) == "" && cmd.String(flagPrepareOutput) == "" && cmd.String(flagArtifactsOutput) == "" {
				return fmt.Errorf("name at least one of --%s, --%s and --%s: %w", flagOutput, flagPrepareOutput, flagArtifactsOutput, errs.ErrUsage)
			}

			if (cmd.String(flagFamily) == "") != (cmd.String(flagArtifactsOutput) == "") {
				return fmt.Errorf("--%s and --%s go together: %w", flagFamily, flagArtifactsOutput, errs.ErrUsage)
			}

			file := cmd.String(flagFile)
			if file == "" {
				file = defaultGitLabArtifactsConfig
			}

			plans, err := appconfig.LoadReleaseStagePlans(appconfig.EmitConfigPlanInput{Path: file}, deps.Annotator(cmd), cmd.String(flagFile) == "")
			if err != nil {
				return err
			}

			if path := cmd.String(flagPrepareOutput); path != "" {
				if err := writeJSONFile(path, plans.Prepare); err != nil {
					return err
				}
			}

			if path := cmd.String(flagArtifactsOutput); path != "" {
				names, err := gitlabpipeline.ReleaseFamilyArtifacts(plans.Build, cmd.String(flagFamily))
				if err != nil {
					return err
				}

				body := ""
				if len(names) > 0 {
					body = strings.Join(names, "\n") + "\n"
				}

				if err := cliio.WriteFile(path, []byte(body), 0o644); err != nil {
					return fmt.Errorf("write %q: %w", path, err)
				}
			}

			if path := cmd.String(flagOutput); path != "" {
				families, err := gitlabpipeline.PlanReleaseFamilies(plans.Prepare, plans.Build, gitlabpipeline.RuntimeImages{
					Java: cmd.String("runtime-image-java"), Node: cmd.String("runtime-image-node"), Go: cmd.String("runtime-image-go"),
					Rust: cmd.String("runtime-image-rust"), Android: cmd.String("runtime-image-android"),
				})
				if err != nil {
					return err
				}

				if err := writeJSONFile(path, families.Jobs()); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// writeJSONFile writes value as indented JSON, which GitLab also reads as YAML.
func writeJSONFile(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	if err := cliio.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}

	return nil
}
