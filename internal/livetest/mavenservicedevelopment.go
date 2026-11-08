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
	"slices"
	"strconv"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/container"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
)

var (
	errServiceRecipeOrImage     = errors.New("name an absolute native recipe directory and a full digest-pinned Java job image")
	errServiceReadinessURL      = errors.New("readiness must be an explicit http://127.0.0.1:<port>/<project-endpoint> URL")
	errServiceReadinessPort     = errors.New("readiness requires a valid container port")
	errServiceSourceBranch      = errors.New("select a valid source branch different from policy main")
	errServiceFixtureRelative   = errors.New("fixture must be an absolute CI script path")
	errServiceFixtureNotRegular = errors.New("fixture must be a regular CI script")
	errMavenServiceProducer     = errors.New("the Maven service development workflow checks a root Maven application, so its snapshot's producers are Maven")
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
// recipes, overlaying CI only in Consumer's private index: the service
// snapshot's declaration, the Maven service development workflow that takes
// it and checks the root Maven application's JAR and the published image's
// readiness, and the fixture script. The original declaration selects
// artifact-first or container-first; checked JAR/image byte equality is NOT
// claimed.
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

	service, err := serviceSnapshotDeclaration(ctx, source, GitLabDefaultArtifactsConfig)
	if err != nil {
		return source, err
	}

	if err = mavenServiceProducers(service.config); err != nil {
		return source, err
	}

	artifact, err := mavenLibraryArtifact(ctx, source)
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

	staged, err := service.stageFiles(ctx, map[string]string{
		forgejoDevelopmentWorkflow:     workflow,
		ServiceSnapshotDeclarationFile: service.declaration,
		".forge-ci/service-fixture.sh": fixture,
	})

	return staged.Consumer, err
}

// mavenServiceProducers holds an admitted service snapshot's declaration to
// the Maven service development workflow, the one development route Forgejo
// has: the producers its container is built from, when it has any, are Maven
// applications, which the workflow's check job builds again as the root
// Maven application. A container-first service has none.
func mavenServiceProducers(cfg *config.Config) error {
	for _, artifact := range cfg.Artifacts {
		if slices.Contains(cfg.Containers[0].From, artifact.Name) && artifact.ProjectType != projecttype.Maven {
			return errMavenServiceProducer
		}
	}

	return nil
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
