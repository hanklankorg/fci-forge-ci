// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/livetest"
)

// runForgejoServiceDevelopment is the standalone service development trial:
// the lifecycle's development phase alone, dispatched on a consumer's policy
// main over a scratch source branch.
func runForgejoServiceDevelopment(t *testing.T) {
	t.Helper()

	root := os.Getenv(consumerCheckoutEnv)
	if root == "" {
		t.Fatal("set FORGE_CI_LIVE_CONSUMER to the clean selected source checkout")
	}

	if livetest.DemoName() != "" {
		t.Fatal("service development requires disposable scratch repositories")
	}

	for _, visibility := range livetest.ConsumerVisibilities(t) {
		t.Run(visibility.Name, func(t *testing.T) {
			runForgejoServiceCase(t, root, visibility.Private)
		})
	}
}

func runForgejoServiceCase(t *testing.T, root string, private bool) {
	t.Helper()

	if !livetest.Requires(t, provider.ForgeForgejo, livetest.NeedsInRunner, livetest.NeedsPackages) {
		livetest.NotCoveredf(t, "service development needs a native runner and packages")
	}

	target := livetest.Accept(t, provider.ForgeForgejo)
	if livetest.RunnerArch(t, target) != "amd64" {
		t.Fatal("service development requires native linux/amd64")
	}

	builder := livetest.ChooseImageBuilder(t, target)
	// This end-to-end trial also starts the published service with Podman.
	// Keep its existing container runner for both builders and readiness.
	buildLabel := livetest.PrivilegedRunnerLabel(t, target)
	if buildLabel == "" {
		livetest.NotCoveredf(t, "service readiness runs the published image with Podman and needs the contract's runner.privileged_label independently of the image builder")
	}

	t.Logf("development image builder: %s; container/readiness runner: %q", builder.Builder, buildLabel)

	declaration, err := livetest.ServiceSnapshotSource(root)
	if err != nil {
		t.Fatal(err)
	}

	config := livetest.ServiceDevelopmentConfig{
		RecipeDirectory: root, Declaration: declaration, JobImage: os.Getenv("FORGE_CI_LIVE_SERVICE_JOB_IMAGE"),
		Branch: os.Getenv("FORGE_CI_LIVE_SERVICE_BRANCH"), ReadinessURL: os.Getenv("FORGE_CI_LIVE_SERVICE_READINESS_URL"),
		Fixture: os.Getenv("FORGE_CI_LIVE_SERVICE_FIXTURE"),
	}
	source := livetest.OpenConsumer(t, target, root)
	admitted := source.Commit
	consumer := livetest.NewServiceDevelopmentFixture(t, source, config)
	layer, repo := newConsumerRepos(t, target, consumer, "consumer-development", private)
	jobImage := consumerBuilderJobImage(t, target, builder, layer)

	registry, err := livetest.RegistryHost(target)
	if err != nil {
		t.Fatal(err)
	}

	image := registry + "/" + strings.ToLower(livetest.RepoSlug(target, repo))

	plan, err := layer.PlanConsumer(consumer.Workflows(t), livetest.ConsumerFacts{Inputs: map[string]string{"forge-ci-repository": layer.Repository}})
	if err != nil {
		t.Fatal(err)
	}

	variables, err := livetest.MergeConsumerVariables(plan.Variables, map[string]string{
		"CI_IMAGE_BUILDER": builder.Builder, "CI_IMAGE_BUILDER_JOB_IMAGE": jobImage,
		layerRunnerVariable:     runnerLabelsJSON(t, livetest.RunnerLabel(t, target)),
		containerRunnerVariable: runnerLabelsJSON(t, buildLabel), "FORGE_CI_REPOSITORY": layer.Repository,
		"SERVICE_IMAGE_REPOSITORY": image, "DEVELOPMENT_REGISTRY_USERNAME": target.CredentialUsername,
	})
	if err != nil {
		t.Fatal(err)
	}

	maps.Copy(variables, livetest.ServiceSnapshotJobImageVariables(livetest.Context(t), t, consumer, config.JobImage))

	main := consumer.Push(t, repo, plan)
	head := consumer.PushChange(t, repo, config.Branch, main)
	requireObjectFormats(t, consumer.ObjectFormat, head, layer.Commit)
	scenario := layerScenario{target: target, repo: repo, layer: layer, label: buildLabel}

	defer func() {
		scenario.requireBranches(t, map[string]string{"main": main, config.Branch: head})
		scenario.requireNone(t, "/tags", "/releases")
	}()

	// Forgejo's variables API is unavailable while repository Actions are off.
	livetest.EnableActions(t, target, repo)

	for name, value := range variables {
		livetest.SetRepoVariable(t, target, repo, name, value)
	}

	livetest.SetRepoSecret(t, target, repo, "DEVELOPMENT_REGISTRY_TOKEN", target.Token)
	livetest.DispatchConsumerDevelopment(t, target, repo)
	run := livetest.WaitForConsumerRun(t, target, repo, developmentWorkflow, "workflow_dispatch", "")
	jobs := livetest.RunJobs(t, target, repo, run.ID)
	scenario.logJobs(t, jobs)

	requireRunIDs(t, target, repo, run.ID)

	receipt, producers := requireServiceDevelopment(t, target, repo, run, consumer, config, head, image)

	t.Logf("admitted=%s policy=%s selected=%s branch=%s engine=%s image=%s readiness=%s; producers=%d; declared build route preserved, no image byte-equality claim", admitted, main, head, config.Branch, layer.Commit, receipt.Ref, config.ReadinessURL, producers)
}

// developmentWorkflow is the workflow file a consumer's development run is
// dispatched from.
const developmentWorkflow = "development.yml"

// requireServiceDevelopment reads what a dispatched service development run
// left: the frozen snapshot plan bound to the selected source head and
// branch, one amd64 image receipt, the registry's digest and tag with no
// stable alias, and the readiness evidence binding the published digest to
// that source. It returns the image receipt and the count of the producers
// the plan built, none for a container-first service.
func requireServiceDevelopment(t *testing.T, target livetest.Target, repo string, run livetest.WorkflowRun, consumer livetest.Consumer, config livetest.ServiceDevelopmentConfig, head, image string) (snapshotimage.Receipt, int) {
	t.Helper()

	if run.Status != jobSuccess || (run.Event != "" && run.Event != "workflow_dispatch") || run.TriggerEvent != "workflow_dispatch" || run.WorkflowID != developmentWorkflow {
		t.Fatalf("service development run %d: %s; a failure of the consumer's own Containerfile or build is a CI integration failure, not permission to alter source", run.ID, run.Status)
	}

	artifacts := map[string]map[string][]byte{}

	for _, artifact := range livetest.RunArtifacts(t, target, repo, run.ID) {
		if artifact.Name != "service-evidence-"+head && artifact.Name != "snapshot-images-plan" {
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

	return receipt, len(expected.Artifacts)
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
