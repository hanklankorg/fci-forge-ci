// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package workflowguard

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/domain/runners"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// The migrated layers take their vocabulary from the domain owner, not a
// second role table or a regex that silently misses a newly added input.
func TestRunnerRolesAreCoherentAcrossForges(t *testing.T) {
	t.Parallel()

	doc := string(reporoot.ReadFile(t, "docs/runners.md"))
	declared := map[provider.ForgeAPI][]string{}

	for dir, forge := range map[string]provider.ForgeAPI{
		"templates": provider.ForgeGitLab, ".github/workflows": provider.ForgeGitHub,
	} {
		known, err := runners.Names(forge)
		if err != nil {
			t.Fatal(err)
		}

		for _, path := range workflowInputFiles(t, dir) {
			for _, name := range declaredInputs(t, path) {
				if !isRunnerInput(name) {
					continue
				}

				declared[forge] = append(declared[forge], name)

				if !slices.Contains(known, name) {
					t.Errorf("%s: runner input %s is not owned by runners.Names(%s)", path, name, forge)
				}

				if !strings.Contains(doc, "`"+name+"`") {
					t.Errorf("docs/runners.md does not name %s's %s", path, name)
				}

				// An image build's runner is always bound to a platform.
				if forge == provider.ForgeGitLab && name == "build-runner-tags" {
					t.Errorf("%s: %s binds image builds to no platform", path, name)
				}
			}
		}
	}

	for _, path := range workflowInputFiles(t, ".forgejo/workflows") {
		for _, name := range declaredInputs(t, path) {
			if isRunnerInput(name) {
				declared[provider.ForgeForgejo] = append(declared[provider.ForgeForgejo], name)
			}
		}
	}

	// Each role's row of the shared table names the role's input in each
	// forge's cell, and every declared runner input is some role's. Forgejo's
	// are its legacy names until it migrates.
	claimed := map[provider.ForgeAPI][]string{}

	for _, role := range []struct {
		row      string
		role     runners.Role
		platform string
		forgejo  []string
	}{
		{"ordinary", runners.Ordinary, "", []string{"runner-label"}},
		{"image-build", runners.Build, "linux/amd64", []string{"runner-label", "runner-label-container"}},
		{"image-build-arm64", runners.Build, "linux/arm64", []string{"runner-label-arm64", "runner-label-container-arm64"}},
		{"signer", runners.Signer, "", []string{"runner-label"}},
		{"macos", runners.Ordinary, "darwin/arm64", nil},
	} {
		var cells []string

		for line := range strings.SplitSeq(doc, "\n") {
			if strings.HasPrefix(line, "| "+role.row+" |") {
				cells = strings.Split(line, "|")
			}
		}

		if len(cells) != 6 {
			t.Errorf("docs/runners.md has no row for the runner role %s", role.row)

			continue
		}

		inputs := map[provider.ForgeAPI][]string{provider.ForgeForgejo: role.forgejo}

		// GitHub fixes the signer runner: no input, and the cell says so.
		for _, forge := range []provider.ForgeAPI{provider.ForgeGitLab, provider.ForgeGitHub} {
			if name, err := runners.Input(forge, role.role, role.platform); err == nil {
				inputs[forge] = []string{name}
			}
		}

		for column, forge := range []provider.ForgeAPI{provider.ForgeGitLab, provider.ForgeForgejo, provider.ForgeGitHub} {
			for _, name := range inputs[forge] {
				claimed[forge] = append(claimed[forge], name)

				if !slices.Contains(declared[forge], name) {
					t.Errorf("role %s: %s declares no input %s", role.row, forge, name)
				}

				if !strings.Contains(cells[column+2], "`"+name+"`") {
					t.Errorf("docs/runners.md, role %s, does not name %s's %s", role.row, forge, name)
				}
			}
		}
	}

	for forge, names := range declared {
		slices.Sort(names)

		for _, name := range slices.Compact(names) {
			if !slices.Contains(claimed[forge], name) {
				t.Errorf("%s runner input %s belongs to no role row of docs/runners.md", forge, name)
			}
		}
	}
}

// Each GitLab component declares exactly its roles' runner inputs: one that
// gains or loses an input fails here until its jobs and docs/runners.md agree.
func TestGitLabRunnerInputsPerTemplate(t *testing.T) {
	t.Parallel()

	ordinary := []string{"runner-tags"}
	release := []string{"build-runner-tags-linux-amd64", "build-runner-tags-linux-arm64", "runner-tags", "signer-runner-tags"}
	expected := map[string][]string{
		"build-cargo.yml":            ordinary,
		"build-go.yml":               ordinary,
		"build-gradle-android.yml":   ordinary,
		"build-gradle.yml":           ordinary,
		"build-maven.yml":            ordinary,
		"build-npm.yml":              ordinary,
		"check.yml":                  ordinary,
		"megalinter.yml":             ordinary,
		"nanolinter.yml":             ordinary,
		"publish-apple-appstore.yml": {"runner-tags-darwin-arm64"},
		"publish-forge-packages.yml": ordinary,
		"publish-maven-central.yml":  ordinary,
		"release-jobs.yml":           release,
		"release.yml":                release,
		"snapshot-image-jobs.yml":    ordinary,
		"snapshot-images.yml":        {"build-runner-tags-linux-amd64", "build-runner-tags-linux-arm64", "runner-tags"},
	}

	for _, path := range workflowInputFiles(t, "templates") {
		var got []string

		for _, name := range declaredInputs(t, path) {
			if isRunnerInput(name) {
				got = append(got, name)
			}
		}

		name := filepath.Base(path)
		if !slices.Equal(got, expected[name]) {
			t.Errorf("%s: runner inputs = %v, want %v", path, got, expected[name])
		}

		delete(expected, name)
	}

	if len(expected) != 0 {
		t.Errorf("GitLab components disappeared: %v", expected)
	}
}

// Freeze the not-yet-migrated Forgejo surface separately. Removing this gate
// requires migrating the callers, role-specific inputs and signer authority
// together; its old names must not become aliases in the domain owner.
func TestForgejoRunnerMigrationBoundary(t *testing.T) {
	t.Parallel()

	expected := map[string][]string{
		"prepare-release.yml":               {"runner-label"},
		"package-retention.yml":             {"runner-label"},
		"promote-base-container-images.yml": {"runner-label"},
		"verify-base-container-images.yml":  {"runner-label"},
		"runtime-images.yml":                {"runner-label", "runner-label-arm64", "runner-label-container", "runner-label-container-arm64"},
		"release-images.yml":                {"runner-label", "runner-label-arm64"},
		"snapshot-images.yml":               {"runner-label", "runner-label-arm64"},
		"sign-and-publish-release.yml":      {"runner-label"},
		"sign-container-images.yml":         {"runner-label"},
	}

	for _, path := range workflowInputFiles(t, ".forgejo/workflows") {
		var got []string

		for _, name := range declaredInputs(t, path) {
			if isRunnerInput(name) {
				got = append(got, name)
			}
		}

		name := filepath.Base(path)
		if !slices.Equal(got, expected[name]) {
			t.Errorf("%s: runner inputs = %v, migration boundary = %v", path, got, expected[name])
		}

		delete(expected, name)
	}

	if len(expected) != 0 {
		t.Errorf("Forgejo runner workflows disappeared: %v", expected)
	}
}

func isRunnerInput(name string) bool {
	return strings.Contains(name, "runner") || strings.Contains(name, "runs-on") || name == "macos-version"
}

func workflowInputFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(reporoot.Path(t), dir))
	if err != nil {
		t.Fatal(err)
	}

	var paths []string

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".yml" {
			paths = append(paths, dir+"/"+entry.Name())
		}
	}

	return paths
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
