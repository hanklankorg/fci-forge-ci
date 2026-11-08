// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package release_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apprelease "github.com/diggsweden/forge-ci/internal/app/release"
	releasecmd "github.com/diggsweden/forge-ci/internal/cli/commands/release"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/stretchr/testify/require"
)

func classifiedCLIWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("EXPECTED_DIGEST", "")
	t.Setenv("DIST_DIGEST", "")
	root := t.TempDir()
	t.Chdir(root)
	require.NoError(t, os.Mkdir("dist", 0o700))

	for name, body := range map[string]string{
		"product-v1.2.3-aarch64.tar.gz": "archive\x00\xff\n",
		"product_1.2.3_arm64.deb":       "package\n",
		"producer.cdx.json":             "{ \"serialNumber\" : \"producer-pinned\" }\n",
		"producer-plan.json":            "{ \"plan\" : 1 }\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join("dist", name), []byte(body), 0o600))
	}

	_, err := apprelease.DeclareReleaseFiles(apprelease.DeclareReleaseFilesInput{
		Assets:        []string{"dist/product-v1.2.3-aarch64.tar.gz", "dist/product_1.2.3_arm64.deb"},
		SBOMs:         []string{"dist/producer.cdx.json"},
		EvidenceFiles: []string{"dist/producer-plan.json"},
		ChecksumsFile: "dist/product_1.2.3_checksums.txt",
	})
	require.NoError(t, err)

	return root
}

func classifiedCLIBytes(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{}

	require.NoError(t, fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		body, err := os.ReadFile(filepath.Join(root, path))
		files[path] = string(body)

		return err
	}))

	return files
}

func TestClassifiedFilesCLIStagesExactProducerBytes(t *testing.T) {
	for _, digestMode := range []string{"inline", "file"} {
		t.Run(digestMode, func(t *testing.T) {
			source := classifiedCLIWorkspace(t)
			digest := capturePayloadDigest(t, "--classified-files", "--dist-dir", source)
			want, err := apprelease.ClassifiedFilePayloadDigest(source)
			require.NoError(t, err)
			require.Equal(t, want, digest)
			destination := filepath.Join(t.TempDir(), "admitted")
			args := []string{"release", "validate-dist", "--classified-files", "--dist-dir", source, "--stage-dir", destination}

			if digestMode == "file" {
				record := filepath.Join(t.TempDir(), "digest")
				require.NoError(t, os.WriteFile(record, []byte(digest+"\n"), 0o600))
				args = append(args, "--digest-file", record)
			} else {
				args = append(args, "--expected-digest", digest)
			}

			require.NoError(t, releasecmd.New().Run(t.Context(), args))
			require.Equal(t, classifiedCLIBytes(t, source), classifiedCLIBytes(t, destination), "manifest, names, roles, checksum and pinned SBOM bytes must remain exact")
			require.Equal(t, digest, capturePayloadDigest(t, "--classified-files", "--dist-dir", destination))
		})
	}
}

func TestClassifiedFilesCLIRefusesInvalidHandoffWithoutInstallation(t *testing.T) {
	for _, scenario := range []string{"extra", "tamper", "symlink", "preseed", "image-ledger", "base-root", "missing-manifest"} {
		t.Run(scenario, func(t *testing.T) {
			source := classifiedCLIWorkspace(t)
			digest := capturePayloadDigest(t, "--classified-files", "--dist-dir", source)

			switch scenario {
			case "extra":
				require.NoError(t, os.WriteFile("dist/unlisted", []byte("extra"), 0o600))
			case "tamper":
				// Evidence is outside product checksums, but inside the handoff digest.
				require.NoError(t, os.WriteFile("dist/producer-plan.json", []byte("changed"), 0o600))
			case "symlink":
				require.NoError(t, os.Symlink("producer.cdx.json", "dist/link"))
			case "preseed":
				require.NoError(t, os.WriteFile("dist/producer.cdx.json.bundle", []byte("preseed"), 0o600))
			case "image-ledger":
				require.NoError(t, os.WriteFile("dist/release-images.json", []byte("[]"), 0o600))
			case "base-root":
				require.NoError(t, os.Mkdir("premade-sboms", 0o700))
				require.NoError(t, os.WriteFile("premade-sboms/base.cdx.json", []byte("{}"), 0o600))
			case "missing-manifest":
				require.NoError(t, os.Remove("dist/release-files.json"))
			}

			if scenario != "tamper" && scenario != "symlink" {
				var err error

				digest, err = apprelease.DistDigestWithManifestRoot(source, ".")
				require.NoError(t, err, "matching generic digest must not replace classified admission")
			}

			destination := filepath.Join(t.TempDir(), "refused")
			err := releasecmd.New().Run(t.Context(), []string{"release", "validate-dist", "--classified-files", "--dist-dir", source, "--stage-dir", destination, "--expected-digest", digest})
			require.Error(t, err)
			_, err = os.Lstat(destination)
			require.True(t, os.IsNotExist(err))

			if scenario != "tamper" {
				require.Error(t, releasecmd.New().Run(t.Context(), []string{"release", "dist-digest", "--classified-files", "--dist-dir", source}))
			}
		})
	}
}

func TestClassifiedFilesCLIFlagsFailBeforeFilesystemAccess(t *testing.T) {
	t.Setenv("EXPECTED_DIGEST", "")
	t.Setenv("DIST_DIGEST", "")

	for _, verb := range []string{"dist-digest", "validate-dist"} {
		for _, modeFlags := range [][]string{{"--canonical-payload"}, {"--manifest-root", "."}, {"--manifest-root", ""}, {"--dist-dir", ""}} {
			args := append([]string{"release", verb, "--classified-files", "--dist-dir", "does-not-exist"}, modeFlags...)
			err := releasecmd.New().Run(t.Context(), args)
			require.ErrorIs(t, err, errs.ErrUsage)
			require.Contains(t, err.Error(), "--classified-files")
		}
	}

	for _, scenario := range []string{"no-stage", "empty-stage", "no-digest", "bad-digest", "two-digest-sources"} {
		t.Run(scenario, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "not-created", "destination")
			args := []string{"release", "validate-dist", "--classified-files", "--dist-dir", "does-not-exist"}

			switch scenario {
			case "no-stage":
				args = append(args, "--digest-file", "must-not-be-read")
			case "empty-stage":
				args = append(args, "--stage-dir", "", "--digest-file", "must-not-be-read")
			case "no-digest":
				args = append(args, "--stage-dir", destination)
			case "bad-digest":
				args = append(args, "--stage-dir", destination, "--expected-digest", "not-a-digest")
			case "two-digest-sources":
				args = append(args, "--stage-dir", destination, "--expected-digest", strings.Repeat("a", 64), "--digest-file", "must-not-be-read")
			}

			err := releasecmd.New().Run(t.Context(), args)
			require.ErrorIs(t, err, errs.ErrUsage)
			require.NotContains(t, err.Error(), "must-not-be-read")
			_, err = os.Lstat(filepath.Dir(destination))
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestClassifiedFilesCLIFreshStageAndLegacyCompatibility(t *testing.T) {
	source := classifiedCLIWorkspace(t)
	digest := capturePayloadDigest(t, "--classified-files", "--dist-dir", source)
	destination := t.TempDir()
	sentinel := filepath.Join(destination, "keep")
	require.NoError(t, os.WriteFile(sentinel, []byte("existing bytes"), 0o600))
	err := releasecmd.New().Run(t.Context(), []string{"release", "validate-dist", "--classified-files", "--dist-dir", source, "--stage-dir", destination, "--expected-digest", digest})
	require.ErrorIs(t, err, errs.ErrValidation)
	require.Equal(t, map[string]string{"keep": "existing bytes"}, classifiedCLIBytes(t, destination))

	legacy, err := apprelease.DistDigest(source)
	require.NoError(t, err)
	require.Equal(t, legacy, capturePayloadDigest(t, "--dist-dir", source))
	require.Equal(t, legacy, capturePayloadDigest(t, "--classified-files=false", "--dist-dir", source))
	require.NoError(t, releasecmd.New().Run(t.Context(), []string{"release", "validate-dist", "--dist-dir", source, "--expected-digest", legacy}))
	require.NotEqual(t, legacy, digest, "classified mode alone fixes the '.' prefix")
}
