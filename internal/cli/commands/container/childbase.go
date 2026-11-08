// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/diggsweden/forge-ci/internal/adapters/ociregistry"
	appbaseimages "github.com/diggsweden/forge-ci/internal/app/baseimages"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
)

const flagParentBaseJSON = "parent-base-json"

func childBaseGroup() *cli.Command {
	plan := nativeBasePlanCmd(readChildBasePlanFile)
	plan.Usage = "admit one cold child BASE against an explicit caller-verified parent"
	build := nativeImagesBuildCmd(readChildBasePlanFile)
	build.Usage = "build one child BASE using secret-free prepared parent inputs"
	build.Description = "Requires --inputs-dir from the existing release-images fetch-build-inputs boundary. No registry fetch credentials or parent authentication run in the builder."

	for _, command := range []*cli.Command{plan, build} {
		command.Flags = append(command.Flags, parentBaseFlag())
	}

	return &cli.Command{
		Name:        "child-base",
		Usage:       "single cold child BASE stage; no graph scheduling, warm/cache or expected-parent promotion",
		Description: "The protected caller must independently verify the parent with Cosign, its trust key, source/workflow and input ID before supplying parent-base-json. The producer only binds that assertion to its registry input and unsigned lineage; it never authenticates the parent or traverses a graph.",
		Commands:    []*cli.Command{plan, build, childBasePublishCmd()},
	}
}

func parentBaseFlag() cli.Flag {
	return &cli.StringFlag{Name: flagParentBaseJSON, Required: true, Usage: "explicit caller-verified immediate parent as {ref: digest-pinned-ref, input_id: opaque-sha256-ID}; not producer authentication"}
}

func childBaseParent(cmd *cli.Command) (appbaseimages.BaseImageParent, error) {
	var parent appbaseimages.BaseImageParent

	decoder := json.NewDecoder(strings.NewReader(cmd.String(flagParentBaseJSON)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&parent); err != nil {
		return parent, fmt.Errorf("invalid explicit parent projection: %w: %w", err, errs.ErrInvalidConfig)
	}

	if err := decoder.Decode(new(any)); err != io.EOF {
		return parent, fmt.Errorf("explicit parent projection must contain one object: %w", errs.ErrInvalidConfig)
	}

	return parent, nil
}

func readChildBasePlanFile(cmd *cli.Command) (pipeline.ReleaseImagesPlan, error) {
	parent, err := childBaseParent(cmd)
	if err != nil {
		return pipeline.ReleaseImagesPlan{}, err
	}

	return readBaseStagePlanFile(cmd, func(plan pipeline.ReleaseImagesPlan, repository string) error {
		return appbaseimages.ValidateChildBasePlan(plan, repository, parent)
	})
}

func childBasePublishCmd() *cli.Command {
	return &cli.Command{
		Name:  subCmdPublish,
		Usage: "scan/gate all child layouts and publish unsigned canonical evidence retaining parent lineage",
		Flags: append(baseStagePublishFlags(), parentBaseFlag()),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			plan, err := readChildBasePlanFile(cmd)
			if err != nil {
				return err
			}

			parent, err := childBaseParent(cmd)
			if err != nil {
				return err
			}

			input, err := baseStagePublicationInput(cmd, plan)
			if err != nil {
				return err
			}

			return appbaseimages.PublishChildBase(ctx, ociregistry.WithAuthFile(cmd.String(flagAuthFile)), os.Stderr,
				appbaseimages.PublishChildBaseInput{PublishRootBaseInput: input, ParentBase: parent})
		},
	}
}
