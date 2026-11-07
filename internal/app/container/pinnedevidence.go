// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package container

import (
	"context"
	"fmt"
	"io"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/imageledger"
)

// ValidatePinnedImageSBOM reads the same producer bytes the signer consumes.
// It has no registry, generator or signer capability and never generates a BOM.
func ValidatePinnedImageSBOM(ctx context.Context, evidenceRoot string, entry imageledger.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if evidenceRoot == "" || entry.SBOMSHA256 == "" {
		return fmt.Errorf("pinned evidence requires an explicit root and SBOM pin: %w", errs.ErrValidation)
	}

	return ensureImageSBOMAt(ctx, nil, io.Discard, "", entry, evidenceRoot)
}
