// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build linux

package forgejobehavior

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The signer's tag checks, one in each privileged job.
var signerTagChecks = []struct{ job, step string }{
	{"sign", "Verify release tag points to release SHA"},
	{"promote", "Verify release tag before image promotion"},
	{"publish", "Verify release tag before publish"},
}

// signerRelease is a consumer repository at a prepared release: the request
// commit with its projects at 0.1.9 and the release-request/v0.2.0 tag, and
// its single child, tagged v0.2.0, with the changelog and the files extra
// changes, pushed to a bare remote. It returns the sandbox, the request and
// the release commit.
func signerRelease(t *testing.T, extra map[string]string) (*sandbox, string, string) {
	t.Helper()

	box := versionBumpCheckout(t, true)
	box.write("helpers/.engine/bin/lib-forge-ci.sh", versionBumpLib, 0o600)

	steps := [][]string{{"tag", "--annotate", "--message", "request", "release-request/v0.2.0"}}
	for path, body := range extra {
		box.write(path, body, 0o600)
		steps = append(steps, []string{"add", "--", path})
	}

	box.write("CHANGELOG.md", "# Changelog\n\n## v0.2.0\n", 0o600)
	steps = append(steps,
		[]string{"add", "--", "CHANGELOG.md"},
		[]string{"commit", "--quiet", "--message", "chore(release): v0.2.0"},
		[]string{"tag", "--annotate", "--message", "release", "v0.2.0"},
		[]string{"init", "--quiet", "--bare", filepath.Join(box.root, "server", "origin.git")},
		[]string{"push", "--quiet", filepath.Join(box.root, "server", "origin.git"), "main", "--tags"},
	)

	for _, arguments := range steps {
		out, err := git(t, box.root, arguments...)
		require.NoError(t, err, out)
	}

	request, err := git(t, box.root, "rev-parse", "release-request/v0.2.0^{commit}")
	require.NoError(t, err, request)

	release, err := git(t, box.root, "rev-parse", "HEAD")
	require.NoError(t, err, release)

	return box, strings.TrimSpace(request), strings.TrimSpace(release)
}

// signerTagEnv is what a signer tag check runs with.
func signerTagEnv(t *testing.T, box *sandbox, request, release string) []string {
	t.Helper()

	return []string{
		"CONSUMER_WORKSPACE=" + box.root, "HELPERS_DIR=" + filepath.Join(box.root, "helpers"),
		"FORGE_CI_ENGINE=" + forgeCIEngine(t), "RUNNER_TEMP=" + t.TempDir(),
		"FORGEJO_SERVER=" + filepath.Join(box.root, "server"), "FORGEJO_REPO=origin",
		"RELEASE_TAG=v0.2.0", "RELEASE_SHA=" + release, "RELEASE_REQUEST_SHA=" + request, "ARTIFACTS_CONFIG=",
	}
}

// Every privileged signer job admits a release commit that changes the
// changelog and exactly the version bump its declaration plans, and refuses
// one that changes anything else beside them.
func TestSignerTagChecksAdmitExactlyThePlannedBump(t *testing.T) {
	t.Parallel()

	bumped := versionBumpFiles("0.2.0")
	planned := map[string]string{}

	for path, body := range bumped {
		if versionBumpProject(path) {
			planned[path] = body
		}
	}

	stray := map[string]string{"README.md": "# Example, changed\n"}
	for path, body := range planned {
		stray[path] = body
	}

	for _, check := range signerTagChecks {
		script := workflowStep(t, "sign-and-publish-release.yml", check.job, check.step).Run

		box, request, release := signerRelease(t, planned)
		got := box.run(script, signerTagEnv(t, box, request, release)...)
		require.Equal(t, 0, got.status, "%s: %s", check.job, got.stderr)

		box, request, release = signerRelease(t, stray)
		got = box.run(script, signerTagEnv(t, box, request, release)...)
		require.NotEqual(t, 0, got.status, "%s admitted a stray README.md change", check.job)
		require.Contains(t, got.stderr, "must change only CHANGELOG.md and the planned version bumps", check.job)
	}
}

// The signer's preflight hands on a repository-relative artifacts-config,
// emits nothing for the default, and refuses a path that leaves the
// checkout before any signer job starts.
func TestSignerPreflightAdmitsOnlyARelativeDeclaration(t *testing.T) {
	t.Parallel()

	script := workflowStep(t, "sign-and-publish-release.yml", "coordinates", "Validate trust coordinates").Run
	env := []string{
		"HELPERS_REPOSITORY_INPUT=owner/forge-ci", "SERVER_URL=https://forge.example", "TRANSPARENCY_LOG_INPUT=none",
		"SIGNER_IMAGE_INPUT=forge.example/owner/signer@sha256:" + strings.Repeat("a", 64),
	}

	for config, want := range map[string]string{"": "", ".forge-ci/release.yml": "artifacts_config=.forge-ci/release.yml\n"} {
		box := newSandbox(t, "coordinates")
		got := box.run(script, append(env, "ARTIFACTS_CONFIG_INPUT="+config)...)
		require.Equal(t, 0, got.status, got.stderr)
		require.True(t, strings.HasPrefix(box.output(), want+"helpers_repository="), "artifacts-config %q: %q", config, box.output())
	}

	for _, config := range []string{"/etc/artifacts.yml", "../artifacts.yml", "a/../../artifacts.yml", "-x"} {
		box := newSandbox(t, "coordinates")
		got := box.run(script, append(env, "ARTIFACTS_CONFIG_INPUT="+config)...)
		require.NotEqual(t, 0, got.status, "artifacts-config %q admitted", config)
		require.Contains(t, got.stderr, "artifacts-config must be a repository-relative path")
	}
}
