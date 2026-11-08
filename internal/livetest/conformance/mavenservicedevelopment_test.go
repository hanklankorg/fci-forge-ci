// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/livetest"
)

// developmentWorkflow is the workflow file a consumer's development run is
// dispatched from.
const developmentWorkflow = "development.yml"

// requireMavenServiceDevelopment reads what a dispatched service development
// run of the Maven service fixture left: the frozen snapshot plan bound to the
// selected source head and branch, one amd64 image receipt with its readiness
// evidence, the registry's digest and tag with no stable alias, and the
// checked JARs. It returns the image receipt, the count of checked JARs and
// of the declared producers.
func requireMavenServiceDevelopment(t *testing.T, target livetest.Target, repo string, run livetest.WorkflowRun, consumer livetest.Consumer, config livetest.MavenServiceDevelopmentConfig, head, image string) (snapshotimage.Receipt, int, int) {
	t.Helper()

	if run.Status != jobSuccess || (run.Event != "" && run.Event != "workflow_dispatch") || run.TriggerEvent != "workflow_dispatch" || run.WorkflowID != developmentWorkflow {
		t.Fatalf("service development run %d: %s; original Containerfile/POM failures are CI integration failures, not permission to alter source", run.ID, run.Status)
	}

	artifacts := map[string]map[string][]byte{}

	for _, artifact := range livetest.RunArtifacts(t, target, repo, run.ID) {
		if artifact.Name != "service-jars-"+head && artifact.Name != "service-evidence-"+head && artifact.Name != "snapshot-images-plan" {
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

	evidence := artifacts["service-evidence-"+head]
	receipt, expected := requireServiceSnapshot(livetest.Context(t), t, target, repo, consumer, config.Branch, head, image, artifacts["snapshot-images-plan"]["snapshot-plan.json"], evidence["container-images.json"])

	if string(evidence["image-ref.txt"]) != receipt.Ref+"\n" || string(evidence["source-sha.txt"]) != head+"\n" || string(evidence["readiness-url.txt"]) != config.ReadinessURL+"\n" || len(evidence["readiness-body.txt"]) == 0 {
		t.Fatalf("readiness evidence does not bind selected source and published image: %+v", receipt)
	}

	jars := artifacts["service-jars-"+head]
	if string(jars["source-sha.txt"]) != head+"\n" {
		t.Fatal("checked JAR source differs from selected branch")
	}

	count := 0

	for name, body := range jars {
		if !strings.HasSuffix(name, ".jar") {
			continue
		}

		jar, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil || len(jar.File) == 0 {
			t.Fatalf("not an actual nonempty JAR: %s", name)
		}

		entry := fmt.Sprintf("%x  checked-jars/%s\n", sha256.Sum256(body), name)
		if !strings.Contains(string(jars["SHA256SUMS"]), entry) {
			t.Fatalf("checked JAR checksum differs: %s", name)
		}

		count++
	}

	if count == 0 {
		t.Fatal("no actual checked source JAR")
	}

	return receipt, count, len(expected.Artifacts)
}

// requireServiceSnapshot reads what a service snapshot of a consumer left,
// from its frozen plan and its receipts: the plan the trial
// expects for the selected source head and branch, one amd64 image receipt
// bound to them, the registry serving that digest under the receipt's tag
// with no stable alias. It returns the receipt and the expected plan.
func requireServiceSnapshot(ctx context.Context, t *testing.T, target livetest.Target, repo string, consumer livetest.Consumer, branch, head, image string, plan, receipts []byte) (snapshotimage.Receipt, snapshotimage.Plan) {
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
		!reflect.DeepEqual(frozen, expected) || len(frozen.Builds) != 1 || frozen.Builds[0].Arch != "amd64" {
		t.Fatalf("wrong frozen service plan: %+v: %v", frozen, err)
	}

	var published []snapshotimage.Receipt
	if err := json.Unmarshal(receipts, &published); err != nil || len(published) != 1 {
		t.Fatalf("missing service receipt: %v", err)
	}

	receipt := published[0]
	if receipt.SourceSHA != head || receipt.SourceBranch != branch || receipt.ImageRepository != image || receipt.Ref != image+"@"+receipt.Digest || receipt.Tag != frozen.Tag {
		t.Fatalf("the receipt does not bind the selected source and the published image: %+v", receipt)
	}

	imageConfig := livetest.ImageConfig(t, target, repo, receipt.Digest) //nolint:contextcheck // registry helper derives a bounded context from the test owner.
	if imageConfig.OS != "linux" || imageConfig.Architecture != "amd64" ||
		imageConfig.Config.Labels["org.opencontainers.image.revision"] != head ||
		imageConfig.Config.Labels["org.opencontainers.image.ref.name"] != branch {
		t.Fatal("published platform image does not match the selected architecture, source and branch")
	}

	if digest, found := livetest.ImageDigestContext(ctx, t, target, repo, receipt.Tag); !found || digest != receipt.Digest {
		t.Fatalf("snapshot tag does not resolve to receipt digest: %s", digest)
	}

	for tag := range livetest.RegistrySnapshot(t, target, repo) { //nolint:contextcheck // registry helper derives a bounded context from the test owner.
		if tag != receipt.Tag && tag != receipt.Tag+"-amd64" {
			t.Errorf("unexpected registry alias %s", tag)
		}
	}

	return receipt, expected
}
