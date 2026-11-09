// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package conformance_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
	"gopkg.in/yaml.v3"

	"github.com/diggsweden/forge-ci/internal/domain/provider"
	"github.com/diggsweden/forge-ci/internal/livetest"
)

// gitlabSnapshotPolicyFile is the one policy file of a snapshot's project.
const gitlabSnapshotPolicyFile = "snapshot.yml"

// gitlabRequestedMainRule admits an include for the pipeline the harness
// requests on main through the API alone, no push's: a snapshot is requested,
// not the effect of a change the harness makes.
const gitlabRequestedMainRule = `$CI_PIPELINE_SOURCE == "api" && $CI_COMMIT_REF_NAME == "main"`

// A service snapshot on GitLab: the lifecycle's development phase alone, in
// one GitLab project under one pinned policy that includes the
// snapshot-images component and nothing else, over every image the planner
// plans from the consumer's declaration (docs/live-campaigns.md, Snapshot).
// The consumer's main and a change on branch development are pushed before
// the policy is in place, and no rule of it admits a push; the harness then
// requests one pipeline on main, whose child snapshots development, and
// observes what it left: the frozen plan in the parent's plan job, every
// planned image's receipt in the child's collect job, the registry's tags
// (requireSnapshotImages), and a project with that pipeline, its two
// branches unmoved and nothing released. The component probes no readiness,
// so none is claimed. The development registry credential is a protected
// variable scoped to the publisher's environment, on a protected main.
func TestGitLabLayer_ServiceSnapshot(t *testing.T) {
	livetest.RegisterLiveScenario(t)

	root := os.Getenv(consumerCheckoutEnv)
	if root == "" {
		t.Skip("no consumer checkout is named; run `just test-live-consumer gitlab <checkout> snapshot job-archive=<path>`")
	}

	if livetest.DemoName() != "" {
		t.Fatal("a snapshot runs in disposable scratch projects, not in a kept demo")
	}

	for _, visibility := range livetest.ConsumerVisibilities(t) {
		t.Run(visibility.Name, func(t *testing.T) {
			runGitLabServiceSnapshot(t, root, visibility.Private)
		})
	}
}

func runGitLabServiceSnapshot(t *testing.T, root string, private bool) {
	t.Helper()

	if event := os.Getenv(consumerEventEnv); event != livetest.TrialSnapshot {
		t.Fatalf("the event is %s; this trial runs %s", event, livetest.TrialSnapshot)
	}

	if settings := strings.TrimSpace(os.Getenv(consumerSettingsEnv)); settings != "" {
		t.Fatalf("a GitLab %s takes job-archive= alone, not %q", livetest.TrialSnapshot, settings)
	}

	source, err := livetest.ServiceSnapshotSource(root)
	if err != nil {
		t.Fatal(err)
	}

	jobArchive := os.Getenv("FORGE_CI_LIVE_JOB_IMAGE_ARCHIVE")
	if jobArchive == "" {
		livetest.NotCoveredf(t, "a GitLab snapshot requires the reviewed job image archive of job-archive=")
	}

	if !livetest.Requires(t, provider.ForgeGitLab, livetest.NeedsInRunner) {
		livetest.NotCoveredf(t, "a snapshot needs a GitLab runner")
	}

	target := livetest.Accept(t, provider.ForgeGitLab)
	builder := livetest.ChooseImageBuilder(t, target)

	builderArchive := os.Getenv(livetest.KanikoJobImageArchiveEnv)
	if builder.Builder == livetest.ImageBuilderBuildah {
		if builderArchive = os.Getenv("FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE"); builderArchive == "" {
			livetest.NotCoveredf(t, "Buildah snapshot images require FORGE_CI_LIVE_BUILDAH_JOB_IMAGE_ARCHIVE, a saved Buildah job image")
		}
	}

	layer := livetest.PushLayer(t, target, gitlabReleaseRepo(t, target, "snapshot-engine", livetest.ReadLayerAnonymously))
	consumer := livetest.OpenConsumer(t, target, root).WithDraftLayer(t, layer).WithSnapshotImagesDeclaration(t, source)
	families := livetest.ServiceSnapshotFamilies(livetest.Context(t), t, consumer)

	jobImage := livetest.PushJobImage(t, target, gitlabReleaseRepo(t, target, "snapshot-job-image", livetest.ReadJobImageAnonymously), jobArchive)

	builderRepo := gitlabReleaseRepo(t, target, "snapshot-builder-job-image", livetest.ReadJobImageAnonymously)

	// A kaniko job image is labelled with the layer it runs from, which
	// admits it.
	var builderJobImage string
	if builder.Builder == livetest.ImageBuilderKaniko {
		builderJobImage = livetest.PushKanikoJobImage(t, target, builderRepo, builderArchive, layer)
	} else {
		builderJobImage = livetest.PushJobImage(t, target, builderRepo, builderArchive)
	}

	// Each producer builds in the runtime image of its toolchain family.
	producerImages := map[string]string{}

	for _, family := range families {
		archive := os.Getenv(livetest.GitLabSnapshotProducerArchiveEnv(family))
		if archive == "" {
			livetest.NotCoveredf(t, "the snapshot builds a %s producer; set %s to the %s runtime image archive", family, livetest.GitLabSnapshotProducerArchiveEnv(family), family)
		}

		producerImages[family] = livetest.PushJobImage(t, target, gitlabReleaseRepo(t, target, "snapshot-producer-image-"+family, livetest.ReadJobImageAnonymously), archive)
	}

	policyRepo := gitlabReleaseRepo(t, target, "snapshot-policy", "")
	policyProject := livetest.RepoSlug(target, policyRepo)
	repo := livetest.NewBareScratchRepo(t, target, livetest.ServiceSnapshotScenario, consumer.ObjectFormat, private)
	project := livetest.RepoSlug(target, repo)

	client, err := gitlabapi.NewClient(target.Token, gitlabapi.WithBaseURL(target.BaseURL()+"/api/v4/"),
		gitlabapi.WithHTTPClient(livetest.HTTPClient(t, target, time.Minute)))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(livetest.Context(t), 60*time.Minute)
	defer cancel()

	livetest.RetainGitLabJobLogsAfter(t, target, repo, map[string]string{"SNAPSHOT_REGISTRY_PASSWORD": target.Token})
	createGitLabProtectedVariables(t, client, project, []gitlabProjectVariable{
		{"SNAPSHOT_REGISTRY_USER", target.CredentialUsername, "snapshot/registry"},
		{"SNAPSHOT_REGISTRY_PASSWORD", target.Token, "snapshot/registry"},
	})

	registry, err := livetest.RegistryHost(target)
	if err != nil {
		t.Fatal(err)
	}

	image := registry + "/" + strings.ToLower(project)

	inputs := map[string]any{
		"branch": lifecycleBranch, "artifacts-config": livetest.ServiceSnapshotDeclarationFile,
		"image-repository": image, "forge-ci-repository": layer.Repository,
		"server-url": target.BaseURL(), "registry": registry,
		"job-image": jobImage, "build-image": builderJobImage,
		"image-builder": builder.Builder, "builder-job-image": builderJobImage,
		"runner-tags": []string{}, "build-runner-tags": builder.RunnerLabels,
	}

	for family, producerImage := range producerImages {
		inputs["artifact-job-image-"+family] = producerImage
	}

	policyBody, err := yaml.Marshal(map[string]any{"include": []any{map[string]any{
		"component": gitlabComponent(target, layer, "snapshot-images"),
		"inputs":    layer.ComponentAdminInputs(t, "snapshot-images", inputs),
		"rules":     []any{map[string]string{"if": gitlabRequestedMainRule}},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	policy := gitlabPolicyCommit(t, ctx, client, policyProject, gitlabSnapshotPolicyFile, string(policyBody), gitlabapi.FileCreate)

	livetest.RestrictGitLabJobToken(t, target, repo)

	t.Logf("consumer %s at %s (%s); layer %s at %s; job image %s, producer images %v of %s, image builder %s", root, consumer.Commit, consumer.ObjectFormat, layer.Repository, layer.Commit, jobImage, producerImages, source, builder.Builder)

	base := consumer.PushSnapshot(t, repo)
	requireObjectFormats(t, consumer.ObjectFormat, base, layer.Commit)

	head := consumer.PushChange(t, repo, lifecycleBranch, base)

	useGitLabAdminPolicy(ctx, t, client, project, gitlabSnapshotPolicyFile+"@"+policyProject+":"+policy.ID)
	waitGitLabPush(t, ctx, client, project, lifecycleBranch, head)
	protectGitLabMain(ctx, t, client, project)

	requested, _, err := client.Pipelines.CreatePipeline(project, &gitlabapi.CreatePipelineOptions{Ref: new(layerMain)}, gitlabapi.WithContext(ctx))
	if err != nil {
		t.Fatalf("request the snapshot pipeline on %s: %v", layerMain, err)
	}

	run := waitGitLabRequestedPipeline(t, ctx, client, project, requested.ID, base)
	child := requireGitLabChildPipeline(t, ctx, client, project, run)

	if run.Status != "success" {
		for _, id := range []int64{run.ID, child} {
			logGitLabTrace(t, gitlabTrialTrace(t, ctx, client, project, id))
		}

		t.Fatalf("snapshot pipeline %d ended %q, want success", run.ID, run.Status)
	}

	plan := gitlabJobArtifactFile(t, ctx, client, target, repo, run.ID, gitlabSnapshotPlanJob, "snapshot-plan.json")
	receipts := gitlabJobArtifactFile(t, ctx, client, target, repo, child, gitlabSnapshotCollectJob, "container-images.json")
	published, expected := requireSnapshotImages(ctx, t, target, repo, consumer, lifecycleBranch, head, image, plan, receipts)

	requireGitLabPipelineIDs(t, ctx, client, project, run.ID)
	requireGitLabNoReleases(t, ctx, client, project)
	requireBranches(t, target, repo, map[string]string{layerMain: base, lifecycleBranch: head})

	refs := make([]string, 0, len(published))
	for _, receipt := range published {
		refs = append(refs, receipt.Name+"="+receipt.Ref)
	}

	t.Logf("snapshot: pipeline %d requested on %s@%s, child %d, snapshots %s@%s as %s; producers %d", run.ID, layerMain, base, child, lifecycleBranch, head, strings.Join(refs, " "), len(expected.Artifacts))
}

// protectGitLabMain protects main, the ref the requested pipeline runs on,
// so the protected development registry credential reaches its publisher:
// GitLab may already have protected the default branch after the first push,
// which leaves it protected all the same.
func protectGitLabMain(ctx context.Context, t *testing.T, client *gitlabapi.Client, project string) {
	t.Helper()

	_, response, err := client.ProtectedBranches.ProtectRepositoryBranches(project, &gitlabapi.ProtectRepositoryBranchesOptions{
		Name: new(layerMain), AllowForcePush: new(false),
	}, gitlabapi.WithContext(ctx))
	if err != nil && (response == nil || response.StatusCode != http.StatusConflict) {
		t.Fatalf("protect %s: %v", layerMain, err)
	}
}

// waitGitLabRequestedPipeline waits for the pipeline the harness requested
// through the API to finish, bound to that request, the ref main and its
// exact commit, and logs GitLab's account of one that did not succeed.
func waitGitLabRequestedPipeline(t *testing.T, ctx context.Context, client *gitlabapi.Client, project string, id int64, sha string) *gitlabapi.Pipeline {
	t.Helper()

	started := time.Now()

	for {
		run, _, err := client.Pipelines.GetPipeline(project, id, gitlabapi.WithContext(ctx))
		if err != nil {
			t.Fatal(err)
		}

		if run.SHA != sha || run.Source != "api" || run.Ref != layerMain {
			t.Fatal("the pipeline is not bound to the harness's request, main and its exact commit")
		}

		switch run.Status {
		case "success", "failed", "canceled", "skipped", "manual":
			livetest.LogGitLabPipelineFailure(t, ctx, client, project, run)
			t.Logf("GitLab requested pipeline %d: %s after %s (%s)", run.ID, run.Status, time.Since(started).Round(time.Second), run.WebURL)

			return run
		}

		select {
		case <-ctx.Done():
			t.Fatal("the requested pipeline did not finish within its bounded wait")
		case <-time.After(2 * time.Second):
		}
	}
}
