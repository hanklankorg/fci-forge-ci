// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// runnerChoice is how one forge layer chooses a role's runner: the inputs
// that name it, each "file:input", or, where none does, what decides instead.
type runnerChoice struct {
	inputs []string
	none   string
}

// runnerRole is one role the forge layers choose a runner for.
type runnerRole struct {
	name                    string
	gitlab, forgejo, github runnerChoice
}

// runnerRoles is docs/runners.md, Runner roles across the forges, as data.
var runnerRoles = []runnerRole{
	{
		name: "ordinary",
		gitlab: runnerChoice{inputs: []string{
			"templates/check.yml:runner-tags", "templates/nanolinter.yml:runner-tags", "templates/megalinter.yml:runner-tags",
			"templates/build-cargo.yml:runner-tags", "templates/build-go.yml:runner-tags", "templates/build-gradle.yml:runner-tags",
			"templates/build-gradle-android.yml:runner-tags", "templates/build-maven.yml:runner-tags", "templates/build-npm.yml:runner-tags",
			"templates/publish-forge-packages.yml:runner-tags", "templates/publish-maven-central.yml:runner-tags",
			"templates/release.yml:runner-tags", "templates/release-jobs.yml:runner-tags",
			"templates/snapshot-images.yml:runner-tags", "templates/snapshot-image-jobs.yml:runner-tags",
		}},
		forgejo: runnerChoice{inputs: []string{
			".forgejo/workflows/prepare-release.yml:runner-label", ".forgejo/workflows/package-retention.yml:runner-label",
			".forgejo/workflows/promote-base-container-images.yml:runner-label", ".forgejo/workflows/verify-base-container-images.yml:runner-label",
			".forgejo/workflows/runtime-images.yml:runner-label",
		}},
		github: runnerChoice{
			inputs: []string{".github/workflows/slsa-attestor.yml:runs-on"},
			none:   "every other job is the hosted ubuntu-26.04 literal",
		},
	},
	{
		name: "image-build",
		gitlab: runnerChoice{inputs: []string{
			"templates/release.yml:build-runner-tags-linux-amd64", "templates/release-jobs.yml:build-runner-tags-linux-amd64",
			"templates/snapshot-images.yml:build-runner-tags-linux-amd64",
		}},
		forgejo: runnerChoice{inputs: []string{
			".forgejo/workflows/release-images.yml:runner-label", ".forgejo/workflows/snapshot-images.yml:runner-label",
			".forgejo/workflows/runtime-images.yml:runner-label-container",
		}},
		github: runnerChoice{none: "the hosted ubuntu-26.04 literal, whose VMs permit user namespaces"},
	},
	{
		name: "image-build-arm64",
		gitlab: runnerChoice{inputs: []string{
			"templates/release.yml:build-runner-tags-linux-arm64", "templates/release-jobs.yml:build-runner-tags-linux-arm64",
			"templates/snapshot-images.yml:build-runner-tags-linux-arm64",
		}},
		forgejo: runnerChoice{inputs: []string{
			".forgejo/workflows/release-images.yml:runner-label-arm64", ".forgejo/workflows/snapshot-images.yml:runner-label-arm64",
			".forgejo/workflows/runtime-images.yml:runner-label-arm64", ".forgejo/workflows/runtime-images.yml:runner-label-container-arm64",
		}},
		github: runnerChoice{none: "the hosted ubuntu-26.04-arm literal"},
	},
	{
		name:   "signer",
		gitlab: runnerChoice{inputs: []string{"templates/release.yml:signer-runner-tags", "templates/release-jobs.yml:signer-runner-tags"}},
		forgejo: runnerChoice{inputs: []string{
			".forgejo/workflows/sign-and-publish-release.yml:runner-label", ".forgejo/workflows/sign-container-images.yml:runner-label",
		}},
		github: runnerChoice{none: "the hosted ubuntu-26.04 literal of the layer"},
	},
	{
		name:    "macos",
		gitlab:  runnerChoice{inputs: []string{"templates/publish-apple-appstore.yml:runner-tags-darwin-arm64"}},
		forgejo: runnerChoice{none: "the Forgejo layer has no Apple workflows"},
		github: runnerChoice{inputs: []string{
			".github/workflows/build-xcode-ios.yml:macos-version", ".github/workflows/lint-swift.yml:macos-version",
			".github/workflows/publish-apple-appstore.yml:macos-version",
		}},
	},
}

// runnerInputName is a runner input of any layer by its name: GitLab's
// *runner-tags*, Forgejo's runner-label*, GitHub's runs-on and macos-version.
var runnerInputName = regexp.MustCompile(`^((?:[a-z0-9]+-)*runner-tags(?:-[a-z0-9-]+)?|runner-label(?:-[a-z0-9-]+)?|runs-on|macos-version)$`)

// gitLabRunnerInputName is the one shape a GitLab runner input may take,
// [role-]runner-tags[-<os>-<arch>]: the role when it is not the ordinary
// one, and the platform when the role's runner is bound to one, as an image
// build's always is.
var gitLabRunnerInputName = regexp.MustCompile(`^(?:signer-)?runner-tags(?:-(?:linux|darwin)-(?:amd64|arm64))?$|^build-runner-tags-linux-(?:amd64|arm64)$`)

// Every runner input of a GitLab component follows the naming pattern of
// docs/runners.md, and none goes by an older name.
func TestGitLabRunnerInputsFollowTheNamingPattern(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.Join(reporoot.Path(t), "templates"))
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		path := "templates/" + entry.Name()
		for _, name := range declaredInputs(t, path) {
			if strings.Contains(name, "runner") && !gitLabRunnerInputName.MatchString(name) {
				t.Errorf("%s: runner input %s is not named [role-]runner-tags[-<os>-<arch>]", path, name)
			}
		}
	}

	for _, name := range []string{"build-runner-tags", "build-runner-tags-arm64", "checks-runner-tags", "runner-tags-macos", "privileged-runner-tags"} {
		if gitLabRunnerInputName.MatchString(name) {
			t.Errorf("the naming pattern admits %s", name)
		}
	}
}

// Every role has a runner choice on every forge, an input or the reason
// there is none; every runner input of any layer belongs to a role; and the
// shared table in docs/runners.md names each role's inputs on each forge. An
// option one forge gains then fails here until the others have it or the
// table says why not.
func TestRunnerRolesAreCoherentAcrossForges(t *testing.T) {
	t.Parallel()

	declared := runnerInputs(t)
	claimed := map[string]bool{}
	doc := string(reporoot.ReadFile(t, "docs/runners.md"))

	for _, role := range runnerRoles {
		row := ""

		for line := range strings.SplitSeq(doc, "\n") {
			if strings.HasPrefix(line, "| "+role.name+" |") {
				row = line
			}
		}

		if row == "" {
			t.Errorf("docs/runners.md has no row for the runner role %s", role.name)
		}

		for forge, choice := range map[string]runnerChoice{"gitlab": role.gitlab, "forgejo": role.forgejo, "github": role.github} {
			if len(choice.inputs) == 0 && choice.none == "" {
				t.Errorf("role %s: %s names neither an input nor why it has none", role.name, forge)
			}

			for _, input := range choice.inputs {
				claimed[input] = true

				if !declared[input] {
					t.Errorf("role %s: %s declares no input %s", role.name, forge, input)
				}

				_, name, _ := strings.Cut(input, ":")
				if !strings.Contains(row, "`"+name+"`") {
					t.Errorf("docs/runners.md, role %s, does not name %s's %s", role.name, forge, name)
				}
			}
		}
	}

	for input := range declared {
		if !claimed[input] {
			t.Errorf("runner input %s belongs to no runner role; add it to runnerRoles and docs/runners.md", input)
		}
	}
}

// runnerInputs is every runner input the three layers declare, "file:input".
func runnerInputs(t *testing.T) map[string]bool {
	t.Helper()

	root := reporoot.Path(t)
	inputs := map[string]bool{}

	for _, dir := range []string{"templates", ".forgejo/workflows", ".github/workflows"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range entries {
			if filepath.Ext(entry.Name()) != ".yml" {
				continue
			}

			path := dir + "/" + entry.Name()
			for _, name := range declaredInputs(t, path) {
				if runnerInputName.MatchString(name) {
					inputs[path+":"+name] = true
				}
			}
		}
	}

	return inputs
}

// declaredInputs is a GitLab component's spec inputs or a workflow's
// workflow_call inputs.
func declaredInputs(t *testing.T, path string) []string {
	t.Helper()

	text := reporoot.ReadFile(t, path)
	if strings.HasPrefix(path, "templates/") {
		header, _, _ := strings.Cut(string(text), "\n---\n")
		text = []byte(header)
	}

	var file map[string]any
	if err := yaml.Unmarshal(text, &file); err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	field := func(value any, key string) any {
		mapping, _ := value.(map[string]any)

		return mapping[key]
	}

	var names []string

	for _, inputs := range []any{field(field(file, "spec"), "inputs"), field(field(field(file, "on"), "workflow_call"), "inputs")} {
		mapping, _ := inputs.(map[string]any)
		for name := range mapping {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	return names
}
