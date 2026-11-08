// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package container

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apprelease "github.com/diggsweden/forge-ci/internal/app/release"
	"github.com/diggsweden/forge-ci/internal/domain/imageledger"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/testutil/mockbinary"
)

func rootAdmissionCLIHash(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

func rootAdmissionCLIArgs(t *testing.T) ([]string, string, string) {
	t.Helper()
	writeReleaseJobsArtifacts(t)
	require.NoError(t, releaseImagesPlanCmd().Run(t.Context(), releaseJobsPlanArgs("--output", "ordinary.json")))

	body, err := os.ReadFile("ordinary.json")
	require.NoError(t, err)

	var plan pipeline.ReleaseImagesPlan
	require.NoError(t, json.Unmarshal(body, &plan))
	image := &plan.Images[0]
	image.ImageKind, image.Role, image.Flavor, image.BaseInputID = "base", "base", "opaque", strings.Repeat("a", 64)
	image.FinalTagName = image.BaseInputID + "-opaque"

	image.CandidateTag, image.Revision = "staging-"+image.FinalTagName, strings.Repeat("c", 64)
	for i := range plan.Builds {
		plan.Builds[i].FinalTagName, plan.Builds[i].CandidateTag, plan.Builds[i].Revision = image.FinalTagName, image.CandidateTag, image.Revision
	}

	body, err = json.Marshal(plan)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("approved.json", body, 0o600))
	root := t.TempDir()
	sbom := []byte(`{"bomFormat":"CycloneDX"}`)
	sbomPath := imageledger.DefaultSBOMPath(imageledger.ImageKindBase, image.Flavor, image.Role)
	require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(sbomPath)), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, sbomPath), sbom, 0o600))
	entry := imageledger.Entry{
		Role: image.Role, Flavor: image.Flavor, ImageKind: imageledger.ImageKindBase, BaseInputID: image.BaseInputID,
		Digest: "sha256:" + strings.Repeat("d", 64), Ref: image.ImageRepository + "@sha256:" + strings.Repeat("d", 64),
		FinalTag: image.ImageRepository + ":" + image.FinalTagName, CandidateTag: image.ImageRepository + ":" + image.CandidateTag,
		SBOM: sbomPath, SBOMSHA256: rootAdmissionCLIHash(sbom),
	}
	ledger, err := imageledger.Marshal([]imageledger.Entry{entry})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dist"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dist/base-images.json"), ledger, 0o600))
	digest, err := apprelease.CanonicalPayloadDigest(root)
	require.NoError(t, err)

	source, commit, workflow := "https://forge.example/group/source", strings.Repeat("b", 40), "bases.yml"

	return []string{
		"root-base", "admit-payload", "--plan", "approved.json", "--image-repository", image.ImageRepository,
		"--expected-plan-sha256", rootAdmissionCLIHash(body),
		"--source", source, "--source-sha", commit, "--workflow", workflow,
		"--expected-source", source, "--expected-source-sha", commit, "--expected-workflow", workflow,
		"--evidence-root", root, "--expected-payload-digest", digest,
	}, root, sbomPath
}

func TestRootAdmissionCLIIsCredentialFreeAndReadOnly(t *testing.T) {
	for _, defect := range []string{"none", "source commit", "source repository", "workflow", "plan digest", "missing approval flag", "payload digest", "plan file", "SBOM mutation", "missing pin", "outside root", "symlink root", "missing ledger", "key flag"} {
		t.Run(defect, func(t *testing.T) {
			args, root, sbomPath := rootAdmissionCLIArgs(t)
			set := func(flag, value string) {
				for i, arg := range args {
					if arg == flag {
						args[i+1] = value

						return
					}
				}

				t.Fatalf("missing fixture flag %s", flag)
			}

			switch defect {
			case "source commit":
				set("--source-sha", strings.Repeat("e", 40))
			case "source repository":
				set("--source", "https://forge.example/foreign/source")
			case "workflow":
				set("--workflow", "foreign.yml")
			case "plan digest":
				set("--expected-plan-sha256", strings.Repeat("e", 64))
			case "missing approval flag":
				for i, arg := range args {
					if arg == "--expected-plan-sha256" {
						t.Setenv("EXPECTED_PLAN_SHA256", args[i+1])

						args = append(args[:i], args[i+2:]...)

						break
					}
				}
			case "payload digest":
				set("--expected-payload-digest", strings.Repeat("e", 64))
			case "plan file":
				set("--plan", filepath.Join(t.TempDir(), "missing"))
			case "SBOM mutation":
				require.NoError(t, os.WriteFile(filepath.Join(root, sbomPath), []byte(`{"changed":true}`), 0o600))
			case "missing pin":
				file := filepath.Join(root, "dist/base-images.json")
				body, err := os.ReadFile(file)
				require.NoError(t, err)

				var entries []imageledger.Entry
				require.NoError(t, json.Unmarshal(body, &entries))
				entries[0].SBOMSHA256 = ""
				body, err = json.Marshal(entries)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(file, body, 0o600))
			case "outside root":
				set("--evidence-root", filepath.Join(t.TempDir(), "missing"))
			case "symlink root":
				alias := filepath.Join(t.TempDir(), "alias")
				require.NoError(t, os.Symlink(root, alias))
				set("--evidence-root", alias)
			case "missing ledger":
				require.NoError(t, os.Remove(filepath.Join(root, "dist/base-images.json")))
			case "key flag":
				args = append(args, "--key", "env://COSIGN_KEY")
			}

			if defect == "SBOM mutation" || defect == "missing pin" || defect == "missing ledger" {
				digest, err := apprelease.CanonicalPayloadDigest(root)
				require.NoError(t, err)
				set("--expected-payload-digest", digest)
			}

			// A malformed private key and nonexistent credential paths must have
			// no bearing on either successful or refused offline admission.
			t.Setenv("COSIGN_KEY", "not-a-private-key")
			t.Setenv("COSIGN_PASSWORD", "unused")
			t.Setenv("REGISTRY_PASSWORD_FILE", filepath.Join(t.TempDir(), "never-read"))
			bins := mockbinary.New(t)
			bins.OnPath()

			for _, tool := range []string{"cosign", "syft", "trivy", "buildah", "docker", "git"} {
				bins.Add(tool, "exit 99")
			}

			before, err := apprelease.CanonicalPayloadDigest(root)
			require.NoError(t, err)

			err = rootBaseGroup().Run(t.Context(), args)
			if defect == "none" {
				require.NoError(t, err)
				// Plan-only admission has the same independent source binding,
				// and requires no payload or native executable.
				args[1] = "admit-plan"
				args = args[:len(args)-4]
				require.NoError(t, rootBaseGroup().Run(t.Context(), args))
			} else {
				require.Error(t, err)
			}

			after, err := apprelease.CanonicalPayloadDigest(root)
			require.NoError(t, err)
			require.Equal(t, before, after)

			for _, tool := range []string{"cosign", "syft", "trivy", "buildah", "docker", "git"} {
				require.Empty(t, bins.Invocations(tool), tool)
			}
		})
	}
}
