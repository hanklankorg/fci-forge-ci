// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/diggsweden/forge-ci/internal/adapters/cosign"
	appconfig "github.com/diggsweden/forge-ci/internal/app/config"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/output"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/release"
	"github.com/diggsweden/forge-ci/test/live/internal/livetest"
)

// The prepared commit was verified by requireReleaseGit. Re-plan its immutable
// declaration through the engine owner, never the host checkout or ledger.
func requireLocalReleaseImageOutputs(t *testing.T, target livetest.Target, repo string, runID int64, prepared string, keys releaseKeys, settings livetest.TrialSettings) {
	t.Helper()

	mirror := livetest.MirrorRepo(t, target, repo)

	mode, err := mirror.Git(t, "ls-tree", prepared, "--", settings.ReleaseImagesConfig)
	if err != nil || !strings.HasPrefix(mode, "100644 blob ") {
		t.Fatalf("release image declaration must be a regular blob in the verified prepared source: %v", err)
	}

	declaration, err := mirror.Git(t, "show", prepared+":"+settings.ReleaseImagesConfig)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "artifacts.yml")
	if err := os.WriteFile(file, []byte(declaration), 0o600); err != nil {
		t.Fatal(err)
	}

	host, err := livetest.RegistryHost(target)
	if err != nil {
		t.Fatal(err)
	}

	repository := strings.ToLower(host + "/" + livetest.RepoSlug(target, repo))

	platforms, err := livetest.NativeReleasePlatforms(settings.Variables, livetest.RunnerArch(t, target))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := appconfig.ResolveReleaseImagesPlan(appconfig.EmitReleaseImagesPlanInput{
		Config: appconfig.EmitConfigPlanInput{Path: file},
		Images: pipeline.ReleaseImagesInput{ImageRepository: repository, Repository: livetest.RepoSlug(target, repo), ReleaseTag: "v" + settings.Version, ReleasePlatforms: platforms["RELEASE_PLATFORMS"]},
	}, output.Annotator{})
	if err != nil {
		t.Fatal("release image expected plan: ", err)
	}

	// Select the engine's standard collected output from this exact run. A
	// custom artifact namespace or resolved/base plan needs its own authority
	// seam; missing receipts must not be interpreted as an image-free release.
	var ledger []byte

	for _, artifact := range livetest.RunArtifacts(t, target, repo, runID) {
		if artifact.Name != "release-images" {
			continue
		}

		if ledger != nil || artifact.Expired {
			t.Fatal("release image receipt is duplicate or expired")
		}

		tree, err := livetest.ZIPTree(livetest.ArtifactZIP(t, target, repo, artifact.ID), maxReleaseFile)
		if err != nil {
			t.Fatal(err)
		}

		ledger = tree["release-images.json"]
		if len(ledger) == 0 {
			t.Fatal("collected release image artifact has no ledger")
		}
	}

	if len(ledger) == 0 {
		t.Fatal("missing planned release-images run artifact")
	}

	verifier := &cosign.Adapter{
		Bin: livetest.HostTool(t, "cosign"), AuthFile: livetest.RegistryAuthFile(t, target, t.TempDir()),
		Env: livetest.ToolEnvironment(t, target, nil), Transparency: release.TransparencyNone,
	}

	ctx, cancel := context.WithTimeout(livetest.Context(t), 2*time.Minute)
	defer cancel()

	resolve := func(ctx context.Context, ref string) (string, error) {
		tag, err := name.NewTag(ref)
		if err != nil || tag.Context().Name() != repository {
			return "", fmt.Errorf("release image read is outside the expected repository: %w", errs.ErrValidation)
		}

		digest, found := livetest.ImageDigestContext(ctx, t, target, repo, tag.TagStr())
		if !found {
			return "", fmt.Errorf("planned final image is missing: %w", errs.ErrValidation)
		}

		return digest, nil
	}

	if err := livetest.VerifyReleaseImageOutputs(ctx, livetest.ReleaseImageOutputs{
		Plan: plan, Ledger: ledger, Repository: repository, Tag: "v" + settings.Version, Commit: prepared,
		Source: strings.TrimRight(target.BaseURL(), "/") + "/" + livetest.RepoSlug(target, repo), Workflow: ".forgejo/workflows/" + settings.Workflow, PublicKey: keys.cosignPublicPath,
	}, resolve, verifier, os.Stdout); err != nil {
		t.Fatal("local release image readback: ", err)
	}
}
