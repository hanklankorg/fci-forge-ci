// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package container

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/diggsweden/forge-ci/internal/adapters/ociregistry"
	"github.com/diggsweden/forge-ci/internal/adapters/syft"
	"github.com/diggsweden/forge-ci/internal/adapters/trivy"
	appbaseimages "github.com/diggsweden/forge-ci/internal/app/baseimages"
	"github.com/diggsweden/forge-ci/internal/cli/deps"
	"github.com/diggsweden/forge-ci/internal/cliio"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
)

// This is not a general BASE plan switch: every verb independently admits one
// cold root and the ordinary release-images boundary continues to refuse BASE.
func rootBaseGroup() *cli.Command {
	build := nativeImagesBuildCmd(readRootBasePlanFile)
	build.Usage = "build one admitted root base's native architecture without registry writes"
	build.Description = "Builds a plan admitted by root-base plan using the existing native builder. Parents, mixed/multiple images and warm caches are unsupported."

	return &cli.Command{
		Name:     "root-base",
		Usage:    "cold single-root native BASE boundary; no parent, mixed or warm graph support",
		Commands: []*cli.Command{rootBasePlanCmd(), build, rootBasePublishCmd(), rootBaseAdmissionCmd(false), rootBaseAdmissionCmd(true)},
	}
}

func readRootBasePlanFile(cmd *cli.Command) (pipeline.ReleaseImagesPlan, error) {
	return readBaseStagePlanFile(cmd, appbaseimages.ValidateRootBasePlan)
}

func readBaseStagePlanFile(cmd *cli.Command, validate func(pipeline.ReleaseImagesPlan, string) error) (pipeline.ReleaseImagesPlan, error) {
	body, err := cliio.ReadRegularFile(cmd.String(flagPlan))
	if err != nil {
		return pipeline.ReleaseImagesPlan{}, err
	}

	plan, err := pipeline.ReadImageBuildPlan(string(body), cmd.String(flagImageRepository), cmd.String(flagReleaseTag))
	if err != nil {
		return pipeline.ReleaseImagesPlan{}, err
	}

	return plan, validate(plan, cmd.String(flagImageRepository))
}

func rootBasePlanCmd() *cli.Command {
	return nativeBasePlanCmd(readRootBasePlanFile)
}

func nativeBasePlanCmd(readPlan func(*cli.Command) (pipeline.ReleaseImagesPlan, error)) *cli.Command {
	return &cli.Command{
		Name:  subCmdPlan,
		Usage: "admit a resolved consumer single-root plan and freeze it for native jobs",
		Flags: append(releaseImagesJobFlags(),
			&cli.StringFlag{Name: flagOutput, Required: true, Usage: "admitted plan output file"},
			&cli.StringFlag{Name: flagRunnerArchs, Required: true, Usage: "native architectures scheduled by the caller's YAML"},
		),
		Action: func(_ context.Context, cmd *cli.Command) error {
			plan, err := readPlan(cmd)
			if err != nil {
				return err
			}

			return writeNativeImagesPlanFile(cmd.String(flagOutput), plan, cmd.String(flagRunnerArchs))
		},
	}
}

func rootBasePublishCmd() *cli.Command {
	return &cli.Command{
		Name:        subCmdPublish,
		Usage:       "scan/gate all root layouts, publish unsigned candidates and write a canonical payload",
		Description: "Output is a new workspace containing dist/base-images.json and premade-sboms/. Transport and verify the entire workspace with release dist-digest/validate-dist --canonical-payload before trusted base-images import/sign/promote. No final tags or signatures are produced here.",
		Flags:       baseStagePublishFlags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			plan, err := readRootBasePlanFile(cmd)
			if err != nil {
				return err
			}

			input, err := baseStagePublicationInput(cmd, plan)
			if err != nil {
				return err
			}

			return appbaseimages.PublishRootBase(ctx, ociregistry.WithAuthFile(cmd.String(flagAuthFile)), os.Stderr, input)
		},
	}
}

func baseStagePublishFlags() []cli.Flag {
	return append(releaseImagesJobFlags(),
		ledgerAuthFileFlag("staging registry auth file; never signing/publication credentials"),
		&cli.StringFlag{Name: flagLayoutsDir, Required: true, Usage: "complete admitted native layout set"},
		&cli.StringFlag{Name: flagOutputDir, Required: true, Usage: "new canonical payload directory with an existing parent"},
	)
}

func baseStagePublicationInput(cmd *cli.Command, plan pipeline.ReleaseImagesPlan) (appbaseimages.PublishRootBaseInput, error) {
	if cmd.String(flagAuthFile) == "" {
		return appbaseimages.PublishRootBaseInput{}, fmt.Errorf("base stage publication requires --auth-file: %w", errs.ErrUsage)
	}

	scanner := trivy.New()

	return appbaseimages.PublishRootBaseInput{
		Plan: plan, ExpectedRepository: cmd.String(flagImageRepository),
		LayoutsDir: cmd.String(flagLayoutsDir), OutputDir: cmd.String(flagOutputDir), AuthFile: cmd.String(flagAuthFile),
		SourceSHA: cmd.String(flagReleaseSHA), ServerURL: cmd.String(flagServerURL), Repository: cmd.String(flagRepository),
		RetryAttempts: cmd.Int(flagRetryAttempts), Trivy: scanner, Syft: syft.New(), Gate: publicationGate(scanner, deps.Annotator(cmd)),
	}, nil
}
