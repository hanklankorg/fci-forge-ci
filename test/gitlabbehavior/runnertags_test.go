// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package gitlabbehavior

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/testutil/gitlabrender"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// runnerTagsInput is how a component's job names its runner: one array
// input, interpolated whole.
var runnerTagsInput = regexp.MustCompile(`^\$\[\[ inputs\.((?:[a-z0-9]+-)*runner-tags(?:-arm64)?) \]\]$`)

// runnerTagsProblems is every job of one component whose tags are not one
// array input of the component. GitLab expands a CI variable in a tag when
// it creates the job, so a tag written as a variable lets a project, group
// or instance variable choose the runner, the one that sees a signer job's
// credentials included.
func runnerTagsProblems(path, text string) []string {
	header, body, found := strings.Cut(text, "\n---\n")
	if !found {
		return []string{path + ": not a component"}
	}

	var spec struct {
		Spec struct {
			Inputs map[string]map[string]any `yaml:"inputs"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal([]byte(header), &spec); err != nil {
		return []string{fmt.Sprintf("%s: %v", path, err)}
	}

	var jobs map[string]any

	if err := yaml.Unmarshal([]byte(body), &jobs); err != nil {
		return []string{fmt.Sprintf("%s: %v", path, err)}
	}

	var problems []string

	for name, value := range jobs {
		job, isJob := value.(map[string]any)
		if !isJob {
			continue
		}

		tags, named := job["tags"]
		if !named {
			continue
		}

		text, _ := tags.(string)

		match := runnerTagsInput.FindStringSubmatch(text)
		if match == nil {
			problems = append(problems, fmt.Sprintf("%s: job %s takes its tags %v from no runner-tags input", path, name, tags))

			continue
		}

		if input := spec.Spec.Inputs[match[1]]; input["type"] != "array" || !reflect.DeepEqual(input["default"], []any{}) {
			problems = append(problems, fmt.Sprintf("%s: job %s: input %s must be an array that defaults to untagged", path, name, match[1]))
		}
	}

	slices.Sort(problems)

	return problems
}

// Every GitLab component job that names its runner takes the tags from an
// array input of its own, which defaults to untagged; a tag written as a
// variable, as a signer job's would be to follow a project's choice, is
// refused.
func TestGitLabJobsTakeRunnerTagsOnlyFromInputs(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(reporoot.Path(t), "templates")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		path := "templates/" + entry.Name()
		if path == "templates/publish-apple-appstore.yml" {
			// A macOS runner: its tags default to the hosted one's.
			continue
		}

		for _, problem := range runnerTagsProblems(path, string(reporoot.ReadFile(t, path))) {
			t.Error(problem)
		}
	}

	const head = "spec:\n  inputs:\n    signer-runner-tags:\n      type: array\n      default: []\n---\n"

	for _, tags := range []string{"[$SIGNER_RUNNER_TAG]", "$SIGNER_RUNNER_TAGS", "[\"$[[ inputs.signer-runner-tags ]]\", $EXTRA]", "$[[ inputs.signer-runner-tags ]]-$SUFFIX"} {
		component := head + "sign:\n  tags: " + tags + "\n  script: [sign]\n"
		if problems := runnerTagsProblems("signer.yml", component); len(problems) != 1 {
			t.Errorf("tags %s: want one refusal, got %v", tags, problems)
		}
	}

	if problems := runnerTagsProblems("signer.yml", head+"sign:\n  tags: $[[ inputs.signer-runner-tags ]]\n  script: [sign]\n"); len(problems) != 0 {
		t.Errorf("the signer-runner-tags input is refused: %v", problems)
	}
}

// Each single-job component runs its job untagged by default and on exactly
// the runner tags given.
func TestGitLabComponentJobsRunOnTheirRunnerTags(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"templates/check.yml", "templates/nanolinter.yml", "templates/megalinter.yml",
		"templates/build-cargo.yml", "templates/build-go.yml", "templates/build-gradle.yml", "templates/build-gradle-android.yml",
		"templates/build-maven.yml", "templates/build-npm.yml", "templates/publish-forge-packages.yml", "templates/publish-maven-central.yml",
	} {
		for _, want := range [][]any{{}, {"linux", "ci-ordinary"}} {
			given := map[string]any{}
			if len(want) != 0 {
				given["runner-tags"] = want
			}

			jobs := gitlabrender.Jobs(t, path, given)
			if len(jobs) != 1 {
				t.Fatalf("%s renders %d jobs, want one", path, len(jobs))
			}

			for name, job := range jobs {
				if !reflect.DeepEqual(job["tags"], want) {
					t.Errorf("%s: job %s runs on %v, want %v", path, name, job["tags"], want)
				}
			}
		}
	}
}

// The release's signer jobs, those in signer-image that hold the signing and
// publication credentials, run on the policy's signer-runner-tags and no
// other job does; every other job keeps its own role's tags. By default
// every job is untagged, as before the inputs existed.
func TestGitLabReleaseSignerJobsRunOnlyOnSignerRunnerTags(t *testing.T) {
	t.Parallel()

	signerImage, _ := gitlabrender.FixtureValue("signer-image")

	for _, custom := range []bool{false, true} {
		given := map[string]any{}
		if custom {
			given = map[string]any{
				"runner-tags": []any{"ordinary"}, "signer-runner-tags": []any{"signer", "release"}, "checks-runner-tags": []any{"checks"},
				"build-runner-tags": []any{"privileged"}, "build-runner-tags-arm64": []any{"arm64"},
			}
		}

		signers := 0

		for name, job := range gitlabrender.Jobs(t, "templates/release-jobs.yml", given) {
			tags := job["tags"]

			switch {
			case !custom:
				if !reflect.DeepEqual(tags, []any{}) {
					t.Errorf("job %s runs on %v by default, want untagged", name, tags)
				}
			case job["image"] == signerImage:
				signers++

				if !reflect.DeepEqual(tags, given["signer-runner-tags"]) {
					t.Errorf("signer job %s runs on %v, want signer-runner-tags", name, tags)
				}
			case reflect.DeepEqual(tags, given["signer-runner-tags"]):
				t.Errorf("job %s, outside signer-image, runs on signer-runner-tags", name)
			case !slices.ContainsFunc([]string{"runner-tags", "checks-runner-tags", "build-runner-tags", "build-runner-tags-arm64"},
				func(input string) bool { return reflect.DeepEqual(tags, given[input]) }):
				t.Errorf("job %s runs on %v, which no runner input names", name, tags)
			}
		}

		if custom && signers < 5 {
			t.Errorf("found %d signer jobs; the access checks, signing, promotion and publication run in signer-image", signers)
		}
	}
}
