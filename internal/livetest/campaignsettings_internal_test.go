// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

func TestCampaignDeclarationsRefuseTheWholeLaneOnMismatch(t *testing.T) {
	t.Parallel()

	dir, path := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", "setting visibility=private\nsetting image-builder=kaniko\nscenario TestOne\nscenario TestTwo\n")
	for name, settings := range map[string]map[string]string{
		"missing":    nil,
		"visibility": {"visibility": "both", "image-builder": "kaniko"},
		"builder":    {"visibility": "private", "image-builder": "buildah"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			plan, err := PlanCampaignLane(reporoot.Path(t), dir, "forgejo", path, campaignHead, CampaignRun{Settings: settings})
			if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || len(plan.Env) != 0 {
				t.Fatalf("mismatched lane returned plan=%+v err=%v", plan, err)
			}
		})
	}

	settings := map[string]string{"visibility": "private", "image-builder": "kaniko"}

	plan, err := PlanCampaignLane(reporoot.Path(t), dir, "forgejo", path, campaignHead, CampaignRun{Settings: settings, BuilderSelector: "kaniko"})
	if err != nil || len(plan.Cases) != 2 || plan.Cases[0].Text != "scenario TestOne" || plan.Cases[1].Text != "scenario TestTwo" {
		t.Fatalf("matching declarations became cases or changed their order: plan=%+v err=%v", plan, err)
	}

	if !reflect.DeepEqual(settings, map[string]string{"visibility": "private", "image-builder": "kaniko"}) {
		t.Fatal("declarations changed the caller's settings")
	}
}

func TestCampaignDeclarationGrammarIsSharedWithPreparation(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"no case":   "setting visibility=private\n",
		"duplicate": "setting visibility=private\nsetting visibility=private\nscenario TestOne\n",
		"late":      "scenario TestOne\nsetting visibility=private\n",
		"empty":     "setting visibility=\nscenario TestOne\n",
		"expanded":  "setting visibility=@VISIBILITY\nscenario TestOne\n",
		"extra":     "setting visibility=private another=value\nscenario TestOne\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, path := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", body)
			if _, err := readCampaignCases(path); !errors.Is(err, errs.ErrValidation) {
				t.Fatalf("preparation reader accepted declaration: %v", err)
			}

			if _, err := PlanCampaignLane(reporoot.Path(t), dir, "forgejo", path, campaignHead, CampaignRun{Settings: map[string]string{"visibility": "private"}}); !errors.Is(err, errs.ErrValidation) {
				t.Fatalf("lane admission accepted declaration: %v", err)
			}
		})
	}
}

func TestCampaignDeclarationAdmissionIsNotTiedToIndividualSettings(t *testing.T) {
	t.Parallel()

	dir, path := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", "setting future-setting=value\nscenario TestOne\n")
	if _, err := PlanCampaignLane(reporoot.Path(t), dir, "forgejo", path, campaignHead, CampaignRun{Settings: map[string]string{"future-setting": "value"}}); err != nil {
		t.Fatalf("a caller-supplied run setting required a new case-parser branch: %v", err)
	}

	if _, err := PlanCampaignLane(reporoot.Path(t), dir, "forgejo", path, campaignHead, CampaignRun{Settings: map[string]string{"visibility": "private"}}); !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("unknown run setting was accepted: %v", err)
	}
}

// An administrator setting is declared only where an orchestrator of the
// lane's forge declares its input, read from this checkout: GitLab's
// components take all four, Forgejo's workflows and actions no
// enforce-reproducible-builds. Preparation and the lane refuse alike.
func TestCampaignAdminDeclarationsNameConsumedInputsOnly(t *testing.T) {
	t.Parallel()

	root := reporoot.Path(t)
	reproducible := map[string]string{"admin-reproducible-builds": "true"}

	dir, path := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", "setting admin-reproducible-builds=true\nscenario TestOne\n")
	if _, err := PlanCampaignLane(root, dir, "gitlab", path, campaignHead, CampaignRun{Settings: reproducible}); err != nil {
		t.Fatalf("GitLab's release component takes enforce-reproducible-builds: %v", err)
	}

	for name, err := range map[string]error{
		"lane":        must(PlanCampaignLane(root, dir, "forgejo", path, campaignHead, CampaignRun{Settings: reproducible})),
		"preparation": must(CampaignRuntimeTargets(root, []string{"forgejo=" + path}, nil)),
	} {
		if !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), "enforce-reproducible-builds") || !strings.Contains(err.Error(), "forgejo") {
			t.Errorf("%s: a Forgejo lane declaring reproducible builds: %v; want a refusal naming the input nothing consumes", name, err)
		}
	}

	consumed := map[string]string{"admin-lint": "false", "admin-vulnerabilities": "false", "admin-application-tests": "false"}

	dir, path = newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n", "setting admin-lint=false\nsetting admin-vulnerabilities=false\nsetting admin-application-tests=false\nscenario TestOne\n")
	if _, err := PlanCampaignLane(root, dir, "forgejo", path, campaignHead, CampaignRun{Settings: consumed}); err != nil {
		t.Fatalf("Forgejo's workflows and actions take the other three inputs: %v", err)
	}
}

// must keeps only the error of a two-valued call, for a table of refusals.
func must[T any](_ T, err error) error { return err }

func TestCampaignRunSettingsUseTheSameDefaultsAsTheScenarios(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ forge, selector string }{
		{"forgejo", ForgejoImageBuilderEnv},
		{"gitlab", GitLabImageBuilderEnv},
	} {
		t.Run(tc.forge, func(t *testing.T) {
			t.Parallel()

			defaults, err := CampaignRunSettings(tc.forge, func(string) string { return "" })
			if err != nil || defaults.Settings["visibility"] != "both" || defaults.Settings["image-builder"] != "buildah" || defaults.BuilderSelector != "" {
				t.Fatalf("defaults=%v err=%v", defaults, err)
			}

			environment := map[string]string{consumerVisibilityEnv: "private", tc.selector: "kaniko"}

			selected, err := CampaignRunSettings(tc.forge, func(key string) string { return environment[key] })
			if err != nil || selected.Settings["visibility"] != "private" || selected.Settings["image-builder"] != "kaniko" || selected.BuilderSelector != "kaniko" {
				t.Fatalf("selection=%v err=%v", selected, err)
			}
		})
	}
}

func TestCampaignAdmissionChecksTheEffectiveBuilderOfEveryConsumer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, declaration, selector, consumer string
		refused                               bool
	}{
		{"unset selector with contradictory override", "buildah", "", "CI_IMAGE_BUILDER=kaniko", true},
		{"selected kaniko with default consumer", "kaniko", "kaniko", "", true},
		{"selected kaniko with contradictory override", "kaniko", "kaniko", "CI_IMAGE_BUILDER=buildah", true},
		{"explicit selector without declaration", "", "kaniko", "", true},
		{"invalid consumer builder", "", "", "CI_IMAGE_BUILDER=unknown", true},
		{"matching defaults", "buildah", "", "", false},
		{"matching explicit selections", "kaniko", "kaniko", "CI_IMAGE_BUILDER=kaniko", false},
		{"consumer selection without run requirement", "", "", "CI_IMAGE_BUILDER=kaniko", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			declaration := ""
			if tc.declaration != "" {
				declaration = "setting image-builder=" + tc.declaration + "\n"
			}

			dir, path := newCampaign(t, "CAMPAIGN_COMMIT="+campaignHead+"\n",
				declaration+"scenario TestFirst\nconsumer hello-go-cli pull-request "+tc.consumer+"\n")

			run, err := CampaignRunSettings("forgejo", func(key string) string {
				if key == ForgejoImageBuilderEnv {
					return tc.selector
				}

				return ""
			})
			if err != nil {
				t.Fatal(err)
			}

			plan, err := PlanCampaignLane(reporoot.Path(t), dir, "forgejo", path, campaignHead, run)
			if tc.refused {
				if !errors.Is(err, errs.ErrValidation) || len(plan.Cases) != 0 || len(plan.Env) != 0 {
					t.Fatalf("conflicting later case left executable work: plan=%+v err=%v", plan, err)
				}

				return
			}

			if err != nil || len(plan.Cases) != 2 {
				t.Fatalf("valid consumer selection refused: plan=%+v err=%v", plan, err)
			}
		})
	}
}
