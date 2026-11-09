// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/livetest"
)

// developmentWorkflow is the workflow file a consumer's development run is
// dispatched from.
const developmentWorkflow = "development.yml"

// requireServiceDevelopment reads what a dispatched service development run
// left: the frozen snapshot plan bound to the selected source head and
// branch, the receipts of every planned amd64 image as the snapshot
// workflow's collect job exports them, the registry's digest and tag of each
// with no stable alias, and, where the plan has a service to probe
// (livetest.ServiceReadiness), the readiness evidence binding its published
// digest to that source. It returns the receipts, in plan order, and the
// count of the producers the plan built, none for a container-first
// service.
func requireServiceDevelopment(t *testing.T, target livetest.Target, repo string, run livetest.WorkflowRun, consumer livetest.Consumer, config livetest.ServiceDevelopmentConfig, head, image string) ([]snapshotimage.Receipt, int) {
	t.Helper()

	if run.Status != jobSuccess || (run.Event != "" && run.Event != "workflow_dispatch") || run.TriggerEvent != "workflow_dispatch" || run.WorkflowID != developmentWorkflow {
		t.Fatalf("service development run %d: %s; a failure of the consumer's own Containerfile or build is a CI integration failure, not permission to alter source", run.ID, run.Status)
	}

	artifacts := map[string]map[string][]byte{}

	for _, artifact := range livetest.RunArtifacts(t, target, repo, run.ID) {
		if !slices.Contains([]string{"service-evidence-" + head, snapshotArtifact, snapshotArtifact + "-plan"}, artifact.Name) {
			continue
		}

		if artifact.Expired || artifact.SizeInBytes <= 0 || artifact.SizeInBytes > 256<<20 {
			t.Fatalf("invalid service artifact: %+v", artifact)
		}

		address := fmt.Sprintf("%s/api/v1/repos/%s/actions/artifacts/%d/zip", target.BaseURL(), livetest.RepoSlug(target, repo), artifact.ID)

		tree, err := livetest.ZIPTree(livetest.Download(t, target, address, 256<<20), 256<<20)
		if err != nil {
			t.Fatal(err)
		}

		artifacts[artifact.Name] = tree
	}

	receipts, expected := requireSnapshotImages(livetest.Context(t), t, target, repo, consumer, config.Branch, head, image,
		artifacts[snapshotArtifact+"-plan"]["snapshot-plan.json"], artifacts[snapshotArtifact]["container-images.json"])

	evidence, probed := artifacts["service-evidence-"+head]
	if probed != livetest.ServiceReadiness(expected) {
		t.Fatalf("readiness evidence present %t, want %t for a plan of %d images", probed, !probed, len(expected.Images))
	}

	if probed && (string(evidence["image-ref.txt"]) != receipts[0].Ref+"\n" || string(evidence["source-sha.txt"]) != head+"\n" ||
		string(evidence["readiness-url.txt"]) != config.ReadinessURL+"\n" || len(evidence["readiness-body.txt"]) == 0) {
		t.Fatalf("readiness evidence does not bind selected source and published image: %+v", receipts[0])
	}

	return receipts, len(expected.Artifacts)
}

// snapshotArtifact is the snapshot-images workflow's artifact namespace, its
// artifact-name input's default: the collect job's receipts, and with -plan,
// the frozen plan.
const snapshotArtifact = "snapshot-images"

// requireServiceSnapshot is requireSnapshotImages of a service snapshot, the
// lifecycle's development phase: its plan has one image, whose receipt it
// returns with the expected plan.
func requireServiceSnapshot(ctx context.Context, t *testing.T, target livetest.Target, repo string, consumer livetest.Consumer, branch, head, image string, plan, receipts []byte) (snapshotimage.Receipt, snapshotimage.Plan) {
	t.Helper()

	published, expected := requireSnapshotImages(ctx, t, target, repo, consumer, branch, head, image, plan, receipts)
	if len(published) != 1 {
		t.Fatalf("a service snapshot published %d images, want one", len(published))
	}

	return published[0], expected
}

// requireSnapshotImages reads what a snapshot of a consumer left, from its
// frozen plan and its receipts: the plan the trial expects for the selected
// source head and branch, which the planner makes of the staged declaration,
// every build native amd64, and a receipt of each planned image, bound to
// them as the product's own collection requires (snapshotimage.Collect),
// each image the registry serves under its receipt's tag, with no alias but
// the tags of the planned images. It returns the receipts, in plan order,
// and the expected plan.
func requireSnapshotImages(ctx context.Context, t *testing.T, target livetest.Target, repo string, consumer livetest.Consumer, branch, head, image string, plan, receipts []byte) ([]snapshotimage.Receipt, snapshotimage.Plan) {
	t.Helper()

	var frozen snapshotimage.Plan
	if err := json.Unmarshal(plan, &frozen); err != nil {
		t.Fatal(err)
	}

	expected := livetest.ServiceSnapshotPlan(ctx, t, consumer, snapshotimage.Input{
		SourceSHA: head, SourceBranch: branch, Tag: frozen.Tag, ImageRepository: image,
		SourceDateEpoch: frozen.SourceDateEpoch, Publish: true,
	})
	if err := frozen.Validate(); err != nil || frozen.SourceSHA != head || frozen.SourceBranch != branch ||
		!reflect.DeepEqual(frozen, expected) || len(frozen.Builds) != len(frozen.Images) ||
		slices.ContainsFunc(frozen.Builds, func(build snapshotimage.Build) bool { return build.Arch != "amd64" }) {
		t.Fatalf("wrong frozen snapshot plan: %+v: %v", frozen, err)
	}

	var published []snapshotimage.Receipt
	if err := json.Unmarshal(receipts, &published); err != nil {
		t.Fatalf("missing snapshot receipts: %v", err)
	}

	published, err := snapshotimage.Collect(frozen, published)
	if err != nil {
		t.Fatalf("the receipts do not bind the selected source and every planned image: %v", err)
	}

	tags := map[string]bool{}

	for _, receipt := range published {
		if receipt.ImageRepository != image {
			t.Fatalf("the receipt of %s names %s, not the run's image repository %s", receipt.Name, receipt.ImageRepository, image)
		}

		imageConfig := livetest.ImageConfig(t, target, repo, receipt.Digest) //nolint:contextcheck // registry helper derives a bounded context from the test owner.
		if imageConfig.OS != "linux" || imageConfig.Architecture != "amd64" ||
			imageConfig.Config.Labels["org.opencontainers.image.revision"] != head ||
			imageConfig.Config.Labels["org.opencontainers.image.ref.name"] != branch {
			t.Fatalf("published image %s does not match the selected architecture, source and branch", receipt.Name)
		}

		if digest, found := livetest.ImageDigestContext(ctx, t, target, repo, receipt.Tag); !found || digest != receipt.Digest {
			t.Fatalf("snapshot tag %s does not resolve to receipt digest: %s", receipt.Tag, digest)
		}

		tags[receipt.Tag], tags[receipt.Tag+"-amd64"] = true, true
	}

	for tag := range livetest.RegistrySnapshot(t, target, repo) { //nolint:contextcheck // registry helper derives a bounded context from the test owner.
		if !tags[tag] {
			t.Errorf("unexpected registry alias %s", tag)
		}
	}

	return published, expected
}
