// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
)

// sbomBundle is whether a release must offer its SBOM bundle,
// <project>-<version>-sboms.zip.
type sbomBundle int

const (
	// sbomBundleUnjudged: the trial cannot tell, because no declaration names
	// the artifacts' layers or only a container's layer remains.
	sbomBundleUnjudged sbomBundle = iota
	// sbomBundleOffered: the release policy and the artifacts share a layer
	// every artifact produces.
	sbomBundleOffered
	// sbomBundleWithheld: release.sboms and the artifacts share no layer.
	sbomBundleWithheld
)

const sbomBundleSuffix = "-sboms.zip"

// The release orchestrator inputs the profile reads, and their defaults.
const (
	inputSBOMs            = "release.sboms"
	inputChangelogCreator = "changelog-creator"
	inputSkipVersionBump  = "changelog.skipversionbump"
	inputSBOMsDefault     = "all"
	inputGitCliff         = "git-cliff"
)

// githubReleaseProfile is what a fixture's release workflow and its artifacts
// configuration promise of a GitHub release, read from the rendered consumer
// the trial pushed, beyond the artifact sign method githubReleaseSignMethod
// reads: how its Git objects are signed, whether it prepares a commit, and
// whether it offers the SBOM bundle.
type githubReleaseProfile struct {
	GitSigning config.GitSignMethod
	// Prepares is whether the release commits the changelog and the version.
	// Without a changelog creator, or with the version bump skipped, it
	// tags the requested commit and main stays where it was.
	Prepares   bool
	SBOMBundle sbomBundle
}

// githubReleaseProfileOf reads the profile of the release workflow in the
// staged consumer at dir. The orchestrator's own defaults apply to an input
// the workflow does not pass.
func githubReleaseProfileOf(dir, workflow string) (githubReleaseProfile, error) {
	body, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", filepath.Base(workflow))) //nolint:gosec // the staged consumer's own workflow, named by the trial.
	if err != nil {
		return githubReleaseProfile{}, fmt.Errorf("release workflow: %w", err)
	}

	declared, err := githubReleaseConfig(dir, workflow, string(body))
	if err != nil {
		return githubReleaseProfile{}, err
	}

	inputs, err := githubOrchestratorInputs(workflow, string(body))
	if err != nil {
		return githubReleaseProfile{}, err
	}

	profile := githubReleaseProfile{GitSigning: config.DefaultGitSignMethod}
	if declared != nil {
		profile.GitSigning = declared.GitSigning.EffectiveMethod()
	}

	profile.Prepares = pipeline.VersionBumpRuns(inputs.changelogCreator == inputGitCliff, inputs.skipVersionBump)

	profile.SBOMBundle, err = releaseSBOMBundle(inputs.sboms, declared)
	if err != nil {
		return githubReleaseProfile{}, fmt.Errorf("release workflow %s: %w", workflow, err)
	}

	return profile, nil
}

// releaseSBOMBundle intersects the workflow's release.sboms with the layers
// the declared artifacts request, as the release plan does.
func releaseSBOMBundle(releaseSBOMs string, declared *config.Config) (sbomBundle, error) {
	release, err := config.ExpandSBOMs(releaseSBOMs)
	if err != nil {
		return sbomBundleUnjudged, err
	}

	if len(release) == 0 {
		return sbomBundleWithheld, nil
	}

	if declared == nil {
		return sbomBundleUnjudged, nil
	}

	// The layers each artifact effectively requests, with the project type's
	// default where it names none.
	if err := config.Derive(declared); err != nil {
		return sbomBundleUnjudged, err
	}

	requested, err := config.ExpandSBOMs(config.PipelineSBOMs(declared.Artifacts))
	if err != nil {
		return sbomBundleUnjudged, err
	}

	bundle := sbomBundleWithheld

	for _, layer := range release {
		switch {
		case !slices.Contains(requested, layer):
		case layer == config.SBOMLayerAnalyzedContainer:
			// Only a container produces this layer, and the bundle is made
			// from the SBOMs that exist.
			if bundle == sbomBundleWithheld {
				bundle = sbomBundleUnjudged
			}
		default:
			bundle = sbomBundleOffered
		}
	}

	return bundle, nil
}

// releaseSBOMBundleGap judges a release's assets against what its profile
// promises of the SBOM bundle.
func releaseSBOMBundleGap(expect sbomBundle, assets map[string]int64) error {
	var bundles []string

	for name := range assets {
		if strings.HasSuffix(name, sbomBundleSuffix) {
			bundles = append(bundles, name)
		}
	}

	slices.Sort(bundles)

	switch {
	case expect == sbomBundleOffered && len(bundles) != 1:
		return fmt.Errorf("offers %d SBOM bundles %v, want one *%s: %w", len(bundles), bundles, sbomBundleSuffix, errReleaseEvidence)
	case expect == sbomBundleWithheld && len(bundles) != 0:
		return fmt.Errorf("offers the SBOM bundle %v although its SBOM policy selects no layer: %w", bundles, errReleaseEvidence)
	}

	return nil
}

// orchestratorInputs are the release orchestrator's inputs a profile reads,
// with the orchestrator's defaults.
type orchestratorInputs struct {
	sboms            string
	changelogCreator string
	skipVersionBump  bool
}

// githubOrchestratorInputs reads them from the one job that calls the release
// orchestrator. A workflow that calls none keeps the defaults; a value that is
// not a literal cannot be judged and is refused.
func githubOrchestratorInputs(name, body string) (orchestratorInputs, error) {
	var workflow struct {
		Jobs map[string]struct {
			Uses string         `yaml:"uses"`
			With map[string]any `yaml:"with"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal([]byte(body), &workflow); err != nil {
		return orchestratorInputs{}, fmt.Errorf("release workflow %s: %w", name, err)
	}

	inputs, calls := orchestratorInputs{sboms: inputSBOMsDefault, changelogCreator: inputGitCliff}, 0

	for _, job := range workflow.Jobs {
		if !strings.Contains(job.Uses, "/release-orchestrator.yml") {
			continue
		}

		calls++

		for input, value := range job.With {
			text := strings.TrimSpace(fmt.Sprint(value))
			if value == nil {
				text = ""
			}

			if input != inputSBOMs && input != inputChangelogCreator && input != inputSkipVersionBump {
				continue
			}

			if strings.Contains(text, "${{") {
				return orchestratorInputs{}, fmt.Errorf("release workflow %s passes %s as an expression; the oracle needs a literal: %w", name, input, errs.ErrValidation)
			}

			switch input {
			case inputSBOMs:
				inputs.sboms = text
			case inputChangelogCreator:
				inputs.changelogCreator = text
			case inputSkipVersionBump:
				inputs.skipVersionBump = text == strconv.FormatBool(true)
			}
		}
	}

	if calls > 1 {
		return orchestratorInputs{}, fmt.Errorf("release workflow %s calls the release orchestrator %d times; the oracle needs one: %w", name, calls, errs.ErrValidation)
	}

	return inputs, nil
}

// gitSignatureKind names the format of an armored Git object signature, as
// GitHub's API and the object itself carry it.
func gitSignatureKind(signature string) config.GitSignMethod {
	switch {
	case strings.Contains(signature, "-----BEGIN SSH SIGNATURE-----"):
		return config.GitSignSSH
	case strings.Contains(signature, "-----BEGIN PGP SIGNATURE-----"):
		return config.GitSignGPG
	default:
		return ""
	}
}

// gitSignatureGap verifies the signature of a commit or tag in the working
// copy at dir against the release's own trust: for gpg, a valid signature by
// the trial's GPG key; for ssh, a good signature by a principal of the
// allowed-signers file. kind is "commit" or "tag".
func (trial *GitHubTrial) gitSignatureGap(dir, kind, object string, method config.GitSignMethod, key TrialGPGKey, allowedSigners string) error {
	raw, err := trial.outputEnv(dir, trial.gitEnv(), "git", "-C", dir, "cat-file", kind, object)
	if err != nil {
		return fmt.Errorf("read the %s %s: %w: %s", kind, object, err, raw)
	}

	if got := gitSignatureKind(raw); got != method {
		return fmt.Errorf("the %s %s carries a %q signature, and the release declares git-signing %s: %w", kind, object, got, method, errReleaseEvidence)
	}

	if method == config.GitSignSSH {
		output, err := trial.outputEnv(dir, trial.gitEnv(), "git", "-C", dir, "-c", "gpg.format=ssh", "-c", "gpg.ssh.allowedSignersFile="+allowedSigners, "verify-"+kind, object)
		if err != nil || !strings.Contains(output, `Good "git" signature for `) {
			return fmt.Errorf("the SSH signature of the %s %s is not by an allowed signer: %s: %w", kind, object, strings.TrimSpace(output), errReleaseEvidence)
		}

		return nil
	}

	// The status lines, not the exit status, name the signing key.
	status, _ := trial.outputEnv(dir, append(trial.gitEnv(), "GNUPGHOME="+key.Home), "git", "-C", dir, "-c", "gpg.format=openpgp", "verify-"+kind, "--raw", object)
	if !signedBy(status, key.Fingerprint) {
		return fmt.Errorf("the GPG signature of the %s %s is not the trial key %s's: %w", kind, object, key.Fingerprint, errReleaseEvidence)
	}

	return nil
}
