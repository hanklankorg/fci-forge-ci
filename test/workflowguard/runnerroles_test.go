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

				if !slices.Contains(known, name) {
					t.Errorf("%s: runner input %s is not owned by runners.Names(%s)", path, name, forge)
				}

				if !strings.Contains(doc, "`"+name+"`") {
					t.Errorf("docs/runners.md does not name %s's %s", path, name)
				}
			}
		}
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
