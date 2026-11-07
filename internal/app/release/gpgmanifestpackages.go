// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package release

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/pathsafe"
)

// ValidateGPGPackageManifest preflights explicit manifest selection without
// reading secrets or invoking a signer. The CLI calls it before resolving keys;
// GPGSignPackages repeats it before keymaterial.Key for direct application callers.
// It is not source authorization or a replacement for closed payload admission.
func ValidateGPGPackageManifest(in FilesInput) error {
	_, err := manifestPackageFiles(in)

	return err
}

func manifestPackageFiles(in FilesInput) ([]string, error) {
	const apkExtension = ".apk"

	if err := validatePackageManifestInput(in); err != nil {
		return nil, err
	}

	file, err := openReleaseFile(in.ManifestFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	manifest, err := decodeStrictFileManifest(file)
	if err != nil {
		return nil, err
	}

	ctx := releaseFilesContext(in)
	if err := validateReleaseFileManifest(ctx, manifest); err != nil {
		return nil, err
	}

	var packages []string

	for _, entry := range manifest.Assets {
		if filepath.Clean(entry.Path) != entry.Path || strings.Contains(entry.Path, `\`) {
			return nil, fmt.Errorf("package manifest refuses path alias %q: %w", entry.Path, errs.ErrValidation)
		}

		switch filepath.Ext(entry.Path) {
		case ".deb", ".rpm", apkExtension:
			packages = append(packages, entry.Path)
		}
	}

	if err := validateUnsignedReleaseFiles(ctx, manifest); err != nil {
		return nil, err
	}

	if err := rejectPackageSignerPreseeds(ctx.distDir); err != nil {
		return nil, err
	}

	in.ManifestExplicit = true
	if err := ValidateReleaseChecksums(ValidateReleaseChecksumsInput{FilesInput: in}); err != nil {
		return nil, err
	}

	return packages, nil
}

func validatePackageManifestInput(in FilesInput) error {
	if in.ManifestFile == "" || strings.TrimSpace(in.ManifestFile) != in.ManifestFile {
		return fmt.Errorf("package selection requires an explicit manifest path: %w", errs.ErrUsage)
	}

	for _, path := range []string{in.ManifestFile, in.DistDir} {
		if path != "" && (!pathsafe.Relative(path) || filepath.Clean(path) != path || strings.Contains(path, `\`)) {
			return fmt.Errorf("package selection refuses path alias %q: %w", path, errs.ErrValidation)
		}
	}

	return nil
}

// Unlisted packages are ignored, never signed. Unlisted signer sidecars are
// refused too: the opt-in path is an unsigned workspace, not a stale-output
// cleanup operation. Legacy selection retains its existing cleanup behavior.
func rejectPackageSignerPreseeds(dir string) error {
	root, err := pathsafe.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		if !entry.Type().IsRegular() {
			return fmt.Errorf("package workspace refuses nonregular file %s: %w", path, errs.ErrValidation)
		}

		for _, signature := range releaseSignatures() {
			if strings.HasSuffix(path, signature.suffix) {
				return fmt.Errorf("package workspace refuses preexisting signer output %s: %w", path, errs.ErrValidation)
			}
		}

		return nil
	})
}
