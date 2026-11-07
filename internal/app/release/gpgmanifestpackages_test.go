// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package release_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apprelease "github.com/diggsweden/forge-ci/internal/app/release"
	"github.com/stretchr/testify/require"
)

func manifestPackageFixture(t *testing.T) (apprelease.GPGSignPackagesInput, *apprelease.FileManifest) {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll("dist/packages", 0o700))

	manifest := &apprelease.FileManifest{Version: 1}

	var sums strings.Builder

	for _, path := range []string{"dist/packages/exact-name.deb", "dist/packages/other.rpm", "dist/alpine.apk", "dist/archive.tgz", "dist/native.cdx.json"} {
		body := []byte(path)
		require.NoError(t, os.WriteFile(path, body, 0o600))
		entry := apprelease.FileEntry{Path: path, Name: filepath.Base(path), Source: "producer"}

		manifest.Assets = append(manifest.Assets, entry)
		if strings.HasSuffix(path, ".json") {
			manifest.SBOMs = append(manifest.SBOMs, entry)
		}

		_, err := fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(body), entry.Name)
		require.NoError(t, err)
	}

	checksum := apprelease.FileEntry{Path: "dist/pinned-checksums.txt", Name: "pinned-checksums.txt", Source: "checksums"}
	manifest.Assets = append(manifest.Assets, checksum)
	manifest.Checksums = []apprelease.FileEntry{checksum}
	require.NoError(t, os.WriteFile(checksum.Path, []byte(sums.String()), 0o600))
	writePackageManifest(t, manifest)

	return apprelease.GPGSignPackagesInput{
		Dir: "dist", ManifestFile: "dist/release-files.json", PrivateKey: testPGPKeyArmor, Fingerprint: "ABCDEF", Passphrase: "secret",
	}, manifest
}

func writePackageManifest(t *testing.T, manifest *apprelease.FileManifest) {
	t.Helper()

	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("dist/release-files.json", body, 0o600))
}

func TestGPGManifestPackagesExactSelectionAndUnchangedInventory(t *testing.T) {
	in, _ := manifestPackageFixture(t)
	require.NoError(t, os.WriteFile("dist/stray.deb", []byte("unlisted"), 0o600))
	require.NoError(t, os.WriteFile("dist/packages/stray.rpm", []byte("unlisted"), 0o600))

	before := make(map[string][]byte)

	for _, path := range []string{in.ManifestFile, "dist/pinned-checksums.txt", "dist/native.cdx.json"} {
		body, err := os.ReadFile(path)
		require.NoError(t, err)

		before[path] = body
	}

	signer := &fakeGPGPackageSigner{listed: "fpr:::::::::ABCDEF:"}
	count, err := apprelease.GPGSignPackages(t.Context(), signer, io.Discard, in)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, []string{"dist/packages/exact-name.deb", "dist/packages/other.rpm", "dist/alpine.apk"}, signer.signed)

	for path, body := range before {
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, body, after)
	}

	for _, path := range []string{"dist/stray.deb.sig", "dist/packages/stray.rpm.sig", "dist/archive.tgz.sig", "dist/native.cdx.json.sig"} {
		_, err := os.Lstat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestGPGManifestPackagesEntireSelectionBeforeKeyEffects(t *testing.T) {
	for _, scenario := range []string{"missing", "wrong-hash", "duplicate", "alias", "traversal", "symlink", "parent-symlink", "directory", "manifest-link", "manifest-unknown", "manifest-trailing", "missing-checksums", "preseed", "stray-sidecar", "sidecar-link"} {
		t.Run(scenario, func(t *testing.T) {
			in, manifest := manifestPackageFixture(t)
			// The bad entry follows valid packages: no partial signing/import.
			last := &manifest.Assets[4]

			switch scenario {
			case "missing":
				require.NoError(t, os.Remove(last.Path))
			case "wrong-hash":
				require.NoError(t, os.WriteFile(last.Path, []byte("tampered"), 0o600))
			case "duplicate":
				manifest.Assets = append(manifest.Assets, manifest.Assets[0])
			case "alias":
				manifest.Assets[0].Path = "dist/packages/./exact-name.deb"
			case "traversal":
				last.Path = "dist/../native.cdx.json"
			case "symlink":
				require.NoError(t, os.Remove(last.Path))
				require.NoError(t, os.Symlink("archive.tgz", last.Path))
			case "parent-symlink":
				require.NoError(t, os.Rename("dist/packages", "dist/real-packages"))
				require.NoError(t, os.Symlink("real-packages", "dist/packages"))
			case "directory":
				require.NoError(t, os.Remove(last.Path))
				require.NoError(t, os.Mkdir(last.Path, 0o700))
			case "missing-checksums":
				require.NoError(t, os.Remove("dist/pinned-checksums.txt"))
			case "preseed":
				require.NoError(t, os.WriteFile(manifest.Assets[1].Path+".sig", []byte("existing"), 0o600))
			case "stray-sidecar":
				require.NoError(t, os.WriteFile("dist/stray.rpm.sig", []byte("existing"), 0o600))
			case "sidecar-link":
				require.NoError(t, os.Symlink("archive.tgz", "dist/stray.rpm.sig"))
			}

			writePackageManifest(t, manifest)

			switch scenario {
			case "manifest-link":
				require.NoError(t, os.Rename(in.ManifestFile, "dist/real-manifest.json"))
				require.NoError(t, os.Symlink("real-manifest.json", in.ManifestFile))
			case "manifest-unknown":
				body, err := os.ReadFile(in.ManifestFile)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(in.ManifestFile, append([]byte(`{"unknown":true,`), body[1:]...), 0o600))
			case "manifest-trailing":
				body, err := os.ReadFile(in.ManifestFile)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(in.ManifestFile, append(body, []byte(" null")...), 0o600))
			}

			signer := &fakeGPGPackageSigner{listed: "fpr:::::::::ABCDEF:"}
			in.PrivateKey = "not-valid-base64-key-material"
			count, err := apprelease.GPGSignPackages(t.Context(), signer, io.Discard, in)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "signing material", "selection must fail before key decoding")
			require.Zero(t, count)
			require.Zero(t, signer.imports)
			require.Zero(t, signer.lists)
			require.Empty(t, signer.signed)

			_, statErr := os.Lstat("dist/packages/exact-name.deb.sig")
			require.ErrorIs(t, statErr, os.ErrNotExist)

			if scenario == "preseed" {
				body, readErr := os.ReadFile(manifest.Assets[1].Path + ".sig")
				require.NoError(t, readErr)
				require.Equal(t, "existing", string(body))
			}
		})
	}
}

func TestGPGManifestPackagesFingerprintStillRequired(t *testing.T) {
	in, _ := manifestPackageFixture(t)
	signer := &fakeGPGPackageSigner{listed: "fpr:::::::::OTHER:"}
	_, err := apprelease.GPGSignPackages(t.Context(), signer, io.Discard, in)
	require.ErrorContains(t, err, "fingerprint does not match")
	require.Equal(t, 1, signer.imports)
	require.Empty(t, signer.signed)
}

func TestGPGManifestPackagesComposeIndependentBundleRoleAndFinalizer(t *testing.T) {
	in, before := manifestPackageFixture(t)
	_, err := apprelease.GPGSignPackages(t.Context(), &fakeGPGPackageSigner{listed: "fpr:::::::::ABCDEF:"}, io.Discard, in)
	require.NoError(t, err)

	// Inert mechanisms prove independent selection/lifecycle, not crypto validity.
	bundles := &classifiedBundleSigner{}
	require.NoError(t, apprelease.SignArtifacts(t.Context(), bundles, io.Discard, apprelease.SignInput{
		ChecksumsFromManifest: true, ManifestSections: []string{"sboms"}, SkipReleaseArtifactsDir: true,
	}))
	require.ElementsMatch(t, []string{"dist/pinned-checksums.txt", "dist/native.cdx.json"}, bundles.signed)
	require.NoError(t, os.WriteFile("dist/slsa-provenance.intoto.json", []byte("inert provenance"), 0o600))
	require.NoError(t, os.WriteFile("dist/slsa-provenance.intoto.json.bundle", []byte("inert bundle"), 0o600))

	after, err := apprelease.WriteReleaseFileManifest(apprelease.WriteReleaseFileManifestInput{})
	require.NoError(t, err)
	require.Equal(t, before.SBOMs, after.SBOMs)
	require.Equal(t, before.Checksums, after.Checksums)

	for _, path := range []string{"dist/packages/exact-name.deb.sig", "dist/packages/other.rpm.sig", "dist/alpine.apk.sig"} {
		require.Contains(t, after.Assets, apprelease.FileEntry{Path: path, Name: filepath.Base(path), Source: "package_signature"})
	}

	require.NoError(t, apprelease.ValidateReleaseChecksums(apprelease.ValidateReleaseChecksumsInput{}))
}
