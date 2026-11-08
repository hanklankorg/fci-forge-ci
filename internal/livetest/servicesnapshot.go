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
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"gopkg.in/yaml.v3"
)

// A service snapshot is the development route whose pipeline the forge's
// snapshot-images policy owns, the reusable workflow on Forgejo and the
// component on GitLab, over the consumer's own declaration of one container.
// The trial overlays that declaration, native linux/amd64 and unsigned, into
// ServiceSnapshotDeclarationFile and judges the snapshot by the frozen plan
// and the receipts the route leaves. A consumer whose declaration the snapshot
// cannot build has no development route, and a lifecycle refuses it.

var errServiceDeclaration = errors.New("a service snapshot requires one container of the root Containerfile and context, native amd64, without image name, tag suffix, build secrets, extraction or provenance, from at most one root artifact of a snapshot family, an application where the family has libraries")

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
// declaration overlaid for the trial, parsed, and the toolchain families of
// the producers the snapshot builds its container from.
type serviceSnapshot struct {
	Consumer

	declaration string
	config      *config.Config
	families    []string
}

// serviceSnapshotDeclaration admits the consumer for a service snapshot: a
// root Containerfile, and at source a declaration the overlay accepts.
func serviceSnapshotDeclaration(ctx context.Context, consumer Consumer, source string) (serviceSnapshot, error) {
	if _, err := githubWorkflow(ctx, consumer, "Containerfile"); err != nil {
		return serviceSnapshot{}, err
	}

	declared, err := githubWorkflow(ctx, consumer, source)
	if err != nil {
		return serviceSnapshot{}, err
	}

	declaration, cfg, err := serviceSnapshotDeclarationOverlay(declared)
	if err != nil {
		return serviceSnapshot{}, err
	}

	return serviceSnapshot{Consumer: consumer, declaration: declaration, config: cfg, families: serviceSnapshotFamilies(cfg)}, nil
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

	ctx, cancel := context.WithTimeout(Context(tb), timeoutLayerPush)
	defer cancel()

	declared, err := serviceSnapshotDeclaration(ctx, consumer, source)
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

	return serviceSnapshotFamilies(readServiceSnapshotDeclaration(ctx, tb, consumer))
}

// ServiceSnapshotPlan reads the staged declaration through the same Git
// reader as the fixtures, then plans it with the shared planner. Acceptance
// must not assume that every service is container-first.
func ServiceSnapshotPlan(ctx context.Context, tb TB, consumer Consumer, in snapshotimage.Input) snapshotimage.Plan {
	tb.Helper()

	plan, err := snapshotimage.New(pipeline.NewConfigPlan(readServiceSnapshotDeclaration(ctx, tb, consumer)), in)
	if err != nil {
		tb.Fatalf("service plan: %v", err)
	}

	return plan
}

// readServiceSnapshotDeclaration parses the consumer's staged declaration.
func readServiceSnapshotDeclaration(ctx context.Context, tb TB, consumer Consumer) *config.Config {
	tb.Helper()

	body, err := githubWorkflow(ctx, consumer, ServiceSnapshotDeclarationFile)
	if err != nil {
		tb.Fatalf("service declaration: %v", err)
	}

	cfg, err := parseServiceDeclaration(body)
	if err != nil {
		tb.Fatalf("service declaration: %v", err)
	}

	return cfg
}

// serviceSnapshotDeclarationOverlay preserves the declared build graph and
// raw ecosystem settings. Only the trial's native platform and unsigned
// policy are overlaid; release signing settings remain in the original
// declaration. It returns the overlay and the parsed original.
func serviceSnapshotDeclarationOverlay(body string) (string, *config.Config, error) {
	cfg, err := parseServiceDeclaration(body)
	if err != nil {
		return "", nil, err
	}

	if err = validateServiceSnapshot(cfg); err != nil {
		return "", nil, err
	}

	ctr := &cfg.Containers[0]
	ctr.Platforms = build.DefaultPlatform
	ctr.EnableSLSA = new(bool)
	// Artifact's typed ecosystem fields are not YAML-marshallable. Keep their
	// original nodes after the strict shared parser has admitted the whole input.
	var raw struct {
		Artifacts yaml.Node `yaml:"artifacts,omitempty"`
	}
	if err = yaml.Unmarshal([]byte(body), &raw); err != nil {
		return "", nil, err
	}

	overlay, err := yaml.Marshal(struct {
		Artifacts  yaml.Node          `yaml:"artifacts,omitempty"`
		Containers []config.Container `yaml:"containers"`
	}{raw.Artifacts, cfg.Containers})
	if err != nil {
		return "", nil, err
	}
	// An alias whose anchor lived in an omitted release-policy block must not
	// leave an invalid private declaration behind.
	if _, err = parseServiceDeclaration(string(overlay)); err != nil {
		return "", nil, err
	}

	return string(overlay), cfg, nil
}

// validateServiceSnapshot holds a declaration to what a service snapshot
// builds: one container, built from the root Containerfile and context on
// native amd64, plain, from producers the snapshot has.
func validateServiceSnapshot(cfg *config.Config) error {
	if len(cfg.Containers) != 1 {
		return errServiceDeclaration
	}

	typed := pipeline.NewConfigPlan(cfg)
	if !serviceSnapshotContainer(cfg.Containers[0], typed.Containers.All[0]) {
		return errServiceDeclaration
	}

	for _, artifact := range serviceSnapshotProducers(typed, cfg.Containers[0].From) {
		if !serviceSnapshotProducer(artifact) {
			return errServiceDeclaration
		}
	}

	return nil
}

// serviceSnapshotContainer reports whether the declared container is one a
// service snapshot builds: the root Containerfile and context, native amd64,
// plain, from at most one artifact. Unsupported provenance is not silently
// turned into a supported request.
func serviceSnapshotContainer(ctr config.Container, selected pipeline.PlannedContainer) bool {
	plain := ctr.ImageName == "" && ctr.TagSuffix == "" && len(ctr.From) <= 1 && len(ctr.BuildSecrets) == 0 && ctr.Extract == nil &&
		(ctr.EnableSLSA == nil || !*ctr.EnableSLSA)

	return plain && selected.ContainerFile == "Containerfile" && selected.Context == "." && serviceSnapshotSupportsAMD64(selected.Platforms)
}

// serviceSnapshotProducer reports whether an artifact is one a service
// snapshot builds first: a root artifact of a snapshot family, an application
// where the family has libraries.
func serviceSnapshotProducer(artifact pipeline.PlannedArtifact) bool {
	if _, err := snapshotimage.ToolchainFamily(artifact.ProjectType); err != nil || artifact.WorkingDirectory != "." {
		return false
	}

	return !projecttype.IsIn(artifact.ProjectType, []projecttype.Type{projecttype.Maven, projecttype.Gradle}) || artifact.BuildType == config.BuildTypeApplication
}

func serviceSnapshotSupportsAMD64(platforms string) bool {
	hasAMD64 := false
	seen := map[string]bool{}

	for _, platform := range strings.FieldsFunc(platforms, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if seen[platform] || (platform != build.DefaultPlatform && platform != "linux/arm64") {
			return false
		}

		seen[platform] = true
		hasAMD64 = hasAMD64 || platform == build.DefaultPlatform
	}

	return hasAMD64
}

// serviceSnapshotProducers are the artifacts the container is built from
// that the snapshot builds first: as the planner, a container-first Go or
// Cargo artifact is built by the Containerfile and is no producer.
func serviceSnapshotProducers(typed pipeline.ConfigPlan, from []string) []pipeline.PlannedArtifact {
	producers := []pipeline.PlannedArtifact{}

	for _, name := range from {
		for _, artifact := range typed.Artifacts.All {
			containerFirst := (artifact.ProjectType == projecttype.Go && artifact.GoBuildMode == config.GoBuildModeContainerFirst) ||
				(artifact.ProjectType == projecttype.Cargo && artifact.CargoBuildMode == config.CargoBuildModeContainerFirst)
			if artifact.Name == name && !containerFirst {
				producers = append(producers, artifact)
			}
		}
	}

	return producers
}

// serviceSnapshotFamilies lists the toolchain families of an admitted
// declaration's producers, each once, sorted.
func serviceSnapshotFamilies(cfg *config.Config) []string {
	typed := pipeline.NewConfigPlan(cfg)
	families := []string{}

	for _, artifact := range serviceSnapshotProducers(typed, cfg.Containers[0].From) {
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
