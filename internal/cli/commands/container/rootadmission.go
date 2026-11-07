// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package container

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	appbaseimages "github.com/diggsweden/forge-ci/internal/app/baseimages"
	"github.com/diggsweden/forge-ci/internal/cliio"
)

func rootBaseAdmissionCmd(payload bool) *cli.Command {
	name, usage := "admit-plan", "check a cold root plan against independent exact-byte and source expectations (read-only)"
	flags := []cli.Flag{
		&cli.StringFlag{Name: "plan", Required: true, Usage: "exact approved frozen root plan file"},
		&cli.StringFlag{Name: "image-repository", Required: true, Usage: "exact unsigned staging repository"},
		&cli.StringFlag{Name: "expected-plan-sha256", Required: true, Usage: "independently approved SHA-256 of exact plan bytes; never a producer self-approval"},
		&cli.StringFlag{Name: "source", Required: true, Usage: "independently authenticated producer source repository URL (not inferred from plan)"},
		&cli.StringFlag{Name: "source-sha", Required: true, Usage: "authenticated producer checkout commit, distinct from opaque plan content revision"},
		&cli.StringFlag{Name: "workflow", Required: true, Usage: "authenticated producer workflow path"},
		&cli.StringFlag{Name: "expected-source", Required: true, Usage: "protected caller's approved HTTPS source repository URL"},
		&cli.StringFlag{Name: "expected-source-sha", Required: true, Usage: "protected caller's approved checkout commit"},
		&cli.StringFlag{Name: "expected-workflow", Required: true, Usage: "protected caller's approved producer workflow path"},
	}

	if payload {
		name, usage = "admit-payload", "check a cold root canonical handoff, approved plan, sole root ledger and producer SBOM pins (read-only)"

		flags = append(flags,
			&cli.StringFlag{Name: "evidence-root", Required: true, Usage: "verified canonical unsigned workspace containing dist/base-images.json and premade-sboms/"},
			&cli.StringFlag{Name: "expected-payload-digest", Required: true, Usage: "canonical payload SHA-256 from the authenticated exact publisher job; not source authentication"},
		)
	}

	return &cli.Command{
		Name: name, Usage: usage,
		Description: "No credentials, registry access, key loading, writes or SBOM regeneration. The caller must authenticate the exact producer project/pipeline/job and policy revision, obtain observed source facts from that authenticated context, and supply independently protected expectations. Comparing caller-supplied facts is not producer authentication. Exact plan approval constrains content revisions without requiring them to equal the checkout commit. Run before import or signing; success grants no signing authority.",
		Flags:       flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			body, err := cliio.ReadRegularFile(cmd.String("plan"))
			if err != nil {
				return err
			}

			in := appbaseimages.RootBasePlanAdmission{
				Plan: body, PlanSHA256: cmd.String("expected-plan-sha256"), ImageRepository: cmd.String("image-repository"),
				Source:   appbaseimages.RootBaseSource{Repository: cmd.String("source"), Commit: cmd.String("source-sha"), Workflow: cmd.String("workflow")},
				Expected: appbaseimages.RootBaseSource{Repository: cmd.String("expected-source"), Commit: cmd.String("expected-source-sha"), Workflow: cmd.String("expected-workflow")},
			}
			if payload {
				err = appbaseimages.AdmitRootBasePayload(ctx, appbaseimages.RootBasePayloadAdmission{
					Plan: in, EvidenceRoot: cmd.String("evidence-root"), PayloadDigest: cmd.String("expected-payload-digest"),
				})
			} else {
				_, err = appbaseimages.AdmitRootBasePlan(in)
			}

			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(os.Stdout, "root admission: approved bytes and supplied producer facts agree; producer authentication remains caller-owned")

			return err
		},
	}
}
