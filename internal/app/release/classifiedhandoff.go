// SPDX-FileCopyrightText: The Forge CI Authors
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strings"

	domainartifact "github.com/diggsweden/forge-ci/internal/domain/artifact"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/imageledger"
	"github.com/diggsweden/forge-ci/internal/domain/provenance"
	domainrelease "github.com/diggsweden/forge-ci/internal/domain/release"
	"github.com/diggsweden/forge-ci/internal/pathsafe"
)

// ClassifiedFilePayloadDigest admits a closed, unsigned file-only payload and
// hashes it using the existing stable '.' handoff digest. The payload contains
// dist/release-files.json and exactly the files its Assets section names. The
// producer projects its inventory into this directory explicitly; build-tool
// metadata and image/base ledgers must not enter it. No file is regenerated.
//
// This is integrity admission, NOT source or artifact authorization. A future
// trusted caller must separately authenticate producer repository, commit, run,
// job result and immutable artifact ID before admitting downloaded bytes.
func ClassifiedFilePayloadDigest(directory string) (string, error) {
	root, err := pathsafe.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()

	if err := admitUnsignedClassifiedFiles(root.FS()); err != nil {
		return "", err
	}

	return domainrelease.DistDigest(root.FS(), ".")
}

// StageVerifiedClassifiedFilePayload installs a new private workspace only
// after both closed-set admission and the expected digest hold on the staged
// bytes. It retains the producer's manifest, paths, classifications, SBOM and
// checksum bytes verbatim. It loads no keys, logs into no registry, executes no
// producer code and makes no authorization claim about expectedDigest.
func StageVerifiedClassifiedFilePayload(ctx context.Context, source, destination, expectedDigest string) error {
	if !distDigestPattern.MatchString(expectedDigest) {
		return fmt.Errorf("classified handoff requires a lowercase SHA-256: %w", errs.ErrUsage)
	}

	return stageVerifiedDist(ctx, source, destination, expectedDigest, ".", admitUnsignedClassifiedFiles)
}

func admitUnsignedClassifiedFiles(payload fs.FS) error {
	if err := domainrelease.WalkSafe(payload); err != nil {
		return err
	}

	manifest, err := readClassifiedPayloadManifest(payload)
	if err != nil {
		return err
	}

	ctx := releaseFilesCtx{distDir: defaultReleaseDistDir, manifestFile: DefaultReleaseFilesManifest, manifestExplicit: true, payload: payload}
	if err := validateReleaseFileManifest(ctx, manifest); err != nil {
		return err
	}

	if err := validateUnsignedReleaseFiles(ctx, manifest); err != nil {
		return err
	}

	if err := validateClassifiedFileOnlyRoles(manifest); err != nil {
		return err
	}

	if err := validateClassifiedPayloadMembership(payload, manifest); err != nil {
		return err
	}

	return validateClassifiedPayloadChecksums(payload, manifest)
}

func readClassifiedPayloadManifest(payload fs.FS) (*FileManifest, error) {
	file, err := payload.Open(DefaultReleaseFilesManifest)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	// Metadata is bounded independently of potentially large product packages.
	const maxManifestBytes = 4 << 20

	body, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil {
		return nil, err
	}

	if len(body) > maxManifestBytes {
		return nil, fmt.Errorf("classified manifest exceeds byte limit: %w", errs.ErrValidation)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	var manifest FileManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("classified manifest: %w", err)
	}

	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("classified manifest must contain one JSON value: %w", errs.ErrValidation)
	}

	return &manifest, nil
}

func validateClassifiedFileOnlyRoles(manifest *FileManifest) error {
	assets := make(map[string]FileEntry, len(manifest.Assets))
	for _, entry := range manifest.Assets {
		assets[entry.Path] = entry
		// These names are lifecycle control files, never file-only subjects.
		switch entry.Name {
		case "release-files.json", imageledger.DefaultBasename, "base-images.json", "release-images-publication.json", "base-images-publication.json", ProvenanceStatementName:
			return fmt.Errorf("file-only payload refuses lifecycle file %s: %w", entry.Path, errs.ErrValidation)
		}

		if strings.HasSuffix(entry.Name, ".sig") || strings.HasSuffix(entry.Name, ".bundle") || strings.HasSuffix(entry.Name, ".asc") {
			return fmt.Errorf("unsigned file-only payload refuses signature %s: %w", entry.Path, errs.ErrValidation)
		}
	}

	classified := make(map[string]bool)

	for _, section := range [][]FileEntry{manifest.Checksums, manifest.SBOMs, manifest.Evidence} {
		for _, entry := range section {
			if classified[entry.Path] || assets[entry.Path] != entry {
				return fmt.Errorf("classified role must match one asset and cannot overlap another role: %s: %w", entry.Path, errs.ErrValidation)
			}

			classified[entry.Path] = true
		}
	}

	return nil
}

func validateClassifiedPayloadMembership(payload fs.FS, manifest *FileManifest) error {
	allowed := stringSet(entryPaths(manifest.Assets))
	allowed[DefaultReleaseFilesManifest] = struct{}{}

	var files int

	var total int64

	return fs.WalkDir(payload, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if path == "." || path == defaultReleaseDistDir {
				return nil
			}

			for file := range allowed {
				if strings.HasPrefix(file, path+"/") {
					return nil
				}
			}

			return fmt.Errorf("undeclared payload directory %s: %w", path, errs.ErrValidation)
		}

		if _, ok := allowed[path]; !ok {
			return fmt.Errorf("undeclared payload file %s: %w", path, errs.ErrValidation)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		files++

		total += info.Size()
		if files > domainartifact.MaxFileCount || info.Size() > domainartifact.MaxFileBytes || total > domainartifact.MaxTotalBytes {
			return fmt.Errorf("classified payload exceeds artifact limits: %w", errs.ErrValidation)
		}

		return nil
	})
}

func validateClassifiedPayloadChecksums(payload fs.FS, manifest *FileManifest) error {
	file, err := payload.Open(manifest.Checksums[0].Path)
	if err != nil {
		return err
	}

	const maxChecksumsBytes = 4 << 20

	body, readErr := io.ReadAll(io.LimitReader(file, maxChecksumsBytes+1))
	closeErr := file.Close()

	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if len(body) > maxChecksumsBytes {
		return fmt.Errorf("classified checksums exceed byte limit: %w", errs.ErrValidation)
	}

	subjects, err := provenance.ParseChecksums(bytes.NewReader(body))
	if err != nil {
		return err
	}

	paths := manifestChecksumSubjectPaths(manifest)
	if err := checkChecksumSubjects(subjects, sortedChecksumSubjectNames(paths)); err != nil {
		return err
	}

	for _, subject := range subjects {
		file, err := payload.Open(paths[subject.Name])
		if err != nil {
			return err
		}

		hash := sha256.New()
		_, copyErr := domainartifact.CopyAtMost(hash, file, domainartifact.MaxFileBytes)
		closeErr := file.Close()

		if copyErr != nil {
			return copyErr
		}

		if closeErr != nil {
			return closeErr
		}

		if hex.EncodeToString(hash.Sum(nil)) != subject.SHA256 {
			return fmt.Errorf("checksum digest mismatch for %s: %w", subject.Name, errs.ErrValidation)
		}
	}

	return nil
}
