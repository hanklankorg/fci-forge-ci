// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package plan

import (
	"context"

	"github.com/urfave/cli/v3"

	appplan "github.com/diggsweden/forge-ci/internal/app/plan"
)

func validateClassifiedFilesCmd() *cli.Command {
	return &cli.Command{
		Name:  "validate-classified-files",
		Usage: "validate existing root Go/Cargo plans for the opt-in classified file-only route",
		Description: `Consumes trusted, already generated config and whole release plans; the release
plan includes its prepare, build, publish and transfer contracts. This does not
compose plans, execute producers or tests, authenticate administrator ownership,
or authorize signing/publication. No separate image/base/flavor plan is supported.
The config source must already have passed its supported schema owner.

Success is exit zero with no outputs or admission token. Failure has no fallback
to native planning. Selections must include mandatory policy tests; the file-only
declaration cannot prove that an opaque producer actually performs only file work.

EXAMPLE:
   forge-ci --provider local --runner local plan validate-classified-files \
     --config-plan-json "$config_plan" --release-plan-json "$release_plan" \
     --producer-file-only --release-sboms none \
     --go-test-selections-json '{"product":{"packages":["./test/policy"],"gate":"policy"}}'`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: configPlanJSONFlag, Required: true, Usage: "trusted typed config-plan JSON from config parse-artifacts"},
			&cli.StringFlag{Name: "release-plan-json", Required: true, Usage: "trusted whole release-plan JSON from plan release, including all stages"},
			&cli.BoolFlag{Name: "producer-file-only", Usage: "explicit administrator declaration that the producer has only file capabilities (not proof of ownership or execution)"},
			&cli.StringFlag{Name: "release-sboms", Required: true, Usage: "original release-level SBOM request; this route requires none"},
			&cli.StringFlag{Name: "go-test-selections-json", Usage: "complete trusted administrator Go selection map"},
			&cli.StringFlag{Name: "cargo-test-selections-json", Usage: "complete trusted administrator Cargo selection map"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return appplan.AdmitClassifiedFiles(appplan.ClassifiedFileAdmissionInput{
				ConfigPlanJSON:          cmd.String(configPlanJSONFlag),
				ReleasePlanJSON:         cmd.String("release-plan-json"),
				ProducerFileOnly:        cmd.Bool("producer-file-only"),
				ReleaseSBOMs:            cmd.String("release-sboms"),
				GoTestSelectionsJSON:    cmd.String("go-test-selections-json"),
				CargoTestSelectionsJSON: cmd.String("cargo-test-selections-json"),
			})
		},
	}
}
