// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/sbom"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// What a table's case must hold to for its live run to be worth starting,
// where the live scenario does not refuse the case itself. Each is read from
// the shipped tree, so a case that could only fail costs no run to learn.

// eachPreparedCase runs check on every case of both tables, with its demo's
// shipped consumer checkout.
func eachPreparedCase(t *testing.T, check func(t *testing.T, forge string, c PreparedCase, consumer string)) {
	t.Helper()

	for _, forge := range []string{campaignForgejo, campaignGitLab} {
		for _, c := range PreparedCases(forge) {
			t.Run(forge+"/"+c.Name(), func(t *testing.T) {
				t.Parallel()

				consumer := demoConsumerDir(reporoot.Path(t), c.Demo)
				if !isDir(consumer) {
					t.Fatalf("no shipped demo %s", c.Demo)
				}

				check(t, forge, c, consumer)
			})
		}
	}
}

// A case runs an event its demo admits: a lifecycle needs the demo's
// development route, a root Containerfile and a declaration the service
// snapshot takes, and on GitLab the release build it ends in; a snapshot a
// declaration the snapshot plans, and on Forgejo the release workflow the
// harness's is pinned by; a GitLab release the demo's release build. A
// GitLab gate, release and lifecycle hand the gate a lint archive, so the
// demo's gate lints.
func TestPreparedCasesRunWhatTheirDemoAdmits(t *testing.T) {
	t.Parallel()

	eachPreparedCase(t, func(t *testing.T, forge string, c PreparedCase, consumer string) {
		t.Helper()

		var err error

		switch c.Event {
		case TrialLifecycle:
			err = matrixLifecycleAdmission(forge, consumer)
		case TrialSnapshot:
			err = campaignAdmitSnapshot(forge, consumer)
		case TrialRelease:
			if forge == campaignGitLab {
				err = requireGitLabReleaseBuild(consumer)
			}
		case TrialPullRequest:
		default:
			t.Fatalf("an event the tables do not run: %s", c.Event)
		}

		if err != nil {
			t.Fatal(err)
		}

		if forge != campaignGitLab || c.Event == TrialSnapshot {
			return
		}

		gate, err := readDemoGate(filepath.Join(consumer, filepath.FromSlash(GitLabGateFile)))
		if err != nil {
			t.Fatal(err)
		}

		if !gate.lints {
			t.Fatal("the case hands its gate a lint archive, and the demo's gate does not lint")
		}
	})
}

// A release must not request a version its demo neither declares nor bumps
// to: the release refuses a commit that does not carry the version it is
// asked for, and that costs a live run to learn. So every release case
// either bumps or requests, explicitly or by default, the version its demo's
// manifest declares. A Forgejo release that bumps refuses a bump that
// changes nothing, so its case requests another.
func TestPreparedReleaseCasesRequestWhatTheirDemoDeclaresOrBump(t *testing.T) {
	t.Parallel()

	// Both forges' trials request 0.0.1 when no version is named.
	defaults, err := ParseTrialSettings(TrialRelease, nil)
	if err != nil {
		t.Fatal(err)
	}

	eachPreparedCase(t, func(t *testing.T, forge string, c PreparedCase, consumer string) {
		t.Helper()

		if c.Event != TrialRelease {
			return
		}

		// The GitLab release bumps what the plan of its declaration bumps,
		// the Forgejo release what the plan of the declaration its
		// prepare-release.yml call names does.
		_, artifactsConfig, err := GitLabReleaseImages(consumer)
		if err != nil {
			t.Fatal(err)
		}

		bumps, err := PlannedVersionBumps(consumer, artifactsConfig)
		if forge == campaignForgejo {
			bumps, err = ForgejoPlannedVersionBumps(consumer, defaults.Workflow)
		}

		if err != nil {
			t.Fatal(err)
		}

		requested, declared := cmp.Or(c.Version, defaults.Version), declaredDemoVersion(t, consumer)
		if declared != "" && len(bumps) == 0 && requested != declared {
			t.Errorf("the release requests %s, but the demo declares %s and its release does not bump; give the case Version %s", requested, declared, declared)
		}

		if forge == campaignForgejo && len(bumps) != 0 && requested == declared {
			t.Errorf("the release bumps to %s, which the demo already declares; Forgejo refuses a bump that changes nothing, so request another version", requested)
		}
	})
}

// declaredDemoVersion returns the version the demo's package.json, Cargo.toml
// or pom.xml (the project's own <version>) declares, or "" for none.
func declaredDemoVersion(t *testing.T, consumer string) string {
	t.Helper()

	read := func(name string) []byte {
		body, err := os.ReadFile(filepath.Join(consumer, name))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}

		return body
	}

	cargo, err := sbom.CargoTOMLVersion(read("Cargo.toml"), nil)
	if err != nil {
		t.Fatal(err)
	}

	var pom struct {
		Version string `xml:"version"`
	}

	if body := read("pom.xml"); body != nil {
		if err := xml.Unmarshal(body, &pom); err != nil {
			t.Fatal(err)
		}
	}

	return cmp.Or(sbom.PackageJSONVersion(read("package.json")), cargo, pom.Version)
}

// A scenario a lane names, and the test that runs a table's cases of an
// event, is a live test of the tier: `go test -run` passes on a name that
// matches nothing.
func TestPreparedLanesNameLiveTests(t *testing.T) {
	t.Parallel()

	var source strings.Builder

	for _, dir := range []string{"test/live/conformance", "test/live/internal/livetest"} {
		files, err := filepath.Glob(filepath.Join(reporoot.Path(t), dir, "*_test.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no tests in %s: %v", dir, err)
		}

		for _, file := range files {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			source.Write(body)
		}
	}

	names := []string{}
	for _, lane := range PreparedLanes {
		names = append(names, lane.Scenarios...)
	}

	for _, forge := range []string{campaignForgejo, campaignGitLab} {
		for _, c := range PreparedCases(forge) {
			names = append(names, PreparedCaseTest(forge, c.Event))
		}
	}

	for _, name := range names {
		if !strings.Contains(source.String(), "\nfunc "+name+"(t *testing.T) {\n") {
			t.Errorf("%s is no live test", name)
		}
	}
}

// campaign.env is read as data and never evaluated: a line that is not
// KEY=value with a capital key, an empty value, a key set twice, a NUL byte
// and a file that names no commit are refused.
func TestReadCampaignEnvRefusesWhatIsNotData(t *testing.T) {
	t.Parallel()

	const good = "# images\n\nCAMPAIGN_COMMIT=0123\nJOB_IMAGE_GO=lab/go\n"

	path := writeFile(t, filepath.Join(t.TempDir(), CampaignEnvFile), good)
	if env, err := ReadCampaignEnv(path); err != nil || len(env) != 2 || env["JOB_IMAGE_GO"] != "lab/go" {
		t.Fatalf("a campaign.env of two keys: %v %v", env, err)
	}

	for name, test := range map[string][2]string{
		"no commit":           {"JOB_IMAGE_GO=lab/go\n", "names no CAMPAIGN_COMMIT"},
		"a lowercase key":     {good + "job_image=lab/go\n", "want KEY=value"},
		"a line without =":    {good + "JOB_IMAGE_JAVA\n", "want KEY=value"},
		"a shell line":        {good + "export X=1\n", "want KEY=value"},
		"an empty value":      {good + "JOB_IMAGE_JAVA=\n", "JOB_IMAGE_JAVA is empty"},
		"a key set twice":     {good + "JOB_IMAGE_GO=lab/other\n", "JOB_IMAGE_GO is set twice"},
		"a NUL byte":          {good + "JOB_IMAGE_JAVA=lab\x00/java\n", "a NUL byte"},
		"a value in brackets": {"[images]\n" + good, "want KEY=value"},
	} {
		path := writeFile(t, filepath.Join(t.TempDir(), CampaignEnvFile), test[0])
		if _, err := ReadCampaignEnv(path); !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), test[1]) {
			t.Errorf("%s: %v, want a refusal naming %q", name, err, test[1])
		}
	}
}

// Every case's lane is one lane of its forge (PreparedLanes), which the
// testing matrices list it under, and no two lanes of a forge run the same
// cases.
func TestPreparedCasesAreInAListedLane(t *testing.T) {
	t.Parallel()

	listed := map[string]int{}
	for _, lane := range PreparedLanes {
		if lane.Lane != "" {
			listed[lane.Forge+"/"+lane.Lane]++
		}
	}

	for key, count := range listed {
		if count != 1 {
			t.Errorf("%d lanes run the cases of %s", count, key)
		}
	}

	for _, forge := range []string{campaignForgejo, campaignGitLab} {
		for _, c := range PreparedCases(forge) {
			if listed[forge+"/"+c.Lane] == 0 {
				t.Errorf("%s/%s: no lane lists it", forge, c.Name())
			}
		}
	}
}
