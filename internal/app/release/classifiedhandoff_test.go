// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package release_test

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apprelease "github.com/diggsweden/forge-ci/internal/app/release"
	"github.com/stretchr/testify/require"
)

func classifiedFileFixture(t *testing.T) *apprelease.FileManifest {
	t.Helper()
	t.Chdir(t.TempDir())
	writeReleaseFilesTestFile(t, "dist/product-1.2.3-linux-aarch64.tar.gz", "archive bytes\x00\xff\n")
	writeReleaseFilesTestFile(t, "dist/product_1.2.3_arm64.deb", "package bytes\n")
	// Deliberately noncanonical JSON: handoff must preserve actual producer bytes.
	writeReleaseFilesTestFile(t, "dist/product.cdx.json", "{ \"serialNumber\" : \"urn:uuid:producer-pinned\", \"bomFormat\" : \"CycloneDX\" }\n")
	writeReleaseFilesTestFile(t, "dist/product-plan.json", "{ \"release\" : \"1.2.3\" }\n")

	manifest, err := apprelease.DeclareReleaseFiles(apprelease.DeclareReleaseFilesInput{
		Assets: []string{"dist/product-1.2.3-linux-aarch64.tar.gz", "dist/product_1.2.3_arm64.deb"},
		SBOMs:  []string{"dist/product.cdx.json"}, EvidenceFiles: []string{"dist/product-plan.json"},
		ChecksumsFile: "dist/product_1.2.3_checksums.txt",
	})
	require.NoError(t, err)

	return manifest
}

func writeClassifiedManifest(t *testing.T, manifest *apprelease.FileManifest) {
	t.Helper()

	body, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	writeReleaseFilesTestFile(t, "dist/release-files.json", string(body)+"\n")
}

func classifiedFileInventory(t *testing.T, directory string) map[string]string {
	t.Helper()

	files := make(map[string]string)

	require.NoError(t, fs.WalkDir(os.DirFS(directory), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		body, err := os.ReadFile(filepath.Join(directory, path))
		files[path] = string(body)

		return err
	}))

	return files
}

func TestClassifiedFileHandoff_PreservesProducerInventoryAndBytes(t *testing.T) {
	classifiedFileFixture(t)
	before := classifiedFileInventory(t, ".")
	digest, err := apprelease.ClassifiedFilePayloadDigest(".")
	require.NoError(t, err)
	destination := filepath.Join(t.TempDir(), "accepted")
	require.NoError(t, apprelease.StageVerifiedClassifiedFilePayload(t.Context(), ".", destination, digest))
	require.Equal(t, before, classifiedFileInventory(t, destination))
	actual, err := apprelease.ClassifiedFilePayloadDigest(destination)
	require.NoError(t, err)
	require.Equal(t, digest, actual, "directory names must not change the handoff digest")
	// The new opt-in payload remains consumable by the existing native manifest
	// commands. No replacement assembly, SBOM ZIP or checksum name is introduced.
	t.Chdir(destination)
	require.NoError(t, apprelease.ValidateReleaseFileManifest(apprelease.ValidateReleaseFileManifestInput{Unsigned: true}))
	require.NoError(t, apprelease.ValidateReleaseChecksums(apprelease.ValidateReleaseChecksumsInput{}))
}

func TestClassifiedFileHandoff_RejectsInvalidProducerEvenWithMatchingDigest(t *testing.T) {
	for _, scenario := range []string{
		"missing-manifest", "missing-asset", "missing-checksums", "extra", "hidden-extra", "build-metadata", "empty-extra-directory",
		"asset-link", "parent-link", "outside-path", "noncanonical-path", "duplicate-asset", "overlapping-role", "different-role-source",
		"preseed-bundle", "preseed-package-sig", "preseed-provenance", "declared-provenance", "declared-signature",
		"image-ledger", "declared-image-ledger", "base-ledger", "premade-base-sbom",
		"wrong-checksum", "missing-checksum-subject", "extra-checksum-subject", "duplicate-checksum-subject", "unknown-manifest-field", "trailing-json",
	} {
		t.Run(scenario, func(t *testing.T) {
			manifest := classifiedFileFixture(t)

			switch scenario {
			case "missing-manifest":
				require.NoError(t, os.Remove("dist/release-files.json"))
			case "missing-asset":
				require.NoError(t, os.Remove(manifest.Assets[0].Path))
			case "missing-checksums":
				require.NoError(t, os.Remove(manifest.Checksums[0].Path))
			case "extra", "hidden-extra", "build-metadata", "image-ledger", "base-ledger", "premade-base-sbom":
				path := map[string]string{"extra": "dist/unlisted.deb", "hidden-extra": "dist/.unlisted", "build-metadata": "dist/artifacts.json", "image-ledger": "dist/release-images.json", "base-ledger": "dist/base-images.json", "premade-base-sbom": "premade-sboms/base-sbom-core.cyclonedx.json"}[scenario]
				writeReleaseFilesTestFile(t, path, "not admitted\n")
			case "empty-extra-directory":
				require.NoError(t, os.Mkdir("extra", 0o700))
			case "asset-link":
				require.NoError(t, os.Remove(manifest.Assets[0].Path))
				require.NoError(t, os.Symlink("product.cdx.json", manifest.Assets[0].Path))
			case "parent-link":
				require.NoError(t, os.Rename("dist", "real-dist"))
				require.NoError(t, os.Symlink("real-dist", "dist"))
			case "outside-path", "noncanonical-path":
				manifest.Assets[0].Path = "../outside.tar.gz"
				if scenario == "noncanonical-path" {
					manifest.Assets[0].Path = "dist/../dist/product-1.2.3-linux-aarch64.tar.gz"
				}

				writeClassifiedManifest(t, manifest)
			case "duplicate-asset":
				manifest.Assets = append(manifest.Assets, manifest.Assets[0])
				writeClassifiedManifest(t, manifest)
			case "overlapping-role":
				manifest.Evidence = append(manifest.Evidence, manifest.SBOMs[0])
				writeClassifiedManifest(t, manifest)
			case "different-role-source":
				manifest.SBOMs[0].Source = "not-the-producer-role"
				writeClassifiedManifest(t, manifest)
			case "preseed-bundle":
				writeReleaseFilesTestFile(t, "dist/product.cdx.json.bundle", "preseed")
			case "preseed-package-sig":
				writeReleaseFilesTestFile(t, "dist/product_1.2.3_arm64.deb.sig", "preseed")
			case "preseed-provenance":
				writeReleaseFilesTestFile(t, "dist/slsa-provenance.intoto.json", "preseed")
			case "declared-provenance", "declared-signature", "declared-image-ledger":
				name := map[string]string{"declared-provenance": "slsa-provenance.intoto.json", "declared-signature": "foreign.asc", "declared-image-ledger": "release-images.json"}[scenario]
				entry := apprelease.FileEntry{Path: "dist/" + name, Name: name, Source: "producer"}
				writeReleaseFilesTestFile(t, entry.Path, "preseed")
				manifest.Assets = append(manifest.Assets, entry)
				manifest.Evidence = append(manifest.Evidence, entry)
				writeClassifiedManifest(t, manifest)
			case "wrong-checksum":
				writeReleaseFilesTestFile(t, "dist/product.cdx.json", "mutated SBOM")
			case "missing-checksum-subject":
				writeReleaseFilesTestFile(t, manifest.Checksums[0].Path, "")
			case "extra-checksum-subject":
				writeReleaseFilesTestFile(t, manifest.Checksums[0].Path, readFile(t, manifest.Checksums[0].Path)+strings.Repeat("a", 64)+"  foreign.deb\n")
			case "duplicate-checksum-subject":
				body := readFile(t, manifest.Checksums[0].Path)
				writeReleaseFilesTestFile(t, manifest.Checksums[0].Path, body+body)
			case "unknown-manifest-field":
				body := readFile(t, "dist/release-files.json")
				writeReleaseFilesTestFile(t, "dist/release-files.json", strings.Replace(body, "{", `{ "images": [],`, 1))
			case "trailing-json":
				writeReleaseFilesTestFile(t, "dist/release-files.json", readFile(t, "dist/release-files.json")+"null\n")
			}

			_, err := apprelease.ClassifiedFilePayloadDigest(".")
			require.Error(t, err)
			// Matching a caller-supplied digest cannot waive structural admission.
			digest, err := apprelease.DistDigestWithManifestRoot(".", ".")
			if err != nil { // unsafe filesystem trees are refused before hashing too
				digest = strings.Repeat("a", 64)
			}

			destination := filepath.Join(t.TempDir(), "refused")
			require.Error(t, apprelease.StageVerifiedClassifiedFilePayload(t.Context(), ".", destination, digest))
			_, err = os.Lstat(destination)
			require.True(t, os.IsNotExist(err), "failed admission must install no destination")
		})
	}
}

func TestClassifiedFileHandoff_RefusesTamperExistingTargetAndCancellation(t *testing.T) {
	for _, scenario := range []string{"tamper", "existing", "cancelled", "invalid-digest"} {
		t.Run(scenario, func(t *testing.T) {
			classifiedFileFixture(t)

			digest, err := apprelease.ClassifiedFilePayloadDigest(".")
			require.NoError(t, err)
			destination := filepath.Join(t.TempDir(), "accepted")
			ctx := t.Context()

			switch scenario {
			case "tamper":
				// Evidence is deliberately excluded from package checksums, but
				// its pinned bytes are still covered by the handoff digest.
				writeReleaseFilesTestFile(t, "dist/product-plan.json", "mutated evidence")
			case "existing":
				require.NoError(t, os.Mkdir(destination, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(destination, "sentinel"), []byte("keep"), 0o600))
			case "cancelled":
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "invalid-digest":
				digest = ""
			}

			require.Error(t, apprelease.StageVerifiedClassifiedFilePayload(ctx, ".", destination, digest))

			if scenario == "existing" {
				require.Equal(t, map[string]string{"sentinel": "keep"}, classifiedFileInventory(t, destination))
			} else {
				_, err = os.Lstat(destination)
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}

type classifiedBundleSigner struct{ signed []string }

func (s *classifiedBundleSigner) Extensions() []string { return []string{".bundle"} }
func (s *classifiedBundleSigner) SignFile(_ context.Context, path string) error {
	s.signed = append(s.signed, path)

	return os.WriteFile(path+".bundle", []byte("inert test bundle, not crypto evidence"), 0o600)
}

func TestClassifiedFileHandoff_ComposesIndependentPackageGPGAndNativeFinalizer(t *testing.T) {
	before := classifiedFileFixture(t)
	bytesBefore := classifiedFileInventory(t, ".")
	digest, err := apprelease.ClassifiedFilePayloadDigest(".")
	require.NoError(t, err)
	destination := filepath.Join(t.TempDir(), "accepted")
	require.NoError(t, apprelease.StageVerifiedClassifiedFilePayload(t.Context(), ".", destination, digest))
	t.Chdir(destination)
	// Mock mechanisms only: prove role composition, not actual cryptography,
	// source authorization, GitHub acceptance or production signer isolation.
	gpg := &fakeGPGPackageSigner{listed: "fpr:::::::::ABCDEF:"}
	count, err := apprelease.GPGSignPackages(t.Context(), gpg, io.Discard, apprelease.GPGSignPackagesInput{
		PrivateKey: testPGPKeyArmor, Fingerprint: "ABCDEF", Passphrase: "secret", Dir: "dist",
	})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	bundles := &classifiedBundleSigner{}
	require.NoError(t, apprelease.SignArtifacts(t.Context(), bundles, io.Discard, apprelease.SignInput{
		ChecksumsFromManifest: true, ManifestSections: []string{"sboms", "evidence"}, SkipReleaseArtifactsDir: true,
	}))
	require.ElementsMatch(t, []string{"dist/product_1.2.3_checksums.txt", "dist/product.cdx.json", "dist/product-plan.json"}, bundles.signed)
	writeReleaseFilesTestFile(t, "dist/slsa-provenance.intoto.json", "inert signer-owned provenance")
	writeReleaseFilesTestFile(t, "dist/slsa-provenance.intoto.json.bundle", "inert provenance bundle")

	after, err := apprelease.WriteReleaseFileManifest(apprelease.WriteReleaseFileManifestInput{})
	require.NoError(t, err)
	require.Equal(t, before.Checksums, after.Checksums)
	require.Equal(t, before.SBOMs, after.SBOMs)
	require.Equal(t, before.Evidence, after.Evidence)
	assertReleaseAssetsInclude(t, after.Assets, "product_1.2.3_arm64.deb.sig", "product_1.2.3_checksums.txt.bundle", "product.cdx.json.bundle", "product-plan.json.bundle", "slsa-provenance.intoto.json", "slsa-provenance.intoto.json.bundle")
	require.Len(t, after.Assets, len(before.Assets)+6)

	for path, body := range bytesBefore {
		if path != "dist/release-files.json" {
			require.Equal(t, body, readFile(t, path), path)
		}
	}

	require.NoError(t, apprelease.ValidateReleaseChecksums(apprelease.ValidateReleaseChecksumsInput{}))
}
