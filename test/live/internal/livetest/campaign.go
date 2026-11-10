// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// A live campaign is the inputs of one commit, which `just campaign-prepare`
// writes to a directory: the images and demo checkouts the cases of the Go
// tables (PreparedCases, PreparedLanes) run on. This file reads its
// campaign.env as data and says which runtime images a campaign prepares.
// docs/live-campaigns.md, Run a campaign, shows the use.

// CampaignEnvFile is the campaign's KEY=value file, in its directory.
const CampaignEnvFile = "campaign.env"

// campaignCommitKey names the commit a campaign was prepared from.
const campaignCommitKey = "CAMPAIGN_COMMIT"

// The providers a campaign has lanes on.
const (
	campaignForgejo = "forgejo"
	campaignGitLab  = "gitlab"
)

func isCampaignProvider(provider string) bool {
	return provider == campaignForgejo || provider == campaignGitLab
}

// campaignScenarioTarget is the runtime target of FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE,
// the job image of the GitLab scenarios.
const campaignScenarioTarget = "go"

// The campaign.env keys of a nanolinter job image prepared with
// job-image=<flavor>:<image>, each followed by campaignFlavorKey(flavor): its
// reference, a Forgejo case's CI_JOB_IMAGE, and its archive, which a GitLab
// case's gate lints in.
const (
	campaignJobImagePrefix    = "JOB_IMAGE_"
	campaignLintArchivePrefix = "LINT_IMAGE_ARCHIVE_"
)

// campaignFlavor is a job-image= flavor, as campaign prepare accepts it.
var campaignFlavor = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// campaignKey is a key of campaign.env, campaignDemo the name of a demo.
var (
	campaignKey  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	campaignDemo = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// campaignFlavorKey is a flavor in campaign.env's keys: capitals, '-' as '_'.
func campaignFlavorKey(flavor string) string {
	return strings.ToUpper(strings.ReplaceAll(flavor, "-", "_"))
}

// ReadCampaignEnv reads KEY=value lines, skipping blank lines and # comments.
// Keys are capitals, digits and underscores, each set once, to a value that
// is not empty. Nothing is evaluated.
func ReadCampaignEnv(path string) (map[string]string, error) {
	env := map[string]string{}

	err := eachLine(path, func(number int, line string) error {
		if line == "" || strings.HasPrefix(line, "#") {
			return nil
		}

		key, value, found := strings.Cut(line, "=")
		switch {
		case !found || !campaignKey.MatchString(key):
			return fmt.Errorf("%s:%d: want KEY=value, KEY in capitals, digits and underscores: %w", path, number, errs.ErrValidation)
		case value == "" || strings.Contains(value, "\r"):
			return fmt.Errorf("%s:%d: %s is empty or not one line: %w", path, number, key, errs.ErrValidation)
		}

		if _, twice := env[key]; twice {
			return fmt.Errorf("%s:%d: %s is set twice: %w", path, number, key, errs.ErrValidation)
		}

		env[key] = value

		return nil
	})
	if err != nil {
		return nil, err
	}

	if env[campaignCommitKey] == "" {
		return nil, fmt.Errorf("%s names no %s: %w", path, campaignCommitKey, errs.ErrValidation)
	}

	return env, nil
}

// CampaignRuntimeTargets lists the runtime targets a campaign prepares image
// archives of, from the repository at root, for what it is named to run: a
// forge, every lane of it, or a lane of PreparedLanes by name. A GitLab lane
// takes the go image when it runs scenarios, and for each of its cases the
// image of the case's flavor or its Runtime, every $CI_JOB_IMAGE_<TARGET> the
// gate of its demo checks in and, for a snapshot, which runs no gate, the
// toolchain family of each producer its plan builds. A Forgejo lane takes
// none. Each must be a target of containers/runtime/images.json. Every
// nanolinter image a case takes, a Forgejo case's job image and what a GitLab
// gate lints in, must be of one of flavors, the job-image= flavors the
// campaign publishes.
func CampaignRuntimeTargets(root string, names, flavors []string) ([]string, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("name what the campaign prepares for: forgejo, gitlab or a lane: %w", errs.ErrValidation)
	}

	for _, flavor := range flavors {
		if !campaignFlavor.MatchString(flavor) {
			return nil, fmt.Errorf("a job image flavor is lowercase letters, digits and hyphens, got %q: %w", flavor, errs.ErrValidation)
		}
	}

	body, err := os.ReadFile(filepath.Join(root, "containers", "runtime", "images.json")) //nolint:gosec // This repository's roster, read as data.
	if err != nil {
		return nil, err
	}

	var roster []struct {
		Target string `json:"target"`
	}
	if err = json.Unmarshal(body, &roster); err != nil {
		return nil, fmt.Errorf("containers/runtime/images.json: %w", err)
	}

	targets := map[string]bool{}

	for _, name := range names {
		lanes := slices.DeleteFunc(slices.Clone(PreparedLanes), func(lane PreparedLane) bool { return name != lane.Forge && name != lane.Name })
		if len(lanes) == 0 {
			return nil, fmt.Errorf("want forgejo, gitlab or a lane of them, got %q: %w", name, errs.ErrValidation)
		}

		for _, lane := range lanes {
			if err := laneRuntimeTargets(root, lane, flavors, targets); err != nil {
				return nil, err
			}
		}
	}

	for target := range targets {
		if !slices.ContainsFunc(roster, func(image struct {
			Target string `json:"target"`
		}) bool {
			return image.Target == target
		}) {
			return nil, fmt.Errorf("the campaign names the runtime target %q, which containers/runtime/images.json does not build: %w", target, errs.ErrValidation)
		}
	}

	return slices.Sorted(maps.Keys(targets)), nil
}

// laneRuntimeTargets adds the runtime targets a lane needs, and refuses a
// case whose nanolinter image is of a flavor not among flavors.
func laneRuntimeTargets(root string, lane PreparedLane, flavors []string, targets map[string]bool) error {
	if lane.Forge == campaignGitLab && len(lane.Scenarios) != 0 {
		targets[campaignScenarioTarget] = true
	}

	for _, c := range PreparedCases(lane.Forge) {
		if lane.Lane == "" || c.Lane != lane.Lane {
			continue
		}

		// A GitLab snapshot runs no gate, so it lints in nothing.
		lints := lane.Forge == campaignForgejo || c.Event != TrialSnapshot
		if lints && !slices.Contains(flavors, c.JobImage) {
			return fmt.Errorf("%s: %s takes a nanolinter job image the campaign does not publish; add job-image=%s:<local image>: %w",
				lane.Name, c.Name(), c.JobImage, errs.ErrValidation)
		}

		if lane.Forge != campaignGitLab {
			continue
		}

		targets[cmp.Or(c.Runtime, gitLabRuntimeTarget[c.JobImage])] = true

		further, err := gitLabCaseFurtherImages(demoConsumerDir(root, c.Demo), c.Event)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", lane.Name, c.Name(), err)
		}

		for _, name := range further {
			targets[strings.ToLower(name)] = true
		}
	}

	return nil
}

// gitLabCaseFurtherImages names the runtime images a GitLab case of event on
// the demo at checkout runs in beside its own: the toolchain family of each
// producer a snapshot's plan builds, or every $CI_JOB_IMAGE_<NAME> the
// demo's gate checks in.
func gitLabCaseFurtherImages(checkout, event string) ([]string, error) {
	if event == TrialSnapshot {
		plan, err := snapshotImagesAdmission(checkout)
		if err != nil {
			return nil, err
		}

		return serviceSnapshotFamilies(plan), nil
	}

	gate, err := readDemoGate(filepath.Join(checkout, filepath.FromSlash(GitLabGateFile)))

	return gate.extraJobs, err
}

// demosDir holds the shipped demos, relative to the repository root.
const demosDir = "test/live/fixtures/consumers"

// demoConsumerDir is the consumer checkout a shipped demo is run from.
func demoConsumerDir(root, demo string) string {
	return filepath.Join(root, filepath.FromSlash(demosDir), demo, "consumer")
}

// GitLabSnapshotProducerArchiveEnv names the variable of the runtime image
// archive a GitLab snapshot builds a producer of the toolchain family in:
// the family is the runtime target of its toolchain (containers/runtime).
func GitLabSnapshotProducerArchiveEnv(family string) string {
	return GitLabJobImageArchiveEnv + strings.ToUpper(family)
}

// requireGitLabReleaseBuild refuses a GitLab release of a demo without the
// build the release policy's build job runs, GitLabConsumerBuildScript: the
// release commands are the adopter's, and the example demos ship their
// merge-request gate alone (test/live/fixtures/consumers/README.md), so their GitLab
// release is not a case to run.
func requireGitLabReleaseBuild(checkout string) error {
	info, err := os.Lstat(filepath.Join(checkout, GitLabConsumerBuildScript))
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("the demo has no %s, so no GitLab release: its release commands are the adopter's (test/live/fixtures/consumers/README.md): %w", GitLabConsumerBuildScript, errs.ErrValidation)
	}

	return nil
}

// demoGate is what a demo's GitLab gate runs in beside $CI_JOB_IMAGE: the
// NAME of each $CI_JOB_IMAGE_<NAME> it checks in, and whether it lints in
// $CI_LINT_IMAGE.
type demoGate struct {
	extraJobs []string
	lints     bool
}

// readDemoGate reads the demo's GitLab gate at path; a demo without one runs
// in nothing further.
func readDemoGate(path string) (demoGate, error) {
	body, err := os.ReadFile(path) //nolint:gosec // A demo of this repository, read as data.
	if errors.Is(err, fs.ErrNotExist) {
		return demoGate{}, nil
	}

	if err != nil {
		return demoGate{}, err
	}

	var gate demoGate
	if gate.extraJobs, err = gitlabGateExtraJobImages(body); err == nil {
		gate.lints, err = gitlabGateLints(body)
	}

	if err != nil {
		return demoGate{}, fmt.Errorf("%s: %w", path, err)
	}

	return gate, nil
}

// eachLine calls visit with each line of the file at path, numbered from 1.
func eachLine(path string, visit func(int, string) error) error {
	file, err := os.Open(path) //nolint:gosec // Operator-named campaign input, read as data.
	if err != nil {
		return err
	}

	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for number := 1; scanner.Scan(); number++ {
		if strings.ContainsRune(scanner.Text(), 0) {
			return fmt.Errorf("%s:%d: a NUL byte: %w", path, number, errs.ErrValidation)
		}

		if err := visit(number, scanner.Text()); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
