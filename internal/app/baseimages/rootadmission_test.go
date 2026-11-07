// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package baseimages

import (
	"context"
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
)

func admissionHash(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

func rootAdmissionFixture(t *testing.T) (RootBasePayloadAdmission, pipeline.ReleaseImagesPlan, imageledger.Entry) {
	t.Helper()

	repository, ownID, revision := "registry.example/group/root", strings.Repeat("a", 64), strings.Repeat("c", 64)
	final := ownID + "-opaque"
	plan := pipeline.ReleaseImagesPlan{
		Images: []pipeline.ReleaseImage{{
			Name: "root", Role: "base", ImageKind: "base", Flavor: "opaque", BaseInputID: ownID,
			ImageRepository: repository, FinalTagName: final, CandidateTag: "staging-" + final,
			Revision: revision, Archs: "amd64", Scan: true,
		}},
		Builds: []pipeline.ReleaseImageBuild{{
			Name: "root", Arch: "amd64", Platform: "linux/amd64", Context: ".", ContainerFile: "Containerfile",
			ImageRepository: repository, FinalTagName: final, CandidateTag: "staging-" + final, Revision: revision,
		}},
	}
	body, err := json.Marshal(plan)
	require.NoError(t, err)

	source := RootBaseSource{Repository: "https://forge.example/group/source", Commit: strings.Repeat("b", 40), Workflow: "bases.yml"}
	in := RootBasePayloadAdmission{Plan: RootBasePlanAdmission{
		Plan: body, PlanSHA256: admissionHash(body), ImageRepository: repository, Source: source, Expected: source,
	}, EvidenceRoot: t.TempDir()}
	_, entry, err := admitRootBasePublication(PublishRootBaseInput{Plan: plan, ExpectedRepository: repository})
	require.NoError(t, err)

	entry.Digest = "sha256:" + strings.Repeat("d", 64)
	entry.Ref = repository + "@" + entry.Digest
	sbom := []byte(`{"bomFormat":"CycloneDX"}`)
	entry.SBOMSHA256 = admissionHash(sbom)
	require.NoError(t, os.MkdirAll(filepath.Join(in.EvidenceRoot, filepath.Dir(entry.SBOM)), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(in.EvidenceRoot, entry.SBOM), sbom, 0o600))
	writeAdmissionLedger(t, in.EvidenceRoot, []imageledger.Entry{entry})
	refreshAdmissionDigest(t, &in)

	return in, plan, entry
}

func writeAdmissionLedger(t *testing.T, root string, entries []imageledger.Entry) {
	t.Helper()

	body, err := imageledger.Marshal(entries)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dist"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dist/base-images.json"), body, 0o600))
}

func refreshAdmissionDigest(t *testing.T, in *RootBasePayloadAdmission) {
	t.Helper()

	var err error

	in.PayloadDigest, err = apprelease.CanonicalPayloadDigest(in.EvidenceRoot)
	require.NoError(t, err)
}

func TestRootAdmissionAcceptsDistinctApprovedContentRevision(t *testing.T) {
	t.Parallel()
	in, original, _ := rootAdmissionFixture(t)
	plan, err := AdmitRootBasePlan(in.Plan)
	require.NoError(t, err)
	require.Equal(t, original, plan)
	require.NotEqual(t, in.Plan.Source.Commit, plan.Images[0].Revision)
	require.NotEqual(t, in.Plan.Source.Commit, plan.Images[0].BaseInputID)
	require.NoError(t, AdmitRootBasePayload(t.Context(), in))
}

func TestRootAdmissionPreservesApprovedSourceFallback(t *testing.T) {
	t.Parallel()
	in, plan, _ := rootAdmissionFixture(t)
	plan.Images[0].Revision, plan.Builds[0].Revision = "", ""
	body, err := json.Marshal(plan)
	require.NoError(t, err)

	in.Plan.Plan, in.Plan.PlanSHA256 = body, admissionHash(body)
	require.NoError(t, AdmitRootBasePayload(t.Context(), in))
}

func TestRootAdmissionCancellationPrecedesPayloadReads(t *testing.T) {
	t.Parallel()
	in, _, _ := rootAdmissionFixture(t)
	in.EvidenceRoot = filepath.Join(t.TempDir(), "absent")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, AdmitRootBasePayload(ctx, in), context.Canceled)
}

func TestRootAdmissionRejectsPlanAndSourceChanges(t *testing.T) {
	t.Parallel()

	for _, defect := range []string{"source commit", "source repo", "workflow", "missing approval", "bad approval", "plan bytes", "effective image revision", "effective build revision", "effective coherent revision", "parent", "warm", "mixed", "prepared parent"} {
		t.Run(defect, func(t *testing.T) {
			t.Parallel()
			in, plan, _ := rootAdmissionFixture(t)

			switch defect {
			case "source commit":
				in.Plan.Source.Commit = strings.Repeat("e", 40)
			case "source repo":
				in.Plan.Source.Repository = "https://forge.example/other/source"
			case "workflow":
				in.Plan.Source.Workflow = "other.yml"
			case "missing approval":
				in.Plan.PlanSHA256 = ""
			case "bad approval":
				in.Plan.Expected.Workflow = "../bases.yml"
			case "plan bytes":
				in.Plan.Plan = append(in.Plan.Plan, '\n')
			case "effective image revision":
				plan.Images[0].Revision = strings.Repeat("e", 64)
			case "effective build revision":
				plan.Builds[0].Revision = strings.Repeat("e", 64)
			case "effective coherent revision":
				plan.Images[0].Revision, plan.Builds[0].Revision = strings.Repeat("e", 64), strings.Repeat("e", 64)
			case "parent":
				plan.Images[0].BaseRef = "registry.example/group/parent@sha256:" + strings.Repeat("e", 64)
			case "warm":
				plan.Images[0].CacheTagPrefix = "cache-root"
			case "mixed":
				plan.Images = append(plan.Images, plan.Images[0])
			case "prepared parent":
				plan.Builds[0].RegistryInputs = map[string]string{"BASE": "registry.example/group/parent@sha256:" + strings.Repeat("e", 64)}
			}

			if strings.HasPrefix(defect, "effective") || defect == "parent" || defect == "warm" || defect == "mixed" || defect == "prepared parent" {
				body, err := json.Marshal(plan)
				require.NoError(t, err)

				in.Plan.Plan = body
				// Unsupported plans fail even if their exact bytes were approved.
				if !strings.HasPrefix(defect, "effective") {
					in.Plan.PlanSHA256 = admissionHash(body)
				}
			}

			_, err := AdmitRootBasePlan(in.Plan)
			require.Error(t, err)
			// Binding failures precede even reading an evidence directory.
			in.EvidenceRoot = filepath.Join(t.TempDir(), "absent")
			require.EqualError(t, AdmitRootBasePayload(t.Context(), in), err.Error())
		})
	}
}

func TestRootAdmissionRejectsPayloadAndLedgerChanges(t *testing.T) {
	t.Parallel()

	for _, defect := range []string{"payload digest", "SBOM bytes", "missing pin", "missing BOM", "symlink BOM", "BOM path", "nonobject BOM", "null BOM", "own ID", "provenance", "parent", "multiple", "mixed dist", "missing root", "root symlink"} {
		t.Run(defect, func(t *testing.T) {
			t.Parallel()
			in, _, entry := rootAdmissionFixture(t)
			sbom := filepath.Join(in.EvidenceRoot, entry.SBOM)

			switch defect {
			case "payload digest":
				in.PayloadDigest = strings.Repeat("f", 64)
			case "SBOM bytes":
				require.NoError(t, os.WriteFile(sbom, []byte(`{"changed":true}`), 0o600))
			case "missing pin":
				entry.SBOMSHA256 = ""
			case "missing BOM":
				require.NoError(t, os.Remove(sbom))
			case "symlink BOM":
				require.NoError(t, os.Remove(sbom))
				require.NoError(t, os.Symlink("/missing", sbom))
			case "BOM path":
				entry.SBOM = "../outside.cyclonedx.json"
			case "nonobject BOM", "null BOM":
				body := []byte(`[]`)
				if defect == "null BOM" {
					body = []byte(`null`)
				}

				require.NoError(t, os.WriteFile(sbom, body, 0o600))
				entry.SBOMSHA256 = admissionHash(body)
			case "own ID":
				entry.BaseInputID = strings.Repeat("f", 64)
				entry.FinalTag = in.Plan.ImageRepository + ":" + entry.BaseInputID + "-opaque"
				entry.CandidateTag = in.Plan.ImageRepository + ":staging-" + entry.BaseInputID + "-opaque"
			case "provenance":
				entry.Provenance = map[string]any{"unexpected": true}
			case "parent":
				entry.Provenance = map[string]any{"parent_base": nil}
			case "mixed dist":
				require.NoError(t, os.WriteFile(filepath.Join(in.EvidenceRoot, "dist/release-images.json"), []byte(`[]`), 0o600))
			case "missing root":
				in.EvidenceRoot = ""
			case "root symlink":
				alias := filepath.Join(t.TempDir(), "alias")
				require.NoError(t, os.Symlink(in.EvidenceRoot, alias))
				in.EvidenceRoot = alias
			}

			if defect != "missing root" {
				entries := []imageledger.Entry{entry}
				if defect == "multiple" {
					entries = append(entries, entry)
				}
				// Write raw JSON to exercise admission of invalid ledger fields too.
				body, err := json.Marshal(entries)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(in.EvidenceRoot, "dist/base-images.json"), body, 0o600))
			}

			if defect != "payload digest" && defect != "symlink BOM" && defect != "root symlink" && defect != "missing root" {
				// Rebinding the transport digest must not bypass semantic/pin checks.
				refreshAdmissionDigest(t, &in)
			}

			require.Error(t, AdmitRootBasePayload(t.Context(), in))
		})
	}
}
