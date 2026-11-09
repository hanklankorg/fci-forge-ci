// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/provenance"
)

// VerifyReleaseReadback composes bundle authentication and published checksum
// verification with the release evidence owner. Bounded provider reads and
// native signature verification are supplied by the caller.
func VerifyReleaseReadback(evidence *ReleaseEvidence, names []string, download func(string) ([]byte, error), verify func([]byte, []byte) error) (map[string]bool, error) {
	read, err := evidence.CaptureReadback(download)
	if err != nil {
		return nil, err
	}

	authenticatedChecksums := map[string][]byte{}
	verified, bundleErr := VerifyReleaseBundles(names, func(subject, bundle string) error {
		body, err := read(subject)
		if err != nil {
			return err
		}

		signature, err := read(bundle)
		if err != nil {
			return err
		}

		if err := verify(body, signature); err != nil {
			return err
		}

		if subject == "checksums.txt" || strings.HasSuffix(subject, "_checksums.txt") {
			authenticatedChecksums[subject] = body
		}

		return nil
	})
	failures := []error{bundleErr}

	// These are the very bytes handed to signature verification, not another
	// read of even the same asset name or retained file after authentication.
	for _, body := range authenticatedChecksums {
		failures = append(failures, VerifyReleaseChecksums(body, names, read))
	}

	return verified, errors.Join(failures...)
}

// VerifyReleaseChecksums binds a verified checksum document to the actual
// published payloads. Signature verification is caller-owned; this must only be
// called with an authenticated document. It does not assert that the consumer
// built its complete intended inventory or verify embedded package signatures.
// The read callback owns bounded provider reads and evidence retention.
func VerifyReleaseChecksums(document []byte, names []string, read func(string) ([]byte, error)) error {
	subjects, err := provenance.ParseChecksums(bytes.NewReader(document))
	if err != nil {
		return fmt.Errorf("release checksums: %w", err)
	}

	offered := make(map[string]bool, len(names))
	for _, name := range names {
		if offered[name] {
			return fmt.Errorf("duplicate release asset %q: %w", name, errReleaseEvidence)
		}

		offered[name] = true
	}

	seen := map[string]bool{}

	for _, subject := range subjects {
		name := subject.Name
		if name == "." || path.Base(name) != name || !offered[name] || seen[name] {
			return fmt.Errorf("checksum subject %q is not one unique offered asset: %w", name, errReleaseEvidence)
		}

		seen[name] = true

		body, err := read(name)
		if err != nil {
			return fmt.Errorf("read checksum subject %q: %w", name, err)
		}

		if fmt.Sprintf("%x", sha256.Sum256(body)) != subject.SHA256 {
			return fmt.Errorf("published checksum subject %q differs from signed digest: %w", name, errReleaseEvidence)
		}
	}

	return nil
}
