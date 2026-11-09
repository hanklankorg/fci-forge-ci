// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/build"
	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/domain/version"
	"gopkg.in/yaml.v3"
)

// A service snapshot is the development route whose pipeline the forge's
// snapshot-images policy owns, the reusable workflow on Forgejo and the
// component on GitLab, over the consumer's own declaration of one container.
// The trial overlays that declaration, native linux/amd64 and unsigned, into
// ServiceSnapshotDeclarationFile and judges the snapshot by the frozen plan
// and the receipts the route leaves. What a snapshot builds is the planner's
// rule (snapshotimage.New), and the harness keeps no copy of it: a
// declaration is admitted by planning it. The harness adds its own acceptance
// scope alone, serviceSnapshotScope. A consumer whose declaration the planner
// refuses has no development route, and a lifecycle refuses it.
//
// The snapshot event takes the same route over every image the planner
// plans from the declaration, under snapshotImagesScope: the trial judges
// each planned image's receipt, and the plan, not the case, says which
// images those are.

var (
	errServiceDeclaration = errors.New("the trial judges one container of the root Containerfile and context, planned for amd64, without image name, tag suffix or declared provenance, from at most one artifact")
	errSnapshotImages     = errors.New("the trial judges every planned image in the run's one image repository, each planned for amd64, without image name or declared provenance")
)

// snapshotScope is a trial's acceptance scope over a declaration the planner
// plans: the declarations it admits, and its refusal of the others.
type snapshotScope struct {
	admits  func(cfg *config.Config) bool
	refusal error
	// rootContainerfile is whether the scope builds from the root
	// Containerfile, which a consumer must then have.
	rootContainerfile bool
}

var (
	// serviceScope is the lifecycle's: one service, serviceSnapshotScope.
	serviceScope = snapshotScope{
		admits:            func(cfg *config.Config) bool { return serviceSnapshotScope(cfg) == nil },
		refusal:           errServiceDeclaration,
		rootContainerfile: true,
	}
	// snapshotImagesScope is the snapshot event's: every planned image.
	snapshotImagesScope = snapshotScope{admits: snapshotImagesAdmitted, refusal: errSnapshotImages}
)

// ServiceSnapshotDeclarationFile is the consumer path of the service
// snapshot's declaration: the consumer's own declaration, overlaid for the
// trial's native platform and unsigned policy. A development route names it
// as its artifacts config.
const ServiceSnapshotDeclarationFile = ".forge-ci/service-development.yml"

// ServiceSnapshotSource names the declaration a consumer's service snapshot
// is taken from: the dedicated release-images declaration when the consumer
// has one, since its releases build their images from it, else the default.
func ServiceSnapshotSource(consumerRoot string) (string, error) {
	_, dedicated, err := GitLabReleaseImages(consumerRoot)
	if err != nil {
		return "", err
	}

	return cmp.Or(dedicated, GitLabDefaultArtifactsConfig), nil
}

// serviceSnapshot is a consumer admitted for a service snapshot: its
// declaration overlaid for the trial, and the plan the planner makes of it.
type serviceSnapshot struct {
	Consumer

	declaration string
	plan        snapshotimage.Plan
}

// serviceSnapshotDeclaration admits the consumer for a snapshot in scope: at
// source a declaration the planner plans, and the root Containerfile where
// the scope builds from it.
func serviceSnapshotDeclaration(ctx context.Context, consumer Consumer, source string, scope snapshotScope) (serviceSnapshot, error) {
	if scope.rootContainerfile {
		if _, err := githubWorkflow(ctx, consumer, "Containerfile"); err != nil {
			return serviceSnapshot{}, err
		}
	}

	declared, err := githubWorkflow(ctx, consumer, source)
	if err != nil {
		return serviceSnapshot{}, err
	}

	declaration, plan, err := snapshotDeclarationOverlay(declared, scope)
	if err != nil {
		return serviceSnapshot{}, err
	}

	return serviceSnapshot{Consumer: consumer, declaration: declaration, plan: plan}, nil
}

// stageFiles commits the files onto the admitted tree, in the consumer's
// private index alone.
func (snapshot serviceSnapshot) stageFiles(ctx context.Context, files map[string]string) (serviceSnapshot, error) {
	if _, err := snapshot.git.run(ctx, snapshot.stage, "read-tree", snapshot.Commit); err != nil {
		return snapshot, err
	}

	for path, text := range files {
		if err := snapshot.stageFile(ctx, "100644", path, text); err != nil {
			return snapshot, err
		}
	}

	commit, err := snapshot.commit(ctx, consumerBaseMessage)
	if err != nil {
		return snapshot, err
	}

	snapshot.Commit = commit

	return snapshot, nil
}

// WithServiceSnapshotDeclaration returns the consumer with the service
// snapshot's declaration in its tree, ServiceSnapshotDeclarationFile overlaid
// from source, and nothing else: a route whose pipeline the forge's policy
// owns, as GitLab's snapshot-images component, reads the declaration from the
// consumer and needs no workflow of the consumer's.
func (consumer Consumer) WithServiceSnapshotDeclaration(tb TB, source string) Consumer {
	tb.Helper()

	return consumer.withSnapshotDeclaration(tb, source, serviceScope)
}

// WithSnapshotImagesDeclaration is WithServiceSnapshotDeclaration for the
// snapshot event: the declaration overlaid for every image the planner plans
// from it (snapshotImagesScope).
func (consumer Consumer) WithSnapshotImagesDeclaration(tb TB, source string) Consumer {
	tb.Helper()

	return consumer.withSnapshotDeclaration(tb, source, snapshotImagesScope)
}

func (consumer Consumer) withSnapshotDeclaration(tb TB, source string, scope snapshotScope) Consumer {
	tb.Helper()

	ctx, cancel := context.WithTimeout(Context(tb), timeoutLayerPush)
	defer cancel()

	declared, err := serviceSnapshotDeclaration(ctx, consumer, source, scope)
	if err == nil {
		declared, err = declared.stageFiles(ctx, map[string]string{ServiceSnapshotDeclarationFile: declared.declaration})
	}

	if err != nil {
		tb.Fatalf("service snapshot declaration: %v", err)
	}

	return declared.Consumer
}

// ServiceSnapshotFamilies lists the toolchain families of the producers the
// consumer's staged snapshot declaration builds its container from, each
// once, sorted: none for a container-first service, whose Containerfile
// builds the application itself.
func ServiceSnapshotFamilies(ctx context.Context, tb TB, consumer Consumer) []string {
	tb.Helper()

	in, err := serviceSnapshotStandInInput()
	if err != nil {
		tb.Fatalf("service plan input: %v", err)
	}

	return serviceSnapshotFamilies(ServiceSnapshotPlan(ctx, tb, consumer, in))
}

// ServiceSnapshotPlan reads the staged declaration through the same Git
// reader as the fixtures, then plans it with the shared planner. Acceptance
// must not assume that every service is container-first.
func ServiceSnapshotPlan(ctx context.Context, tb TB, consumer Consumer, in snapshotimage.Input) snapshotimage.Plan {
	tb.Helper()

	body, err := githubWorkflow(ctx, consumer, ServiceSnapshotDeclarationFile)
	if err != nil {
		tb.Fatalf("service declaration: %v", err)
	}

	cfg, err := parseServiceDeclaration(body)
	if err != nil {
		tb.Fatalf("service declaration: %v", err)
	}

	plan, err := snapshotimage.New(pipeline.NewConfigPlan(cfg), in)
	if err != nil {
		tb.Fatalf("service plan: %v", err)
	}

	return plan
}

// serviceSnapshotStandInInput is the planner input a declaration is admitted
// with before the trial has a source: the planner's rules on a declaration do
// not depend on the source identity, so a stand-in one plans it.
func serviceSnapshotStandInInput() (snapshotimage.Input, error) {
	const (
		shortSHA = "aaaaaaa"
		branch   = "development"
	)

	tag, err := version.ComposeSnapshotContainerTag("0.0.0", branch, shortSHA)
	if err != nil {
		return snapshotimage.Input{}, err
	}

	return snapshotimage.Input{
		SourceSHA: shortSHA + strings.Repeat("a", 33), SourceBranch: branch, Tag: tag,
		ImageRepository: "registry.invalid/service/snapshot", SourceDateEpoch: "1", Publish: true,
	}, nil
}

// serviceSnapshotDeclarationOverlay is snapshotDeclarationOverlay in the
// lifecycle's scope, one service.
func serviceSnapshotDeclarationOverlay(body string) (string, snapshotimage.Plan, error) {
	return snapshotDeclarationOverlay(body, serviceScope)
}

// snapshotDeclarationOverlay preserves the declared build graph and raw
// ecosystem settings. Only the trial's native platform and unsigned policy
// are overlaid; release signing settings remain in the original declaration.
// The overlay is then planned as the snapshot will plan it, and the
// planner's refusal is the declaration's; the scope's refusal is the
// trial's. It returns the overlay and its plan.
func snapshotDeclarationOverlay(body string, scope snapshotScope) (string, snapshotimage.Plan, error) {
	cfg, err := parseServiceDeclaration(body)
	if err != nil {
		return "", snapshotimage.Plan{}, err
	}

	if !scope.admits(cfg) {
		return "", snapshotimage.Plan{}, scope.refusal
	}

	in, err := serviceSnapshotStandInInput()
	if err != nil {
		return "", snapshotimage.Plan{}, err
	}

	// The planner judges the declaration's snapshot, with the platforms as
	// declared, before the overlay narrows them.
	original, err := snapshotimage.New(pipeline.NewConfigPlan(snapshotView(cfg)), in)
	if err != nil {
		return "", snapshotimage.Plan{}, err
	}

	if len(original.Images) == 0 || slices.ContainsFunc(original.Images, func(image snapshotimage.Image) bool {
		return !slices.Contains(image.Platforms, build.DefaultPlatform)
	}) {
		return "", snapshotimage.Plan{}, scope.refusal
	}

	for i := range cfg.Containers {
		cfg.Containers[i].Platforms = build.DefaultPlatform
	}

	cfg = snapshotView(cfg)
	// Artifact's typed ecosystem fields are not YAML-marshallable. Keep their
	// original nodes after the strict shared parser has admitted the whole input.
	var raw struct {
		Artifacts yaml.Node `yaml:"artifacts,omitempty"`
	}
	if err = yaml.Unmarshal([]byte(body), &raw); err != nil {
		return "", snapshotimage.Plan{}, err
	}

	overlay, err := encodeYAML(struct {
		Artifacts  yaml.Node          `yaml:"artifacts,omitempty"`
		Containers []config.Container `yaml:"containers"`
	}{raw.Artifacts, cfg.Containers})
	if err != nil {
		return "", snapshotimage.Plan{}, err
	}
	// An alias whose anchor lived in an omitted release-policy block must not
	// leave an invalid private declaration behind.
	overlaid, err := parseServiceDeclaration(overlay)
	if err != nil {
		return "", snapshotimage.Plan{}, err
	}

	plan, err := snapshotimage.New(pipeline.NewConfigPlan(overlaid), in)
	if err != nil {
		return "", snapshotimage.Plan{}, err
	}

	return overlay, plan, nil
}

// snapshotView is the declaration as an unsigned snapshot builds it: no
// container requests OCI provenance, which a snapshot never carries. The
// declaration itself is left as it is.
func snapshotView(cfg *config.Config) *config.Config {
	view := *cfg
	view.Containers = slices.Clone(cfg.Containers)

	for i := range view.Containers {
		view.Containers[i].EnableSLSA = new(bool)
	}

	return &view
}

// serviceSnapshotScope is the trial's acceptance scope, not a product rule:
// the planner decides what a snapshot builds; the trial judges one image,
// built from the root Containerfile and context, under the repository and
// tag it derives, so it takes one container without image name or tag suffix,
// from at most one artifact. The overlay narrows the platforms to native
// amd64, which the planned declaration must therefore include. A snapshot
// carries no provenance, so the overlay turns a release's default provenance
// off; a declared request for provenance is refused instead, so it is not
// silently dropped.
func serviceSnapshotScope(cfg *config.Config) error {
	if len(cfg.Containers) != 1 {
		return errServiceDeclaration
	}

	ctr := cfg.Containers[0]
	selected := pipeline.NewConfigPlan(cfg).Containers.All[0]

	if ctr.ImageName != "" || ctr.TagSuffix != "" || len(ctr.From) > 1 || (ctr.EnableSLSA != nil && *ctr.EnableSLSA) ||
		selected.ContainerFile != "Containerfile" || selected.Context != "." {
		return errServiceDeclaration
	}

	return nil
}

// snapshotImagesAdmitted is the snapshot event's acceptance scope, not a
// product rule: the planner decides what a snapshot builds, and the trial
// judges every image it plans, each under the run's one image repository,
// so no container names an image of its own. The overlay narrows every
// image's platforms to native amd64, and turns a release's default
// provenance off; a declared request for provenance is refused instead, as
// in serviceSnapshotScope.
func snapshotImagesAdmitted(cfg *config.Config) bool {
	return !slices.ContainsFunc(cfg.Containers, func(ctr config.Container) bool {
		return ctr.ImageName != "" || (ctr.EnableSLSA != nil && *ctr.EnableSLSA)
	})
}

// ServiceReadiness is whether a snapshot of the plan checks a service's
// readiness: where the plan's one image is the service. A plan of several
// images declares no primary service among them, so none is probed, and the
// snapshot is judged by its receipts alone.
func ServiceReadiness(plan snapshotimage.Plan) bool {
	return len(plan.Images) == 1
}

// serviceSnapshotFamilies lists the toolchain families of a plan's producers,
// each once, sorted.
func serviceSnapshotFamilies(plan snapshotimage.Plan) []string {
	families := []string{}

	for _, artifact := range plan.Artifacts {
		// The planner admitted every producer by this mapping.
		if family, err := snapshotimage.ToolchainFamily(artifact.ProjectType); err == nil {
			families = append(families, family)
		}
	}

	slices.Sort(families)

	return slices.Compact(families)
}

func parseServiceDeclaration(body string) (*config.Config, error) {
	cfg, err := config.Parse([]byte(body))
	if err != nil {
		return nil, err
	}

	if err = config.Validate(cfg); err != nil {
		return nil, err
	}

	if err = config.Derive(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
