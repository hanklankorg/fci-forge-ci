// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// The overlay is hand-kept, so a malformed line is refused with its number
// rather than silently dropped from the matrices.
func TestReadMatrixEvidenceRefusesMalformedLines(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"short":    "forgejo hello-maven-service lifecycle\n",
		"provider": "bitbucket hello-maven-service lifecycle ✅\n",
		"twice":    "forgejo hello-maven-service lifecycle ✅\nforgejo hello-maven-service lifecycle ◐ again\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, filepath.Join(t.TempDir(), "evidence.txt"), "# comment\n\n"+body)
			if _, err := readMatrixEvidence(path); err == nil || !strings.Contains(err.Error(), "evidence.txt:") {
				t.Fatalf("malformed overlay: %v, want a refusal naming the line", err)
			}
		})
	}
}

// The matrices are read from the tree, so the shipped tree decides them: the
// Maven service is the one demo with a lifecycle on both forges, the Cargo
// container has a GitLab line, the npm service is admitted on GitLab without
// a line, and a refusal carries the planner's or the admission's reason.
func TestRenderTestingMatricesReadsTheShippedTree(t *testing.T) {
	t.Parallel()

	page, err := RenderTestingMatrices(reporoot.Path(t), "<!-- generated:project-kinds -->\n<!-- /generated:project-kinds -->\n<!-- generated:e2e-variants -->\n<!-- /generated:e2e-variants -->\n")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"| hello-maven-service (java · artifact-first · one image) |",
		"| hello-cargo-container (rust · container-first · one image) |",
		"| hello-npm-service (node · artifact-first · one image) |",
		"| ⛔ / admitted / — |",
		"| hello-go-cli (go · no container · no image) |",
		"- hello-go-service, gitlab: .forge-ci/artifacts.yml is not a service snapshot's declaration",
		"|  | container-first | ⛔ hello-cargo-container | 📝 hello-cargo-container | — |",
		"| architecture | amd64 |",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("matrices lack %q:\n%s", want, page)
		}
	}

	if _, err := RenderTestingMatrices(reporoot.Path(t), "no markers"); err == nil {
		t.Fatal("a page without the generated markers was rendered")
	}
}
