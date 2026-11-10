// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// writeFile writes body to path, making its directory.
func writeFile(t *testing.T, path, body string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// A campaign prepares the runtime images the GitLab cases of its lanes run
// in: each case's own, the scenarios' go image, the further job images of
// the demos' gates and the producer families of the snapshots. A forge names
// every lane of it. A name that is no forge or lane, a target images.json
// does not build and a nanolinter image of a flavor the campaign does not
// publish are refused.
func TestCampaignRuntimeTargets_ComeFromTheLanesCasesAndTheirDemosGates(t *testing.T) {
	t.Parallel()

	repo := reporoot.Path(t)
	flavors := []string{"go", "java", "javascript-typescript", "rust"}

	for _, tc := range []struct{ names, flavors, want []string }{
		{[]string{"k3s-private-gitlab"}, flavors, []string{"go", "java", "node", "rust"}},
		{[]string{"lifecycle-gitlab"}, flavors, []string{"java", "node", "rust"}},
		// A snapshot lints in nothing, and the monorepo's producers build in
		// their families' images.
		{[]string{"snapshot-gitlab"}, nil, []string{"go", "java", "node", "rust"}},
		// The scenarios' go image alone.
		{[]string{"build-capability-gitlab"}, nil, []string{"go"}},
		// A Forgejo lane runs no GitLab gate and no GitLab scenario.
		{[]string{"k3s-private-forgejo", "forgejo"}, flavors, nil},
		{[]string{"k3s-private-forgejo", "lifecycle-gitlab"}, flavors, []string{"java", "node", "rust"}},
		{[]string{"gitlab"}, append(slices.Clone(flavors), "kotlin"), []string{"android", "go", "java", "node", "rust"}},
	} {
		if targets, err := CampaignRuntimeTargets(repo, tc.names, tc.flavors); err != nil || !slices.Equal(targets, tc.want) {
			t.Errorf("%q: targets %q, %v; want %q", tc.names, targets, err, tc.want)
		}
	}

	// A demo whose gate checks in a further job image, in a repository whose
	// roster builds it, and in one whose roster does not.
	gate := "include:\n  - component: $CI_SERVER_FQDN/team/engine/check@" + strings.Repeat("a", 40) + "\n    inputs:\n      job-image: $CI_JOB_IMAGE_NODE\n"
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "containers", "runtime", "images.json"), `[{"target":"android"},{"target":"node"}]`)
	writeFile(t, filepath.Join(demoConsumerDir(root, "hello-android"), filepath.FromSlash(GitLabGateFile)), gate)

	if targets, err := CampaignRuntimeTargets(root, []string{"k3s-private-android-gitlab"}, []string{"kotlin"}); err != nil || !slices.Equal(targets, []string{"android", "node"}) {
		t.Errorf("a gate's further job image: targets %q, %v", targets, err)
	}

	unbuilt := t.TempDir()
	writeFile(t, filepath.Join(unbuilt, "containers", "runtime", "images.json"), `[{"target":"android"}]`)
	writeFile(t, filepath.Join(demoConsumerDir(unbuilt, "hello-android"), filepath.FromSlash(GitLabGateFile)), gate)

	for _, tc := range []struct {
		root           string
		names, flavors []string
		want           string
	}{
		{unbuilt, []string{"k3s-private-android-gitlab"}, []string{"kotlin"}, `"node"`},
		{repo, []string{"lifecycle-gitlab"}, []string{"go"}, "add job-image=java:<local image>"},
		{repo, []string{"k3s-private-forgejo"}, nil, "add job-image=go:<local image>"},
		{repo, []string{"lifecycle-gitlab"}, []string{"Java"}, "a job image flavor"},
		{repo, nil, flavors, "name what the campaign prepares for"},
		{repo, []string{"github"}, flavors, "want forgejo, gitlab or a lane"},
		{repo, []string{"gitlab=lifecycle-gitlab"}, flavors, "want forgejo, gitlab or a lane"},
	} {
		if _, err := CampaignRuntimeTargets(tc.root, tc.names, tc.flavors); !errors.Is(err, errs.ErrValidation) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q with %q: got %v, want a validation error naming %q", tc.names, tc.flavors, err, tc.want)
		}
	}
}

func TestAndroidCampaignRuntimeIsExplicitlyOptedInAndRequiresItsLintFlavor(t *testing.T) {
	t.Parallel()

	root := reporoot.Path(t)
	base, android := "k3s-private-gitlab", "k3s-private-android-gitlab"
	flavors := []string{"go", "java", "javascript-typescript", "rust"}
	ordinary, err := CampaignRuntimeTargets(root, []string{base}, flavors)

	if err != nil || slices.Contains(ordinary, "android") {
		t.Fatalf("ordinary campaign unexpectedly prepares Android: %q, %v", ordinary, err)
	}

	if _, err := CampaignRuntimeTargets(root, []string{base, android}, flavors); err == nil || !strings.Contains(err.Error(), "job-image=kotlin:") {
		t.Fatalf("Android lint image must be explicit before any image build: %v", err)
	}

	targets, err := CampaignRuntimeTargets(root, []string{base, android}, append(flavors, "kotlin"))
	if want := []string{"android", "go", "java", "node", "rust"}; err != nil || !slices.Equal(targets, want) {
		t.Fatalf("opt-in runtime targets: got %q, %v; want %q", targets, err, want)
	}
}
