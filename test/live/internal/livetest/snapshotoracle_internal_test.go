// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
	"github.com/diggsweden/forge-ci/internal/testutil/reporoot"
)

// The evidence oracle is judged here on trees shaped as the snapshot
// publisher retains them; no run produced them, so this is no native
// evidence of SBOM generation.
func TestSnapshotEvidenceFindings(t *testing.T) {
	t.Parallel()

	const (
		scan     = `{"SchemaVersion":2,"ArtifactName":"registry.invalid/service/snapshot:1"}`
		document = `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1,"components":[]}`
	)

	plan := snapshotimage.Plan{Images: []snapshotimage.Image{
		{Name: "service", Platforms: []string{"linux/amd64"}, Scan: true, SBOM: true},
		{Name: "worker", Platforms: []string{"linux/amd64"}, Scan: true},
	}}

	receipts := make([]snapshotimage.Receipt, 0, len(plan.Images))
	for i, image := range plan.Images {
		digest := "sha256:" + strings.Repeat(string(rune('a'+i)), 64)
		receipts = append(receipts, snapshotimage.Receipt{
			Name: image.Name, ImageRepository: "registry.invalid/service/snapshot", Tag: "1-" + image.Name, Digest: digest, Ref: "registry.invalid/service/snapshot@" + digest,
		})
	}

	record := func(receipt snapshotimage.Receipt) []byte {
		return fmt.Appendf(nil, `{"tag":%q,"digest":%q,"ref":%q}`, receipt.ImageRepository+":"+receipt.Tag, receipt.Digest, receipt.Ref)
	}

	// kept is what a run that kept the plan's policy retains: the SBOM of
	// service, which the plan requires, and none of worker, which it forbids.
	kept := func() map[string]map[string][]byte {
		return map[string]map[string][]byte{
			"service": {
				"service-dist/service.json":               record(receipts[0]),
				"service/linux-amd64/trivy.json":          []byte(scan),
				"service/linux-amd64/sbom.cyclonedx.json": []byte(document),
			},
			"worker": {
				"worker-dist/worker.json":       record(receipts[1]),
				"worker/linux-amd64/trivy.json": []byte(scan),
			},
		}
	}

	for _, tc := range []struct {
		name   string
		mutate func(evidence map[string]map[string][]byte)
		want   []string
	}{
		{name: "required present, forbidden absent with its scan", mutate: func(map[string]map[string][]byte) {}},
		{name: "derived scan reports", mutate: func(evidence map[string]map[string][]byte) {
			evidence["worker"]["worker/linux-amd64/trivy.sarif"] = []byte(`{}`)
			evidence["worker"]["worker/linux-amd64/gitlab.json"] = []byte(`{}`)
		}},
		{name: "required and missing", mutate: func(evidence map[string]map[string][]byte) {
			delete(evidence["service"], "service/linux-amd64/sbom.cyclonedx.json")
		}, want: []string{"image service (linux/amd64) retained no SBOM, which its plan requires"}},
		{name: "forbidden but present", mutate: func(evidence map[string]map[string][]byte) {
			evidence["worker"]["worker/linux-amd64/sbom.cyclonedx.json"] = []byte(document)
		}, want: []string{"image worker (linux/amd64) retained an SBOM its plan forbids"}},
		{name: "forbidden under another name", mutate: func(evidence map[string]map[string][]byte) {
			evidence["worker"]["worker/linux-amd64/inventory.cyclonedx.json"] = []byte(document)
		}, want: []string{"image worker retained unplanned evidence worker/linux-amd64/inventory.cyclonedx.json"}},
		{name: "forbidden absent without its scan", mutate: func(evidence map[string]map[string][]byte) {
			delete(evidence["worker"], "worker/linux-amd64/trivy.json")
		}, want: []string{"image worker (linux/amd64) retained no Trivy report"}},
		{name: "another image's SBOM", mutate: func(evidence map[string]map[string][]byte) {
			evidence["service"]["worker/linux-amd64/sbom.cyclonedx.json"] = evidence["service"]["service/linux-amd64/sbom.cyclonedx.json"]
			delete(evidence["service"], "service/linux-amd64/sbom.cyclonedx.json")
		}, want: []string{
			"image service (linux/amd64) retained no SBOM, which its plan requires",
			"image service retained unplanned evidence worker/linux-amd64/sbom.cyclonedx.json",
		}},
		{name: "unplanned platform", mutate: func(evidence map[string]map[string][]byte) {
			evidence["service"]["service/linux-arm64/sbom.cyclonedx.json"] = []byte(document)
		}, want: []string{"image service retained unplanned evidence service/linux-arm64/sbom.cyclonedx.json"}},
		{name: "bound to another digest", mutate: func(evidence map[string]map[string][]byte) {
			evidence["service"]["service-dist/service.json"] = record(receipts[1])
		}, want: []string{"the evidence of image service is not bound to its receipt: service-dist/service.json records {Tag:registry.invalid/service/snapshot:1-worker Digest:" + receipts[1].Digest + " Ref:" + receipts[1].Ref + "}"}},
		{name: "bound to a bare tag", mutate: func(evidence map[string]map[string][]byte) {
			evidence["service"]["service-dist/service.json"] = fmt.Appendf(nil, `{"tag":%q,"digest":%q,"ref":%q}`, receipts[0].Tag, receipts[0].Digest, receipts[0].Ref)
		}, want: []string{"the evidence of image service is not bound to its receipt: service-dist/service.json records {Tag:1-service Digest:" + receipts[0].Digest + " Ref:" + receipts[0].Ref + "}"}},
		{name: "unbound", mutate: func(evidence map[string]map[string][]byte) {
			delete(evidence["service"], "service-dist/service.json")
		}, want: []string{"the evidence of image service is not bound to its receipt: service-dist/service.json records {Tag: Digest: Ref:}"}},
		{name: "malformed JSON", mutate: func(evidence map[string]map[string][]byte) {
			evidence["service"]["service/linux-amd64/sbom.cyclonedx.json"] = []byte(document[:len(document)-1])
		}, want: []string{"image service (linux/amd64) retained an SBOM that is not a CycloneDX document"}},
		{name: "another format", mutate: func(evidence map[string]map[string][]byte) {
			evidence["service"]["service/linux-amd64/sbom.cyclonedx.json"] = []byte(`{"spdxVersion":"SPDX-2.3"}`)
		}, want: []string{"image service (linux/amd64) retained an SBOM that is not a CycloneDX document"}},
		{name: "missing evidence artifact", mutate: func(evidence map[string]map[string][]byte) {
			delete(evidence, "worker")
		}, want: []string{"image worker retained no evidence artifact"}},
		{name: "unexpected evidence artifact", mutate: func(evidence map[string]map[string][]byte) {
			evidence["extra"] = map[string][]byte{"extra/linux-amd64/sbom.cyclonedx.json": []byte(document)}
		}, want: []string{"evidence artifact of unplanned image extra"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			evidence := kept()
			tc.mutate(evidence)

			if got := SnapshotEvidenceFindings(plan, receipts, evidence); strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("findings %q, want %q", got, tc.want)
			}
		})
	}

	// A receipt the collection did not return binds nothing.
	if got := SnapshotEvidenceFindings(plan, receipts[:1], kept()); len(got) != 1 || !strings.HasPrefix(got[0], "the evidence of image worker is not bound to its receipt") {
		t.Fatalf("findings %q, want the unbound worker alone", got)
	}
}

// The oracle on the plans of the shipped declarations, one per cell of
// snapshot route and image SBOM policy. Evidence shaped as a run that kept a
// plan's policy has no finding; the opposite of it, an SBOM where the
// declaration forbids one or none where it requires one, has the finding the
// policy names. No run produced these trees: which cell a native run keeps
// is for that run to show.
func TestSnapshotEvidenceFindingsOfTheDemoPolicies(t *testing.T) {
	t.Parallel()

	const (
		scan     = `{"SchemaVersion":2}`
		document = `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1}`
		digest   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)

	for name, cell := range map[string]struct {
		declaration string
		producers   int
		sbom        bool
	}{
		"artifact-first with":     {"hello-maven-service/consumer/" + GitLabDefaultArtifactsConfig, 1, true},
		"artifact-first without":  {"hello-maven-service/consumer/" + mavenServiceBuildSBOMDeclaration, 1, false},
		"container-first with":    {"hello-go-service/consumer/" + GitLabDefaultArtifactsConfig, 0, true},
		"container-first without": {"hello-cargo-container/consumer/" + GitLabDedicatedReleaseImagesConfig, 0, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, plan, err := serviceSnapshotDeclarationOverlay(string(reporoot.ReadFile(t, "test/live/fixtures/consumers/"+cell.declaration)))
			if err != nil {
				t.Fatal(err)
			}

			if len(plan.Images) != 1 || len(plan.Artifacts) != cell.producers || plan.Images[0].SBOM != cell.sbom {
				t.Fatalf("%s plans %d producers and image SBOM %t, want %d and %t", cell.declaration, len(plan.Artifacts), plan.Images[0].SBOM, cell.producers, cell.sbom)
			}

			image := plan.Images[0]
			receipts := []snapshotimage.Receipt{{Name: image.Name, ImageRepository: image.ImageRepository, Tag: image.Tag, Digest: digest, Ref: image.ImageRepository + "@" + digest}}
			sbom := image.Name + "/linux-amd64/sbom.cyclonedx.json"

			evidence := map[string]map[string][]byte{image.Name: {
				image.Name + "-dist/" + image.Name + ".json": fmt.Appendf(nil, `{"tag":%q,"digest":%q,"ref":%q}`, image.ImageRepository+":"+image.Tag, receipts[0].Digest, receipts[0].Ref),
				image.Name + "/linux-amd64/trivy.json":       []byte(scan),
			}}
			if cell.sbom {
				evidence[image.Name][sbom] = []byte(document)
			}

			if got := SnapshotEvidenceFindings(plan, receipts, evidence); len(got) != 0 {
				t.Fatalf("a run that kept the policy has findings %q", got)
			}

			want := "image " + image.Name + " (linux/amd64) retained an SBOM its plan forbids"
			if cell.sbom {
				delete(evidence[image.Name], sbom)

				want = "image " + image.Name + " (linux/amd64) retained no SBOM, which its plan requires"
			} else {
				evidence[image.Name][sbom] = []byte(document)
			}

			if got := SnapshotEvidenceFindings(plan, receipts, evidence); len(got) != 1 || got[0] != want {
				t.Fatalf("findings %q, want %q", got, want)
			}
		})
	}
}
