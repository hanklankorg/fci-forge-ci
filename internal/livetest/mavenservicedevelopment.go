// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/build"
	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"gopkg.in/yaml.v3"
)

var (
	errServiceRecipeOrImage     = errors.New("name an absolute native recipe directory and a full digest-pinned Java job image")
	errServiceReadinessURL      = errors.New("readiness must be an explicit http://127.0.0.1:<port>/<project-endpoint> URL")
	errServiceReadinessPort     = errors.New("readiness requires a valid container port")
	errServiceSourceBranch      = errors.New("select a valid source branch different from policy main")
	errServiceFixtureRelative   = errors.New("fixture must be an absolute CI script path")
	errServiceFixtureNotRegular = errors.New("fixture must be a regular CI script")
	errServiceDeclaration       = errors.New("service trial requires one root Containerfile/context, native amd64, and only a root Maven application dependency")
)

// MavenServiceDevelopmentConfig supplies trusted trial infrastructure. Fixture
// is a CI-side script producing trial-runtime/runtime.env and optional files;
// it runs in the Java job, never on the controller or in the source checkout.
type MavenServiceDevelopmentConfig struct {
	RecipeDirectory, JavaImage, Branch, ReadinessURL, Fixture string
}

//go:embed testdata/maven-service-development.yml.tmpl
var mavenServiceDevelopmentRecipe string

// NewMavenServiceDevelopmentFixture reuses the native Maven and snapshot layer
// recipes, overlaying CI only in Consumer's private index. The original
// declaration selects artifact-first or container-first; checked JAR/image byte
// equality is NOT claimed.
func NewMavenServiceDevelopmentFixture(tb TB, source Consumer, config MavenServiceDevelopmentConfig) Consumer {
	tb.Helper()

	ctx, cancel := context.WithTimeout(Context(tb), timeoutLayerPush)
	defer cancel()

	fixture, err := mavenServiceDevelopmentFixture(ctx, source, config)
	if err != nil {
		tb.Fatalf("Maven service development fixture: %v", err)
	}

	return fixture
}

func mavenServiceDevelopmentFixture(ctx context.Context, source Consumer, config MavenServiceDevelopmentConfig) (Consumer, error) {
	if err := config.validateInfrastructure(); err != nil {
		return source, err
	}

	if _, err := source.git.run(ctx, source.stage, "check-ref-format", "refs/heads/"+config.Branch); err != nil || config.Branch == defaultBranch || config.Branch == "" {
		return source, errServiceSourceBranch
	}

	artifact, err := mavenLibraryArtifact(ctx, source)
	if err != nil {
		return source, err
	}

	if _, err = githubWorkflow(ctx, source, "Containerfile"); err != nil {
		return source, err
	}

	declared, err := githubWorkflow(ctx, source, ".forge-ci/artifacts.yml")
	if err != nil {
		return source, err
	}

	declaration, err := mavenServiceSnapshotDeclaration(declared)
	if err != nil {
		return source, err
	}

	fixture, err := readMavenServiceFixture(config.Fixture)
	if err != nil {
		return source, err
	}

	body, err := os.ReadFile(filepath.Join(config.RecipeDirectory, ".forgejo/workflows/release.yml"))
	if err != nil {
		return source, err
	}

	pin, err := ReadConsumerPin(map[string]string{trialReleaseWorkflow: string(body)})
	if err != nil {
		return source, err
	}
	// YAML quoting prevents branch/endpoint metadata becoming syntax.
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

	workflow := strings.NewReplacer("@ACTIONS@", pin.Actions, "@WORKFLOWS@", pin.Workflows,
		"@PIN@", pin.Commit, "@IMAGE@", config.JavaImage, "@BRANCH@", quote(config.Branch),
		"@URL@", quote(config.ReadinessURL), "@ARTIFACT@", artifact).Replace(mavenServiceDevelopmentRecipe)
	if _, err = source.git.run(ctx, source.stage, "read-tree", source.Commit); err != nil {
		return source, err
	}

	for path, text := range map[string]string{
		forgejoDevelopmentWorkflow:          workflow,
		".forge-ci/service-development.yml": declaration,
		".forge-ci/service-fixture.sh":      fixture,
	} {
		if err = source.stageFile(ctx, "100644", path, text); err != nil {
			return source, err
		}
	}

	source.Commit, err = source.commit(ctx, consumerBaseMessage)

	return source, err
}

// mavenServiceSnapshotDeclaration preserves the declared build graph and raw
// ecosystem settings. Only the trial's native platform and unsigned policy are
// overlaid; release signing settings remain in the original declaration.
func mavenServiceSnapshotDeclaration(body string) (string, error) {
	cfg, err := parseMavenServiceDeclaration(body)
	if err != nil {
		return "", err
	}

	if err = validateMavenServiceTrial(cfg); err != nil {
		return "", err
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
		return "", err
	}

	overlay, err := yaml.Marshal(struct {
		Artifacts  yaml.Node          `yaml:"artifacts,omitempty"`
		Containers []config.Container `yaml:"containers"`
	}{raw.Artifacts, cfg.Containers})
	if err != nil {
		return "", err
	}
	// An alias whose anchor lived in an omitted release-policy block must not
	// leave an invalid private declaration behind.
	if _, err = parseMavenServiceDeclaration(string(overlay)); err != nil {
		return "", err
	}

	return string(overlay), nil
}

func validateMavenServiceTrial(cfg *config.Config) error {
	if len(cfg.Containers) != 1 {
		return errServiceDeclaration
	}

	ctr := cfg.Containers[0]
	typed := pipeline.NewConfigPlan(cfg)
	selected := typed.Containers.All[0]

	if selected.ContainerFile != "Containerfile" || selected.Context != "." || ctr.ImageName != "" || ctr.TagSuffix != "" ||
		len(ctr.From) > 1 || len(ctr.BuildSecrets) > 0 || ctr.Extract != nil {
		return errServiceDeclaration
	}
	// Do not silently turn unsupported provenance into a supported request.
	if ctr.EnableSLSA != nil && *ctr.EnableSLSA {
		return errServiceDeclaration
	}

	if !mavenServiceSupportsAMD64(selected.Platforms) {
		return errServiceDeclaration
	}

	return validateMavenServiceDependencies(typed, ctr.From)
}

func mavenServiceSupportsAMD64(platforms string) bool {
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

func validateMavenServiceDependencies(typed pipeline.ConfigPlan, from []string) error {
	for _, name := range from {
		for _, a := range typed.Artifacts.All {
			if a.Name == name && (a.ProjectType != projecttype.Maven || a.WorkingDirectory != "." || a.BuildType != config.BuildTypeApplication) {
				return errServiceDeclaration
			}
		}
	}

	return nil
}

// MavenServiceDevelopmentPlan reads the selected private declaration through
// the same Git reader as the fixture, then uses the existing typed planner.
// Acceptance must not assume that every service is container-first.
func MavenServiceDevelopmentPlan(tb TB, source Consumer, in snapshotimage.Input) snapshotimage.Plan {
	tb.Helper()

	body, err := githubWorkflow(Context(tb), source, ".forge-ci/service-development.yml")
	if err != nil {
		tb.Fatalf("service declaration: %v", err)
	}

	cfg, err := parseMavenServiceDeclaration(body)
	if err != nil {
		tb.Fatalf("service declaration: %v", err)
	}

	plan, err := snapshotimage.New(pipeline.NewConfigPlan(cfg), in)
	if err != nil {
		tb.Fatalf("service plan: %v", err)
	}

	return plan
}

func parseMavenServiceDeclaration(body string) (*config.Config, error) {
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

// validateInfrastructure checks the recipe directory, the job image and the
// readiness endpoint before any Git work.
func (c MavenServiceDevelopmentConfig) validateInfrastructure() error {
	if !filepath.IsAbs(c.RecipeDirectory) || !container.ValidDigestReference(c.JavaImage) {
		return errServiceRecipeOrImage
	}

	endpoint, err := url.Parse(c.ReadinessURL)
	if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() == "" || endpoint.Path == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return errServiceReadinessURL
	}

	if port, err := strconv.Atoi(endpoint.Port()); err != nil || port < 1 || port > 65535 {
		return errServiceReadinessPort
	}

	return nil
}

// readMavenServiceFixture reads the optional operator-supplied CI script; an
// empty path yields an empty script.
func readMavenServiceFixture(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if !filepath.IsAbs(path) {
		return "", errServiceFixtureRelative
	}

	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errServiceFixtureNotRegular
	}

	body, err := os.ReadFile(path) //nolint:gosec // operator-supplied fixture, checked above as an absolute regular file.
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// DispatchConsumerDevelopment dispatches policy main once; selected source is
// resolved by the shared snapshot planner, not by this controller.
func DispatchConsumerDevelopment(tb TB, target Target, repo string) {
	tb.Helper()
	requireAccepted(tb, target)

	ctx, cancel := context.WithTimeout(Context(tb), timeoutLayerPush)
	defer cancel()

	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(repo) {
		tb.Fatalf("invalid scratch repository")
	}

	err := discard(ctx, target, http.MethodPost, actionsURL(target, repo, "/actions/workflows/development.yml/dispatches"), map[string]string{"ref": defaultBranch}, http.StatusNoContent)
	if err != nil {
		tb.Fatalf("dispatch development policy: %v", err)
	}
}
