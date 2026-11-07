// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package baseimages

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/url"
	"os"

	appcontainer "github.com/diggsweden/forge-ci/internal/app/container"
	apprelease "github.com/diggsweden/forge-ci/internal/app/release"
	domaincontainer "github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/git"
	"github.com/diggsweden/forge-ci/internal/domain/imageledger"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/provenance"
	"github.com/diggsweden/forge-ci/internal/pathsafe"
)

// RootBaseSource names observed producer facts, not authentication evidence.
// The protected caller must authenticate their origin independently.
type RootBaseSource struct {
	Repository string
	Commit     string
	Workflow   string
}

// RootBasePlanAdmission binds exact plan bytes and observed source facts to
// independently approved expectations. Never derive Expected or PlanSHA256 from
// the producer's own assertions. Content revisions remain opaque plan choices:
// approval of these exact bytes constrains them without equating them to Commit.
type RootBasePlanAdmission struct {
	Plan            []byte
	PlanSHA256      string
	ImageRepository string
	Source          RootBaseSource
	Expected        RootBaseSource
}

// AdmitRootBasePlan is read-only and has no external capabilities. It does not
// authenticate a producer, inspect a checkout or authorize a workflow.
func AdmitRootBasePlan(in RootBasePlanAdmission) (pipeline.ReleaseImagesPlan, error) {
	var empty pipeline.ReleaseImagesPlan
	if !domaincontainer.ValidSHA256Hex(in.PlanSHA256) || !validRootSource(in.Expected) {
		return empty, fmt.Errorf("root admission requires an approved plan SHA-256 and source/workflow/commit: %w", errs.ErrUsage)
	}

	if in.Source != in.Expected {
		return empty, fmt.Errorf("root producer source facts differ from independent expectations: %w", errs.ErrValidation)
	}

	if fmt.Sprintf("%x", sha256.Sum256(in.Plan)) != in.PlanSHA256 {
		return empty, fmt.Errorf("root plan differs from independently approved bytes: %w", errs.ErrValidation)
	}

	plan, err := pipeline.ReadImageBuildPlan(string(in.Plan), in.ImageRepository, "")
	if err != nil {
		return empty, err
	}

	if err := ValidateRootBasePlan(plan, in.ImageRepository); err != nil {
		return empty, err
	}

	return plan, nil
}

func validRootSource(source RootBaseSource) bool {
	parsed, err := url.Parse(source.Repository)

	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.Path != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" &&
		git.ValidCommitSHA(source.Commit) && provenance.ValidWorkflowPath(source.Workflow)
}

// RootBasePayloadAdmission adds the entire canonical unsigned workspace and its
// independently transported digest. PayloadDigest binds bytes, not authorship.
type RootBasePayloadAdmission struct {
	Plan          RootBasePlanAdmission
	EvidenceRoot  string
	PayloadDigest string
}

// AdmitRootBasePayload verifies source/plan binding, canonical payload bytes,
// the sole root ledger's agreement with that plan and retained producer SBOMs.
// No credentials, registry login, key loading, SBOM regeneration or writes occur.
func AdmitRootBasePayload(ctx context.Context, in RootBasePayloadAdmission) error {
	plan, err := AdmitRootBasePlan(in.Plan)
	if err != nil {
		return err
	}

	if in.EvidenceRoot == "" || !domaincontainer.ValidSHA256Hex(in.PayloadDigest) {
		return fmt.Errorf("root admission requires an evidence root and payload SHA-256: %w", errs.ErrUsage)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := apprelease.VerifyCanonicalPayload(in.EvidenceRoot, in.PayloadDigest); err != nil {
		return err
	}

	root, err := pathsafe.OpenRoot(in.EvidenceRoot)
	if err != nil {
		return err
	}

	defer func() { _ = root.Close() }()

	entry, err := readRootPayloadLedger(root, plan, in.Plan.ImageRepository)
	if err != nil {
		return err
	}

	return appcontainer.ValidatePinnedImageSBOM(ctx, in.EvidenceRoot, entry)
}

func readRootPayloadLedger(root *os.Root, plan pipeline.ReleaseImagesPlan, repository string) (imageledger.Entry, error) {
	var empty imageledger.Entry

	files, err := fs.ReadDir(root.FS(), "dist")
	if err != nil {
		return empty, err
	}

	if len(files) != 1 || files[0].Name() != "base-images.json" {
		return empty, fmt.Errorf("root payload dist must contain only base-images.json: %w", errs.ErrValidation)
	}

	ledger, err := root.ReadFile("dist/base-images.json")
	if err != nil {
		return empty, err
	}

	entries, err := AdmitBaseImageLedger(ledger, repository)
	if err != nil {
		return empty, err
	}

	if len(entries) != 1 {
		return empty, fmt.Errorf("root payload requires exactly one ledger entry: %w", errs.ErrValidation)
	}

	_, expected, err := admitRootBasePublication(PublishRootBaseInput{Plan: plan, ExpectedRepository: repository})
	if err != nil {
		return empty, err
	}
	// Only publication results are absent from the approved plan. All other
	// ledger fields, including own ID, paths, tags and provenance, must agree.
	actual := entries[0]
	expected.Ref, expected.Digest, expected.SBOMSHA256 = actual.Ref, actual.Digest, actual.SBOMSHA256

	want, err := imageledger.Marshal([]imageledger.Entry{expected})
	if err != nil {
		return empty, err
	}

	got, err := imageledger.Marshal(entries)
	if err != nil {
		return empty, err
	}

	if !bytes.Equal(want, got) {
		return empty, fmt.Errorf("root ledger differs from approved plan: %w", errs.ErrValidation)
	}

	return actual, nil
}
