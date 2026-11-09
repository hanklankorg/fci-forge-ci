// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"

	appcontainer "github.com/diggsweden/forge-ci/internal/app/container"
	"github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/imageledger"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
)

// ReleaseImageOutputs binds a run's engine ledger to an independently planned
// output set. Plan MUST come from the admitted signed source declaration, never
// from this ledger or mutable host files. The ledger is a digest hint, not trust:
// final tags must serve it and the existing verifier must authenticate that exact
// digest's signature, SBOM and SLSA source/commit/tag/workflow identity.
type ReleaseImageOutputs struct {
	Plan                                                 pipeline.ReleaseImagesPlan
	Ledger                                               []byte
	Repository, Tag, Commit, Source, Workflow, PublicKey string
}

type releaseOutputVerifier interface {
	VerifyImage(ctx context.Context, in container.ImageVerifyRequest, errOut io.Writer) error
	VerifyAttestation(ctx context.Context, in container.AttestationVerifyRequest, errOut io.Writer) error
	VerifyAttestationOutput(ctx context.Context, in container.AttestationVerifyRequest, out, errOut io.Writer) error
}

// VerifyReleaseImageOutputs performs complete plan/receipt admission before any
// registry reads. resolve returns the top-level digest, never a selected child.
// Evidence semantics and Cosign verification stay with VerifyExistingReleaseImage;
// re-attestation eligibility is deliberately not acceptance.
func VerifyReleaseImageOutputs(ctx context.Context, in ReleaseImageOutputs, resolve func(context.Context, string) (string, error), verifier releaseOutputVerifier, out io.Writer) error {
	entries, err := admitReleaseImageOutputs(in)
	if err != nil {
		return err
	}

	if resolve == nil || verifier == nil {
		return fmt.Errorf("release image readback requires registry and evidence readers: %w", errs.ErrUsage)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		digest, err := resolve(ctx, entry.FinalTag)
		if err != nil {
			return fmt.Errorf("read planned release image %s: %w", entry.FinalTag, err)
		}

		if digest != entry.Digest {
			return fmt.Errorf("planned final image %s does not serve receipt digest: %w", entry.FinalTag, errs.ErrValidation)
		}

		result, err := appcontainer.VerifyExistingReleaseImage(ctx, verifier, nil, out, appcontainer.ReleaseImageVerifyExistingInput{
			Ref: entry.Ref, CosignPublicKey: in.PublicKey,
			ExpectedTag: in.Tag, ExpectedCommit: in.Commit, ExpectedSource: in.Source, ExpectedWorkflow: in.Workflow,
		})
		if err != nil {
			return err
		}

		if result.Status != appcontainer.ReleaseImageStatusVerified {
			return fmt.Errorf("planned release image is not fully verified: %w", errs.ErrValidation)
		}
	}

	return nil
}

func admitReleaseImageOutputs(in ReleaseImageOutputs) ([]imageledger.Entry, error) {
	if !releaseOutputIdentityPresent(in) {
		return nil, fmt.Errorf("release image readback requires verified source identity and key: %w", errs.ErrValidation)
	}

	body, err := json.Marshal(in.Plan)
	if err != nil {
		return nil, err
	}

	plan, err := pipeline.ReadImageBuildPlan(string(body), in.Repository, in.Tag)
	if err != nil {
		return nil, err
	}

	entries, err := imageledger.Parse(in.Ledger)
	if err != nil {
		return nil, err
	}

	if err := imageledger.ValidateAll(entries, in.Tag); err != nil {
		return nil, err
	}

	if err := imageledger.ValidateEntryRepositories(entries, in.Repository); err != nil {
		return nil, err
	}

	if len(entries) != len(plan.Images) {
		return nil, fmt.Errorf("release receipt omits or adds planned images: %w", errs.ErrValidation)
	}

	byTag := map[string]imageledger.Entry{}
	for _, entry := range entries {
		byTag[entry.FinalTag] = entry
	}

	for _, image := range plan.Images {
		// This bounded oracle covers release outputs, not content-addressed
		// bases or custom policy lineage (which have separate owners).
		if !standardReleaseOutput(image) {
			return nil, fmt.Errorf("release image readback requires standard declarative release policy: %w", errs.ErrValidation)
		}

		entry, found := byTag[image.ImageRepository+":"+image.FinalTagName]
		if !found || !releaseOutputEntryMatches(image, entry) {
			return nil, fmt.Errorf("release receipt differs from independently planned output %s: %w", image.Name, errs.ErrValidation)
		}
	}

	return entries, nil
}

func releaseOutputIdentityPresent(in ReleaseImageOutputs) bool {
	return fixtureObjectID(in.Commit) && in.Source != "" && in.Workflow != "" && in.PublicKey != ""
}

func standardReleaseOutput(image pipeline.ReleaseImage) bool {
	return image.ImageKind != string(imageledger.ImageKindBase) && image.BaseRef == "" && image.BaseInputID == "" && image.MovingTagName == "" && image.ProvenanceJSON == ""
}

func releaseOutputEntryMatches(image pipeline.ReleaseImage, entry imageledger.Entry) bool {
	return entry.Ref == image.ImageRepository+"@"+entry.Digest &&
		entry.CandidateTag == image.ImageRepository+":"+image.CandidateTag &&
		entry.Role == cmp.Or(image.Role, image.Name) && entry.Flavor == image.Flavor &&
		cmp.Or(string(entry.ImageKind), "release") == cmp.Or(image.ImageKind, "release") &&
		entry.BaseRef == "" && entry.BaseInputID == "" && entry.MovingTag == ""
}
