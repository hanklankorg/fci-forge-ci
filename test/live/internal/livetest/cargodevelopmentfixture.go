// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
)

// This is a fixture projection, not a general stale-action rewrite. The complete
// admitted f50369f1 workflow (including pins, setup, tests and receipt exporter)
// is authenticated before replacing exactly one obsolete publication step.
const cargoDevelopmentWorkflowSHA256 = "1b3a661dcdfea6df21fcd4a9312af3e705bbf07ff51c1b332133aabff5830367"

const cargoLegacyPublication = `        uses: https://codefloe.com/itiquette/forge-ci/.forgejo/actions/build-release-image@ea604df3f4746d9effe2ddd9d9574de0b0c2ae8c
        with:
          role: runtime
          tag: dev-${{ forgejo.sha }}
          # Explicit ledger scope is required for non-semver OCI tags; no Git tag is created.
          release-tag: dev-${{ forgejo.sha }}
          revision: ${{ forgejo.sha }}
          containerfile: Containerfile
          builds: '[{"platform":"linux/amd64"}]'
          scan-output: dist/trivy-results-runtime-{arch}.json
          title: hello-cargo-container
          auth-file: ${{ steps.registry-login.outputs.auth-file }}
`

func cargoDevelopmentFixture(ctx context.Context, source Consumer, getenv func(string) string) (Consumer, error) {
	body, err := githubWorkflow(ctx, source, ".forgejo/workflows/development.yml")
	if err != nil {
		return source, err
	}

	policy, err := readAdminInputs(getenv)
	if err != nil {
		return source, err
	}

	rendered, err := cargoDevelopmentWorkflow(body, policy["enforce-vulnerabilities"])
	if err != nil {
		return source, err
	}

	return stageDevelopmentWorkflow(ctx, source, rendered)
}

func cargoDevelopmentWorkflow(body string, enforce bool) (string, error) {
	const runner = "runs-on: ${{ fromJSON(vars.CI_RUNNER_LABEL_CONTAINER) }}"

	admitted := strings.Replace(body, runner, "runs-on: ${{ vars.CI_RUNNER_LABEL }}", 1)
	if !strings.Contains(body, runner) || fmt.Sprintf("%x", sha256.Sum256([]byte(admitted))) != cargoDevelopmentWorkflowSHA256 || strings.Count(body, cargoLegacyPublication) != 1 {
		return "", fmt.Errorf("cargo publication fixture refuses an unknown historical workflow: %w", errs.ErrValidation)
	}

	// Keep expression values out of shell source; quote every environment value.
	// role/release-tag belonged to the removed release ledger, not this native
	// development route. No stable alias, ledger or signing step is introduced.
	publication := fmt.Sprintf(`        env:
          DEVELOPMENT_SHA: ${{ forgejo.sha }}
          REGISTRY_AUTH_FILE: ${{ steps.registry-login.outputs.auth-file }}
          DEVELOPMENT_SERVER_URL: ${{ forgejo.server_url }}
          DEVELOPMENT_REPOSITORY: ${{ forgejo.repository }}
        run: |
          set -euo pipefail
          forge-ci --enforce-vulnerabilities=%t container build-push-oci-image \
            --tag "dev-${DEVELOPMENT_SHA}" \
            --revision "$DEVELOPMENT_SHA" \
            --containerfile Containerfile \
            --context . \
            --builds-json '[{"platform":"linux/amd64"}]' \
            --scan-output-template 'dist/trivy-results-runtime-{arch}.json' \
            --title hello-cargo-container \
            --auth-file "$REGISTRY_AUTH_FILE" \
            --server-url "$DEVELOPMENT_SERVER_URL" \
            --repository "$DEVELOPMENT_REPOSITORY"
`, enforce)

	return strings.Replace(body, cargoLegacyPublication, publication, 1), nil
}
