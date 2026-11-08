// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package baseimages

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

	appcontainer "github.com/diggsweden/forge-ci/internal/app/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/imageledger"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
)

// PublishChildBaseInput reuses the root publisher's staging/evidence ports, but
// carries a separate caller-verified immediate parent expectation. The producer
// does not authenticate that parent; protected workflow callers must do so first.
type PublishChildBaseInput struct {
	PublishRootBaseInput

	ParentBase BaseImageParent
}

// PublishChildBase publishes exactly one cold child base's unsigned candidates.
// No recursive graph, warm cache, final-tag write or signing authority is added.
func PublishChildBase(ctx context.Context, registry appcontainer.StagedSignerRegistry, out io.Writer, in PublishChildBaseInput) error {
	image, entry, err := admitChildBasePublication(in.PublishRootBaseInput, in.ParentBase)
	if err != nil {
		return err
	}

	return publishBaseStage(ctx, registry, out, in.PublishRootBaseInput, image, entry)
}

// ValidateChildBasePlan admits the entire architecture partition and requires
// the same immediate parent in caller facts, build arguments and image lineage.
// Ref/ID comparison is not signature verification or producer authentication.
func ValidateChildBasePlan(plan pipeline.ReleaseImagesPlan, repository string, parent BaseImageParent) error {
	_, _, err := admitChildBasePublication(PublishRootBaseInput{Plan: plan, ExpectedRepository: repository}, parent)

	return err
}

func admitChildBasePublication(in PublishRootBaseInput, parent BaseImageParent) (pipeline.ReleaseImage, imageledger.Entry, error) {
	image, entry, err := admitBaseStagePublication(in)
	if err != nil {
		return image, entry, err
	}

	if _, err := baseInputByFlavor([]BaseInput{{Flavor: image.Flavor, BaseInputID: image.BaseInputID, ParentBase: &parent}}); err != nil {
		return image, entry, err
	}

	if parent.InputID == image.BaseInputID {
		return image, entry, fmt.Errorf("child base requires a distinct parent input ID: %w", errs.ErrInvalidConfig)
	}

	expected := map[string]any{"ref": parent.Ref, "input_id": parent.InputID}
	if !reflect.DeepEqual(entry.Provenance["parent_base"], expected) {
		return image, entry, fmt.Errorf("child base lineage must match the explicit parent projection exactly: %w", errs.ErrInvalidConfig)
	}

	argument := ""

	for _, build := range in.Plan.Builds {
		if len(build.RegistryInputs) != 1 {
			return image, entry, fmt.Errorf("child base requires exactly one prepared parent per architecture: %w", errs.ErrInvalidConfig)
		}

		for key, ref := range build.RegistryInputs {
			if ref != parent.Ref || (argument != "" && argument != key) {
				return image, entry, fmt.Errorf("child base registry input differs from its parent projection: %w", errs.ErrInvalidConfig)
			}

			argument = key
		}

		matches := 0

		for line := range strings.SplitSeq(build.BuildArgs, "\n") {
			key, value, _ := strings.Cut(line, "=")
			if key == argument {
				if value != parent.Ref {
					return image, entry, fmt.Errorf("child base parent build argument conflicts: %w", errs.ErrInvalidConfig)
				}

				matches++
			}
		}

		if matches != 1 {
			return image, entry, fmt.Errorf("child base requires one exact parent build argument: %w", errs.ErrInvalidConfig)
		}
	}

	return image, entry, nil
}
