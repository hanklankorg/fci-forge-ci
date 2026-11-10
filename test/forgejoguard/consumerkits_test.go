// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
//
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package forgejoguard

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
	"github.com/goccy/go-yaml/ast"
)

// consumerKit is one ecosystem example's Forgejo pair: the directory holding
// its pullrequest.yml and release.yml, and the build action both use. An
// ecosystem the layer has no build action for names, in its place, the
// command each build job runs: the build tool itself for the pull request,
// the engine's build for the release.
type consumerKit struct {
	dir          string
	buildAction  string
	prBuild      string
	releaseBuild string
}

var errKit = errors.New("a consumer kit names its build action or both build commands, not both or neither")

// validate refuses a kit whose build would match any step: one that names
// neither a build action nor both build commands, or names both kinds.
func (k consumerKit) validate() error {
	commands := k.prBuild != "" && k.releaseBuild != ""
	noCommands := k.prBuild == "" && k.releaseBuild == ""

	if (k.buildAction != "" && noCommands) || (k.buildAction == "" && commands) {
		return nil
	}

	return fmt.Errorf("%w: %s has action %q and commands %q, %q", errKit, k.dir, k.buildAction, k.prBuild, k.releaseBuild)
}

// runsOnly reports whether a step's run script is command alone, after the
// shell's strict mode at most: a script that also does something else, or
// masks the command's failure, is not the build.
func runsOnly(run, command string) bool {
	var lines []string

	for line := range strings.Lines(run) {
		if line = strings.TrimSpace(line); line != "" && line != "set -euo pipefail" {
			lines = append(lines, line)
		}
	}

	return command != "" && slices.Equal(lines, []string{command})
}

// runScript is a step's run script as the shell gets it: a literal block's
// text without its block indicator.
func runScript(step ast.Node) string {
	if literal, ok := mappingValue(step, "run").(*ast.LiteralNode); ok {
		return literal.Value.Value
	}

	return scalarString(mappingValue(step, "run"))
}

func (k consumerKit) release() string     { return k.dir + "/release.yml" }
func (k consumerKit) pullRequest() string { return k.dir + "/pullrequest.yml" }

// consumerKits are every consumer pair under examples/, each held to the
// same rules. TestConsumerKitsAreEveryForgejoExample keeps the list whole.
func consumerKits() []consumerKit {
	return []consumerKit{
		{dir: "examples/go-cli/forgejo", buildAction: "build-go"},
		{dir: "examples/go-service/forgejo", buildAction: "build-go"},
		{dir: "examples/cargo-app/forgejo", buildAction: "build-cargo"},
		{dir: "examples/maven-app/forgejo", buildAction: "build-maven"},
		{dir: "examples/gradle-app/forgejo", prBuild: "./gradlew --no-daemon build", releaseBuild: "forge-ci build gradle run"},
	}
}

// forgejoExampleTrees are the example directories a layer-wide scan reads:
// the organization's signer-image example and every consumer pair.
func forgejoExampleTrees() []string {
	kits := consumerKits()

	trees := make([]string, 0, 1+len(kits))
	trees = append(trees, "examples/forgejo")

	for _, kit := range kits {
		trees = append(trees, kit.dir)
	}

	return trees
}

// Every examples/**/forgejo/ directory with a release caller is a kit above,
// so a new ecosystem's pair cannot escape the consumer rules by being left
// off the list.
func TestConsumerKitsAreEveryForgejoExample(t *testing.T) {
	t.Parallel()

	root := reporoot.Path(t)

	var found []string

	err := filepath.WalkDir(filepath.Join(root, "examples"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "release.yml" || filepath.Base(filepath.Dir(path)) != "forgejo" {
			return err
		}

		relative, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}

		found = append(found, filepath.ToSlash(relative))

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	listed := make([]string, 0, len(consumerKits()))
	for _, kit := range consumerKits() {
		listed = append(listed, kit.dir)
	}

	slices.Sort(found)
	slices.Sort(listed)

	if !slices.Equal(found, listed) {
		t.Errorf("the Forgejo consumer pairs under examples/ are %v, but consumerKits lists %v", found, listed)
	}
}

// Every listed kit names one build, and a kit naming none, both kinds or
// half of the commands is refused, since its empty field would match any
// step.
func TestConsumerKitsNameOneBuild(t *testing.T) {
	t.Parallel()

	for _, kit := range consumerKits() {
		if err := kit.validate(); err != nil {
			t.Error(err)
		}
	}

	for name, kit := range map[string]consumerKit{
		"neither":          {dir: "neither"},
		"action and run":   {dir: "both", buildAction: "build-maven", prBuild: "mvn verify", releaseBuild: "forge-ci build maven run"},
		"pr command only":  {dir: "pr", prBuild: "./gradlew build"},
		"release only":     {dir: "release", releaseBuild: "forge-ci build gradle run"},
		"action, one half": {dir: "half", buildAction: "build-go", releaseBuild: "forge-ci build go run"},
	} {
		if err := kit.validate(); !errors.Is(err, errKit) {
			t.Errorf("%s: validate = %v, want the kit refused", name, err)
		}
	}
}

// A build step is the command alone: a script that masks its failure,
// adds another command or runs nothing is not the build.
func TestRunsOnlyTakesTheCommandAlone(t *testing.T) {
	t.Parallel()

	const command = "forge-ci build gradle run"

	for run, want := range map[string]bool{
		command:                                true,
		"set -euo pipefail\n" + command + "\n": true,
		command + " || true":                   false,
		command + "\necho done":                false,
		"echo first\n" + command:               false,
		"":                                     false,
		"set -euo pipefail\n":                  false,
	} {
		if got := runsOnly(run, command); got != want {
			t.Errorf("runsOnly(%q) = %t, want %t", run, got, want)
		}
	}

	if runsOnly("", "") {
		t.Error("runsOnly accepts an empty command")
	}
}
