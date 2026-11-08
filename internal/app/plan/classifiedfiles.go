// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package plan

import (
	"fmt"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/build"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
)

// ClassifiedFileAdmissionInput carries existing contracts and caller-owned
// decisions, not a new wire plan. ProducerFileOnly must be an administrator's
// explicit declaration; this function cannot authenticate its owner or inspect
// an opaque producer script. Any separately supplied image plan is unsupported.
type ClassifiedFileAdmissionInput struct {
	ConfigPlanJSON          string
	ReleasePlanJSON         string
	ProducerFileOnly        bool
	ReleaseSBOMs            string
	GoTestSelectionsJSON    string
	CargoTestSelectionsJSON string
	ImagePlan               *pipeline.ReleaseImagesPlan
}

// AdmitClassifiedFiles validates route eligibility before producer scheduling.
// It performs no I/O, emits no authorization token, runs no tests and changes no
// native defaults. The native manifest's SBOM roles are outside this predicate.
// Application-test enforcement cannot waive the required policy selections.
func AdmitClassifiedFiles(in ClassifiedFileAdmissionInput) error {
	if !in.ProducerFileOnly {
		return fmt.Errorf("classified files need an explicit administrator file-only producer declaration: %w", errs.ErrUsage)
	}

	if in.ImagePlan != nil {
		return fmt.Errorf("classified files do not accept image, base or flavor plans, even empty ones: %w", errs.ErrInvalidConfig)
	}

	if strings.TrimSpace(in.ConfigPlanJSON) == "" || strings.TrimSpace(in.ReleasePlanJSON) == "" {
		return fmt.Errorf("config-plan-json and release-plan-json are required: %w", errs.ErrUsage)
	}

	cfg, err := pipeline.DecodeConfigPlan(in.ConfigPlanJSON)
	if err != nil {
		return err
	}

	release, err := pipeline.DecodeClassifiedFileReleasePlan(in.ReleasePlanJSON)
	if err != nil {
		return err
	}

	artifact, err := pipeline.ValidateClassifiedFilePlans(cfg, release, in.ReleaseSBOMs)
	if err != nil {
		return err
	}

	goSelections, cargoSelections, err := pipeline.ReleaseTestSelections(release.Stages.Build, in.GoTestSelectionsJSON, in.CargoTestSelectionsJSON)
	if err != nil {
		return err
	}

	if artifact.ProjectType == projecttype.Go {
		if goSelections[artifact.Name].Gate != build.PolicyTestGate {
			return fmt.Errorf("classified Go requires an explicit policy-gated administrator selection: %w", errs.ErrUsage)
		}

		return nil
	}

	for _, run := range cargoSelections[artifact.Name].Runs {
		if run.Gate == build.PolicyTestGate {
			return nil
		}
	}

	return fmt.Errorf("classified Cargo requires an explicit administrator selection including a policy executable: %w", errs.ErrUsage)
}
