// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package gitlabbehavior

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diggsweden/forge-ci/internal/testutil/gitlabrender"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

func TestReleaseArtifactsAreReadableWithoutOpeningPrivateFiles(t *testing.T) {
	t.Parallel()

	// Each producer job and the hand-off directory it leaves for the next.
	for job, dir := range map[string]string{
		"Prepare: changelog":     "_release-render",
		"Prepare: version java":  "_release-bump-java",
		"Prepare: version node":  "_release-bump-node",
		"Prepare: version rust":  "_release-bump-rust",
		"Build: java":            "_release-build-java",
		"Build: node":            "_release-build-node",
		"Build: go":              "_release-build-go",
		"Build: rust":            "_release-build-rust",
		"Build: android":         "_release-build-android",
		"Build: release files":   "_release-build",
		"Images: plan":           "_release-images-plan",
		"Images: build amd64":    "_release-images-amd64",
		"Images: build arm64":    "_release-images-arm64",
		"Images: publish":        "_release-images",
		"Sign: files and images": "_release-sign",
		"Publish: images":        "_release-promote",
	} {
		t.Run(job, func(t *testing.T) {
			t.Parallel()

			workspace := t.TempDir()
			handoff := filepath.Join(workspace, dir)
			private := filepath.Join(workspace, "private")
			incoming := filepath.Join(workspace, "incoming")

			for _, dir := range []string{handoff, private, incoming} {
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}

			for _, file := range []string{filepath.Join(handoff, "payload"), filepath.Join(private, "key"), filepath.Join(incoming, "payload")} {
				if err := os.WriteFile(file, []byte("fixture bytes"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := os.Symlink(private, filepath.Join(handoff, "private-link")); err != nil {
				t.Fatal(err)
			}

			runReleaseAfterScript(t, workspace, job)
			requireMode(t, handoff, 0o755)
			requireMode(t, filepath.Join(handoff, "payload"), 0o644)
			requireMode(t, private, 0o700)
			requireMode(t, filepath.Join(private, "key"), 0o600)
			requireMode(t, incoming, 0o700)
			requireMode(t, filepath.Join(incoming, "payload"), 0o600)

			if body, err := os.ReadFile(filepath.Join(handoff, "payload")); err != nil || string(body) != "fixture bytes" {
				t.Fatal("artifact publication changed its bytes")
			}
		})
	}
}

func TestReleaseArtifactsNeverFollowASymlinkedOutputRoot(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	private := filepath.Join(t.TempDir(), "private")

	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(private, filepath.Join(workspace, "_release-sign")); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(private, "key"), []byte("private fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	runReleaseAfterScript(t, workspace, "Sign: files and images")
	requireMode(t, private, 0o700)
	requireMode(t, filepath.Join(private, "key"), 0o600)
}

// runReleaseAfterScript runs a job's after_script as GitLab runs it, named
// another job by a project variable, which must not move its hand-off.
func runReleaseAfterScript(t *testing.T, workspace, job string) {
	t.Helper()

	body := releaseJobsBodyFor(t, "buildah")

	script := joinedScript(gitlabrender.ResolveExtends(t, body, body[job].(map[string]any))["after_script"])
	if script == "" {
		t.Fatal("release artifact publication hook is missing")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, componentBash(t), "--noprofile", "--norc", "-c", script)
	command.Dir = workspace
	command.Env = []string{"PATH=" + componentTools(t), "HOME=" + workspace, "CI_PROJECT_DIR=" + workspace,
		"CI_JOB_NAME=Publish: release", "rc_handoff=private", "CI_JOB_STATUS=failed"}

	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("artifact hook failed: %v\n%s", err, output)
	}
}

func requireMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != mode {
		t.Fatalf("%s mode=%#o, want %#o", path, info.Mode().Perm(), mode)
	}
}

// "Build: release files" gathers exactly the planned build jobs' hand-offs: a
// planned family that handed nothing on, or files from a family the plan
// does not build, stops the release before the declaring build runs.
func TestReleaseFilesGatherExactlyThePlannedFamilies(t *testing.T) {
	t.Parallel()

	script := filepath.Join(reporoot.Path(t), ".gitlab", "bin", "release.sh")

	for name, scenario := range map[string]struct {
		handoffs []string
		refusal  string
	}{
		"as planned":                  {handoffs: []string{"java"}},
		"a planned build is gone":     {refusal: `the plan builds the java family, but "Build: java" handed nothing on`},
		"an unplanned family's files": {handoffs: []string{"java", "node"}, refusal: `"Build: node" handed files on, but the plan builds no node family`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for _, family := range scenario.handoffs {
				if err := os.Mkdir(filepath.Join(dir, "_release-build-"+family), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			command := exec.CommandContext(ctx, componentBash(t), "--noprofile", "--norc", "-eu", "-c",
				`source "$1"; rc_engine() { printf 'engine %s\n' "$*"; }; rc_planned_jobs() { printf 'Prepare: version java\nBuild: java\n' >"$1"; }; rc_gather_families`,
				"gather", script)
			command.Env = []string{"PATH=/usr/bin:/bin", "CI_PROJECT_DIR=" + dir, "RC_WORK=" + t.TempDir()}

			out, err := command.CombinedOutput()
			if scenario.refusal == "" {
				if err != nil || !strings.Contains(string(out), "engine release gather-exports") {
					t.Fatalf("gather refused the planned hand-offs: %v\n%s", err, out)
				}

				return
			}

			if err == nil || !strings.Contains(string(out), scenario.refusal) || strings.Contains(string(out), "gather-exports") {
				t.Fatalf("gather went on: %v\n%s", err, out)
			}
		})
	}
}
